// Design: ai/rules/plugins.md -- static consumers are GENERATED, not hand-maintained
//
// Package featuretags regenerates the build-tag lists that DERIVE from
// feature-gates.txt but live inside files a program cannot read at run time:
//
//   - .golangci.yml            build-tags  = ze_core + every gate tag, in manifest order
//   - gokrazy/ze/config.json   GoBuildTags = ze_core, ze_appliance + every gate tag, sorted
//   - docs/guide/quickstart.md the go install command = ze_core, ze_distro + every gate tag, sorted
//   - .github/workflows/codeql.yml the two shipped go build combos
//
// feature-gates.txt is the single source of truth (ai/rules/plugins.md). The
// Makefile's ZE_FEATURES, the test runner, the plugin-imports generator and
// dep_audit all derive from the manifest already; these four files could not,
// so they were hand-maintained and drift-gated. This makes them derived too:
// add a gate to the manifest, run the write action, and all four follow.
//
// Every edit is SURGICAL. Only the tag list moves; the indentation, the
// neighboring keys and every other line of each file survive untouched, so
// the check's comparison is exact and a one-tag change stays a one-line diff.
package featuretags

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// manifestFile is the single source of truth for the gate tags, repo-relative.
const manifestFile = "feature-gates.txt"

// The two base tags a personality carries beside its gates. ze_core is in every
// build; the second says which product this build is.
const (
	coreTag      = "ze_core"
	applianceTag = "ze_appliance"
	distroTag    = "ze_distro"
)

// target is one file whose tag list is derived, and the edit that derives it.
type target struct {
	// rel is the file, relative to the tree. It is what the report names and
	// what a developer opens.
	rel string
	// rewrite answers what the file must hold, given what it holds now. It
	// errors when the anchor it edits is absent, so a restructured file stops
	// the run instead of being left behind silently.
	rewrite func(content []byte) ([]byte, error)
}

// targets answers the four derived files, in the order the report names them.
func targets(declared, sorted []string) []target {
	golangci := append([]string{coreTag}, declared...)
	gokrazy := append([]string{coreTag, applianceTag}, sorted...)
	// The quickstart's go install command mirrors `make build`, so a reader who
	// installs without cloning gets the feature set the repo ships.
	quickstart := append([]string{coreTag, distroTag}, sorted...)

	return []target{
		{rel: ".golangci.yml", rewrite: func(b []byte) ([]byte, error) { return rewriteGolangci(b, golangci) }},
		{rel: filepath.Join("gokrazy", "ze", "config.json"), rewrite: func(b []byte) ([]byte, error) { return rewriteGokrazy(b, gokrazy) }},
		{rel: filepath.Join("docs", "guide", "quickstart.md"), rewrite: func(b []byte) ([]byte, error) { return rewriteQuickstart(b, quickstart) }},
		// codeql.yml builds the two shipped combos, and the two lists above
		// already carry exactly those personality prefixes.
		{rel: filepath.Join(".github", "workflows", "codeql.yml"), rewrite: func(b []byte) ([]byte, error) {
			return rewriteCodeQL(b, quickstart, gokrazy)
		}},
	}
}

// derivedFiles names the four files, for the verdict a person reads. It is
// derived from the same table the edits read, so the sentence cannot name a
// file the tool does not touch.
func derivedFiles() string {
	var tb textbuf.Buffer
	for i, one := range targets(nil, nil) {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(filepath.ToSlash(one.rel))
	}
	return tb.String()
}

// readTags parses feature-gates.txt ("<tag> <pkg>" per line, with '#' comments
// and blank lines ignored) into the unique tags in first-appearance order and,
// separately, sorted.
//
// A manifest declaring no gate is an ERROR rather than an empty list. Writing
// the empty answer would strip every gate tag out of four files at once, and a
// check would then report that the emptied files are current.
func readTags(root string) (declared, sorted []string, err error) {
	path := filepath.Join(root, manifestFile)

	f, err := os.Open(path) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return nil, nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

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
		declared = append(declared, tag)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(declared) == 0 {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Str(path).Str(": no feature-gate tags found").String())
	}

	sorted = slices.Clone(declared)
	slices.Sort(sorted)

	return declared, sorted, nil
}

// rewriteGolangci replaces the `- <tag>` items under the `build-tags:` key,
// keeping their indentation and every other line.
func rewriteGolangci(content []byte, tags []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")

	key := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "build-tags:" {
			key = i
			break
		}
	}
	if key < 0 {
		return nil, errors.New(".golangci.yml: `build-tags:` key not found")
	}

	// The run of `- ` items immediately below the key, and their indentation.
	start, end, indent := key+1, key+1, ""
	for end < len(lines) {
		trimmed := strings.TrimLeft(lines[end], " ")
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if indent == "" {
			indent = lines[end][:len(lines[end])-len(trimmed)]
		}
		end++
	}
	if end == start {
		return nil, errors.New(".golangci.yml: no `- <tag>` item follows the build-tags key")
	}

	rebuilt := make([]string, 0, len(tags))
	for _, tag := range tags {
		var tb textbuf.Buffer
		rebuilt = append(rebuilt, tb.Str(indent).Str("- ").Str(tag).String())
	}

	out := slices.Clone(lines[:start])
	out = append(out, rebuilt...)
	out = append(out, lines[end:]...)

	return []byte(strings.Join(out, "\n")), nil
}

