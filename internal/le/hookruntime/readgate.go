// Design: docs/architecture/core-design.md -- native edit hook policy
// Detail: writeedit.go -- writeStyleGuideRead and writeDesignDoorsRead, the checks that read these records
// Related: specdoors.go -- writeSpecDoorsRead, the spec-edit check that reads them too
// Related: lifecycle.go -- hookSourceRead records a whole Read of a gated document
// Related: bash.go -- bashDocumentRead records a whole Bash print of one
package hookruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/doc/doorindex"
	"github.com/ze-software/ze/internal/le/doc/readrecord"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// styleGuidePath is the Go style guide a context reads before its first Go
// edit, relative to the checkout root.
const styleGuidePath = "docs/contributing/ze-go-style.md"

// doorsDir holds the subsystem doors, relative to the checkout root. The
// door-base rule that names which doors cover a file is doorindex's, which
// `le spec design` reads too.
const doorsDir = doorindex.Dir

// systemDoorPath is the door every code edit needs, whatever the file:
// ai/doors/README.md says to read it first, always.
const systemDoorPath = doorindex.SystemDoor

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

// documentCheckout is a checkout whose copies of the gated documents bind the
// edits of its files: the session's own root, or another checkout of Ze, such
// as a worktree under the session's .claude/worktrees or one elsewhere on the
// disk (ai/owner/machines.md). Two checkouts can hold different copies when a
// branch changes them, so a read counts only for the checkout whose copy it
// read. root is where the checkout's files are joined. label names a checkout
// other than the session's root in a marker and in a refusal, and is "" for
// the session's root.
type documentCheckout struct {
	root  string
	label string
}

// name is how a marker and a refusal spell relative, a checkout-relative path:
// unchanged in the session's root, and under the checkout's label in another.
func (checkout documentCheckout) name(relative string) string {
	if checkout.label == "" {
		return relative
	}
	return checkout.label + "/" + relative
}

// gatedCheckout answers the checkout path (absolute) lies in, with path
// relative to it, and whether path lies in one at all. The checkout is the
// nearest ancestor of path that is the session's root or a git checkout of Ze
// (gitCheckout), so a file in a worktree under the session's root belongs to
// the worktree, not to the root that holds it. The path as written is walked
// first, so a file written under the session's root stays that root's file
// even through a symlink that leaves it (tmp/), as checkoutPath judges it; only
// a path that reaches no checkout as written is walked again with symlinks
// resolved, so a symlink alias of a checkout is that checkout. It reads the
// filesystem alone, because the write hook runs on every edit and asking git
// would cost a process each time.
func gatedCheckout(ctx context, path string) (documentCheckout, string, bool) {
	sessionRoot := ctx.root
	if resolved, err := filepath.EvalSymlinks(ctx.root); err == nil {
		sessionRoot = resolved
	}
	checkout, relative, found := walkCheckout(ctx, filepath.Clean(path), filepath.Clean(ctx.root), sessionRoot)
	if found {
		return checkout, relative, true
	}
	return walkCheckout(ctx, resolveExisting(path), sessionRoot, sessionRoot)
}

// walkCheckout answers the nearest ancestor of file that is root (the
// session's root as file is spelled) or a git checkout, as gatedCheckout
// answers it. A checkout other than the session's root is labeled from its
// resolved directory, so one checkout has one label whichever spelling reached
// it, and a checkout whose resolved directory is the session's resolved root
// (sessionRoot) is that root: a spelling through a link to it, or the physical
// spelling when the session's root is itself a link, never a second checkout
// labeled ".". The walk shortens the path by one element a step and ends at
// the filesystem root, where filepath.Dir returns its own input.
func walkCheckout(ctx context, file, root, sessionRoot string) (documentCheckout, string, bool) {
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		if dir == root {
			return gatedCheckoutAt(documentCheckout{root: ctx.root}, dir, file)
		}
		if gitCheckout(dir) {
			resolved := dir
			if physical, err := filepath.EvalSymlinks(dir); err == nil {
				resolved = physical
			}
			if resolved == sessionRoot {
				return gatedCheckoutAt(documentCheckout{root: ctx.root}, dir, file)
			}
			return gatedCheckoutAt(documentCheckout{root: resolved, label: checkoutLabel(sessionRoot, resolved)}, dir, file)
		}
		if filepath.Dir(dir) == dir {
			return documentCheckout{}, "", false
		}
	}
}

