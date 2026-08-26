// Design: docs/architecture/core-design.md -- the consistency gate, as a command
//
// Package consistency is the `ze-consistency-check` gate: it reads the tree and
// reports where the code and the documentation disagree with each other.
//
// Seven checks run over one walk of the tree:
//
//   - JSON struct tags are kebab-case, not snake_case or camelCase;
//   - plugin.Response carries a Status* constant, not a string literal;
//   - a non-exempt file under internal/, pkg/ or cmd/ carries a // Design: line;
//   - a // Detail:, // Overview: or // Related: cross-reference resolves, and the
//     file it names points back;
//   - no source file passes 1000 lines;
//   - every BGP command plugin carries doc.go, schema/ and dispatch_test.go;
//   - a comma split that keeps its slice uses stringsx.SplitCount.
//
// The tool answers a Report (report.go) rather than printing one. That is what
// lets `| json` feed a script and `| match` keep one check's rows, and it is why
// nothing here writes a line of JSON, YAML or table code.
//
// The walk is bounded by the filesystem: filepath.Walk does not follow
// symbolic links, so a linked tree is visited once and a link cycle cannot make
// it repeat.

package consistency

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/lepath"
)

// fileSizeMax is the ONLY size threshold (ai/rules/go-standards.md, Thomas
// 2026-08-01). A 600-line WARN used to sit below this. It fired on cohesive
// single-concern files.
const fileSizeMax = 1000

// checker holds one run: the tree it reads and what it found in it. The
// findings were package state while this was a script, which made the tool
// unusable twice in one process and untestable without forking it.
type checker struct {
	root     string
	findings []Finding
	errors   int
	warnings int
}

// Check reads the tree at root and answers everything the seven checks found.
// A finding's File is relative to root, so the answer does not change with the
// working directory the caller happens to have.
func Check(root string) Report {
	c := &checker{root: root, findings: []Finding{}}

	// The order is the order the report groups its findings in, so it is part
	// of what a reader sees rather than an implementation detail.
	c.checkJSONTags()
	c.checkStatusConstants()
	c.checkDesignRefs()
	c.checkCrossRefs()
	c.checkFileSizes()
	c.checkPluginStructure()
	c.checkSplitCountUsage()

	return Report{Findings: c.findings, Errors: c.errors, Warnings: c.warnings}
}

// Answer is the `le consistency` command. It takes no arguments: the tree is
// the checkout, and the rendering is the operator's to choose with a pipe
// operator (ai/rules/cli.md).
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "error: consistency takes no arguments, got %q\n", args[0]) //nolint:errcheck // CLI output
		fmt.Fprintln(os.Stderr, "usage: le consistency [| json | yaml | table]")           //nolint:errcheck // CLI output
		return nil, 1
	}

	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	report := Check(root)
	if report.Errors > 0 {
		return report, 1
	}
	return report, 0
}

// report records one finding and keeps the two counts the verdict reads.
func (c *checker) report(severity, check, file string, line int, message string) {
	c.findings = append(c.findings, Finding{
		Severity: severity, Check: check, File: file, Line: line, Message: message,
	})
	if severity == SeverityError {
		c.errors++
		return
	}
	c.warnings++
}

// path answers the absolute path of a file the walk reported.
func (c *checker) path(rel string) string {
	return filepath.Join(c.root, rel)
}

// --- JSON Tag Check ---

var (
	jsonTagRE   = regexp.MustCompile(`json:"([^"]+)"`)
	snakeCaseRE = regexp.MustCompile(`[a-z]_[a-z]`)
	camelCaseRE = regexp.MustCompile(`[a-z][A-Z]`)
)

func (c *checker) checkJSONTags() {
	c.walkGoFiles(func(path string) {
		// Skip test files and research code.
		if isTestFile(path) {
			return
		}
		c.scanLines(path, func(line int, text string) {
			matches := jsonTagRE.FindAllStringSubmatch(text, -1)
			for _, m := range matches {
				tag, _, _ := strings.Cut(m[1], ",") // strip ,omitempty etc.
				if tag == "-" || tag == "" {
					continue
				}
				if snakeCaseRE.MatchString(tag) {
					var tb textbuf.Buffer
					c.report(SeverityError, "json-kebab-case", path, line,
						tb.Str("snake_case JSON tag ").Quoted(tag).Str(" — use kebab-case").String())
				}
				if camelCaseRE.MatchString(tag) {
					var tb textbuf.Buffer
					c.report(SeverityError, "json-kebab-case", path, line,
						tb.Str("camelCase JSON tag ").Quoted(tag).Str(" — use kebab-case").String())
				}
			}
		})
	})
}

