// Design: docs/architecture/plugin/plugin-system.md -- compiled-in plugin bus send coverage
// Related: actions.go -- wires CheckPluginSends into `le cli dispatch check`
// Related: report.go -- KindUndeclaredSend finding format and report rendering
// Related: resolver.go -- stringLit, selectorConst, qualifiedConsts, treeModulePath

package clidispatch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	pluginRegistryImportPath = "github.com/ze-software/ze/internal/component/plugin/registry"
	routeInstallImportPath   = "github.com/ze-software/ze/internal/core/rib/routeinstall"
	ribEventsImportPath      = "github.com/ze-software/ze/internal/core/bgp/ribevents"

	msgBGPUpdate            = bgpevents.Namespace + ":" + bgpevents.SendUpdate
	msgBGPRefresh           = bgpevents.Namespace + ":" + bgpevents.SendRefresh
	msgBGPRaw               = bgpevents.Namespace + ":" + bgpevents.SendRaw
	msgSystemRIBRouteInst   = sysribevents.Namespace + ":" + sysribevents.EventRouteInstall
	msgSystemRIBRouteMetric = sysribevents.Namespace + ":" + sysribevents.EventRouteMetrics
	msgResolveDNS           = "resolve:dns"
	cliMessagePrefix        = events.NamespaceCLI + ":"

	internalDir    = "internal"
	pkgDir         = "pkg"
	goFileExt      = ".go"
	testFileSuffix = "_test.go"
	fieldName      = "Name"
	fieldCommand   = "Command"
	peerMarkerBoRR = "borr"
	peerMarkerEoRR = "eorr"

	// maxWrapperHops bounds transitive same-package dispatch wrapper propagation.
	maxWrapperHops = 4
	// maxExprDepth bounds recursive AST prefix evaluation.
	maxExprDepth = 16
)

// staticPrefix is a statically known string or leading prefix evaluated from an
// AST expression. complete is true when no dynamic suffix follows text.
type staticPrefix struct {
	text     string
	complete bool
}

// wrapperSpec records which argument of a dispatch function or struct field
// carries the CLI command.
type wrapperSpec struct {
	cmdArgIdx     int
	isInputStruct bool
	isWithTimeout bool
}

// requiredSend is one bus send or CLI command prefix required by a static call
// site inside a compiled-in plugin package.
type requiredSend struct {
	file    string
	line    int
	call    string
	message string
	isCLI   bool
}

// textBufStep is one fluent method call in a textbuf.Buffer chain.
type textBufStep struct {
	method string
	args   []ast.Expr
}

// pluginFile holds one parsed non-test Go source file owned by a plugin.
type pluginFile struct {
	rel             string
	dir             string
	file            *ast.File
	pkgConsts       map[string]string
	importedConsts  map[string]map[string]string
	routeInstallPkg string
	ribEventsPkg    string
}

// regCandidate is a parsed file under internal/ that imports plugin/registry.
type regCandidate struct {
	dir   string
	alias string
	file  *ast.File
}

// pluginFuncEnv holds function-local static string, textbuf, and
// DispatchCommandInput bindings while walking statements in order.
type pluginFuncEnv struct {
	strVars        map[string]staticPrefix
	bufVars        map[string]staticPrefix
	inputVars      map[string]staticPrefix
	pkgConsts      map[string]string
	importedConsts map[string]map[string]string
}

func newPluginFuncEnv(pkgConsts map[string]string, imported map[string]map[string]string) *pluginFuncEnv {
	return &pluginFuncEnv{
		strVars:        make(map[string]staticPrefix),
		bufVars:        make(map[string]staticPrefix),
		inputVars:      make(map[string]staticPrefix),
		pkgConsts:      pkgConsts,
		importedConsts: imported,
	}
}

func (e *pluginFuncEnv) clone() *pluginFuncEnv {
	return &pluginFuncEnv{
		strVars:        maps.Clone(e.strVars),
		bufVars:        maps.Clone(e.bufVars),
		inputVars:      maps.Clone(e.inputVars),
		pkgConsts:      e.pkgConsts,
		importedConsts: e.importedConsts,
	}
}

// CheckPluginSends verifies that every compiled-in plugin's Registration.Sends
// covers every static engine operation and CLI command path invoked by its
// package directory (and subpackages that do not declare their own
// registry.Register).
//
// It returns one KindUndeclaredSend finding per uncovered call site, the count
// of plugins checked, and an error if tree cannot be read or any registered
// plugin cannot be mapped to a package directory under tree/internal.
func CheckPluginSends(tree string, regs []*registry.Registration) ([]Finding, int, error) {
	if len(regs) == 0 {
		return nil, 0, fmt.Errorf("plugin send check: no plugins registered -- refusing to pass vacuously")
	}

	module, err := treeModulePath(tree)
	if err != nil {
		return nil, 0, err
	}

	fset := token.NewFileSet()
	constsByDir, packagesByDir, candidates, pendingConsts, err := indexPluginTree(tree, fset)
	if err != nil {
		return nil, 0, err
	}
	qualified := qualifiedConsts{
		tree:     tree,
		module:   module,
		consts:   constsByDir,
		packages: packagesByDir,
	}
	resolvePendingStringConsts(pendingConsts, constsByDir, qualified)

	dirByPlugin := make(map[string]string, len(candidates))
	registeredDirs := make(map[string]bool, len(candidates))
	for _, cand := range candidates {
		names := extractRegisteredNames(cand.file, cand.alias, constsByDir[cand.dir], qualified.forFile(cand.file))
		for _, name := range names {
			dirByPlugin[name] = cand.dir
			registeredDirs[cand.dir] = true
		}
	}

	ordered := make([]*registry.Registration, 0, len(regs))
	for _, reg := range regs {
		if reg == nil {
			return nil, 0, fmt.Errorf("plugin send check: nil registration in input")
		}
		ordered = append(ordered, reg)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Name < ordered[j].Name
	})

	var findings []Finding
	for _, reg := range ordered {
		pluginDir, ok := dirByPlugin[reg.Name]
		if !ok {
			return nil, 0, fmt.Errorf("plugin send check: registered plugin %q has no registry.Register call under %s",
				reg.Name, filepath.Join(tree, internalDir))
		}

		files, collectErr := collectPluginFiles(tree, pluginDir, registeredDirs, fset, constsByDir, qualified)
		if collectErr != nil {
			return nil, 0, collectErr
		}
		findings = append(findings, checkPluginFiles(reg, files, fset)...)
	}

	sortPluginFindings(findings)
	return findings, len(ordered), nil
}

