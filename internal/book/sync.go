package book

// Multi-device sync support.
//
// The same data folder is shared by several arthik instances through Syncthing:
// the server on nimory (the main device) and the phone app (a replica). When two
// devices change the same file before they sync, Syncthing keeps the newer copy
// and saves the other as "<name>.sync-conflict-<date>-<time>-<device>.md".
//
// arthik makes that safe:
//   - every transaction carries an "updated" stamp; a deletion leaves a tombstone;
//   - on load, all copies of a transaction (main files, conflict copies, a copy left
//     in an old month file) are resolved by id — the newest stamp wins;
//   - definition files (accounts, categories, labels) merge by id, the main copy
//     winning on the same id; inbox drafts and learned rules merge by id, newest
//     "updated" winning;
//   - only the main device writes the result back and deletes the conflict copies;
//     a replica merges in memory so its totals are right before the main device
//     catches up.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/store"
)

// TombstoneTTL is how long a deletion is remembered before the main device forgets it.
const TombstoneTTL = 90 * 24 * time.Hour

// Now is the clock used for "updated" / "deleted" stamps (replaced in tests).
var Now = time.Now

// stamp returns the current time in the stored format.
func stamp() string { return Now().UTC().Format(stampLayout) }

// stampLayout is RFC 3339 with milliseconds, e.g. 2026-10-10T18:10:12.345Z.
const stampLayout = "2006-01-02T15:04:05.000Z07:00"

// parseStamp reads a stored stamp; anything unreadable counts as "very old".
func parseStamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// version is the time a record last changed (deletion time for tombstones).
func version(t model.Transaction) time.Time {
	if t.IsTombstone() {
		return parseStamp(t.Deleted)
	}
	return parseStamp(t.Updated)
}

// monthFileRe matches transaction_YYYYMM.md and its Syncthing conflict copies.
var monthFileRe = regexp.MustCompile(`^transaction_(\d{6})(\.sync-conflict-.+)?\.md$`)

// conflictRe matches a conflict copy of a top-level file, e.g. accounts.sync-conflict-….md.
var conflictRe = regexp.MustCompile(`^([a-z]+)\.sync-conflict-.+\.md$`)

// IsConflictCopy reports whether a file name is a Syncthing conflict copy.
func IsConflictCopy(name string) bool { return strings.Contains(name, ".sync-conflict-") }

// loadedTx is a transaction plus where it was read from.
type loadedTx struct {
	t        model.Transaction
	month    string // month key of the file it came from
	conflict string // path of the conflict copy, "" for a main file
}

// txResolution is the outcome of resolving every loaded transaction record.
type txResolution struct {
	live      []model.Transaction
	tombs     map[string]model.Transaction
	src       map[string]string // id → month file the record came from
	dirty     map[string]bool   // month files that must be rewritten
	conflicts map[string]int    // conflict copy path → records taken from it
	problems  []Problem
}

// resolveTxns picks one winner per id: newest stamp wins; on a tie a main-file record
// beats a conflict copy; identical duplicates collapse; real ties keep both (renamed).
func resolveTxns(all []loadedTx, now time.Time, purge bool) txResolution {
	r := txResolution{tombs: map[string]model.Transaction{}, src: map[string]string{}, dirty: map[string]bool{}, conflicts: map[string]int{}}
	taken := map[string]bool{}
	for _, l := range all {
		if l.t.ID != "" {
			taken[l.t.ID] = true
		}
	}
	best := map[string]loadedTx{}
	var order []string
	for _, l := range all {
		if l.conflict != "" {
			r.conflicts[l.conflict] += 0 // remember every copy, even an empty one
		}
		if l.t.ID == "" { // hand-typed without id: give it one
			l.t.ID = NewTxID(l.t.Date, func(s string) bool { return taken[s] })
			taken[l.t.ID] = true
			r.dirty[l.month] = true
		}
		cur, seen := best[l.t.ID]
		if !seen {
			best[l.t.ID] = l
			order = append(order, l.t.ID)
			continue
		}
		vNew, vCur := version(l.t), version(cur.t)
		switch {
		case vNew.After(vCur), vNew.Equal(vCur) && cur.conflict != "" && l.conflict == "":
			best[l.t.ID] = l // newer wins; on a tie the main file wins
			r.dirty[cur.month] = true
		case vNew.Equal(vCur) && l.conflict == "" && cur.conflict == "" && !reflect.DeepEqual(l.t, cur.t):
			// Two different records share an id in main files: keep both.
			nid := NewTxID(l.t.Date, func(s string) bool { return taken[s] })
			taken[nid] = true
			r.problems = append(r.problems, Problem{File: TxDir, ID: nid, Msg: fmt.Sprintf("two different transactions used id %q; the second was renamed to %s", l.t.ID, nid)})
			l.t.ID = nid
			best[nid] = l
			order = append(order, nid)
			r.dirty[l.month] = true
		default:
			r.dirty[l.month] = true // older or identical copy: drop it
		}
	}
	for _, id := range order {
		w := best[id]
		if w.conflict != "" {
			r.conflicts[w.conflict]++
			r.dirty[w.month] = true // copy content moves into the main file
		}
		target := w.month
		if dd, err := dates.Parse(w.t.Date); err == nil {
			target = dd.MonthKey()
		}
		if target != w.month { // record sits in the wrong month file
			r.dirty[w.month], r.dirty[target] = true, true
		}
		if w.t.IsTombstone() {
			if purge && now.Sub(parseStamp(w.t.Deleted)) > TombstoneTTL {
				r.dirty[w.month] = true // old enough to forget
				continue
			}
			r.tombs[id] = w.t
			r.src[id] = w.month
			continue
		}
		r.live = append(r.live, w.t)
		r.src[id] = w.month
	}
	return r
}

