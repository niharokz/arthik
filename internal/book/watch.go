package book

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

// sig is a cheap file signature (size + modification time).
type sig struct {
	size int64
	mod  time.Time
}

// watched lists the files whose edits by other apps should be picked up.
// money.md is output-only and deliberately not watched.
func (b *Book) watched() []string {
	files := []string{b.path(SettingsFile), b.path(AccountsFile), b.path(CategoriesFile), b.path(LabelsFile), b.path(RemindersFile)}
	tx, _ := filepath.Glob(filepath.Join(b.Dir, TxDir, "transaction_*.md"))
	return append(files, tx...)
}

func (b *Book) scanLocked() map[string]sig {
	out := map[string]sig{}
	for _, f := range b.watched() {
		if st, err := os.Stat(f); err == nil {
			out[f] = sig{st.Size(), st.ModTime()}
		}
	}
	return out
}

func (b *Book) changedOnDiskLocked() bool {
	now := b.scanLocked()
	if len(now) != len(b.sigs) {
		return true
	}
	for f, s := range now {
		if o, ok := b.sigs[f]; !ok || o != s {
			return true
		}
	}
	return false
}

// Watch polls the data files every interval and reloads + rebuilds when another app
// (Obsidian, Syncthing, a text editor) changed them. A change is acted on once it has
// been stable for one interval, so half-synced files are not read mid-write.
func (b *Book) Watch(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	var last map[string]sig // signatures at the previous tick while a change is pending
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		b.mu.RLock()
		changed := b.changedOnDiskLocked()
		now := b.scanLocked()
		b.mu.RUnlock()
		if !changed {
			last = nil
			continue
		}
		if !sameSigs(last, now) { // still being written (or first sighting): wait a tick
			last = now
			continue
		}
		last = nil
		b.mu.Lock()
		if b.changedOnDiskLocked() {
			if err := b.reloadLocked(); err != nil {
				log.Printf("[%s] WARN external edit could not be loaded: %v", b.Name, err)
			} else {
				log.Printf("[%s] external edit picked up — computed files rebuilt", b.Name)
			}
		}
		b.mu.Unlock()
	}
}

func sameSigs(a, b map[string]sig) bool {
	if a == nil || len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
