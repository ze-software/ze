// Design: docs/architecture/plugin/feature-gates.md -- final external-effect closure.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

type externalIssueKind uint8

const (
	externalIssueInvalid externalIssueKind = iota
	externalUnresolved
	externalUnsupported
)

// externalIssue distinguishes incomplete ownership from a directly observed
// unsupported operation. Only incomplete ownership permits private diagnostic
// traversal; neither class permits publication.
type externalIssue struct {
	kind     externalIssueKind
	position token.Pos
	cause    error
}

func (issue *externalIssue) Error() string { return issue.cause.Error() }
func (issue *externalIssue) Unwrap() error { return issue.cause }

// externalEffects is private to one report reader. Bindings, return summaries,
// reachable bodies and ambient references only grow. No provisional admission
// may be consumed until solve has reached its fixed point and validated metadata.
type externalEffects struct {
	source         *externalSource
	resolver       *expression
	typeIdentities *externalTypeIdentityIndex
	// instantiations indexes written generic instantiations by base name,
	// built on first use (instantiationSites).
	instantiations  map[string][]externalInstantiationSite
	receiverQueries map[externalReceiverQuery]externalReceiverResult
	// ambient counts growths of amb, so a settled type value read from one
	// pool is never answered once the pool has grown (growAmbient).
	ambient int
	// ambientGrowths keeps each value that grew amb, in order: growth n is
	// ambientGrowths[n-1]. The pool only grows, so the pool at version v is
	// the pool at an earlier version u joined with growths u+1 to v.
	ambientGrowths []externalValue
	// formalAmbient records, for each external parameter, the ambient version
	// its binding last absorbed the pool at. Bindings only grow, so a
	// parameter joins only the growths after that version (externalParameters).
	formalAmbient map[*ast.Object]int
	// readCache holds each object's complete readObject answer for the
	// current fixpoint round; solve clears it at the start of every round.
	readCache map[*ast.Object]externalValue
	// settledTypes keeps settled type frames for the whole analysis
	// (externalTypeWalk.settle).
	settledTypes map[int32]externalTypeSettled
	// typeProbe is set only by tests (externalTypeProbe); production leaves it nil.
	typeProbe *externalTypeProbe
	// reflectionNames memoizes the reflection walk's resolution of each type
	// reference it meets, and reflectionKeys interns what they resolve to
	// (externalReflectionNamed). Both live for the whole analysis.
	reflectionNames map[externalReflectionNameKey]externalReflectionNamed
	reflectionKeys  externalReflectionKeys
	// reflectedValues keeps reflectedValue answers for the whole analysis
	// (externalReflectionMemo).
	reflectedValues externalReflectionMemo
	// typeTerms interns exact type arguments and frames, and
	// reflectionFrames keeps each reflection frame's contribution, for the
	// whole analysis (externalTypeTerms, externalReflectionWalk).
	typeTerms        externalTypeTerms
	reflectionFrames map[int32]externalReflection
	// counters is set only by tests (externalAnalysisCounters); production
	// leaves it nil and counts nothing.
	counters        *externalAnalysisCounters
	methodCensuses  map[*sourcePackage]*externalMethodCensus
	bindings        map[*ast.Object]externalValue
	returns         map[ast.Node]externalValue
	bodies          map[ast.Node]sourceExpr
	possible        map[ast.Node]bool
	amb             externalValue
	metadata        map[*ast.Object]bool
	operations      map[externalCallSite]sourceExpr
	admitted        map[externalCallSite]bool
	changed         bool
	failure         error
	definitions     map[*ast.Object][]externalDefinition
	types           map[*ast.Object]sourceExpr
	reading         map[*ast.Object]bool
	stores          []externalStore
	linknames       map[*ast.Object]sourceExpr
	linkDefinitions map[string][]sourceExpr
	linkInvalid     map[string]sourceExpr
	assembly        *assemblyIndex
	assemblyErr     error
	writes          map[*ast.Object]sourceExpr
	formals         map[*ast.Object]externalFormal
	formalDemand    map[*ast.Object]bool
	formalExternal  map[*ast.Object]bool
	formalIndirect  map[ast.Node]sourceExpr
	// formalUnbound holds methods taken as a method value: the value binds
	// the receiver where it is taken, but its parameters come from callers
	// the census cannot see.
	formalUnbound map[ast.Node]sourceExpr
	formalInvalid map[ast.Node]string
	// rangeIndexes holds each package's occurrence index for the ranged
	// element population proof; rangeVariables the scope of every fresh
	// range variable, so formal provenance can reach its collection.
	rangeIndexes   map[*sourcePackage]*externalRangeUses
	rangeVariables map[*ast.Object]sourceExpr
	// indexedRound holds, for each indexed collection (its variable, or the
	// index expression when it is no variable), the reason its element
	// population is not proved this round, empty once proved; beginRound
	// clears it. indexedElements holds the element value each proved
	// collection has joined so far, monotone across rounds, as bindings holds
	// a range variable's (indexedValue).
	indexedRound    map[any]string
	indexedElements map[any]externalValue
}

