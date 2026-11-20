package command

import (
	"testing"
)

// TestPipeOperationsForCommand proves the derivation of a command's
// `pipe-operations` list (AC-19): which generics a declared shape keeps, the
// address-field gate, a streaming operator's availability, an override in its
// generic's place with its own description, an own operation with its own
// argument, and an alias with its expansion.
//
// Method: PipeOperations over hand-built declarations, one per case, then
// PipeOperationsForCommand over the registries for one path, so the daemon's
// view is the same join.
func TestPipeOperationsForCommand(t *testing.T) {
	rows := byOperationName(PipeOperations(Declared{Shape: ShapeTab, ShapeDeclared: true}))
	wantOperation(t, "rows", rows, "match", PipeSourceGeneric, PipeAvailableAlways, "text")
	wantOperation(t, "rows", rows, "fill", PipeSourceGeneric, PipeAvailableAlways, "optional")
	wantOperation(t, "rows", rows, "log", PipeSourceGeneric, PipeAvailableWhenStreaming, "none")
	wantOperation(t, "rows", rows, "save", PipeSourceGeneric, PipeAvailableAlways, "path")
	if !rows["save"].LocalOnly {
		t.Errorf("rows: save is not marked local-only")
	}
	if rows["count"].Class != ClassData.String() || !rows["count"].Overridable {
		t.Errorf("rows: count = %+v, want class data and overridable", rows["count"])
	}
	wantAbsent(t, "rows without an address field", rows, "resolve", "origin")

	document := byOperationName(PipeOperations(Declared{Shape: ShapeDoc, ShapeDeclared: true}))
	wantOperation(t, "document", document, "json", PipeSourceGeneric, PipeAvailableAlways, "optional")
	wantAbsent(t, "document", document, "match", "count", "first", "display", "fill")

	undeclared := byOperationName(PipeOperations(Declared{}))
	wantOperation(t, "undeclared", undeclared, "match", PipeSourceGeneric, PipeAvailableWithRows, "text")
	wantOperation(t, "undeclared", undeclared, "table", PipeSourceGeneric, PipeAvailableAlways, "none")
	wantAbsent(t, "undeclared", undeclared, "resolve", "origin")

	// An address field without a declared shape is still refused by the
	// validator, so the address operators stay unpublished.
	unshapedAddress := byOperationName(PipeOperations(Declared{AddressFields: []string{"address"}}))
	wantAbsent(t, "address without shape", unshapedAddress, "resolve", "origin")

	address := byOperationName(PipeOperations(Declared{
		Shape: ShapeMap, ShapeDeclared: true, AddressFields: []string{"address"},
	}))
	wantOperation(t, "address", address, "resolve", PipeSourceGeneric, PipeAvailableAlways, "none")
	wantOperation(t, "address", address, "origin", PipeSourceGeneric, PipeAvailableAlways, "none")
	wantAbsent(t, "map", address, "fill")

	declared := Declared{
		Shape: ShapeDoc, ShapeDeclared: true,
		Operations: []PipeFilter{
			{Name: "count", Description: "Count matching routes", Argument: ArgNone},
			{Name: "histogram", Description: "Routes per prefix length", Argument: ArgText},
		},
		Aliases: []Alias{{Name: "peers", Description: "The peer rows", Expansion: "display state"}},
	}
	list := PipeOperations(declared)
	own := byOperationName(list)
	wantOperation(t, "override", own, "count", PipeSourceOverride, PipeAvailableAlways, "none")
	if got := own["count"]; got.Description != "Count matching routes" || got.Class != ClassData.String() {
		t.Errorf("override: count = %+v, want its own description and the generic's class", got)
	}
	if n := countNamed(list, "count"); n != 1 {
		t.Errorf("override: count appears %d times, want once", n)
	}
	wantOperation(t, "own", own, "histogram", PipeSourceCommand, PipeAvailableAlways, "text")
	if got := own["histogram"]; got.Class != "" || got.Description != "Routes per prefix length" {
		t.Errorf("own: histogram = %+v, want no class and its own description", got)
	}
	wantOperation(t, "alias", own, "peers", PipeSourceAlias, PipeAvailableAlways, "none")
	if got := own["peers"].Expansion; got != "display state" {
		t.Errorf("alias: peers expansion = %q, want display state", got)
	}

	const path = "show introspection rib"
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	t.Cleanup(ResetShapesForTest)
	RegisterShape([]string{path}, ShapeTab)
	RegisterPipeFilters([]string{path}, PipeFilter{Name: "count", Description: "Count matching routes", Argument: ArgNone})
	registered := byOperationName(PipeOperationsForCommand(path))
	wantOperation(t, "registry", registered, "count", PipeSourceOverride, PipeAvailableAlways, "none")
	wantOperation(t, "registry", registered, "fill", PipeSourceGeneric, PipeAvailableAlways, "optional")
}

