package fibkernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: preservesForwardingState, the answer this plugin gives the BGP
// engine for RFC 4724's Forwarding State bit.
// PREVENTS: Ze telling a peer its forwarding state survived a restart that
// flushed the routes, and the opposite, Ze denying a restart that kept them.
// RFC 4724 Section 4.1 allows the bit only where the state "has indeed been
// preserved", and RFC 4724 Section 4.2 has the peer drop every retained route
// of a family whose bit is clear.

func TestPreservesForwardingState(t *testing.T) {
	tests := []struct {
		name string
		tree map[string]any
		want bool
	}{
		{
			name: "no fib block at all",
			tree: map[string]any{},
			want: false,
		},
		{
			name: "configured, flush-on-stop absent",
			tree: fibTree(map[string]any{}),
			want: true,
		},
		{
			name: "configured, flush-on-stop false",
			tree: fibTree(map[string]any{"flush-on-stop": "false"}),
			want: true,
		},
		{
			name: "configured, flush-on-stop true",
			tree: fibTree(map[string]any{"flush-on-stop": "true"}),
			want: false,
		},
		{
			// parseFIBConfig refuses this section, so the daemon never runs
			// with it. False is the answer that claims nothing.
			name: "configured, flush-on-stop unreadable",
			tree: fibTree(map[string]any{"flush-on-stop": "sometimes"}),
			want: false,
		},
		{
			// A leaf arrives as the string the operator wrote, but a test
			// that builds its own map can hold a Go bool and configvalue
			// accepts it.
			name: "configured, flush-on-stop as a bool",
			tree: fibTree(map[string]any{"flush-on-stop": true}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, preservesForwardingState(tt.tree))
		})
	}
}

// fibTree wraps a fib/kernel body in the path the config lowering delivers it
// under, which configRoot "fib/kernel" spells as {"fib":{"kernel":{...}}}.
func fibTree(body map[string]any) map[string]any {
	return map[string]any{"fib": map[string]any{"kernel": body}}
}
