// Design: ai/rules/feature-gate-registration.md -- the three static consumers are GENERATED, not hand-maintained
//
// feature_tags regenerates the build-tag lists that DERIVE from feature-gates.txt
// but live inside files a program cannot self-derive at runtime:
//
//   - .golangci.yml           `build-tags`   = ze_core + every gate tag (manifest order)
//   - gokrazy/ze/config.json  `GoBuildTags`  = ze_core, ze_appliance + every gate tag (sorted)
//   - docs/guide/quickstart.md `go install`  = ze_core, ze_distro + every gate tag (sorted)
//
// feature-gates.txt is the single source of truth (ai/rules/feature-gate-registration.md).
// The Makefile ZE_FEATURES, the test runner, the plugin_imports generator, and
// dep_audit all already derive from the manifest; these three static files could not,
// so they were hand-maintained + drift-gated. This generator makes them derived too:
// add a gate to feature-gates.txt, run `make generate`, and all three files update.
//
// Surgical, byte-stable edits: only the tag list changes, everything else in each
// file is preserved untouched, so the --check comparison is exact.
//
// Usage:  go run scripts/codegen/feature_tags.go [--check]
// Called by: make generate (and `--check` by the generate check target).
//
//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	checkOnly := flag.Bool("check", false, "verify the generated tag lists are current without writing them")
	flag.Parse()

	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}

	manifestOrder, sorted, err := readFeatureTags(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		fatal(err)
	}

	// .golangci.yml: ze_core + gate tags in manifest first-appearance order
	// (dep_audit compares as a set, so the order is cosmetic; manifest order keeps
	// the diff minimal against the historical hand-maintained list).
	golangciTags := append([]string{"ze_core"}, manifestOrder...)
	// gokrazy GoBuildTags: ze_core, ze_appliance base + gate tags sorted.
	gokrazyTags := append([]string{"ze_core", "ze_appliance"}, sorted...)
	// quickstart `go install` command: mirrors `make build` = ze_core, ze_distro +
	// ZE_FEATURES (sorted), so a user who go-installs without cloning gets the same
	// feature set the repo ships.
	quickstartTags := append([]string{"ze_core", "ze_distro"}, sorted...)

	targets := []struct {
		path      string
		transform func([]byte) ([]byte, error)
	}{
		{filepath.Join(root, ".golangci.yml"), func(b []byte) ([]byte, error) { return rewriteGolangci(b, golangciTags) }},
		{filepath.Join(root, "gokrazy", "ze", "config.json"), func(b []byte) ([]byte, error) { return rewriteGokrazy(b, gokrazyTags) }},
		{filepath.Join(root, "docs", "guide", "quickstart.md"), func(b []byte) ([]byte, error) { return rewriteQuickstart(b, quickstartTags) }},
	}

	stale := 0
	for _, t := range targets {
		changed, err := applyTarget(t.path, t.transform, *checkOnly)
		if err != nil {
			fatal(err)
		}
		if changed {
			stale++
			rel, _ := filepath.Rel(root, t.path)
			if *checkOnly {
				fmt.Fprintf(os.Stderr, "%s is stale; run make generate\n", rel)
			} else {
				fmt.Fprintf(os.Stdout, "updated %s\n", rel)
			}
		}
	}

	if *checkOnly {
		if stale > 0 {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "feature-tag lists are current (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md)")
		return
	}
	if stale == 0 {
		fmt.Fprintln(os.Stdout, "feature-tag lists already current (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md)")
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "feature_tags:", err)
	os.Exit(1)
}

// applyTarget reads path, runs transform, and either writes the result (returning
// whether it changed) or, in check mode, only reports whether it would change.
func applyTarget(path string, transform func([]byte) ([]byte, error), checkOnly bool) (bool, error) {
	orig, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	next, err := transform(orig)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Equal(orig, next) {
		return false, nil
	}
	if checkOnly {
		return true, nil
	}
	if err := os.WriteFile(path, next, 0o644); err != nil { //nolint:gosec // repo fixture, not secrets
		return false, err
	}
	return true, nil
}