// --- Status Constants Check ---

var statusLiteralRE = regexp.MustCompile(`Status:\s*"(done|error|ok)"`)

func (c *checker) checkStatusConstants() {
	c.walkGoFiles(func(path string) {
		c.scanLines(path, func(line int, text string) {
			if m := statusLiteralRE.FindStringSubmatch(text); m != nil {
				var tb textbuf.Buffer
				c.report(SeverityWarn, "status-constants", path, line,
					tb.Str("hardcoded Status: ").Quoted(m[1]).Str(" — use plugin.Status* constant").String())
			}
		})
	})
}

// --- Design Doc References ---

func (c *checker) checkDesignRefs() {
	c.walkGoFiles(func(path string) {
		if isExemptFile(path) || isTestFile(path) {
			return
		}
		// Only check files under internal/, pkg/, cmd/.
		if !strings.Contains(path, "internal/") && !strings.Contains(path, "pkg/") && !strings.Contains(path, "cmd/") {
			return
		}
		hasDesign := false
		c.scanLines(path, func(_ int, text string) {
			if strings.Contains(text, "// Design:") {
				hasDesign = true
			}
		})
		if !hasDesign {
			c.report(SeverityWarn, "design-refs", path, 0, "missing // Design: comment")
		}
	})
}

// --- Cross-Reference Bidirectionality ---

var xrefRE = regexp.MustCompile(`// (Detail|Overview|Related): (\S+\.go)`)

// ref is one cross-reference comment: what it claims, and where it says it.
type ref struct {
	keyword string
	target  string
	line    int
}

