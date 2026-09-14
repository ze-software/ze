// Design: docs/architecture/core-design.md -- a development gate as one le area
//
// Detail: corpus.go -- the key sets a literal is judged against
// Detail: doctorcheck.go -- the second finding kind, a check nothing registers
// Detail: report.go -- what the actions answer
//
// Package enumeration refuses a Go literal that enumerates what a live registry
// already holds. Two declarations of one fact drift, and the reader of the
// second cannot tell which side is wrong (ai/rules/principles.md).
//
// The judgement is syntactic and it is deliberately narrow. One syntactic unit
// is one composite literal, one const block, or one switch statement. A unit
// whose string constants include TWO OR MORE distinct keys of one registry is a
// copy, unless the unit is where the registry gets them. ONE key is never a
// finding, because every mention of a plugin name would be one.
//
// The exclusion attaches to the DECLARING SYMBOL, never to its package. A
// registry's own package holds one symbol that writes the set down, and a
// second list beside it is the copy nothing else in the tree can catch: it is
// the one place both sides of a drift can sit
// (plan/journal/gate-excludes-part-of-its-population.md).
//
// The distinction the gate turns on is not syntactic, so it is not automated: a
// list that states what is ALLOWED is policy and is legitimately written out,
// while a list that states what EXISTS is a copy (internal/le/plugin/imports/
// pluginimports.go states it for its own pluginDirs). A policy list carries an
// exemption marker with a reason, and the marker set is itself accounted for,
// so a marker that suppresses nothing turns the gate red rather than rotting in
// place.

package enumeration

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/population"
)

// scanFloor is the least non-test Go files the walk must read before the gate
// believes it saw the product tree. This checkout carried 4976 on 2026-09-14,
// so the floor fires on a tree that was never read rather than on one that
// shrank (ai/rules/evidence.md).
const scanFloor = 2000

// keyThreshold is the least distinct keys of one registry that make a unit a
// copy of it. One key is a mention; two is a set being written down again.
const keyThreshold = 2

// ErrShortScan names a walk that read fewer files than the floor.
var ErrShortScan = errors.New("the walk read fewer Go files than the floor")

// markerRe matches the inline exemption marker. The reason in parentheses after
// it is what makes it an exemption: markerReason reads it, and a marker without
// one suppresses nothing.
var markerRe = regexp.MustCompile(`enumeration:\s*exempt`)

// productRoots are the directories holding the Go the gate judges, relative to
// the checkout.
//
// This list is POLICY by the same test the gate applies to every other list: it
// states where product Go is ALLOWED to live, which no registry holds. A list
// derived from the tree would agree with whatever the tree contains, and would
// stop being a gate.
func productRoots() []string { return []string{"internal", "cmd", "pkg"} }

// Check walks roots under tree and answers every literal that restates one of
// corpora, plus every exemption marker that excuses nothing.
//
// fileFloor is a parameter rather than a constant because a fixture tree holds
// a handful of files: the action passes scanFloor and a test over testdata
// passes 0.
func Check(tree string, roots []string, corpora []Corpus, fileFloor int) (Findings, error) {
	walked := newScan(corpora)
	for _, root := range roots {
		if err := walked.walk(tree, filepath.Join(tree, filepath.FromSlash(root))); err != nil {
			return nil, err
		}
	}
	if walked.read < fileFloor {
		return nil, fmt.Errorf("%w: %d non-test Go files under %s, below the floor of %d: this tree was not read",
			ErrShortScan, walked.read, tree, fileFloor)
	}
	if err := walked.judge(tree); err != nil {
		return nil, err
	}
	return walked.findings()
}

// scan is the state one walk accumulates: the files it read, the candidate
// copies it found, and the markers it must account for.
type scan struct {
	corpora []Corpus
	read    int
	// files holds the walk's non-test Go files, by package directory. The
	// judgement is per PACKAGE rather than per file, because a const block is
	// declared in one file and registered in another: internal/plugins/ospf
	// declares its diagnostic codes in doctor.go and registers them in
	// register.go.
	files      map[string][]string
	candidates []candidate
	// markers maps each marker's key to where it stands and why. A marker with
	// no reason never reaches this map: it is reported where it was read.
	markers map[string]markerSite
	// matched holds the markers that suppressed a unit, which is what tells a
	// dead marker apart from a live one.
	matched map[string]bool
	// walkFound is what the walk itself reports, apart from the copies: a
	// marker that states no reason is a finding wherever it stands.
	walkFound Findings
}

