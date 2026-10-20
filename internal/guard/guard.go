// Package guard bans IP addresses after repeated failed sign-ins.
//
// Failures are counted in memory; a ban is written to <state dir>/bans.json so it
// survives restarts and lasts its full term. Only the server owner lifts a ban early:
// `arthik unban <ip|all>` (or editing / deleting the file). The running server
// re-reads the file when it changes.
package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileName is the ban list inside the state folder.
const FileName = "bans.json"

// failWindow is how long a failed sign-in counts towards a ban.
const failWindow = 24 * time.Hour

// Ban is one banned address.
type Ban struct {
	Until  time.Time `json:"until"`
	Since  time.Time `json:"since"`
	Reason string    `json:"reason"`
}

// Guard tracks failures and bans. The zero value is not usable; call New.
type Guard struct {
	mu     sync.Mutex
	path   string // "" = bans kept in memory only
	max    int
	term   time.Duration
	exempt []*net.IPNet
	fails  map[string][]time.Time
	bans   map[string]Ban
	seen   os.FileInfo // bans.json as last read (nil = none)
	now    func() time.Time
}

// Options configure a Guard.
type Options struct {
	Dir      string        // state folder for bans.json ("" = memory only)
	MaxFails int           // failures before a ban
	Term     time.Duration // ban length
	Exempt   []*net.IPNet  // never banned (e.g. your home network)
}

// New loads the ban list from opt.Dir.
func New(opt Options) *Guard {
	g := &Guard{max: max(1, opt.MaxFails), term: opt.Term, exempt: opt.Exempt,
		fails: map[string][]time.Time{}, bans: map[string]Ban{}, now: time.Now}
	if opt.Dir != "" {
		if err := os.MkdirAll(opt.Dir, 0o700); err != nil {
			log.Printf("WARN bans kept in memory only: %v", err)
		} else {
			g.path = filepath.Join(opt.Dir, FileName)
		}
	}
	g.mu.Lock()
	g.reload()
	g.mu.Unlock()
	return g
}

// Banned reports whether ip may not reach the server.
func (g *Guard) Banned(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.reload()
	b, ok := g.bans[ip]
	return ok && g.now().Before(b.Until)
}

// Fail records a failed sign-in; it returns true when this failure banned ip.
func (g *Guard) Fail(ip, reason string) bool {
	if g.isExempt(ip) {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	keep := g.fails[ip][:0]
	for _, t := range g.fails[ip] {
		if now.Sub(t) < failWindow {
			keep = append(keep, t)
		}
	}
	keep = append(keep, now)
	if len(keep) < g.max {
		g.fails[ip] = keep
		return false
	}
	delete(g.fails, ip)
	g.reload()
	g.bans[ip] = Ban{Since: now, Until: now.Add(g.term), Reason: reason}
	if err := g.save(); err != nil {
		log.Printf("WARN could not save the ban list: %v", err)
	}
	log.Printf("SECURITY banned %s until %s after %d failed sign-ins (%s)", ip, now.Add(g.term).Format(time.RFC3339), g.max, reason)
	return true
}

// Left is how many more failures ip may make before it is banned.
func (g *Guard) Left(ip string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return max(0, g.max-len(g.fails[ip]))
}

// Reset forgets ip's failures after a good sign-in.
func (g *Guard) Reset(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fails, ip)
}

// Middleware answers every request from a banned address with 403 (health checks pass).
func (g *Guard) Middleware(ipOf func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && g.Banned(ipOf(r)) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Guard) isExempt(ip string) bool {
	p := net.ParseIP(ip)
	for _, n := range g.exempt {
		if p != nil && n.Contains(p) {
			return true
		}
	}
	return false
}

// reload re-reads bans.json when it changed on disk and drops expired bans. Call with mu held.
func (g *Guard) reload() {
	if g.path == "" {
		return
	}
	st, err := os.Stat(g.path)
	if errors.Is(err, os.ErrNotExist) {
		if g.seen != nil { // deleted by hand: all bans lifted
			g.bans, g.seen = map[string]Ban{}, nil
		}
		return
	}
	if err != nil || unchanged(g.seen, st) {
		return
	}
	bans, err := Read(filepath.Dir(g.path))
	if err != nil {
		log.Printf("WARN %v — keeping the bans already loaded", err)
		return
	}
	g.bans, g.seen = bans, st
}

// unchanged compares file identity too: every write replaces the file (new inode), so
// a change is seen even when the clock is too coarse to move the modification time.
func unchanged(a, b os.FileInfo) bool {
	return a != nil && os.SameFile(a, b) && a.ModTime().Equal(b.ModTime()) && a.Size() == b.Size()
}

func (g *Guard) save() error {
	if g.path == "" {
		return nil
	}
	now := g.now()
	for ip, b := range g.bans {
		if !now.Before(b.Until) {
			delete(g.bans, ip)
		}
	}
	if err := Write(filepath.Dir(g.path), g.bans); err != nil {
		return err
	}
	if st, err := os.Stat(g.path); err == nil {
		g.seen = st
	}
	return nil
}

// Read loads the ban list in dir (empty when there is none).
func Read(dir string) (map[string]Ban, error) {
	bans := map[string]Ban{}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return bans, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return bans, nil
	}
	if err := json.Unmarshal(b, &bans); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return bans, nil
}

// Write saves the ban list in dir atomically (owner-only permissions).
func Write(dir string, bans map[string]Ban) error {
	b, err := json.MarshalIndent(bans, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bans-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, FileName))
}

// Unban lifts the ban on ip ("all" lifts every ban); it returns how many were lifted.
func Unban(dir, ip string) (int, error) {
	bans, err := Read(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	if ip == "all" {
		n, bans = len(bans), map[string]Ban{}
	} else if _, ok := bans[ip]; ok {
		delete(bans, ip)
		n = 1
	}
	if n == 0 {
		return 0, nil
	}
	return n, Write(dir, bans)
}

// List returns the active bans, soonest to end first.
func List(dir string, now time.Time) ([]string, map[string]Ban, error) {
	bans, err := Read(dir)
	if err != nil {
		return nil, nil, err
	}
	var ips []string
	for ip, b := range bans {
		if now.Before(b.Until) {
			ips = append(ips, ip)
		}
	}
	sort.Slice(ips, func(i, j int) bool { return bans[ips[i]].Until.Before(bans[ips[j]].Until) })
	return ips, bans, nil
}

// ParseNets reads a comma-separated list of CIDRs or single addresses.
func ParseNets(s string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.Contains(f, "/") {
			if ip := net.ParseIP(f); ip != nil {
				bits := 128
				if ip.To4() != nil {
					bits = 32
				}
				f = fmt.Sprintf("%s/%d", f, bits)
			}
		}
		_, n, err := net.ParseCIDR(f)
		if err != nil {
			return nil, fmt.Errorf("not an address or CIDR: %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}

// ClientIP finds the real client address. Forwarding headers (CF-Connecting-IP,
// X-Forwarded-For) are only believed when the direct peer is a trusted proxy, so a
// client can't pick its own address to dodge or frame a ban.
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if !inNets(peer, trusted) {
		return peer
	}
	if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); net.ParseIP(v) != nil {
		return v
	}
	// The right-most address that is not one of our proxies is the client.
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		h := strings.TrimSpace(hops[i])
		if net.ParseIP(h) == nil {
			continue
		}
		if !inNets(h, trusted) {
			return h
		}
		peer = h
	}
	return peer
}

func inNets(ip string, nets []*net.IPNet) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(p) {
			return true
		}
	}
	return false
}