func newExternalEffects(source *externalSource) *externalEffects {
	return &externalEffects{
		source: source, resolver: source.snapshot.evaluator(), bindings: make(map[*ast.Object]externalValue),
		typeIdentities:  newExternalTypeIdentityIndex(source.snapshot),
		receiverQueries: make(map[externalReceiverQuery]externalReceiverResult),
		methodCensuses:  make(map[*sourcePackage]*externalMethodCensus),
		returns:         make(map[ast.Node]externalValue), bodies: make(map[ast.Node]sourceExpr),
		possible: make(map[ast.Node]bool), metadata: make(map[*ast.Object]bool),
		operations: make(map[externalCallSite]sourceExpr), admitted: make(map[externalCallSite]bool),
		definitions: make(map[*ast.Object][]externalDefinition), types: make(map[*ast.Object]sourceExpr), reading: make(map[*ast.Object]bool),
		linknames:       make(map[*ast.Object]sourceExpr),
		linkDefinitions: make(map[string][]sourceExpr),
		linkInvalid:     make(map[string]sourceExpr),
		writes:          make(map[*ast.Object]sourceExpr),
		formals:         make(map[*ast.Object]externalFormal),
		formalDemand:    make(map[*ast.Object]bool),
		formalExternal:  make(map[*ast.Object]bool),
		formalAmbient:   make(map[*ast.Object]int),
		readCache:       make(map[*ast.Object]externalValue),
		formalIndirect:  make(map[ast.Node]sourceExpr),
		formalUnbound:   make(map[ast.Node]sourceExpr),
		formalInvalid:   make(map[ast.Node]string),
		rangeIndexes:    make(map[*sourcePackage]*externalRangeUses),
		rangeVariables:  make(map[*ast.Object]sourceExpr),
		indexedRound:    make(map[any]string),
		indexedElements: make(map[any]externalValue),
	}
}

func (e *externalEffects) problem(source sourceExpr, reason string) {
	e.issue(source, reason, externalUnresolved)
}

func (e *externalEffects) issue(source sourceExpr, reason string, kind externalIssueKind) {
	position := token.NoPos
	if source.value != nil {
		position = source.value.Pos()
	} else if source.file != nil {
		position = source.file.Pos()
	}
	e.prefer(&externalIssue{kind: kind, position: position, cause: e.cause(source, reason)})
}

// cause names the committed site of reason when one is known. An issue with
// no site still refuses, with the reason alone.
func (e *externalEffects) cause(source sourceExpr, reason string) error {
	if source.file == nil {
		return fmt.Errorf("external %s", reason)
	}
	return e.source.snapshot.evaluator().problem(source, "external "+reason)
}

