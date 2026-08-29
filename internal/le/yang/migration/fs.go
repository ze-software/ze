// Design: plan/spec-le-is-a-ze-binary.md -- shared preflight and apply machinery
package yangmigration

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func readRegular(path string) ([]byte, fs.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file")
	}
	data, err := os.ReadFile(path) //nolint:gosec // a refactor tool reads the checkout it was pointed at
	return data, info.Mode().Perm(), err
}

func planMove(root, source, destination string, report *Report) {
	sourceBytes, _, err := readRegular(source)
	if err != nil {
		report.Refusals = append(report.Refusals, Refusal{Path: relative(root, source), Reason: err.Error()})
		return
	}
	move := Move{Source: relative(root, source), Destination: relative(root, destination)}
	destinationBytes, _, err := readRegular(destination)
	switch {
	case err == nil && bytes.Equal(sourceBytes, destinationBytes):
		move.Identical = true
		report.Moves = append(report.Moves, move)
	case err == nil:
		report.Refusals = append(report.Refusals, Refusal{Path: move.Destination, Reason: "destination exists with different content"})
	case os.IsNotExist(err):
		report.Moves = append(report.Moves, move)
	default:
		report.Refusals = append(report.Refusals, Refusal{Path: move.Destination, Reason: "cannot inspect destination: " + err.Error()})
	}
}

func planEdit(root, path string, before, after []byte, report *Report) {
	if bytes.Equal(before, after) {
		return
	}
	report.Edits = append(report.Edits, Edit{
		Path:   relative(root, path),
		Before: string(before),
		After:  string(after),
	})
}

func applyReport(root string, report *Report) error {
	if report.Refused() || !report.Apply {
		return nil
	}
	for _, edit := range report.Edits {
		path := filepath.Join(root, filepath.FromSlash(edit.Path))
		_, mode, err := readRegular(path)
		if err != nil {
			return fmt.Errorf("edit %s: %w", edit.Path, err)
		}
		if err := os.WriteFile(path, []byte(edit.After), mode); err != nil {
			return fmt.Errorf("edit %s: %w", edit.Path, err)
		}
	}
	for _, move := range report.Moves {
		source := filepath.Join(root, filepath.FromSlash(move.Source))
		destination := filepath.Join(root, filepath.FromSlash(move.Destination))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { //nolint:gosec // a directory inside the checkout, which the tree keeps world-readable so a second agent account can read it
			return fmt.Errorf("create destination for %s: %w", move.Destination, err)
		}
		if move.Identical {
			if err := os.Remove(source); err != nil {
				return fmt.Errorf("remove identical source %s: %w", move.Source, err)
			}
			continue
		}
		if err := os.Rename(source, destination); err != nil {
			return fmt.Errorf("move %s to %s: %w", move.Source, move.Destination, err)
		}
	}
	for _, removal := range report.Removals {
		path := filepath.Join(root, filepath.FromSlash(removal))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect removal %s: %w", removal, err)
		}
		if info.IsDir() {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove directory %s: %w", removal, err)
			}
		} else if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove file %s: %w", removal, err)
		}
	}
	return nil
}

func walkFiles(root string, include func(string) bool) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (name == "vendor" || name == "tmp" || name == "testdata" || strings.HasPrefix(name, ".git")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && include(path) {
			paths = append(paths, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(paths)
	return paths, err
}

func appendRefusal(report *Report, root, path string, err error) {
	report.Refusals = append(report.Refusals, Refusal{Path: relative(root, path), Reason: err.Error()})
}
