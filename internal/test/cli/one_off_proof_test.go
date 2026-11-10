// Design: docs/architecture/testing/ci-format.md -- engine-step executor plugin and its step kinds.
// Related: cmd_engine_steps.go -- CmdEngineSteps, the `le mock engine-steps` entry the file and stdin routes reach.

package cli

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestEngineStepUnknownKindRefused proves an engine step of a kind no runner
// arm handles is refused on every route a step list takes, and a named kind
// still runs.
//
// VALIDATES: P-1 (engine steps) of spec-tooling-one-off-defects. One subtest
// per route, each discriminating on its own:
//   - file: CmdEngineSteps reads the JSON file the runner materializes. An
//     unknown kind fails parsing, naming the kind and the file, and never
//     reaches the TLS dial; a valid list passes parsing and reaches the dial,
//     which the fixture leaves unconfigured.
//   - stdin: the same entry with `-`, the list on standard input.
//   - direct: runner.RunEngineSteps over a decoded list a Go caller built. An
//     unknown kind fails, naming its position and kind, after the valid step
//     before it ran and without dispatching its own text; a valid step
//     dispatches.
//
// PREVENTS: a step the executor skips while the test reads as passed.
//
// MUTATION: (internal/test/runner/engine_steps.go, UnmarshalEngineSteps, `return nil, fmt.Errorf("engine steps file: step %d: unknown kind %d", i+1, steps[i].Kind)`, the line deleted): the file and stdin subtests go red, because the list passes parsing and fails later at the dial.
// MUTATION: (internal/test/runner/engine_steps.go, RunEngineSteps, `return fmt.Errorf("engine step %d: unknown kind %d", i+1, step.Kind)`, the line deleted): the direct subtest goes red, because the unknown step is skipped and the run answers nil.
func TestEngineStepUnknownKindRefused(t *testing.T) {
	unknown := []struct {
		name string
		data string
		kind string
	}{
		{"missing", `[{}]`, "unknown kind 0"},
		{"zero", `[{"kind":0}]`, "unknown kind 0"},
		{"unnamed", `[{"kind":255}]`, "unknown kind 255"},
		{"after a valid step", `[{"kind":1,"text":"show version"},{"kind":255}]`, "step 2: unknown kind 255"},
	}
	const valid = `[{"kind":1,"text":"show version"}]`

	for _, carrier := range []string{"file", "stdin"} {
		t.Run(carrier, func(t *testing.T) {
			isolateEngineStepsDial(t)
			for _, one := range unknown {
				output, path := runEngineStepsEntry(t, carrier, one.data)
				if !strings.Contains(output, one.kind) {
					t.Fatalf("%s: output %q does not name %q", one.name, output, one.kind)
				}
				if !strings.Contains(output, "parse steps file") {
					t.Fatalf("%s: the unknown kind did not fail parsing: %s", one.name, output)
				}
				if !strings.Contains(output, "path="+path) {
					t.Fatalf("%s: the refusal does not name the input %s: %s", one.name, path, output)
				}
				if strings.Contains(output, "TLS connect-back") {
					t.Fatalf("%s: the unknown kind reached the dial: %s", one.name, output)
				}
			}
			output, _ := runEngineStepsEntry(t, carrier, valid)
			if strings.Contains(output, "parse steps file") {
				t.Fatalf("a valid list failed parsing: %s", output)
			}
			if !strings.Contains(output, "TLS connect-back failed") {
				t.Fatalf("a valid list did not reach the dial: %s", output)
			}
		})
	}

	t.Run("direct", func(t *testing.T) {
		for _, one := range unknown {
			steps := directEngineSteps(t, one.data)
			var dispatched []string
			dispatch := func(_ context.Context, command string) (string, string, error) {
				dispatched = append(dispatched, command)
				return "done", "", nil
			}
			err := runner.RunEngineSteps(t.Context(), dispatch, runner.NewEngineEventBuffer(), steps)
			if err == nil {
				t.Fatalf("%s: an unknown kind ran to success", one.name)
			}
			if !strings.Contains(err.Error(), one.kind) {
				t.Fatalf("%s: error %q does not name %q", one.name, err, one.kind)
			}
			var want []string
			if len(steps) > 1 {
				want = []string{"show version"}
			}
			if !slices.Equal(dispatched, want) {
				t.Fatalf("%s: dispatched %q, want %q", one.name, dispatched, want)
			}
		}

		var dispatched []string
		dispatch := func(_ context.Context, command string) (string, string, error) {
			dispatched = append(dispatched, command)
			return "done", "", nil
		}
		steps := []runner.EngineStep{{Kind: runner.EngineStepCommand, Text: "show version"}}
		if err := runner.RunEngineSteps(t.Context(), dispatch, runner.NewEngineEventBuffer(), steps); err != nil {
			t.Fatalf("a valid step failed: %v", err)
		}
		if !slices.Equal(dispatched, []string{"show version"}) {
			t.Fatalf("a valid step dispatched %q, want [show version]", dispatched)
		}
	})
}

// directEngineSteps decodes data as a Go caller would hold it, without the
// file decoder's refusal: RunEngineSteps is the only judge on this route.
func directEngineSteps(t *testing.T, data string) []runner.EngineStep {
	t.Helper()
	switch data {
	case `[{}]`, `[{"kind":0}]`:
		return []runner.EngineStep{{}}
	case `[{"kind":255}]`:
		return []runner.EngineStep{{Kind: 255}}
	case `[{"kind":1,"text":"show version"},{"kind":255}]`:
		return []runner.EngineStep{{Kind: runner.EngineStepCommand, Text: "show version"}, {Kind: 255}}
	}
	t.Fatalf("no direct form for %s", data)
	return nil
}

// isolateEngineStepsDial clears the hub token so the dial a valid list reaches
// fails at once, and restores the default logger afterwards.
func isolateEngineStepsDial(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.ReplaceAll(strings.ToLower(key), "_", ".") == "ze.plugin.hub.token" {
			t.Setenv(key, "")
		}
	}
	t.Setenv("ZE_PLUGIN_HUB_TOKEN", "")
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })
}

// runEngineStepsEntry runs CmdEngineSteps with data on the named carrier and
// answers its log output and the path argument it was given. The entry always
// exits 1 here: an unknown kind refuses, and a valid list fails at the dial.
func runEngineStepsEntry(t *testing.T, carrier, data string) (output, path string) {
	t.Helper()
	var log bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	path = "-"
	if carrier == "file" {
		path = filepath.Join(t.TempDir(), "steps.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		restore := cliio.SwapStreams(strings.NewReader(data), &log)
		defer restore()
	}
	if code := CmdEngineSteps([]string{path}); code != 1 {
		t.Fatalf("exit = %d, want 1: %s", code, log.String())
	}
	return log.String(), path
}