// prefer keeps the issue of the highest kind, then the earliest position, then
// the smallest message, so the refusal does not depend on visiting order.
func (e *externalEffects) prefer(candidate *externalIssue) {
	previous, found := e.failure.(*externalIssue)
	if found {
		if previous.kind > candidate.kind {
			return
		}
		if previous.kind == candidate.kind {
			if previous.position < candidate.position {
				return
			}
			if previous.position == candidate.position {
				if previous.Error() <= candidate.Error() {
					return
				}
			}
		}
	}
	e.failure = candidate
}

// refuseUnknown stops only on a permanent ambient fact. Provisional failures
// still need later rounds; current preferred issues retain their normal order.
func (e *externalEffects) refuseUnknown() error {
	if e.amb.unknown == "" {
		return nil
	}
	// The ambient refusal keeps no position, so it orders before every other
	// unresolved issue as it always has; only its message names the site
	// that first produced the reason (unknownAt, absent only when no walk
	// could site it).
	reason := "registration capability is unknown: " + e.amb.unknown
	e.prefer(&externalIssue{kind: externalUnresolved, position: token.NoPos, cause: e.cause(e.amb.unknownAt, reason)})
	return e.failure
}

func (e *externalEffects) bind(object *ast.Object, value externalValue) {
	if e.amb.unknown != "" {
		return
	}
	if object == nil {
		return
	}
	current := e.bindings[object]
	if current.join(value) {
		e.bindings[object] = current
		e.changed = true
	}
}

func (e *externalEffects) body(node ast.Node, source sourceExpr, possible bool) {
	if e.amb.unknown != "" {
		return
	}
	if function, declared := node.(*ast.FuncDecl); declared {
		if e.linkedObject(function.Name.Obj) {
			return
		}
	}
	if _, exists := e.bodies[node]; !exists {
		e.bodies[node] = source
		e.changed = true
	}
	if possible {
		if !e.possible[node] {
			e.possible[node] = true
			e.changed = true
		}
	}
}

// solve uses 128 monotone rounds, each visiting only captured declarations and
// scheduled bodies. Exhaustion refuses; recursion never yields an early clean
// result merely because its declaration is already active. Permanent ambient
// unknown stops expansion before another round can clear the current failure.
func (e *externalEffects) solve() error {
	if err := e.refuseUnknown(); err != nil {
		return err
	}
	directories := make([]string, 0, len(e.source.snapshot.packages))
	for directory := range e.source.snapshot.packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	e.indexDefinitions(directories)
	e.indexFormalSources(directories)
	for directory := range e.source.roots {
		for _, identity := range e.source.imports[directory] {
			first, _, _ := strings.Cut(identity, "/")
			if strings.Contains(first, ".") {
				continue
			}
			// unsafe is a compiler intrinsic, not an external initializer.
			// Its operations refuse separately at the capability-call boundary.
			if identity == "unsafe" {
				continue
			}
			if err := e.source.witness(identity); err != nil {
				return err
			}
			e.growAmbient(externalOrigin(identity))
		}
	}
	for range 128 {
		if err := e.refuseUnknown(); err != nil {
			return err
		}
		e.beginRound()
		for _, directory := range directories {
			pkg := e.source.snapshot.packages[directory]
			if e.source.roots[directory] {
				e.linkageDefinitions(directory)
				for _, initializer := range pkg.initializerValues {
					e.value(initializer, false, 0)
					if err := e.refuseUnknown(); err != nil {
						return err
					}
				}
				for _, initializer := range pkg.initializers {
					e.body(initializer.decl, sourceExpr{file: initializer.file, pkg: pkg}, false)
					if err := e.refuseUnknown(); err != nil {
						return err
					}
				}
			}
		}
		for node, source := range e.bodies {
			var body *ast.BlockStmt
			switch function := node.(type) {
			case *ast.FuncDecl:
				body = function.Body
			case *ast.FuncLit:
				body = function.Body
			}
			if body == nil {
				function, declared := node.(*ast.FuncDecl)
				if !declared {
					e.problem(source, "execution body is unavailable")
					continue
				}
				result := e.assemblyResult(sourceFunction{decl: function, file: source.file, pkg: source.pkg}, e.amb)
				if err := e.refuseUnknown(); err != nil {
					return err
				}
				current := e.returns[node]
				if current.join(result) {
					e.returns[node] = current
					e.changed = true
				}
				continue
			}
			e.statements(body, source, node, e.possible[node])
			e.namedResults(node, source)
			if err := e.refuseUnknown(); err != nil {
				return err
			}
		}
		e.staticRoots()
		if err := e.refuseUnknown(); err != nil {
			return err
		}
		e.expose(e.amb)
		if err := e.refuseUnknown(); err != nil {
			return err
		}
		e.indirectStores()
		if err := e.refuseUnknown(); err != nil {
			return err
		}
		if e.changed {
			continue
		}
		e.validateFormals()
		if e.failure != nil {
			return e.failure
		}
		for object := range e.amb.locations {
			if e.metadata[object] {
				return fmt.Errorf("external mutation can change registration metadata through %s; isolate the retained metadata before publishing inventory", object.Name)
			}
		}
		for object, source := range e.writes {
			if e.metadata[object] {
				return e.source.snapshot.evaluator().problem(source, "external registration metadata depends on a mutated retained backing location")
			}
		}
		for site := range e.operations {
			e.admitted[site] = true
		}
		return nil
	}
	return fmt.Errorf("external registration effect closure exceeds 128 rounds; simplify the retained capability graph before publishing inventory")
}

