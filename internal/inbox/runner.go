package inbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gitlab.com/niharokz/arthik/internal/ai"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/config"
	"gitlab.com/niharokz/arthik/internal/model"
)

// Runner imports producer files into the review list and, when an AI is
// configured, categorises drafts nothing else could. Only the main device runs it.
type Runner struct {
	Book *book.Book
	Cfg  config.Inbox
	AI   *ai.Client // nil = no AI
	Loc  *time.Location
	Now  func() time.Time // clock (nil = time.Now); tests set it

	importMu sync.Mutex // one import at a time (ticker and "Check now")
	kSig     string     // kronos.md as last read
	sSig     string     // SMS files as last read
	seenRun  time.Time  // newest phone capture (last_run) the last SMS read saw
	lastScan time.Time  // last Kronos read, for the periodic re-scan
	aiPause  time.Time  // after an AI failure, wait before trying again
	lastDay  string
	paused   string // load error that stopped imports, logged once
	statusMu sync.Mutex
	status   Status
}

// Status is what the inbox screen shows about the background work.
type Status struct {
	AI         bool      `json:"ai"`
	LastImport time.Time `json:"last_import"`
	SMSTime    string    `json:"sms_time"` // daily SMS → drafts time ("" = as they arrive)
	NextSMS    time.Time `json:"next_sms"` // next scheduled SMS import
	LastSMS    time.Time `json:"last_sms"` // last time SMS were read
	Error      string    `json:"error,omitempty"`
	AIError    string    `json:"ai_error,omitempty"`
}

var runners sync.Map // dataset dir → *Runner

// Runs reports whether this device imports the inbox for the dataset in dir.
func Runs(dir string) bool {
	_, ok := runners.Load(dir)
	return ok
}

// StatusOf returns the runner status for a dataset (zero when none runs there).
func StatusOf(dir string) Status {
	if r, ok := runners.Load(dir); ok {
		rr := r.(*Runner)
		rr.statusMu.Lock()
		defer rr.statusMu.Unlock()
		return rr.status
	}
	return Status{}
}

// AIRetry is how long the runner waits after the AI could not be reached.
var AIRetry = 5 * time.Minute

// Run checks the producer files every interval until ctx ends.
func (r *Runner) Run(ctx context.Context, every time.Duration) {
	if r.Loc == nil {
		r.Loc = time.Local
	}
	r.setStatus(func(s *Status) { s.AI = r.AI != nil })
	runners.Store(r.Book.Dir, r)
	defer runners.Delete(r.Book.Dir)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		r.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sources says which producer files an import reads.
type sources struct{ kronos, sms bool }

// Tick imports Kronos drafts when kronos.md changed (or the day changed, or the
// re-scan interval passed), and bank SMS either the same way or once a day at the
// time set in Settings. Then it lets the AI look at one batch of drafts.
func (r *Runner) Tick(ctx context.Context) {
	if !r.Book.Main() {
		return
	}
	if le := r.Book.LoadError(); le != nil {
		if msg := le.Error(); msg != r.paused { // say it once, not every tick
			r.paused = msg
			log.Printf("[%s] WARN inbox import paused until the data files load again: %v", r.Book.Name, le)
			r.setStatus(func(s *Status) { s.Error = "Import paused — a data file can't be read: " + msg })
		}
		return
	}
	if r.paused != "" {
		r.paused = ""
		r.setStatus(func(s *Status) { s.Error = "" })
	}
	now := r.now()
	at, last := r.smsSchedule()
	r.importMu.Lock()
	w := r.due(now, at, last)
	r.importMu.Unlock()
	if w.kronos || w.sms {
		if _, err := r.run(w); err != nil {
			log.Printf("[%s] WARN inbox import: %v", r.Book.Name, err)
		}
	}
	r.showSchedule(now)
	if r.AI != nil && time.Now().After(r.aiPause) {
		r.categoriseBatch(ctx, 5)
	}
}

func (r *Runner) now() time.Time {
	t := time.Now()
	if r.Now != nil {
		t = r.Now()
	}
	if r.Loc != nil {
		t = t.In(r.Loc)
	}
	return t
}

// smsSchedule returns the SMS → drafts time from settings ("" = as they arrive)
// and when SMS were last imported.
func (r *Runner) smsSchedule() (at string, last time.Time) {
	_ = r.Book.View(func(d *book.Data, _ *book.Computed) error {
		at, last = d.Settings.InboxSMSTime, book.ParseStamp(d.SMSImport)
		return nil
	})
	return at, last
}

// due decides what to read this tick. Call with importMu held.
func (r *Runner) due(now time.Time, smsAt string, lastSMS time.Time) sources {
	rescan := r.Cfg.Rescan
	if rescan <= 0 {
		rescan = 10 * time.Minute
	}
	kSig, sSig := r.signatures()
	fresh := r.Book.Today().String() != r.lastDay || now.Sub(r.lastScan) >= rescan
	w := sources{kronos: kSig != r.kSig || fresh}
	if smsAt == "" {
		w.sms = sSig != r.sSig || fresh
	} else {
		w.sms = SMSDue(now, smsAt, lastSMS) || sSig != r.sSig && r.late(now, smsAt)
	}
	return w
}

// late reports a phone capture made before the last scheduled import but synced
// after it (Syncthing on the phone often waits until morning): it is read now
// instead of a day later.
func (r *Runner) late(now time.Time, smsAt string) bool {
	slot, ok := lastSlot(now, smsAt)
	run := LastCapture(r.Book.Dir)
	return ok && run.After(r.seenRun) && run.Before(slot)
}

// SMSDue reports whether the daily SMS import at hh:mm ("23:45") should run now:
// the latest such time has passed and nothing was imported since. After the
// server was down at that time, it runs once as soon as it is back.
func SMSDue(now time.Time, hhmm string, last time.Time) bool {
	slot, ok := lastSlot(now, hhmm)
	return ok && last.Before(slot)
}

// lastSlot is the latest hh:mm at or before now.
func lastSlot(now time.Time, hhmm string) (time.Time, bool) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, false
	}
	slot := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if slot.After(now) {
		slot = slot.AddDate(0, 0, -1)
	}
	return slot, true
}

