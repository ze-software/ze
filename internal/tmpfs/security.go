// Design: docs/architecture/system-architecture.md — temporary filesystem management

package tmpfs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Validate checks all files for security issues.
// Returns an error if any file path is unsafe:
//   - Absolute paths (starting with /)
//   - Parent traversal (..)
//   - Hidden files (starting with .)
//   - Path escape attempts
func (v *Tmpfs) Validate() error {
	for _, f := range v.Files {
		if err := validatePath(f.Path); err != nil {
			return err
		}
	}
	return nil
}

// validatePath checks a single path for security issues.
func validatePath(path string) error {
	// Reject absolute paths
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("absolute path not allowed: %s", path)
	}

	// Reject parent traversal before cleaning
	if strings.Contains(path, "..") {
		return fmt.Errorf("parent traversal not allowed: %s", path)
	}

	// Reject hidden files (starting with . at any component)
	for component := range strings.SplitSeq(path, "/") {
		if strings.HasPrefix(component, ".") && component != "." {
			return fmt.Errorf("hidden file not allowed: %s", path)
		}
	}

	// Clean the path and verify it doesn't escape
	cleaned := filepath.Clean(path)

	// After cleaning, verify no parent traversal
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path escape not allowed: %s -> %s", path, cleaned)
	}

	// Verify cleaned path doesn't become absolute
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("path resolves to absolute: %s -> %s", path, cleaned)
	}

	return nil
}

// ValidateOptions provides optional validation settings.
type ValidateOptions struct {
	AllowHidden bool // Allow hidden files (starting with .)
}

// ValidateWithOptions checks all files with custom options.
func (v *Tmpfs) ValidateWithOptions(opts ValidateOptions) error {
	for _, f := range v.Files {
		if err := validatePathWithOptions(f.Path, opts); err != nil {
			return err
		}
	}
	return nil
}

// validatePathWithOptions checks a path with custom options.
func validatePathWithOptions(path string, opts ValidateOptions) error {
	// Reject absolute paths
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("absolute path not allowed: %s", path)
	}

	// Reject parent traversal before cleaning
	if strings.Contains(path, "..") {
		return fmt.Errorf("parent traversal not allowed: %s", path)
	}

	// Reject hidden files unless allowed
	if !opts.AllowHidden {
		for component := range strings.SplitSeq(path, "/") {
			if strings.HasPrefix(component, ".") && component != "." {
				return fmt.Errorf("hidden file not allowed: %s", path)
			}
		}
	}

	// Clean the path and verify it doesn't escape
	cleaned := filepath.Clean(path)

	// After cleaning, verify no parent traversal
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path escape not allowed: %s -> %s", path, cleaned)
	}

	// Verify cleaned path doesn't become absolute
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("path resolves to absolute: %s -> %s", path, cleaned)
	}

	return nil
}
