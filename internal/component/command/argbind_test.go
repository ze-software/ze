package command

import "testing"

// socketFilterDefs are the three optional filters `show system sockets`
// declares. They are the case R-1 of spec-generated-command-usage names:
// `state` is a pattern-less string, so it accepts a port number, and only the
// alphabet kept 8080 away from it.
func socketFilterDefs() []ArgDef {
	return []ArgDef{
		{Name: "protocol", Kind: ArgEnum, EnumValues: []string{"tcp", "udp"}},
		{Name: "state", Kind: ArgString},
		{Name: "port", Kind: ArgUint, UintBits: 32},
	}
}

// permutations returns every ordering of defs, so a binding test can prove that
// no slice order changes its answer. Three definitions give six orderings, and
// the recursion is bounded by len(defs).
func permutations(defs []ArgDef) [][]ArgDef {
	if len(defs) <= 1 {
		return [][]ArgDef{append([]ArgDef(nil), defs...)}
	}
	var out [][]ArgDef
	for i := range defs {
		rest := make([]ArgDef, 0, len(defs)-1)
		rest = append(rest, defs[:i]...)
		rest = append(rest, defs[i+1:]...)
		for _, tail := range permutations(rest) {
			out = append(out, append([]ArgDef{defs[i]}, tail...))
		}
	}
	return out
}

// VALIDATES: a positional token goes to the definition that constrains it most,
// not to the first definition in the slice that happens to accept it.
// PREVENTS: a pattern-less string swallowing a value an enumeration or an
// integer leaf names exactly, which is a wrong answer rather than an error.
func TestPositionalDefPrefersConstrainedDef(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want string
	}{
		{name: "an enumerated word goes to the enumeration", arg: "tcp", want: "protocol"},
		{name: "an integer goes to the integer leaf", arg: "8080", want: "port"},
		{name: "a word no other type admits goes to the string", arg: "ESTABLISHED", want: "state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, defs := range permutations(socketFilterDefs()) {
				def := positionalDef(tc.arg, defs, map[string]bool{})
				if def == nil {
					t.Fatalf("ordering %d bound %q to nothing", i, tc.arg)
				}
				if def.Name != tc.want {
					t.Errorf("ordering %d bound %q to %q, want %q", i, tc.arg, def.Name, tc.want)
				}
			}
		})
	}
}

// VALIDATES: the answer `show system sockets 8080` and `show system sockets
// ESTABLISHED` produce is the same for every ordering of the filter
// definitions, read at the entry point rather than at the helper.
// PREVENTS: R-1 of spec-generated-command-usage -- changing the
// definition order to the declared one moving a bare port onto the state
// filter, which the daemon then acts on without an error.
func TestPositionalBindingIsOrderIndependent(t *testing.T) {
	for _, tc := range []struct {
		arg  string
		want string
	}{
		{arg: "8080", want: "port"},
		{arg: "ESTABLISHED", want: "state"},
		{arg: "udp", want: "protocol"},
	} {
		t.Run(tc.arg, func(t *testing.T) {
			for i, defs := range permutations(socketFilterDefs()) {
				lone, err := ValidateArgs([]string{tc.arg}, defs, nil)
				if err != nil {
					t.Fatalf("ordering %d refused %q: %v", i, tc.arg, err)
				}
				if len(lone) != 1 {
					t.Fatalf("ordering %d bound %q to %d leaves: %v", i, tc.arg, len(lone), lone)
				}
				if lone[tc.want] != tc.arg {
					t.Errorf("ordering %d bound %q to %v, want the %s leaf", i, tc.arg, lone, tc.want)
				}
			}
		})
	}
}

// VALIDATES: a required leaf is still offered a token before an optional one,
// whatever their constraint strengths say.
// PREVENTS: the constraint ranking undoing the mandatory tier, which would
// turn a complete command into "required argument missing: port".
func TestPositionalDefKeepsTheMandatoryTierFirst(t *testing.T) {
	defs := []ArgDef{
		{Name: "state", Kind: ArgEnum, EnumValues: []string{"up", "down"}},
		{Name: "mode", Kind: ArgString, Mandatory: true},
	}
	def := positionalDef("up", defs, map[string]bool{})
	if def == nil || def.Name != "mode" {
		t.Fatalf("a required leaf was not offered the token first: %v", def)
	}
}
