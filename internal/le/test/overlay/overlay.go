// Design: docs/contributing/rfc-conformance-gates.md -- every carrier applies the break through a Go overlay
// Detail: carrier.go -- the runners an overlay is handed to
// Detail: interop.go -- the interop scenario's verdict readers
// Detail: result.go -- the .ci and `go test` verdict readers
//
// Package overlay runs a source change against one test carrier with the tree
// untouched. It writes Go's own -overlay file, builds the argv and environment
// each carrier's own runner takes it through, and reads that runner's verdict.
// Nothing here re-implements a runner the repository already has, and nothing
// here knows why a run is made: `le rfc record` and the red walk each judge
// what the verdict means for their own question.
package overlay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// File is one whole file an overlay replaces for the length of one run.
//
// Absent is a deletion: the build then runs as if the file did not exist, which
// Go's overlay states as an empty backing path (`go help build`, -overlay). It
// is a field of its own rather than an empty Content, because an empty file and
// no file compile differently: an empty .go file is a syntax error.
type File struct {
	// Rel is the slash-separated path of the replaced file, relative to the tree.
	Rel string
	// Content is the text the build reads in place of the file on disk.
	Content string
	// Absent says the build sees no file at Rel, whatever Content holds.
	Absent bool
}

// Write writes the replacement files and the overlay that selects them, under
// this session's scratch directory for area, and answers the overlay's path.
//
// Go's own overlay, which is what gomu applies its mutants through: the
// compiler reads the replacement and the file on disk is never touched.
//
// Each run stages both files in a directory of its own. The session scratch is
// shared by every subagent of the session, and a replacement keyed on the
// producer's basename alone let one agent's break of register.go compile
// against a sibling's (plan/journal/concurrent-session-corruption.md).
func Write(tree, area string, files ...File) (string, error) {
	shared, err := Scratch(tree, area)
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	scratch, err := os.MkdirTemp(shared, "run-*")
	if err != nil {
		return "", errors.New(tb.Str("cannot create this run's break directory: ").Err(err).String())
	}
	// One replacement per file of a paired break, numbered so two producers
	// sharing a basename never overwrite each other's copy.
	replace := make(map[string]string, len(files))
	for position, file := range files {
		target := filepath.Join(tree, filepath.FromSlash(file.Rel))
		if file.Absent {
			replace[target] = ""
			continue
		}
		replacement := filepath.Join(scratch, tb.Reset().Str("broken-").Int(int64(position)).
			Byte('-').Str(filepath.Base(file.Rel)).String())
		if err := os.WriteFile(replacement, []byte(file.Content), 0o600); err != nil {
			return "", errors.New(tb.Reset().Str("cannot write the broken source: ").Err(err).String())
		}
		replace[target] = replacement
	}
	raw, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: replace})
	if err != nil {
		return "", errors.New(tb.Reset().Str("cannot render the overlay: ").Err(err).String())
	}
	path := filepath.Join(scratch, "overlay.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", errors.New(tb.Reset().Str("cannot write the overlay: ").Err(err).String())
	}
	return path, nil
}

// Scratch answers this session's directory for one area's runs, creating it.
func Scratch(tree, area string) (string, error) {
	paths, err := lepath.ResolveSession(tree, true)
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("cannot resolve this session's scratch directory, which is ").
			Str("where a break is staged: ").Err(err).String())
	}
	directory := filepath.Join(tree, filepath.FromSlash(paths.Scratch), area)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str(relativeTo(tree, directory)).Str(": cannot create directory: ").
			Err(err).String())
	}
	return directory, nil
}

// relativeTo answers path relative to tree, slash-separated, or path itself
// when it lies outside the tree.
func relativeTo(tree, path string) string {
	rel, err := filepath.Rel(tree, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// SetGoFlags puts the overlay where every Go compile this process starts will
// read it, and answers the restore.
func SetGoFlags(overlay string) (func(), error) {
	const key = "GOFLAGS"
	previous, held := os.LookupEnv(key)
	var tb textbuf.Buffer
	if err := os.Setenv(key, tb.Str("-overlay=").Str(overlay).String()); err != nil {
		return nil, errors.New(tb.Reset().Str("cannot place the overlay in GOFLAGS: ").Err(err).String())
	}
	return func() {
		if held {
			os.Setenv(key, previous) //nolint:errcheck // restoring what was read a moment ago
			return
		}
		os.Unsetenv(key) //nolint:errcheck // restoring what was read a moment ago
	}, nil
}
