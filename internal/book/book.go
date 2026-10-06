// Package book is Arthik's ledger engine for one dataset (one folder of data files).
//
// It keeps the whole dataset in memory (personal scale: thousands of transactions,
// not millions), serves reads from that copy, and turns every change into:
//
//  1. a validated, balanced edit of the in-memory data,
//  2. an atomic rewrite of only the files that changed (definition files and the
//     affected transaction_YYYYMM.md months), then
//  3. a rebuild of every computed value — account balances in accounts.md,
//     category totals in categories.md, label counts in labels.md and the overview
//     in money.md — written back only when they actually changed.
//
// Transactions are the single source of truth; everything computed can be thrown
// away and rebuilt (`arthik rebuild`). Edits made to the files by other apps
// (Obsidian, Syncthing) are picked up by the watcher (watch.go); future-dated and
// recurring transactions are handled by the scheduler (schedule.go).
package book

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/store"
)

// File names inside a dataset folder.
const (
	SettingsFile   = "settings.md"
	AccountsFile   = "accounts.md"
	CategoriesFile = "categories.md"
	LabelsFile     = "labels.md"
	RemindersFile  = "reminders.md"
	MoneyFile      = "money.md"
	TxDir          = "transactions"
)

// ErrReadOnly is returned for any change to a read-only (demo) book.
var ErrReadOnly = errors.New("this is the read-only demo — changes are disabled")

var monthFileRe = regexp.MustCompile(`^transaction_(\d{6})\.md$`)

// Data is the full in-memory dataset.
type Data struct {
	Settings   model.Settings
	Accounts   []model.Account
	Categories []model.Category
	Labels     []model.Label
	Reminders  []model.Reminder
	Txns       []model.Transaction // sorted by date, time, id

	src map[string]string // transaction id → month key of the file it came from
	idx index
}

type index struct {
	acc  map[string]int
	cat  map[string]int
	lab  map[string]int
	rem  map[string]int
	tx   map[string]int
	kids map[string][]string // parent category id → child ids
}

// Problem is a data issue that does not stop the app (the record is excluded from totals).
type Problem struct {
	File string `json:"file" yaml:"file"`
	ID   string `json:"id,omitempty" yaml:"id,omitempty"`
	Msg  string `json:"msg" yaml:"msg"`
}

// Change marks which files a mutation touched.
type Change struct {
	Settings, Accounts, Categories, Labels, Reminders bool
	Months                                            map[string]bool
}

// Month marks a transaction month file as changed.
func (c *Change) Month(key string) {
	if c.Months == nil {
		c.Months = map[string]bool{}
	}
	if key != "" {
		c.Months[key] = true
	}
}

func (c *Change) any() bool {
	return c.Settings || c.Accounts || c.Categories || c.Labels || c.Reminders || len(c.Months) > 0
}

// Book is one dataset.
type Book struct {
	Dir      string
	ReadOnly bool
	Name     string            // "main" or "demo", for logs
	Today    func() dates.Date // injectable clock (tests)

	mu       sync.RWMutex
	data     *Data
	comp     *Computed
	loadErr  error // a data file could not be parsed; writes are refused until fixed
	problems []Problem
	sigs     map[string]sig // file signatures after our last load/write (watch.go)
}

// Open loads the dataset in dir (creating the folder and default files if needed).
func Open(dir, name string, readOnly bool) (*Book, error) {
	b := &Book{Dir: dir, Name: name, ReadOnly: readOnly, Today: dates.Today}
	if !readOnly {
		if err := os.MkdirAll(filepath.Join(dir, TxDir), 0o755); err != nil {
			return nil, fmt.Errorf("create data folder: %w", err)
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.reloadLocked(); err != nil {
		log.Printf("[%s] WARN %v — reads use what could be loaded, changes are blocked until the file is fixed", name, err)
	}
	return b, nil
}

// LoadError returns the current blocking data-file error, if any.
func (b *Book) LoadError() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.loadErr
}

// View runs fn with read access to the data and the computed values.
func (b *Book) View(fn func(d *Data, c *Computed) error) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return fn(b.data, b.comp)
}

