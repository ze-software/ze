// Design: docs/architecture/core-design.md -- canonical per-session scratch path
// Related: ../lepath/session.go -- the single session identity and directory resolver
package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/lepath"
)

// ScratchReport is the checkout-root-relative path of this session's private
// scratch directory and whether this call ensured that directory exists.
type ScratchReport struct {
	Path    string `json:"path"`
	Ensured bool   `json:"ensured"`
}

// Text renders the path exactly as path-only callers consume it.
func (r ScratchReport) Text() string { return r.Path + "\n" }

// Scratch resolves this session's private scratch path. When ensure is true,
// the directory exists before Scratch returns successfully.
func Scratch(root string, ensure bool) (ScratchReport, error) {
	paths, err := lepath.ResolveSession(root, ensure)
	if err != nil {
		return ScratchReport{}, err
	}
	return ScratchReport{Path: filepath.ToSlash(paths.Scratch), Ensured: ensure}, nil
}

// scratchCleanReport records the current session directory explicitly removed
// by a clean invocation. It intentionally renders no text.
type scratchCleanReport struct {
	Session string `json:"session"`
	Removed bool   `json:"removed"`
}

// Text preserves the clean form's silent success.
func (scratchCleanReport) Text() string { return "" }

// cleanScratch removes this session's whole dated directory, including bin,
// etc, scratch, and state. It refuses a symlinked session root.
func cleanScratch(root string) (scratchCleanReport, error) {
	paths, err := lepath.ResolveSession(root, false)
	if err != nil {
		return scratchCleanReport{}, err
	}
	report := scratchCleanReport{Session: filepath.ToSlash(paths.Dir)}
	sessionRoot := filepath.Join(root, "tmp", "session")
	info, err := os.Lstat(sessionRoot)
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return report, fmt.Errorf("session scratch: refusing unsafe session root %s", sessionRoot)
	}
	sessionPath := filepath.Join(root, paths.Dir)
	if filepath.Dir(sessionPath) != sessionRoot {
		return report, fmt.Errorf("session scratch: refusing path outside %s", sessionRoot)
	}
	if _, err := os.Lstat(sessionPath); os.IsNotExist(err) {
		return report, nil
	} else if err != nil {
		return report, err
	}
	if err := os.RemoveAll(sessionPath); err != nil {
		return report, err
	}
	report.Removed = true
	return report, nil
}
