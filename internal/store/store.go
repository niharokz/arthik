// Package store reads and writes Arthik's data files: YAML documents saved as .md so
// they sit in the Obsidian vault next to everything else.
//
// File layout:
//
//	# <one-line header comment>
//	updated: 'YYYY-MM-DD HH:MM:SS'
//
//	<key>:
//	  - ...
//
// Safety rules (same spirit as Kronos):
//   - a missing file is fine (empty data);
//   - a file that fails to parse is an error with file + line, never treated as empty,
//     so a typo in Obsidian can never wipe data;
//   - writes are atomic (temp file + fsync + rename) and skipped when only the
//     "updated" stamp would change, so Syncthing is not flooded with no-op edits.
package store

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileError is a data file that exists but cannot be used.
type FileError struct {
	File string // base name, e.g. "accounts.md"
	Line int    // 0 when unknown
	Msg  string
}

func (e *FileError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s line %d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Msg)
}

var lineRe = regexp.MustCompile(`line (\d+): `)

// Read unmarshals path into v. It returns (false, nil) when the file does not exist.
func Read(path string, v any) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	name := filepath.Base(path)
	if err != nil {
		return true, &FileError{File: name, Msg: err.Error()}
	}
	if len(bytes.TrimSpace(stripComments(b))) == 0 {
		return true, nil // empty file = empty data
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		msg := strings.TrimPrefix(err.Error(), "yaml: ")
		fe := &FileError{File: name, Msg: msg}
		if m := lineRe.FindStringSubmatch(msg); m != nil {
			fmt.Sscan(m[1], &fe.Line)
			fe.Msg = strings.Replace(msg, m[0], "", 1)
			fe.Msg = strings.TrimPrefix(fe.Msg, "unmarshal errors:\n  ")
		}
		return true, fe
	}
	return true, nil
}

// Marshal renders v as 2-space-indented YAML.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return unescapeAstral(buf.Bytes()), nil
}

// yaml.v3 writes characters outside the Basic Multilingual Plane (emoji icons) as
// "\U0001F372" escapes. They are valid inside double quotes either way; turning them
// back into the literal character keeps the files readable in Obsidian.
var astralRe = regexp.MustCompile(`(^|[^\\])((?:\\\\)*)\\U([0-9A-Fa-f]{8})`)

func unescapeAstral(b []byte) []byte {
	for astralRe.Match(b) { // loop: adjacent escapes share the boundary character
		b = astralRe.ReplaceAllFunc(b, func(m []byte) []byte {
			sm := astralRe.FindSubmatch(m)
			var r rune
			fmt.Sscanf(string(sm[3]), "%x", &r)
			return append(append(append([]byte{}, sm[1]...), sm[2]...), []byte(string(r))...)
		})
	}
	return b
}

// Write saves v to path with a header comment and an "updated" stamp. It returns
// changed=false (and writes nothing) when the content is identical to what is on disk.
func Write(path, header string, v any, perm fs.FileMode) (changed bool, err error) {
	body, err := Marshal(v)
	if err != nil {
		return false, fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(bodyOf(old), bytes.TrimSpace(body)) {
		return false, nil
	}
	var out bytes.Buffer
	for _, l := range strings.Split(strings.TrimSpace(header), "\n") {
		out.WriteString("# " + strings.TrimSpace(strings.TrimPrefix(l, "#")) + "\n")
	}
	fmt.Fprintf(&out, "updated: '%s'\n\n", time.Now().Format("2006-01-02 15:04:05"))
	out.Write(body)
	return true, AtomicWrite(path, out.Bytes(), perm)
}

// bodyOf strips comment lines and the top-level "updated:" line for comparison.
func bodyOf(b []byte) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(stripComments(b)))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "updated:") {
			continue
		}
		out.WriteString(l + "\n")
	}
	return bytes.TrimSpace(out.Bytes())
}

// stripComments drops whole-line comments at column 0 (the header).
func stripComments(b []byte) []byte {
	var out bytes.Buffer
	for _, l := range bytes.Split(b, []byte("\n")) {
		if bytes.HasPrefix(l, []byte("#")) {
			continue
		}
		out.Write(l)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// AtomicWrite writes data to a temp file in the same directory, fsyncs it and
// renames it over path, so readers never see a half-written file.
func AtomicWrite(path string, data []byte, perm fs.FileMode) error {
	if perm == 0 {
		perm = 0o644
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		return err
	}
	return os.Rename(name, path)
}