func newScan(corpora []Corpus) *scan {
	return &scan{
		corpora: corpora,
		files:   map[string][]string{},
		markers: map[string]markerSite{},
		matched: map[string]bool{},
	}
}

// markerSite is one exemption marker: where it stands, and the reason its
// author gave for it.
type markerSite struct {
	file   string
	line   int
	reason string
}

// candidate is one syntactic unit holding two or more keys of one group, before
// the exemption markers and the enclosing-unit filter have had their say.
//
// It carries the unit's extent twice, and the two are not interchangeable. from
// and to are token positions, which is what the enclosing-unit filter compares:
// two literals can open and close on one line, and a line range cannot tell one
// from the other, so a line-based filter reported both. start and end are lines,
// which is what a reader is given and what an exemption marker is matched
// against.
type candidate struct {
	file   string
	symbol string
	from   token.Pos
	to     token.Pos
	start  int
	end    int
	corpus string
	group  string
	detail string
	keys   []string
	// strings is how many distinct string constants the whole unit holds. The
	// flat-corpus rule is a ratio against this, and a reader is given both
	// numbers for the same reason.
	strings int
}

// walk reads every non-test Go file under root.
//
// A root the tree does not carry is passed over, because a fixture tree holds
// only the roots its case needs. Every other stat failure stops the run: a walk
// that ended early answers the same "no findings" a clean tree answers.
func (s *scan) walk(tree, root string) error {
	info, err := os.Stat(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("stat the scan root %s: %w", root, err)
	case !info.IsDir():
		return fmt.Errorf("the scan root %s is not a directory", root)
	}

	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if entry.IsDir() {
			if skipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return relErr
		}
		s.read++
		slashed := filepath.ToSlash(rel)
		pkgDir := pathpkg.Dir(slashed)
		s.files[pkgDir] = append(s.files[pkgDir], slashed)
		return nil
	})
}

// skipDir answers the directories a product walk never enters. Generated
// dependencies, test fixtures and tool state are not the product's Go.
func skipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return name == "vendor" || name == "testdata"
}

// judge reads every package the walk collected and records what each one holds.
//
// The packages are read in path order, so two runs over one tree answer in one
// order, and each package is read into its own file set, which is freed before
// the next: the walk covers five thousand files and holding every tree at once
// is not worth the memory.
func (s *scan) judge(tree string) error {
	pkgDirs := make([]string, 0, len(s.files))
	for pkgDir := range s.files {
		pkgDirs = append(pkgDirs, pkgDir)
	}
	slices.Sort(pkgDirs)
	for _, pkgDir := range pkgDirs {
		if err := s.judgePackage(tree, pkgDir, s.files[pkgDir]); err != nil {
			return err
		}
	}
	return nil
}

// judgePackage records one package's candidates and its markers.
func (s *scan) judgePackage(tree, pkgDir string, rels []string) error {
	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(rels))
	for _, rel := range rels {
		file, err := parser.ParseFile(fset, filepath.Join(tree, filepath.FromSlash(rel)), nil,
			parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		parsed = append(parsed, file)
		s.readMarkers(fset, file, rel)
	}

	declaring := s.declaringSymbols(pkgDir)
	registered := registeredConstants(parsed, declaring)
	for i, file := range parsed {
		for _, decl := range file.Decls {
			symbol := declSymbol(decl)
			against := s.judgeable(pkgDir, symbol)
			if isRegisteredConstBlock(registered, decl) {
				against = composedKeys(against)
			}
			if len(against) == 0 {
				continue
			}
			s.scanDecl(fset, rels[i], symbol, decl, against)
		}
	}
	return nil
}

