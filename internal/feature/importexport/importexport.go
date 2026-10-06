// Package importexport is CSV export of any filtered transaction list and CSV import
// (with a dry-run preview). The export format imports back unchanged.
//
// Columns: id, date, time, type, title, amount, account, to_account, category,
// labels, status, notes, splits.
//   - account / to_account / category are names ("Food > Groceries" for a child);
//     ids are accepted on import too.
//   - labels are separated by "|".
//   - splits holds split lines "Category=amount|Category=amount"; for free-form
//     journal entries it holds every posting "account:Name=-100|category:Name=100".
//   - Without a type column, a negative amount is an expense and a positive one income.
package importexport

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/dates"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/model"
	"gitlab.com/niharokz/arthik/internal/money"
	"gitlab.com/niharokz/arthik/internal/query"
)

// Columns is the export header.
var Columns = []string{"id", "date", "time", "type", "title", "amount", "account", "to_account", "category", "labels", "status", "notes", "splits"}

// Register adds the routes.
func Register(rt *httpx.Router) {
	rt.Handle("GET /api/export.csv", export)
	rt.Handle("POST /api/import", importCSV)
}

func catName(d *book.Data, id string) string {
	c, ok := d.Category(id)
	if !ok {
		return id
	}
	if p, ok := d.Category(c.Parent); ok {
		return p.Name + " > " + c.Name
	}
	return c.Name
}

func accName(d *book.Data, id string) string {
	if a, ok := d.Account(id); ok {
		return a.Name
	}
	return id
}

