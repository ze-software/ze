// Design: docs/architecture/testing/interop.md -- native general interoperability gate.
// Related: prepare.go -- scenario rendering and peer construction.
// Related: checkers.go -- complete typed scenario checker catalog.
package bgp

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const defaultFRRImage = "quay.io/frrouting/frr:10.3.1"

// defaultPMACCTImage is the third-party BMP collector the RFC 9069 Loc-RIB
// scenario reads ze's stream with. pmacct decodes the Loc-RIB Instance Peer
// type, the Peer Up Information TLVs and the Peer Down reason itself, so what
// it writes is another implementation's reading of ze's bytes rather than ze's
// own.
const defaultPMACCTImage = "pmacct/pmbmpd:latest"

// LabBinaries declares the two personalities this lab stages into its Docker
// build context, which is what test/interop/Dockerfile.ze copies in.
//
// TWO of them, and that is what separates this lab from the other four: 14
// scenario ze.conf files run `ze-test interop-bgp process ...` from inside the
// container, so an image carrying the daemon alone answers those scenarios with
// "ze-test: not found".
//
// The bases differ because the personalities do. ze_core selects the daemon
// dispatch table that registers the `start` root command, ze_distro selects the
// distribution plugin mode, and ze_test selects the functional-test
// personality. The feature gates are added to each by the producer, from
// feature-gates.txt, so neither binary can carry a smaller feature set than the
// shipped daemon.
func LabBinaries() []interoplab.LabBinary {
	return []interoplab.LabBinary{
		{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop/ze-linux"},
		{Name: "ze-test", Base: "ze_test", Output: "test/interop/ze-test-linux"},
	}
}

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
	suite, err := suiteFor(root, options)
	if err != nil {
		return setupFailure(err)
	}
	return suite.Run(ctx)
}

// suiteFor builds the complete suite without starting anything, so a test can
// assert what this lab declares rather than what a Docker run happens to do.
func suiteFor(root string, options Options) (interoplab.Suite, error) {
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
		return interoplab.Suite{}, err
	}
	plans, err := scenarioPlans(root, producer, options.Suffix, sources)
	if err != nil {
		return interoplab.Suite{}, err
	}
	return interoplab.Suite{
		Docker:    interoplab.NewDocker(),
		Preflight: interoplab.StageBinaries(root, options.NoBuild, LabBinaries()...),
		Images: []interoplab.ImageBuild{
			{Name: "ze", Tag: "ze-interop", Dockerfile: filepath.Join(producer, "Dockerfile.ze"), Context: root, Required: true},
			{Name: peerBIRD, Tag: "bird-interop", Dockerfile: filepath.Join(producer, "Dockerfile.bird"), Context: producer, Required: true},
			{Name: peerGoBGP, Tag: "gobgp-interop", Dockerfile: filepath.Join(producer, "Dockerfile.gobgp"), Context: producer},
			{Name: peerKeepalived, Tag: "keepalived-interop", Dockerfile: filepath.Join(producer, "Dockerfile.keepalived"), Context: producer, Required: true},
			{Name: peerStayRTR, Tag: "stayrtr-interop", Dockerfile: filepath.Join(producer, "Dockerfile.stayrtr"), Context: producer},
			{Name: peerFRR, Tag: environment.Image, Pull: true, Required: true},
			{Name: peerPMACCT, Tag: defaultPMACCTImage, Pull: true},
		},
		Scenarios: plans,
		NoBuild:   options.NoBuild,
	}, nil
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}
