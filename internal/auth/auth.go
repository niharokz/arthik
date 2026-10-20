// Package auth handles Arthik's two users (the owner and the read-only demo user),
// signed session cookies and banning addresses that keep failing to sign in.
//
// Passwords come from .env. The owner's ARTHIK_PASSWORD may be plain text or a
// bcrypt hash ($2a$/$2b$/$2y$…, made with `arthik hash-password`). Sessions are a
// signed cookie (HMAC-SHA256 with ARTHIK_SECRET), so they survive restarts when a
// secret is set; there is no session store.
//
// A sign-in lasts Options.SessionTTL. A device that has the device key
// (ARTHIK_DEVICE_KEY, typed once into Settings) stays signed in until it signs out;
// changing the key in .env signs every such device out.
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
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gitlab.com/niharokz/arthik/internal/guard"
	"gitlab.com/niharokz/arthik/internal/httpx"
)

// CookieName is the session cookie.
const CookieName = "arthik_session"

// rememberFor is the cookie lifetime of a device-key session; the cookie is renewed
// as the app is used, so in practice it lasts until sign-out.
const rememberFor = 400 * 24 * time.Hour

// MinDeviceKey is the shortest accepted device key.
const MinDeviceKey = 24

// Account is a configured user.
type Account struct {
	User     httpx.User
	Password string // plain text or bcrypt hash
	ShowHint bool   // demo: the password may be shown on the login screen
}

// Options tune sign-in and sessions.
type Options struct {
	SessionTTL time.Duration              // a sign-in without the device key lasts this long
	DeviceKey  string                     // "" = no remembered devices
	Guard      *guard.Guard               // bans after failed sign-ins (nil = none, e.g. local mode)
	ClientIP   func(*http.Request) string // real client address (behind the reverse proxy)
}

// Auth holds the users and the signing key.
type Auth struct {
	accounts   map[string]*Account
	secret     []byte
	opt        Options
	localToken string // set in local mode: the phone app opens sessions with it
}

// EnableLocal turns on local mode: no passwords; the phone app opens a session for a
// user through /local/enter with this token (only the app knows it).
func (a *Auth) EnableLocal(token string) { a.localToken = token }

// New builds an Auth. An empty secret gets a random one (sessions end on restart).
func New(secret string, opt Options, accounts ...*Account) *Auth {
	if opt.SessionTTL <= 0 {
		opt.SessionTTL = 12 * time.Hour
	}
	if opt.ClientIP == nil {
		opt.ClientIP = func(r *http.Request) string { return guard.ClientIP(r, nil) }
	}
	a := &Auth{accounts: map[string]*Account{}, opt: opt}
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

// keyTag ties a remembered session to the current device key, so changing the key
// in .env ends every remembered session.
func (a *Auth) keyTag() string {
	if a.opt.DeviceKey == "" {
		return ""
	}
	return "k" + a.sign("device-key:" + a.opt.DeviceKey)[:16]
}

// token is base64(user|expiry-unix|kind|signature); kind is "" for a normal
// sign-in and the key tag for a remembered device (expiry 0 = until sign-out).
func (a *Auth) token(user string, exp time.Time, remember bool) string {
	e, kind := strconv.FormatInt(exp.Unix(), 10), ""
	if remember {
		e, kind = "0", a.keyTag()
	}
	p := user + "|" + e + "|" + kind
	return base64.RawURLEncoding.EncodeToString([]byte(p + "|" + a.sign(p)))
}

// session is a verified cookie.
type session struct {
	user     string
	exp      time.Time // zero when remembered
	remember bool
}

func (a *Auth) parse(tok string) (session, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return session{}, false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return session{}, false
	}
	p := strings.Join(parts[:3], "|")
	if !hmac.Equal([]byte(a.sign(p)), []byte(parts[3])) {
		return session{}, false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return session{}, false
	}
	if parts[2] != "" { // remembered: valid while the device key is unchanged
		if exp != 0 || parts[2] != a.keyTag() || a.keyTag() == "" {
			return session{}, false
		}
		return session{user: parts[0], remember: true}, true
	}
	if exp == 0 || time.Now().Unix() > exp {
		return session{}, false
	}
	return session{user: parts[0], exp: time.Unix(exp, 0)}, true
}

func (a *Auth) session(r *http.Request) (session, *Account) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return session{}, nil
	}
	s, ok := a.parse(c.Value)
	if !ok {
		return session{}, nil
	}
	return s, a.accounts[s.user]
}

// Resolve returns the signed-in user for r, or nil.
func (a *Auth) Resolve(r *http.Request) *httpx.User {
	if _, ac := a.session(r); ac != nil {
		return &ac.User
	}
	return nil
}

// setSession writes the session cookie for user.
func (a *Auth) setSession(w http.ResponseWriter, r *http.Request, user string, remember bool) {
	life := a.opt.SessionTTL
	if remember {
		life = rememberFor
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: a.token(user, time.Now().Add(life), remember), Path: "/",
		MaxAge: int(life.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r),
	})
}

// keyOK checks a device key in constant time.
func (a *Auth) keyOK(key string) bool {
	return a.opt.DeviceKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(a.opt.DeviceKey)) == 1
}