type pendingStringConst struct {
	dir  string
	file *ast.File
	name string
	expr ast.Expr
}

// indexPluginTree walks tree/internal (and tree/pkg when present) once to
// gather package-level string constants, package names, and every file under
// internal/ that imports plugin/registry.
func indexPluginTree(tree string, fset *token.FileSet) (
	map[string]map[string]string, map[string]string, []regCandidate, []pendingStringConst, error,
) {
	roots := []string{filepath.Join(tree, internalDir)}
	pkgPath := filepath.Join(tree, pkgDir)
	if _, statErr := os.Stat(pkgPath); statErr == nil {
		roots = append(roots, pkgPath)
	} else if !os.IsNotExist(statErr) {
		return nil, nil, nil, nil, statErr
	}

	constsByDir := make(map[string]map[string]string)
	packages := make(map[string]string)
	var candidates []regCandidate
	var pending []pendingStringConst

	for _, root := range roots {
		isInternalRoot := root == roots[0]
		walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == testdataDir {
					return filepath.SkipDir
				}
				return nil
			}
			if !isNonTestGoFile(path) {
				return nil
			}
			src, readErr := os.ReadFile(path) //nolint:gosec // repository path
			if readErr != nil {
				return readErr
			}
			file, parseErr := parser.ParseFile(fset, path, src, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", path, parseErr)
			}
			dir := filepath.Dir(path)
			packages[dir] = file.Name.Name
			pending = append(pending, collectFileStringConsts(file, dir, constsByDir)...)

			if !isInternalRoot {
				return nil
			}
			relPath, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			rel := filepath.ToSlash(relPath)
			if isExcludedPluginTree(rel) {
				return nil
			}
			if alias, imported := findImportAlias(file, pluginRegistryImportPath); imported {
				candidates = append(candidates, regCandidate{dir: dir, alias: alias, file: file})
			}
			return nil
		})
		if walkErr != nil {
			return nil, nil, nil, nil, walkErr
		}
	}
	return constsByDir, packages, candidates, pending, nil
}

func isNonTestGoFile(path string) bool {
	if !strings.HasSuffix(path, goFileExt) {
		return false
	}
	return !strings.HasSuffix(path, testFileSuffix)
}

func isExcludedPluginTree(rel string) bool {
	if strings.HasPrefix(rel, "internal/test/") {
		return true
	}
	return strings.HasPrefix(rel, "internal/le/")
}

func collectFileStringConsts(file *ast.File, dir string, out map[string]map[string]string) []pendingStringConst {
	var pending []pendingStringConst
	for _, decl := range file.Decls {
		general, isGeneral := decl.(*ast.GenDecl)
		if !isGeneral {
			continue
		}
		if general.Tok != token.CONST && general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			values, isValues := spec.(*ast.ValueSpec)
			if !isValues {
				continue
			}
			for i, ident := range values.Names {
				if i >= len(values.Values) {
					continue
				}
				lit, isLit := stringLit(values.Values[i])
				if !isLit {
					if isResolvableStringExpr(values.Values[i]) {
						pending = append(pending, pendingStringConst{
							dir:  dir,
							file: file,
							name: ident.Name,
							expr: values.Values[i],
						})
					}
					continue
				}
				if out[dir] == nil {
					out[dir] = make(map[string]string)
				}
				out[dir][ident.Name] = lit
			}
		}
	}
	return pending
}

func isResolvableStringExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.BinaryExpr, *ast.ParenExpr:
		return true
	default:
		return false
	}
}

func resolvePendingStringConsts(pending []pendingStringConst, constsByDir map[string]map[string]string, qualified qualifiedConsts) {
	for range maxWrapperHops {
		added := false
		for _, item := range pending {
			if constsByDir[item.dir] != nil {
				if _, exists := constsByDir[item.dir][item.name]; exists {
					continue
				}
			}
			val, ok := evalStaticPrefix(item.expr, newPluginFuncEnv(constsByDir[item.dir], qualified.forFile(item.file)), 0)
			if !ok {
				continue
			}
			if !val.complete {
				continue
			}
			if constsByDir[item.dir] == nil {
				constsByDir[item.dir] = make(map[string]string)
			}
			constsByDir[item.dir][item.name] = val.text
			added = true
		}
		if !added {
			break
		}
	}
}

func findImportAlias(file *ast.File, importPath string) (string, bool) {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if path != importPath {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" {
				return "", false
			}
			if spec.Name.Name == "." {
				return "", false
			}
			return spec.Name.Name, true
		}
		last := importPath
		if slash := strings.LastIndexByte(importPath, '/'); slash >= 0 {
			last = importPath[slash+1:]
		}
		return last, true
	}
	return "", false
}

func extractRegisteredNames(file *ast.File, registryAlias string, pkgConsts map[string]string,
	imported map[string]map[string]string,
) []string {
	var names []string
	for _, decl := range file.Decls {
		fn, isFn := decl.(*ast.FuncDecl)
		if !isFn {
			continue
		}
		if fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			if len(call.Args) != 1 {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			if sel.Sel.Name != "Register" {
				return true
			}
			recv, isIdent := sel.X.(*ast.Ident)
			if !isIdent {
				return true
			}
			if recv.Name != registryAlias {
				return true
			}
			nameExpr, ok := registrationNameExpr(call.Args[0], fn.Body, file)
			if !ok {
				return true
			}
			val, resolved := evalStaticPrefix(nameExpr, newPluginFuncEnv(pkgConsts, imported), 0)
			if !resolved {
				return true
			}
			if val.complete && val.text != "" {
				names = append(names, val.text)
			}
			return true
		})
	}
	return names
}

