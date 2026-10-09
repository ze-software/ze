// Design: docs/architecture/api/commands.md — command argument definitions

package command

import (
	"errors"
	"regexp"
	"slices"
	"testing"
)

// mustArgDef answers def, and fails the test binary when its constructor
// refused: a test's definition is literal data, so a refusal is a test defect.
// commandtest.Must is the same helper for other packages; it cannot be
// imported here, because it imports this package.
func mustArgDef(def ArgDef, err error) ArgDef {
	if err != nil {
		panic("BUG: test argument definition refused: " + err.Error())
	}
	return def
}

// VALIDATES: the ArgDef constructors refuse every definition the invariant
// forbids, copy their inputs, and expose no slice.
// PREVENTS: a definition whose ranges overlap or whose payload the kind does
// not carry, and a validated definition changed through an alias.

// TestArgDefConstructorRefuses proves each constructor refuses an invalid
// definition and returns no value for it.
//
// Method: every case changes ONE fact of a definition that the first table
// proves valid, so a refusal is attributable to that fact alone. The refusal
// must wrap ErrArgDef and the value returned must not be a constructed one.
func TestArgDefConstructorRefuses(t *testing.T) {
	word := regexp.MustCompile(`^[a-z]+$`)
	valid := map[string]func() (ArgDef, error){
		"string": func() (ArgDef, error) {
			return NewStringArg("label", []UintRange{{1, 8}, {16, 32}}, []*regexp.Regexp{word}, ArgOptions{})
		},
		"uint": func() (ArgDef, error) { return NewUintArg("count", 8, []UintRange{{0, 10}, {20, 255}}, ArgOptions{}) },
		"enum": func() (ArgDef, error) { return NewEnumArg("mode", []string{"on", "off"}, ArgOptions{}) },
		"flag": func() (ArgDef, error) { return NewFlagArg("force", ArgOptions{}) },
		"union": func() (ArgDef, error) {
			return NewUnionArg("target", []ArgDef{mustArgDef(NewEnumArg("target", []string{"all"}, ArgOptions{}))}, ArgOptions{})
		},
	}
	for name, build := range valid {
		def, err := build()
		if err != nil {
			t.Fatalf("baseline %s refused: %v", name, err)
		}
		if !def.constructed {
			t.Fatalf("baseline %s: constructor returned an unconstructed value", name)
		}
	}

	refused := map[string]func() (ArgDef, error){
		"empty name": func() (ArgDef, error) {
			return NewStringArg("", []UintRange{{1, 8}, {16, 32}}, []*regexp.Regexp{word}, ArgOptions{})
		},
		"width 7":   func() (ArgDef, error) { return NewUintArg("count", 7, []UintRange{{0, 10}, {20, 100}}, ArgOptions{}) },
		"width 0":   func() (ArgDef, error) { return NewUintArg("count", 0, []UintRange{{0, 10}, {20, 100}}, ArgOptions{}) },
		"width 128": func() (ArgDef, error) { return NewUintArg("count", 128, []UintRange{{0, 10}, {20, 100}}, ArgOptions{}) },
		"range above width": func() (ArgDef, error) {
			return NewUintArg("count", 8, []UintRange{{0, 10}, {20, 256}}, ArgOptions{})
		},
		"overlapping ranges": func() (ArgDef, error) {
			return NewUintArg("count", 8, []UintRange{{0, 20}, {20, 255}}, ArgOptions{})
		},
		"descending ranges": func() (ArgDef, error) {
			return NewUintArg("count", 8, []UintRange{{20, 255}, {0, 10}}, ArgOptions{})
		},
		"range part descends": func() (ArgDef, error) {
			return NewUintArg("count", 8, []UintRange{{10, 0}, {20, 255}}, ArgOptions{})
		},
		"overlapping lengths": func() (ArgDef, error) {
			return NewStringArg("label", []UintRange{{1, 16}, {16, 32}}, []*regexp.Regexp{word}, ArgOptions{})
		},
		"descending lengths": func() (ArgDef, error) {
			return NewStringArg("label", []UintRange{{16, 32}, {1, 8}}, []*regexp.Regexp{word}, ArgOptions{})
		},
		"nil pattern": func() (ArgDef, error) {
			return NewStringArg("label", []UintRange{{1, 8}, {16, 32}}, []*regexp.Regexp{nil}, ArgOptions{})
		},
		"empty enum":       func() (ArgDef, error) { return NewEnumArg("mode", nil, ArgOptions{}) },
		"empty enum value": func() (ArgDef, error) { return NewEnumArg("mode", []string{"on", ""}, ArgOptions{}) },
		"flag empty name":  func() (ArgDef, error) { return NewFlagArg("", ArgOptions{}) },
		"union with an invalid member": func() (ArgDef, error) {
			return NewUnionArg("target", []ArgDef{{}}, ArgOptions{})
		},
	}
	for name, build := range refused {
		t.Run(name, func(t *testing.T) {
			def, err := build()
			if !errors.Is(err, ErrArgDef) {
				t.Fatalf("err = %v, want ErrArgDef", err)
			}
			if def.constructed {
				t.Fatalf("a refused definition was returned as constructed: %+v", def)
			}
		})
	}
}

