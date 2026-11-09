// Design: docs/architecture/plugin/feature-gates.md -- a generic type's methods execute as its capability.
package repoinventory

import (
	"go/ast"
	"strings"
	"testing"
)

// genericQueue is the shape of cilium/ebpf's internal.Deque, held by value in
// btf's encoder: a generic struct whose methods move values of type T.
const genericQueue = "type queue[T any] struct { elems []T }\n" +
	"func (q *queue[T]) Push(e T) { q.elems = append(q.elems, e) }\n" +
	"func (q *queue[T]) Shift() T { var zero T; if len(q.elems) == 0 { return zero }; head := q.elems[0]; q.elems = q.elems[1:]; return head }\n" +
	"type encoder struct { pending queue[int] }\n"

const genericExposure = "enc := &encoder{}; order.Slice([]any{enc}, func(i, j int) bool { return false })"

// TestCommittedExternalGenericMethodsExecute requires a retained value whose
// field is a generic type to expose that type's methods as executed bodies,
// as btf's encoder exposes Deque's. Method: an encoder reaches sort.Slice,
// so its type capability, the methods of queue[int] included, reaches the
// ambient pool; every method is inert, so the report publishes. A further
// method of queue that installs refuses as registration, so the bodies were
// walked and not skipped.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypeidentity.go,
// `		if function.decl.Body == nil {`,
// `		if receiver.generic { result.join(externalValue{unknown: "generic receiver method capability execution is unsupported"}); continue }; if function.decl.Body == nil {`):
// the generic refusal returns, and the inert case refuses as registration
// capability is unknown.
// MUTATION: (internal/le/repo/inventory/externaleffecttypeidentity.go,
// `			if receivers := externalReceiverTypeParameters(declaration); len(receivers) != 0 {`,
// `			if receivers := externalReceiverTypeParameters(declaration); len(receivers) == -1 {`):
// T in a method body is no longer a type parameter, so var zero T resolves
// no declaration and the inert case refuses.
func TestCommittedExternalGenericMethodsExecute(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run("inert/"+kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(genericQueue, genericExposure))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
		t.Run("registering/"+kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			registering := genericQueue + "func (q *queue[T]) Flush() { install(0); q.elems = nil }\n"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(registering, genericExposure))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalGenericInstantiationsBoundReflection requires an
// exposed generic method's type parameter to reflect as every type the
// program instantiates its receiver with. Method: the exposed encoder holds
// queue[int], and a separate package variable instantiates queue[secret],
// whose exported method installs. Shift's signature names T, so reflecting
// the exposed method reveals secret's descriptor and its Fire method, and
// the report refuses; with only queue[int] written it publishes (the inert
// case of TestCommittedExternalGenericMethodsExecute).
//
// MUTATION: (internal/le/repo/inventory/externalreflectiontypes.go,
// `	arguments, reason := w.effects.receiverInstantiations(parameter)`,
// `	arguments, reason := []externalTypeArgument(nil), ""`): the census
// contributes nothing, secret stays hidden, and the report publishes.
func TestCommittedExternalGenericInstantiationsBoundReflection(t *testing.T) {
	declarations := genericQueue +
		"type secret struct{}\nfunc (secret) Fire() { install(0) }\nvar unrelated queue[secret]\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, "_ = unrelated; "+genericExposure))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalGenericParameterMethodDispatchesStoredValue requires
// a method called on a value whose type is a type parameter to run what the
// value holds: the method set is the constraint's, and no declaration says
// which body runs, so the call dispatches through the stored value's own
// capability. Method: holder's Go method calls Run on its T field, and an
// exposed outer holds a holder[runner]; with item unset it publishes, and
// with item set to loud, whose Run installs, it refuses as registration.
func TestCommittedExternalGenericParameterMethodDispatchesStoredValue(t *testing.T) {
	const declarations = "type runner interface { Run() }\n" +
		"type loud struct{}\nfunc (loud) Run() { install(0) }\n" +
		"type holder[T runner] struct { item T }\n" +
		"func (h *holder[T]) Go() { h.item.Run() }\n" +
		"type outer struct { inner holder[runner] }\n"
	for _, shape := range []struct{ name, literal string }{
		{"unset", "&outer{}"},
		{"stored", "&outer{inner: holder[runner]{item: loud{}}}"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				call := strings.Replace(genericExposure, "&encoder{}", shape.literal, 1)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
				commitInventoryFixture(t, fixture.root)
				if shape.name == "unset" {
					externalEffectsPositive(t, fixture)
					return
				}
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestExternalLexicalScopesBindReceiverParameters requires both lexical type
// scope routes, the per-file index and the traversal it falls back to for a
// file the index does not hold, to bind a method's receiver type parameter
// and a generic declaration's own parameter as parameters the instantiation
// census can bound. Method: T read in Shift's body and in queue's field type.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypeidentity.go,
// `			if len(externalReceiverTypeParameters(declaration)) != 0 {`,
// `			if len(externalReceiverTypeParameters(declaration)) == -1 {`):
// the traversal no longer binds the receiver's T, and the body read fails.
func TestExternalLexicalScopesBindReceiverParameters(t *testing.T) {
	const directory = "internal/genericscopeproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		filename: "package genericscopeproof\n" + genericQueue,
	})
	file := typeIdentitySelectedFile(t, effects.source.snapshot.packages[directory], filename)
	pkg := effects.source.snapshot.packages[directory]
	var reads []*ast.Ident
	ast.Inspect(file, func(node ast.Node) bool {
		if name, named := node.(*ast.Ident); named && name.Name == "T" {
			reads = append(reads, name)
		}
		return true
	})
	// The struct field's T, then the zero declaration's T inside Shift.
	var field, body *ast.Ident
	for _, read := range reads {
		position := effects.source.snapshot.fset.Position(read.Pos())
		switch position.Line {
		case 2:
			if field == nil && position.Column > 20 {
				field = read
			}
		case 4:
			if body == nil && position.Column > 40 {
				body = read
			}
		}
	}
	if field == nil || body == nil {
		t.Fatalf("fixture reads of T not found: field %v body %v", field, body)
	}
	for _, read := range []struct {
		name  string
		ident *ast.Ident
	}{{"declaration field", field}, {"method body", body}} {
		source := sourceExpr{value: read.ident, file: file, pkg: pkg}
		for _, route := range []struct {
			name  string
			scope *externalTypeScope
		}{
			{"index", effects.typeIdentities.lexicalScope(source)},
			{"traversal", externalLexicalTypeScope(source)},
		} {
			binding, found := route.scope.binding(read.ident)
			if !found {
				t.Fatalf("%s read of T is unbound through the %s route", read.name, route.name)
			}
			if binding.receiver == nil {
				t.Fatalf("%s read of T through the %s route carries no census parameter", read.name, route.name)
			}
		}
	}
}
