// Design: docs/architecture/plugin/feature-gates.md -- monotone external capabilities.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestExternalJoinMergeCompatibility checks semantic growth and independent
// ownership through direct abstract-value merges, including valid empty values.
func TestExternalJoinMergeCompatibility(t *testing.T) {
	t.Run("writable-empty-fields", func(t *testing.T) {
		var initialized externalValue
		if initialized.join(externalValue{}) {
			t.Fatal("an empty join reported semantic growth")
		}
		// A writer that adds a field makes the table itself (join allocates it
		// only when the other value carries fields), as literal construction does.
		initialized.fields = map[string]externalValue{"Empty": {}}
		var merged externalValue
		if !merged.join(initialized) {
			t.Fatal("a new empty named field did not report semantic growth")
		}
		if _, present := merged.fields["Empty"]; !present {
			t.Fatal("joining lost an explicitly present empty named field")
		}
		if merged.join(initialized) {
			t.Fatal("an identical empty named field reported semantic growth")
		}
		delete(merged.fields, "Empty")
		if _, present := initialized.fields["Empty"]; !present {
			t.Fatal("joined field storage aliases the input")
		}
	})

	t.Run("first-callable-source", func(t *testing.T) {
		first := ast.NewIdent("first")
		second := ast.NewIdent("second")
		original := sourceExpr{}
		later := sourceExpr{value: ast.NewIdent("laterSource"), name: "later source"}
		input := externalValue{callables: map[ast.Node]sourceExpr{first: original}}
		var merged externalValue
		if !merged.join(input) {
			t.Fatal("a new callable did not report semantic growth")
		}
		if merged.join(externalValue{callables: map[ast.Node]sourceExpr{first: later}}) {
			t.Fatal("a repeated callable identity reported semantic growth")
		}
		retained, present := merged.callables[first]
		if !present {
			t.Fatal("a callable with a zero source was discarded")
		}
		if retained != original {
			t.Fatal("a later source replaced the first callable's zero source")
		}
		if !merged.join(externalCallable(second, later)) {
			t.Fatal("a different callable identity was not added")
		}
		if merged.callables[second] != later {
			t.Fatal("a new callable lost its exact source")
		}
		delete(merged.callables, first)
		if _, present := input.callables[first]; !present {
			t.Fatal("joined callable storage aliases the input")
		}
	})

	t.Run("false-boolean-keys", func(t *testing.T) {
		location := &ast.Object{Name: "location"}
		read := &ast.Object{Name: "read"}
		const origin = "retained origin"
		input := externalValue{
			locations: map[*ast.Object]bool{location: false},
			reads:     map[*ast.Object]bool{read: false},
			origins:   map[string]bool{origin: false},
		}
		for _, scenario := range []struct {
			name        string
			destination externalValue
		}{
			{name: "new-keys"},
			{
				name: "existing-false-keys",
				destination: externalValue{
					locations: map[*ast.Object]bool{location: false},
					reads:     map[*ast.Object]bool{read: false},
					origins:   map[string]bool{origin: false},
				},
			},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				merged := scenario.destination
				if !merged.join(input) {
					t.Fatal("retained false-valued keys did not report semantic growth")
				}
				if !merged.locations[location] {
					t.Fatal("joining discarded a false-valued location key")
				}
				if !merged.reads[read] {
					t.Fatal("joining discarded a false-valued metadata read key")
				}
				if !merged.origins[origin] {
					t.Fatal("joining discarded a false-valued origin key")
				}
				if !merged.references {
					t.Fatal("a location key did not imply reference capability")
				}
				if merged.join(input) {
					t.Fatal("identical retained keys reported semantic growth")
				}
				if input.locations[location] {
					t.Fatal("joining changed the input location value")
				}
				if input.reads[read] {
					t.Fatal("joining changed the input metadata read value")
				}
				if input.origins[origin] {
					t.Fatal("joining changed the input origin value")
				}
			})
		}
	})

	t.Run("nested-independent-ownership", func(t *testing.T) {
		leftCallback := ast.NewIdent("leftCallback")
		rightCallback := ast.NewIdent("rightCallback")
		leftRead := &ast.Object{Name: "left read"}
		rightRead := &ast.Object{Name: "right read"}
		leftMember := externalValue{
			callables:        map[ast.Node]sourceExpr{leftCallback: {value: leftCallback}},
			reads:            map[*ast.Object]bool{leftRead: true},
			unknown:          "first nested unknown",
			unresolvedFields: "first nested obligation",
		}
		rightMember := externalValue{
			callables:        map[ast.Node]sourceExpr{rightCallback: {value: rightCallback}},
			reads:            map[*ast.Object]bool{rightRead: true},
			unknown:          "later nested unknown",
			unresolvedFields: "later nested obligation",
		}
		left := externalValue{fields: map[string]externalValue{
			"Outer": {fields: map[string]externalValue{"Member": leftMember, "OnlyLeft": {}}},
		}}
		right := externalValue{fields: map[string]externalValue{
			"Outer": {fields: map[string]externalValue{"Member": rightMember, "OnlyRight": {}}},
		}}
		var merged externalValue
		if !merged.join(left) {
			t.Fatal("the first nested field merge reported no growth")
		}
		if !merged.join(right) {
			t.Fatal("the second nested field merge reported no growth")
		}
		if merged.join(right) {
			t.Fatal("an identical nested field merge reported semantic growth")
		}
		member := merged.fields["Outer"].fields["Member"]
		if member.callables[leftCallback].value != leftCallback {
			t.Fatal("nested merging lost the first callback")
		}
		if member.callables[rightCallback].value != rightCallback {
			t.Fatal("nested merging lost the second callback")
		}
		if !member.reads[leftRead] {
			t.Fatal("nested merging lost the first metadata read")
		}
		if !member.reads[rightRead] {
			t.Fatal("nested merging lost the second metadata read")
		}
		if member.unknown != leftMember.unknown {
			t.Fatal("nested merging replaced the first unknown")
		}
		if member.unresolvedFields != leftMember.unresolvedFields {
			t.Fatal("nested merging replaced the first field obligation")
		}
		if _, present := merged.fields["Outer"].fields["OnlyLeft"]; !present {
			t.Fatal("nested merging lost the first empty named field")
		}
		if _, present := merged.fields["Outer"].fields["OnlyRight"]; !present {
			t.Fatal("nested merging lost the second empty named field")
		}
		var independent externalValue
		independent.join(left)
		independent.join(right)
		delete(member.callables, leftCallback)
		delete(member.callables, rightCallback)
		delete(member.reads, leftRead)
		delete(member.reads, rightRead)
		if member.fields == nil {
			member.fields = make(map[string]externalValue)
		}
		member.fields["New"] = externalValue{}
		delete(merged.fields["Outer"].fields, "OnlyLeft")
		if leftMember.callables[leftCallback].value != leftCallback {
			t.Fatal("nested output callable storage aliases the first input")
		}
		if rightMember.callables[rightCallback].value != rightCallback {
			t.Fatal("nested output callable storage aliases the second input")
		}
		if !leftMember.reads[leftRead] {
			t.Fatal("nested output read storage aliases the first input")
		}
		if !rightMember.reads[rightRead] {
			t.Fatal("nested output read storage aliases the second input")
		}
		otherMember := independent.fields["Outer"].fields["Member"]
		if otherMember.callables[leftCallback].value != leftCallback {
			t.Fatal("independent nested output lost the first callback")
		}
		if otherMember.callables[rightCallback].value != rightCallback {
			t.Fatal("independent nested output lost the second callback")
		}
		if !otherMember.reads[leftRead] {
			t.Fatal("independent nested output lost the first metadata read")
		}
		if !otherMember.reads[rightRead] {
			t.Fatal("independent nested output lost the second metadata read")
		}
		if _, present := otherMember.fields["New"]; present {
			t.Fatal("independent nested outputs share field storage")
		}
		if _, present := left.fields["Outer"].fields["OnlyLeft"]; !present {
			t.Fatal("nested output field storage aliases the input")
		}
		if _, present := independent.fields["Outer"].fields["OnlyLeft"]; !present {
			t.Fatal("independent nested outputs share outer field storage")
		}
	})

	t.Run("unknown-and-reference-order", func(t *testing.T) {
		unknown := externalValue{unknown: "first unknown", unresolvedFields: "first obligation"}
		reference := externalValue{references: true}
		for _, scenario := range []struct {
			name   string
			inputs []externalValue
		}{
			{name: "unknown-first", inputs: []externalValue{unknown, reference}},
			{name: "reference-first", inputs: []externalValue{reference, unknown}},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				var merged externalValue
				for _, input := range scenario.inputs {
					if !merged.join(input) {
						t.Fatal("a distinct unknown or reference fact reported no growth")
					}
				}
				later := externalValue{unknown: "later unknown", unresolvedFields: "later obligation"}
				if merged.join(later) {
					t.Fatal("later conflicting unknown facts reported semantic growth")
				}
				if merged.join(externalValue{}) {
					t.Fatal("an empty value changed retained unknown or reference facts")
				}
				if merged.unknown != unknown.unknown {
					t.Fatal("a later merge replaced the first unknown")
				}
				if merged.unresolvedFields != unknown.unresolvedFields {
					t.Fatal("a later merge replaced the first field obligation")
				}
				if !merged.references {
					t.Fatal("a merge erased an explicit reference without locations")
				}
			})
		}
	})
}

// TestExternalJoinEmptyAllocationBudget requires an empty join to allocate
// nothing: join creates each map only when the other value has entries, and a
// writer that adds a field makes its own table, so an empty input owes no map.
func TestExternalJoinEmptyAllocationBudget(t *testing.T) {
	// A buffered channel forces the output to escape without global state or a
	// receiver goroutine. The sink is allocated before the measurement.
	sink := make(chan externalValue, 1)
	var retained externalValue
	for _, scenario := range []struct {
		name  string
		input externalValue
	}{
		{name: "nil-input-maps"},
		{
			name: "initialized-empty-input-maps",
			input: externalValue{
				callables: make(map[ast.Node]sourceExpr),
				locations: make(map[*ast.Object]bool),
				reads:     make(map[*ast.Object]bool),
				origins:   make(map[string]bool),
				fields:    make(map[string]externalValue),
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			joined := testing.AllocsPerRun(100, func() {
				var value externalValue
				value.join(scenario.input)
				sink <- value
				retained = <-sink
			})
			_ = retained
			if joined != 0 {
				t.Fatalf("empty join allocated %g times/op, want 0", joined)
			}
		})
	}
}
