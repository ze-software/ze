// Design: docs/architecture/core-design.md -- the source-anchor reverse index
// Overview: docstocode.go -- the mirror index, from `// Design:` headers
// Detail: codetodocs_report.go -- what this answers
//
// codetodocs.go builds the MIRROR of the package's other index. That index maps
// `// Design:` headers to documents. This index maps `<!-- source: -->` anchors
// in docs/ to files.
//
// The index also reports two forms of claims that nobody can check. A stale
// path names a pointer that nobody can follow. A description that names an
// undeclared symbol makes a claim that nobody can verify. Both use the same
// report terms.
//
// The scan is TEXT rather than a type check: it has no build context, so a
// declaration behind `//go:build linux` is found on any host. It reads
// declarations only -- funcs, methods, types, vars, consts, struct fields,
// interface methods -- and never a function body.

package docstocode

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CodeOutputRel is where the reverse index is written, relative to the tree.
// It is gitignored, so it never appears in `git status` and no committed copy
// can be stale.
const CodeOutputRel = "ai/CODE-TO-DOCS.md"

// docsDir is the tree the anchors are read from.
const docsDir = "docs"

// namedRefs limits the documents named for a stale path before the report
// counts the rest. namedInline limits the package files listed as bullets
// before the renderer uses a table.
const (
	namedRefs   = 3
	namedInline = 3
)

// pathPrefixes are the roots that a source anchor CAN reference. A
// comma-separated token without one is relative to the previous full path's
// directory. Thus, an anchor names two files in one package and does not repeat
// the directory.
var pathPrefixes = []string{
	"go.mod", "internal/", "cmd/", "pkg/", "test/", "rfc/",
}

// These expressions define the scan. Each const is one string literal because
// ai/rules/performance.md bans pattern concatenation. A single literal also
// keeps a regular expression readable.
const (
	// anchorPattern captures the whole content of one source anchor. It is
	// non-greedy, so two anchors on one line are two matches.
	anchorPattern = `<!--\s*source:\s*(.+?)\s*-->`
	// descSepPattern is the separator between a segment's paths and the
	// description of them. Three spellings are accepted, and the em dash is the
	// one a document written in prose carries.
	descSepPattern = `\s+(?:--|-|\x{2014})\s+`
	// symbolClaimPattern matches only a declaration claim: an identifier or a
	// dotted identifier chain. All other descriptions apply to the file. These
	// include phrases, hyphenated binary names, and ranges such as
	// StateIdle..StateEstablished.
	symbolClaimPattern = `^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`
	// lineSuffixPattern is the `:42` that a path CAN carry.
	lineSuffixPattern = `:\d+$`
	// prosePattern is severity rule 1. A single lowercase word -- no capital,
	// no underscore, no dot -- is an English noun in a prose list rather than
	// an identifier. Measured over the whole tree: of 372 claims that resolved
	// against no declaration, 105 are this shape, and every one sampled was
	// prose. The separators keep the rule narrow: `sa_count` and
	// `ze.storage.blob` carry no capital either and both name something a
	// document really claims. The cost is priced and accepted: an all-lowercase
	// Go declaration can no longer be claimed by an anchor.
	prosePattern = `^[a-z][a-z0-9]*$`
	// goFuncPattern, goTypeBodyPattern, goTopDeclPattern, goGroupOpenPattern
	// and goMemberPattern read declarations. gofmt puts every top-level
	// declaration at column 0 and closes it at column 0, so indentation alone
	// separates a declaration from a function body.
	goFuncPattern      = `^func\s+(?:\(\s*\w*\s*\*?(?P<recv>[A-Za-z_]\w*)(?:\[[^\]]*\])?\s*\)\s*)?(?P<name>[A-Za-z_]\w*)`
	goTypeBodyPattern  = `^type\s+(?P<name>[A-Za-z_]\w*)(?:\[[^\]]*\])?\s+(?:struct|interface)\s*\{\s*$`
	goTopDeclPattern   = `^(?:type|var|const)\s+(?P<names>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)`
	goGroupOpenPattern = `^(?:type|var|const)\s*\(\s*$`
	goMemberPattern    = `^(?P<names>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)`
	// wordBoundary is what severity rule 2 wraps a claim in. It is a const so
	// the pattern is built by textbuf rather than by `+` beside a quote.
	wordBoundary = `\b`
)

