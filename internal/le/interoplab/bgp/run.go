// Design: docs/architecture/testing/interop.md -- native general interoperability gate.
// Related: prepare.go -- scenario rendering and peer construction.
// Related: checkers.go -- complete typed scenario checker catalogue.
package bgp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const defaultFRRImage = "quay.io/frrouting/frr:10.3.1"

// Options selects one exact scenario and controls image reuse. Empty fields use
// the documented environment and process-id defaults.
type Options struct {
	Scenario string
	NoBuild  bool
	Suffix   string
}

// Run executes the native ze-interop-test adapter from the current checkout.
func Run(ctx context.Context, options Options) interoplab.SuiteReport {
	root, err := os.Getwd()
	if err != nil {
		return setupFailure(err)
	}
	return RunAt(ctx, root, options)
}

// RunAt executes the native ze-interop-test adapter at root.
func RunAt(ctx context.Context, root string, options Options) interoplab.SuiteReport {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: "INTEROP_SCENARIO",
		SuffixVariable:   "ZE_INTEROP_SUFFIX",
		DefaultImage:     defaultFRRImage,
		DefaultSuffix:    options.Suffix,
	})
	if options.Scenario == "" {
		options.Scenario = environment.Selector
	}
	if options.Suffix == "" {
		options.Suffix = environment.Suffix
	}
	if !options.NoBuild {
		options.NoBuild = environment.NoBuild
	}

	producer := filepath.Join(root, "test", "interop")
	sources, err := interoplab.Discover(
		filepath.Join(producer, "scenarios"), options.Scenario, checkers())
	if err != nil {
		return setupFailure(err)
	}
	plans, err := scenarioPlans(root, producer, options.Suffix, sources)
	if err != nil {
		return setupFailure(err)
	}
	tags, err := featuretags.DaemonTags(root)
	if err != nil {
		return setupFailure(err)
	}

	suite := interoplab.Suite{
		Docker: interoplab.NewDocker(),
		Images: []interoplab.ImageBuild{
			{Name: "ze", Tag: "ze-interop", Dockerfile: filepath.Join(producer, "Dockerfile.ze"), Context: root, BuildArgs: []string{"ZE_FEATURES=" + strings.Join(tags, " ")}, Required: true},
			{Name: "bird", Tag: "bird-interop", Dockerfile: filepath.Join(producer, "Dockerfile.bird"), Context: producer, Required: true},
			{Name: "gobgp", Tag: "gobgp-interop", Dockerfile: filepath.Join(producer, "Dockerfile.gobgp"), Context: producer},
			{Name: "keepalived", Tag: "keepalived-interop", Dockerfile: filepath.Join(producer, "Dockerfile.keepalived"), Context: producer, Required: true},
			{Name: "stayrtr", Tag: "stayrtr-interop", Dockerfile: filepath.Join(producer, "Dockerfile.stayrtr"), Context: producer},
			{Name: "frr", Tag: environment.Image, Pull: true, Required: true},
		},
		Scenarios: plans,
		NoBuild:   options.NoBuild,
	}
	return suite.Run(ctx)
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}