// signatures change whenever kronos.md, or an SMS file (or the SMS rules), changes.
func (r *Runner) signatures() (kronos, sms string) {
	dir := filepath.Join(r.Book.Dir, book.InboxDir)
	files, _ := filepath.Glob(filepath.Join(dir, SMSGlob))
	files = append(files, filepath.Join(r.Book.Dir, RulesFile))
	sort.Strings(files)
	return fileSig(filepath.Join(dir, KronosFile)), fileSig(files...)
}

func fileSig(files ...string) string {
	var b strings.Builder
	for _, f := range files {
		if st, err := os.Stat(f); err == nil { // size too: a copy can keep an older mtime
			fmt.Fprintf(&b, "%s|%d|%d|", f, st.ModTime().UnixNano(), st.Size())
		}
	}
	return b.String()
}

// Import reads every producer file now — SMS too, whatever the schedule — and
// adds what is new; it returns how many drafts were added ("Check now").
func (r *Runner) Import() (int, error) {
	added, err := r.run(sources{kronos: true, sms: true})
	r.showSchedule(r.now())
	return added, err
}

// ImportFor runs an import for the dataset in dir; ok is false when no importer
// runs there (this device is not the main device).
func ImportFor(dir string) (added int, ok bool, err error) {
	v, found := runners.Load(dir)
	if !found {
		return 0, false, nil
	}
	added, err = v.(*Runner).Import()
	return added, true, err
}

// run reads the chosen producer files and adds new drafts. A failed read is
// retried on the next tick (the signatures are only taken on success).
func (r *Runner) run(w sources) (int, error) {
	r.importMu.Lock()
	defer r.importMu.Unlock()
	kSig, sSig := r.signatures()
	day, now := r.Book.Today().String(), r.now()
	var items []Item
	var errs []error
	var seen time.Time
	if w.kronos {
		k, e := ReadKronos(r.Book.Dir)
		items, errs = append(items, k...), append(errs, e...)
	}
	if w.sms {
		p, err := LoadParser(r.Book.Dir)
		if err != nil {
			r.setStatus(func(s *Status) { s.Error = err.Error() })
			return 0, err
		}
		m, e := ReadSMS(r.Book.Dir, p, r.Loc)
		items, errs = append(items, m...), append(errs, e...)
		seen = LastCapture(r.Book.Dir)
	}
	added := 0
	err := r.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		added = Ingest(d, ch, items, Options{Today: r.Book.Today(), Now: time.Now(), MaxAge: r.Cfg.MaxAgeDays, KeepDays: r.Cfg.KeepDays, PairDays: r.Cfg.PairDays})
		if w.sms {
			d.SMSImport, ch.Drafts = book.Stamp(now), true
		}
		return nil
	})
	if err == nil {
		if w.kronos {
			r.kSig, r.lastScan, r.lastDay = kSig, now, day
		}
		if w.sms {
			r.sSig, r.seenRun = sSig, seen
		}
	}
	if err == nil && len(errs) > 0 {
		err = errors.Join(errs...)
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	r.setStatus(func(s *Status) {
		s.LastImport, s.Error = time.Now(), msg
		if w.sms {
			s.LastSMS = now
		}
	})
	if added > 0 {
		log.Printf("[%s] inbox: %d new draft(s) to review", r.Book.Name, added)
	}
	return added, err
}