// declaringSymbols answers the top-level symbols of pkgDir that WRITE a
// corpus's set down, so registeredConstants can read the const names those
// symbols use.
func (s *scan) declaringSymbols(pkgDir string) map[string]bool {
	symbols := map[string]bool{}
	for _, corpus := range s.corpora {
		for _, site := range corpus.Declarations {
			if site.Package != pkgDir {
				continue
			}
			symbols[site.Symbol] = true
		}
	}
	return symbols
}

// registeredConstants answers every identifier a package hands to a registrar,
// anywhere: inside a registration literal, as an argument of a Register call,
// or inside a symbol this package declares a corpus at.
//
// This is the const form of the declaration rule. `const
// codeOSPFRouterIDMissing = "doctor-ospf-router-id-missing"` is where that
// diagnostic code comes from, because `CodeMeta{Code: codeOSPFRouterIDMissing}`
// carries the const itself into the registry. Asking the author to derive it
// from the registry asks them to derive it from itself.
//
// A declaring symbol reaches its const block the same way. `command.Verbs`
// takes no registrar call, and the thirteen `VerbShow`-style constants it keys
// on are the spellings it is built from, so the map that uses them is what says
// the const block is their declaration.
func registeredConstants(files []*ast.File, declaring map[string]bool) map[string]bool {
	used := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if declaring[declSymbol(decl)] {
				collectIdents(decl, posRange{start: decl.Pos(), end: decl.End()}, used)
				continue
			}
			for _, area := range declarationRanges(decl) {
				collectIdents(decl, area, used)
			}
		}
	}
	return used
}

// collectIdents records every identifier written inside one declaration range.
func collectIdents(decl ast.Decl, area posRange, used map[string]bool) {
	ast.Inspect(decl, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Pos() >= area.start && ident.End() <= area.end {
			used[ident.Name] = true
		}
		return true
	})
}

// composedKeys answers the corpora whose keys a const block can never declare,
// because the registrar builds each key rather than taking it whole.
//
// This is what keeps a const block of family names a finding while a const
// block of diagnostic codes is not, and the difference is not a preference: a
// diagnostic code enters the registry as the const itself, while a family name
// enters it as an AFI name and a SAFI name that family.MustRegister joins. So
// internal/component/bgp/message/family.go holds four joined strings the
// registry never received, beside a registrar call that takes those same four
// strings to say where they came from, and that call is not their declaration.
func composedKeys(corpora []Corpus) []Corpus {
	composed := make([]Corpus, 0, len(corpora))
	for _, corpus := range corpora {
		if corpus.WrittenWhole {
			continue
		}
		composed = append(composed, corpus)
	}
	return composed
}

// isRegisteredConstBlock reports whether decl is a const block whose every
// string constant is one the package registers.
//
// EVERY, not any: a block mixing registered names with a copy of somebody
// else is still holding the copy, and the reader needs to be told about it.
func isRegisteredConstBlock(registered map[string]bool, decl ast.Decl) bool {
	block, ok := decl.(*ast.GenDecl)
	if !ok || block.Tok != token.CONST {
		return false
	}
	named := 0
	for _, spec := range block.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range value.Names {
			named++
			if !registered[name.Name] {
				return false
			}
		}
	}
	return named > 0
}

// judgeable answers the corpora this symbol does not declare. The symbol that
// BUILDS a registry writes its keys out once, and that once is the declaration;
// every other symbol of the same package is judged, because a second
// declaration beside the first is a copy wherever it stands
// (plan/journal/gate-excludes-part-of-its-population.md).
func (s *scan) judgeable(pkgDir, symbol string) []Corpus {
	judged := make([]Corpus, 0, len(s.corpora))
	for _, corpus := range s.corpora {
		if corpus.declares(pkgDir, symbol) {
			continue
		}
		judged = append(judged, corpus)
	}
	return judged
}

// readMarkers records every exemption marker in one file. A marker whose
// parentheses are empty or absent is reported where it stands, because it asks
// the next reader to take the exemption on trust.
func (s *scan) readMarkers(fset *token.FileSet, file *ast.File, rel string) {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			found := markerRe.FindStringIndex(comment.Text)
			if found == nil {
				continue
			}
			line := fset.Position(comment.Pos()).Line
			reason := markerReason(comment.Text[found[1]:])
			if reason == "" {
				s.walkFound = append(s.walkFound, Finding{
					Kind:   KindMarker,
					File:   rel,
					Line:   line,
					Detail: "the marker states no reason, so it suppresses nothing: write the reason in parentheses after it",
				})
				continue
			}
			s.markers[markerKey(rel, line)] = markerSite{file: rel, line: line, reason: reason}
		}
	}
}