func (c *checker) checkCrossRefs() {
	// Collect all refs: map[dir][file] -> list of (keyword, target).
	dirRefs := map[string]map[string][]ref{}

	c.walkGoFiles(func(path string) {
		if isExemptFile(path) || isTestFile(path) {
			return
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		c.scanLines(path, func(line int, text string) {
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

	// Sorted rather than in map order. Two runs of a gate over one tree must
	// answer one thing, and a report whose 1154 lines shuffle between runs
	// cannot be diffed, reviewed, or ratcheted.
	for _, dir := range sortedKeys(dirRefs) {
		fileRefs := dirRefs[dir]
		for _, source := range sortedKeys(fileRefs) {
			for _, r := range fileRefs[source] {
				// Check target file exists.
				targetPath := filepath.Join(dir, r.target)
				if _, err := os.Stat(c.path(targetPath)); os.IsNotExist(err) {
					var tb textbuf.Buffer
					c.report(SeverityError, "cross-refs", filepath.Join(dir, source), r.line,
						tb.Str("stale ref to ").Str(r.target).Str(" (file does not exist)").String())
					continue
				}
				// An exempt target (doc.go and its siblings) needs no back-reference.
				targetBase := filepath.Base(r.target)
				if isExemptFilename(targetBase) {
					continue
				}
				// The back-reference must exist. A target CAN sit in a
				// subdirectory, as "show/main.go" does, so resolve its own
				// directory and read that directory's references.
				expectedKW := inverse[r.keyword]
				targetDir := filepath.Dir(targetPath)
				targetFileRefs := dirRefs[targetDir]
				// The target's back-reference MUST point at source. When the
				// two sit in different directories, that back-reference
				// path must be relative from target's directory to source.
				expectedSource := source
				if targetDir != dir {
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
					var tb textbuf.Buffer
					c.report(SeverityWarn, "cross-refs", targetPath, 0,
						tb.Str("missing ").Str(expectedKW).Str(": ").Str(expectedSource).
							Str(" (referenced by ").Str(filepath.Join(dir, source)).
							Str(" with ").Str(r.keyword).Str(":)").String())
				}
			}
		}
	}
}

// sortedKeys answers a map's keys in order, which is what makes a walk over it
// repeatable.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// --- SplitCount Usage Check ---

var (
	commaSplitRE = regexp.MustCompile(`strings\.Split\(.*,\s*","`)
	commaCountRE = regexp.MustCompile(`strings\.Count\(.*,\s*","`)
)

func (c *checker) checkSplitCountUsage() {
	c.walkGoFiles(func(path string) {
		if strings.Contains(path, "internal/core/stringsx/") {
			return
		}
		c.scanLines(path, func(line int, text string) {
			if commaCountRE.MatchString(text) && strings.Contains(text, "+1") {
				c.report(SeverityWarn, "split-count", path, line,
					"comma split capacity counted separately -- use stringsx.SplitCount")
				return
			}
			if commaSplitRE.MatchString(text) {
				c.report(SeverityWarn, "split-count", path, line,
					"comma strings.Split materializes via a pre-count -- use stringsx.SplitCount when keeping the slice")
			}
		})
	})
}

// --- File Size Check ---

func (c *checker) checkFileSizes() {
	c.walkGoFiles(func(path string) {
		if isTestFile(path) {
			return
		}
		lines := c.countLines(path)
		if lines > fileSizeMax {
			var tb textbuf.Buffer
			c.report(SeverityError, "file-size", path, 0,
				tb.Int(int64(lines)).Str(" lines (max ").Int(int64(fileSizeMax)).Byte(')').String())
		}
	})
}

// --- Plugin Structure Check ---

func (c *checker) checkPluginStructure() {
	const cmdDir = "internal/component/bgp/plugins/cmd"
	cmdEntries, err := os.ReadDir(c.path(cmdDir))
	if err != nil {
		// A tree that has no BGP command plugins has nothing to check here,
		// which is the case for every fixture and for a checkout of a
		// different project. It is not a failure to read one that does exist:
		// the walk above reports that.
		return
	}

	for _, e := range cmdEntries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(cmdDir, e.Name())
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
			if _, err := os.Stat(c.path(target)); os.IsNotExist(err) {
				var tb textbuf.Buffer
				c.report(SeverityError, "plugin-structure", dir, 0,
					tb.Str("missing ").Str(req.pattern).Str(" (").Str(req.desc).Byte(')').String())
			}
		}

		// Check for dispatch_test.go (wiring test).
		if !fileExists(c.path(filepath.Join(dir, "dispatch_test.go"))) {
			var tb textbuf.Buffer
			c.report(SeverityWarn, "plugin-structure", dir, 0,
				tb.Str(name).Str(": missing dispatch_test.go (wiring test)").String())
		}
	}
}

// --- Helpers ---

// walkGoFiles calls fn with every .go file under the tree, named relative to
// its root.
//
// A path the walk cannot read is REPORTED and the walk continues. Returning nil
// on the error instead -- which is what this did while it was a script -- drops
// every file under an unreadable directory, and a check that reads no file
// finds nothing, which is indistinguishable from a clean tree.
func (c *checker) walkGoFiles(fn func(path string)) {
	_ = filepath.Walk(c.root, func(path string, info os.FileInfo, err error) error {
		rel, relErr := filepath.Rel(c.root, path)
		if relErr != nil {
			rel = path
		}
		if err != nil {
			c.unreadable(rel, 0, "cannot read, so no check ran over this path", err)
			return nil
		}
		// Skip hidden dirs (except the root itself), vendor, research, caches, tmp.
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" || base == "research" || base == "modcache" || base == "tmp" {
				return filepath.SkipDir
			}
			if base == "gokrazy" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(base, ".") && path != c.root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			fn(filepath.ToSlash(rel))
		}
		return nil
	})
}

// unreadable reports a file this tool could not read in full.
//
// A skipped file is the fail-open shape: every check over it then finds
// nothing, and nothing distinguishes that from a clean file.
func (c *checker) unreadable(path string, line int, what string, err error) {
	var tb textbuf.Buffer
	c.report(SeverityError, "unreadable", path, line, tb.Str(what).Str(": ").Err(err).String())
}

// scanLines feeds every line of path to fn.
//
// Scan returns false on EOF, on a read error, and on a line above
// bufio.MaxScanTokenSize alike. A gate that treats "loop ended" as "file
// finished" checks the head of a file and calls the whole of it clean, so the
// error is read back and reported.
func (c *checker) scanLines(path string, fn func(line int, text string)) {
	f, err := os.Open(c.path(path))
	if err != nil {
		c.unreadable(path, 0, "cannot open, so no check ran over this file", err)
		return
	}
	defer f.Close() //nolint:errcheck // read-only
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		fn(lineNum, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		c.unreadable(path, lineNum, "read stopped early, so the rest of this file was not checked", err)
	}
}

// countLines returns the line count of path, and -1 when the file cannot be
// read in full. A partial count is a smaller number, which is the direction
// that passes the file-size check without saying anything.
func (c *checker) countLines(path string) int {
	f, err := os.Open(c.path(path))
	if err != nil {
		c.unreadable(path, 0, "cannot open, so file size was not checked", err)
		return -1
	}
	defer f.Close() //nolint:errcheck // read-only
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}
	if err := scanner.Err(); err != nil {
		c.unreadable(path, n, "read stopped early, so file size was not checked", err)
		return -1
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
