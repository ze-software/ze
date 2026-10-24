// Design: docs/contributing/ze-go-style.md -- a zero value is never an answer, checked on changed code
// Related: annotation.go -- the refuse-by-default annotation and the guard it names
// Related: report.go -- what the check answers
//
// refusebydefault.go finds the shapes that let a function answer a plausible
// value where it has no answer. The caller cannot tell that value from a real
// one, so the defect surfaces far from its cause and reads as data
// (ai/rules/principles.md, first directive). Four shapes produced the family
// plan/release/fixes/spec-refuse-instead-of-defaulting.md owns:
//
//   - a switch whose unmatched value answers a nil error: a `default:` arm that
//     returns one, or the return after a switch whose every case returns
//   - a branch on a miss that answers as if it hit: `!ok` from a comma-ok
//     binding, `== nil` on a registered slot, `err != nil` from a read, and the
//     collapse `return err == nil`, each answering `true`, a zero, or a nil error
//   - an ADD-PATH path identifier compared with zero, when zero is a valid
//     identifier and presence follows the negotiated capability
//   - a state or health answer written from a literal: `StatusHealthy` with no
//     measured read, a check that is one literal return, a literal boolean in
//     a State, Status or Health value
//
// The detection is syntactic, like `arch compound-guard`: it reads one file's
// AST and resolves no types, so it names a shape for a reader to judge, and the
// annotation (annotation.go) is how the reader records a zero that is a guard.
// The precision cuts (recordsMiss, mentions, parses, the callback flag) were
// each measured against the last 200 commits; the spec's A-2 row has the count.

package archrefusebydefault

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// Rule names one shape this check refuses. The value is what a finding prints.
type Rule string

// The rules, one per shape the spec's acceptance criteria name.
const (
	// RuleSwitchNilError is AC-1: an unmatched switch value answers nil error.
	RuleSwitchNilError Rule = "switch-unmatched-nil-error"
	// RuleMissAnswered is AC-2: a lookup miss or read error answers as a hit.
	RuleMissAnswered Rule = "miss-answered-as-value"
	// RulePathIDZero is AC-3: an ADD-PATH identifier compared with zero.
	RulePathIDZero Rule = "path-id-compared-with-zero"
	// RuleLiteralState is AC-4: a state or health answer from a literal.
	RuleLiteralState Rule = "state-from-literal"
	// RuleAnnotationNoGuard is AC-6: an annotation that names no guard.
	RuleAnnotationNoGuard Rule = "annotation-names-no-guard"
)

// Hit is one shape found in a source file, before the change set or an
// annotation judges it.
type Hit struct {
	Rule Rule
	// Line is where the shape starts, and where its annotation is looked for:
	// on this line or the line above.
	Line int
	// Anchors are the line spans that make the shape: a change set that
	// touches none of them did not add it.
	Anchors []span
	// Fn is the enclosing function, `Type.Method` for a method.
	Fn string
	// Detail says what the shape is in this instance.
	Detail string
}

// span is an inclusive line range.
type span struct{ first, last int }

// packageScope names the enclosing function of a shape outside any function.
const packageScope = "(package scope)"

// ScanFile answers every hit and every annotation in one Go source file.
//
// A file that does not parse is an error rather than a file with no hit: a
// gate that skips what it cannot read reports a clean tree for it. A generated
// file answers nothing, because its author is a generator.
func ScanFile(fset *token.FileSet, path string) ([]Hit, []Annotation, error) {
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	if ast.IsGenerated(file) {
		return nil, nil, nil
	}
	scan := &fileScan{fset: fset, packageVars: packageVars(file)}
	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if !isFunc {
			scan.walkOutside(decl, packageScope)
			continue
		}
		if funcDecl.Body == nil {
			continue
		}
		name := funcName(funcDecl)
		scan.checkConstantCheck(funcDecl, name)
		scan.function(name, funcDecl.Type, funcDecl.Body, false)
	}
	return scan.hits, annotations(fset, file), nil
}

// fileScan carries one file's state through the walk.
type fileScan struct {
	fset        *token.FileSet
	packageVars map[string]bool
	hits        []Hit
}

// line answers the line of a position.
func (s *fileScan) line(pos token.Pos) int { return s.fset.Position(pos).Line }