// markerReason answers the parenthesised reason that follows a marker, or the
// empty string when the author gave none. Copied in shape from
// internal/le/doc/check/citation.go, which requires a reason for the same
// reason: the reason is the whole value of an exception.
func markerReason(tail string) string {
	open := strings.IndexByte(tail, '(')
	if open < 0 {
		return ""
	}
	closing := strings.IndexByte(tail[open+1:], ')')
	if closing < 0 {
		return ""
	}
	return strings.TrimSpace(tail[open+1 : open+1+closing])
}

func markerKey(rel string, line int) string {
	return rel + ":" + strconv.Itoa(line)
}

// declSymbol names the top-level declaration a unit was found in, so a finding
// tells the reader which symbol to open.
func declSymbol(decl ast.Decl) string {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		return typed.Name.Name
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			switch named := spec.(type) {
			case *ast.ValueSpec:
				if len(named.Names) > 0 {
					return named.Names[0].Name
				}
			case *ast.TypeSpec:
				return named.Name.Name
			}
		}
	}
	return "(file scope)"
}

// scanDecl judges every syntactic unit inside one declaration, apart from the
// units that DECLARE what a registry then holds.
func (s *scan) scanDecl(fset *token.FileSet, rel, symbol string, decl ast.Decl, judged []Corpus) {
	declared := declarationRanges(decl)
	ast.Inspect(decl, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		if !isUnit(node) {
			return true
		}
		if within(declared, node) {
			// The subtree is skipped rather than passed over, because the keys
			// of a declaration sit in a field of it: the Dependencies slice of
			// a Registration is a nested literal of its own.
			return false
		}
		keys := stringConstants(node)
		if len(keys) < keyThreshold {
			return true
		}
		start := fset.Position(node.Pos()).Line
		end := fset.Position(node.End()).Line
		for _, corpus := range judged {
			group, matched := corpus.bestGroup(keys)
			if len(matched) < keyThreshold {
				continue
			}
			s.candidates = append(s.candidates, candidate{
				file: rel, symbol: symbol,
				from: node.Pos(), to: node.End(), start: start, end: end,
				corpus: corpus.Name, group: group, detail: corpus.detail(group),
				keys: matched, strings: len(keys),
			})
		}
		return true
	})
}

// registrarPrefix is how this repository spells a call that FEEDS a registry.
// Register, RegisterBuiltinFamilies, MustRegisterLocalData: the verb is the
// convention, and the convention is what makes the call findable without a list
// of registrar names here.
const registrarPrefix = "Register"

// registrationSuffix is how this repository spells the TYPE of a value a
// registrar takes: registry.Registration, and every sibling named for the same
// job.
const registrationSuffix = "Registration"

// posRange is one half-open source range, used to mark the parts of a
// declaration the judgement does not enter.
type posRange struct {
	start token.Pos
	end   token.Pos
}