var (
	anchorRe      = regexp.MustCompile(anchorPattern)
	descSepRe     = regexp.MustCompile(descSepPattern)
	symbolClaimRe = regexp.MustCompile(symbolClaimPattern)
	lineSuffixRe  = regexp.MustCompile(lineSuffixPattern)
	proseRe       = regexp.MustCompile(prosePattern)
	goFuncRe      = regexp.MustCompile(goFuncPattern)
	goTypeBodyRe  = regexp.MustCompile(goTypeBodyPattern)
	goTopDeclRe   = regexp.MustCompile(goTopDeclPattern)
	goGroupOpenRe = regexp.MustCompile(goGroupOpenPattern)
	goMemberRe    = regexp.MustCompile(goMemberPattern)
)

// Segment is one part of a source anchor. It contains the code paths and their
// description.
//
// The description is a CLAIM about those paths, so it stays with them.
// checkAnchors verifies its symbols, and one description covers every path in
// the segment.
type Segment struct {
	Paths       []string
	Description string
}

// Anchor is one segment, with where it was found.
type Anchor struct {
	Doc     string
	Line    int
	Paths   []string
	Descrip string
}

// anchorSegments splits one anchor's content into its segments.
//
// Two formats are handled. Semicolons separate segments, each `paths --
// description`. Commas separate paths inside a segment, and a name that starts
// with no known root is read relative to the last full path's directory.
func anchorSegments(content string) []Segment {
	var segments []Segment
	for raw := range strings.SplitSeq(content, ";") {
		seg := strings.TrimSpace(raw)
		if seg == "" {
			continue
		}

		parts := descSepRe.Split(seg, 2)
		segPath := strings.TrimSpace(parts[0])
		description := ""
		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		var paths []string
		lastDir := ""
		for token := range strings.SplitSeq(segPath, ",") {
			part := strings.TrimSpace(token)
			if part == "" {
				continue
			}
			switch {
			case hasKnownRoot(part):
				paths = append(paths, part)
				if idx := strings.LastIndex(part, "/"); idx >= 0 {
					lastDir = part[:idx]
				}
			case lastDir != "" && !strings.HasPrefix(part, "/"):
				var tb textbuf.Buffer
				paths = append(paths, tb.Str(lastDir).Byte('/').Str(part).String())
			}
		}
		if len(paths) > 0 {
			segments = append(segments, Segment{Paths: paths, Description: description})
		}
	}
	return segments
}

// hasKnownRoot reports whether a token names a path from the tree's root.
func hasKnownRoot(part string) bool {
	for _, prefix := range pathPrefixes {
		if strings.HasPrefix(part, prefix) {
			return true
		}
	}
	return false
}

// anchorSymbolTokens answers the declaration claims in a description, in order.
//
// The description is a comma-separated list. A token is a claim only when it
// contains a declaration name. A token with a space, hyphen, or other
// non-identifier text is prose and is dropped. The same applies to one
// lowercase word.
func anchorSymbolTokens(description string) []string {
	var claims []string
	for raw := range strings.SplitSeq(description, ",") {
		token := strings.TrimSpace(raw)
		if bare, cut := strings.CutSuffix(token, "()"); cut {
			token = strings.TrimRight(bare, " \t\n\v\f\r")
		}
		if token == "" || !symbolClaimRe.MatchString(token) {
			continue
		}
		if proseRe.MatchString(token) {
			continue
		}
		claims = append(claims, token)
	}
	return claims
}