// gatedCheckoutAt answers checkout, found at the resolved directory dir, with
// file relative to dir, as gatedCheckout answers it.
func gatedCheckoutAt(checkout documentCheckout, dir, file string) (documentCheckout, string, bool) {
	relative, err := filepath.Rel(dir, file)
	if err != nil {
		return documentCheckout{}, "", false
	}
	return checkout, filepath.ToSlash(relative), true
}

// gitCheckout reports whether dir is the top level of a git checkout of Ze:
// it carries the markers lepath names a Ze checkout by, and a .git entry, the
// directory of a clone or the file of a linked worktree. The .git entry is
// what keeps a fixture tree that copies the markers (a testdata checkout)
// from passing for a checkout whose doors bind its files.
func gitCheckout(dir string) bool {
	if !lepath.IsCheckout(dir) {
		return false
	}
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// checkoutLabel names a checkout other than the session's root, both resolved:
// its path relative to that root when it lies inside it
// (`.claude/worktrees/<name>`), else its absolute path.
func checkoutLabel(sessionRoot, dir string) string {
	if relative, err := filepath.Rel(sessionRoot, dir); err == nil {
		relative = filepath.ToSlash(relative)
		if insideRoot(relative) {
			return relative
		}
	}
	return filepath.ToSlash(dir)
}

// readDocument answers the checkout and the gated document a read of path
// reads, and whether it reads one: path is that document's own copy in the
// checkout it lies in. A copy kept anywhere else in a checkout
// (tmp/ai/doors/system.md) is no gated document there, so its read records
// nothing.
func readDocument(ctx context, path string) (documentCheckout, string, bool) {
	checkout, relative, inside := gatedCheckout(ctx, path)
	if !inside {
		return documentCheckout{}, "", false
	}
	document := gatedDocument(relative)
	if document == "" {
		return documentCheckout{}, "", false
	}
	if document != relative {
		return documentCheckout{}, "", false
	}
	return checkout, document, true
}

// markerKey is how a marker name carries relative, a checkout-relative path:
// unchanged in the session's root, and under the SHA-256 of the checkout's
// label in another, so a checkout's path of any depth keeps the marker name
// inside NAME_MAX (a name too long to create would refuse the edit forever).
func (checkout documentCheckout) markerKey(relative string) string {
	if checkout.label == "" {
		return relative
	}
	digest := sha256.Sum256([]byte(checkout.label))
	return hex.EncodeToString(digest[:]) + "/" + relative
}

// documentReadMarker names the file recording this context's whole read of
// checkout's copy of one gated document, in the format readrecord declares
// (WholeReadMarker), which `le spec design judge` reads too. The name carries
// the copy as checkout.markerKey spells it, so a read of one checkout's copy
// is no record for another's. A subagent's hook payload carries its PARENT's session_id and its
// own agent_id, so a subagent gets markers of its own: a read in the parent's
// context put nothing into the subagent's. The name ends in the fingerprint
// the way every per-agent file does (lepath.AgentFingerprint,
// lepath.MarkerOwnedBy), and every marker lives in the session root's
// tmp/session, whichever checkout the copy lies in, so the session reaper
// removes it with its session. It answers "" when the session or the agent
// cannot be named, and every caller reads "" as no record.
func documentReadMarker(ctx context, checkout documentCheckout, document string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	fingerprint, err := lepath.AgentFingerprint(id, ctx.payload.AgentID)
	if err != nil {
		return ""
	}
	return readrecord.WholeReadMarker(readrecord.SessionDir(ctx.root), checkout.markerKey(document), fingerprint)
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

// checkoutRelative is the source-evidence identity: always canonical, even for
// lexical in-root paths. Lookup failures are errors, never known-outside paths.
func checkoutRelative(ctx context) (string, error) {
	root, err := filepath.Abs(ctx.root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout root: %w", err)
	}
	file, err := resolveEvidencePath(absolutePath(ctx))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, file)
	if err != nil {
		return "", fmt.Errorf("resolve checkout-relative path: %w", err)
	}
	relative = filepath.ToSlash(relative)
	if !insideRoot(relative) {
		return "", fmt.Errorf("%s resolves outside this checkout", ctx.path)
	}
	return relative, nil
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
// This is planned-write membership, not the strict source-evidence identity.
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

// resolveEvidencePath resolves symlinks in path or, for a genuinely new path,
// in its nearest existing ancestor. Unlike planned-write membership, an
// existing but unresolvable symlink or any lookup error other than absence
// refuses the source identity.
func resolveEvidencePath(path string) (string, error) {
	rest := ""
	for {
		if _, err := os.Lstat(path); err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", fmt.Errorf("resolve %s: %w", path, err)
			}
			return filepath.Join(resolved, rest), nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %w", path, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("cannot resolve existing ancestor of %s", path)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}
