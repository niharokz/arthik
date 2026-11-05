package inbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/store"
)

// RequestFile is how a device that does not import (the phone in online mode) asks
// the main device for "SMS → drafts now": it writes this file, Syncthing carries it
// to the server, the server imports and deletes it, and the drafts sync back.
const RequestFile = "run_request.md"

const hdrRequest = "arthik: a device asked the main device to turn SMS into drafts now. Deleted once done."

type runRequest struct {
	Requested string `yaml:"requested"`
	Device    string `yaml:"device"`
}

func requestPath(dir string) string { return filepath.Join(dir, book.InboxDir, RequestFile) }

// Request asks the main device for an import; device names who asked.
func Request(dir, device string, now time.Time) error {
	if err := os.MkdirAll(filepath.Join(dir, book.InboxDir), 0o755); err != nil {
		return err
	}
	_, err := store.Write(requestPath(dir), hdrRequest, runRequest{Requested: now.Format(time.RFC3339), Device: device}, 0o644)
	return err
}

// Requested reports whether an import request is waiting.
func Requested(dir string) bool {
	_, err := os.Stat(requestPath(dir))
	return err == nil
}

// doneRequest removes the request after the import ran.
func doneRequest(dir string) error {
	if err := os.Remove(requestPath(dir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
