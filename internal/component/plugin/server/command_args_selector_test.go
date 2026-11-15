// Design: docs/architecture/api/commands.md -- how a handler reads the value its leaf declares
// Related: command.go -- CommandContext.ArgsOrSelector, the producer under test

package server

import (
	"slices"
	"testing"
)

// VALIDATES: ArgsOrSelector hands a forwarder the value the operator typed in
// each form that reaches it: bound as a typed selector with no argument (the
// leaf shares its name with a key token of the command, matchCommandTokens),
// as a plain argument, which wins over a selector, and as the `<leaf> <value>`
// pair the web admin form posts, whose keyword is dropped. (Moved from the
// firewall-irr forwarders' TestArgsOrSelectorRecoversTheBoundValue: the
// function is this package's, and every forwarder reads it.)
// METHOD: call it with the literal leaf names the IRR commands declare, "asn"
// and "as-set", over each form.
// PREVENTS: a forwarder discarding a value the dispatcher already validated,
// so the plugin answers `update irr asn 65001` with its usage line.
func TestArgsOrSelectorRecoversTheBoundValue(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		selectors map[string]string
		leaf      string
		want      []string
	}{
		{"asn from selector", nil, map[string]string{"asn": "65001"}, "asn", []string{"65001"}},
		{"as-set from selector", nil, map[string]string{"as-set": "AS-TEST"}, "as-set", []string{"AS-TEST"}},
		{"args win when present", []string{"64496"}, map[string]string{"asn": "65001"}, "asn", []string{"64496"}},
		{"leaf keyword stripped", []string{"asn", "64496"}, nil, "asn", []string{"64496"}},
		{"leaf keyword stripped in any case", []string{"AS-SET", "AS-TEST"}, nil, "as-set", []string{"AS-TEST"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &CommandContext{Selectors: tt.selectors}
			if got := ctx.ArgsOrSelector(tt.args, tt.leaf); !slices.Equal(got, tt.want) {
				t.Fatalf("ArgsOrSelector(%v, %q) = %v, want %v", tt.args, tt.leaf, got, tt.want)
			}
		})
	}
}

// VALIDATES: a command typed with no value stays empty, so the plugin still
// answers with its usage line instead of acting on a stale selector, and a
// lone token spelled like the leaf is a value, never a stripped keyword.
// METHOD: call ArgsOrSelector with no argument and no selector, on a nil
// context, and with the leaf name as the only argument.
// PREVENTS: a value invented where the operator typed none.
func TestArgsOrSelectorKeepsAMissingValueMissing(t *testing.T) {
	ctx := &CommandContext{}
	if got := ctx.ArgsOrSelector(nil, "asn"); len(got) != 0 {
		t.Fatalf("ArgsOrSelector = %v, want no arguments", got)
	}
	var none *CommandContext
	if got := none.ArgsOrSelector(nil, "as-set"); len(got) != 0 {
		t.Fatalf("nil context ArgsOrSelector = %v, want no arguments", got)
	}
	if got := ctx.ArgsOrSelector([]string{"asn"}, "asn"); !slices.Equal(got, []string{"asn"}) {
		t.Fatalf("ArgsOrSelector([asn]) = %v, want the lone token kept as the value", got)
	}
}
