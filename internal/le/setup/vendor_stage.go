// Design: docs/contributing/patching-the-standard-library.md -- publish only a validated, patched vendor tree.
package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// vendorPatchArgv confines a patch to the tree whose module versions were
// checked. The checkout itself uses "."; a replacement uses its owned stage.
func vendorPatchArgv(directory string, arguments ...string) []string {
	capacity := 2 + len(arguments)
	if directory != "." {
		capacity += 2
	}
	argv := make([]string, 0, capacity)
	argv = append(argv, toolGit, gitApply)
	if directory != "." {
		argv = append(argv, "--directory", filepath.ToSlash(directory))
	}
	return append(argv, arguments...)
}

// patchedVendor is constructed only after the candidate passed every stamp and
// patch check. Its zero is uninitialized and publication refuses it.
type patchedVendor struct {
	root  string
	stage string
}

// publishVendor replaces live vendor only after its staged replacement passed
// patch validation. A failed second rename restores the original tree. Callers
// MUST retain stage on error, since a failed restore leaves the original there.
func publishVendor(replacement patchedVendor) error {
	if replacement.root == "" {
		return errors.New("publish vendor: uninitialized checkout")
	}
	if replacement.stage == "" {
		return errors.New("publish vendor: uninitialized patched replacement")
	}
	live := filepath.Join(replacement.root, "vendor")
	previous := filepath.Join(replacement.stage, "previous")
	info, err := os.Lstat(live)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect original vendor tree: %w", err)
		}
		if err := os.Rename(filepath.Join(replacement.stage, "vendor"), live); err != nil {
			return fmt.Errorf("publish first vendor tree: %w", err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("original vendor path %s is not a directory; refusing replacement", live)
	}
	if err := os.Rename(live, previous); err != nil {
		return fmt.Errorf("retain original vendor tree: %w", err)
	}
	if err := os.Rename(filepath.Join(replacement.stage, "vendor"), live); err != nil {
		publishErr := fmt.Errorf("publish patched vendor tree: %w", err)
		if restoreErr := os.Rename(previous, live); restoreErr != nil {
			return errors.Join(publishErr, fmt.Errorf("restore original vendor tree from %s: %w", previous, restoreErr))
		}
		return publishErr
	}
	return nil
}