// Problems returns load problems plus validation problems.
func (b *Book) Problems() []Problem {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := append([]Problem(nil), b.problems...)
	if b.comp != nil {
		out = append(out, b.comp.Problems...)
	}
	return out
}

// Mutate applies fn to a copy of the data. fn must validate before changing anything
// and must replace (not edit in place) the records it changes. On success the
// touched files are written and computed values rebuilt.
func (b *Book) Mutate(fn func(d *Data, c *Change) error) error {
	if b.ReadOnly {
		return ErrReadOnly
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Pick up any edit another app saved since our last look, so we build on it.
	if b.changedOnDiskLocked() {
		if err := b.reloadLocked(); err != nil {
			return err
		}
	}
	if b.loadErr != nil {
		return fmt.Errorf("fix the data file first: %w", b.loadErr)
	}
	nd := b.data.clone()
	var ch Change
	if err := fn(nd, &ch); err != nil {
		return err
	}
	if !ch.any() {
		return nil
	}
	nd.sortTxns()
	nd.reindex()
	if err := b.persistLocked(nd, &ch); err != nil {
		// Disk and memory may disagree now; start again from what is on disk.
		_ = b.reloadLocked()
		return fmt.Errorf("save failed: %w", err)
	}
	b.data = nd
	b.recomputeLocked(true)
	return nil
}

// Reload re-reads every file (used by the watcher and `arthik rebuild`).
func (b *Book) Reload() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reloadLocked()
}

// Recompute rebuilds computed values (e.g. after midnight) and writes them if changed.
func (b *Book) Recompute() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recomputeLocked(!b.ReadOnly)
}

// ---------------------------------------------------------------------------
// loading

type settingsDoc struct {
	Settings model.Settings `yaml:"settings"`
}
type accountsDoc struct {
	Accounts []model.Account `yaml:"accounts"`
}
type categoriesDoc struct {
	Categories []model.Category `yaml:"categories"`
}
type labelsDoc struct {
	Labels []model.Label `yaml:"labels"`
}
type remindersDoc struct {
	Reminders []model.Reminder `yaml:"reminders"`
}
type txDoc struct {
	Transactions []model.Transaction `yaml:"transactions"`
}

func (b *Book) path(name string) string { return filepath.Join(b.Dir, name) }

func monthPath(dir, key string) string {
	return filepath.Join(dir, TxDir, "transaction_"+key+".md")
}

// reloadLocked reads everything from disk. On a parse error it keeps the previous
// good data (if any) for reads and sets loadErr so writes are refused.
func (b *Book) reloadLocked() error {
	d := &Data{Settings: model.DefaultSettings(), src: map[string]string{}}
	var problems []Problem
	var errs []string

	read := func(name string, v any) {
		if _, err := store.Read(b.path(name), v); err != nil {
			errs = append(errs, err.Error())
		}
	}
	var sd settingsDoc
	sd.Settings = model.DefaultSettings()
	read(SettingsFile, &sd)
	d.Settings = normSettings(sd.Settings)
	var ad accountsDoc
	read(AccountsFile, &ad)
	d.Accounts = ad.Accounts
	var cd categoriesDoc
	read(CategoriesFile, &cd)
	d.Categories = cd.Categories
	var ld labelsDoc
	read(LabelsFile, &ld)
	d.Labels = ld.Labels
	var rd remindersDoc
	read(RemindersFile, &rd)
	d.Reminders = rd.Reminders

	files, _ := filepath.Glob(filepath.Join(b.Dir, TxDir, "transaction_*.md"))
	sort.Strings(files)
	for _, f := range files {
		m := monthFileRe.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			continue
		}
		var td txDoc
		if _, err := store.Read(f, &td); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, t := range td.Transactions {
			d.Txns = append(d.Txns, t)
			if t.ID != "" {
				d.src[t.ID] = m[1]
			}
		}
	}

	if len(errs) > 0 {
		b.loadErr = errors.New(strings.Join(errs, "; "))
		if b.data == nil { // first load: serve whatever parsed
			d.sortTxns()
			d.reindex()
			b.data = d
		}
		b.sigs = b.scanLocked()
		b.recomputeLocked(false)
		return b.loadErr
	}
	b.loadErr = nil

	// Repair what can be repaired safely (ids, misfiled months, unknown labels).
	ch := d.fixup(&problems)
	d.sortTxns()
	d.reindex()
	if ch.any() && !b.ReadOnly {
		if err := b.persistLocked(d, &ch); err != nil {
			log.Printf("[%s] WARN could not save repairs: %v", b.Name, err)
		} else {
			log.Printf("[%s] repaired data files (ids / months / labels)", b.Name)
		}
	}
	b.data = d
	b.problems = problems
	b.sigs = b.scanLocked()
	b.recomputeLocked(!b.ReadOnly)
	return nil
}