// showSchedule puts the SMS schedule on the inbox screen.
func (r *Runner) showSchedule(now time.Time) {
	at, last := r.smsSchedule()
	next := time.Time{}
	if slot, ok := lastSlot(now, at); ok {
		next = slot.AddDate(0, 0, 1)
	}
	r.setStatus(func(s *Status) {
		s.SMSTime, s.NextSMS = at, next
		if !last.IsZero() {
			s.LastSMS = last
		}
	})
}

// categoriseBatch asks the AI about up to n drafts that only have the fallback
// category (or none). The lock is never held while waiting for the model.
func (r *Runner) categoriseBatch(ctx context.Context, n int) {
	type job struct {
		id  string
		req ai.Request
	}
	var jobs []job
	_ = r.Book.View(func(d *book.Data, c *book.Computed) error {
		for _, x := range d.Drafts {
			if len(jobs) >= n {
				break
			}
			if needsAI(x) {
				jobs = append(jobs, job{x.ID, request(d, x)})
			}
		}
		return nil
	})
	for _, j := range jobs {
		ans, err := r.AI.Categorise(ctx, j.req)
		if err != nil {
			r.aiPause = time.Now().Add(AIRetry)
			r.setStatus(func(s *Status) { s.AIError = err.Error() })
			log.Printf("[%s] WARN inbox AI: %v (retrying in %s)", r.Book.Name, err, AIRetry)
			return
		}
		r.setStatus(func(s *Status) { s.AIError = "" })
		err = r.Book.Mutate(func(d *book.Data, ch *book.Change) error {
			x, ok := d.Draft(j.id)
			if !ok || !needsAI(x) { // reviewed or edited meanwhile
				return nil
			}
			x.AITried = true
			if ans.Category != "" && categoryFits(d, ans.Category, x.Type) {
				x.Category, x.Via = ans.Category, model.ViaAI
			}
			if ans.Title != "" && x.Via != model.ViaLearned && x.Merchant != "" {
				x.Title = ans.Title
			}
			d.SetDraft(x, ch)
			return nil
		})
		if err != nil {
			log.Printf("[%s] WARN inbox AI save: %v", r.Book.Name, err)
			return
		}
	}
}

func needsAI(x model.Draft) bool {
	return x.Status == model.DraftPending && !x.Edited && !x.AITried &&
		(x.Type == model.TxExpense || x.Type == model.TxIncome) &&
		(x.Category == "" || x.Via == model.ViaDefault)
}

// request builds the AI question: the draft, your categories of that kind and a
// sample of how you named and categorised past transactions.
func request(d *book.Data, x model.Draft) ai.Request {
	req := ai.Request{Kind: x.Type, Amount: x.Amount.String(), Merchant: x.Merchant, Text: x.Raw}
	if x.CategoryHint != "" { // e.g. Kronos: the subscription's own category name
		req.Text = strings.TrimSpace(req.Text + "\nThe bill's own category: " + x.CategoryHint)
	}
	if req.Merchant == "" {
		req.Merchant = x.Title
	}
	for _, c := range d.Categories {
		if !categoryFits(d, c.ID, x.Type) || c.Hidden {
			continue
		}
		name := c.Name
		if p, ok := d.Category(c.Parent); ok {
			name = p.Name + " › " + c.Name
		}
		req.Choices = append(req.Choices, ai.Choice{ID: c.ID, Name: name})
	}
	seen := map[string]bool{}
	for i := len(d.Txns) - 1; i >= 0 && len(req.Examples) < 30; i-- {
		t := d.Txns[i]
		k := Key(t.Title)
		if t.Type != x.Type || seen[k] {
			continue
		}
		if e := book.EntryOf(t); e.Category != "" {
			seen[k] = true
			req.Examples = append(req.Examples, ai.Example{Title: t.Title, Category: e.Category})
		}
	}
	return req
}

func (r *Runner) setStatus(fn func(*Status)) {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	fn(&r.status)
}