// findModuleRoot walks up from the working directory to the directory holding go.mod.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// readFeatureTags parses feature-gates.txt ("<tag> <pkg>" per line; '#' comments and
// blank lines ignored) and returns the unique tags in first-appearance (manifest)
// order and, separately, sorted.
func readFeatureTags(path string) (manifestOrder, sorted []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag := strings.Fields(line)[0]
		if seen[tag] {
			continue
		}
		seen[tag] = true
		manifestOrder = append(manifestOrder, tag)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(manifestOrder) == 0 {
		return nil, nil, fmt.Errorf("%s: no feature-gate tags found", path)
	}
	sorted = append([]string(nil), manifestOrder...)
	sort.Strings(sorted)
	return manifestOrder, sorted, nil
}

// rewriteGolangci replaces the `- <tag>` items under the `build-tags:` key with tags,
// preserving indentation and every other line. Errors if the key is absent.
func rewriteGolangci(content []byte, tags []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	keyIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "build-tags:" {
			keyIdx = i
			break
		}
	}
	if keyIdx < 0 {
		return nil, fmt.Errorf(".golangci.yml: `build-tags:` key not found")
	}
	// Find the run of `- ` list items immediately below the key and their indent.
	start := keyIdx + 1
	end := start
	itemIndent := ""
	for end < len(lines) {
		trimmed := strings.TrimLeft(lines[end], " ")
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if itemIndent == "" {
			itemIndent = lines[end][:len(lines[end])-len(trimmed)]
		}
		end++
	}
	if end == start {
		return nil, fmt.Errorf(".golangci.yml: no `- <tag>` items under build-tags:")
	}
	rebuilt := make([]string, 0, len(tags))
	for _, tag := range tags {
		var b strings.Builder
		b.WriteString(itemIndent)
		b.WriteString("- ")
		b.WriteString(tag)
		rebuilt = append(rebuilt, b.String())
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, rebuilt...)
	out = append(out, lines[end:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// rewriteGokrazy replaces the single `"GoBuildTags": [...],` line with tags,
// preserving the leading indentation and trailing comma. Errors if absent.
func rewriteGokrazy(content []byte, tags []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, "\"GoBuildTags\":") {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("gokrazy/ze/config.json: `GoBuildTags` key not found")
	}
	indent := lines[idx][:len(lines[idx])-len(strings.TrimLeft(lines[idx], " \t"))]
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		quoted[i] = strconv.Quote(tag)
	}
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString("\"GoBuildTags\": [")
	b.WriteString(strings.Join(quoted, ", "))
	b.WriteString("],")
	lines[idx] = b.String()
	return []byte(strings.Join(lines, "\n")), nil
}

// rewriteQuickstart replaces the tag list inside the `go install -tags '...'` line
// of the quickstart doc, preserving the rest of the line (module path, @latest).
// Errors if the line is absent or malformed.
func rewriteQuickstart(content []byte, tags []string) ([]byte, error) {
	const marker = "go install -tags '"
	lines := strings.Split(string(content), "\n")
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("docs/guide/quickstart.md: `%s` line not found", marker)
	}
	line := lines[idx]
	quoteStart := strings.Index(line, marker) + len(marker)
	rest := line[quoteStart:]
	closeRel := strings.IndexByte(rest, '\'')
	if closeRel < 0 {
		return nil, fmt.Errorf("docs/guide/quickstart.md: unterminated -tags quote")
	}
	var b strings.Builder
	b.WriteString(line[:quoteStart])
	b.WriteString(strings.Join(tags, " "))
	b.WriteString(line[quoteStart+closeRel:])
	lines[idx] = b.String()
	return []byte(strings.Join(lines, "\n")), nil
}