// declarations is what one Go file declares: the simple names, and the
// Recv.Member names.
type declarations struct {
	Names  map[string]bool
	Dotted map[string]bool
}

// goDeclarations reads the declarations of one Go file's text.
func goDeclarations(text string) declarations {
	decls := declarations{Names: map[string]bool{}, Dotted: map[string]bool{}}
	owner := ""
	inGroup := false

	for line := range strings.SplitSeq(text, "\n") {
		// A blank line separates members inside a declaration body. It opens or
		// closes nothing. Testing column 0 first would treat "" as a top-level
		// line and drop every member after the first blank.
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			owner, inGroup = "", false
			if match := goFuncRe.FindStringSubmatch(line); match != nil {
				name := match[goFuncRe.SubexpIndex("name")]
				decls.Names[name] = true
				if recv := match[goFuncRe.SubexpIndex("recv")]; recv != "" {
					var tb textbuf.Buffer
					decls.Dotted[tb.Str(recv).Byte('.').Str(name).String()] = true
				}
				continue
			}
			if match := goTypeBodyRe.FindStringSubmatch(line); match != nil {
				owner = match[goTypeBodyRe.SubexpIndex("name")]
				decls.Names[owner] = true
				continue
			}
			if goGroupOpenRe.MatchString(line) {
				inGroup = true
				continue
			}
			if match := goTopDeclRe.FindStringSubmatch(line); match != nil {
				for _, name := range splitNames(match[goTopDeclRe.SubexpIndex("names")]) {
					decls.Names[name] = true
				}
			}
			continue
		}
		if owner == "" && !inGroup {
			continue
		}
		member := strings.TrimSpace(line)
		if member == "" || strings.HasPrefix(member, "//") {
			continue
		}
		match := goMemberRe.FindStringSubmatch(member)
		if match == nil {
			continue
		}
		for _, name := range splitNames(match[goMemberRe.SubexpIndex("names")]) {
			decls.Names[name] = true
			if owner != "" {
				var tb textbuf.Buffer
				decls.Dotted[tb.Str(owner).Byte('.').Str(name).String()] = true
			}
		}
	}
	return decls
}

// splitNames answers the trimmed names of a comma-separated declaration list.
//
// stringsx.SplitCount rather than strings.Split: the slice is kept, so the
// count is what sizes it once instead of letting append grow it.
func splitNames(list string) []string {
	parts, count := stringsx.SplitCount(list, ",")
	names := make([]string, 0, count)
	for _, name := range parts {
		names = append(names, strings.TrimSpace(name))
	}
	return names
}

// claimIsDeclared reports whether the anchored files declare what a claim
// names.
//
// A dotted claim resolves when the files declare the member itself, or declare
// the member's name on its own. It does NOT resolve on the prefix: a
// package-qualified call such as events.Register names another package's
// declaration, which these files do not hold.
func claimIsDeclared(claim string, names, dotted map[string]bool) bool {
	if !strings.Contains(claim, ".") {
		return names[claim]
	}
	if dotted[claim] {
		return true
	}
	return names[claim[strings.LastIndex(claim, ".")+1:]]
}

// wordRe caches the whole-word expression for one claim. The same claim appears
// in many anchors, and compiling it once per anchor is the cost this removes.
var wordRe sync.Map

// claimAppearsAsWord reports whether one anchored file's text holds the claim
// as a whole word.
//
// Severity rule 2. If a reader can find the name in the anchored file, the
// anchor is not false even when the file declares nothing by that name. The
// name can appear in a call, receiver member, string key, local, or comment.
// Word boundaries find `Run` inside `p.Run()`. They find `events.Register` only
// inside that exact dotted text.
func claimAppearsAsWord(claim string, texts []string) bool {
	var re *regexp.Regexp
	if cached, ok := wordRe.Load(claim); ok {
		re = cached.(*regexp.Regexp) //nolint:errcheck,forcetypeassert // the map holds only what this function stores
	} else {
		var tb textbuf.Buffer
		compiled, err := regexp.Compile(tb.Str(wordBoundary).Str(regexp.QuoteMeta(claim)).Str(wordBoundary).String())
		if err != nil {
			// A claim that cannot be made into an expression is one this rule
			// cannot demote, so it stays a finding rather than passing.
			return false
		}
		wordRe.Store(claim, compiled)
		re = compiled
	}
	return slices.ContainsFunc(texts, re.MatchString)
}