// add records a hit whose anchors are the given nodes' line spans.
func (s *fileScan) add(rule Rule, fn, detail string, nodes ...ast.Node) {
	anchors := make([]span, 0, len(nodes))
	for _, node := range nodes {
		anchors = append(anchors, span{first: s.line(node.Pos()), last: s.line(node.End())})
	}
	s.hits = append(s.hits, Hit{Rule: rule, Line: anchors[0].first, Anchors: anchors, Fn: fn, Detail: detail})
}

// addHeader records a hit anchored on a header, from first to the line of
// last, and on each further node's span.
func (s *fileScan) addHeader(rule Rule, fn, detail string, first, last token.Pos, nodes ...ast.Node) {
	anchors := make([]span, 0, 1+len(nodes))
	anchors = append(anchors, span{first: s.line(first), last: s.line(last)})
	for _, node := range nodes {
		anchors = append(anchors, span{first: s.line(node.Pos()), last: s.line(node.End())})
	}
	s.hits = append(s.hits, Hit{Rule: rule, Line: anchors[0].first, Anchors: anchors, Fn: fn, Detail: detail})
}

// walkOutside judges the expression shapes of a package-level declaration. No
// function encloses it, so only the rules that need no result list apply.
func (s *fileScan) walkOutside(node ast.Node, fn string) {
	ast.Inspect(node, func(child ast.Node) bool {
		if lit, isLit := child.(*ast.FuncLit); isLit {
			s.literal(fn, lit)
			return false
		}
		s.expression(child, fn)
		return true
	})
}

// literal judges a function literal as a function of its own.
func (s *fileScan) literal(fn string, lit *ast.FuncLit) {
	s.function(fn, lit.Type, lit.Body, true)
}

// function judges one function body against its own result list. A function
// literal inside it is judged as a function of its own, against its own
// results, so a return in a closure is never read as the outer function's.
func (s *fileScan) function(fn string, funcType *ast.FuncType, body *ast.BlockStmt, literal bool) {
	results := resultsOf(funcType)
	results.callback = literal && answersOneBool(funcType)
	bindings := bindingsOf(body)
	healthy := []*ast.ReturnStmt{}
	measured := false
	// presence holds the calls compared with nil, which answer whether a
	// thing is registered rather than measure its state. A parent is visited
	// before its children, so a call is in the set before it is judged.
	presence := map[*ast.CallExpr]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			s.literal(fn+".func", typed)
			return false
		case *ast.BlockStmt:
			s.fallOff(typed.List, results, fn)
		case *ast.CaseClause:
			s.fallOff(typed.Body, results, fn)
		case *ast.CommClause:
			s.fallOff(typed.Body, results, fn)
		case *ast.SwitchStmt:
			s.defaultArm(typed.Body, results, fn)
		case *ast.TypeSwitchStmt:
			s.defaultArm(typed.Body, results, fn)
		case *ast.IfStmt:
			s.missBranch(typed, results, bindings, fn)
		case *ast.ReturnStmt:
			s.collapsedError(typed, results, bindings, fn)
			if returnsHealthy(typed) {
				healthy = append(healthy, typed)
			}
		case *ast.BinaryExpr:
			if call, isCall := nilComparand(typed).(*ast.CallExpr); isCall {
				presence[call] = true
			}
		case *ast.CallExpr:
			if presence[typed] {
				break
			}
			if measures(typed) {
				measured = true
			}
		}
		s.expression(node, fn)
		return true
	})
	if measured {
		return
	}
	for _, ret := range healthy {
		s.add(RuleLiteralState, fn, "StatusHealthy is answered and nothing in the function measures the state it reports", ret)
	}
}

// expression judges the shapes that need no result list: the path identifier
// compared with zero, and a literal boolean in a state value.
func (s *fileScan) expression(node ast.Node, fn string) {
	switch typed := node.(type) {
	case *ast.BinaryExpr:
		if comparesPathIDWithZero(typed) {
			s.add(RulePathIDZero, fn, pathIDDetail, typed)
		}
	case *ast.CompositeLit:
		if !isStateType(typed.Type) {
			return
		}
		for _, element := range typed.Elts {
			pair, isPair := element.(*ast.KeyValueExpr)
			if !isPair {
				continue
			}
			if isBoolLiteral(pair.Value) {
				s.add(RuleLiteralState, fn, "a literal boolean is written into a state value", pair)
			}
		}
	}
}

