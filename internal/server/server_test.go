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
	srv := httptest.NewServer(New(a, Options{}))
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

func TestLocalMode(t *testing.T) {
	dir := t.TempDir()
	own, _ := book.OpenWith(dir+"/mine", book.Options{Name: "phone", Replica: true})
	if err := demo.Seed(dir + "/demo"); err != nil {
		t.Fatal(err)
	}
	db, _ := book.Open(dir+"/demo", "demo", true)
	const token = "0123456789abcdef0123"
	a := auth.New("",
		&auth.Account{User: httpx.User{Name: "me", Book: own}},
		&auth.Account{User: httpx.User{Name: "demouser", ReadOnly: true, Book: db}})
	a.EnableLocal(token)
	srv := httptest.NewServer(New(a, Options{Local: true}))
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	noRedirect := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	c := &client{t: t, srv: srv, c: noRedirect}

	get := func(path string) *http.Response {
		res, err := noRedirect.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	if res := get("/"); res.StatusCode != 302 || res.Header.Get("Location") != "/app" {
		t.Errorf("/ in local mode = %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if res := get("/local/enter?token=wrong&user=me"); res.StatusCode != 403 {
		t.Errorf("wrong token = %d", res.StatusCode)
	}
	if code, _ := c.do("GET", "/api/state", ""); code != 401 {
		t.Errorf("state before entering = %d", code)
	}
	if code, out := c.do("GET", "/api/me", ""); code != 200 || out["local"] != true || out["demo"] != nil {
		t.Errorf("me = %d %v", code, out)
	}
	if code, _ := c.do("POST", "/api/login", `{"user":"me","password":""}`); code != 401 {
		t.Errorf("password login must be impossible in local mode, got %d", code)
	}
	if res := get("/local/enter?token=" + token + "&user=nobody"); res.StatusCode != 404 {
		t.Errorf("unknown user = %d", res.StatusCode)
	}
	if res := get("/local/enter?token=" + token + "&user=me"); res.StatusCode != 303 || res.Header.Get("Location") != "/app#/home" {
		t.Fatalf("enter = %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if code, out := c.do("GET", "/api/state", ""); code != 200 || out["read_only"] != false {
		t.Fatalf("state as me = %d %v", code, out)
	}
	if code, out := c.do("POST", "/api/accounts", `{"name":"Wallet","type":"wallet"}`); code != 200 {
		t.Fatalf("phone write = %d %v", code, out)
	}
	get("/local/enter?token=" + token + "&user=demouser")
	if code, out := c.do("GET", "/api/state", ""); code != 200 || out["read_only"] != true {
		t.Fatalf("state as demo = %d %v", code, out)
	}
	if code, _ := c.do("POST", "/api/accounts", `{"name":"x","type":"bank"}`); code != 403 {
		t.Errorf("demo write = %d", code)
	}
}

// Remembered transactions: the add form's autofill list.
func TestSuggest(t *testing.T) {
	o, _ := setup(t)
	for _, b := range []string{
		`{"name":"Bank","type":"bank","opening_balance":"1000"}`,
		`{"name":"Wallet","type":"cash"}`,
	} {
		if code, out := o.do("POST", "/api/accounts", b); code != 200 {
			t.Fatalf("account %d %v", code, out)
		}
	}
	o.do("POST", "/api/categories", `{"name":"Food","kind":"expense"}`)
	o.do("POST", "/api/labels", `{"name":"home"}`)
	for _, b := range []string{
		`{"type":"expense","date":"2026-10-01","title":"Milk","amount":"30","account":"bank","category":"food"}`,
		`{"type":"expense","date":"2026-10-03","title":"Milk","amount":"32","account":"wallet","category":"food","labels":["home"],"notes":"1 litre"}`,
		`{"type":"expense","date":"2026-10-02","title":"Almond milk","amount":"180","account":"bank","category":"food"}`,
		`{"type":"transfer","date":"2026-10-02","title":"Cash withdrawal","amount":"500","account":"bank","to_account":"wallet"}`,
	} {
		if code, out := o.do("POST", "/api/transactions", b); code != 200 {
			t.Fatalf("txn %d %v", code, out)
		}
	}
	list := func(q string) []map[string]any {
		t.Helper()
		req, _ := http.NewRequest("GET", o.srv.URL+"/api/suggest?"+q, nil)
		res, err := o.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out []map[string]any
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	got := list("q=mil")
	if len(got) != 2 || got[0]["title"] != "Milk" || got[1]["title"] != "Almond milk" {
		t.Fatalf("q=mil order = %v", got)
	}
	m := got[0] // the latest Milk wins: wallet, ₹32, label, notes; used twice
	if m["account"] != "wallet" || m["amount"] != "32.00" || m["notes"] != "1 litre" || m["uses"].(float64) != 2 {
		t.Errorf("milk = %v", m)
	}
	if l, _ := m["labels"].([]any); len(l) != 1 || l[0] != "home" {
		t.Errorf("milk labels = %v", m["labels"])
	}
	if got := list("q=cash"); len(got) != 1 || got[0]["type"] != "transfer" || got[0]["to_account"] != "wallet" {
		t.Errorf("transfer = %v", got)
	}
	if got := list("q=zzz"); len(got) != 0 {
		t.Errorf("no match = %v", got)
	}
	if got := list("limit=1"); len(got) != 1 {
		t.Errorf("limit = %d", len(got))
	}
}

// Menu order is saved in settings.md: unknown keys refused, repeats dropped, missing ones appended.
func TestNavOrder(t *testing.T) {
	o, d := setup(t)
	_, st := o.do("GET", "/api/state", "")
	s := st["settings"].(map[string]any)
	s["nav_order"] = []string{"home", "bogus"}
	b, _ := json.Marshal(s)
	if code, _ := o.do("PUT", "/api/settings", string(b)); code != 400 {
		t.Errorf("unknown item = %d", code)
	}
	s["nav_order"] = []string{"home", "tx", "calendar", "reports", "accounts", "tx"}
	b, _ = json.Marshal(s)
	if code, out := o.do("PUT", "/api/settings", string(b)); code != 200 {
		t.Fatalf("put %d %v", code, out)
	}
	_, st = o.do("GET", "/api/state", "")
	got, _ := json.Marshal(st["settings"].(map[string]any)["nav_order"])
	want := `["home","tx","calendar","reports","accounts","inbox","categories","budgets","labels"]`
	if string(got) != want {
		t.Errorf("nav_order = %s", got)
	}
	// a settings save without nav_order keeps the saved order
	delete(s, "nav_order")
	b, _ = json.Marshal(s)
	o.do("PUT", "/api/settings", string(b))
	_, st = o.do("GET", "/api/state", "")
	if got, _ := json.Marshal(st["settings"].(map[string]any)["nav_order"]); string(got) != want {
		t.Errorf("order lost: %s", got)
	}
	if code, _ := d.do("PUT", "/api/settings", string(b)); code != 403 {
		t.Errorf("demo put = %d", code)
	}
}

// Inbox API: the demo shows drafts but cannot decide; the owner reviews.
func TestInboxAPI(t *testing.T) {
	o, d := setup(t)
	code, out := d.do("GET", "/api/inbox", "")
	if code != 200 || out["pending"].(float64) < 1 {
		t.Fatalf("demo inbox %d %v", code, out)
	}
	first := out["items"].([]any)[0].(map[string]any)["id"].(string)
	if code, _ := d.do("POST", "/api/inbox/"+first+"/accept", `{}`); code != 403 {
		t.Errorf("demo accept = %d", code)
	}
	if code, _ := o.do("GET", "/api/inbox?status=bogus", ""); code != 400 {
		t.Errorf("bad status = %d", code)
	}
	if code, _ := o.do("POST", "/api/inbox/nope/reject", `{}`); code != 404 {
		t.Errorf("missing draft = %d", code)
	}
	if code, out := o.do("GET", "/api/inbox/rules", ""); code != 200 || out["_raw"] == nil {
		t.Errorf("rules = %d %v", code, out)
	}
	// state carries the pending count for the menu badge
	_, st := d.do("GET", "/api/state", "")
	if st["inbox"].(map[string]any)["pending"].(float64) < 1 {
		t.Errorf("state inbox = %v", st["inbox"])
	}
}
