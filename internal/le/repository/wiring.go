// Design: docs/architecture/core-design.md -- the cross-package wiring check
//
// Overview: repository.go -- the gate these findings belong to
//
// wiring.go answers one question about every exported symbol a session added:
// does anything OUTSIDE its own package call it.
//
// The check uses a whole-word search over internal/, cmd/, and pkg/.
// Five undercounted type shapes have bounded exemptions: constants, struct
// composition, wired constructors, wired setters, and live interface embedding.
// Two more cover interface-only method
// dispatch through an exported package interface or generated gRPC service
// registration.
//
// Every exemption has the same bound. The exported function or interface that
// carries the type must itself have a cross-package caller. Without that bound,
// almost every dead exported type would qualify through a signature.
package repository

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The declaration patterns preserve the prior gate's source contract.
const (
	// ExportedFuncPattern matches an exported func or method declaration. The
	// optional group consumes a receiver, so the parameter list is what
	// follows the match.
	ExportedFuncPattern = `^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(`
	// ExportedTypePattern matches an exported type declaration.
	ExportedTypePattern = `^type\s+([A-Z][A-Za-z0-9_]*)\b`
	// FuncRecvPattern reads the receiver TYPE of a method declaration, named
	// or unnamed, so a concrete implementation can be told from a free
	// function.
	FuncRecvPattern = `^func\s+\(\s*(?:\w+\s+)?\*?(?P<recvtype>[A-Za-z_][A-Za-z0-9_]*)(?:\[[^\]]*\])?\s*\)\s*[A-Z]`
	// ExportedIfacePattern matches the opening line of an exported interface.
	ExportedIfacePattern = `^type\s+[A-Z][A-Za-z0-9_]*\s+interface\s*\{`
	// ExportedIfaceNamedPattern is the same line with the interface's name
	// captured.
	ExportedIfaceNamedPattern = `^type\s+(?P<name>[A-Z][A-Za-z0-9_]*)\s+interface\s*\{`
	// IfaceMethodPattern matches one exported method line of an interface body.
	IfaceMethodPattern = `^\s*([A-Z][A-Za-z0-9_]*)\s*\(`
	// RegisteredServerPattern matches a generated gRPC service registration
	// binding a concrete unexported receiver to an exported service interface.
	RegisteredServerPattern = `\b[A-Za-z_][A-Za-z0-9_]*\.Register(?P<interface>[A-Z][A-Za-z0-9_]*Server)\s*\([^,\n]+,\s*&(?P<receiver>[a-z][A-Za-z0-9_]*)\s*\{`
	// ConstSpecPattern reads one const spec's leading form: the comma-separated
	// names, the explicit type when there is one, and the `=` when a value
	// expression follows.
	ConstSpecPattern = `^\s*([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)(?:\s+([A-Za-z_][A-Za-z0-9_.]*))?(\s*=)?`
	// ConstBlockPattern and ConstLinePattern tell a `const (` block from a
	// single-line const declaration.
	ConstBlockPattern = `^const\s*\(`
	ConstLinePattern  = `^const\s+(.+)$`
)

var (
	exportedFuncRe      = regexp.MustCompile(ExportedFuncPattern)
	exportedIfaceRe     = regexp.MustCompile(ExportedIfacePattern)
	exportedIfaceNameRe = regexp.MustCompile(ExportedIfaceNamedPattern)
	ifaceMethodRe       = regexp.MustCompile(IfaceMethodPattern)
	registeredServerRe  = regexp.MustCompile(RegisteredServerPattern)
	constSpecRe         = regexp.MustCompile(ConstSpecPattern)
	constBlockRe        = regexp.MustCompile(ConstBlockPattern)
	constLineRe         = regexp.MustCompile(ConstLinePattern)
)

// searchDirs lists the first-party trees that can contain callers.
var searchDirs = [...]string{"cmd", "demos", "docker", "internal", "pkg", "tools"}

// dispatchSite names one package and one unexported receiver whose methods are
// called by an external library through an interface it holds.
type dispatchSite struct {
	Package  string
	Receiver string
}