// expose expands the ambient pool until its monotone unknown guarantees refusal.
// A local unknown becomes terminal only after its value joins that pool.
func (e *externalEffects) expose(value externalValue) {
	if e.amb.unknown != "" {
		return
	}
	if e.growAmbient(value) {
		e.changed = true
	}
	if e.amb.unknown != "" {
		return
	}
	e.promoteReflection(value.reflection)
	if e.amb.unknown != "" {
		return
	}
	for object := range value.locations {
		if e.growAmbient(e.readObject(object)) {
			e.changed = true
		}
		if e.amb.unknown != "" {
			return
		}
	}
	for node, source := range value.callables {
		if externalRegistration(node, source) {
			e.problem(source, "registration may execute through an escaped capability")
			continue
		}
		e.exposeSignature(node, source)
		if e.amb.unknown != "" {
			return
		}
		e.externalParameters(node, source)
		if e.amb.unknown != "" {
			return
		}
		e.body(node, source, true)
		if e.amb.unknown != "" {
			return
		}
		if e.growAmbient(e.returns[node]) {
			e.changed = true
		}
		if e.amb.unknown != "" {
			return
		}
	}
}

func (e *externalEffects) valueSpec(spec *ast.ValueSpec, scope sourceExpr) {
	var joined externalValue
	for _, value := range spec.Values {
		joined.join(e.value(sourceExpr{value: value, file: scope.file, pkg: scope.pkg}, true, 0))
	}
	if spec.Type != nil {
		joined.join(e.typeMethods(sourceExpr{value: spec.Type, file: scope.file, pkg: scope.pkg}, 0))
	}
	for _, name := range spec.Names {
		e.bind(name.Obj, joined)
	}
}

func (e *externalEffects) assignment(statement *ast.AssignStmt, scope sourceExpr, possible bool) {
	var joined externalValue
	for _, right := range statement.Rhs {
		joined.join(e.value(sourceExpr{value: right, file: scope.file, pkg: scope.pkg}, possible, 0))
	}
	for _, left := range statement.Lhs {
		object := e.object(sourceExpr{value: left, file: scope.file, pkg: scope.pkg})
		if object != nil {
			if statement.Tok != token.DEFINE {
				e.writes[object] = sourceExpr{value: left, file: scope.file, pkg: scope.pkg}
			}
			e.bind(object, joined)
			continue
		}
		if name, ok := left.(*ast.Ident); ok {
			if name.Name == "_" {
				continue
			}
		}
		target := sourceExpr{value: left, file: scope.file, pkg: scope.pkg}
		location := e.storeLocation(target, possible)
		if len(location.locations) == 0 {
			// Sited at the target, never the file: a file-start position
			// orders first and would mask the target's own reason.
			reason := "assignment location ownership is unresolved"
			if location.unknown != "" {
				reason += ": " + location.unknown
			}
			e.problem(target, reason)
			continue
		}
		for object := range location.locations {
			e.writes[object] = sourceExpr{value: left, file: scope.file, pkg: scope.pkg}
			e.bind(object, joined)
		}
	}
}

