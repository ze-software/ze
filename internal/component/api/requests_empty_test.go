// Design: docs/architecture/api/architecture.md -- lossless parameter adapters.
package api

import "testing"

// TestBuildCommandPreservesExplicitEmptyContent covers the builder both REST
// and gRPC execute/stream adapters use, including the omitted-input control.
func TestBuildCommandPreservesExplicitEmptyContent(t *testing.T) {
	for _, test := range []struct {
		params map[string]string
		want   string
	}{
		{want: "show data encode"},
		{params: map[string]string{"content": ""}, want: `show data encode content ""`},
		{params: map[string]string{"content": "IA=="}, want: "show data encode content IA=="},
	} {
		got, err := BuildCommand("show data encode", test.params)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("command = %q, want %q", got, test.want)
		}
	}
}