// results describes a function's result list.
type results struct {
	// count is the number of result values, named or not.
	count int
	// errorLast is true when the last result's type is `error`.
	errorLast bool
	// callback is true for a function literal answering one bool: an
	// ast.Inspect visitor, a range-over-func yield, a poll predicate. Its
	// bool steers the caller's loop ("keep going", "not yet"), so it answers
	// nothing about the miss it branched on.
	callback bool
}

// resultsOf answers a function type's result list.
func resultsOf(funcType *ast.FuncType) results {
	if funcType.Results == nil {
		return results{}
	}
	answer := results{}
	for _, field := range funcType.Results.List {
		names := len(field.Names)
		if names == 0 {
			names = 1
		}
		answer.count += names
	}
	last := funcType.Results.List[len(funcType.Results.List)-1]
	ident, isIdent := last.Type.(*ast.Ident)
	answer.errorLast = isIdent && ident.Name == "error"
	return answer
}

// answersOneBool reports whether a function type's only result is a bool.
func answersOneBool(funcType *ast.FuncType) bool {
	if funcType.Results.NumFields() != 1 {
		return false
	}
	return isIdentNamed(funcType.Results.List[0].Type, "bool")
}

// answersNilError reports whether a return answers every result and a nil
// error in the last position. A bare return in a function with named results
// answers whatever they hold, which this check cannot read, so it is not one.
func answersNilError(ret *ast.ReturnStmt, fnResults results) bool {
	if !fnResults.errorLast {
		return false
	}
	if len(ret.Results) != fnResults.count {
		return false
	}
	return isNil(ret.Results[len(ret.Results)-1])
}

// answersPlausibly reports whether a return answers a value a caller reads as
// a hit: a nil error in a function with an error result, or, in a function
// with none, a literal `true` anywhere or a lone zero literal.
//
// A non-error function answering several values whose last is `false` is the
// comma-ok form, which hands the miss to its caller, so it answers nothing
// plausible. A lone `false` is the predicate's own "no", which is the honest
// answer to a miss and a refusal to a failed verification; only after a READ
// ERROR is it plausible, because "could not look" then reads as "absent".
func answersPlausibly(ret *ast.ReturnStmt, fnResults results, readError bool) bool {
	if fnResults.errorLast {
		if !answersNilError(ret, fnResults) {
			return false
		}
		// `return T{}, false, nil`: the flag hands the miss on, and the nil
		// error says only that nothing failed.
		for _, result := range ret.Results {
			if isIdentNamed(result, "false") {
				return false
			}
		}
		return true
	}
	if len(ret.Results) == 0 {
		return false
	}
	if fnResults.callback {
		return false
	}
	for _, result := range ret.Results {
		if isIdentNamed(result, "true") {
			return true
		}
	}
	if len(ret.Results) != 1 {
		return false
	}
	if isIdentNamed(ret.Results[0], "false") {
		return readError
	}
	return isZeroLiteral(ret.Results[0])
}

// defaultArm judges a switch body's `default:` clause: one that returns a nil
// error answers an unrecognized value as recognized.
func (s *fileScan) defaultArm(body *ast.BlockStmt, fnResults results, fn string) {
	for _, stmt := range body.List {
		clause, isClause := stmt.(*ast.CaseClause)
		if !isClause {
			continue
		}
		if clause.List != nil {
			continue
		}
		ret, isReturn := lastStatement(clause.Body).(*ast.ReturnStmt)
		if !isReturn {
			continue
		}
		if !answersNilError(ret, fnResults) {
			continue
		}
		if recordsMiss(clause.Body, "") {
			continue
		}
		s.addHeader(RuleSwitchNilError, fn, "the default arm answers a nil error for a value no case recognized", clause.Case, clause.Colon, ret)
	}
}

// fallOff judges each switch in a statement list whose every case returns:
// the statement after it runs only for a value no case matched, so a return
// there that answers a nil error answers an unrecognized value as recognized.
func (s *fileScan) fallOff(list []ast.Stmt, fnResults results, fn string) {
	for index := 0; index+1 < len(list); index++ {
		body, header := switchBody(list[index])
		if body == nil {
			continue
		}
		ret, isReturn := list[index+1].(*ast.ReturnStmt)
		if !isReturn {
			continue
		}
		if !onlyUnmatchedFallsOff(body) {
			continue
		}
		if !answersNilError(ret, fnResults) {
			continue
		}
		s.addHeader(RuleSwitchNilError, fn, "the return after the switch answers a nil error for a value no case recognized", header, body.Lbrace, ret)
	}
}