// rewriteGokrazy replaces the one `"GoBuildTags": [...],` line, keeping its
// indentation and its trailing comma.
func rewriteGokrazy(content []byte, tags []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")

	at := -1
	for i, line := range lines {
		if strings.Contains(line, "\"GoBuildTags\":") {
			at = i
			break
		}
	}
	if at < 0 {
		return nil, errors.New("gokrazy/ze/config.json: `GoBuildTags` key not found")
	}

	indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " \t"))]

	var tb textbuf.Buffer
	tb.Str(indent).Str("\"GoBuildTags\": [")
	for i, tag := range tags {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(strconv.Quote(tag))
	}
	tb.Str("],")
	lines[at] = tb.String()

	return []byte(strings.Join(lines, "\n")), nil
}

// quickstartMarker opens the go install command the quickstart publishes.
const quickstartMarker = "CGO_ENABLED=0 go install -tags '"

// rewriteQuickstart replaces the tag list inside the quickstart's go install
// command, keeping the module path and the version suffix after it.
func rewriteQuickstart(content []byte, tags []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")

	at := -1
	for i, line := range lines {
		if strings.Contains(line, quickstartMarker) {
			at = i
			break
		}
	}
	if at < 0 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("docs/guide/quickstart.md: `").Str(quickstartMarker).Str("` line not found").String())
	}

	edited, err := replaceQuoted(lines[at], quickstartMarker, tags)
	if err != nil {
		return nil, errors.New("docs/guide/quickstart.md: unterminated -tags quote")
	}
	lines[at] = edited

	return []byte(strings.Join(lines, "\n")), nil
}

// codeqlMarker opens each go build command the CodeQL workflow analyses.
const codeqlMarker = "CGO_ENABLED=0 go build -tags '"

// rewriteCodeQL replaces the tag lists in the workflow's two SHIPPED build
// lines: the first takes the distro combo and the second the appliance one. A
// build line whose list does not open with ze_core carries no gate tag and is
// left alone, which is what keeps the ze_setup build out of this.
//
// A file that does not hold exactly two such lines is an error. A workflow
// restructure then fails loudly, rather than silently dropping CodeQL coverage
// of the gated surface.
func rewriteCodeQL(content []byte, distro, appliance []string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	combos := [][]string{distro, appliance}

	found := 0
	for i, line := range lines {
		_, rest, holds := strings.Cut(line, codeqlMarker)
		if !holds {
			continue
		}
		inside, _, terminated := strings.Cut(rest, "'")
		if !terminated {
			return nil, errors.New("codeql.yml: unterminated -tags quote")
		}
		if !strings.HasPrefix(inside, coreTag) {
			continue // the ze_setup combo carries no gate tag
		}
		if found >= len(combos) {
			return nil, errors.New("codeql.yml: more than two ze_core `CGO_ENABLED=0 go build -tags` lines; update rewriteCodeQL")
		}
		edited, err := replaceQuoted(line, codeqlMarker, combos[found])
		if err != nil {
			return nil, errors.New("codeql.yml: unterminated -tags quote")
		}
		lines[i] = edited
		found++
	}
	if found != len(combos) {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("codeql.yml: found ").Int(int64(found)).
			Str(" ze_core `CGO_ENABLED=0 go build -tags` lines, want ").Int(int64(len(combos))).String())
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// replaceQuoted rewrites the single-quoted list that follows marker in line,
// keeping everything before the opening quote and everything from the closing
// quote onward.
func replaceQuoted(line, marker string, tags []string) (string, error) {
	start := strings.Index(line, marker) + len(marker)
	rest := line[start:]

	closing := strings.IndexByte(rest, '\'')
	if closing < 0 {
		return "", errors.New("unterminated quote")
	}

	var tb textbuf.Buffer
	tb.Str(line[:start])
	for i, tag := range tags {
		if i > 0 {
			tb.Byte(' ')
		}
		tb.Str(tag)
	}
	tb.Str(line[start+closing:])

	return tb.String(), nil
}

// edit is one derived file and the bytes it owes.
type edit struct {
	rel     string
	current []byte
	want    []byte
}

// derive answers what each of the four files must hold. A file that cannot be
// read, or whose anchor is gone, stops the run.
func derive(root string) ([]edit, error) {
	declared, sorted, err := readTags(root)
	if err != nil {
		return nil, err
	}

	all := targets(declared, sorted)
	edits := make([]edit, 0, len(all))

	for _, one := range all {
		path := filepath.Join(root, one.rel)

		current, readErr := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
		if readErr != nil {
			return nil, readErr
		}

		want, rewriteErr := one.rewrite(current)
		if rewriteErr != nil {
			// Named twice on purpose: the inner message names the file by
			// convention and this names the one actually edited, which is what
			// tells a reader WHICH tree the anchor is missing from.
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(filepath.ToSlash(one.rel)).Str(": ").Err(rewriteErr).String())
		}

		edits = append(edits, edit{rel: filepath.ToSlash(one.rel), current: current, want: want})
	}

	return edits, nil
}

// Check reports every derived file whose tag list disagrees with the manifest.
// It writes nothing.
func Check(root string) (CheckReport, error) {
	edits, err := derive(root)
	if err != nil {
		return CheckReport{}, err
	}

	var report CheckReport
	for _, one := range edits {
		if !bytes.Equal(one.current, one.want) {
			report.Stale = append(report.Stale, one.rel)
		}
	}

	return report, nil
}

// Write brings every derived file up to date, and answers the ones it changed.
func Write(root string) (WriteReport, error) {
	edits, err := derive(root)
	if err != nil {
		return WriteReport{}, err
	}

	var report WriteReport
	for _, one := range edits {
		if bytes.Equal(one.current, one.want) {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(one.rel)), one.want, 0o644); err != nil { //nolint:gosec // repo files, read by everyone who reads the tree
			return WriteReport{}, err
		}
		report.Updated = append(report.Updated, one.rel)
	}

	return report, nil
}
