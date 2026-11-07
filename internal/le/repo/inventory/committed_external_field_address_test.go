// Design: docs/architecture/plugin/feature-gates.md -- the storage an address points into.
package repoinventory

import (
	"strings"
	"testing"
)

// externalAddressFixture adds encoding/binary, whose Read writes through its
// pointer argument, to the import-cycle witnesses of both registration sinks,
// so the call needs no other certificate than the ones the fixture proves.
func externalAddressFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\"; _ \"encoding/binary\")\n")
	}
	return fixture
}

func externalAddressSource(declarations, call string) string {
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "binary \"encoding/binary\"\n conduit \"io\"",
	}, call)
}

// externalAddressDecoder is the vendored zstd fseDecoder shape: a struct of
// array, scalar and nested fields whose pointer method reads each field from
// a stream with binary.Read(r, order, &s.field).
const externalAddressDecoder = "type fseCounter struct { count uint16 }\n" +
	"type fseDecoder struct {\n\tdt [4]uint64\n\tsymbolLen uint16\n\tmaxBits uint8\n\tstateTable [8]uint16\n\tinner fseCounter\n\tpreDefined bool\n}\n" +
	"func (s *fseDecoder) mustReadFrom(r conduit.Reader) {\n" +
	"\tfatalErr := func(err error) { _ = err }\n" +
	"\tfatalErr(binary.Read(r, binary.LittleEndian, OPERAND))\n}\n"

const externalAddressDecoderCall = "decoder := &fseDecoder{dt: [4]uint64{}, symbolLen: 0, maxBits: 0, stateTable: [8]uint16{}, inner: fseCounter{count: 0}, preDefined: false}; decoder.mustReadFrom(nil)"

