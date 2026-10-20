// Design: docs/architecture/core-design.md -- native edit hook policy
// Detail: writeedit.go -- writeStyleGuideRead and writeDesignDoorsRead, the checks that read these records
// Related: lifecycle.go -- hookSourceRead records a whole Read of a gated document
// Related: bash.go -- bashDocumentRead records a whole Bash print of one
package hookruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// styleGuidePath is the Go style guide a context reads before its first Go
// edit, relative to the checkout root.
const styleGuidePath = "docs/contributing/ze-go-style.md"

// doorsDir holds the subsystem doors, relative to the checkout root.
const doorsDir = "ai/doors"

// systemDoorPath is the door every code edit needs, whatever the file:
// ai/doors/README.md says to read it first, always.
const systemDoorPath = doorsDir + "/system.md"

// doorBaseOpener and doorBaseCloser frame the header line naming the paths a
// door covers: `<!-- door-base: internal/component/bgp internal/core/bgp -->`.
const (
	doorBaseOpener = "<!-- door-base:"
	doorBaseCloser = "-->"
)

// gatedDocument answers the checkout-relative path of the document that path
// names, when an edit gate asks for that document's read: the Go style guide,
// or a page under ai/doors. Anything else answers "". A path written from
// another directory (`./x`, `/repo/x`) is matched on its trailing components.
func gatedDocument(path string) string {
	path = filepath.ToSlash(path)
	if path == styleGuidePath || strings.HasSuffix(path, "/"+styleGuidePath) {
		return styleGuidePath
	}
	slash := strings.LastIndexByte(path, '/')
	if slash < 0 {
		return ""
	}
	dir, name := path[:slash], path[slash+1:]
	if !strings.HasSuffix(name, ".md") {
		return ""
	}
	if dir == doorsDir || strings.HasSuffix(dir, "/"+doorsDir) {
		return doorsDir + "/" + name
	}
	return ""
}

// documentMarkerName escapes a document path into one file-name component.
// The escape is reversible, so two documents never share a marker, and it
// escapes `+`, the separator lepath.MarkerOwnedBy splits an agent id on.
var documentMarkerName = strings.NewReplacer("%", "%25", "/", "%2F", "+", "%2B")

// documentReadMarker names the file recording this context's whole read of
// one gated document. A subagent's hook payload carries its PARENT's
// session_id and its own agent_id, so a subagent gets markers of its own: a
// read in the parent's context put nothing into the subagent's. The name ends
// in the fingerprint the way every per-agent file does
// (lepath.AgentFingerprint, lepath.MarkerOwnedBy), so the session reaper
// removes it with its session. It answers "" when the session or the agent
// cannot be named, and every caller reads "" as no record.
func documentReadMarker(ctx context, document string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	fingerprint, err := lepath.AgentFingerprint(id, ctx.payload.AgentID)
	if err != nil {
		return ""
	}
	return filepath.Join(ctx.root, "tmp", "session", ".doc-read-"+documentMarkerName.Replace(document)+"-"+fingerprint)
}

// recorded reports whether a marker exists. MUST be called with a marker
// documentReadMarker named: "" is not a record.
func recorded(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// codeFile reports whether path is a file the door gate guards: Go (tests
// included), YANG, or templ. Each one carries a design the doors describe.
func codeFile(path string) bool {
	for _, extension := range []string{".go", ".yang", ".templ"} {
		if strings.HasSuffix(path, extension) {
			return true
		}
	}
	return false
}

// door is one page under ai/doors and the checkout paths its header covers.
type door struct {
	page  string
	bases []string
}

// doorIndexPage is the one page under ai/doors that is an index rather than a
// door, so it carries no door-base header.
const doorIndexPage = doorsDir + "/README.md"

// errDoorsMissing is an ai/doors directory the checkout does not hold.
var errDoorsMissing = errors.New("the doors directory cannot be read")

// doorHeaderError is a page under ai/doors whose door-base header is absent or
// cannot be read as a path list.
type doorHeaderError struct {
	page   string
	reason string
}

func (e *doorHeaderError) Error() string { return e.page + ": " + e.reason }

// loadDoors parses the door-base header of every page under ai/doors except
// the README index. A page with no header, or a malformed one, is an error
// naming the page, never a door that covers nothing: a door that lost its
// header would otherwise release every file it covers in silence.
func loadDoors(root string) ([]door, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(doorsDir)))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errDoorsMissing, err)
	}
	var doors []door
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		page := doorsDir + "/" + entry.Name()
		if page == doorIndexPage {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page))) //nolint:gosec // a page directly under the checkout's ai/doors
		if err != nil {
			return nil, &doorHeaderError{page: page, reason: err.Error()}
		}
		bases, reason := doorBases(string(body))
		if reason != "" {
			return nil, &doorHeaderError{page: page, reason: reason}
		}
		doors = append(doors, door{page: page, bases: bases})
	}
	return doors, nil
}

// doorBases reads the first door-base header line of a page. It answers why
// the page has no usable header, or "" with the paths the header names.
func doorBases(body string) (bases []string, reason string) {
	for line := range strings.Lines(body) {
		rest, opened := strings.CutPrefix(strings.TrimSpace(line), doorBaseOpener)
		if !opened {
			continue
		}
		rest, closed := strings.CutSuffix(rest, doorBaseCloser)
		if !closed {
			return nil, "its door-base header does not close with -->"
		}
		for _, base := range strings.Fields(rest) {
			bases = append(bases, strings.TrimSuffix(base, "/"))
		}
		if len(bases) == 0 {
			return nil, "its door-base header names no path"
		}
		return bases, ""
	}
	return nil, "it carries no line starting " + doorBaseOpener
}

// checkoutPath answers file's path relative to the checkout root, and whether
// the file lies inside the checkout at all. A path that leaves the root is
// retried with symlinks resolved on both sides, so a symlink alias of the
// checkout is still this checkout. A file outside it after that (scratch,
// another checkout or worktree) answers false.
func checkoutPath(ctx context) (string, bool) {
	file := relativePath(ctx)
	if insideRoot(file) {
		return file, true
	}
	root, err := filepath.EvalSymlinks(ctx.root)
	if err != nil {
		return file, false
	}
	relative, err := filepath.Rel(root, resolveExisting(absolutePath(ctx)))
	if err != nil {
		return file, false
	}
	relative = filepath.ToSlash(relative)
	return relative, insideRoot(relative)
}

// insideRoot reports whether a checkout-relative path stays under the root.
func insideRoot(relative string) bool {
	if filepath.IsAbs(relative) {
		return false
	}
	if relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, "../")
}

// resolveExisting resolves symlinks in path. A file a Write is about to
// create does not exist yet, so the nearest existing ancestor is resolved and
// the rest joined back. The loop ends at the filesystem root, where
// filepath.Dir returns its own input.
func resolveExisting(path string) string {
	rest := ""
	for {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(path, rest)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}

// coveringDoors answers the doors covering file: those whose base is the
// longest prefix of file on whole path components. Every door tied at that
// length covers it, because each describes a contract the file sits under.
// A file no base reaches answers nothing.
func coveringDoors(doors []door, file string) []string {
	longest := -1
	var covering []string
	for _, candidate := range doors {
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