// failed records a failed secret check; the caller answers 401 (or 403 when banned).
func (a *Auth) failed(w http.ResponseWriter, r *http.Request, msg, reason string) {
	time.Sleep(400 * time.Millisecond) // blunt brute force a little more
	if g := a.opt.Guard; g != nil {
		ip := a.opt.ClientIP(r)
		if g.Fail(ip, reason) {
			httpx.Error(w, http.StatusForbidden, "Forbidden")
			return
		}
		if left := g.Left(ip); left <= 2 {
			msg += fmt.Sprintf(" — %d more and this address is blocked", left)
		}
	}
	httpx.Error(w, http.StatusUnauthorized, msg)
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

// Register adds the auth routes: login, logout, me and the device key.
func (a *Auth) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r)})
		httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/me", a.me)
	mux.HandleFunc("POST /api/device-key", a.remember)
	mux.HandleFunc("DELETE /api/device-key", a.forget)
	mux.HandleFunc("GET /local/enter", a.localEnter)
}

func (a *Auth) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	out := map[string]any{"signed_in": false}
	if s, ac := a.session(r); ac != nil {
		out = map[string]any{"signed_in": true, "user": ac.User.Name, "read_only": ac.User.ReadOnly,
			"remembered": s.remember, "device_key": a.opt.DeviceKey != "" && a.localToken == ""}
		if s.remember {
			a.setSession(w, r, s.user, true) // keep the cookie fresh while the app is used
		} else {
			out["expires"] = s.exp
		}
	}
	if a.localToken != "" {
		out["local"] = true // the phone app handles sign-in (no password screen)
	} else {
		out["session_hours"] = max(1, int(a.opt.SessionTTL.Hours()))
		if du, dp, ok := a.DemoHint(); ok {
			out["demo"] = map[string]string{"user": du, "password": dp}
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

// remember turns the current sign-in into one that lasts until sign-out, when
// the device key matches. A wrong key counts as a failed sign-in.
func (a *Auth) remember(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Arthik") == "" {
		httpx.Error(w, http.StatusForbidden, "missing X-Arthik header")
		return
	}
	s, ac := a.session(r)
	if ac == nil || ac.User.ReadOnly || a.localToken != "" {
		httpx.Error(w, http.StatusUnauthorized, "sign in first")
		return
	}
	if a.opt.DeviceKey == "" {
		httpx.Error(w, http.StatusConflict, "no device key is set on the server (ARTHIK_DEVICE_KEY in .env)")
		return
	}
	var in struct{ Key string }
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !a.keyOK(in.Key) {
		a.failed(w, r, "wrong device key", "device key")
		return
	}
	a.setSession(w, r, s.user, true)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true, "remembered": true})
}

// forget makes this device's sign-in a normal one again.
func (a *Auth) forget(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Arthik") == "" {
		httpx.Error(w, http.StatusForbidden, "missing X-Arthik header")
		return
	}
	s, ac := a.session(r)
	if ac == nil {
		httpx.Error(w, http.StatusUnauthorized, "sign in first")
		return
	}
	a.setSession(w, r, s.user, false)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true, "remembered": false})
}

// enterRoute limits ?to= to an app screen name such as "inbox" or "account/hdfc".
var enterRoute = regexp.MustCompile(`^[a-z]+(/[A-Za-z0-9_.-]+)?$`)

// localEnter opens a session for ?user= when ?token= matches, then shows the app
// (on the screen named by ?to=, e.g. from the phone's widget or notification).
func (a *Auth) localEnter(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if a.localToken == "" || subtle.ConstantTimeCompare([]byte(q.Get("token")), []byte(a.localToken)) != 1 {
		httpx.Error(w, http.StatusForbidden, "open arthik from the phone app")
		return
	}
	name := strings.ToLower(q.Get("user"))
	if a.accounts[name] == nil {
		httpx.Error(w, http.StatusNotFound, "no such user: "+name)
		return
	}
	// The phone's own engine: its app lock guards it, so the session lasts until the app stops.
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: a.token(name, time.Now().Add(rememberFor), false), Path: "/",
		MaxAge: int(rememberFor.Seconds()), HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	to := "home"
	if t := q.Get("to"); enterRoute.MatchString(t) {
		to = t
	}
	http.Redirect(w, r, "/app#/"+to, http.StatusSeeOther)
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Arthik") == "" {
		httpx.Error(w, http.StatusForbidden, "missing X-Arthik header")
		return
	}
	var in struct{ User, Password, Key string }
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	ac := a.accounts[strings.ToLower(strings.TrimSpace(in.User))]
	if ac == nil || ac.Password == "" || !checkPassword(ac.Password, in.Password) {
		a.failed(w, r, "wrong user name or password", "password")
		return
	}
	remember := false
	if in.Key != "" && !ac.User.ReadOnly {
		if !a.keyOK(in.Key) {
			a.failed(w, r, "wrong device key — fix it in Settings or clear it to sign in for "+hours(a.opt.SessionTTL), "device key")
			return
		}
		remember = true
	}
	if g := a.opt.Guard; g != nil {
		g.Reset(a.opt.ClientIP(r))
	}
	a.setSession(w, r, strings.ToLower(ac.User.Name), remember)
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "user": ac.User.Name, "read_only": ac.User.ReadOnly, "remembered": remember})
}

func hours(d time.Duration) string {
	if h := int(d.Hours()); h >= 1 {
		return fmt.Sprintf("%d h", h)
	}
	return d.String()
}

// secure reports whether the browser reached us over HTTPS (directly or via Caddy/Cloudflare).
func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v)
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
		switch {
		case ac.Password == "":
			kind = "no password"
		case IsHash(ac.Password):
			kind = "bcrypt hash"
		}
		s = append(s, fmt.Sprintf("%s (%s, %s)", ac.User.Name, mode, kind))
	}
	return strings.Join(s, ", ")
}
