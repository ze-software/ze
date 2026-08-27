// Design: docs/architecture/core-design.md -- duplicate-then-swap gate contract proof
// The duplicate-then-swap proof for scripts/le/application/unit.py and
// internal/le/testunit. This migration-only test leaves with the Python producer.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/testunit"
)

const unitParityTimeout = 120 * time.Second

var unitParityEnvironmentKeys = [...]string{
	"GOCACHE",
	"GOLANGCI_LINT_CACHE",
	"CGO_ENABLED",
	"GOTOOLCHAIN",
	"GOMAXPROCS",
}

type unitParitySnapshot struct {
	Area  string           `json:"area"`
	Gates []unitParityGate `json:"gates"`
}

type unitParityGate struct {
	Name        string            `json:"name"`
	Argv        []string          `json:"argv"`
	Why         string            `json:"why"`
	Writes      bool              `json:"writes"`
	Environment map[string]string `json:"environment"`
}

// TestUnitGateTableMatchesThePythonProducer compares every observable command
// fact under defaults and overrides. A reduced tag set or lost race environment
// can otherwise produce a green run over a smaller product.
func TestUnitGateTableMatchesThePythonProducer(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}

	probes := []struct {
		name        string
		environment map[string]string
	}{
		{name: "ambient environment"},
		{
			name: "timeout and feature overrides",
			environment: map[string]string{
				"GO_TEST_TIMEOUT": "41s",
				"ZE_TAGS":         "ze_fixture_a ze_fixture_b",
			},
		},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			for key, value := range probe.environment {
				t.Setenv(key, value)
			}
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			want := runUnitParityPython(t, root)
			got := unitParityGoSnapshot(t, root)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unit gate contract differs:\nPython: %#v\nGo:     %#v", want, got)
			}
		})
	}
}

func runUnitParityPython(t *testing.T, root string) unitParitySnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), unitParityTimeout)
	defer cancel()

	fragment := `
import json
from le.application import unit
keys = ("GOCACHE", "GOLANGCI_LINT_CACHE", "CGO_ENABLED", "GOTOOLCHAIN", "GOMAXPROCS")
rows = []
for gate in unit.GATES.gates:
    environment = unit._environment(gate)
    rows.append({
        "name": gate.name,
        "argv": list(gate.argv),
        "why": gate.why,
        "writes": gate.writes,
        "environment": {key: environment[key] for key in keys if key in environment},
    })
print(json.dumps({"area": unit.GATES.area, "gates": rows}))
`
	//nolint:gosec // this migration test executes its fixed fragment against the checked-in producer
	cmd := exec.CommandContext(ctx, "python3", "-c", fragment)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "scripts"))
	output, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("Python unit producer failed: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("start Python unit producer: %v", err)
	}

	var snapshot unitParitySnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		t.Fatalf("decode Python unit producer: %v\n%s", err, output)
	}
	return snapshot
}

func unitParityGoSnapshot(t *testing.T, root string) unitParitySnapshot {
	t.Helper()
	tc, err := gotoolchain.New(root)
	if err != nil {
		t.Fatalf("derive Go toolchain: %v", err)
	}

	gates := testunit.Table()
	actions := testunit.Actions().Actions
	if len(actions) != len(gates) {
		t.Fatalf("Go action table has %d rows for %d gates", len(actions), len(gates))
	}

	snapshot := unitParitySnapshot{Area: testunit.Area, Gates: make([]unitParityGate, 0, len(gates))}
	for index, gate := range gates {
		snapshot.Gates = append(snapshot.Gates, unitParityGate{
			Name:        gate.Name,
			Argv:        gate.Argv(tc),
			Why:         gate.Why,
			Writes:      actions[index].Writes,
			Environment: unitParityEnvironment(tc.Environment(gate.EnvOptions())),
		})
	}
	return snapshot
}

func unitParityEnvironment(entries []string) map[string]string {
	all := make(map[string]string, len(entries))
	for _, entry := range entries {
		for index := range len(entry) {
			if entry[index] == '=' {
				all[entry[:index]] = entry[index+1:]
				break
			}
		}
	}

	selected := make(map[string]string, len(unitParityEnvironmentKeys))
	for _, key := range unitParityEnvironmentKeys {
		if value, ok := all[key]; ok {
			selected[key] = value
		}
	}
	return selected
}
