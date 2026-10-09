package command

import (
	"errors"
	"slices"
	"testing"
)

// socketFilterDefs are the three optional filters `show system sockets`
// declares. They are the case R-1 of spec-generated-command-usage names:
// `state` is a pattern-less string, so it accepts a port number, and only the
// alphabet kept 8080 away from it.
func socketFilterDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewEnumArg("protocol", []string{"tcp", "udp"}, ArgOptions{})),
		mustArgDef(NewStringArg("state", nil, nil, ArgOptions{})),
		mustArgDef(NewUintArg("port", 32, nil, ArgOptions{})),
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
				if def.name != tc.want {
					t.Errorf("ordering %d bound %q to %q, want %q", i, tc.arg, def.name, tc.want)
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
				validated, err := ValidateArgs([]string{tc.arg}, defs, nil)
				if err != nil {
					t.Fatalf("ordering %d refused %q: %v", i, tc.arg, err)
				}
				if value, found := validated.Positional(tc.want); !found || value != tc.arg {
					t.Errorf("ordering %d bound %q to leaf %q, want the %s leaf", i, tc.arg, validated.bound, tc.want)
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
		mustArgDef(NewEnumArg("state", []string{"up", "down"}, ArgOptions{})),
		mustArgDef(NewStringArg("mode", nil, nil, ArgOptions{Mandatory: true})),
	}
	def := positionalDef("up", defs, map[string]bool{})
	if def == nil || def.name != "mode" {
		t.Fatalf("a required leaf was not offered the token first: %v", def)
	}
}

// VALIDATES: AC-23 and A-7. A path whose model declares no argument definition
// passes every token through ValidateArgs unchanged, in order, binds no leaf,
// and refuses nothing, so a route that now validates such a path changes no
// answer.
// PREVENTS: the validate-on-every-route change (D-3) refusing a command that
// worked because its leaves are not modeled.
// METHOD: arbitrary tokens, flag-shaped and keyword-shaped included, against
// nil and an empty definition list; then the ownership contract (AC-6): a write
// to the input after the call, and to what Tokens answers, changes nothing.
func TestValidateArgsEmptyDefinitionsPassTokens(t *testing.T) {
	for _, defs := range [][]ArgDef{nil, {}} {
		args := []string{"anything", "--flag", "name", "x y", ""}
		validated, err := ValidateArgs(args, defs, nil)
		if err != nil {
			t.Fatalf("defs %v refused %q: %v", defs, args, err)
		}
		want := slices.Clone(args)
		if got := validated.Tokens(); !slices.Equal(got, want) {
			t.Fatalf("Tokens() = %q, want %q", got, want)
		}
		if _, found := validated.Positional("name"); found {
			t.Errorf("no definition declared, yet a leaf was bound")
		}
		args[0] = "changed"
		validated.Tokens()[1] = "changed"
		if got := validated.Tokens(); !slices.Equal(got, want) {
			t.Errorf("a write to the input or to Tokens() reached the value: %q", got)
		}
	}
	if got := (ValidatedArgs{}).Tokens(); len(got) != 0 {
		t.Errorf("zero ValidatedArgs carries tokens %q", got)
	}
}

// VALIDATES: a refusal for a missing mandatory leaf is a *MissingArgumentError
// with the operator-facing text, and it reports the lone binding the call made.
// PREVENTS: the dispatcher losing the positional selector it adopts before it
// reports the missing leaf (plugin/server Dispatch).
func TestValidateArgsMissingMandatoryReportsBinding(t *testing.T) {
	defs := []ArgDef{
		mustArgDef(NewEnumArg("selector", []string{"peer-a"}, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("label", nil, nil, ArgOptions{Mandatory: true})),
	}
	validated, err := ValidateArgs([]string{"peer-a"}, defs, nil)
	var missing *MissingArgumentError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want *MissingArgumentError", err)
	}
	if err.Error() != "required argument missing: label" {
		t.Errorf("text = %q", err.Error())
	}
	if value, found := missing.Positional("selector"); !found || value != "peer-a" {
		t.Errorf("binding = %q %v, want peer-a", value, found)
	}
	if len(validated.Tokens()) != 0 {
		t.Errorf("a refused call answered tokens %q", validated.Tokens())
	}
}