// checkAnchors verifies that every symbol a source anchor names is declared
// where it points.
//
// A path that does not exist is left to the stale-reference check, which
// already owns it. A claim the files do not declare is reported only when their
// text does not hold it either, which is severity rule 2.
//
// It FAILS CLOSED when it cannot read a file. An unreadable anchored file is a
// finding, not a pass. Its unresolved claims are UNKNOWN rather than absent.
// Otherwise, "not declared" would itself be a false claim about unread text.
func checkAnchors(root string, anchors []Anchor) []string {
	problems := []string{}
	// path -> the declarations and the text, or a zero entry with read=false
	// when the file cannot be read. The text is cached beside the declarations
	// so rule 2 costs no second read.
	type cached struct {
		decls declarations
		text  string
		read  bool
	}
	seen := map[string]cached{}

	for _, anchor := range anchors {
		claims := anchorSymbolTokens(anchor.Descrip)
		if len(claims) == 0 {
			continue
		}

		var goPaths []string
		for _, path := range anchor.Paths {
			clean := lineSuffixRe.ReplaceAllString(path, "")
			if !strings.HasSuffix(clean, ".go") {
				continue
			}
			if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean))); err != nil || info.IsDir() {
				continue
			}
			goPaths = append(goPaths, clean)
		}
		if len(goPaths) == 0 {
			continue
		}

		names, dotted := map[string]bool{}, map[string]bool{}
		var texts, unreadable []string
		for _, path := range goPaths {
			entry, known := seen[path]
			if !known {
				body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // a path an anchor in this tree names
				if err != nil {
					entry = cached{}
				} else {
					entry = cached{decls: goDeclarations(string(body)), text: string(body), read: true}
				}
				seen[path] = entry
			}
			if !entry.read {
				unreadable = append(unreadable, path)
				continue
			}
			for name := range entry.decls.Names {
				names[name] = true
			}
			for name := range entry.decls.Dotted {
				dotted[name] = true
			}
			texts = append(texts, entry.text)
		}

		for _, path := range unreadable {
			problems = append(problems, unreadableLine(anchor, path))
		}

		where := strings.Join(goPaths, ", ")
		for _, claim := range claims {
			switch {
			case claimIsDeclared(claim, names, dotted):
			case len(unreadable) > 0:
			case claimAppearsAsWord(claim, texts):
			default:
				problems = append(problems, undeclaredLine(anchor, where, claim))
			}
		}
	}
	return problems
}

// unreadableLine renders the finding for an anchored file that this run cannot
// read.
func unreadableLine(anchor Anchor, path string) string {
	var tb textbuf.Buffer
	return tb.Str(anchor.Doc).Byte(':').Int(int64(anchor.Line)).Str(": source anchor ").Str(path).
		Str(": cannot read the anchored file, so its symbols are unverifiable").String()
}

// undeclaredLine renders the finding for a claim the anchored files neither
// declare nor mention.
func undeclaredLine(anchor Anchor, where, claim string) string {
	var tb textbuf.Buffer
	return tb.Str(anchor.Doc).Byte(':').Int(int64(anchor.Line)).Str(": source anchor ").Str(where).
		Str(" names '").Str(claim).Str("', which is not declared there").String()
}