func registrationNameExpr(arg ast.Expr, body *ast.BlockStmt, file *ast.File) (ast.Expr, bool) {
	if lit, isLit := unwrapUnaryAddr(arg).(*ast.CompositeLit); isLit {
		return compositeFieldExpr(lit, fieldName)
	}
	ident, isIdent := arg.(*ast.Ident)
	if !isIdent {
		return nil, false
	}

	var found ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				if i >= len(stmt.Rhs) {
					continue
				}
				if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
					if expr, has := compositeFieldFromExpr(stmt.Rhs[i], fieldName); has {
						found = expr
					}
				}
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == fieldName {
					if recv, isRecv := sel.X.(*ast.Ident); isRecv && recv.Name == ident.Name {
						found = stmt.Rhs[i]
					}
				}
			}
		case *ast.ValueSpec:
			if expr, has := valueSpecFieldExpr(stmt, ident.Name, fieldName); has {
				found = expr
			}
		}
		return true
	})
	if found != nil {
		return found, true
	}

	for _, decl := range file.Decls {
		general, isGeneral := decl.(*ast.GenDecl)
		if !isGeneral {
			continue
		}
		if general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			values, isValues := spec.(*ast.ValueSpec)
			if !isValues {
				continue
			}
			if expr, has := valueSpecFieldExpr(values, ident.Name, fieldName); has {
				return expr, true
			}
		}
	}
	return nil, false
}

func valueSpecFieldExpr(values *ast.ValueSpec, varName, key string) (ast.Expr, bool) {
	for i, name := range values.Names {
		if name.Name != varName {
			continue
		}
		if i >= len(values.Values) {
			continue
		}
		if expr, has := compositeFieldFromExpr(values.Values[i], key); has {
			return expr, true
		}
	}
	return nil, false
}

func compositeFieldFromExpr(expr ast.Expr, key string) (ast.Expr, bool) {
	lit, isLit := unwrapUnaryAddr(expr).(*ast.CompositeLit)
	if !isLit {
		return nil, false
	}
	return compositeFieldExpr(lit, key)
}

func unwrapUnaryAddr(expr ast.Expr) ast.Expr {
	if unary, isUnary := expr.(*ast.UnaryExpr); isUnary && unary.Op == token.AND {
		return unary.X
	}
	return expr
}

func compositeFieldExpr(lit *ast.CompositeLit, key string) (ast.Expr, bool) {
	for _, elt := range lit.Elts {
		pair, isPair := elt.(*ast.KeyValueExpr)
		if !isPair {
			continue
		}
		ident, isIdent := pair.Key.(*ast.Ident)
		if !isIdent {
			continue
		}
		if ident.Name == key {
			return pair.Value, true
		}
	}
	return nil, false
}

func collectPluginFiles(tree, pluginDir string, registeredDirs map[string]bool, fset *token.FileSet,
	constsByDir map[string]map[string]string, qualified qualifiedConsts,
) ([]pluginFile, error) {
	var files []pluginFile
	walkErr := filepath.WalkDir(pluginDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == testdataDir {
				return filepath.SkipDir
			}
			if path != pluginDir && registeredDirs[path] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isNonTestGoFile(path) {
			return nil
		}
		src, readErr := os.ReadFile(path) //nolint:gosec // repository path
		if readErr != nil {
			return readErr
		}
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		relPath, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return relErr
		}
		dir := filepath.Dir(path)
		riAlias, _ := findImportAlias(file, routeInstallImportPath)
		reAlias, _ := findImportAlias(file, ribEventsImportPath)
		files = append(files, pluginFile{
			rel:             filepath.ToSlash(relPath),
			dir:             dir,
			file:            file,
			pkgConsts:       constsByDir[dir],
			importedConsts:  qualified.forFile(file),
			routeInstallPkg: riAlias,
			ribEventsPkg:    reAlias,
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}

func checkPluginFiles(reg *registry.Registration, files []pluginFile, fset *token.FileSet) []Finding {
	funcWrappers, fieldWrappers := discoverPackageWrappers(files)

	var reqs []requiredSend
	for _, pf := range files {
		reqs = append(reqs, scanPluginFile(pf, fset, funcWrappers, fieldWrappers)...)
	}

	type dedupKey struct {
		file    string
		line    int
		call    string
		message string
	}
	seen := make(map[dedupKey]bool, len(reqs))
	var findings []Finding
	for _, req := range reqs {
		if sendDeclCovers(reg.Sends, req) {
			continue
		}
		key := dedupKey{file: req.file, line: req.line, call: req.call, message: req.message}
		if seen[key] {
			continue
		}
		seen[key] = true
		displayMsg := req.message
		if req.isCLI {
			displayMsg = cliMessagePrefix + req.message
		}
		findings = append(findings, Finding{
			File:    req.file,
			Line:    req.line,
			Kind:    KindUndeclaredSend,
			Emitter: req.call,
			Command: displayMsg,
			Detail: opererr.New(
				formatUndeclaredDetail(reg.Name, req.call, displayMsg),
				"every compiled-in plugin must declare in Registration.Sends every engine operation and CLI command it sends",
				"./le cli dispatch check",
			).Error(),
		})
	}
	return findings
}

func funcDeclsOf(files []pluginFile) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fn, isFn := decl.(*ast.FuncDecl)
			if !isFn {
				continue
			}
			if fn.Body == nil {
				continue
			}
			out = append(out, fn)
		}
	}
	return out
}

// discoverPackageWrappers finds same-package helper functions and struct fields
// that forward a command argument to an SDK DispatchCommand* method. Bounded to
// maxWrapperHops transitive hops.
func discoverPackageWrappers(files []pluginFile) (map[string]wrapperSpec, map[string]wrapperSpec) {
	funcWrappers := make(map[string]wrapperSpec)
	fieldWrappers := make(map[string]wrapperSpec)
	decls := funcDeclsOf(files)

	declsByName := make(map[string][]*ast.FuncDecl, len(decls))
	for _, fn := range decls {
		declsByName[fn.Name.Name] = append(declsByName[fn.Name.Name], fn)
	}
	for _, fn := range decls {
		discoverFieldWiringInBody(fn.Body, declsByName, fieldWrappers)
	}

	for range maxWrapperHops {
		added := false
		for _, fn := range decls {
			if discoverFieldClosuresInBody(fn.Body, funcWrappers, fieldWrappers) {
				added = true
			}
			if _, exists := funcWrappers[fn.Name.Name]; exists {
				continue
			}
			if spec, ok := forwardedDispatchParam(fn.Body, funcParamIndices(fn.Type.Params), funcWrappers, fieldWrappers); ok {
				funcWrappers[fn.Name.Name] = spec
				added = true
			}
		}
		if !added {
			break
		}
	}
	return funcWrappers, fieldWrappers
}