// unionByID adds records from other whose id is missing in main (main wins on the same id).
func unionByID[T any](main, other []T, id func(T) string) ([]T, int) {
	have := map[string]bool{}
	for _, m := range main {
		have[id(m)] = true
	}
	added := 0
	for _, o := range other {
		if k := id(o); k != "" && !have[k] {
			main = append(main, o)
			have[k] = true
			added++
		}
	}
	return main, added
}

// defConflicts merges conflict copies of the definition files into d.
// It returns the copies handled and which definition files changed.
func (b *Book) defConflicts(d *Data, ch *Change, notes *[]Problem) []string {
	files, _ := filepath.Glob(filepath.Join(b.Dir, "*.sync-conflict-*.md"))
	sort.Strings(files)
	var done []string
	for _, f := range files {
		m := conflictRe.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			continue
		}
		name, added := filepath.Base(f), 0
		var err error
		switch m[1] {
		case "accounts":
			var doc accountsDoc
			if _, err = store.Read(f, &doc); err == nil {
				d.Accounts, added = unionByID(d.Accounts, doc.Accounts, func(a model.Account) string { return a.ID })
				ch.Accounts = ch.Accounts || added > 0
			}
		case "categories":
			var doc categoriesDoc
			if _, err = store.Read(f, &doc); err == nil {
				d.Categories, added = unionByID(d.Categories, doc.Categories, func(c model.Category) string { return c.ID })
				ch.Categories = ch.Categories || added > 0
			}
		case "labels":
			var doc labelsDoc
			if _, err = store.Read(f, &doc); err == nil {
				d.Labels, added = unionByID(d.Labels, doc.Labels, func(l model.Label) string { return l.ID })
				ch.Labels = ch.Labels || added > 0
			}
		case "settings", "money":
			// main copy wins; money.md is rebuilt anyway
		default:
			continue // not an arthik file
		}
		if err != nil {
			*notes = append(*notes, Problem{File: name, Level: LevelWarn, Msg: "conflict copy could not be read: " + err.Error()})
			continue
		}
		*notes = append(*notes, Problem{File: name, Level: LevelInfo, Msg: fmt.Sprintf("conflict copy merged (%d record(s) added)", added)})
		done = append(done, f)
	}
	return append(done, b.inboxConflicts(d, ch, notes)...)
}

// inboxConflicts merges conflict copies of inbox/drafts.md and inbox/learned.md:
// both devices may change a draft (accept on the phone, import on nimory), so the
// record with the newest "updated" stamp wins.
func (b *Book) inboxConflicts(d *Data, ch *Change, notes *[]Problem) []string {
	files, _ := filepath.Glob(filepath.Join(b.Dir, InboxDir, "*.sync-conflict-*.md"))
	sort.Strings(files)
	var done []string
	for _, f := range files {
		m := conflictRe.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			continue
		}
		name, taken := filepath.Base(f), 0
		var err error
		switch m[1] {
		case "drafts":
			var doc draftsDoc
			if _, err = store.Read(f, &doc); err == nil {
				d.Drafts, taken = newestByID(d.Drafts, doc.Drafts, func(x model.Draft) (string, string) { return x.ID, x.Updated })
				ch.Drafts = ch.Drafts || taken > 0
			}
		case "learned":
			var doc rulesDoc
			if _, err = store.Read(f, &doc); err == nil {
				d.Rules, taken = newestByID(d.Rules, doc.Rules, func(r model.Rule) (string, string) { return r.Match, r.Updated })
				ch.Rules = ch.Rules || taken > 0
			}
		default:
			continue // source files (kronos.md, sms_*.md) belong to their producer
		}
		if err != nil {
			*notes = append(*notes, Problem{File: name, Msg: "conflict copy could not be read: " + err.Error()})
			continue
		}
		*notes = append(*notes, Problem{File: name, Level: LevelInfo, Msg: fmt.Sprintf("conflict copy merged (%d record(s) taken)", taken)})
		done = append(done, f)
	}
	return done
}

// newestByID merges other into main by key: a record with a newer stamp replaces
// the main one, unknown keys are added.
func newestByID[T any](main, other []T, key func(T) (string, string)) ([]T, int) {
	at := map[string]int{}
	for i, m := range main {
		k, _ := key(m)
		at[k] = i
	}
	taken := 0
	for _, o := range other {
		k, st := key(o)
		if k == "" {
			continue
		}
		i, ok := at[k]
		if !ok {
			main = append(main, o)
			at[k] = len(main) - 1
			taken++
			continue
		}
		if _, cur := key(main[i]); parseStamp(st).After(parseStamp(cur)) {
			main[i] = o
			taken++
		}
	}
	return main, taken
}

// removeConflicts deletes merged conflict copies (main device only).
func (b *Book) removeConflicts(paths []string) {
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("[%s] WARN could not remove %s: %v", b.Name, filepath.Base(p), err)
		} else {
			log.Printf("[%s] merged and removed conflict copy %s", b.Name, filepath.Base(p))
		}
	}
}

// Tombstoned reports whether id was deleted (and the deletion is still remembered).
func (d *Data) Tombstoned(id string) bool {
	_, ok := d.tombs[id]
	return ok
}
