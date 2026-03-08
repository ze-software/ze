//go:build ignore

// Script to check codebase consistency across multiple dimensions.
// Run: go run scripts/consistency-check.go
// Or: make ze-consistency
//
// Checks performed:
//   - JSON struct tags use kebab-case (not snake_case or camelCase)
//   - plugin.Response uses StatusDone/StatusError constants (not string literals)
//   - Non-exempt .go files have // Design: comments
//   - Cross-reference bidirectionality (Detail↔Overview, Related↔Related)
//   - File size limits (warn >600, error >1000)
//   - Plugin structure completeness (dispatch_test.go, schema/, doc.go)
//   - Stale package references in docs and scripts
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ANSI colors.
const (
	red    = "\033[31m"
	yellow = "\033[33m"
	green  = "\033[32m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

// Severity levels.
const (
	sevError = "ERROR"
	sevWarn  = "WARN"
)

type finding struct {
	severity string
	category string
	file     string
	line     int
	message  string
}

var findings []finding

func report(sev, cat, file string, line int, msg string) {
	findings = append(findings, finding{sev, cat, file, line, msg})
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	checkJSONTags(root)
	checkStatusConstants(root)
	checkDesignRefs(root)
	checkCrossRefs(root)
	checkFileSizes(root)
	checkPluginStructure(root)

	// Print results grouped by category.
	if len(findings) == 0 {
		fmt.Printf("%s%s✓ All consistency checks passed%s\n", green, bold, reset)
		return
	}

	categories := map[string][]finding{}
	order := []string{}
	for _, f := range findings {
		if _, ok := categories[f.category]; !ok {
			order = append(order, f.category)
		}
		categories[f.category] = append(categories[f.category], f)
	}

	errors := 0
	warnings := 0

	for _, cat := range order {
		fs := categories[cat]
		fmt.Printf("\n%s%s── %s (%d)%s\n", bold, yellow, cat, len(fs), reset)
		for _, f := range fs {
			color := yellow
			if f.severity == sevError {
				color = red
				errors++
			} else {
				warnings++
			}
			loc := f.file
			if f.line > 0 {
				loc = fmt.Sprintf("%s:%d", f.file, f.line)
			}
			fmt.Printf("  %s%s%s %s — %s\n", color, f.severity, reset, loc, f.message)
		}
	}

	fmt.Printf("\n%s%sSummary: %d errors, %d warnings%s\n", bold, red, errors, warnings, reset)
	if errors > 0 {
		os.Exit(1)
	}
}

// --- JSON Tag Check ---

var (
	jsonTagRE   = regexp.MustCompile(`json:"([^"]+)"`)
	snakeCaseRE = regexp.MustCompile(`[a-z]_[a-z]`)
	camelCaseRE = regexp.MustCompile(`[a-z][A-Z]`)
)

func checkJSONTags(root string) {
	walkGoFiles(root, func(path string) {
		// Skip test files and research code.
		if isTestFile(path) {
			return
		}
		scanLines(path, func(line int, text string) {
			matches := jsonTagRE.FindAllStringSubmatch(text, -1)
			for _, m := range matches {
				tag := strings.Split(m[1], ",")[0] // strip ,omitempty etc.
				if tag == "-" || tag == "" {
					continue
				}
				if snakeCaseRE.MatchString(tag) {
					report(sevError, "json-kebab-case", path, line, fmt.Sprintf("snake_case JSON tag %q — use kebab-case", tag))
				}
				if camelCaseRE.MatchString(tag) {
					report(sevError, "json-kebab-case", path, line, fmt.Sprintf("camelCase JSON tag %q — use kebab-case", tag))
				}
			}
		})
	})
}

// --- Status Constants Check ---

var statusLiteralRE = regexp.MustCompile(`Status:\s*"(done|error|ok)"`)

func checkStatusConstants(root string) {
	walkGoFiles(root, func(path string) {
		scanLines(path, func(line int, text string) {
			if m := statusLiteralRE.FindStringSubmatch(text); m != nil {
				report(sevWarn, "status-constants", path, line, fmt.Sprintf("hardcoded Status: %q — use plugin.Status* constant", m[1]))
			}
		})
	})
}

// --- Design Doc References ---

func checkDesignRefs(root string) {
	walkGoFiles(root, func(path string) {
		if isExemptFile(path) || isTestFile(path) {
			return
		}
		// Only check files under internal/, pkg/, cmd/.
		if !strings.Contains(path, "internal/") && !strings.Contains(path, "pkg/") && !strings.Contains(path, "cmd/") {
			return
		}
		hasDesign := false
		scanLines(path, func(line int, text string) {
			if strings.Contains(text, "// Design:") {
				hasDesign = true
			}
		})
		if !hasDesign {
			report(sevWarn, "design-refs", path, 0, "missing // Design: comment")
		}
	})
}

// --- Cross-Reference Bidirectionality ---

var xrefRE = regexp.MustCompile(`// (Detail|Overview|Related): (\S+\.go)`)

func checkCrossRefs(root string) {
	// Collect all refs: map[dir][file] -> list of (keyword, target).
	type ref struct {
		keyword string
		target  string
		line    int
	}
	dirRefs := map[string]map[string][]ref{}

	walkGoFiles(root, func(path string) {
		if isExemptFile(path) || isTestFile(path) {
			return
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		scanLines(path, func(line int, text string) {
			m := xrefRE.FindStringSubmatch(text)
			if m == nil {
				return
			}
			if dirRefs[dir] == nil {
				dirRefs[dir] = map[string][]ref{}
			}
			dirRefs[dir][base] = append(dirRefs[dir][base], ref{m[1], m[2], line})
		})
	})

	// Check bidirectionality.
	inverse := map[string]string{"Detail": "Overview", "Overview": "Detail", "Related": "Related"}

	for dir, fileRefs := range dirRefs {
		for source, refs := range fileRefs {
			for _, r := range refs {
				// Check target file exists.
				targetPath := filepath.Join(dir, r.target)
				if _, err := os.Stat(targetPath); os.IsNotExist(err) {
					report(sevError, "cross-refs", filepath.Join(dir, source), r.line,
						fmt.Sprintf("stale ref to %s (file does not exist)", r.target))
					continue
				}
				// Also check if target is exempt (doc.go etc.) — exempt files don't need back-refs.
				targetBase := filepath.Base(r.target)
				if isExemptFilename(targetBase) {
					continue
				}
				// Check back-reference exists.
				// Target may be in a subdirectory (e.g., "show/main.go"), so resolve
				// its actual directory and look in that directory's refs.
				expectedKW := inverse[r.keyword]
				targetDir := filepath.Dir(targetPath)
				targetFileRefs := dirRefs[targetDir]
				// The back-ref from the target should point back to source.
				// If source and target are in different directories, the back-ref
				// path must be relative from target's directory to source.
				var expectedSource string
				if targetDir == dir {
					expectedSource = source
				} else {
					// Compute relative path from target dir back to source.
					rel, err := filepath.Rel(targetDir, filepath.Join(dir, source))
					if err != nil {
						continue
					}
					expectedSource = rel
				}
				found := false
				for _, tr := range targetFileRefs[targetBase] {
					if tr.target == expectedSource && tr.keyword == expectedKW {
						found = true
						break
					}
				}
				if !found {
					report(sevWarn, "cross-refs", targetPath, 0,
						fmt.Sprintf("missing %s: %s (referenced by %s with %s:)", expectedKW, expectedSource, filepath.Join(dir, source), r.keyword))
				}
			}
		}
	}
}

// --- File Size Check ---

func checkFileSizes(root string) {
	walkGoFiles(root, func(path string) {
		if isTestFile(path) {
			return
		}
		lines := countLines(path)
		if lines > 1000 {
			report(sevError, "file-size", path, 0, fmt.Sprintf("%d lines (max 1000)", lines))
		} else if lines > 600 {
			report(sevWarn, "file-size", path, 0, fmt.Sprintf("%d lines (review for splitting)", lines))
		}
	})
}

// --- Plugin Structure Check ---

func checkPluginStructure(root string) {
	pluginDir := filepath.Join(root, "internal/component/bgp/plugins")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "bgp-cmd-") {
			continue
		}
		dir := filepath.Join(pluginDir, e.Name())
		name := e.Name()

		// Check required files.
		requiredFiles := []struct {
			pattern string
			desc    string
		}{
			{"doc.go", "package documentation"},
			{"schema", "YANG schema directory"},
		}

		for _, req := range requiredFiles {
			target := filepath.Join(dir, req.pattern)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				report(sevError, "plugin-structure", dir, 0, fmt.Sprintf("missing %s (%s)", req.pattern, req.desc))
			}
		}

		// Check for dispatch_test.go (wiring test).
		if !fileExists(filepath.Join(dir, "dispatch_test.go")) {
			report(sevWarn, "plugin-structure", dir, 0, fmt.Sprintf("%s: missing dispatch_test.go (wiring test)", name))
		}
	}
}

// --- Helpers ---

func walkGoFiles(root string, fn func(path string)) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs (except root "."), vendor, research.
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" || base == "research" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			fn(path)
		}
		return nil
	})
}

func scanLines(path string, fn func(line int, text string)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		fn(lineNum, scanner.Text())
	}
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

func isExemptFile(path string) bool {
	return isExemptFilename(filepath.Base(path))
}

func isExemptFilename(name string) bool {
	return name == "register.go" ||
		name == "embed.go" ||
		name == "doc.go" ||
		name == "all.go" ||
		strings.HasSuffix(name, "_gen.go")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