func discoverFieldWiringInBody(body *ast.BlockStmt, declsByName map[string][]*ast.FuncDecl, fieldWrappers map[string]wrapperSpec) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if i >= len(n.Rhs) {
					continue
				}
				sel, isSel := lhs.(*ast.SelectorExpr)
				if !isSel {
					continue
				}
				if spec, ok := sdkDispatchMethodValue(n.Rhs[i]); ok {
					fieldWrappers[sel.Sel.Name] = spec
				}
			}
		case *ast.CallExpr:
			calleeName := callTargetName(n.Fun)
			if calleeName == "" {
				return true
			}
			for argIdx, arg := range n.Args {
				spec, ok := sdkDispatchMethodValue(arg)
				if !ok {
					continue
				}
				for _, callee := range declsByName[calleeName] {
					if paramName, hasParam := paramNameAtIndex(callee.Type.Params, argIdx); hasParam {
						recordFieldAssignedFromParam(callee.Body, paramName, spec, fieldWrappers)
					}
				}
			}
		}
		return true
	})
}

func recordFieldAssignedFromParam(body *ast.BlockStmt, paramName string, spec wrapperSpec, fieldWrappers map[string]wrapperSpec) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.KeyValueExpr:
			key, isKey := n.Key.(*ast.Ident)
			val, isVal := n.Value.(*ast.Ident)
			if isKey && isVal && val.Name == paramName {
				fieldWrappers[key.Name] = spec
			}
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if i >= len(n.Rhs) {
					continue
				}
				sel, isSel := lhs.(*ast.SelectorExpr)
				val, isVal := n.Rhs[i].(*ast.Ident)
				if isSel && isVal && val.Name == paramName {
					fieldWrappers[sel.Sel.Name] = spec
				}
			}
		}
		return true
	})
}

func discoverFieldClosuresInBody(body *ast.BlockStmt, funcWrappers, fieldWrappers map[string]wrapperSpec) bool {
	added := false
	ast.Inspect(body, func(node ast.Node) bool {
		assign, isAssign := node.(*ast.AssignStmt)
		if !isAssign {
			return true
		}
		for i, lhs := range assign.Lhs {
			if i >= len(assign.Rhs) {
				continue
			}
			sel, isSel := lhs.(*ast.SelectorExpr)
			if !isSel {
				continue
			}
			if _, exists := fieldWrappers[sel.Sel.Name]; exists {
				continue
			}
			fnLit, isFnLit := assign.Rhs[i].(*ast.FuncLit)
			if !isFnLit {
				continue
			}
			if fnLit.Body == nil {
				continue
			}
			if spec, ok := forwardedDispatchParam(fnLit.Body, funcParamIndices(fnLit.Type.Params), funcWrappers, fieldWrappers); ok {
				fieldWrappers[sel.Sel.Name] = spec
				added = true
			}
		}
		return true
	})
	return added
}

func forwardedDispatchParam(body *ast.BlockStmt, paramIdx map[string]int,
	funcWrappers, fieldWrappers map[string]wrapperSpec,
) (wrapperSpec, bool) {
	var matched wrapperSpec
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		spec, ok := matchDispatchCall(call, funcWrappers, fieldWrappers)
		if !ok {
			return true
		}
		if spec.isWithTimeout && len(call.Args) >= 3 {
			for _, candidate := range []ast.Expr{call.Args[2], call.Args[1]} {
				if ident, isIdent := candidate.(*ast.Ident); isIdent {
					if idx, isParam := paramIdx[ident.Name]; isParam {
						matched = wrapperSpec{cmdArgIdx: idx}
						found = true
						return false
					}
				}
			}
			return true
		}
		if spec.cmdArgIdx >= len(call.Args) {
			return true
		}
		arg := call.Args[spec.cmdArgIdx]
		if spec.isInputStruct {
			if cmdExpr, hasCmd := compositeFieldFromExpr(arg, fieldCommand); hasCmd {
				arg = cmdExpr
			}
		}
		ident, isIdent := arg.(*ast.Ident)
		if !isIdent {
			return true
		}
		idx, isParam := paramIdx[ident.Name]
		if !isParam {
			return true
		}
		matched = wrapperSpec{cmdArgIdx: idx}
		found = true
		return false
	})
	return matched, found
}

func funcParamIndices(params *ast.FieldList) map[string]int {
	out := make(map[string]int)
	if params == nil {
		return out
	}
	idx := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			idx++
			continue
		}
		for _, name := range field.Names {
			out[name.Name] = idx
			idx++
		}
	}
	return out
}

func paramNameAtIndex(params *ast.FieldList, target int) (string, bool) {
	if params == nil {
		return "", false
	}
	idx := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			idx++
			continue
		}
		for _, name := range field.Names {
			if idx == target {
				return name.Name, true
			}
			idx++
		}
	}
	return "", false
}

func callTargetName(fun ast.Expr) string {
	switch target := fun.(type) {
	case *ast.Ident:
		return target.Name
	case *ast.SelectorExpr:
		return target.Sel.Name
	default:
		return ""
	}
}

func sdkDispatchMethodValue(expr ast.Expr) (wrapperSpec, bool) {
	sel, isSel := expr.(*ast.SelectorExpr)
	if !isSel {
		return wrapperSpec{}, false
	}
	return sdkDispatchSpec(sel.Sel.Name, 2)
}

