// VALIDATES: the le personality refuses to answer when the running binary is
// not the build `./le --name <name>` asked the launcher to produce.
// PREVENTS: a stale bin/le, a hardcoded binary path or a shadowed PATH entry
// answering a named session with code that is no longer in the tree.
package main

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestInvokedBuildNameReadsTheDirectoryForANamedBuild(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "named build", path: "/repo/bin/le-interop/le", want: "interop"},
		{name: "named build with punctuation", path: "/repo/bin/le-a.b_c-1/le", want: "a.b_c-1"},
		{name: "shared binary", path: "/repo/bin/le", want: "le"},
		{name: "shared binary reached by a bare name", path: "le", want: "le"},
		{name: "product binary", path: "/repo/bin/ze", want: "ze"},
		{name: "a file named le- something", path: "/repo/bin/le-interop/le-old", want: "le-old"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := invokedBuildName(tt.path); got != tt.want {
				t.Errorf("invokedBuildName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRefuseWrongBuildNameGuardsANamedSession(t *testing.T) {
	tests := []struct {
		name     string
		asked    string
		argv0    string
		wantCode int
		wantText []string
	}{
		{name: "no name asked for", asked: "", argv0: "/repo/bin/le", wantCode: 0},
		{name: "the named build answers", asked: "interop", argv0: "/repo/bin/le-interop/le", wantCode: 0},
		{
			name:     "the shared binary is refused",
			asked:    "interop",
			argv0:    "/repo/bin/le",
			wantCode: 2,
			wantText: []string{"interop", "'le'", "/repo/bin/le"},
		},
		{
			name:     "another session's build is refused",
			asked:    "interop",
			argv0:    "/repo/bin/le-weekly/le",
			wantCode: 2,
			wantText: []string{"interop", "weekly"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ZE_LE_BUILD_NAME", tt.asked)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			saved := slices.Clone(os.Args)
			os.Args = []string{tt.argv0}
			t.Cleanup(func() { os.Args = saved })

			var code int
			message := captureBuildNameStderr(t, func() { code = refuseWrongBuildName() })
			if code != tt.wantCode {
				t.Fatalf("refuseWrongBuildName() = %d, want %d (stderr %q)", code, tt.wantCode, message)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(message, want) {
					t.Errorf("refusal %q does not name %q", message, want)
				}
			}
			if tt.wantCode == 0 && message != "" {
				t.Errorf("an accepted build wrote %q", message)
			}
		})
	}
}

// captureBuildNameStderr collects what run writes to os.Stderr. It carries its
// own name because the package's other helper of this shape is ze_core-only,
// and the le personality does not set that tag.
func captureBuildNameStderr(t *testing.T, run func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("open pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write

	collected := make(chan string, 1)
	go func() {
		body, _ := io.ReadAll(read) //nolint:errcheck // a closed pipe ends the read
		collected <- string(body)
	}()

	run()
	os.Stderr = saved
	if err := write.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	message := <-collected
	if err := read.Close(); err != nil {
		t.Fatalf("close pipe reader: %v", err)
	}
	return message
}
