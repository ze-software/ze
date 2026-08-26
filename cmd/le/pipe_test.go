// le inherits the command contract rather than reimplementing it, and the pipe
// operators are the sharpest test of that: a tool answers ONE payload, and
// `| json`, `| yaml` and `| table` are three renderings of it. No tool here
// writes a line of JSON, YAML or table code.

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

// pipeProbe answers a payload with a nested array and a scalar, which is the
// shape every renderer has to handle.
func pipeProbe([]string) (any, int) {
	return map[string]any{
		"gates":          2,
		"unported-gates": []string{"ze-tier-check", "ze-repository-check"},
	}, 0
}

// TestLeCommandAnswersStructuredData is AC-7. It drives the real dispatch
// path, so what it proves is what a developer gets from the binary.
func TestLeCommandAnswersStructuredData(t *testing.T) {
	const name = "pipe-probe"
	leroot.Register(name, pipeProbe, registry.Meta{
		Description: "a test probe", Mode: "offline", Section: registry.SectionTest,
	})

	t.Run("json", func(t *testing.T) {
		out := captureStdout(t, func() { dispatch([]string{name, "|", "json"}) })
		var decoded map[string]any
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("`le %s | json` did not answer JSON: %v\n%s", name, err, out)
		}
		if decoded["gates"] != float64(2) {
			t.Errorf("the payload's scalar did not survive the rendering: %v", decoded["gates"])
		}
		gates, ok := decoded["unported-gates"].([]any)
		if !ok || len(gates) != 2 {
			t.Errorf("the payload's array did not survive the rendering: %v", decoded["unported-gates"])
		}
	})

	t.Run("yaml", func(t *testing.T) {
		out := captureStdout(t, func() { dispatch([]string{name, "|", "yaml"}) })
		if !strings.Contains(out, "gates: 2") {
			t.Errorf("`le %s | yaml` did not render the payload: %q", name, out)
		}
		if !strings.Contains(out, "ze-tier-check") {
			t.Errorf("`le %s | yaml` dropped the array: %q", name, out)
		}
	})

	t.Run("table", func(t *testing.T) {
		out := captureStdout(t, func() { dispatch([]string{name, "|", "table"}) })
		if !strings.Contains(out, "gates") || !strings.Contains(out, "ze-tier-check") {
			t.Errorf("`le %s | table` did not render the payload: %q", name, out)
		}
	})

	t.Run("match", func(t *testing.T) {
		out := captureStdout(t, func() { dispatch([]string{name, "|", "match", "tier"}) })
		if !strings.Contains(out, "ze-tier-check") {
			t.Errorf("`le %s | match tier` dropped the matching row: %q", name, out)
		}
	})
}

// TestLeRefusesTwoFormatOperators proves le inherited the contract's REFUSALS
// too, not only its renderings. A chain naming two formats is ambiguous, and
// the engine says so.
func TestLeRefusesTwoFormatOperators(t *testing.T) {
	const name = "pipe-refusal-probe"
	leroot.Register(name, pipeProbe, registry.Meta{
		Description: "a test probe", Mode: "offline", Section: registry.SectionTest,
	})

	code := 0
	out := captureStdout(t, func() { code = dispatch([]string{name, "|", "json", "|", "yaml"}) })
	if code == 0 {
		t.Error("a chain naming two format operators was accepted")
	}
	if out != "" {
		t.Errorf("a refused chain wrote to stdout: %q", out)
	}
}