// switchBody answers a statement's switch body and the position of its
// keyword, or nil when the statement is no switch.
func switchBody(stmt ast.Stmt) (*ast.BlockStmt, token.Pos) {
	switch typed := stmt.(type) {
	case *ast.SwitchStmt:
		return typed.Body, typed.Switch
	case *ast.TypeSwitchStmt:
		return typed.Body, typed.Switch
	default:
		return nil, token.NoPos
	}
}

// onlyUnmatchedFallsOff reports whether every non-default case of a switch
// body leaves by return or panic, there is at least one, and the default arm,
// if any, does not: control then reaches the next statement only for a value
// no case recognized.
func onlyUnmatchedFallsOff(body *ast.BlockStmt) bool {
	cases := 0
	for _, stmt := range body.List {
		clause, isClause := stmt.(*ast.CaseClause)
		if !isClause {
			return false
		}
		leaves := returnsOrPanics(lastStatement(clause.Body))
		if clause.List == nil {
			if leaves {
				return false
			}
			continue
		}
		if !leaves {
			return false
		}
		cases++
	}
	return cases > 0
}

// returnsOrPanics reports whether a statement is a return or a panic call.
func returnsOrPanics(stmt ast.Stmt) bool {
	switch typed := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		call, isCall := typed.X.(*ast.CallExpr)
		if !isCall {
			return false
		}
		return isIdentNamed(call.Fun, "panic")
	default:
		return false
	}
}

// lastStatement answers the last statement of a list, or nil for none.
func lastStatement(list []ast.Stmt) ast.Stmt {
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1]
}

// missBranch judges an if whose condition is a miss and whose body ends by
// answering as if it hit.
func (s *fileScan) missBranch(ifStmt *ast.IfStmt, fnResults results, local bindings, fn string) {
	ret, isReturn := lastStatement(ifStmt.Body.List).(*ast.ReturnStmt)
	if !isReturn {
		return
	}
	miss, readError, name := s.missOf(ifStmt.Cond, local)
	if miss == "" {
		return
	}
	if !answersPlausibly(ret, fnResults, readError) {
		return
	}
	if mentions(ret, name) {
		// The answer carries what was met (`"could not read: " + err.Error()`).
		return
	}
	if recordsMiss(ifStmt.Body.List, name) {
		return
	}
	s.addHeader(RuleMissAnswered, fn, miss+" is answered as a value", ifStmt.If, ifStmt.Body.Lbrace, ret)
}

// recordsMiss reports whether a branch records what it met before it
// answers: a write into a field or an element (`state.failure = err`,
// `d.Refusals = append(...)`), a counter step (`s.drops++`), or an
// assignment that hands the miss's own identifier to an outer variable
// (`refusal = err`). The miss then reaches a reader through that record,
// which is a refusal or a counted discard rather than a silent value. A log
// call is no such record: a WARN beside a default is still the default.
func recordsMiss(body []ast.Stmt, name string) bool {
	for _, stmt := range body {
		switch typed := stmt.(type) {
		case *ast.AssignStmt:
			if typed.Tok == token.DEFINE {
				continue
			}
			for _, rhs := range typed.Rhs {
				if mentions(rhs, name) {
					return true
				}
			}
			if slices.ContainsFunc(typed.Lhs, recordTarget) {
				return true
			}
		case *ast.IncDecStmt:
			if recordTarget(typed.X) {
				return true
			}
		}
	}
	return false
}

// mentions reports whether a node refers to the identifier name. An empty
// name is mentioned nowhere.
func mentions(node ast.Node, name string) bool {
	if name == "" {
		return false
	}
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		if ident, isIdent := child.(*ast.Ident); isIdent {
			if ident.Name == name {
				found = true
			}
		}
		return !found
	})
	return found
}

