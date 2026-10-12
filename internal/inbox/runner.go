package inbox

import (
	"context"
	"errors"
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

	sigs     string
	aiPause  time.Time // after an AI failure, wait before trying again
	lastDay  string
	statusMu sync.Mutex
	status   Status
}

// Status is what the inbox screen shows about the background work.
type Status struct {
	AI         bool      `json:"ai"`
	LastImport time.Time `json:"last_import"`
	Error      string    `json:"error,omitempty"`
	AIError    string    `json:"ai_error,omitempty"`
}

var runners sync.Map // dataset dir → *Runner

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

// Tick imports when a producer file changed (or the day changed), then lets the
// AI look at one batch of drafts.
func (r *Runner) Tick(ctx context.Context) {
	if !r.Book.Main() || r.Book.LoadError() != nil {
		return
	}
	day := r.Book.Today().String()
	if sig := r.signature(); sig != r.sigs || day != r.lastDay {
		if err := r.importNow(); err != nil {
			log.Printf("[%s] WARN inbox import: %v", r.Book.Name, err)
		} else {
			r.sigs, r.lastDay = sig, day
		}
	}
	if r.AI != nil && time.Now().After(r.aiPause) {
		r.categoriseBatch(ctx, 5)
	}
}

// signature changes whenever a producer file (or the user's SMS rules) changes.
func (r *Runner) signature() string {
	dir := filepath.Join(r.Book.Dir, book.InboxDir)
	files, _ := filepath.Glob(filepath.Join(dir, SMSGlob))
	files = append(files, filepath.Join(dir, KronosFile), filepath.Join(r.Book.Dir, RulesFile))
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		if st, err := os.Stat(f); err == nil {
			b.WriteString(f + st.ModTime().String() + "|")
		}
	}
	return b.String()
}

func (r *Runner) importNow() error {
	p, err := LoadParser(r.Book.Dir)
	if err != nil {
		r.setStatus(func(s *Status) { s.Error = err.Error() })
		return err
	}
	items, errs := ReadSources(r.Book.Dir, p, r.Loc)
	added := 0
	err = r.Book.Mutate(func(d *book.Data, ch *book.Change) error {
		added = Ingest(d, ch, items, Options{Today: r.Book.Today(), Now: time.Now(), MaxAge: r.Cfg.MaxAgeDays, KeepDays: r.Cfg.KeepDays, PairDays: r.Cfg.PairDays})
		return nil
	})
	if err == nil && len(errs) > 0 {
		err = errors.Join(errs...)
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	r.setStatus(func(s *Status) { s.LastImport, s.Error = time.Now(), msg })
	if added > 0 {
		log.Printf("[%s] inbox: %d new draft(s) to review", r.Book.Name, added)
	}
	return err
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