// TestArgDefAccessorsDoNotAlias proves a validated definition cannot be
// changed through the slices it was built from or the values it yields.
//
// Method: build every kind from caller-owned slices, then overwrite those
// slices and every collected accessor result, and read the payload again.
func TestArgDefAccessorsDoNotAlias(t *testing.T) {
	lengths := []UintRange{{1, 8}}
	patterns := []*regexp.Regexp{regexp.MustCompile(`^[a-z]+$`)}
	ranges := []UintRange{{1, 10}}
	values := []string{"on", "off"}

	str := mustArgDef(NewStringArg("label", lengths, patterns, ArgOptions{}))
	num := mustArgDef(NewUintArg("count", 8, ranges, ArgOptions{}))
	enum := mustArgDef(NewEnumArg("mode", values, ArgOptions{}))
	members := []ArgDef{enum, num}
	union := mustArgDef(NewUnionArg("target", members, ArgOptions{}))

	lengths[0] = UintRange{0, 255}
	patterns[0] = regexp.MustCompile(`.*`)
	ranges[0] = UintRange{0, 255}
	values[0] = "changed"
	members[0] = num

	collected := slices.Collect(enum.EnumValues())
	collected[0] = "changed-again"
	collectedRanges := slices.Collect(num.Ranges())
	collectedRanges[0] = UintRange{0, 255}

	if got := slices.Collect(str.Lengths()); !slices.Equal(got, []UintRange{{1, 8}}) {
		t.Errorf("lengths = %v, want [{1 8}]", got)
	}
	if got := slices.Collect(str.Patterns()); !slices.Equal(got, []string{`^[a-z]+$`}) {
		t.Errorf("patterns = %v", got)
	}
	if got := slices.Collect(num.Ranges()); !slices.Equal(got, []UintRange{{1, 10}}) {
		t.Errorf("ranges = %v, want [{1 10}]", got)
	}
	if got := slices.Collect(enum.EnumValues()); !slices.Equal(got, []string{"on", "off"}) {
		t.Errorf("enum values = %v, want [on off]", got)
	}
	got := slices.Collect(union.UnionDefs())
	if len(got) != 2 || got[0].Kind() != ArgEnum || got[1].Kind() != ArgUint {
		t.Errorf("union members changed: %+v", got)
	}
	if values := slices.Collect(union.EnumValues()); !slices.Equal(values, []string{"on", "off"}) {
		t.Errorf("union enum values = %v, want [on off]", values)
	}
	if err := ValidateArgString("on", &enum); err != nil {
		t.Errorf("enum no longer accepts its own value: %v", err)
	}
}

// TestUsageTokenDoesNotAliasTheDefinition proves a rendered usage token
// cannot change the enumeration it was rendered from.
//
// Method: render an enum definition, overwrite the token's values, then ask
// the definition to validate its original value.
func TestUsageTokenDoesNotAliasTheDefinition(t *testing.T) {
	enum := mustArgDef(NewEnumArg("mode", []string{"on", "off"}, ArgOptions{}))
	token := usageToken(&enum, UsageValue)
	if len(token.Values) != 2 {
		t.Fatalf("token values = %v, want [on off]", token.Values)
	}
	token.Values[0] = "changed"
	if got := slices.Collect(enum.EnumValues()); !slices.Equal(got, []string{"on", "off"}) {
		t.Errorf("enum values = %v, want [on off]", got)
	}
	if err := ValidateArgString("on", &enum); err != nil {
		t.Errorf("enum no longer accepts its own value: %v", err)
	}
}

// TestArgDefWithAnchorCopies proves the anchor setter leaves its receiver
// unchanged, which is what lets appendAnchored anchor a shared definition.
func TestArgDefWithAnchorCopies(t *testing.T) {
	def := mustArgDef(NewStringArg("name", nil, nil, ArgOptions{Anchor: "peer"}))
	anchored := def.WithAnchor("interface")
	if def.Anchor() != "peer" || anchored.Anchor() != "interface" {
		t.Fatalf("anchors = %q, %q; want peer, interface", def.Anchor(), anchored.Anchor())
	}
}