// recordTarget reports whether an assignment target outlives the branch: a
// field or an element, not a local.
func recordTarget(expr ast.Expr) bool {
	switch ast.Unparen(expr).(type) {
	case *ast.SelectorExpr, *ast.IndexExpr:
		return true
	default:
		return false
	}
}

// missOf answers what miss a condition tests, or "" when it tests none: a
// negated comma-ok flag, a nil registered slot, or a non-nil read error, and
// whether it is the read error, and the identifier it tests.
func (s *fileScan) missOf(cond ast.Expr, local bindings) (miss string, readError bool, name string) {
	cond = ast.Unparen(cond)
	if unary, isUnary := cond.(*ast.UnaryExpr); isUnary {
		if unary.Op != token.NOT {
			return "", false, ""
		}
		ident, isIdent := ast.Unparen(unary.X).(*ast.Ident)
		if !isIdent {
			return "", false, ""
		}
		if !local.commaOK[ident.Name] {
			return "", false, ""
		}
		return "a miss (!" + ident.Name + ")", false, ident.Name
	}
	binary, isBinary := cond.(*ast.BinaryExpr)
	if !isBinary {
		return "", false, ""
	}
	ident, isIdent := nilComparand(binary).(*ast.Ident)
	if !isIdent {
		return "", false, ""
	}
	if isErrorName(ident.Name) {
		if binary.Op != token.NEQ {
			return "", false, ""
		}
		if !local.readError(ident.Name) {
			return "", false, ""
		}
		return "a read error (" + ident.Name + " != nil)", true, ident.Name
	}
	if binary.Op != token.EQL {
		return "", false, ""
	}
	if local.slotCopy[ident.Name] {
		return "an unregistered slot (" + ident.Name + " == nil)", false, ident.Name
	}
	if s.packageVars[ident.Name] {
		return "an unregistered slot (" + ident.Name + " == nil)", false, ident.Name
	}
	return "", false, ""
}

// collapsedError judges `return err == nil` and `return err != nil` in a
// function with no error result: any failure to read becomes a plain answer,
// so a caller cannot tell "absent" from "could not look".
func (s *fileScan) collapsedError(ret *ast.ReturnStmt, fnResults results, local bindings, fn string) {
	if fnResults.errorLast {
		return
	}
	for _, result := range ret.Results {
		binary, isBinary := ast.Unparen(result).(*ast.BinaryExpr)
		if !isBinary {
			continue
		}
		ident, isIdent := nilComparand(binary).(*ast.Ident)
		if !isIdent {
			continue
		}
		if !isErrorName(ident.Name) {
			continue
		}
		if !local.readError(ident.Name) {
			continue
		}
		s.add(RuleMissAnswered, fn, "an error is collapsed into a boolean ("+ident.Name+" compared with nil)", ret)
		return
	}
}

// nilComparand answers the operand a == or != comparison holds against nil,
// or nil when the comparison is not with nil.
func nilComparand(binary *ast.BinaryExpr) ast.Expr {
	if binary.Op != token.EQL {
		if binary.Op != token.NEQ {
			return nil
		}
	}
	if isNil(binary.Y) {
		return ast.Unparen(binary.X)
	}
	if isNil(binary.X) {
		return ast.Unparen(binary.Y)
	}
	return nil
}

// isErrorName reports whether an identifier names an error by convention:
// `err`, or a name ending in `Err` or `err`.
func isErrorName(name string) bool {
	if strings.HasSuffix(name, "Err") {
		return true
	}
	return strings.HasSuffix(name, "err")
}

// bindings records how a function's locals were bound, which is how a
// condition is recognized as a miss without resolving types.
type bindings struct {
	// commaOK holds every name bound in a non-first position of a
	// multi-value assignment from one expression: `v, ok := m[k]`,
	// `v, ok := x.(T)`, `off, present, ok := read(b)`.
	commaOK map[string]bool
	// slotCopy holds every name bound from a plain read of another variable,
	// a field or an index: `fn := resolver`, `h := handlers[name]`.
	slotCopy map[string]bool
	// readErr and parseErr hold the error names bound from a call: a read
	// (anything) or a parse (`strconv.Atoi`, `netip.ParseAddr`).
	readErr  map[string]bool
	parseErr map[string]bool
}

