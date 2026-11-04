// Package plugin holds Arthik's personal plugins: small optional screens that live
// outside the core features. A plugin is on for a dataset when its own data file
// exists in the data folder (e.g. loans.md), so nothing changes for anyone else.
package plugin

import (
	"os"
	"path/filepath"

	"gitlab.com/niharokz/arthik/internal/httpx"
)

// Plugin is one optional add-on.
type Plugin struct {
	ID       string                 `json:"id"`
	Title    string                 `json:"title"`
	Icon     string                 `json:"icon"`
	File     string                 `json:"-"` // data file that turns it on
	Register func(rt *httpx.Router) `json:"-"`
}

var all []Plugin

// Add registers a plugin (called from each plugin's init via Install).
func Add(p Plugin) { all = append(all, p) }

// Install adds every plugin's routes.
func Install(rt *httpx.Router) {
	for _, p := range all {
		p.Register(rt)
	}
}

// Enabled lists the plugins whose data file exists in dir.
func Enabled(dir string) []Plugin {
	out := []Plugin{}
	for _, p := range all {
		if _, err := os.Stat(filepath.Join(dir, p.File)); err == nil {
			out = append(out, p)
		}
	}
	return out
}