// send stores the sent value into every location the channel operand names,
// the way assignment stores into an indexed slice element: a channel's binding
// is the flattened union of everything any holder sent, so a receive anywhere
// reads it. A channel whose location is unresolved refuses, never drops it.
func (e *externalEffects) send(statement *ast.SendStmt, scope sourceExpr, possible bool) {
	target := sourceExpr{value: statement.Chan, file: scope.file, pkg: scope.pkg}
	sent := externalFlat(e.value(sourceExpr{value: statement.Value, file: scope.file, pkg: scope.pkg}, possible, 0), 0)
	location := e.value(target, possible, 0)
	if len(location.locations) == 0 {
		reason := "channel send location ownership is unresolved"
		if location.unknown != "" {
			reason += ": " + location.unknown
		}
		e.problem(target, reason)
		return
	}
	for object := range location.locations {
		e.writes[object] = target
		e.bind(object, sent)
	}
}

func (e *externalEffects) statements(node ast.Node, scope sourceExpr, owner ast.Node, possible bool) {
	if e.amb.unknown != "" {
		return
	}
	if node == nil {
		return
	}
	switch statement := node.(type) {
	case *ast.FuncLit:
		return
	case *ast.CallExpr:
		e.value(sourceExpr{value: statement, file: scope.file, pkg: scope.pkg}, possible, 0)
		return
	case *ast.ReturnStmt:
		result := e.returns[owner]
		for _, expression := range statement.Results {
			if result.join(e.value(sourceExpr{value: expression, file: scope.file, pkg: scope.pkg}, possible, 0)) {
				e.changed = true
			}
		}
		e.returns[owner] = result
		return
	case *ast.AssignStmt:
		e.assignment(statement, scope, possible)
		return
	case *ast.RangeStmt:
		e.rangeExecution(sourceExpr{value: statement.X, file: scope.file, pkg: scope.pkg})
		e.rangeAssignment(statement, scope, possible)
		// An unresolved source kind alone does not prune header/body effects.
		// Only permanent ambient unknown stops their remaining expansion.
		e.statements(statement.Key, scope, owner, possible)
		e.statements(statement.Value, scope, owner, possible)
		e.statements(statement.X, scope, owner, possible)
		e.statements(statement.Body, scope, owner, possible)
		return
	case *ast.IncDecStmt:
		target := sourceExpr{value: statement.X, file: scope.file, pkg: scope.pkg}
		if object := e.object(target); object != nil {
			e.writes[object] = target
			return
		}
		location := e.storeLocation(target, possible)
		if len(location.locations) == 0 {
			e.problem(target, "increment location ownership is unresolved")
			return
		}
		for object := range location.locations {
			e.writes[object] = target
		}
		return
	case *ast.GoStmt:
		e.value(sourceExpr{value: statement.Call, file: scope.file, pkg: scope.pkg}, true, 0)
		return
	case *ast.DeferStmt:
		e.value(sourceExpr{value: statement.Call, file: scope.file, pkg: scope.pkg}, true, 0)
		return
	case *ast.SendStmt:
		e.send(statement, scope, possible)
		return
	}
	first := true
	ast.Inspect(node, func(child ast.Node) bool {
		if e.amb.unknown != "" {
			return false
		}
		if first {
			first = false
			return true
		}
		if child != nil {
			e.statements(child, scope, owner, possible)
		}
		return false
	})
}