func sdkDispatchSpec(method string, argCount int) (wrapperSpec, bool) {
	switch method {
	case "DispatchCommand", "DispatchCommandResult", "DispatchCommandAnswer",
		"DispatchCommandArgs", "DispatchCommandArgsResult":
		if argCount >= 2 {
			return wrapperSpec{cmdArgIdx: 1}, true
		}
	case "DispatchCommandWithTimeout":
		if argCount >= 2 {
			return wrapperSpec{cmdArgIdx: 1, isWithTimeout: true}, true
		}
	case "DispatchDestinationCommand", "DispatchPeerCommand":
		if argCount >= 2 {
			return wrapperSpec{cmdArgIdx: 1, isInputStruct: true}, true
		}
	}
	return wrapperSpec{}, false
}

func matchDispatchCall(call *ast.CallExpr, funcWrappers, fieldWrappers map[string]wrapperSpec) (wrapperSpec, bool) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if spec, ok := sdkDispatchSpec(fun.Sel.Name, len(call.Args)); ok {
			return spec, true
		}
		if spec, ok := funcWrappers[fun.Sel.Name]; ok && len(call.Args) > spec.cmdArgIdx {
			return spec, true
		}
		if spec, ok := fieldWrappers[fun.Sel.Name]; ok && len(call.Args) > spec.cmdArgIdx {
			return spec, true
		}
	case *ast.Ident:
		if spec, ok := funcWrappers[fun.Name]; ok && len(call.Args) > spec.cmdArgIdx {
			return spec, true
		}
	}
	return wrapperSpec{}, false
}

func scanPluginFile(pf pluginFile, fset *token.FileSet, funcWrappers, fieldWrappers map[string]wrapperSpec) []requiredSend {
	var out []requiredSend
	for _, decl := range pf.file.Decls {
		fn, isFn := decl.(*ast.FuncDecl)
		if !isFn {
			continue
		}
		if fn.Body == nil {
			continue
		}
		env := newPluginFuncEnv(pf.pkgConsts, pf.importedConsts)
		scanBlockStmt(fn.Body, env, pf, fset, funcWrappers, fieldWrappers, &out)
	}
	return out
}

func scanBlockStmt(block *ast.BlockStmt, env *pluginFuncEnv, pf pluginFile, fset *token.FileSet,
	funcWrappers, fieldWrappers map[string]wrapperSpec, out *[]requiredSend,
) {
	if block == nil {
		return
	}
	for _, stmt := range block.List {
		scanStmt(stmt, env, pf, fset, funcWrappers, fieldWrappers, out)
	}
}

