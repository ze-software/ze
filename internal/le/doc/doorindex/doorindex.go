// Design: docs/architecture/core-design.md -- the door index every le tool reads
//
// Package doorindex answers which pages under ai/doors cover a checkout path:
// the rule the door read gate (internal/le/hookruntime, writeDesignDoorsRead)
// and the derivation of `le spec design documents` both apply. It sits outside
// hookruntime because hookruntime will read the spec design package, which
// therefore cannot import hookruntime; one rule in one package keeps the gate
// and the derivation from disagreeing about which door a file sits under.
package doorindex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Dir is the directory the doors live in, from the checkout root.
const Dir = "ai/doors"

// SystemDoor is the door every change reads first, whatever it touches.
const SystemDoor = Dir + "/system.md"

// indexPage is the one page under Dir that is an index rather than a door, so
// it carries no door-base header.
const indexPage = Dir + "/README.md"

// The door-base header: `<!-- door-base: <path> ... -->`.
const (
	baseOpener = "<!-- door-base:"
	baseCloser = "-->"
)

// ErrDoorsMissing is a checkout whose Dir cannot be read.
var ErrDoorsMissing = errors.New("the doors directory cannot be read")

// HeaderError is a page under Dir whose door-base header is absent or cannot
// be read as a path list.
type HeaderError struct {
	Page   string
	Reason string
}

func (e *HeaderError) Error() string { return e.Page + ": " + e.Reason }

// door is one page under Dir and the checkout paths its header covers.
type door struct {
	page  string
	bases []string
}

// Index is the doors of one checkout. Only Load builds one, so an Index
// always holds every door the checkout carries. Safe for concurrent use: it
// is never written after Load.
type Index struct {
	doors []door
}

// Load parses the door-base header of every page under Dir except the README
// index. A page with no header, or a malformed one, is an error naming the
// page, never a door that covers nothing: a door that lost its header would
// otherwise release every file it covers in silence.
func Load(root string) (Index, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(Dir)))
	if err != nil {
		return Index{}, fmt.Errorf("%w: %w", ErrDoorsMissing, err)
	}
	var index Index
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		page := Dir + "/" + entry.Name()
		if page == indexPage {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page))) //nolint:gosec // a page directly under the checkout's ai/doors
		if err != nil {
			return Index{}, &HeaderError{Page: page, Reason: err.Error()}
		}
		bases, reason := doorBases(string(body))
		if reason != "" {
			return Index{}, &HeaderError{Page: page, Reason: reason}
		}
		index.doors = append(index.doors, door{page: page, bases: bases})
	}
	return index, nil
}

// doorBases reads the first door-base header line of a page. It answers why
// the page has no usable header, or "" with the paths the header names.
func doorBases(body string) (bases []string, reason string) {
	for line := range strings.Lines(body) {
		rest, opened := strings.CutPrefix(strings.TrimSpace(line), baseOpener)
		if !opened {
			continue
		}
		rest, closed := strings.CutSuffix(rest, baseCloser)
		if !closed {
			return nil, "its door-base header does not close with " + baseCloser
		}
		for base := range strings.FieldsSeq(rest) {
			bases = append(bases, strings.TrimSuffix(base, "/"))
		}
		if len(bases) == 0 {
			return nil, "its door-base header names no path"
		}
		return bases, ""
	}
	return nil, "it carries no line starting " + baseOpener
}

// Covering answers the doors covering file, a checkout-relative slash path:
// those whose base is the longest prefix of file on whole path components.
// Every door tied at that length covers it, because each describes a contract
// the file sits under. A file no base reaches answers nothing.
func (index Index) Covering(file string) []string {
	longest := -1
	var covering []string
	for _, candidate := range index.doors {
		for _, base := range candidate.bases {
			if !underBase(file, base) {
				continue
			}
			if len(base) < longest {
				continue
			}
			if len(base) > longest {
				longest = len(base)
				covering = covering[:0]
			}
			if !slices.Contains(covering, candidate.page) {
				covering = append(covering, candidate.page)
			}
		}
	}
	return covering
}

// underBase reports whether file is base itself or sits below it, so
// internal/component/bgpx is not under internal/component/bgp.
func underBase(file, base string) bool {
	if file == base {
		return true
	}
	return strings.HasPrefix(file, base+"/")
}