// packageDir answers the package directory a code path belongs to.
func packageDir(path string) string {
	if strings.HasSuffix(path, "/") {
		return strings.TrimRight(path, "/")
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return path
}

// filterGitignored drops the paths git ignores, keeping the order.
//
// ai/CODE-TO-DOCS.md is derived from docs/. That tree also contains gitignored
// research output on machines with local research files. An index of that
// output is not reproducible: one measured host had 1439 code paths, while a
// clean checkout had 1438.
//
// When git is unavailable or the tree is not a repository, the unfiltered list
// remains usable. This deliberate fallback lets the generator run where git is
// unrelated to the index request.
func filterGitignored(root string, rels []string) []string {
	if len(rels) == 0 {
		return rels
	}

	cmd := exec.Command("git", "-C", root, "check-ignore", "--stdin") //nolint:gosec,noctx // a build tool queries the checkout it was pointed at
	cmd.Stdin = strings.NewReader(strings.Join(rels, "\n"))
	out, err := cmd.Output()

	// git check-ignore exits 0 when some paths are ignored, 1 when none are,
	// and 128 on error such as "not a git repository". Only 0 and 1 are
	// trustworthy.
	code := 0
	if err != nil {
		code = exitCodeOf(err)
	}
	if code != 0 && code != 1 {
		return rels
	}

	ignored := map[string]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line != "" {
			ignored[line] = true
		}
	}
	kept := make([]string, 0, len(rels))
	for _, rel := range rels {
		if !ignored[rel] {
			kept = append(kept, rel)
		}
	}
	return kept
}

// exitCodeOf answers the status of a finished child. It answers 128 when the
// error is not an exit, which is also git's status for an unreadable tree. This
// caller already distrusts that status.
func exitCodeOf(err error) int {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return 128
	}
	if code := exit.ExitCode(); code >= 0 {
		return code
	}
	return 128
}

// codeIndex is the reverse index: one entry per code path, with the documents
// that anchor it.
type codeIndex struct {
	// Refs maps a code path to the documents and lines that name it.
	Refs map[string][]Ref
	// Anchors is every anchor segment found, in document order, so the symbol
	// check reads this rather than walking docs/ a second time.
	Anchors []Anchor
}

// Ref is one place a document names a code path.
type Ref struct {
	Doc  string
	Line int
}

// buildCodeIndex reads every markdown file under docs/ and inverts the source
// anchors it finds.
func buildCodeIndex(root string) (codeIndex, error) {
	index := codeIndex{Refs: map[string][]Ref{}}

	docs, err := markdownFiles(root)
	if err != nil {
		return codeIndex{}, err
	}

	seen := map[string]map[Ref]bool{}
	for _, rel := range filterGitignored(root, docs) {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a markdown file of the tree the caller named
		if err != nil {
			return codeIndex{}, err
		}
		for number, line := range strings.Split(string(body), "\n") {
			for _, match := range anchorRe.FindAllStringSubmatch(line, -1) {
				for _, segment := range anchorSegments(match[1]) {
					anchor := Anchor{Doc: rel, Line: number + 1, Paths: segment.Paths, Descrip: segment.Description}
					index.Anchors = append(index.Anchors, anchor)
					for _, path := range segment.Paths {
						ref := Ref{Doc: rel, Line: anchor.Line}
						if seen[path] == nil {
							seen[path] = map[Ref]bool{}
						}
						if seen[path][ref] {
							continue
						}
						seen[path][ref] = true
						index.Refs[path] = append(index.Refs[path], ref)
					}
				}
			}
		}
	}
	for path := range index.Refs {
		sortRefs(index.Refs[path])
	}
	return index, nil
}

// sortRefs orders one path's references by document then line, which is the
// order the stale report prints them in.
func sortRefs(refs []Ref) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Doc != refs[j].Doc {
			return refs[i].Doc < refs[j].Doc
		}
		return refs[i].Line < refs[j].Line
	})
}

// markdownFiles answers every .md file under docs/, sorted by its path.
//
// A whole-path sort replaces a per-directory sort. The anchors and the findings
// use this same order.
func markdownFiles(root string) ([]string, error) {
	base := filepath.Join(root, docsDir)
	var found []string
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(found)
	return found, nil
}