// declarationRanges answers the parts of decl that DECLARE a fact to a registry
// rather than read one back.
//
// This is the distinction the whole gate turns on, in its second form. A list
// that states what EXISTS is a copy; a list that CREATES what exists is the one
// declaration the copies are copies of. `registry.Registration{Name: "bgp-rpki-
// decorator", Dependencies: []string{"bgp", "bgp-rpki"}}` names three plugins
// and is not a copy of the plugin registry: it is how two of those three edges
// enter it. Flagging it would ask an author to derive a fact from the registry
// they are in the middle of populating.
//
// Four shapes carry it, and a declaration site needs only one of them. The
// value is built where it is registered, so the literal is an argument of a
// call whose name begins with Register. The value is built into a local first
// and that local is registered, which is what rpki_decorator and every doctor
// check do. The values are written into a range clause and registered one by
// one in its body, which is how a package registers its diagnostic codes. Or
// the value outlives the function that registers it, in which case only its
// TYPE says what it is for, and the type is named for registration.
//
// An exemption marker would be the wrong tool here, and deliberately is not
// used: a marker records a violation somebody decided to live with, and a
// declaration is not a violation.
func declarationRanges(decl ast.Decl) []posRange {
	registered := registeredNames(decl)
	var declared []posRange
	ast.Inspect(decl, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			if strings.HasPrefix(calleeName(typed.Fun), registrarPrefix) {
				declared = append(declared, posRange{start: typed.Pos(), end: typed.End()})
			}
		case *ast.CompositeLit:
			if strings.HasSuffix(calleeName(typed.Type), registrationSuffix) {
				declared = append(declared, posRange{start: typed.Pos(), end: typed.End()})
			}
		case *ast.AssignStmt:
			declared = append(declared, assignedToRegistered(registered, typed.Lhs, typed.Rhs)...)
		case *ast.ValueSpec:
			names := make([]ast.Expr, 0, len(typed.Names))
			for _, name := range typed.Names {
				names = append(names, name)
			}
			declared = append(declared, assignedToRegistered(registered, names, typed.Values)...)
		case *ast.RangeStmt:
			// The fourth shape, and it is the doctor codes: a slice of
			// registrations written into the range clause, each element handed
			// to the registrar in the body.
			if typed.Value == nil {
				return true
			}
			declared = append(declared, assignedToRegistered(registered, []ast.Expr{typed.Value}, []ast.Expr{typed.X})...)
		}
		return true
	})
	return declared
}

// registeredNames answers every variable decl hands to a registrar, by name.
//
// The dataflow this follows is one step long on purpose. A value built into a
// local and registered two lines later is the shape of every doctor check and
// of the plugin registrations; a value that travels further than the
// declaration it is written in is left to the type rule above.
func registeredNames(decl ast.Decl) map[string]bool {
	registered := map[string]bool{}
	ast.Inspect(decl, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !strings.HasPrefix(calleeName(call.Fun), registrarPrefix) {
			return true
		}
		for _, argument := range call.Args {
			if unary, ok := argument.(*ast.UnaryExpr); ok {
				argument = unary.X
			}
			if ident, ok := argument.(*ast.Ident); ok {
				registered[ident.Name] = true
			}
		}
		return true
	})
	return registered
}

// assignedToRegistered answers the ranges of the literals assigned to a name
// that decl then registers.
func assignedToRegistered(registered map[string]bool, names, values []ast.Expr) []posRange {
	if len(names) != len(values) {
		return nil
	}
	var declared []posRange
	for i, name := range names {
		if !writesRegistered(registered, name) {
			continue
		}
		value := values[i]
		if unary, ok := value.(*ast.UnaryExpr); ok {
			value = unary.X
		}
		if literal, ok := value.(*ast.CompositeLit); ok {
			declared = append(declared, posRange{start: literal.Pos(), end: literal.End()})
		}
	}
	return declared
}

// writesRegistered reports whether this assignment target writes into a value
// decl then registers: the variable itself for `checks = ...`, a field of it
// for `reg.DoctorChecks = ...`, an element for `regs[0] = ...`.
//
// A field is as much a declaration as the variable is. `reg.DoctorChecks =
// []registry.DoctorCheckDef{{Codes: []string{"doctor-as112-port-unavailable"}}}`
// at internal/plugins/as112/register.go:143 is how that code enters the
// registry, and reading only an identifier here reported as112 and geodns for
// restating the codes their own checks emit.
func writesRegistered(registered map[string]bool, name ast.Expr) bool {
	switch typed := name.(type) {
	case *ast.Ident:
		return registered[typed.Name]
	case *ast.SelectorExpr:
		return writesRegistered(registered, typed.X)
	case *ast.IndexExpr:
		return writesRegistered(registered, typed.X)
	case *ast.StarExpr:
		return writesRegistered(registered, typed.X)
	}
	return false
}

// calleeName answers the name an expression is written as, without its package
// qualifier: `registry.Register` answers `Register`. An expression that is not a
// name answers the empty string, which matches no prefix and no suffix.
func calleeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	case *ast.IndexExpr:
		return calleeName(typed.X)
	case *ast.StarExpr:
		return calleeName(typed.X)
	}
	return ""
}

