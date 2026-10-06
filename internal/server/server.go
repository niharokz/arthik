// Package server wires Arthik together: the public pages, the auth endpoints, every
// feature's API routes and the security headers.
package server

import (
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
	"gitlab.com/niharokz/arthik/internal/feature/reminders"
	"gitlab.com/niharokz/arthik/internal/feature/reports"
	"gitlab.com/niharokz/arthik/internal/feature/settings"
	"gitlab.com/niharokz/arthik/internal/feature/state"
	"gitlab.com/niharokz/arthik/internal/feature/transactions"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/web"
)

// New returns the HTTP handler.
func New(a *auth.Auth) http.Handler {
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
	reminders.Register(rt)
	budgets.Register(rt)
	reports.Register(rt)
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
	mux.HandleFunc("GET /{$}", page("about.html"))
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
	return headers(mux)
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
