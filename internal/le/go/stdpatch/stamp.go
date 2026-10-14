// Design: docs/contributing/patching-the-standard-library.md -- the review stamp
// Overview: stdpatch.go -- the generator and the activation

package stdpatch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// stampKey opens the line, in a patch's leading text, that names what the
// patch was reviewed against. `git apply` skips leading text, so the line
// costs the patch nothing.
const stampKey = "Ze-Reviewed-Against:"

// ErrNoStamp says a patch carries no review stamp. An unstamped patch is a
// patch nobody can say was reviewed, so every reader refuses it.
var ErrNoStamp = errors.New("the patch carries no " + stampKey + " line in its leading text")

// Patch is one standard-library patch: a git unified diff whose paths are
// relative to GOROOT (a/src/crypto/tls/handshake_server.go).
type Patch struct {
	// Path is the patch file, relative to the checkout.
	Path string
	// Body is the file's bytes.
	Body []byte
}

// Patches answers every *.patch file in PatchDir, sorted by name. An empty
// directory is an error: the marker reference in the EAP-TLS authenticator
// cannot compile without the patch that adds it.
func Patches(root string) ([]Patch, error) {
	entries, err := os.ReadDir(filepath.Join(root, PatchDir))
	if err != nil {
		return nil, fmt.Errorf("stdpatch: reading the patch set: %w", err)
	}
	patches := make([]Patch, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".patch") {
			continue
		}
		path := PatchDir + "/" + entry.Name()
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // a build tool reads the checkout it was pointed at
		if err != nil {
			return nil, fmt.Errorf("stdpatch: reading %s: %w", path, err)
		}
		patches = append(patches, Patch{Path: path, Body: body})
	}
	if len(patches) == 0 {
		return nil, fmt.Errorf("stdpatch: %s holds no *.patch file", PatchDir)
	}
	slices.SortFunc(patches, func(a, b Patch) int { return strings.Compare(a.Path, b.Path) })
	return patches, nil
}

// ReviewStamp answers the value of the Ze-Reviewed-Against line in a patch's
// leading text, the text before its first `diff --git` or `--- ` line. Both
// patch families keep their stamp there; each parses the value its own way
// (Stamp here, vendorpatch.Stamp for vendored modules).
func ReviewStamp(body []byte) (string, error) {
	for line := range bytes.SplitSeq(body, []byte("\n")) {
		text := strings.TrimRight(string(line), "\r")
		if strings.HasPrefix(text, "diff --git ") || strings.HasPrefix(text, "--- ") {
			break
		}
		value, found := strings.CutPrefix(text, stampKey)
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("the %s line names nothing", stampKey)
		}
		return value, nil
	}
	return "", ErrNoStamp
}

// Stamp answers the Go release a standard-library patch was reviewed against,
// for example go1.27.2.
func Stamp(patch Patch) (string, error) {
	value, err := ReviewStamp(patch.Body)
	if err != nil {
		return "", fmt.Errorf("stdpatch: %s: %w", patch.Path, err)
	}
	if strings.ContainsAny(value, " \t") || !strings.HasPrefix(value, "go") {
		return "", fmt.Errorf("stdpatch: %s: the %s line names %q, want one Go release such as go1.27.2", patch.Path, stampKey, value)
	}
	return value, nil
}

// checkStamps refuses, before any tree is built and before `git apply` runs,
// every patch whose stamp is not the toolchain's version. A clean apply
// proves only that the context lines did not move, never that upstream left
// the function's behavior alone, so a version nobody reviewed refuses even
// when the diff would still apply.
func checkStamps(patches []Patch, version string) error {
	for _, patch := range patches {
		stamp, err := Stamp(patch)
		if err != nil {
			return fmt.Errorf("%w\n%s", err, reviewStep(patch, version))
		}
		if stamp != version {
			return fmt.Errorf("stdpatch: %s was reviewed against %s, and the toolchain this checkout builds with is %s. "+
				"A patch that still applies may change code upstream has since changed.\n%s",
				patch.Path, stamp, version, reviewStep(patch, version))
		}
	}
	return nil
}

// reviewStep is the re-review procedure a stamp refusal carries. It needs no
// le: a launcher that must build le stops at this refusal.
func reviewStep(patch Patch, version string) string {
	return "Re-read the functions the patch changes (" + strings.Join(patchTargets(patch.Body), ", ") +
		") in the " + version + " GOROOT. When upstream now does what the patch does, delete the patch and what depends on it. " +
		"Otherwise regenerate the patch if git no longer applies it, run its tests, and change its " + stampKey + " line to " + version +
		" (" + pageSection + "). Then run: " + repairCommand
}

// patchTargets answers the GOROOT-relative paths a patch changes and that
// exist before it applies: the `--- a/<path>` lines.
func patchTargets(body []byte) []string {
	var targets []string
	for line := range bytes.SplitSeq(body, []byte("\n")) {
		path, found := strings.CutPrefix(strings.TrimRight(string(line), "\r"), "--- a/")
		if found && !slices.Contains(targets, path) {
			targets = append(targets, path)
		}
	}
	return targets
}

// AddedFiles answers the GOROOT-relative paths a patch creates: each
// `+++ b/<path>` line that follows `--- /dev/null`. The marker file is one.
func AddedFiles(patch Patch) []string {
	var added []string
	fromNothing := false
	for line := range bytes.SplitSeq(patch.Body, []byte("\n")) {
		text := strings.TrimRight(string(line), "\r")
		if text == "--- /dev/null" {
			fromNothing = true
			continue
		}
		path, found := strings.CutPrefix(text, "+++ b/")
		if found && fromNothing {
			added = append(added, path)
		}
		if strings.HasPrefix(text, "+++ ") {
			fromNothing = false
		}
	}
	return added
}
