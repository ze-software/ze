// Design: docs/architecture/core-design.md -- the htmx upgrade gate in Go
// Detail: scanner.go -- one file's DOM, text, and inheritance findings.
// Detail: report.go -- the structured verdict and its compatible prose.
//
// This file owns the Ze-specific half of the producer: derive htmx-bearing
// packages, scan each package whole, and judge findings against the explained
// list. The scanner has no fixed package list, so a new embedded htmx copy joins
// the gate on its first run.

package htmxupgrade

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	consumerRoot = "internal"
	consumerDir  = "assets"
	htmxPrefix   = "htmx"
	htmxSuffix   = ".js"
	explained    = "scripts/dev/htmx-upgrade-explained.txt"
)

type scanResult struct {
	issues   []Issue
	packages []string
	warnings []string
	files    int
}

type explainedKey struct {
	path     string
	category string
}

type staleRow struct {
	path     string
	category string
}

func scanTree(root string) (scanResult, error) {
	packages, err := htmxPackages(root)
	if err != nil {
		return scanResult{}, err
	}
	files, err := collectFiles(root, packages)
	if err != nil {
		return scanResult{}, err
	}

	result := scanResult{
		issues:   make([]Issue, 0),
		packages: packages,
		files:    len(files),
	}
	var tb textbuf.Buffer
	for _, path := range files {
		issues, readErr := checkFile(path)
		if readErr != nil {
			result.warnings = append(result.warnings, tb.Reset().
				Str("warning: cannot read ").Str(path).Str(": ").Err(readErr).String())
			continue
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return scanResult{}, fmt.Errorf("htmx-upgrade-check: locate %s beneath %s: %w", path, root, relErr)
		}
		for _, issue := range issues {
			issue.Path = filepath.ToSlash(rel)
			result.issues = append(result.issues, issue)
		}
	}
	return result, nil
}

func htmxPackages(root string) ([]string, error) {
	base := filepath.Join(root, consumerRoot)
	if _, err := os.Stat(base); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("htmx-upgrade-check: read %s: %w", base, err)
	}

	packages := make([]string, 0)
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != base {
			name := entry.Name()
			if name == "testdata" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
		}
		if entry.Name() != consumerDir {
			return nil
		}

		children, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, child := range children {
			name := child.Name()
			if child.IsDir() || !strings.HasPrefix(name, htmxPrefix) || !strings.HasSuffix(name, htmxSuffix) {
				continue
			}
			rel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			packages = append(packages, filepath.ToSlash(rel))
			break
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("htmx-upgrade-check: discover htmx packages: %w", err)
	}
	slices.Sort(packages)
	return packages, nil
}

func collectFiles(root string, packages []string) ([]string, error) {
	extensions := slices.Concat(defaultExtensions, extraExtensions)
	files := make([]string, 0)
	for _, pkg := range packages {
		path := filepath.Join(root, filepath.FromSlash(pkg))
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			if hasExtension(path, extensions) {
				files = append(files, path)
			}
			continue
		}
		err = filepath.WalkDir(path, func(candidate string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if hasExtension(entry.Name(), extensions) {
				files = append(files, candidate)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("htmx-upgrade-check: collect files under %s: %w", path, err)
		}
	}
	slices.Sort(files)
	return files, nil
}

func hasExtension(path string, extensions []string) bool {
	for _, extension := range extensions {
		if strings.HasSuffix(path, extension) {
			return true
		}
	}
	return false
}

func readExplained(root string) (map[explainedKey]string, error) {
	path := filepath.Join(root, filepath.FromSlash(explained))
	body, err := os.ReadFile(path) //nolint:gosec // a fixed file beneath the selected checkout
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("htmx-upgrade-check: %s is missing; it is this gate's record of what does not apply, and its absence is not an empty list", explained)
		}
		return nil, fmt.Errorf("htmx-upgrade-check: read %s: %w", explained, err)
	}
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("htmx-upgrade-check: %s is not valid UTF-8", explained)
	}

	rows := make(map[explainedKey]string)
	for index, raw := range splitLines(string(body)) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) == 3 {
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
		}
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			return nil, fmt.Errorf("htmx-upgrade-check: %s:%d: a row is `<path> | <category> | <reason>`, and every field must carry text", explained, index+1)
		}
		rows[explainedKey{path: fields[0], category: fields[1]}] = fields[2]
	}
	return rows, nil
}

func checkTree(root string, stderr io.Writer) (Report, int, error) {
	scan, err := scanTree(root)
	if err != nil {
		return Report{}, 1, err
	}
	writeWarnings(stderr, scan.warnings)
	if len(scan.packages) == 0 {
		return Report{}, 1, fmt.Errorf("htmx-upgrade-check: no %s/**/%s/ directory holds an htmx core file, so this check proved nothing", consumerRoot, consumerDir)
	}
	if scan.files == 0 {
		return Report{}, 1, fmt.Errorf("htmx-upgrade-check: no file was read under %s, so this check proved nothing", strings.Join(scan.packages, ", "))
	}

	rows, err := readExplained(root)
	if err != nil {
		return Report{}, 1, err
	}
	used := make(map[explainedKey]bool, len(rows))
	unexplained := make([]Issue, 0)
	for _, issue := range scan.issues {
		key := explainedKey{path: issue.Path, category: issue.Category}
		if _, ok := rows[key]; ok {
			used[key] = true
			continue
		}
		unexplained = append(unexplained, issue)
	}
	stale := make([]staleRow, 0)
	for key := range rows {
		if !used[key] {
			stale = append(stale, staleRow(key))
		}
	}
	slices.SortFunc(stale, func(a, b staleRow) int {
		if byPath := strings.Compare(a.path, b.path); byPath != 0 {
			return byPath
		}
		return strings.Compare(a.category, b.category)
	})

	report := Report{
		Issues:         unexplained,
		Files:          scan.files,
		Packages:       strings.Join(scan.packages, ", "),
		stale:          stale,
		explainedCount: len(rows),
		mode:           modeCheck,
	}
	if len(unexplained) > 0 || len(stale) > 0 {
		return report, 1, nil
	}
	return report, 0, nil
}

func reportTree(root string, stderr io.Writer) (Report, int, error) {
	scan, err := scanTree(root)
	if err != nil {
		return Report{}, 1, err
	}
	writeWarnings(stderr, scan.warnings)
	return Report{
		Issues:   scan.issues,
		Files:    scan.files,
		Packages: strings.Join(scan.packages, ", "),
		mode:     modeReport,
	}, 0, nil
}

func writeWarnings(target io.Writer, warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintln(target, warning) //nolint:errcheck // CLI diagnostics cannot be recovered
	}
}
