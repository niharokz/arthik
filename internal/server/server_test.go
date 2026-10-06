package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/niharokz/arthik/internal/auth"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/demo"
	"gitlab.com/niharokz/arthik/internal/httpx"
)

type client struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func (c *client) do(method, path, body string) (int, map[string]any) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.srv.URL+path, strings.NewReader(body))
	req.Header.Set("X-Arthik", "1")
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{"_raw": string(b)}
	}
	return res.StatusCode, out
}

func setup(t *testing.T) (*client, *client) {
	dir := t.TempDir()
	own, err := book.Open(dir, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.Seed(dir + "/demouser"); err != nil {
		t.Fatal(err)
	}
	db, _ := book.Open(dir+"/demouser", "demo", true)
	if p := db.Problems(); len(p) > 0 {
		t.Fatalf("demo data has problems: %+v", p)
	}
	a := auth.New("secret",
		&auth.Account{User: httpx.User{Name: "owner", Book: own}, Password: "pw-owner"},
		&auth.Account{User: httpx.User{Name: "demouser", ReadOnly: true, Book: db}, Password: "demo", ShowHint: true})
	srv := httptest.NewServer(New(a))
	t.Cleanup(srv.Close)
	mk := func() *client {
		j, _ := cookiejar.New(nil)
		return &client{t: t, srv: srv, c: &http.Client{Jar: j}}
	}
	o, d := mk(), mk()
	if code, _ := o.do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 200 {
		t.Fatalf("owner login %d", code)
	}
	if code, _ := d.do("POST", "/api/login", `{"user":"demouser","password":"demo"}`); code != 200 {
		t.Fatalf("demo login %d", code)
	}
	return o, d
}

func TestAPI(t *testing.T) {
	o, d := setup(t)

	// unauthenticated + bad password
	anon := &client{t: t, srv: o.srv, c: http.DefaultClient}
	if code, _ := anon.do("GET", "/api/state", ""); code != 401 {
		t.Errorf("anon state = %d", code)
	}
	if code, _ := anon.do("POST", "/api/login", `{"user":"owner","password":"nope"}`); code != 401 {
		t.Errorf("bad password = %d", code)
	}

	// demo is read-only but can read
	if code, _ := d.do("GET", "/api/state", ""); code != 200 {
		t.Errorf("demo state = %d", code)
	}
	if code, _ := d.do("POST", "/api/accounts", `{"name":"x","type":"bank"}`); code != 403 {
		t.Errorf("demo write = %d", code)
	}

	// owner flow
	mustOK := func(method, path, body string) map[string]any {
		t.Helper()
		code, out := o.do(method, path, body)
		if code != 200 {
			t.Fatalf("%s %s = %d %v", method, path, code, out)
		}
		return out
	}
	mustOK("POST", "/api/accounts", `{"name":"Bank","type":"bank","opening_balance":"1000"}`)
	mustOK("POST", "/api/categories", `{"name":"Food","kind":"expense","budget":"3000","budget_period":"monthly"}`)
	body := `{"id":"c-20261006-aa","type":"expense","date":"2026-10-01","title":"Tea","amount":"40","account":"bank","category":"food"}`
	mustOK("POST", "/api/transactions", body)
	if out := mustOK("POST", "/api/transactions", body); out["duplicate"] != true { // offline replay
		t.Errorf("replay not detected: %v", out)
	}
	list := mustOK("GET", "/api/transactions?q=tea", "")
	if list["total"].(float64) != 1 {
		t.Errorf("search total = %v", list["total"])
	}
	// CSV export → import into the same book: everything is skipped as already present.
	req, _ := http.NewRequest("GET", o.srv.URL+"/api/export.csv", nil)
	res, _ := o.c.Do(req)
	csv, _ := io.ReadAll(res.Body)
	res.Body.Close()
	req, _ = http.NewRequest("POST", o.srv.URL+"/api/import", strings.NewReader(string(csv)))
	req.Header.Set("X-Arthik", "1")
	res, _ = o.c.Do(req)
	var rep map[string]any
	_ = json.NewDecoder(res.Body).Decode(&rep)
	res.Body.Close()
	if rep["skipped"].(float64) != 1 || rep["imported"].(float64) != 0 {
		t.Errorf("import report = %v", rep)
	}
	// weekly view of a monthly ₹3,000 budget
	b := mustOK("GET", "/api/budgets?period=weekly&date=2026-10-06", "")
	if got := b["total"].(map[string]any)["budget"]; got != "692.31" {
		t.Errorf("weekly budget = %v", got)
	}
	// CSRF header required
	req, _ = http.NewRequest("POST", o.srv.URL+"/api/labels", strings.NewReader(`{"name":"x"}`))
	res, _ = o.c.Do(req)
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Errorf("missing CSRF header = %d", res.StatusCode)
	}
	// every report answers
	for _, p := range []string{"cashflow", "categories", "networth", "balance-sheet", "labels", "accounts", "calendar", "trend?category=food"} {
		mustOK("GET", "/api/reports/"+p, "")
	}
	mustOK("GET", "/api/upcoming", "")
	mustOK("GET", "/api/period?period=quarterly", "")
	// pages
	for _, p := range []string{"/", "/app", "/sw.js", "/manifest.webmanifest", "/js/main.js"} {
		res, err := http.Get(o.srv.URL + p)
		if err != nil || res.StatusCode != 200 {
			t.Errorf("GET %s = %v %v", p, res.StatusCode, err)
		}
		res.Body.Close()
	}
}