// readError reports whether an error name can hold a read failure: it was
// bound from a call that is not a parse, or from nothing this function shows
// (a parameter, a closure's capture). A parse failure is the parse's own "no",
// and answering zero or false for it is the answer rather than a guess.
func (b bindings) readError(name string) bool {
	if b.readErr[name] {
		return true
	}
	return !b.parseErr[name]
}

// parses reports whether a callee's name says it parses text rather than
// reads a resource.
func parses(callee string) bool {
	if strings.HasPrefix(callee, "Parse") {
		return true
	}
	if strings.HasPrefix(callee, "parse") {
		return true
	}
	switch callee {
	case "Atoi", "Unquote", "Compile":
		return true
	default:
		return false
	}
}

// bindingsOf answers the bindings of one function body, closures included.
func bindingsOf(body *ast.BlockStmt) bindings {
	answer := bindings{commaOK: map[string]bool{}, slotCopy: map[string]bool{}, readErr: map[string]bool{}, parseErr: map[string]bool{}}
	ast.Inspect(body, func(node ast.Node) bool {
		assign, isAssign := node.(*ast.AssignStmt)
		if !isAssign {
			return true
		}
		if len(assign.Rhs) != 1 {
			return true
		}
		answer.errorSources(assign)
		if len(assign.Lhs) > 1 {
			for _, lhs := range assign.Lhs[1:] {
				if ident, isIdent := lhs.(*ast.Ident); isIdent {
					answer.commaOK[ident.Name] = true
				}
			}
			return true
		}
		ident, isIdent := assign.Lhs[0].(*ast.Ident)
		if !isIdent {
			return true
		}
		switch ast.Unparen(assign.Rhs[0]).(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr:
			answer.slotCopy[ident.Name] = true
		}
		return true
	})
	return answer
}

// errorSources records, for each error name an assignment binds from a call,
// whether that call parses or reads.
func (b bindings) errorSources(assign *ast.AssignStmt) {
	call, isCall := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !isCall {
		return
	}
	for _, lhs := range assign.Lhs {
		ident, isIdent := lhs.(*ast.Ident)
		if !isIdent {
			continue
		}
		if !isErrorName(ident.Name) {
			continue
		}
		if parses(finalName(call.Fun)) {
			b.parseErr[ident.Name] = true
			continue
		}
		b.readErr[ident.Name] = true
	}
}

// packageVars answers the names a file declares with `var` at package level.
func packageVars(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen {
			continue
		}
		if gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue {
				continue
			}
			for _, name := range value.Names {
				names[name.Name] = true
			}
		}
	}
	return names
}

// pathIDDetail names the negotiated-presence accessors a path identifier
// test uses instead of its value.
const pathIDDetail = "ADD-PATH path identifier compared with 0, but 0 is a valid identifier; " +
	"read presence from the negotiated capability: nlri.HasPathID(n) for an NLRI, " +
	"Route.AddPath for a bgp.Route, RawRoute.HasPathID for an adj-rib-in route"

// comparesPathIDWithZero reports whether a comparison holds a path
// identifier against the literal 0. RFC 7911 Section 2: "A BGP speaker that
// receives a route should not assume that the identifier carries any
// particular semantics." Zero is one more identifier, so presence follows the
// negotiated capability and never the value.
func comparesPathIDWithZero(binary *ast.BinaryExpr) bool {
	//exhaustive:ignore // a predicate over the six comparison operators; every other operator answers false
	switch binary.Op {
	case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ:
	default:
		return false
	}
	if isIntZero(binary.Y) {
		return namesPathID(binary.X)
	}
	if isIntZero(binary.X) {
		return namesPathID(binary.Y)
	}
	return false
}

// namesPathID reports whether an expression's final name is a path
// identifier: `pathID`, `PathID`, `route.PathID`, `nlriPathId`.
func namesPathID(expr ast.Expr) bool {
	var name string
	switch typed := ast.Unparen(expr).(type) {
	case *ast.Ident:
		name = typed.Name
	case *ast.SelectorExpr:
		name = typed.Sel.Name
	case *ast.CallExpr:
		return namesPathID(typed.Fun)
	default:
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "pathid") {
		return true
	}
	return strings.HasSuffix(lower, "pathidentifier")
}

// returnsHealthy reports whether a return answers `StatusHealthy`.
func returnsHealthy(ret *ast.ReturnStmt) bool {
	for _, result := range ret.Results {
		if finalName(result) == "StatusHealthy" {
			return true
		}
	}
	return false
}