// TestCommittedExternalFieldAddressPublishes requires the address of a field
// selected through a pointer receiver to name the receiver's storage, so the
// standard-library call that writes through it is modeled and the report
// publishes. Method: each operand zstd's fse_decoder.go passes to binary.Read,
// plus a nested field, an array element and a field of a struct variable, is
// written into a method reached from initialization; the fields hold no
// capability, so the only possible refusal is an unresolved address.
//
// MUTATION: (internal/le/repo/inventory/externaladdresses.go,
// "\t\tholder = operand.X\n\tcase *ast.IndexExpr:",
// "\t\treturn externalValue{}, \"selected field storage is withheld\"\n\tcase *ast.IndexExpr:")
// leaves every selected field's storage unresolved: each field case refuses
// as address ownership is unresolved, the defect the actual root met.
func TestCommittedExternalFieldAddressPublishes(t *testing.T) {
	for _, operand := range []string{"&s.dt", "&s.symbolLen", "&s.maxBits", "&s.stateTable", "&s.preDefined", "&s.inner.count", "&s.dt[1]", "&(*s).symbolLen"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(operand+"/"+kind, func(t *testing.T) {
				fixture := externalAddressFixture(t, kind)
				declarations := strings.Replace(externalAddressDecoder, "OPERAND", operand, 1)
				fixture.goFile(t, callableOwnerCaller, externalAddressSource(declarations, externalAddressDecoderCall))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// externalAddressParcel holds a callback beside a scalar. fill passes the
// callback field's address, through a pointer receiver, to binary.Read.
const externalAddressParcel = "type parcel struct {\n\thook func()\n\tsize uint16\n}\n" +
	"func (p *parcel) fill(r conduit.Reader) { _ = binary.Read(r, binary.LittleEndian, &p.hook) }\n"

// TestCommittedExternalFieldAddressKeepsRegistration requires an address to
// carry what its field and its storage hold: a callback reachable through
// the address stays exposed to the external callee. Method: an inert callback
// publishes and a registering one refuses, first through &p.hook on a pointer
// receiver, then through &box.size on a struct variable whose other field holds
// the callback, since exposing a variable's location reads the whole variable.
//
// MUTATION: (internal/le/repo/inventory/externaladdresses.go,
// "\tresult.join(storage)",
// "\tresult = externalValue{references: storage.references}")
// drops the field's value and its storage from the address: the registering
// callback through &p.hook publishes.
// MUTATION: (internal/le/repo/inventory/externaladdresses.go,
// "depth+1); reason == \"\" {", "depth+1); reason == \"-\" {")
// withholds a variable holder's own location: &box.size with the inert
// callback refuses as address ownership is unresolved.
func TestCommittedExternalFieldAddressKeepsRegistration(t *testing.T) {
	for _, shape := range []struct{ name, call string }{
		{"pointer receiver", "box := &parcel{hook: func() { BODY }, size: 0}; box.fill(nil)"},
		{"variable", "box := parcel{hook: func() { BODY }, size: 0}; _ = binary.Read(nil, binary.LittleEndian, &box.size)"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalAddressFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalAddressSource(externalAddressParcel, strings.Replace(shape.call, "BODY", "", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalAddressSource(externalAddressParcel, strings.Replace(shape.call, "BODY", "install(0)", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalFieldAddressWithoutStorageRefuses keeps refusing an
// address whose storage no binding names. Method: a field of a retained call's
// fresh pointer result has no variable holder and its value refers to no
// retained location, so a write through it cannot be placed; the report
// refuses with the address reason rather than publishing.
//
// MUTATION: (internal/le/repo/inventory/externaladdresses.go,
// "if len(storage.locations) == 0 {", "if len(storage.locations) < 0 {")
// admits the unplaced address: the report publishes.
func TestCommittedExternalFieldAddressWithoutStorageRefuses(t *testing.T) {
	const declarations = "type parcel struct { size uint16 }\nfunc fresh() *parcel { return &parcel{size: 0} }\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalAddressFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalAddressSource(declarations, "_ = binary.Read(nil, binary.LittleEndian, &fresh().size)"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "address ownership is unresolved")
		})
	}
}

// TestCommittedExternalLocalClosureCallBindsFormal requires a direct call
// through a local closure's name to bind the closure's formal, as a call of
// a function literal does, while a closure that escapes as a value still
// refuses its unseen callers. Method: zstd's fatalErr helper receives a
// standard-library call result and reads it, which demands the formal's
// provenance; copying the closure into a second name makes it escape.
//
// MUTATION: (internal/le/repo/inventory/externalformals.go,
// "if literal, bound := externalClosureBinding(name); bound {",
// "if literal, bound := externalClosureBinding(name); bound && false {")
// leaves calls through the name unindexed: the direct helper refuses as
// indirect retained caller provenance is unresolved.
// MUTATION: (internal/le/repo/inventory/callablealiases.go,
// "if closureCalledOnly(source.file, binding) {",
// "if closureCalledOnly(source.file, binding) && false {")
// sends the direct call back to the alias chain: the init walker refuses
// it as a callable closure alias with unresolved execution ownership.
// The escape case refuses at whichever layer meets it first, the walker
// (closure alias) or the formal census (indirect provenance), so it asserts
// only the shared word of both reasons.
func TestCommittedExternalLocalClosureCallBindsFormal(t *testing.T) {
	const helper = "fatalErr := func(err error) { _ = err }; "
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", helper+"fatalErr(remote.Chdir(\".\"))"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", helper+"escaped := fatalErr; _ = escaped; fatalErr(remote.Chdir(\".\"))"))
			commitInventoryFixture(t, fixture.root)
			inventoryRefuses(t, fixture, "indirect retained caller provenance is unresolved")
		})
	}
}

// inventoryRefuses requires both inventory entry points to refuse with reason
// and to publish nothing, whichever analysis layer produced the refusal.
func inventoryRefuses(t *testing.T, fixture *asmInventoryFixture, reason string) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("unresolved callee published a report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("unresolved callee published plugins: %+v", plugins)
	}
	if plugins != nil {
		t.Fatalf("refusal leaked partial plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("refusal cause %q absent: %v", reason, err)
		}
	}
}
