// The migration proof for the two platform-vet gates. The Python producer and
// the native command remain together until the composition swap.
//
// VALIDATES: scripts/le/application/repository.py and internal/le/platformvet agree
// on both gate rows, the exact three package patterns, argv, and environment.
// PREVENTS: a platform port that vets the host GOOS, adds tags, changes GOARCH,
// enables CGO, or drops one of the non-Linux package trees.
package application

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/platformvet"
)

type platformVetProducerGate struct {
	Argv        []string          `json:"argv"`
	Why         string            `json:"why"`
	Writes      bool              `json:"writes"`
	Environment map[string]string `json:"environment"`
}

type platformVetProducer struct {
	Platforms map[string]string                  `json:"platforms"`
	Packages  []string                           `json:"packages"`
	Gates     map[string]platformVetProducerGate `json:"gates"`
}

var platformVetEnvironmentKeys = [...]string{
	"GOCACHE",
	"GOLANGCI_LINT_CACHE",
	"CGO_ENABLED",
	"GOTOOLCHAIN",
	"GOMAXPROCS",
	"GOMEMLIMIT",
	"GOOS",
	"GOARCH",
}

func platformVetCheckout(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	return root
}

func platformVetPythonSnapshot(t *testing.T, root string) platformVetProducer {
	t.Helper()
	fragment := `
import json
from le.application.repository import GATES, PLATFORMS, PLATFORM_PACKAGES, _environment
keys = (` + "'GOCACHE','GOLANGCI_LINT_CACHE','CGO_ENABLED','GOTOOLCHAIN','GOMAXPROCS','GOMEMLIMIT','GOOS','GOARCH'" + `)
gates = {}
for gate in GATES.gates:
    if gate.name not in PLATFORMS:
        continue
    environment = _environment(gate)
    gates[gate.name] = {
        'argv': list(gate.argv),
        'why': gate.why,
        'writes': gate.writes,
        'environment': {key: environment.get(key, '<absent>') for key in keys},
    }
print(json.dumps({
    'platforms': PLATFORMS,
    'packages': PLATFORM_PACKAGES,
    'gates': gates,
}))
`
	cmd := exec.CommandContext(t.Context(), "python3", "-c", fragment) //nolint:gosec // the producer under comparison
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(root, "scripts"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read Python platform-vet producer: %v\n%s", err, out)
	}
	var snapshot platformVetProducer
	if err := json.Unmarshal(out, &snapshot); err != nil {
		t.Fatalf("decode Python platform-vet producer: %v\n%s", err, out)
	}
	return snapshot
}

func platformVetLastEnvironment(entries []string, key string) string {
	answer := "<absent>"
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if found && name == key {
			answer = value
		}
	}
	return answer
}

func TestPlatformVetPortMatchesTheProducerRowsAndEnvironment(t *testing.T) {
	root := platformVetCheckout(t)
	// These ambient values prove which fields each implementation overrides and
	// which it deliberately inherits.
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "arm64")
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOTOOLCHAIN", "ambient")
	t.Setenv("GOMAXPROCS", "91")
	t.Setenv("GOMEMLIMIT", "17GiB")

	producer := platformVetPythonSnapshot(t, root)
	if len(producer.Gates) != 2 {
		t.Fatalf("Python producer declared %d platform vets, want 2", len(producer.Gates))
	}
	if !slices.Equal(producer.Packages, platformvet.PackagePatterns()) {
		t.Fatalf("package patterns differ: producer=%v command=%v", producer.Packages, platformvet.PackagePatterns())
	}

	runner, err := platformvet.NewRunner(root)
	if err != nil {
		t.Fatalf("platformvet.NewRunner: %v", err)
	}
	platforms := map[string]platformvet.Platform{
		"ze-platform-vet-darwin":  platformvet.PlatformDarwin,
		"ze-platform-vet-freebsd": platformvet.PlatformFreeBSD,
	}
	actions := map[string]struct {
		why    string
		writes bool
	}{}
	for _, row := range platformvet.Actions().Actions {
		actions[row.Gate] = struct {
			why    string
			writes bool
		}{row.Why, row.Writes}
	}

	for gate, platform := range platforms {
		old, found := producer.Gates[gate]
		if !found {
			t.Errorf("Python producer has no %s", gate)
			continue
		}
		plan, planErr := runner.Plan(platform)
		if planErr != nil {
			t.Errorf("Plan(%s): %v", gate, planErr)
			continue
		}
		if plan.Gate != gate || producer.Platforms[gate] != plan.Platform {
			t.Errorf("%s platform differs: producer=%q command=%q", gate, producer.Platforms[gate], plan.Platform)
		}
		if !slices.Equal(old.Argv, plan.Command) {
			t.Errorf("%s argv differs: producer=%v command=%v", gate, old.Argv, plan.Command)
		}
		newAction, found := actions[gate]
		if !found {
			t.Errorf("native action table has no %s", gate)
		} else if old.Why != newAction.why || old.Writes != newAction.writes {
			t.Errorf("%s metadata differs: producer=(%q,%v) command=(%q,%v)",
				gate, old.Why, old.Writes, newAction.why, newAction.writes)
		}

		portedEnvironment := make(map[string]string, len(platformVetEnvironmentKeys))
		for _, key := range platformVetEnvironmentKeys {
			portedEnvironment[key] = platformVetLastEnvironment(plan.Environment, key)
		}
		if !reflect.DeepEqual(old.Environment, portedEnvironment) {
			t.Errorf("%s environment differs:\nproducer: %v\ncommand:  %v", gate, old.Environment, portedEnvironment)
		}
	}
}