// beginRound resets the per-round state of the fixpoint: the change flag, the
// round's preferred failure, the read cache and the indexed population
// answers, whose answers hold only for the round that filled them.
func (e *externalEffects) beginRound() {
	e.changed = false
	e.failure = nil
	clear(e.readCache)
	clear(e.indexedRound)
	if e.counters != nil {
		e.counters.fixpointRounds++
	}
}

// growAmbient is the one writer of amb. Every growth advances ambient, which
// keys the settled type values, because a type walk reads the pool for
// ambient type parameters and standard-library types.
func (e *externalEffects) growAmbient(value externalValue) bool {
	if !e.amb.join(value) {
		return false
	}
	e.ambient++
	// The log holds its own copy, so a caller that later grows value cannot
	// change a growth already recorded.
	var growth externalValue
	growth.join(value)
	e.ambientGrowths = append(e.ambientGrowths, growth)
	return true
}

// namedResults joins the value of each named result variable into the
// function's return summary. Go specification, Return statements: "The
// expression list may be empty if the function's result type specifies names
// for its result parameters." and "A "return" statement that specifies results
// sets the result parameters before any deferred functions are executed." A
// named result is a variable of its declared type: a read of it yields its
// zero value and every assignment to it, a deferred closure's included, so
// joining that read with the explicit return values bounds what the caller
// receives, whichever return the body takes.
func (e *externalEffects) namedResults(node ast.Node, source sourceExpr) {
	var signature *ast.FuncType
	switch function := node.(type) {
	case *ast.FuncDecl:
		signature = function.Type
	case *ast.FuncLit:
		signature = function.Type
	}
	if signature == nil || signature.Results == nil {
		return
	}
	result := e.returns[node]
	grew := false
	for _, field := range signature.Results.List {
		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			if name.Obj == nil {
				// Object resolution binds every named result; without it the
				// variable cannot be read, so its value is not known.
				e.problem(source, "named result "+name.Name+" has no resolved variable")
				return
			}
			value := e.readObject(name.Obj)
			value.join(externalValue{reads: map[*ast.Object]bool{name.Obj: true}})
			if result.join(value) {
				grew = true
			}
		}
	}
	if grew {
		e.returns[node] = result
		e.changed = true
	}
}

// storeLocation answers the locations a store to target writes: those its
// value names, or, for a direct field of a collection element (x[i].f, or p.f
// through p := &x[i]), the collection's storage (addressStorage). Only that
// write may take the holder's storage, because the element population proof
// records it as a source of field f (externalFieldStore), so a field read of
// the element sees it; the storage keeps it in the collection's flattened
// union for every other reader. Any other field write (s.f on a struct
// variable, x[i].p.f through a pointer field) would reach only that union,
// which a field read never consults, so it keeps naming no location and
// refuses. An unknown value keeps its reason.
func (e *externalEffects) storeLocation(target sourceExpr, possible bool) externalValue {
	location := e.value(target, possible, 0)
	if len(location.locations) == 0 && location.unknown == "" && externalElementFieldTarget(target) {
		if storage, reason := e.addressStorage(target, possible, 0); reason == "" {
			location.join(storage)
		}
	}
	return location
}

// externalElementFieldTarget reports whether target is a direct field of a
// collection element: x[i].f, or p.f where p's one declaration is p := &x[i].
func externalElementFieldTarget(target sourceExpr) bool {
	selector, selected := ast.Unparen(target.value).(*ast.SelectorExpr)
	if !selected {
		return false
	}
	switch holder := ast.Unparen(selector.X).(type) {
	case *ast.IndexExpr:
		return true
	case *ast.Ident:
		if holder.Obj == nil {
			return false
		}
		assignment, assigned := holder.Obj.Decl.(*ast.AssignStmt)
		if !assigned || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return false
		}
		address, addressed := ast.Unparen(assignment.Rhs[0]).(*ast.UnaryExpr)
		if !addressed || address.Op != token.AND {
			return false
		}
		_, indexed := ast.Unparen(address.X).(*ast.IndexExpr)
		return indexed
	}
	return false
}