func normSettings(s model.Settings) model.Settings {
	def := model.DefaultSettings()
	if s.DefaultPeriod == "" || s.DefaultPeriod == "custom" {
		s.DefaultPeriod = def.DefaultPeriod
	}
	if s.WeekStart == "" {
		s.WeekStart = def.WeekStart
	}
	if s.MonthStartDay < 1 || s.MonthStartDay > 28 {
		s.MonthStartDay = 1
	}
	if s.YearStartMonth < 1 || s.YearStartMonth > 12 {
		s.YearStartMonth = 1
	}
	if s.UpcomingDays <= 0 {
		s.UpcomingDays = def.UpcomingDays
	}
	if s.Theme == "" {
		s.Theme = def.Theme
	}
	return s
}

// fixup repairs missing/duplicate ids, transactions saved in the wrong month file
// and labels used on transactions but missing from labels.md.
func (d *Data) fixup(problems *[]Problem) Change {
	var ch Change
	seen := map[string]bool{}
	for i := range d.Accounts {
		a := &d.Accounts[i]
		if a.ID == "" {
			a.ID = uniqueID(Slug(a.Name), func(s string) bool { return seen["a:"+s] })
			ch.Accounts = true
		}
		seen["a:"+a.ID] = true
	}
	for i := range d.Categories {
		c := &d.Categories[i]
		if c.ID == "" {
			base := Slug(c.Name)
			if c.Parent != "" {
				base = c.Parent + "-" + base
			}
			c.ID = uniqueID(base, func(s string) bool { return seen["c:"+s] })
			ch.Categories = true
		}
		seen["c:"+c.ID] = true
	}
	for i := range d.Reminders {
		r := &d.Reminders[i]
		if r.ID == "" {
			r.ID = uniqueID("r-"+Slug(r.Title), func(s string) bool { return seen["r:"+s] })
			ch.Reminders = true
		}
		seen["r:"+r.ID] = true
	}
	labels := map[string]bool{}
	for _, l := range d.Labels {
		labels[l.ID] = true
	}
	for i := range d.Txns {
		t := &d.Txns[i]
		oldSrc := d.src[t.ID]
		if t.ID == "" || seen["t:"+t.ID] {
			nid := NewTxID(t.Date, func(s string) bool { return seen["t:"+s] })
			*problems = append(*problems, Problem{File: TxDir, ID: nid, Msg: fmt.Sprintf("transaction %q had a missing or duplicate id; renamed to %s", t.ID, nid)})
			if oldSrc == "" {
				// duplicate: the original kept the src entry; we can only trust the date
				if dd, err := dates.Parse(t.Date); err == nil {
					oldSrc = dd.MonthKey()
				}
			}
			t.ID = nid
			ch.Month(oldSrc)
		}
		seen["t:"+t.ID] = true
		if oldSrc != "" {
			d.src[t.ID] = oldSrc
		}
		if dd, err := dates.Parse(t.Date); err == nil && dd.MonthKey() != d.src[t.ID] {
			ch.Month(d.src[t.ID])
			ch.Month(dd.MonthKey())
			d.src[t.ID] = dd.MonthKey()
		}
		for _, l := range t.Labels {
			if l != "" && !labels[l] {
				labels[l] = true
				d.Labels = append(d.Labels, model.Label{ID: l, Name: l})
				ch.Labels = true
			}
		}
	}
	return ch
}

