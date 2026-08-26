// Design: docs/architecture/core-design.md -- the generated package map
// Detail: report.go -- what the command answers and the two halves that write it
// Detail: sources.go -- which changed file can drift the index
//
// Package discoveryindex generates one ai/PACKAGE-MAP.md line for each Go
// package. The responsibility text comes from the package doc comment first.
// Otherwise, it comes from the plugin registry Description in register.go.
// Packages with neither source get `TODO`, which makes the map a documentation
// coverage worklist.
//
// Nothing here is hand-maintained: the map is regenerated from the tree, and a
// stale map fails `make ze-discovery-index-check`.
//
// It also answers WHICH changed files can make that map outdated (see
// sources.go). Python kept that answer in another module because a gate cannot
// import a generator reached through `go run`. A compiled package has no such
// problem. Thus, the generator and source rule now hold one fact together
// instead of two forms that can disagree.
package discoveryindex

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// OutputRel is the index this tool writes, relative to the checkout.
	OutputRel = "ai/PACKAGE-MAP.md"

	// StaleExit means that the committed output no longer matches its sources.
	// It differs from 1, which means that the generator failed because of a
	// missing directory, minimal checkout, or crash. The commit gate BLOCKS on
	// drift but stays warn-only when the generator fails. It once distinguished
	// them by matching the human-facing warning. A wording change then made a
	// BLOCKING gate warn-only. The exit code is the contract, and the warning
	// text CAN change.
	StaleExit = 3

	// HeaderLines limits the search for the `// Package` block in a file. It is
	// exported because the Python half states the same CONTRACT value. Output
	// cannot reveal a difference because neither half finds a later header.
	HeaderLines = 40

	// MaxSummary bounds a responsibility line in CHARACTERS, not bytes. Python
	// len() counts code points. A byte-counting port would cut a summary with a
	// non-ASCII character at another place and write a different file.
	MaxSummary = 200

	// maxLineBytes bounds one line of a source file. It is the same bound the
	// verify log reader uses, and three orders of magnitude above the longest
	// generated line in this tree.
	maxLineBytes = 4 << 20
)

// roots are the trees the map covers. Roots publishes them.
//
// `letools` is le's plugin tree. It is a top-level tree because le and ze are
// different programs over the same engine. No build links their packages. An
// omitted tree is absent from the map that an agent reads.
var roots = [...]string{"internal", "pkg", "cmd", "letools"}

// skipDirs are directory names no root is walked into.
var skipDirs = map[string]bool{
	"vendor":       true,
	"tmp":          true,
	"testdata":     true,
	"node_modules": true,
	".git":         true,
}

var (
	// descRe reads a one-line string value out of a register.go
	// Registration{...} struct literal.
	descRe = regexp.MustCompile(`Description:\s*"((?:[^"\\]|\\.)*)"`)
	nameRe = regexp.MustCompile(`\bName:\s*"((?:[^"\\]|\\.)*)"`)
	// nameConstRe covers the other spelling. Some register.go files use a
	// package constant for `Name:` because a doctor check and filter
	// registration read the same spelling. Seven files use this form. A
	// literal-only pattern left their plugin-name column blank. This pattern
	// runs only when the file has no quoted `Name:`. Several files declare a
	// CLI command before plugin registration, so the first form CAN be the
	// command name instead of the plugin name.
	nameConstRe = regexp.MustCompile(`\bName:\s*([A-Za-z_]\w*)`)
)

// Package is one row of the map: where the package sits, what it does, and the
// name it registers under where it has a register.go.
type Package struct {
	Path           string `json:"path"`
	Responsibility string `json:"responsibility"`
	Registered     string `json:"registered"`
}

// Build reads the tree and answers one entry per Go package, sorted by path.
//
// Every read error reaches the caller. If an unreadable file looks like a
// package with no declaration, the generator writes that omission into the
// index. The check then agrees with that index and cannot detect the omission.
// Only a path that VANISHED during the walk is not a failure. This checkout is
// shared, so another session can remove a file that was never in the described
// tree.
func Build(root string) ([]Package, error) {
	packages := make(map[string]Package)
	for _, top := range roots {
		base := filepath.Join(root, top)
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			// A tree that holds no such root is a smaller checkout, which is a
			// fact about the tree rather than a scan that fell short. The
			// Python half skips it the same way.
			continue
		}
		if err := scanDir(root, base, packages); err != nil {
			return nil, err
		}
	}

	out := make([]Package, 0, len(packages))
	for _, pkg := range packages {
		out = append(out, pkg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// scanDir reads one directory, records the package it holds, and descends.
func scanDir(root, dir string, packages map[string]Package) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if vanished(dir, err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	var gofiles, subdirs []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
			subdirs = append(subdirs, name)
			continue
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			gofiles = append(gofiles, name)
		}
	}

	if err := recordPackage(root, dir, gofiles, packages); err != nil {
		return err
	}

	sort.Strings(subdirs)
	for _, name := range subdirs {
		if err := scanDir(root, filepath.Join(dir, name), packages); err != nil {
			return err
		}
	}
	return nil
}

