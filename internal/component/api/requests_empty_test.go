// Design: docs/architecture/api/architecture.md -- lossless parameter adapters.
package api

import (
	"strings"
	"testing"
)

// TestBuildInvocationPreservesExplicitEmptyContent covers the builder both
// REST and gRPC execute/stream adapters use (buildInvocation, through
// prepareCommand), including the omitted-input control.
func TestBuildInvocationPreservesExplicitEmptyContent(t *testing.T) {
	const input = "show data encode"
	for _, test := range []struct {
		params map[string]string
		want   string
	}{
		{want: "show data encode"},
		{params: map[string]string{"content": ""}, want: `show data encode content ""`},
		{params: map[string]string{"content": "IA=="}, want: "show data encode content IA=="},
	} {
		got, err := buildInvocation(input, test.params, strings.Fields(input), -1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("command = %q, want %q", got, test.want)
		}
	}
}