func scanStmt(stmt ast.Stmt, env *pluginFuncEnv, pf pluginFile, fset *token.FileSet,
	funcWrappers, fieldWrappers map[string]wrapperSpec, out *[]requiredSend,
) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		scanBlockStmt(s, env.clone(), pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.DeclStmt:
		scanDeclStmt(s, env, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.AssignStmt:
		for _, rhs := range s.Rhs {
			scanExpr(rhs, env, pf, fset, funcWrappers, fieldWrappers, out)
		}
		updateEnvFromAssign(s, env)
	case *ast.ExprStmt:
		scanExpr(s.X, env, pf, fset, funcWrappers, fieldWrappers, out)
		updateEnvFromTextBufStmt(s.X, env)
	case *ast.ReturnStmt:
		for _, res := range s.Results {
			scanExpr(res, env, pf, fset, funcWrappers, fieldWrappers, out)
		}
	case *ast.DeferStmt:
		scanExpr(s.Call, env, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.GoStmt:
		scanExpr(s.Call, env, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.SendStmt:
		scanExpr(s.Chan, env, pf, fset, funcWrappers, fieldWrappers, out)
		scanExpr(s.Value, env, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.IfStmt:
		branchEnv := env.clone()
		scanStmt(s.Init, branchEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanExpr(s.Cond, branchEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanBlockStmt(s.Body, branchEnv.clone(), pf, fset, funcWrappers, fieldWrappers, out)
		scanStmt(s.Else, branchEnv.clone(), pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.ForStmt:
		loopEnv := env.clone()
		scanStmt(s.Init, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanExpr(s.Cond, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanStmt(s.Post, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanBlockStmt(s.Body, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.RangeStmt:
		loopEnv := env.clone()
		scanExpr(s.X, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanBlockStmt(s.Body, loopEnv, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.SwitchStmt:
		swEnv := env.clone()
		scanStmt(s.Init, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanExpr(s.Tag, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanBlockStmt(s.Body, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.TypeSwitchStmt:
		swEnv := env.clone()
		scanStmt(s.Init, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanStmt(s.Assign, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
		scanBlockStmt(s.Body, swEnv, pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.SelectStmt:
		scanBlockStmt(s.Body, env.clone(), pf, fset, funcWrappers, fieldWrappers, out)
	case *ast.CaseClause:
		caseEnv := env.clone()
		for _, expr := range s.List {
			scanExpr(expr, caseEnv, pf, fset, funcWrappers, fieldWrappers, out)
		}
		for _, bodyStmt := range s.Body {
			scanStmt(bodyStmt, caseEnv, pf, fset, funcWrappers, fieldWrappers, out)
		}
	case *ast.CommClause:
		commEnv := env.clone()
		scanStmt(s.Comm, commEnv, pf, fset, funcWrappers, fieldWrappers, out)
		for _, bodyStmt := range s.Body {
			scanStmt(bodyStmt, commEnv, pf, fset, funcWrappers, fieldWrappers, out)
		}
	case *ast.LabeledStmt:
		scanStmt(s.Stmt, env, pf, fset, funcWrappers, fieldWrappers, out)
	}
}

func scanDeclStmt(declStmt *ast.DeclStmt, env *pluginFuncEnv, pf pluginFile, fset *token.FileSet,
	funcWrappers, fieldWrappers map[string]wrapperSpec, out *[]requiredSend,
) {
	gen, isGen := declStmt.Decl.(*ast.GenDecl)
	if !isGen {
		return
	}
	for _, spec := range gen.Specs {
		values, isValues := spec.(*ast.ValueSpec)
		if !isValues {
			continue
		}
		for i, name := range values.Names {
			if i >= len(values.Values) {
				if isTextBufType(values.Type) {
					env.bufVars[name.Name] = staticPrefix{text: "", complete: true}
				}
				continue
			}
			valExpr := values.Values[i]
			scanExpr(valExpr, env, pf, fset, funcWrappers, fieldWrappers, out)
			updateEnvVar(name.Name, valExpr, env)
		}
	}
}

func updateEnvFromAssign(assign *ast.AssignStmt, env *pluginFuncEnv) {
	if assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN {
		return
	}
	for i, lhs := range assign.Lhs {
		ident, isIdent := lhs.(*ast.Ident)
		if !isIdent {
			continue
		}
		if i >= len(assign.Rhs) {
			delete(env.strVars, ident.Name)
			delete(env.inputVars, ident.Name)
			continue
		}
		updateEnvVar(ident.Name, assign.Rhs[i], env)
	}
}

func updateEnvVar(name string, rhs ast.Expr, env *pluginFuncEnv) {
	if cmdPrefix, ok := evalDispatchInputCommand(rhs, env); ok {
		env.inputVars[name] = cmdPrefix
		delete(env.strVars, name)
		return
	}
	delete(env.inputVars, name)

	if prefix, ok := evalStaticPrefix(rhs, env, 0); ok {
		env.strVars[name] = prefix
		return
	}
	delete(env.strVars, name)
}

func updateEnvFromTextBufStmt(expr ast.Expr, env *pluginFuncEnv) {
	if baseName, prefix, ok := evalTextBufMutation(expr, env, 0); ok {
		env.bufVars[baseName] = prefix
	}
}

func isTextBufType(expr ast.Expr) bool {
	sel, isSel := expr.(*ast.SelectorExpr)
	if !isSel {
		return false
	}
	return sel.Sel.Name == "Buffer"
}

func scanExpr(root ast.Expr, env *pluginFuncEnv, pf pluginFile, fset *token.FileSet,
	funcWrappers, fieldWrappers map[string]wrapperSpec, out *[]requiredSend,
) {
	if root == nil {
		return
	}
	callFuns := make(map[*ast.SelectorExpr]bool)

	ast.Inspect(root, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			if n.Body != nil {
				scanBlockStmt(n.Body, env.clone(), pf, fset, funcWrappers, fieldWrappers, out)
			}
			return false
		case *ast.CompositeLit:
			if marker, hasMarker := updateRouteInputPeerMarker(n, env); hasMarker {
				*out = append(*out, requiredSend{
					file:    pf.rel,
					line:    fset.Position(n.Pos()).Line,
					call:    "UpdateRoute(PeerMarker:" + marker + ")",
					message: msgBGPRefresh,
				})
			}
			return true
		case *ast.CallExpr:
			if sel, isSel := n.Fun.(*ast.SelectorExpr); isSel {
				callFuns[sel] = true
			}
			scanCallExpr(n, env, pf, fset, funcWrappers, fieldWrappers, out)
			return true
		case *ast.SelectorExpr:
			if callFuns[n] {
				return true
			}
			if msg, isCLI, ok := sdkEngineOpMessage(n.Sel.Name); ok {
				*out = append(*out, requiredSend{
					file:    pf.rel,
					line:    fset.Position(n.Pos()).Line,
					call:    n.Sel.Name,
					message: msg,
					isCLI:   isCLI,
				})
			}
			return true
		}
		return true
	})
}

func scanCallExpr(call *ast.CallExpr, env *pluginFuncEnv, pf pluginFile, fset *token.FileSet,
	funcWrappers, fieldWrappers map[string]wrapperSpec, out *[]requiredSend,
) {
	line := fset.Position(call.Pos()).Line

	if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel {
		if req, matched := helperPkgRequiredSend(sel, pf, line); matched {
			*out = append(*out, req)
			return
		}
		if msg, isCLI, ok := sdkEngineOpMessage(sel.Sel.Name); ok {
			if isPeerMarkerOnlyUpdateRouteCall(sel.Sel.Name, call.Args, env) {
				return
			}
			*out = append(*out, requiredSend{
				file:    pf.rel,
				line:    line,
				call:    sel.Sel.Name,
				message: msg,
				isCLI:   isCLI,
			})
			return
		}
	}

	spec, isDispatch := matchDispatchCall(call, funcWrappers, fieldWrappers)
	if !isDispatch {
		return
	}
	prefix, ok := extractDispatchCallPrefix(call, spec, env)
	if !ok {
		return
	}
	cmdPrefix, valid := extractCommandPrefix(prefix)
	if !valid {
		return
	}
	*out = append(*out, requiredSend{
		file:    pf.rel,
		line:    line,
		call:    callTargetName(call.Fun),
		message: cmdPrefix,
		isCLI:   true,
	})
}

func helperPkgRequiredSend(sel *ast.SelectorExpr, pf pluginFile, line int) (requiredSend, bool) {
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return requiredSend{}, false
	}
	if pf.routeInstallPkg != "" && ident.Name == pf.routeInstallPkg && sel.Sel.Name == "New" {
		return requiredSend{
			file:    pf.rel,
			line:    line,
			call:    "routeinstall.New",
			message: msgSystemRIBRouteInst,
		}, true
	}
	if pf.ribEventsPkg != "" && ident.Name == pf.ribEventsPkg {
		switch sel.Sel.Name {
		case "RelayReplay":
			return requiredSend{
				file:    pf.rel,
				line:    line,
				call:    "ribevents.RelayReplay",
				message: ribevents.ReplayRoutesCommand,
				isCLI:   true,
			}, true
		case "RelayReplayPath":
			return requiredSend{
				file:    pf.rel,
				line:    line,
				call:    "ribevents.RelayReplayPath",
				message: ribevents.ReplayPathCommand,
				isCLI:   true,
			}, true
		}
	}
	return requiredSend{}, false
}

func extractDispatchCallPrefix(call *ast.CallExpr, spec wrapperSpec, env *pluginFuncEnv) (staticPrefix, bool) {
	if spec.isInputStruct {
		if spec.cmdArgIdx >= len(call.Args) {
			return staticPrefix{}, false
		}
		return evalDispatchInputCommand(call.Args[spec.cmdArgIdx], env)
	}
	if spec.isWithTimeout && len(call.Args) >= 3 {
		if prefix, ok := evalStaticPrefix(call.Args[2], env, 0); ok {
			return prefix, true
		}
		return evalStaticPrefix(call.Args[1], env, 0)
	}
	if spec.cmdArgIdx >= len(call.Args) {
		return staticPrefix{}, false
	}
	return evalStaticPrefix(call.Args[spec.cmdArgIdx], env, 0)
}

func isPeerMarkerOnlyUpdateRouteCall(method string, args []ast.Expr, env *pluginFuncEnv) bool {
	if method != "UpdateRouteRequest" && method != "UpdateRouteRequestResult" {
		return false
	}
	if len(args) < 2 {
		return false
	}
	lit, isLit := unwrapUnaryAddr(args[1]).(*ast.CompositeLit)
	if !isLit {
		return false
	}
	if _, hasMarker := updateRouteInputPeerMarker(lit, env); !hasMarker {
		return false
	}
	_, hasCommand := compositeFieldExpr(lit, fieldCommand)
	return !hasCommand
}

func updateRouteInputPeerMarker(lit *ast.CompositeLit, env *pluginFuncEnv) (string, bool) {
	if !isTypeNamed(lit.Type, "UpdateRouteInput") {
		return "", false
	}
	markerExpr, hasField := compositeFieldExpr(lit, "PeerMarker")
	if !hasField {
		return "", false
	}
	val, ok := evalStaticPrefix(markerExpr, env, 0)
	if !ok {
		return "", false
	}
	if !val.complete {
		return "", false
	}
	switch val.text {
	case peerMarkerBoRR, peerMarkerEoRR:
		return val.text, true
	default:
		return "", false
	}
}

func isTypeNamed(expr ast.Expr, name string) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return typed.Sel.Name == name
	case *ast.Ident:
		return typed.Name == name
	default:
		return false
	}
}

func evalDispatchInputCommand(expr ast.Expr, env *pluginFuncEnv) (staticPrefix, bool) {
	if ident, isIdent := expr.(*ast.Ident); isIdent {
		prefix, ok := env.inputVars[ident.Name]
		return prefix, ok
	}
	lit, isLit := unwrapUnaryAddr(expr).(*ast.CompositeLit)
	if !isLit {
		return staticPrefix{}, false
	}
	if !isTypeNamed(lit.Type, "DispatchCommandInput") {
		return staticPrefix{}, false
	}
	cmdExpr, hasCmd := compositeFieldExpr(lit, fieldCommand)
	if !hasCmd {
		return staticPrefix{}, false
	}
	return evalStaticPrefix(cmdExpr, env, 0)
}

func sdkEngineOpMessage(method string) (message string, isCLI, ok bool) {
	switch method {
	case "UpdateRoute", "UpdateRouteResult",
		"UpdateRouteSel", "UpdateRouteSelResult",
		"UpdateRouteSelWithMeta", "UpdateRouteSelWithMetaResult",
		"UpdateRouteWithMeta", "UpdateRouteWithMetaResult",
		"UpdateRouteRequest", "UpdateRouteRequestResult",
		"ForwardCached", "RelayStoredRoute",
		"SelectRoutes", "SelectRefresh":
		return msgBGPUpdate, false, true
	case "InjectWireRoute":
		return msgBGPRaw, false, true
	case "RouteInstall", "RouteRemove":
		return msgSystemRIBRouteInst, false, true
	case "RouteMetrics", "RouteMetricsProspective":
		return msgSystemRIBRouteMetric, false, true
	case "ResolveDNS":
		return msgResolveDNS, false, true
	case "BatchValidate":
		return rpc.BatchValidateCommand, true, true
	default:
		return "", false, false
	}
}

// evalStaticPrefix resolves an AST expression to its leading static string
// content, handling literals, local variables, package/imported constants,
// string concatenation, fmt.Sprintf prefixes, and textbuf.Buffer chains.
func evalStaticPrefix(expr ast.Expr, env *pluginFuncEnv, depth int) (staticPrefix, bool) {
	if expr == nil {
		return staticPrefix{}, false
	}
	if depth > maxExprDepth {
		return staticPrefix{}, false
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		val, ok := stringLit(e)
		if !ok {
			return staticPrefix{}, false
		}
		return staticPrefix{text: val, complete: true}, true
	case *ast.ParenExpr:
		return evalStaticPrefix(e.X, env, depth+1)
	case *ast.Ident:
		if local, ok := env.strVars[e.Name]; ok {
			return local, true
		}
		if val, ok := env.pkgConsts[e.Name]; ok {
			return staticPrefix{text: val, complete: true}, true
		}
		return staticPrefix{}, false
	case *ast.SelectorExpr:
		if val, ok := selectorConst(e, env.importedConsts); ok {
			return staticPrefix{text: val, complete: true}, true
		}
		return staticPrefix{}, false
	case *ast.BinaryExpr:
		return evalBinaryStaticPrefix(e, env, depth+1)
	case *ast.CallExpr:
		return evalCallStaticPrefix(e, env, depth+1)
	default:
		return staticPrefix{}, false
	}
}

func evalBinaryStaticPrefix(bin *ast.BinaryExpr, env *pluginFuncEnv, depth int) (staticPrefix, bool) {
	if bin.Op != token.ADD {
		return staticPrefix{}, false
	}
	lhs, ok := evalStaticPrefix(bin.X, env, depth)
	if !ok {
		return staticPrefix{}, false
	}
	if !lhs.complete {
		return lhs, true
	}
	rhs, rhsOK := evalStaticPrefix(bin.Y, env, depth)
	if rhsOK {
		return staticPrefix{text: lhs.text + rhs.text, complete: rhs.complete}, true
	}
	// rhs is dynamic; preserve the leading static prefix and mark it incomplete.
	return staticPrefix{text: lhs.text, complete: false}, true
}

func evalCallStaticPrefix(call *ast.CallExpr, env *pluginFuncEnv, depth int) (staticPrefix, bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return staticPrefix{}, false
	}
	if sel.Sel.Name == "Sprintf" && len(call.Args) >= 1 {
		fmtVal, ok := evalStaticPrefix(call.Args[0], env, depth+1)
		if !ok {
			return staticPrefix{}, false
		}
		if pct := strings.IndexByte(fmtVal.text, '%'); pct >= 0 {
			return staticPrefix{text: fmtVal.text[:pct], complete: false}, true
		}
		return fmtVal, true
	}
	if sel.Sel.Name == "String" && len(call.Args) == 0 {
		return evalTextBufChain(sel.X, env, depth+1)
	}
	return staticPrefix{}, false
}

func evalTextBufMutation(expr ast.Expr, env *pluginFuncEnv, depth int) (string, staticPrefix, bool) {
	baseName, steps, ok := unwindTextBufChain(expr)
	if !ok {
		return "", staticPrefix{}, false
	}
	if baseName == "" {
		return "", staticPrefix{}, false
	}
	if len(steps) == 0 {
		return "", staticPrefix{}, false
	}
	acc, exists := env.bufVars[baseName]
	if !exists {
		acc = staticPrefix{text: "", complete: true}
	}
	return baseName, applyTextBufSteps(acc, steps, env, depth), true
}

func evalTextBufChain(expr ast.Expr, env *pluginFuncEnv, depth int) (staticPrefix, bool) {
	baseName, steps, ok := unwindTextBufChain(expr)
	if !ok {
		return staticPrefix{}, false
	}
	acc := staticPrefix{text: "", complete: true}
	if baseName != "" {
		if existing, exists := env.bufVars[baseName]; exists {
			acc = existing
		}
	}
	acc = applyTextBufSteps(acc, steps, env, depth)
	if acc.text == "" {
		return staticPrefix{}, false
	}
	return acc, true
}

func unwindTextBufChain(expr ast.Expr) (string, []textBufStep, bool) {
	var rev []textBufStep
	cur := expr
	for range maxExprDepth {
		if ident, isIdent := cur.(*ast.Ident); isIdent {
			slices.Reverse(rev)
			return ident.Name, rev, true
		}
		call, isCall := cur.(*ast.CallExpr)
		if !isCall {
			return "", nil, false
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			return "", nil, false
		}
		if !isTextBufMethod(sel.Sel.Name) {
			return "", nil, false
		}
		rev = append(rev, textBufStep{method: sel.Sel.Name, args: call.Args})
		cur = sel.X
	}
	return "", nil, false
}

func isTextBufMethod(name string) bool {
	switch name {
	case "Reset", "Str", "Byte", "Quoted", "Int", "Uint", "Uint8", "Uint16", "Uint32", "Int64", "Uint64", "Err", "Bytes", "Bool":
		return true
	default:
		return false
	}
}

func applyTextBufSteps(acc staticPrefix, steps []textBufStep, env *pluginFuncEnv, depth int) staticPrefix {
	for _, step := range steps {
		switch step.method {
		case "Reset":
			acc = staticPrefix{text: "", complete: true}
		case "Str":
			if !acc.complete {
				continue
			}
			if len(step.args) != 1 {
				acc.complete = false
				continue
			}
			val, ok := evalStaticPrefix(step.args[0], env, depth+1)
			if !ok {
				acc.complete = false
				continue
			}
			acc.text += val.text
			acc.complete = val.complete
		case "Byte":
			if !acc.complete {
				continue
			}
			if len(step.args) != 1 {
				acc.complete = false
				continue
			}
			b, ok := charLit(step.args[0])
			if !ok {
				acc.complete = false
				continue
			}
			acc.text += string(b)
		default:
			acc.complete = false
		}
	}
	return acc
}

func charLit(expr ast.Expr) (rune, bool) {
	lit, isLit := expr.(*ast.BasicLit)
	if !isLit {
		return 0, false
	}
	if lit.Kind != token.CHAR {
		return 0, false
	}
	val, _, _, err := strconv.UnquoteChar(lit.Value[1:len(lit.Value)-1], '\'')
	if err != nil {
		return 0, false
	}
	return val, true
}

// extractCommandPrefix extracts the longest leading sequence of lowercase
// command-word tokens before any format verb, dynamic value, or non-command
// argument.
func extractCommandPrefix(prefix staticPrefix) (string, bool) {
	trimmed := strings.TrimLeft(prefix.text, " \t")
	if trimmed == "" {
		return "", false
	}
	lastByte := trimmed[len(trimmed)-1]
	hasTrailingSpace := lastByte == ' ' || lastByte == '\t'
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", false
	}
	if !prefix.complete && !hasTrailingSpace {
		fields = fields[:len(fields)-1]
	}

	words := make([]string, 0, len(fields))
	for _, tok := range fields {
		if !isCommandWord(tok) {
			break
		}
		words = append(words, tok)
	}
	if len(words) == 0 {
		return "", false
	}
	return textbuf.Join(words, " "), true
}

func isCommandWord(tok string) bool {
	if tok == "" {
		return false
	}
	if tok[0] < 'a' {
		return false
	}
	if tok[0] > 'z' {
		return false
	}
	for i := 1; i < len(tok); i++ {
		if !isCommandWordByte(tok[i]) {
			return false
		}
	}
	return true
}

func isCommandWordByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-':
		return true
	default:
		return false
	}
}

func sendDeclCovers(sends []rpc.MessageSendDecl, req requiredSend) bool {
	if !req.isCLI {
		for i := range sends {
			if sends[i].Message == req.message {
				return true
			}
		}
		return false
	}
	for i := range sends {
		declPrefix, isCLI := strings.CutPrefix(sends[i].Message, cliMessagePrefix)
		if !isCLI {
			continue
		}
		if cliSendCovers(declPrefix, req.message) {
			return true
		}
	}
	return false
}

func cliSendCovers(declPrefix, cmd string) bool {
	if declPrefix == "" {
		return false
	}
	if declPrefix == cmd {
		return true
	}
	return strings.HasPrefix(cmd, declPrefix+" ")
}

func formatUndeclaredDetail(pluginName, call, requiredMsg string) string {
	var tb textbuf.Buffer
	return tb.Str("plugin ").Quoted(pluginName).
		Str(" calls ").Str(call).
		Str(" which requires Sends entry ").Quoted(requiredMsg).
		Str(", not declared in Registration.Sends").String()
}

func sortPluginFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Emitter != findings[j].Emitter {
			return findings[i].Emitter < findings[j].Emitter
		}
		return findings[i].Command < findings[j].Command
	})
}
