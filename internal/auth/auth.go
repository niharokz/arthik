// Package auth handles Arthik's two users (the owner and the read-only demo user),
// signed session cookies and login rate limiting.
//
// Passwords come from .env. The owner's ARTHIK_PASSWORD may be plain text or a
// bcrypt hash ($2a$/$2b$/$2y$…, made with `arthik hash-password`). Sessions are a
// signed cookie (HMAC-SHA256 with ARTHIK_SECRET), so they survive restarts when a
// secret is set; there is no session store.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gitlab.com/niharokz/arthik/internal/httpx"
)

// CookieName is the session cookie.
const CookieName = "arthik_session"

// SessionTTL is how long a sign-in lasts.
const SessionTTL = 30 * 24 * time.Hour

// Account is a configured user.
type Account struct {
	User     httpx.User
	Password string // plain text or bcrypt hash
	ShowHint bool   // demo: the password may be shown on the login screen
}

// Auth holds the users and the signing key.
type Auth struct {
	accounts map[string]*Account
	secret   []byte
	limiter  *limiter
}

// New builds an Auth. An empty secret gets a random one (sessions end on restart).
func New(secret string, accounts ...*Account) *Auth {
	a := &Auth{accounts: map[string]*Account{}, limiter: newLimiter(10, 15*time.Minute)}
	for _, ac := range accounts {
		if ac != nil {
			a.accounts[strings.ToLower(ac.User.Name)] = ac
		}
	}
	if secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		a.secret = b
	} else {
		a.secret = []byte(secret)
	}
	return a
}

// HashPassword returns a bcrypt hash for .env.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	return string(h), err
}

// IsHash reports whether s looks like a bcrypt hash.
func IsHash(s string) bool {
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
}

func checkPassword(stored, given string) bool {
	if IsHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(given)) == nil
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(given)) == 1
}

func (a *Auth) sign(payload string) string {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// token is base64(user|expiry-unix|signature).
func (a *Auth) token(user string, exp time.Time) string {
	p := user + "|" + strconv.FormatInt(exp.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(p + "|" + a.sign(p)))
}

func (a *Auth) parse(tok string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return "", false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", false
	}
	p := parts[0] + "|" + parts[1]
	if !hmac.Equal([]byte(a.sign(p)), []byte(parts[2])) {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	return parts[0], true
}

// Resolve returns the signed-in user for r, or nil.
func (a *Auth) Resolve(r *http.Request) *httpx.User {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	name, ok := a.parse(c.Value)
	if !ok {
		return nil
	}
	ac := a.accounts[name]
	if ac == nil {
		return nil
	}
	return &ac.User
}

// DemoHint returns the demo user name and password if a demo user is configured.
func (a *Auth) DemoHint() (user, pass string, ok bool) {
	for _, ac := range a.accounts {
		if ac.ShowHint {
			return ac.User.Name, ac.Password, true
		}
	}
	return "", "", false
}

// Register adds the public auth routes: login, logout and me.
func (a *Auth) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r)})
		httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		out := map[string]any{"signed_in": false}
		if u := a.Resolve(r); u != nil {
			out = map[string]any{"signed_in": true, "user": u.Name, "read_only": u.ReadOnly}
		}
		if du, dp, ok := a.DemoHint(); ok {
			out["demo"] = map[string]string{"user": du, "password": dp}
		}
		httpx.JSON(w, http.StatusOK, out)
	})
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Arthik") == "" {
		httpx.Error(w, http.StatusForbidden, "missing X-Arthik header")
		return
	}
	ip := clientIP(r)
	if !a.limiter.allow(ip) {
		httpx.Error(w, http.StatusTooManyRequests, "too many failed sign-ins — try again in 15 minutes")
		return
	}
	var in struct{ User, Password string }
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	ac := a.accounts[strings.ToLower(strings.TrimSpace(in.User))]
	if ac == nil || ac.Password == "" || !checkPassword(ac.Password, in.Password) {
		a.limiter.fail(ip)
		time.Sleep(400 * time.Millisecond) // blunt brute force a little more
		httpx.Error(w, http.StatusUnauthorized, "wrong user name or password")
		return
	}
	a.limiter.reset(ip)
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: a.token(strings.ToLower(ac.User.Name), time.Now().Add(SessionTTL)), Path: "/",
		MaxAge: int(SessionTTL.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r),
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "user": ac.User.Name, "read_only": ac.User.ReadOnly})
}

// secure reports whether the browser reached us over HTTPS (directly or via Caddy/Cloudflare).
func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// clientIP prefers the address set by Cloudflare / the reverse proxy.
func clientIP(r *http.Request) string {
	if v := r.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v)
}

// limiter counts failed sign-ins per IP in a sliding window.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *limiter) prune(ip string, now time.Time) []time.Time {
	var keep []time.Time
	for _, t := range l.fails[ip] {
		if now.Sub(t) < l.window {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		delete(l.fails, ip)
	} else {
		l.fails[ip] = keep
	}
	return keep
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(ip, time.Now())) < l.max
}

func (l *limiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.prune(ip, time.Now()), time.Now())
}

func (l *limiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}

// String describes the configured users for the startup log (never passwords).
func (a *Auth) String() string {
	var s []string
	for _, ac := range a.accounts {
		mode := "read-write"
		if ac.User.ReadOnly {
			mode = "read-only"
		}
		kind := "plain password"
		if IsHash(ac.Password) {
			kind = "bcrypt hash"
		}
		s = append(s, fmt.Sprintf("%s (%s, %s)", ac.User.Name, mode, kind))
	}
	return strings.Join(s, ", ")
}
