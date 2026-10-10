package siteterminaldemo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const repoDemoRoot = "../../../../demos/terminal"

// VALIDATES: spec-terminal-demo-showcase AC-6. Every demo in the real
// manifest resolves, through its `validate` id, to one registered scenario
// with a validator; every `ze-demo run <id>` a tape types names a scenario
// with a lab runner; an unregistered id is refused by name.
// PREVENTS: a central switch or map that a new demo must edit, and a demo
// that ships with no check of what it shows.
func TestEveryManifestDemoHasARegisteredScenario(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoDemoRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(manifest.Demos) == 0 {
		t.Fatal("the manifest lists no demo")
	}
	runID := regexp.MustCompile(`ze-demo run ([a-z0-9-]+) `)
	for _, demo := range manifest.Demos {
		if _, err := scenarios.lookup(demo.Validate); err != nil {
			t.Errorf("demo %s: %v", demo.ID, err)
		}
		tape, err := os.ReadFile(filepath.Join(repoDemoRoot, demo.ID, "demo.tape"))
		if err != nil {
			continue // a browser demo has no tape
		}
		for _, match := range runID.FindAllStringSubmatch(string(tape), -1) {
			sc, err := scenarios.lookup(match[1])
			if err != nil {
				t.Errorf("demo %s tape: %v", demo.ID, err)
				continue
			}
			if sc.run == nil {
				t.Errorf("demo %s tape runs `ze-demo run %s` and that scenario has no lab runner", demo.ID, match[1])
			}
		}
	}

	_, err = scenarios.lookup("no-such-demo")
	if err == nil || !strings.Contains(err.Error(), `"no-such-demo"`) {
		t.Fatalf("unknown id: err %v, want a refusal naming it", err)
	}
}

// VALIDATES: spec-terminal-demo-showcase AC-6. A second registration under
// one id, or one with no validator, panics at init rather than shadowing.
func TestScenarioRegistrationRefusesADuplicateId(t *testing.T) {
	ok := func() error { return nil }
	registry := scenarioRegistry{}
	registry.add("demo", scenario{validate: ok})

	for name, register := range map[string]func(){
		"duplicate":    func() { registry.add("demo", scenario{validate: ok}) },
		"no validator": func() { registry.add("other", scenario{}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("registration did not panic")
				}
			}()
			register()
		})
	}
}
