// Design: plan/spec-le-is-a-ze-binary.md -- native development-tool actions
package module

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const binarySniff = 8000

var ignoredDirectories = map[string]bool{
	".git": true, "vendor": true, "tmp": true, "node_modules": true,
}

func readText(path string) (string, fs.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, false, err
	}
	if !info.Mode().IsRegular() {
		return "", info.Mode(), false, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // a refactor tool reads the checkout it was pointed at
	if err != nil {
		return "", info.Mode(), false, err
	}
	n := min(len(raw), binarySniff)
	if bytes.IndexByte(raw[:n], 0) >= 0 || !utf8.Valid(raw) {
		return "", info.Mode(), false, nil
	}
	return string(raw), info.Mode(), true, nil
}

// writeAtomic replaces one regular file without exposing partially written bytes.
// The temporary file sits beside the target, so rename is an atomic operation on
// every supported repository filesystem.
func writeAtomic(path string, contents []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".le-module-*")
	if err != nil {
		return fmt.Errorf("stage %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("preserve permissions for %s: %w", path, err)
	}
	if _, err := tmp.Write(contents); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write staged %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync staged %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close staged %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func cleanRelative(value, label string) (string, error) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "/"))
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." || filepath.IsAbs(value) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%s must be repository-relative: %q", label, value)
	}
	return cleaned, nil
}

func walkFiles(root string, visit func(relative, absolute string) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && ignoredDirectories[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(relative), path)
	})
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