// InterfaceDispatchMethods is the exact list of methods reached that way.
// grpc-go calls them only through stats.Handler, and the concrete handler is
// private. Neither the same-package interface rule nor a cross-package word
// search sees that dispatch.
//
// It stays exact on purpose: another exported method on the same handler still
// needs a caller.
var InterfaceDispatchMethods = map[dispatchSite]map[string]bool{
	{Package: "internal/component/api/grpc", Receiver: "transportCompletionStatsHandler"}: {
		"TagRPC": true, "HandleRPC": true, "TagConn": true, "HandleConn": true,
	},
}

// symbolKind says how a declaration is reached, which decides which exemptions
// apply to it.
type symbolKind int

const (
	kindFunc symbolKind = iota
	kindMethodUnexported
	kindType
)

// symbol is one exported declaration a changed file makes.
type symbol struct {
	file     string
	line     int
	name     string
	pkgDir   string
	kind     symbolKind
	receiver string
}

// scanner holds one run's view of the caller population: the Go files under
// the search dirs, their text read once, and the answers already given.
type scanner struct {
	tree    string
	files   []string
	content map[string]string
	refs    map[string]bool
}

// newScanner lists the caller population once. A tree with none of the four
// search dirs answers no files, which the caller reads as a population it
// cannot judge.
func newScanner(ctx context.Context, tree string) (*scanner, error) {
	s := &scanner{tree: tree, content: make(map[string]string), refs: make(map[string]bool)}
	for _, dir := range searchDirs {
		root := filepath.Join(tree, dir)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}
			rel, relErr := filepath.Rel(tree, name)
			if relErr != nil {
				return relErr
			}
			s.files = append(s.files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(s.files)
	return s, nil
}

// read answers one tree-relative file's text, reading it at most once.
func (s *scanner) read(rel string) (string, error) {
	if text, ok := s.content[rel]; ok {
		return text, nil
	}
	raw, err := os.ReadFile(filepath.Join(s.tree, filepath.FromSlash(rel))) //nolint:gosec // a Go file of the tree the caller named
	if err != nil {
		return "", err
	}
	text := string(raw)
	s.content[rel] = text
	return text, nil
}

// isWordByte reports the characters grep treats as word constituents, which is
// what `grep -w` bounds a match by.
func isWordByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// hasWord reports whether text holds word as a whole word, which is the match
// `grep -w` makes.
func hasWord(text, word string) bool {
	for i := 0; i+len(word) <= len(text); {
		at := strings.Index(text[i:], word)
		if at < 0 {
			return false
		}
		pos := i + at
		before := pos == 0 || !isWordByte(text[pos-1])
		after := pos+len(word) == len(text) || !isWordByte(text[pos+len(word)])
		if before && after {
			return true
		}
		i = pos + 1
	}
	return false
}

// hasCrossPackageRef reports whether sym is named as a whole word by a package
// other than the one that declares it.
//
// Test files do not count, so a symbol called only by tests looks unwired.
// Helper packages under internal/test/ reverse that rule. Their purpose is
// cross-package use from _test.go files, so a TEST caller is their wiring. The
// exemption depends on where the symbol is DEFINED. The calling test must also
// IMPORT the helper. A bare test-file word matched Colors.Red in a comment and
// runner.TestSet in an unrelated test function.
func (s *scanner) hasCrossPackageRef(sym, pkgDir string) (bool, error) {
	var tb textbuf.Buffer
	key := tb.Str(sym).Byte(0).Str(pkgDir).String()
	if answer, ok := s.refs[key]; ok {
		return answer, nil
	}

	definedInTestHelper := strings.HasPrefix(pkgDir, "internal/test/")
	answer := false
	for _, rel := range s.files {
		if path.Dir(rel) == pkgDir {
			continue
		}
		text, err := s.read(rel)
		if err != nil {
			return false, err
		}
		if !hasWord(text, sym) {
			continue
		}
		if !strings.HasSuffix(rel, "_test.go") {
			answer = true
			break
		}
		if !definedInTestHelper {
			continue
		}
		if s.importsPackage(text, pkgDir) {
			answer = true
			break
		}
	}
	s.refs[key] = answer
	return answer, nil
}

// importsPackage reports whether a file's source carries an import path ending
// in pkgDir.
func (s *scanner) importsPackage(text, pkgDir string) bool {
	var tb textbuf.Buffer
	return strings.Contains(text, tb.Byte('/').Str(pkgDir).Byte('"').String())
}

// packageFiles answers the non-test Go files of one package directory, in name
// order.
func (s *scanner) packageFiles(pkgDir string) []string {
	var files []string
	for _, rel := range s.files {
		if path.Dir(rel) != pkgDir || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		files = append(files, rel)
	}
	return files
}

// exportedName reports whether a Go identifier is exported.
func exportedName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

// constSpec parses one const spec into its names, its explicit type, and
// whether it carries a value expression.
func constSpec(text string) (names []string, explicitType string, hasValue bool) {
	match := constSpecRe.FindStringSubmatch(text)
	if len(match) < 4 || match[1] == "" {
		return nil, "", false
	}
	for name := range strings.SplitSeq(match[1], ",") {
		names = append(names, strings.TrimSpace(name))
	}
	return names, match[2], match[3] != ""
}

// exportedConstsOfType answers the exported constants declared with typeName in
// pkgDir.
//
// A typed enum is reached through its values, so another package need not use
// the bare type name. Callers can switch on RouteVerbInstall and omit
// RouteVerb, which makes the word search undercount. This check handles
// single-line and block consts, multi-name specs, and iota. A bare spec inherits
// the prior explicit type. An untyped `= expr` resets that type, as Go specifies.
func (s *scanner) exportedConstsOfType(pkgDir, typeName string) ([]string, error) {
	seen := make(map[string]bool)
	var names []string
	add := func(specNames []string) {
		for _, name := range specNames {
			if !exportedName(name) || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}

	for _, rel := range s.packageFiles(pkgDir) {
		text, err := s.read(rel)
		if err != nil {
			return nil, err
		}
		inBlock, inherits := false, false
		for line := range strings.SplitSeq(text, "\n") {
			stripped := strings.TrimSpace(line)
			if !inBlock {
				if constBlockRe.MatchString(stripped) {
					inBlock, inherits = true, false
					continue
				}
				if match := constLineRe.FindStringSubmatch(stripped); match != nil {
					specNames, explicitType, _ := constSpec(match[1])
					if explicitType == typeName {
						add(specNames)
					}
				}
				continue
			}
			if strings.HasPrefix(stripped, ")") {
				inBlock, inherits = false, false
				continue
			}
			if stripped == "" || strings.HasPrefix(stripped, "//") {
				continue
			}
			specNames, explicitType, hasValue := constSpec(line)
			if len(specNames) == 0 {
				continue
			}
			switch {
			case explicitType != "":
				inherits = explicitType == typeName
				if inherits {
					add(specNames)
				}
			case hasValue:
				// The value gives the constant its own type, which ends the
				// inheritance an untyped spec would otherwise carry.
				inherits = false
			case inherits:
				add(specNames)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// typeUsedAsFieldInPackage reports whether typeName is a struct field type
// inside its own package.
//
// A type composed into a struct is reached through field access (inv.CPU,
// cap.Families), so its bare name need not appear in any other package. The
// search is scoped to the declaring package: a field declaration elsewhere
// would name the type, which the word search already covers.
func (s *scanner) typeUsedAsFieldInPackage(pkgDir, typeName string) (bool, error) {
	var tb textbuf.Buffer
	fieldRe, err := regexp.Compile(tb.Str(`^\s*[A-Z][A-Za-z0-9_]*\s+(?:\[\]|\*|map\[[^\]]+\]|chan\s+)*\*?`).
		Str(regexp.QuoteMeta(typeName)).Str(`\b`).String())
	if err != nil {
		return false, err
	}

	for _, rel := range s.packageFiles(pkgDir) {
		text, readErr := s.read(rel)
		if readErr != nil {
			return false, readErr
		}
		structDepth := 0
		for line := range strings.SplitSeq(text, "\n") {
			if structDepth > 0 {
				if fieldRe.MatchString(line) {
					return true, nil
				}
				structDepth = max(0, structDepth+strings.Count(line, "{")-strings.Count(line, "}"))
				continue
			}
			if strings.Contains(line, "struct {") {
				structDepth = max(0, strings.Count(line, "{")-strings.Count(line, "}"))
			}
		}
	}
	return false, nil
}

// funcSignature answers an exported function or method name, parameters, and
// return signature. It returns false for any other line.
//
// Balancing starts at the parameter list's opening parenthesis. Thus,
// function-typed parameters and multi-value returns are handled. The declaration
// pattern consumes the receiver before the scan balances the signature.
func funcSignature(line string) (name, params, returns string, ok bool) {
	at := exportedFuncRe.FindStringSubmatchIndex(line)
	if at == nil {
		return "", "", "", false
	}
	start := at[1] - 1 // the '(' that opens the parameter list
	depth, i := 0, start
	for ; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && line[i] == ')' {
			break
		}
	}
	if i >= len(line) {
		// An unbalanced declaration line: the script slices past the end and
		// gets an empty tail, which is what these two empty strings are.
		return line[at[2]:at[3]], line[start+1:], "", true
	}
	tail, _, _ := strings.Cut(line[i+1:], "{")
	return line[at[2]:at[3]], line[start+1 : i], tail, true
}

// mentionsType reports whether part names typeName as a standalone word that is
// not a selector's tail.
//
// It is the script's `(?<![\w.])NAME\b`, which RE2 cannot express: a lookbehind
// is what stops `pkg.Report` and `myReport` both counting as a mention of
// Report.
func mentionsType(part, typeName string) bool {
	for i := 0; i+len(typeName) <= len(part); {
		at := strings.Index(part[i:], typeName)
		if at < 0 {
			return false
		}
		pos := i + at
		before := pos == 0 || (!isWordByte(part[pos-1]) && part[pos-1] != '.')
		after := pos+len(typeName) == len(part) || !isWordByte(part[pos+len(typeName)])
		if before && after {
			return true
		}
		i = pos + 1
	}
	return false
}

// signaturePart selects which half of a signature an exemption reads.
type signaturePart int

const (
	partParameters signaturePart = iota
	partReturns
)

// wiredFuncNamesType reports whether a called exported function in the same
// package names typeName in the selected signature half.
//
// Requiring a cross-package caller for the CONTAINING function bounds both
// exemptions. Otherwise, any signature CAN exempt its types.
func (s *scanner) wiredFuncNamesType(pkgDir, typeName string, part signaturePart) (bool, error) {
	for _, rel := range s.packageFiles(pkgDir) {
		text, err := s.read(rel)
		if err != nil {
			return false, err
		}
		for line := range strings.SplitSeq(text, "\n") {
			name, params, returns, ok := funcSignature(line)
			if !ok {
				continue
			}
			chosen := params
			if part == partReturns {
				chosen = returns
			}
			if !mentionsType(chosen, typeName) {
				continue
			}
			if name == typeName {
				continue // its own constructor name collision
			}
			wired, refErr := s.hasCrossPackageRef(name, pkgDir)
			if refErr != nil {
				return false, refErr
			}
			if wired {
				return true, nil
			}
		}
	}
	return false, nil
}

// packageExportedInterfaceMethods answers the method names declared by exported
// interfaces of pkgDir's non-test files.
//
// A method satisfying an exported interface is reached through dispatch, which
// the word search cannot see. Combined with an UNEXPORTED receiver, which no
// other package can name, such a method is wired through the interface rather
// than dead.
func (s *scanner) packageExportedInterfaceMethods(pkgDir string) (map[string]bool, error) {
	names := make(map[string]bool)
	for _, rel := range s.packageFiles(pkgDir) {
		text, err := s.read(rel)
		if err != nil {
			return nil, err
		}
		depth, inIface := 0, false
		for line := range strings.SplitSeq(text, "\n") {
			if !inIface {
				if exportedIfaceRe.MatchString(line) {
					depth = strings.Count(line, "{") - strings.Count(line, "}")
					inIface = depth > 0
				}
				continue
			}
			if match := ifaceMethodRe.FindStringSubmatch(line); match != nil {
				names[match[1]] = true
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if depth <= 0 {
				inIface = false
			}
		}
	}
	return names, nil
}

// typeEmbeddedInWiredInterface reports whether a live exported interface in the
// same package embeds typeName.
//
// Interface embedding carries the embedded contract when no caller names it.
// The outer interface needs a cross-package production reference, or one dead
// interface CAN hide another.
func (s *scanner) typeEmbeddedInWiredInterface(pkgDir, typeName string) (bool, error) {
	for _, rel := range s.packageFiles(pkgDir) {
		text, err := s.read(rel)
		if err != nil {
			return false, err
		}
		outer, depth := "", 0
		for line := range strings.SplitSeq(text, "\n") {
			if depth <= 0 {
				match := exportedIfaceNameRe.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				outer = match[1]
				depth = strings.Count(line, "{") - strings.Count(line, "}")
				continue
			}
			if strings.TrimSpace(line) == typeName {
				wired, refErr := s.hasCrossPackageRef(outer, pkgDir)
				if refErr != nil {
					return false, refErr
				}
				if wired {
					return true, nil
				}
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
		}
	}
	return false, nil
}

// registeredInterfaceMethodsByReceiver answers the exported API methods that
// each unexported pkgDir receiver reaches through generated gRPC registration.
//
// Generated clients call the service interface instead of the concrete private
// implementation. Binding that implementation to Register*Server is the
// production site that makes its interface methods reachable.
func (s *scanner) registeredInterfaceMethodsByReceiver(pkgDir string) (map[string]map[string]bool, error) {
	type registration struct{ receiver, iface string }
	var registrations []registration
	seen := make(map[registration]bool)
	for _, rel := range s.packageFiles(pkgDir) {
		text, err := s.read(rel)
		if err != nil {
			return nil, err
		}
		for _, match := range registeredServerRe.FindAllStringSubmatch(text, -1) {
			found := registration{receiver: match[2], iface: match[1]}
			if seen[found] {
				continue
			}
			seen[found] = true
			registrations = append(registrations, found)
		}
	}
	if len(registrations) == 0 {
		return map[string]map[string]bool{}, nil
	}

	wanted := make(map[string]*regexp.Regexp)
	for _, found := range registrations {
		if _, ok := wanted[found.iface]; ok {
			continue
		}
		var tb textbuf.Buffer
		pattern, err := regexp.Compile(tb.Str(`(?m)^type\s+`).Str(regexp.QuoteMeta(found.iface)).
			Str(`\s+interface\s*\{`).String())
		if err != nil {
			return nil, err
		}
		wanted[found.iface] = pattern
	}

	methodsByInterface := make(map[string]map[string]bool)
	for _, dir := range [...]string{"api", "internal", "pkg"} {
		root := filepath.Join(s.tree, dir)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		var goFiles []string
		err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(s.tree, name)
			if relErr != nil {
				return relErr
			}
			goFiles = append(goFiles, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
		sortByComponent(goFiles)

		for _, rel := range goFiles {
			text, readErr := s.read(rel)
			if readErr != nil {
				return nil, readErr
			}
			for iface, pattern := range wanted {
				at := pattern.FindStringIndex(text)
				if at == nil {
					continue
				}
				names, ok := methodsByInterface[iface]
				if !ok {
					names = make(map[string]bool)
					methodsByInterface[iface] = names
				}
				depth := 1
				for line := range strings.SplitSeq(text[at[1]:], "\n") {
					if match := ifaceMethodRe.FindStringSubmatch(line); match != nil {
						names[match[1]] = true
					}
					depth += strings.Count(line, "{") - strings.Count(line, "}")
					if depth <= 0 {
						break
					}
				}
			}
		}
	}

	byReceiver := make(map[string]map[string]bool, len(registrations))
	for _, found := range registrations {
		names, ok := byReceiver[found.receiver]
		if !ok {
			names = make(map[string]bool)
			byReceiver[found.receiver] = names
		}
		for name := range methodsByInterface[found.iface] {
			names[name] = true
		}
	}
	return byReceiver, nil
}

// declaredSymbols answers every exported declaration of the changed Go files.
func declaredSymbols(tree string, changed []string) ([]symbol, error) {
	var symbols []symbol
	for _, rel := range changed {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if strings.Contains(filepath.ToSlash(rel), "/testdata/") {
			continue
		}
		if !strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "cmd/") {
			continue
		}
		pathname := filepath.Join(tree, filepath.FromSlash(rel))
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, pathname, nil, 0)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		pkgDir := path.Dir(rel)
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if !typed.Name.IsExported() {
					continue
				}
				found := symbol{
					file: rel, line: fset.Position(typed.Name.Pos()).Line,
					name: typed.Name.Name, pkgDir: pkgDir, kind: kindFunc,
				}
				if typed.Recv != nil && len(typed.Recv.List) != 0 {
					found.receiver = receiverTypeName(typed.Recv.List[0].Type)
					if found.receiver != "" && !exportedName(found.receiver) {
						found.kind = kindMethodUnexported
					}
				}
				symbols = append(symbols, found)
			case *ast.GenDecl:
				if typed.Tok != token.TYPE {
					continue
				}
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}
					symbols = append(symbols, symbol{
						file: rel, line: fset.Position(typeSpec.Name.Pos()).Line,
						name: typeSpec.Name.Name, pkgDir: pkgDir, kind: kindType,
					})
				}
			}
		}
	}
	return symbols, nil
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexListExpr:
		return receiverTypeName(typed.X)
	default:
		return ""
	}
}

// checkCrossPackageWiring reports every exported symbol a changed file declares
// that nothing outside its package calls.
func checkCrossPackageWiring(ctx context.Context, tree string, changed []string) ([]Finding, error) {
	symbols, err := declaredSymbols(tree, changed)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	scan, err := newScanner(ctx, tree)
	if err != nil {
		return nil, err
	}
	if len(scan.files) == 0 {
		// No search directory exists, so no caller can be found. Every symbol
		// would become a finding. The script and this port answer nothing.
		return nil, nil
	}

	registered := make(map[string]map[string]map[string]bool)
	var findings []Finding
	for _, found := range symbols {
		// A *ForTest helper exists for calls from another package's tests. The
		// caller search deliberately excludes test files, so the name declares
		// that intent.
		if strings.HasSuffix(found.name, "ForTest") {
			continue
		}

		wired, refErr := scan.hasCrossPackageRef(found.name, found.pkgDir)
		if refErr != nil {
			return nil, refErr
		}
		if wired {
			continue
		}

		exempt, exemptErr := scan.exemptType(found)
		if exemptErr != nil {
			return nil, exemptErr
		}
		if exempt {
			continue
		}

		if found.kind == kindMethodUnexported {
			methods, methodErr := scan.packageExportedInterfaceMethods(found.pkgDir)
			if methodErr != nil {
				return nil, methodErr
			}
			if methods[found.name] {
				continue
			}

			byReceiver, ok := registered[found.pkgDir]
			if !ok {
				byReceiver, methodErr = scan.registeredInterfaceMethodsByReceiver(found.pkgDir)
				if methodErr != nil {
					return nil, methodErr
				}
				registered[found.pkgDir] = byReceiver
			}
			if found.receiver != "" && byReceiver[found.receiver][found.name] {
				continue
			}
		}

		if found.receiver != "" &&
			InterfaceDispatchMethods[dispatchSite{Package: found.pkgDir, Receiver: found.receiver}][found.name] {
			continue
		}

		var tb textbuf.Buffer
		findings = append(findings, Finding{
			Severity: severityIssue, File: found.file, Line: found.line,
			Message: tb.Str("exported symbol ").Str(found.name).
				Str(" has no cross-package non-test caller").String(),
		})
	}
	return findings, nil
}

// exemptType reports whether a TYPE is reached by one of the five seams a bare
// word search cannot see.
func (s *scanner) exemptType(found symbol) (bool, error) {
	if found.kind != kindType {
		return false, nil
	}

	consts, err := s.exportedConstsOfType(found.pkgDir, found.name)
	if err != nil {
		return false, err
	}
	for _, name := range consts {
		wired, refErr := s.hasCrossPackageRef(name, found.pkgDir)
		if refErr != nil {
			return false, refErr
		}
		if wired {
			return true, nil
		}
	}

	seams := []func() (bool, error){
		func() (bool, error) { return s.typeUsedAsFieldInPackage(found.pkgDir, found.name) },
		func() (bool, error) { return s.wiredFuncNamesType(found.pkgDir, found.name, partReturns) },
		func() (bool, error) { return s.wiredFuncNamesType(found.pkgDir, found.name, partParameters) },
		func() (bool, error) { return s.typeEmbeddedInWiredInterface(found.pkgDir, found.name) },
	}
	for _, seam := range seams {
		exempt, seamErr := seam()
		if seamErr != nil {
			return false, seamErr
		}
		if exempt {
			return true, nil
		}
	}
	return false, nil
}