// recordPackage adds the one entry a directory of Go files contributes.
//
// A directory that contains only `embed.go` contributes no entry. A pure
// `//go:embed` package, such as a plugin yang/ schema directory, needs no
// one-line description. Its omission keeps the TODO column focused on package
// documentation.
func recordPackage(root, dir string, gofiles []string, packages map[string]Package) error {
	if len(gofiles) == 0 {
		return nil
	}
	if len(gofiles) == 1 && gofiles[0] == "embed.go" {
		return nil
	}

	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return fmt.Errorf("locating %s under %s: %w", dir, root, err)
	}
	rel = filepath.ToSlash(rel)

	doc := ""
	for _, name := range ordered(gofiles) {
		doc, err = packageDoc(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if doc != "" {
			break
		}
	}

	name, desc := "", ""
	if slices.Contains(gofiles, "register.go") {
		if name, desc, err = registration(filepath.Join(dir, "register.go")); err != nil {
			return err
		}
	}

	responsibility := doc
	if responsibility == "" {
		responsibility = desc
	}
	if responsibility == "" {
		responsibility = "TODO"
	}
	packages[rel] = Package{Path: rel, Responsibility: responsibility, Registered: name}
	return nil
}

// ordered answers the package files in read order. doc.go comes first because
// packages use it for documentation, and all other files follow by name.
func ordered(gofiles []string) []string {
	rest := make([]string, 0, len(gofiles))
	for _, name := range gofiles {
		if name != "doc.go" {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	if slices.Contains(gofiles, "doc.go") {
		return append([]string{"doc.go"}, rest...)
	}
	return rest
}

// packageDoc answers the first sentence of the `// Package ...` comment, joined
// across the `//` lines that continue it.
func packageDoc(path string) (string, error) {
	lines, err := head(path, HeaderLines)
	if err != nil {
		return "", err
	}

	for i, line := range lines {
		rest, ok := packageLine(strings.TrimSpace(line))
		if !ok {
			continue
		}
		parts := []string{strings.TrimSpace(rest)}
		for _, cont := range lines[i+1:] {
			body, ok := commentBody(strings.TrimSpace(cont))
			if !ok {
				break
			}
			parts = append(parts, strings.TrimSpace(body))
			if strings.Contains(body, ".") {
				break
			}
		}
		kept := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				kept = append(kept, part)
			}
		}
		return firstSentence(strings.Join(kept, " ")), nil
	}
	return "", nil
}

// packageLine reads a `// Package <name> <text>` header line and answers the
// text after the package name. A header with no text after the name is not one:
// there is no sentence to summarize.
func packageLine(line string) (string, bool) {
	rest, ok := commentMarker(line)
	if !ok {
		return "", false
	}
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	rest, ok = strings.CutPrefix(rest, "Package")
	if !ok {
		return "", false
	}
	trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
	if trimmed == rest {
		return "", false // no separator: `Packagefoo`, not a header
	}
	// The package name is one run of non-space characters, and the summary is
	// what follows the space after it.
	idx := strings.IndexFunc(trimmed, unicode.IsSpace)
	if idx <= 0 {
		return "", false
	}
	return strings.TrimLeftFunc(trimmed[idx:], unicode.IsSpace), true
}

// commentBody answers the text of a `//` line, dropping at most one space after
// the marker. A line that is not a comment ends the header block.
func commentBody(line string) (string, bool) {
	rest, ok := commentMarker(line)
	if !ok {
		return "", false
	}
	if r, size := utf8.DecodeRuneInString(rest); size > 0 && unicode.IsSpace(r) {
		return rest[size:], true
	}
	return rest, true
}

func commentMarker(line string) (string, bool) {
	return strings.CutPrefix(line, "//")
}

// firstSentence normalises whitespace, keeps what precedes the first sentence
// break, and bounds the result at MaxSummary CHARACTERS.
func firstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if idx := strings.Index(text, ". "); idx != -1 {
		text = text[:idx]
	} else if trimmed, ok := strings.CutSuffix(text, "."); ok {
		text = trimmed
	}

	if utf8.RuneCountInString(text) > MaxSummary {
		runes := []rune(text)
		// The cut is the last space strictly before the bound, so a summary is
		// never truncated inside a word. Its absence leaves the bound itself.
		cut := -1
		for i := 0; i < MaxSummary-1 && i < len(runes); i++ {
			if runes[i] == ' ' {
				cut = i
			}
		}
		end := MaxSummary - 1
		if cut > 0 {
			end = cut
		}
		var tb textbuf.Buffer
		text = tb.Str(strings.TrimRightFunc(string(runes[:end]), unicode.IsSpace)).Str("...").String()
	}
	return strings.TrimSpace(text)
}

