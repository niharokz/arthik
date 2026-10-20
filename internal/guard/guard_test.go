package guard

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestBanAfterMaxFailsSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	g := New(Options{Dir: dir, MaxFails: 5, Term: 30 * 24 * time.Hour})
	for i := 1; i <= 4; i++ {
		if g.Fail("203.0.113.9", "password") {
			t.Fatalf("banned after %d failures", i)
		}
	}
	if g.Left("203.0.113.9") != 1 || g.Banned("203.0.113.9") {
		t.Fatalf("left %d banned %v", g.Left("203.0.113.9"), g.Banned("203.0.113.9"))
	}
	if !g.Fail("203.0.113.9", "password") || !g.Banned("203.0.113.9") {
		t.Fatal("fifth failure must ban")
	}
	if g.Banned("203.0.113.10") {
		t.Fatal("another address is banned")
	}
	if !New(Options{Dir: dir, MaxFails: 5, Term: time.Hour}).Banned("203.0.113.9") {
		t.Fatal("ban lost on restart")
	}
	// Lifted from the command line while the server runs.
	if n, err := Unban(dir, "203.0.113.9"); err != nil || n != 1 {
		t.Fatalf("unban %d %v", n, err)
	}
	if g.Banned("203.0.113.9") {
		t.Fatal("running guard did not notice the unban")
	}
}

func TestBanEndsAndResetAndExempt(t *testing.T) {
	now := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	exempt, _ := ParseNets("192.168.1.0/24")
	g := New(Options{Dir: t.TempDir(), MaxFails: 2, Term: 48 * time.Hour, Exempt: exempt})
	g.now = func() time.Time { return now }
	g.Fail("198.51.100.1", "password")
	g.Reset("198.51.100.1") // a good sign-in clears the count
	if g.Fail("198.51.100.1", "password") {
		t.Fatal("count not reset")
	}
	g.Fail("198.51.100.1", "password")
	if !g.Banned("198.51.100.1") {
		t.Fatal("not banned")
	}
	now = now.Add(49 * time.Hour)
	if g.Banned("198.51.100.1") {
		t.Fatal("ban outlived its term")
	}
	for i := 0; i < 5; i++ {
		g.Fail("192.168.1.20", "password")
	}
	if g.Banned("192.168.1.20") {
		t.Fatal("exempt address banned")
	}
}

func TestOldFailuresExpire(t *testing.T) {
	now := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	g := New(Options{MaxFails: 2, Term: time.Hour})
	g.now = func() time.Time { return now }
	g.Fail("198.51.100.2", "password")
	now = now.Add(25 * time.Hour)
	if g.Fail("198.51.100.2", "password") {
		t.Fatal("a failure from yesterday still counted")
	}
}

func TestClientIP(t *testing.T) {
	trusted, err := ParseNets("172.16.0.0/12, 127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	req := func(peer string, h map[string]string) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = peer + ":5555"
		for k, v := range h {
			r.Header.Set(k, v)
		}
		return ClientIP(r, trusted)
	}
	cases := []struct {
		name, peer string
		h          map[string]string
		want       string
	}{
		{"direct client can't pick its address", "203.0.113.5", map[string]string{"X-Forwarded-For": "1.1.1.1", "CF-Connecting-IP": "1.1.1.1"}, "203.0.113.5"},
		{"cloudflare tunnel via caddy", "172.18.0.4", map[string]string{"CF-Connecting-IP": "198.51.100.7", "X-Forwarded-For": "172.18.0.9"}, "198.51.100.7"},
		{"lan via caddy", "172.18.0.4", map[string]string{"X-Forwarded-For": "192.168.1.20"}, "192.168.1.20"},
		{"forged left entry ignored", "172.18.0.4", map[string]string{"X-Forwarded-For": "1.1.1.1, 203.0.113.8"}, "203.0.113.8"},
		{"proxy without headers", "172.18.0.4", nil, "172.18.0.4"},
	}
	for _, c := range cases {
		if got := req(c.peer, c.h); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	if _, err := ParseNets("not-a-net"); err == nil {
		t.Error("bad CIDR accepted")
	}
}