func export(c *httpx.Ctx) error {
	f := query.Parse(c.R.URL.Query())
	var rows [][]string
	err := c.Book.View(func(d *book.Data, comp *book.Computed) error {
		txs := query.Compile(f, d, comp).Select()
		for i := len(txs) - 1; i >= 0; i-- { // oldest first
			t := txs[i]
			e := book.EntryOf(t)
			var labels []string
			for _, l := range t.Labels {
				if lb, ok := d.Label(l); ok {
					labels = append(labels, lb.Name)
				} else {
					labels = append(labels, l)
				}
			}
			var splits []string
			for _, s := range e.Splits {
				splits = append(splits, catName(d, s.Category)+"="+s.Amount.String())
			}
			if e.Type == model.TxJournal {
				for _, p := range t.Postings {
					if p.Account != "" {
						splits = append(splits, "account:"+accName(d, p.Account)+"="+p.Amount.String())
					} else {
						splits = append(splits, "category:"+catName(d, p.Category)+"="+p.Amount.String())
					}
				}
			}
			cat := ""
			if e.Category != "" {
				cat = catName(d, e.Category)
			}
			to := ""
			if e.ToAccount != "" {
				to = accName(d, e.ToAccount)
			}
			acc := ""
			if e.Account != "" {
				acc = accName(d, e.Account)
			}
			rows = append(rows, []string{t.ID, t.Date, t.Time, e.Type, t.Title, e.Amount.String(), acc, to, cat,
				strings.Join(labels, "|"), e.Status, t.Notes, strings.Join(splits, "|")})
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.W.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.W.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="arthik-%s.csv"`, dates.Today()))
	w := csv.NewWriter(c.W)
	_ = w.Write(Columns)
	_ = w.WriteAll(rows)
	w.Flush()
	return nil
}

// Report is the import result (also the dry-run preview).
type Report struct {
	Rows       int       `json:"rows"`
	Imported   int       `json:"imported"`
	Skipped    int       `json:"skipped"` // already present (same id)
	Errors     []LineErr `json:"errors"`
	Accounts   []string  `json:"new_accounts"`
	Categories []string  `json:"new_categories"`
	Labels     []string  `json:"new_labels"`
	DryRun     bool      `json:"dry_run"`
}

// LineErr is a problem with one CSV line.
type LineErr struct {
	Line int    `json:"line"`
	Msg  string `json:"msg"`
}

// importCSV reads a CSV body. ?dry_run=1 previews; ?create_missing=1 creates unknown
// accounts (as bank accounts) and categories instead of rejecting the row.
// Rows with errors are skipped; the rest are imported in one go.
func importCSV(c *httpx.Ctx) error {
	body, err := io.ReadAll(http.MaxBytesReader(c.W, c.R.Body, 20<<20))
	if err != nil {
		return fmt.Errorf("file too large (20 MB max)")
	}
	dry := c.Q("dry_run") == "1"
	create := c.Q("create_missing") == "1"
	records, err := readCSV(string(body))
	if err != nil {
		return err
	}
	if len(records) < 2 {
		return fmt.Errorf("the file has no data rows")
	}
	rep := &Report{DryRun: dry, Errors: []LineErr{}}
	run := func(d *book.Data, ch *book.Change) error {
		*rep = Report{DryRun: dry, Errors: []LineErr{}}
		apply(d, ch, records, create, rep)
		return nil
	}
	if dry {
		err = c.Book.DryRun(run)
	} else {
		err = c.Book.Mutate(run)
	}
	if err != nil {
		return err
	}
	return c.OK(rep)
}

func readCSV(s string) ([][]string, error) {
	s = strings.TrimPrefix(s, "\uFEFF") // Excel byte-order mark
	r := csv.NewReader(strings.NewReader(s))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV: %v", err)
	}
	return recs, nil
}

// resolver finds (or creates) accounts and categories by name or id.
type resolver struct {
	d      *book.Data
	ch     *book.Change
	create bool
	rep    *Report
}

func (r *resolver) account(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("missing account")
	}
	if _, ok := r.d.Account(name); ok {
		return name, nil
	}
	for _, a := range r.d.Accounts {
		if strings.EqualFold(a.Name, name) {
			return a.ID, nil
		}
	}
	if !r.create {
		return "", fmt.Errorf("unknown account %q", name)
	}
	id := r.d.UniqueAccountID(name)
	r.d.Accounts = append(r.d.Accounts, model.Account{ID: id, Name: name, Type: "bank"})
	r.d.Reindex()
	r.ch.Accounts = true
	r.rep.Accounts = append(r.rep.Accounts, name)
	return id, nil
}

func (r *resolver) category(name, kind string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("missing category")
	}
	if _, ok := r.d.Category(name); ok {
		return name, nil
	}
	parentName, childName, isChild := strings.Cut(name, ">")
	parentName, childName = strings.TrimSpace(parentName), strings.TrimSpace(childName)
	find := func(n, parent string) string {
		for _, c := range r.d.Categories {
			if c.Parent == parent && strings.EqualFold(c.Name, n) {
				return c.ID
			}
		}
		return ""
	}
	mk := func(n, parent string) string {
		id := r.d.UniqueCategoryID(n, parent)
		r.d.Categories = append(r.d.Categories, model.Category{ID: id, Name: n, Kind: kind, Parent: parent})
		r.d.Reindex()
		r.ch.Categories = true
		label := n
		if parent != "" {
			label = parentName + " > " + n
		}
		r.rep.Categories = append(r.rep.Categories, label)
		return id
	}
	if !isChild {
		if id := find(parentName, ""); id != "" {
			return id, nil
		}
		// a bare child name is fine when it is unique
		var hits []string
		for _, c := range r.d.Categories {
			if strings.EqualFold(c.Name, parentName) {
				hits = append(hits, c.ID)
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if !r.create {
			return "", fmt.Errorf("unknown category %q", name)
		}
		return mk(parentName, ""), nil
	}
	pid := find(parentName, "")
	if pid == "" {
		if !r.create {
			return "", fmt.Errorf("unknown category %q", parentName)
		}
		pid = mk(parentName, "")
	}
	if id := find(childName, pid); id != "" {
		return id, nil
	}
	if !r.create {
		return "", fmt.Errorf("unknown category %q", name)
	}
	return mk(childName, pid), nil
}

func apply(d *book.Data, ch *book.Change, recs [][]string, create bool, rep *Report) {
	col := map[string]int{}
	for i, h := range recs[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(row []string, name string) string {
		if i, ok := col[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	for _, need := range []string{"date", "amount"} {
		if _, ok := col[need]; !ok {
			rep.Errors = append(rep.Errors, LineErr{1, "missing column " + need})
			return
		}
	}
	labelsBefore := len(d.Labels)
	res := &resolver{d: d, ch: ch, create: create, rep: rep}
	for n, row := range recs[1:] {
		line := n + 2
		if len(row) == 1 && strings.TrimSpace(row[0]) == "" {
			continue
		}
		rep.Rows++
		fail := func(err error) { rep.Errors = append(rep.Errors, LineErr{line, err.Error()}) }
		id := get(row, "id")
		if id != "" {
			if _, exists := d.Tx(id); exists {
				rep.Skipped++
				continue
			}
			if !book.ValidID(id) {
				fail(fmt.Errorf("invalid id %q", id))
				continue
			}
		}
		date, err := dates.Parse(get(row, "date"))
		if err != nil {
			fail(err)
			continue
		}
		amt, err := money.Parse(get(row, "amount"))
		if err != nil {
			fail(err)
			continue
		}
		typ := strings.ToLower(get(row, "type"))
		if typ == "" {
			typ = model.TxIncome
			if amt < 0 {
				typ, amt = model.TxExpense, -amt
			}
		}
		e := book.Entry{ID: id, Type: typ, Date: date.String(), Time: get(row, "time"), Title: get(row, "title"),
			Amount: amt, Status: strings.ToLower(get(row, "status")), Notes: get(row, "notes")}
		if e.Title == "" {
			e.Title = "(no title)"
		}
		if l := get(row, "labels"); l != "" {
			e.Labels = strings.Split(l, "|")
		}
		kind := model.KindExpense
		if typ == model.TxIncome {
			kind = model.KindIncome
		}
		var rerr error
		switch typ {
		case model.TxExpense, model.TxIncome:
			if e.Account, rerr = res.account(get(row, "account")); rerr != nil {
				break
			}
			if sp := get(row, "splits"); sp != "" {
				for _, part := range strings.Split(sp, "|") {
					name, a, ok := strings.Cut(part, "=")
					if !ok {
						rerr = fmt.Errorf("bad split %q (want Category=amount)", part)
						break
					}
					v, err := money.Parse(a)
					if err != nil {
						rerr = err
						break
					}
					cid, err := res.category(name, kind)
					if err != nil {
						rerr = err
						break
					}
					e.Splits = append(e.Splits, book.Split{Category: cid, Amount: v})
				}
			} else {
				cn := get(row, "category")
				if cn == "" {
					cn = "Uncategorized"
				}
				e.Category, rerr = res.category(cn, kind)
			}
		case model.TxTransfer:
			if e.Account, rerr = res.account(get(row, "account")); rerr == nil {
				e.ToAccount, rerr = res.account(get(row, "to_account"))
			}
		case model.TxJournal:
			for _, part := range strings.Split(get(row, "splits"), "|") {
				ref, a, ok := strings.Cut(part, "=")
				kindRef, name, ok2 := strings.Cut(ref, ":")
				if !ok || !ok2 {
					rerr = fmt.Errorf("bad posting %q (want account:Name=amount or category:Name=amount)", part)
					break
				}
				v, err := money.Parse(a)
				if err != nil {
					rerr = err
					break
				}
				p := model.Posting{Amount: v}
				if kindRef == "account" {
					p.Account, rerr = res.account(name)
				} else {
					p.Category, rerr = res.category(name, model.KindExpense)
				}
				if rerr != nil {
					break
				}
				e.Postings = append(e.Postings, p)
			}
		default:
			rerr = fmt.Errorf("unknown type %q", typ)
		}
		if rerr != nil {
			fail(rerr)
			continue
		}
		if e.ID == "" {
			e.ID = d.NewTxID(e.Date)
		}
		t, err := d.BuildTx(e)
		if err != nil {
			fail(err)
			continue
		}
		t.Labels = d.EnsureLabels(t.Labels, ch)
		d.SetTx(t, ch)
		rep.Imported++
	}
	for _, l := range d.Labels[labelsBefore:] {
		rep.Labels = append(rep.Labels, l.Name)
	}
	sort.Strings(rep.Labels)
}