// within reports whether node sits inside one of the declaration ranges.
func within(declared []posRange, node ast.Node) bool {
	for _, area := range declared {
		if node.Pos() >= area.start && node.End() <= area.end {
			return true
		}
	}
	return false
}

// detail says what this unit did, in the words the corpus can support.
//
// A registry corpus knows which side is the declaration, so the row names it:
// the registry holds the set and the literal restates it. A CLOSED corpus does
// not know. internal/component/ike/ipsec/types.go names the twelve ESP
// encryption algorithms and so does ze-ipsec-conf.yang, and which of the two
// the other should derive from is a design question this gate cannot answer.
// So the row states the fact it is sure of, that two declarations of one set
// must agree, and leaves the direction to whoever repairs it.
func (c Corpus) detail(group string) string {
	if c.Closed {
		return "holds every value of " + group + ", so the two must agree"
	}
	return "restates " + group
}

// isUnit reports whether node is one of the three shapes that write a set down:
// a composite literal, a const block, or a switch statement.
func isUnit(node ast.Node) bool {
	switch typed := node.(type) {
	case *ast.CompositeLit, *ast.SwitchStmt:
		return true
	case *ast.GenDecl:
		return typed.Tok == token.CONST
	}
	return false
}

// stringConstants answers the distinct string constants written inside node.
func stringConstants(node ast.Node) []string {
	seen := map[string]bool{}
	ast.Inspect(node, func(inner ast.Node) bool {
		lit, ok := inner.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			// The file parsed, so a literal that will not unquote is a form the
			// standard library added rather than a defect to report here.
			return true
		}
		if value != "" {
			seen[value] = true
		}
		return true
	})
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// closedGroupKeysMin is the least values a closed enumeration must hold before
// a literal covering all of them means anything. A two-value enumeration is
// covered by any code that names the same two domain words, and `true`/`false`,
// `in`/`out` and `ipv4`/`ipv6` are each an enumeration somewhere in the model.
const closedGroupKeysMin = 3

// flatKeyShareMin is the least share, in percent, of a literal's strings that
// must be registry keys before the literal is judged to be MADE of them.
//
// It is the mirror of the rule above. A closed enumeration has a set to cover,
// so a copy covers it. A registry has no set small enough to cover, so the
// measure runs the other way: a transcription is a literal made of registry
// keys, and a coincidence is a literal about something else that happens to
// hold two of them. internal/core/portname/services_table.go holds 4954 IANA
// service names, six of which are also plugin names, and it is a table of
// service names.
//
// The number is measured rather than chosen. Over this checkout on 2026-09-14,
// one string in ten loses three literals that ARE copies (internal/component/
// cli/model.go:247, internal/component/config/validate_sections.go:230,
// internal/le/yang/migration/commands.go:24, each naming two or three registry
// keys inside a longer legitimate list). One string in twelve loses none of
// them and still drops fourteen literals whose share is under it.
const flatKeyShareMin = 8

// bestGroup answers the group of this corpus that keys matches most, and the
// keys it matched. A unit matching two groups is reported against the one it
// restates most of, because one unit copying one set is one finding.
//
// A CLOSED corpus is judged differently, and the difference is the whole reason
// the flag exists. Its groups are enumerations rather than registries: nobody
// transcribes part of an enumeration, so a partial match is a coincidence of
// values and a full match of a set worth transcribing is the copy. A FLAT
// corpus is measured from the other end, by how much of the literal the keys
// are (flatKeyShareMin).
func (c Corpus) bestGroup(keys []string) (string, []string) {
	bestName := ""
	var best []string
	for _, group := range c.Groups {
		matched := intersect(group.Keys, keys)
		if c.Closed && (len(matched) != len(group.Keys) || len(group.Keys) < closedGroupKeysMin) {
			continue
		}
		if !c.Closed && len(matched)*100 < flatKeyShareMin*len(keys) {
			continue
		}
		if len(matched) > len(best) {
			bestName = group.Name
			best = matched
		}
	}
	return bestName, best
}