// measures reports whether a call reads something a health answer can rest
// on. A builtin or a formatter that shapes values reads nothing. A call only
// compared with nil is not one either, and function leaves it out before
// asking: it answers whether a thing is registered, not what state it is in.
func measures(call *ast.CallExpr) bool {
	switch finalName(call.Fun) {
	case "len", "cap", "make", "new", "append", "panic", "Sprintf", "Errorf", "New":
		return false
	default:
		return true
	}
}

// checkConstantCheck judges a function that is one literal return: no
// receiver, no parameters, one `error` or `bool` result, and a body that
// answers `nil` or `true`. It is a check that cannot fail, so the readiness it
// reports is a constant.
func (s *fileScan) checkConstantCheck(decl *ast.FuncDecl, fn string) {
	if decl.Recv != nil {
		return
	}
	if decl.Type.Params.NumFields() != 0 {
		return
	}
	if decl.Type.Results.NumFields() != 1 {
		return
	}
	resultType, isIdent := decl.Type.Results.List[0].Type.(*ast.Ident)
	if !isIdent {
		return
	}
	if resultType.Name != "error" {
		if resultType.Name != "bool" {
			return
		}
	}
	if len(decl.Body.List) != 1 {
		return
	}
	ret, isReturn := decl.Body.List[0].(*ast.ReturnStmt)
	if !isReturn {
		return
	}
	if len(ret.Results) != 1 {
		return
	}
	if !isNil(ret.Results[0]) {
		if !isIdentNamed(ret.Results[0], "true") {
			return
		}
	}
	s.addHeader(RuleLiteralState, fn, "a check answers a constant, so it cannot fail", decl.Pos(), decl.Body.Lbrace, ret)
}

// isStateType reports whether a composite literal's type names a state, a
// status or a health answer.
func isStateType(typeExpr ast.Expr) bool {
	name := finalName(typeExpr)
	for _, suffix := range []string{"State", "Status", "Health"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// finalName answers the last name of an identifier or a selector, or "".
func finalName(expr ast.Expr) string {
	switch typed := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

// isIdentNamed reports whether an expression is the identifier name.
func isIdentNamed(expr ast.Expr, name string) bool {
	ident, isIdent := ast.Unparen(expr).(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == name
}

// isNil reports whether an expression is the identifier nil.
func isNil(expr ast.Expr) bool { return isIdentNamed(expr, "nil") }

// isBoolLiteral reports whether an expression is `true` or `false`.
func isBoolLiteral(expr ast.Expr) bool {
	if isIdentNamed(expr, "true") {
		return true
	}
	return isIdentNamed(expr, "false")
}

// isIntZero reports whether an expression is the integer literal 0.
func isIntZero(expr ast.Expr) bool {
	literal, isLiteral := ast.Unparen(expr).(*ast.BasicLit)
	if !isLiteral {
		return false
	}
	if literal.Kind != token.INT {
		return false
	}
	return literal.Value == "0"
}

// isZeroLiteral reports whether an expression is a literal zero value: `0`,
// `0.0`, an empty string, `false`, `nil`, or a composite literal with no
// element.
func isZeroLiteral(expr ast.Expr) bool {
	switch typed := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		switch typed.Value {
		case "0", "0.0", `""`, "``":
			return true
		default:
			return false
		}
	case *ast.Ident:
		return typed.Name == "false" || typed.Name == "nil"
	case *ast.CompositeLit:
		return len(typed.Elts) == 0
	default:
		return false
	}
}

// funcName names a declared function, with its receiver type for a method.
func funcName(decl *ast.FuncDecl) string {
	if decl.Recv == nil {
		return decl.Name.Name
	}
	if len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	if star, isStar := receiver.(*ast.StarExpr); isStar {
		receiver = star.X
	}
	if generic, isGeneric := receiver.(*ast.IndexExpr); isGeneric {
		receiver = generic.X
	}
	if generic, isGeneric := receiver.(*ast.IndexListExpr); isGeneric {
		receiver = generic.X
	}
	ident, isIdent := receiver.(*ast.Ident)
	if !isIdent {
		return decl.Name.Name
	}
	return ident.Name + "." + decl.Name.Name
}