// registration answers the registered name and the Description a register.go
// declares.
func registration(path string) (string, string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		if vanished(path, err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("reading %s: %w", path, err)
	}
	text := string(raw)

	desc := ""
	if m := descRe.FindStringSubmatch(text); m != nil {
		desc = m[1]
	}
	if m := nameRe.FindStringSubmatch(text); m != nil {
		return m[1], desc, nil
	}
	if m := nameConstRe.FindStringSubmatch(text); m != nil {
		return constValue(text, m[1]), desc, nil
	}
	return "", desc, nil
}

// constValue answers the value of a same-file string constant, or "" when it is
// not one.
//
// It covers the three Go spellings that carry a plugin name: `const X = "v"`,
// `const X string = "v"`, and a bare `X = "v"` line inside a `const ( ... )`
// block. A constant defined in another file resolves to "", which leaves the
// column empty exactly as an unrecognized value does.
func constValue(text, ident string) string {
	var tb textbuf.Buffer
	pattern := tb.Str(`(?m)^[ \t]*(?:const[ \t]+)?`).Str(regexp.QuoteMeta(ident)).
		Str(`(?:[ \t]+[\w.\[\]*]+)?[ \t]*=[ \t]*"((?:[^"\\]|\\.)*)"`).String()
	re, err := regexp.Compile(pattern)
	if err != nil {
		// QuoteMeta makes the identifier literal, so the pattern is well formed
		// for every identifier the scanner can produce. Answering "" leaves the
		// column empty rather than failing the whole map on one register.go.
		return ""
	}
	if m := re.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// head answers the first n lines of a file, without their line endings.
//
// The scanner accepts lines longer than its default token. Generated Go files
// can contain such lines, and an early scan stop would leave the header
// incomplete. Both failures reach the caller.
func head(path string, n int) ([]string, error) {
	file, err := os.Open(path) //nolint:gosec // path comes from a walk of the tree the caller named
	if err != nil {
		if vanished(path, err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close() //nolint:errcheck // read-only

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	lines := make([]string, 0, n)
	for len(lines) < n && scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return lines, nil
}

// vanished reports a path that disappeared during the walk. This checkout is
// shared, so another session can remove a file between the listing and read.
// That file was not part of the described tree. The generator does not report
// another session's timing as a defect.
func vanished(path string, err error) bool {
	if !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	_, statErr := os.Lstat(path)
	return errors.Is(statErr, fs.ErrNotExist)
}

// areaOf answers the heading a package sits under.
func areaOf(rel string) string {
	if strings.HasPrefix(rel, "internal/component/bgp/plugins/") {
		return "internal/component/bgp/plugins"
	}
	if strings.HasPrefix(rel, "internal/component/bgp/") {
		return "internal/component/bgp"
	}
	parts := strings.Split(rel, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "/")
	}
	return rel
}

// Render answers the whole file the index holds, ending in a newline.
func Render(packages []Package) string {
	var tb textbuf.Buffer
	tb.Str("# Package Map\n\n")
	tb.Str("<!-- GENERATED by scripts/dev/package_map.py -- do not edit -->\n")
	tb.Str("<!-- Regenerate: make ze-discovery-index-update -->\n\n")
	tb.Str("One line per Go package under `internal/`, `pkg/`, `cmd/`, `letools/`.\n")
	tb.Str("Responsibility comes from the `// Package` doc comment, else the plugin\n")
	tb.Str("registry `Description`, else `TODO` (a package that still needs a doc\n")
	tb.Str("comment). `Registered` is the name the package registers under, where it\n")
	tb.Str("has a register.go. Design docs per file: `ai/DOCS-TO-CODE.md`.\n")

	current := ""
	first := true
	for _, pkg := range packages {
		area := areaOf(pkg.Path)
		if first || area != current {
			current = area
			first = false
			tb.Str("\n## `").Str(area).Str("/`\n\n")
			tb.Str("| Package | Responsibility | Registered |\n")
			tb.Str("|---------|----------------|------------|\n")
		}
		tb.Str("| `").Str(pkg.Path).Str("` | ").Str(escapePipes(pkg.Responsibility)).
			Str(" | ").Str(pkg.Registered).Str(" |\n")
	}
	return tb.String()
}

// escapePipes protects a table cell from a responsibility line holding a bar.
func escapePipes(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}

// Roots answers the trees this map covers, in the order they are walked. It is
// derived from the same table the walk reads, so a comparison against the
// Python half's list cannot be satisfied by a second copy.
func Roots() []string { return slices.Clone(roots[:]) }

// SkipDirs answers the directory names no root is walked into, sorted. The
// caller receives a copy, so a reader cannot widen the walk by editing what it
// was handed.
func SkipDirs() []string {
	out := make([]string, 0, len(skipDirs))
	for name := range skipDirs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
