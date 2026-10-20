package server

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/niharokz/arthik/internal/auth"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/guard"
	"gitlab.com/niharokz/arthik/internal/httpx"
)

const deviceKey = "device-key-0123456789abcdef"

func secureServer(t *testing.T, ttl time.Duration, key, stateDir string) *httptest.Server {
	t.Helper()
	own, err := book.Open(t.TempDir(), "main", false)
	if err != nil {
		t.Fatal(err)
	}
	trusted, _ := guard.ParseNets("127.0.0.0/8")
	g := guard.New(guard.Options{Dir: stateDir, MaxFails: 5, Term: 30 * 24 * time.Hour})
	ip := func(r *http.Request) string { return guard.ClientIP(r, trusted) }
	a := auth.New("secret", auth.Options{SessionTTL: ttl, DeviceKey: key, Guard: g, ClientIP: ip},
		&auth.Account{User: httpx.User{Name: "owner", Book: own}, Password: "pw-owner"})
	srv := httptest.NewServer(New(a, Options{Guard: g, ClientIP: ip}))
	t.Cleanup(srv.Close)
	return srv
}

// from makes a client whose requests come (through the trusted proxy) from addr.
func from(t *testing.T, srv *httptest.Server, addr string) *client {
	j, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, c: &http.Client{Jar: j, Transport: forwardFor{addr}}}
}

type forwardFor struct{ addr string }

func (f forwardFor) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-Forwarded-For", f.addr)
	return http.DefaultTransport.RoundTrip(r)
}

func TestFiveFailedSignInsBanTheAddress(t *testing.T) {
	state := t.TempDir()
	srv := secureServer(t, 12*time.Hour, "", state)
	bad, good := from(t, srv, "203.0.113.9"), from(t, srv, "198.51.100.4")
	for i := 1; i <= 4; i++ {
		if code, _ := bad.do("POST", "/api/login", `{"user":"owner","password":"guess"}`); code != 401 {
			t.Fatalf("try %d = %d", i, code)
		}
	}
	if code, _ := bad.do("POST", "/api/login", `{"user":"owner","password":"guess"}`); code != 403 {
		t.Fatalf("fifth failure = %d, want 403", code)
	}
	// Banned: even the right password and every page are refused.
	if code, _ := bad.do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 403 {
		t.Fatalf("banned login = %d", code)
	}
	if code, _ := bad.do("GET", "/app", ""); code != 403 {
		t.Fatalf("banned page = %d", code)
	}
	if code, _ := bad.do("GET", "/healthz", ""); code != 200 {
		t.Fatalf("health check = %d", code)
	}
	if code, _ := good.do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 200 {
		t.Fatalf("other address = %d", code)
	}
	// Still banned after a restart; lifted only by `arthik unban`.
	srv2 := secureServer(t, 12*time.Hour, "", state)
	if code, _ := from(t, srv2, "203.0.113.9").do("GET", "/app", ""); code != 403 {
		t.Fatalf("after restart = %d", code)
	}
	if _, err := guard.Unban(state, "203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	if code, _ := from(t, srv2, "203.0.113.9").do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 200 {
		t.Fatalf("after unban = %d", code)
	}
}

func TestDeviceKeyKeepsTheDeviceSignedIn(t *testing.T) {
	srv := secureServer(t, time.Second, deviceKey, t.TempDir())
	plain, keyed, late := from(t, srv, "198.51.100.1"), from(t, srv, "198.51.100.2"), from(t, srv, "198.51.100.3")

	if code, _ := plain.do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 200 {
		t.Fatal("plain login")
	}
	if code, out := keyed.do("POST", "/api/login", `{"user":"owner","password":"pw-owner","key":"`+deviceKey+`"}`); code != 200 || out["remembered"] != true {
		t.Fatalf("keyed login %d %v", code, out)
	}
	// A device that signed in without the key adds it in Settings.
	if code, _ := late.do("POST", "/api/login", `{"user":"owner","password":"pw-owner"}`); code != 200 {
		t.Fatal("late login")
	}
	if code, _ := late.do("POST", "/api/device-key", `{"key":"wrong-key-wrong-key-wrong"}`); code != 401 {
		t.Fatalf("wrong key = %d", code)
	}
	if code, _ := late.do("POST", "/api/device-key", `{"key":"`+deviceKey+`"}`); code != 200 {
		t.Fatalf("right key = %d", code)
	}
	if _, me := plain.do("GET", "/api/me", ""); me["remembered"] != false || me["expires"] == nil {
		t.Fatalf("plain me %v", me)
	}

	time.Sleep(2100 * time.Millisecond) // the 1 s sign-in runs out
	if code, _ := plain.do("GET", "/api/state", ""); code != 401 {
		t.Fatalf("plain session still valid: %d", code)
	}
	for name, c := range map[string]*client{"keyed": keyed, "late": late} {
		if code, _ := c.do("GET", "/api/state", ""); code != 200 {
			t.Fatalf("%s session expired: %d", name, code)
		}
	}
	// Sign-out ends a remembered session.
	keyed.do("POST", "/api/logout", `{}`)
	if code, _ := keyed.do("GET", "/api/state", ""); code != 401 {
		t.Fatalf("after logout = %d", code)
	}
	// A wrong key at sign-in is a failed sign-in, not a normal one.
	if code, _ := from(t, srv, "198.51.100.9").do("POST", "/api/login", `{"user":"owner","password":"pw-owner","key":"nope-nope-nope-nope-nope-1"}`); code != 401 {
		t.Fatalf("wrong key at login = %d", code)
	}
}

func TestChangedDeviceKeyEndsRememberedSessions(t *testing.T) {
	dir := t.TempDir()
	own, _ := book.Open(dir, "main", false)
	acct := &auth.Account{User: httpx.User{Name: "owner", Book: own}, Password: "pw-owner"}
	old := httptest.NewServer(New(auth.New("secret", auth.Options{DeviceKey: deviceKey}, acct), Options{}))
	defer old.Close()
	j, _ := cookiejar.New(nil)
	c := &client{t: t, srv: old, c: &http.Client{Jar: j}}
	if code, _ := c.do("POST", "/api/login", `{"user":"owner","password":"pw-owner","key":"`+deviceKey+`"}`); code != 200 {
		t.Fatal("login")
	}
	// Same secret, new key in .env: the old cookie no longer works.
	c.srv = httptest.NewServer(New(auth.New("secret", auth.Options{DeviceKey: "a-brand-new-device-key-0001"}, acct), Options{}))
	defer c.srv.Close()
	u, _ := http.NewRequest("GET", old.URL, nil)
	nu, _ := http.NewRequest("GET", c.srv.URL, nil)
	j.SetCookies(nu.URL, j.Cookies(u.URL))
	if code, _ := c.do("GET", "/api/state", ""); code != 401 {
		t.Fatalf("old key still signs in: %d", code)
	}
}
