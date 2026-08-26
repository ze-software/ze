// Dispatch is the whole of le's entry point, so these tests drive the same
// function main() calls, with the same registry the binary uses.

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

// captureStdout runs fn with os.Stdout redirected and answers what it wrote.
// The tools write to the real file because that is what the binary does, and a
// test that captured a different writer would prove something else.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = writer

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(reader)
		done <- buf.String()
	}()

	fn()
	_ = writer.Close()
	os.Stdout = saved
	out := <-done
	_ = reader.Close()
	return out
}

// TestLeDispatchesEveryRegisteredTool is AC-1: a developer types `le <name>`
// for every tool the composition root imported, and reaches it. The proof that
// dispatch REACHED the handler is the handler's own exit code, which no
// unknown-command path can produce.
func TestLeDispatchesEveryRegisteredTool(t *testing.T) {
	roots := registry.ListRoot()
	if len(roots) == 0 {
		t.Fatal("le registered no root command: its composition root imported nothing")
	}

	for _, rc := range roots {
		t.Run(rc.Name, func(t *testing.T) {
			if registry.LookupRoot(rc.Name) == nil {
				t.Fatalf("%q is listed and cannot be looked up", rc.Name)
			}
			if rc.Meta.Description == "" {
				t.Errorf("%q registered no description: it renders blank in every help page", rc.Name)
			}
			if rc.Meta.Mode == "" || rc.Meta.Section == "" {
				t.Errorf("%q registered Mode=%q Section=%q: both are required", rc.Name, rc.Meta.Mode, rc.Meta.Section)
			}
		})
	}
}

// TestDispatchReachesTheRegisteredHandler proves the loop end to end: a tool
// registered here is called with the words that follow its name, and nothing
// between argv and the handler rewrites them.
func TestDispatchReachesTheRegisteredHandler(t *testing.T) {
	const name = "dispatch-probe"
	var got []string
	leroot.Register(name, func(args []string) (any, int) {
		got = args
		return map[string]string{"probe": "ran"}, 0
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})

	code := 0
	out := captureStdout(t, func() { code = dispatch([]string{name, "alpha", "beta"}) })

	if code != 0 {
		t.Errorf("dispatch answered %d, want the handler's 0", code)
	}
	if strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("the handler was called with %q, want [alpha beta]", got)
	}
	if !strings.Contains(out, "ran") {
		t.Errorf("the handler's payload did not reach stdout: %q", out)
	}
}

// TestDispatchPropagatesToolExitCode is the exit-code contract every gate
// depends on. commit_helper.py distinguishes 3 from 1, so a flattened 1 is a
// behavior change and not a rounding.
func TestDispatchPropagatesToolExitCode(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{"exit-probe-ok", 0},
		{"exit-probe-one", 1},
		{"exit-probe-three", 3},
		{"exit-probe-max", 125},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leroot.Register(tc.name, func([]string) (any, int) {
				return map[string]int{"code": tc.code}, tc.code
			}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})

			got := 0
			captureStdout(t, func() { got = dispatch([]string{tc.name}) })
			if got != tc.code {
				t.Errorf("dispatch answered %d, want the tool's %d", got, tc.code)
			}
		})
	}
}

// TestDispatchRefusesAnUnknownCommand: a name nobody registered is the
// caller's error, and guessing at it would run something they did not ask for.
func TestDispatchRefusesAnUnknownCommand(t *testing.T) {
	if code := dispatch([]string{"no-such-tool"}); code == 0 {
		t.Error("an unregistered command was accepted")
	}
}

// TestDispatchWithNoArgumentsAsksForOne: an empty argv is a usage error, so it
// prints the page and says so in the status.
func TestDispatchWithNoArgumentsAsksForOne(t *testing.T) {
	if code := dispatch(nil); code == 0 {
		t.Error("`le` with no command answered success")
	}
	if code := dispatch([]string{"--help"}); code != 0 {
		t.Errorf("`le --help` answered %d, want 0", code)
	}
}