// TestPipeOperationsAgreeWithValidator proves A-7: every generic the list
// publishes as available is one the chain validator admits on that route, and
// every generic it omits is one the validator refuses, for each answer shape,
// an undeclared shape, and a declaration with and without an address field.
//
// Method: register each declaration on its own path, then run every catalog
// operator, with an argument its contract accepts, through
// validatePipesForSurface on the one-shot and the streaming route.
func TestPipeOperationsAgreeWithValidator(t *testing.T) {
	t.Cleanup(ResetShapesForTest)
	t.Cleanup(ResetAddressFieldsForTest)

	type declaration struct {
		path    string
		shape   AnswerShape
		shaped  bool
		address bool
	}
	declarations := []declaration{{path: "show agree undeclared"}}
	for _, shape := range allShapes {
		declarations = append(declarations,
			declaration{path: "show agree " + shape.String(), shape: shape, shaped: true},
			declaration{path: "show agree " + shape.String() + " address", shape: shape, shaped: true, address: true})
	}
	declarations = append(declarations, declaration{path: "show agree unshaped address", address: true})

	for _, d := range declarations {
		if d.shaped {
			RegisterShape([]string{d.path}, d.shape)
		}
		if d.address {
			RegisterAddressFields([]string{d.path}, "address")
		}
		listed := byOperationName(PipeOperationsForCommand(d.path))
		for _, generic := range pipeCatalog {
			ops, err := parsePipeOps(agreementChain(generic))
			if err != nil {
				t.Fatalf("%s: parse %q: %v", d.path, generic.Name, err)
			}
			oneShot := validatePipesForSurface(d.path, ops, pipeSurfaceOneShot, true) == ""
			stream := validatePipesForSurface(d.path, ops, pipeSurfaceStream, true) == ""
			entry, published := listed[generic.Name]
			switch {
			case !published:
				if oneShot || stream {
					t.Errorf("%s: %s is not listed but the validator admits it", d.path, generic.Name)
				}
			case entry.Available == PipeAvailableWhenStreaming:
				if oneShot || !stream {
					t.Errorf("%s: %s is listed when-streaming, validator one-shot=%v stream=%v",
						d.path, generic.Name, oneShot, stream)
				}
			default:
				if !oneShot {
					t.Errorf("%s: %s is listed %s but the validator refuses it",
						d.path, generic.Name, entry.Available)
				}
			}
		}
	}
}

// agreementChain answers one operator with an argument its contract accepts.
func agreementChain(generic PipeOperator) string {
	switch generic.Arg {
	case ArgText:
		return generic.Name + " x"
	case ArgCount:
		return generic.Name + " 1"
	case ArgFields:
		return generic.Name + " address"
	case ArgPath:
		return generic.Name + " /tmp/ze-agreement"
	case ArgNone, ArgOptional:
		return generic.Name
	default:
		panic("BUG: unknown pipe argument kind")
	}
}

func byOperationName(entries []PipeOperationEntry) map[string]PipeOperationEntry {
	byName := make(map[string]PipeOperationEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	return byName
}

func countNamed(entries []PipeOperationEntry, name string) int {
	n := 0
	for _, entry := range entries {
		if entry.Name == name {
			n++
		}
	}
	return n
}

func wantOperation(t *testing.T, what string, byName map[string]PipeOperationEntry, name, source, available, argument string) {
	t.Helper()
	entry, listed := byName[name]
	if !listed {
		t.Errorf("%s: %s is not listed", what, name)
		return
	}
	if entry.Source != source || entry.Available != available || entry.Argument != argument {
		t.Errorf("%s: %s = %+v, want source %s, available %s, argument %s",
			what, name, entry, source, available, argument)
	}
}

func wantAbsent(t *testing.T, what string, byName map[string]PipeOperationEntry, names ...string) {
	t.Helper()
	for _, name := range names {
		if entry, listed := byName[name]; listed {
			t.Errorf("%s: %s is listed (%+v), want it absent", what, name, entry)
		}
	}
}
