// Design: docs/architecture/core-design.md -- the added-symbol wiring check
// Overview: docwiring.go -- the gate that runs this check
// Detail: allowlist.go -- the symbols this check deliberately exempts
//
// wiring.go checks whether every ADDED exported symbol has a non-test reference
// in internal/ or cmd/. An unreferenced symbol is dead or unwired. The change
// that adds it is the cheapest place to find either defect.
//
// The reader deliberately uses lines instead of a Go parser. It reads both HEAD
// and the working file. During a rename, the HEAD copy does not always parse.
// A parser failure would hide the change that this check must inspect.

package docwiring

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Symbol is one exported declaration a file makes.
type Symbol struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

var (
	funcRe      = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(`)
	typeRe      = regexp.MustCompile(`^type\s+([A-Z][A-Za-z0-9_]*)\b`)
	constRe     = regexp.MustCompile(`^const\s+(.+)$`)
	varRe       = regexp.MustCompile(`^var\s+(.+)$`)
	blockRe     = regexp.MustCompile(`^(type|const|var)\s*\(`)
	identListRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\b`)
)

// exportedSymbols answers every exported declaration in one file's content.
func exportedSymbols(path, content string) []Symbol {
	var symbols []Symbol
	blockKind := ""

	for i, line := range strings.Split(content, "\n") {
		lineNo := i + 1
		code := strings.TrimSpace(stripComment(line))
		if code == "" {
			continue
		}

		if blockKind != "" {
			if strings.HasPrefix(code, ")") {
				blockKind = ""
				continue
			}
			for _, ident := range leadingExportedIdents(code) {
				symbols = append(symbols, Symbol{Path: path, Line: lineNo, Kind: blockKind, Name: ident})
			}
			continue
		}

		if m := blockRe.FindStringSubmatch(code); m != nil {
			blockKind = m[1]
			continue
		}
		if m := funcRe.FindStringSubmatch(code); m != nil {
			symbols = append(symbols, Symbol{Path: path, Line: lineNo, Kind: "func", Name: m[1]})
			continue
		}
		if m := typeRe.FindStringSubmatch(code); m != nil {
			symbols = append(symbols, Symbol{Path: path, Line: lineNo, Kind: "type", Name: m[1]})
			continue
		}
		if m := constRe.FindStringSubmatch(code); m != nil {
			for _, ident := range leadingExportedIdents(m[1]) {
				symbols = append(symbols, Symbol{Path: path, Line: lineNo, Kind: "const", Name: ident})
			}
			continue
		}
		if m := varRe.FindStringSubmatch(code); m != nil {
			for _, ident := range leadingExportedIdents(m[1]) {
				symbols = append(symbols, Symbol{Path: path, Line: lineNo, Kind: "var", Name: ident})
			}
		}
	}
	return symbols
}

// stripComment removes text after the first `//` on a line. Thus, a comment
// name is not a declaration or reference.
func stripComment(line string) string {
	code, _, _ := strings.Cut(line, "//")
	return code
}

// leadingExportedIdents answers the exported names in the comma-separated
// identifier list a declaration line opens with.
func leadingExportedIdents(code string) []string {
	m := identListRe.FindStringSubmatch(code)
	if m == nil {
		return nil
	}
	var names []string
	for part := range strings.SplitSeq(m[1], ",") {
		ident := strings.TrimSpace(part)
		if ident != "" && ident[0] >= 'A' && ident[0] <= 'Z' {
			names = append(names, ident)
		}
	}
	return names
}

// checkWiring answers one issue line per exported symbol this change adds that
// no non-test file in internal/ or cmd/ names.
//
// baseline answers a path's content at the change's baseline, which is HEAD for
// the running gate and a fixture for a test.
func checkWiring(root string, changed []string, baseline func(string) string) ([]string, error) {
	// A pure relocation (a rename, or a tier move) deletes a file and re-adds
	// its exported symbols at a new path. Those names are pre-existing API, not
	// new, so a behavior-preserving move must contribute zero added symbols.
	// Without this, relocating a package surfaces every unwired helper it
	// carries as a false addition, because the baseline at the NEW path is
	// empty.
	relocated := make(map[string]bool)
	for _, path := range changed {
		if !isWiringSource(path) {
			continue
		}
		current, err := readCurrentOrEmpty(root, path)
		if err != nil {
			return nil, err
		}
		if current != "" {
			continue // still on disk: not a deletion
		}
		for _, sym := range exportedSymbols(path, baseline(path)) {
			relocated[sym.Name] = true
		}
	}

	var added []Symbol
	for _, path := range changed {
		if !isWiringSource(path) {
			continue
		}
		current, err := readCurrentOrEmpty(root, path)
		if err != nil {
			return nil, err
		}
		if current == "" {
			continue
		}
		old := make(map[string]bool)
		for _, sym := range exportedSymbols(path, baseline(path)) {
			old[sym.Name] = true
		}
		for _, sym := range exportedSymbols(path, current) {
			if old[sym.Name] || relocated[sym.Name] {
				continue
			}
			if wiringAllowlist[allowKey{Path: sym.Path, Name: sym.Name}] {
				continue
			}
			added = append(added, sym)
		}
	}

	var issues []string
	for _, sym := range added {
		// The repository-wide wiring gate uses the same explicit convention: an
		// exported *ForTest helper exists for a cross-package test, not for a
		// production caller. Do not require a fake production reference here.
		if strings.HasSuffix(sym.Name, "ForTest") {
			continue
		}
		if hasProductionReference(root, sym) {
			continue
		}
		var tb textbuf.Buffer
		issues = append(issues, tb.Str(sym.Path).Byte(':').Int(int64(sym.Line)).
			Str(": exported ").Str(sym.Kind).Byte(' ').Str(sym.Name).
			Str(" has no non-test reference in internal/ or cmd/").String())
	}
	return issues, nil
}

// hasProductionReference reports whether any non-test Go file under internal/
// or cmd/ names the symbol, on a line other than the one declaring it.
func hasProductionReference(root string, sym Symbol) bool {
	var tb textbuf.Buffer
	token := regexp.MustCompile(tb.Str(`\b`).Str(regexp.QuoteMeta(sym.Name)).Str(`\b`).String())

	found := false
	for _, base := range [...]string{"internal", "cmd"} {
		if found {
			break
		}
		basePath := filepath.Join(root, base)
		if _, err := os.Stat(basePath); err != nil {
			continue
		}
		// Every read failure leaves the answer FALSE and makes the symbol unwired.
		// This is the safe direction for this check. Other package readers report
		// errors, but this caller must not pass because it cannot open a file.
		_ = filepath.WalkDir(basePath, func(path string, entry os.DirEntry, err error) error {
			if err != nil || found || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil //nolint:nilerr // a file this walk cannot reach holds no reference it can prove
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil //nolint:nilerr // a path outside the tree names no reference in it
			}
			rel = filepath.ToSlash(rel)
			if strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			raw, readErr := os.ReadFile(path) //nolint:gosec // path comes from a walk of the tree the caller named
			if readErr != nil {
				return nil //nolint:nilerr // an unreadable file proves no reference, which reddens rather than passes
			}
			for lineNo, line := range strings.Split(string(raw), "\n") {
				if rel == sym.Path && lineNo+1 == sym.Line {
					continue
				}
				if token.MatchString(stripComment(line)) {
					found = true
					return filepath.SkipAll
				}
			}
			return nil
		})
	}
	return found
}

// readCurrentOrEmpty answers a path's content in the working tree, or "" when
// the working tree does not hold it.
//
// An ABSENT file returns "" without an error. This is how a caller sees a file
// that the change deleted or has not written. A present UNREADABLE file returns
// an error. Otherwise, an unreadable file would look unadded and avoid every
// required check.
func readCurrentOrEmpty(root, path string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // a repository path the caller named
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(raw), nil
}
