// Design: docs/architecture/testing/interop.md -- the owning lab declares each scenario.
// Related: register_scenarios.go -- the established L2TP scenarios.
// Related: register_scale.go -- the many-sessions scenario's independent declaration.
package l2tp

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// scenarioRegistration owns the checker and topology of one scenario. The
// registry is populated only at init and is read-only while a suite runs.
type scenarioRegistration struct {
	name         string
	check        scenarioCheck
	failurePeers []string
	needsKernel  bool
	plan         func(string, interoplab.ScenarioSource, interoplab.Environment) interoplab.ScenarioPlan
	images       func(string, interoplab.ScenarioSource, string) []interoplab.ImageBuild
}

var scenarioRegistrations = make(map[string]scenarioRegistration)

// registerScenario MUST be called only from init. A duplicate or incomplete
// declaration is an internal wiring defect, never a peer-controlled input.
func registerScenario(registration scenarioRegistration) {
	if registration.name == "" {
		panic("BUG: L2TP scenario registration has no name")
	}
	if registration.check == nil {
		panic("BUG: L2TP scenario " + registration.name + " has no checker")
	}
	if registration.plan == nil {
		panic("BUG: L2TP scenario " + registration.name + " has no plan")
	}
	if registration.images == nil {
		panic("BUG: L2TP scenario " + registration.name + " has no image declaration")
	}
	if _, exists := scenarioRegistrations[registration.name]; exists {
		panic("BUG: L2TP scenario " + registration.name + " registered twice")
	}
	scenarioRegistrations[registration.name] = registration
}

func scenarioCheckerMap(timeout time.Duration) map[string]interoplab.Checker {
	checks := make(map[string]interoplab.Checker, len(scenarioRegistrations))
	for name, registration := range scenarioRegistrations {
		checks[name] = checker(registration.check, timeout, registration.failurePeers...)
	}
	return checks
}

func scenarioPlan(root string, source interoplab.ScenarioSource, environment interoplab.Environment) interoplab.ScenarioPlan {
	registration, found := scenarioRegistrations[source.Name]
	if !found {
		return interoplab.ScenarioPlan{Source: source,
			Prepare: func(context.Context, interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
				return interoplab.PreparedScenario{}, fmt.Errorf("unsupported L2TP scenario %s", source.Name)
			}}
	}
	return registration.plan(root, source, environment)
}

// imageBuilds derives the selected image population from each scenario's own
// declaration. Shared images are built once, in first-use order.
func imageBuilds(root string, plans []interoplab.ScenarioPlan, frrImage string) []interoplab.ImageBuild {
	var builds []interoplab.ImageBuild
	seen := make(map[string]bool)
	for index := range plans {
		registration, found := scenarioRegistrations[plans[index].Source.Name]
		if !found {
			// Discover refuses this before a run reaches image selection.
			panic("BUG: image selection reached unregistered L2TP scenario " + plans[index].Source.Name)
		}
		images := registration.images(root, plans[index].Source, frrImage)
		for imageIndex := range images {
			if seen[images[imageIndex].Name] {
				continue
			}
			seen[images[imageIndex].Name] = true
			builds = append(builds, images[imageIndex])
		}
	}
	return builds
}
