// VALIDATES: spec-le-is-a-ze-binary AC-11. The native chaos action table keeps
// the Python registry's gate names, argv, environment, order, write flags, and prose.
// PREVENTS: a port that looks plausible but changes the process boundary that
// the Make targets already expose.

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/testchaos"
)

type chaosParityGate struct {
	Name   string            `json:"name"`
	Argv   []string          `json:"argv"`
	Why    string            `json:"why"`
	Writes bool              `json:"writes"`
	Env    map[string]string `json:"env"`
}

type chaosParitySnapshot struct {
	Area  string            `json:"area"`
	Gates []chaosParityGate `json:"gates"`
}

const chaosParityPython = `
import json
import os
import sys

sys.path.insert(0, os.path.join(os.getcwd(), "scripts"))
from le.application.chaos import GATES, _environment

keys = (
    "GOCACHE",
    "GOLANGCI_LINT_CACHE",
    "CGO_ENABLED",
    "GOTOOLCHAIN",
    "GOMAXPROCS",
    "GOMEMLIMIT",
)
rows = []
for gate in GATES.gates:
    environment = _environment(gate)
    rows.append({
        "name": gate.name,
        "argv": list(gate.argv),
        "why": gate.why,
        "writes": gate.writes,
        "env": {key: environment[key] for key in keys if key in environment},
    })
print(json.dumps({"area": GATES.area, "gates": rows}))
`

// TestChaosNativeTableMatchesPythonRegistry compares the two implementations at
// their shared boundary. Fixed host knobs keep the result independent of RAM.
func TestChaosNativeTableMatchesPythonRegistry(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	t.Setenv("GO_TEST_TIMEOUT", "7m")
	t.Setenv("ZE_LINT_MEMLIMIT", "9GiB")
	t.Setenv("ZE_TAGS", "ze_parity_extra")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	python := chaosParityPythonSnapshot(t, root)
	chain, err := gotoolchain.New(root)
	if err != nil {
		t.Fatalf("derive native toolchain: %v", err)
	}
	if python.Area != testchaos.Area {
		t.Errorf("native area = %q, Python area = %q", testchaos.Area, python.Area)
	}
	gates := testchaos.Table()
	if len(gates) != len(python.Gates) {
		t.Fatalf("native registry has %d gates, Python has %d", len(gates), len(python.Gates))
	}

	for index, gate := range gates {
		got := chaosParityGate{
			Name:   gate.Name,
			Argv:   gate.Argv(chain),
			Why:    gate.Why,
			Writes: gate.Writes,
			Env:    chaosParityOverrideMap(t, gate.Overrides(chain)),
		}
		want := python.Gates[index]
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal native gate %q: %v", gate.Name, err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal Python gate %q: %v", want.Name, err)
		}
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Errorf("gate %d differs\nnative: %s\npython: %s", index, gotJSON, wantJSON)
		}
	}
}

func chaosParityPythonSnapshot(t *testing.T, root string) chaosParitySnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", chaosParityPython)
	cmd.Dir = root
	cmd.Env = chaosParityCleanEnvironment(os.Environ())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read Python chaos registry: %v\n%s", err, output)
	}
	var snapshot chaosParitySnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		t.Fatalf("decode Python chaos registry: %v\n%s", err, output)
	}
	return snapshot
}

func chaosParityOverrideMap(t *testing.T, overrides []string) map[string]string {
	t.Helper()
	wanted := map[string]bool{
		"GOCACHE":             true,
		"GOLANGCI_LINT_CACHE": true,
		"CGO_ENABLED":         true,
		"GOTOOLCHAIN":         true,
		"GOMAXPROCS":          true,
		"GOMEMLIMIT":          true,
	}
	values := make(map[string]string, len(wanted))
	for _, override := range overrides {
		key, value, ok := strings.Cut(override, "=")
		if !ok {
			t.Fatalf("toolchain override %q has no equals sign", override)
		}
		if wanted[key] {
			values[key] = value
		}
	}
	if len(values) == 0 {
		t.Fatal("native gate declared no toolchain environment")
	}
	return values
}

func chaosParityCleanEnvironment(environ []string) []string {
	keys := map[string]bool{
		"GOCACHE":             true,
		"GOLANGCI_LINT_CACHE": true,
		"CGO_ENABLED":         true,
		"GOTOOLCHAIN":         true,
		"GOMAXPROCS":          true,
		"GOMEMLIMIT":          true,
	}
	clean := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if keys[key] {
				continue
			}
		}
		clean = append(clean, entry)
	}
	return clean
}
