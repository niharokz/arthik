// Package server wires Arthik together: the public pages, the auth endpoints, every
// feature's API routes and the security headers.
package server

import (
	"html"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"gitlab.com/niharokz/arthik/internal/auth"
	"gitlab.com/niharokz/arthik/internal/feature/accounts"
	"gitlab.com/niharokz/arthik/internal/feature/budgets"
	"gitlab.com/niharokz/arthik/internal/feature/categories"
	"gitlab.com/niharokz/arthik/internal/feature/importexport"
	"gitlab.com/niharokz/arthik/internal/feature/labels"
	"gitlab.com/niharokz/arthik/internal/feature/reports"
	"gitlab.com/niharokz/arthik/internal/feature/review"
	"gitlab.com/niharokz/arthik/internal/feature/settings"
	"gitlab.com/niharokz/arthik/internal/feature/state"
	"gitlab.com/niharokz/arthik/internal/feature/summary"
	"gitlab.com/niharokz/arthik/internal/feature/transactions"
	"gitlab.com/niharokz/arthik/internal/guard"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/web"
)

// Options tune the handler.
type Options struct {
	Local    bool                       // phone app: no about page, the app opens straight away
	Guard    *guard.Guard               // blocks banned addresses (nil = none)
	ClientIP func(*http.Request) string // real client address, for the guard
}

// New returns the HTTP handler.
func New(a *auth.Auth, o Options) http.Handler {
	mux := http.NewServeMux()
	rt := &httpx.Router{Mux: mux, Resolve: a.Resolve}

	// Public.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	a.Register(mux)

	// Features — one package each.
	state.Register(rt)
	accounts.Register(rt)
	categories.Register(rt)
	labels.Register(rt)
	transactions.Register(rt)
	review.Register(rt)
	budgets.Register(rt)
	reports.Register(rt)
	summary.Register(rt)
	settings.Register(rt)
	importexport.Register(rt)

	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusNotFound, "no such API endpoint")
	})

	// Front end.
	static, _ := fs.Sub(web.Files, "static")
	files := http.FileServer(http.FS(static))
	page := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, err := fs.ReadFile(static, name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(b)
		}
	}
	if o.Local {
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/app", http.StatusFound) })
	} else {
		mux.HandleFunc("GET /{$}", landing(static, a))
	}
	mux.HandleFunc("GET /app", page("app.html"))
	mux.HandleFunc("GET /app/", page("app.html"))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sw.js":
			w.Header().Set("Cache-Control", "no-cache")
		case strings.HasPrefix(r.URL.Path, "/js/"), strings.HasPrefix(r.URL.Path, "/css/"):
			w.Header().Set("Cache-Control", "no-cache") // the service worker does the offline caching
		default:
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		files.ServeHTTP(w, r)
	})
	if o.Guard != nil {
		return headers(o.Guard.Middleware(o.ClientIP, mux))
	}
	return headers(mux)
}

// Repo is the project's home; the newest APK is attached to its latest release.
const Repo = "https://gitlab.com/niharokz/arthik"

// landing serves the public start page with the version, the APK link and the demo login.
func landing(static fs.FS, a *auth.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(static, "about.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		demo := ""
		if u, p, ok := a.DemoHint(); ok {
			demo = `<p class="demo-line">👀 Try the read-only demo: user <code>` + html.EscapeString(u) + `</code> · password <code>` +
				html.EscapeString(p) + `</code> · <a href="/app">open it</a></p>`
		}
		page := strings.NewReplacer("{{VERSION}}", html.EscapeString(state.Version), "{{APK}}", Repo+"/-/releases/permalink/latest/downloads/arthik.apk",
			"{{REPO}}", Repo, "{{DEMO}}", demo).Replace(string(b))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(page))
	}
}

// headers adds security headers to every response.
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// Timeouts for the http.Server.
const (
	ReadTimeout  = 30 * time.Second
	WriteTimeout = 60 * time.Second
	IdleTimeout  = 120 * time.Second
)