// intersect answers the members of keys that are also in set, sorted.
func intersect(set, keys []string) []string {
	holds := make(map[string]bool, len(set))
	for _, key := range set {
		holds[key] = true
	}
	var matched []string
	for _, key := range keys {
		if holds[key] {
			matched = append(matched, key)
		}
	}
	slices.Sort(matched)
	return matched
}

// findings turns the candidates into the answer: the innermost unit for each
// copied set, minus what an exemption marker excuses, plus every marker that
// excused nothing.
func (s *scan) findings() (Findings, error) {
	found := s.walkFound
	for _, unit := range s.innermost() {
		key, excused := s.excuse(unit)
		if excused {
			s.matched[key] = true
			continue
		}
		found = append(found, Finding{
			Kind:    KindLiteral,
			File:    unit.file,
			Line:    unit.start,
			Symbol:  unit.symbol,
			Corpus:  unit.corpus,
			Keys:    unit.keys,
			Strings: unit.strings,
			Detail:  unit.detail,
		})
	}

	dead, err := s.deadMarkers()
	if err != nil {
		return nil, err
	}
	found = append(found, dead...)
	found.sort()
	return found, nil
}

// innermost drops a candidate that encloses another candidate for the same set
// in the same file. The reader wants the literal, not the function around it.
func (s *scan) innermost() []candidate {
	kept := make([]candidate, 0, len(s.candidates))
	for _, unit := range s.candidates {
		if s.encloses(unit) {
			continue
		}
		kept = append(kept, unit)
	}
	return kept
}

// encloses reports whether another candidate for the same set sits strictly
// inside this one.
//
// The comparison is on token positions rather than lines. `Section{Entries:
// []Entry{...}}` written on one line is two literals with one line range, and
// on lines neither encloses the other, so both were reported for one copy.
func (s *scan) encloses(outer candidate) bool {
	for _, inner := range s.candidates {
		if inner.file != outer.file || inner.corpus != outer.corpus || inner.group != outer.group {
			continue
		}
		if inner.from < outer.from || inner.to > outer.to {
			continue
		}
		if inner.from > outer.from || inner.to < outer.to {
			return true
		}
	}
	return false
}

// excuse answers the marker that suppresses this unit, if one does. A marker
// counts when it stands on the unit, inside it, or on the line above it.
//
// A marker on a TABLE also excuses the rows of that table. The author marks the
// literal they can see, and the walker is what split it into an outer literal
// and one inner literal for each row: asking for a marker above every row would
// be asking the author to write the walker's decomposition out by hand.
func (s *scan) excuse(unit candidate) (string, bool) {
	if key, found := s.markerBetween(unit.file, unit.start-1, unit.end); found {
		return key, true
	}
	for _, outer := range s.candidates {
		if outer.file != unit.file || outer.corpus != unit.corpus || outer.group != unit.group {
			continue
		}
		if outer.from > unit.from || outer.to < unit.to {
			continue
		}
		if key, found := s.markerBetween(unit.file, outer.start-1, outer.end); found {
			return key, true
		}
	}
	return "", false
}

// markerBetween answers the first marker standing on a line of the range.
func (s *scan) markerBetween(file string, first, last int) (string, bool) {
	for line := first; line <= last; line++ {
		key := markerKey(file, line)
		if _, found := s.markers[key]; found {
			return key, true
		}
	}
	return "", false
}

// deadMarkers accounts for every marker against the units it suppressed.
//
// A marker that suppresses nothing is not untidiness. It states that some
// literal at that place is legitimately written out, and it keeps stating it
// for whatever code arrives there next, with nobody having judged it.
func (s *scan) deadMarkers() (Findings, error) {
	reasons := make(map[string]string, len(s.markers))
	for key, site := range s.markers {
		reasons[key] = site.reason
	}
	coverage, err := population.Exemptions("enumeration exemption markers", reasons, s.matched)
	if err != nil {
		return nil, err
	}
	dead := make(Findings, 0, len(coverage.Unexcused))
	for _, key := range coverage.Unexcused {
		site := s.markers[key]
		dead = append(dead, Finding{
			Kind:   KindMarker,
			File:   site.file,
			Line:   site.line,
			Detail: "the marker suppresses nothing in the tree the walk read: delete it, or move it to the literal it was written for",
		})
	}
	return dead, nil
}