// ---------------------------------------------------------------------------
// saving

const (
	hdrSettings   = "arthik settings — safe to edit."
	hdrAccounts   = "arthik accounts — edit freely; balance, cleared_balance and projected_balance are computed from the transactions and rewritten by arthik.\nopening_balance is signed: money you owe (credit cards, loans) is negative."
	hdrCategories = "arthik categories — edit freely; period and total are computed for the default period in settings.md and rewritten by arthik.\nkind: expense | income | both.  parent: id of the parent category (two levels)."
	hdrLabels     = "arthik labels — count is computed."
	hdrReminders  = "arthik reminders — scheduled transactions, auto-posted on each due date. next/anchor_day/posted/remaining/done are maintained by arthik."
	hdrTx         = "arthik transactions — one file per month. Each transaction's postings must sum to zero (+ debit, - credit)."
)

func (b *Book) persistLocked(d *Data, ch *Change) error {
	w := func(name, hdr string, v any) error {
		_, err := store.Write(b.path(name), hdr, v, 0o644)
		return err
	}
	if ch.Settings {
		if err := w(SettingsFile, hdrSettings, settingsDoc{d.Settings}); err != nil {
			return err
		}
	}
	if ch.Accounts {
		if err := w(AccountsFile, hdrAccounts, accountsDoc{nonNil(d.Accounts)}); err != nil {
			return err
		}
	}
	if ch.Categories {
		if err := w(CategoriesFile, hdrCategories, categoriesDoc{nonNil(d.Categories)}); err != nil {
			return err
		}
	}
	if ch.Labels {
		if err := w(LabelsFile, hdrLabels, labelsDoc{nonNil(d.Labels)}); err != nil {
			return err
		}
	}
	if ch.Reminders {
		if err := w(RemindersFile, hdrReminders, remindersDoc{nonNil(d.Reminders)}); err != nil {
			return err
		}
	}
	if len(ch.Months) > 0 {
		byMonth := map[string][]model.Transaction{}
		for _, t := range d.Txns {
			k := d.MonthOf(t)
			if ch.Months[k] {
				byMonth[k] = append(byMonth[k], t)
			}
		}
		keys := make([]string, 0, len(ch.Months))
		for k := range ch.Months {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := monthPath(b.Dir, k)
			txs := byMonth[k]
			if len(txs) == 0 {
				if _, err := os.Stat(p); err != nil {
					continue // nothing there and nothing to write
				}
			}
			if _, err := store.Write(p, hdrTx, txDoc{nonNil(txs)}, 0o644); err != nil {
				return err
			}
		}
	}
	b.sigs = b.scanLocked()
	return nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// MonthOf is the month key a transaction is saved under: its date's month, or the
// file it came from when its date cannot be parsed.
func (d *Data) MonthOf(t model.Transaction) string {
	if dd, err := dates.Parse(t.Date); err == nil {
		return dd.MonthKey()
	}
	if k := d.src[t.ID]; k != "" {
		return k
	}
	return dates.Today().MonthKey()
}

// ---------------------------------------------------------------------------
// in-memory helpers

func (d *Data) clone() *Data {
	n := &Data{
		Settings:   d.Settings,
		Accounts:   append([]model.Account(nil), d.Accounts...),
		Categories: append([]model.Category(nil), d.Categories...),
		Labels:     append([]model.Label(nil), d.Labels...),
		Reminders:  append([]model.Reminder(nil), d.Reminders...),
		Txns:       append([]model.Transaction(nil), d.Txns...),
		src:        make(map[string]string, len(d.src)),
	}
	for k, v := range d.src {
		n.src[k] = v
	}
	n.reindex()
	return n
}

func (d *Data) sortTxns() {
	sort.SliceStable(d.Txns, func(i, j int) bool {
		a, b := d.Txns[i], d.Txns[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.Time != b.Time {
			return a.Time < b.Time
		}
		return a.ID < b.ID
	})
}

func (d *Data) reindex() {
	ix := index{acc: map[string]int{}, cat: map[string]int{}, lab: map[string]int{}, rem: map[string]int{}, tx: map[string]int{}, kids: map[string][]string{}}
	for i, a := range d.Accounts {
		ix.acc[a.ID] = i
	}
	for i, c := range d.Categories {
		ix.cat[c.ID] = i
	}
	for _, c := range d.Categories {
		if c.Parent != "" {
			ix.kids[c.Parent] = append(ix.kids[c.Parent], c.ID)
		}
	}
	for i, l := range d.Labels {
		ix.lab[l.ID] = i
	}
	for i, r := range d.Reminders {
		ix.rem[r.ID] = i
	}
	for i, t := range d.Txns {
		ix.tx[t.ID] = i
	}
	d.idx = ix
}

// Account returns the account with id.
func (d *Data) Account(id string) (model.Account, bool) {
	i, ok := d.idx.acc[id]
	if !ok {
		return model.Account{}, false
	}
	return d.Accounts[i], true
}

// Category returns the category with id.
func (d *Data) Category(id string) (model.Category, bool) {
	i, ok := d.idx.cat[id]
	if !ok {
		return model.Category{}, false
	}
	return d.Categories[i], true
}

// Children returns the child category ids of parent.
func (d *Data) Children(parent string) []string { return d.idx.kids[parent] }

// Label returns the label with id.
func (d *Data) Label(id string) (model.Label, bool) {
	i, ok := d.idx.lab[id]
	if !ok {
		return model.Label{}, false
	}
	return d.Labels[i], true
}

// Reminder returns the reminder with id.
func (d *Data) Reminder(id string) (model.Reminder, bool) {
	i, ok := d.idx.rem[id]
	if !ok {
		return model.Reminder{}, false
	}
	return d.Reminders[i], true
}

// Tx returns the transaction with id.
func (d *Data) Tx(id string) (model.Transaction, bool) {
	i, ok := d.idx.tx[id]
	if !ok {
		return model.Transaction{}, false
	}
	return d.Txns[i], true
}

// SetTx inserts or replaces a transaction and marks the month files to rewrite.
func (d *Data) SetTx(t model.Transaction, ch *Change) {
	if i, ok := d.idx.tx[t.ID]; ok {
		ch.Month(d.MonthOf(d.Txns[i]))
		d.Txns[i] = t
	} else {
		d.Txns = append(d.Txns, t)
		d.idx.tx[t.ID] = len(d.Txns) - 1
	}
	if dd, err := dates.Parse(t.Date); err == nil {
		d.src[t.ID] = dd.MonthKey()
	}
	ch.Month(d.MonthOf(t))
}

// DeleteTx removes a transaction.
func (d *Data) DeleteTx(id string, ch *Change) bool {
	i, ok := d.idx.tx[id]
	if !ok {
		return false
	}
	ch.Month(d.MonthOf(d.Txns[i]))
	d.Txns = append(d.Txns[:i:i], d.Txns[i+1:]...)
	delete(d.src, id)
	d.reindex()
	return true
}

// DryRun runs fn on a throw-away copy of the data (nothing is saved) — used for
// import previews.
func (b *Book) DryRun(fn func(d *Data, c *Change) error) error {
	b.mu.RLock()
	nd := b.data.clone()
	b.mu.RUnlock()
	var ch Change
	return fn(nd, &ch)
}

// Reindex rebuilds lookups after records were appended directly (importer, seeder).
func (d *Data) Reindex() { d.reindex() }
