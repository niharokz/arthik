// Package httpx is the small layer every feature's HTTP handlers are written
// against: a Router that resolves the signed-in user and their dataset, JSON helpers
// and errors that turn into clean messages for the UI.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"gitlab.com/niharokz/arthik/internal/book"
)

// User is a signed-in user and the dataset they work on.
type User struct {
	Name     string
	ReadOnly bool
	Book     *book.Book
}

// Ctx is passed to every handler.
type Ctx struct {
	W    http.ResponseWriter
	R    *http.Request
	User *User
	Book *book.Book
}

// HandlerFunc is a feature handler. Returning an error writes it as JSON:
// *StatusError keeps its code, book.ErrReadOnly is 403, anything else is 400.
type HandlerFunc func(c *Ctx) error

// StatusError is an error with an HTTP status.
type StatusError struct {
	Code int
	Msg  string
}

func (e *StatusError) Error() string { return e.Msg }

// Errorf builds a StatusError.
func Errorf(code int, format string, a ...any) error {
	return &StatusError{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// NotFound is a 404 for the named thing.
func NotFound(what string) error { return Errorf(http.StatusNotFound, "%s not found", what) }

// Resolver returns the signed-in user for a request (nil if not signed in).
type Resolver func(r *http.Request) *User

// Router registers feature routes on a ServeMux.
type Router struct {
	Mux     *http.ServeMux
	Resolve Resolver
}

// Handle registers an authenticated route. pattern uses Go 1.22 syntax
// ("GET /api/accounts/{id}"). Every non-GET request must carry the X-Arthik header
// (a CSRF guard: browsers cannot add custom headers cross-site without CORS) and is
// refused for read-only users.
func (rt *Router) Handle(pattern string, h HandlerFunc) {
	rt.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("PANIC %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
				Error(w, http.StatusInternalServerError, "internal error — see the server log")
			}
		}()
		u := rt.Resolve(r)
		if u == nil {
			Error(w, http.StatusUnauthorized, "please sign in")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Arthik") == "" {
				Error(w, http.StatusForbidden, "missing X-Arthik header")
				return
			}
			if u.ReadOnly {
				Error(w, http.StatusForbidden, book.ErrReadOnly.Error())
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		if err := h(&Ctx{W: w, R: r, User: u, Book: u.Book}); err != nil {
			var se *StatusError
			switch {
			case errors.As(err, &se):
				Error(w, se.Code, se.Msg)
			case errors.Is(err, book.ErrReadOnly):
				Error(w, http.StatusForbidden, err.Error())
			default:
				Error(w, http.StatusBadRequest, err.Error())
			}
		}
	})
}

// JSON writes v with status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": msg}.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// OK writes v with 200.
func (c *Ctx) OK(v any) error {
	JSON(c.W, http.StatusOK, v)
	return nil
}

// Decode reads a JSON body (max 2 MB) into v.
func (c *Ctx) Decode(v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.W, c.R.Body, 2<<20))
	if err := dec.Decode(v); err != nil {
		return Errorf(http.StatusBadRequest, "invalid request: %v", err)
	}
	return nil
}

// Q returns a trimmed query parameter.
func (c *Ctx) Q(name string) string { return strings.TrimSpace(c.R.URL.Query().Get(name)) }

// QInt returns an integer query parameter (def when missing or invalid).
func (c *Ctx) QInt(name string, def int) int {
	if v, err := strconv.Atoi(c.Q(name)); err == nil {
		return v
	}
	return def
}

// QList returns a comma-separated query parameter as a list.
func (c *Ctx) QList(name string) []string {
	var out []string
	for _, v := range c.R.URL.Query()[name] {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
