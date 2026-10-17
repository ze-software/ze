package interoplab

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestSuiteNarratesEachScenarioWhileItRuns drives two scenarios through
// Suite.Run and reads the progress sink from INSIDE the second checker.
// VALIDATES: the first scenario's verdict and the second's start are written
// before the second scenario finishes, and a failure's line carries its cause.
// PREVENTS: a suite that prints only its final report, which left a hung BGP
// lab silent for 89 minutes until the docker-lab ssh deadline killed it.
func TestSuiteNarratesEachScenarioWhileItRuns(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = kernelAnswering(func(processCommand) (processResult, error) {
		return processResult{}, nil
	})
	var progress bytes.Buffer
	var seenDuringSecond string
	pass := func(context.Context, *CheckContext) error { return nil }
	fail := func(context.Context, *CheckContext) error {
		seenDuringSecond = progress.String()
		return errors.New("the peer never answered")
	}
	scenario := func(name string, checker Checker) ScenarioPlan {
		return ScenarioPlan{
			Source:  ScenarioSource{Name: name, Directory: "/scenarios/" + name, Checker: checker},
			Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
			Peers:   []PeerConfig{{Name: "ze", Container: "lab-ze", Image: "ze-image", Host: 2}},
		}
	}
	suite := Suite{
		Docker:    newDocker(runner),
		StagedZe:  kernelCheckZe,
		NoBuild:   true,
		Progress:  &progress,
		Scenarios: []ScenarioPlan{scenario("one", pass), scenario("two", fail)},
	}

	report := suite.Run(t.Context())
	if report.Passed != 1 || report.Failed != 1 {
		t.Fatalf("report passed %d failed %d, want 1 and 1: %#v", report.Passed, report.Failed, report)
	}
	for _, want := range []string{"interop: preparing 0 image(s)\n", "interop: images ready (", "interop: [1/2] one: running\n", "interop: [1/2] one: pass (", "interop: [2/2] two: running\n"} {
		if !strings.Contains(seenDuringSecond, want) {
			t.Errorf("progress before the second scenario ended lacks %q:\n%s", want, seenDuringSecond)
		}
	}
	if strings.Contains(seenDuringSecond, "[2/2] two: FAIL") {
		t.Errorf("the second verdict was written before its checker returned:\n%s", seenDuringSecond)
	}
	if !strings.Contains(progress.String(), "): the peer never answered\n") || !strings.Contains(progress.String(), "interop: [2/2] two: FAIL (") {
		t.Errorf("the failing scenario's line lacks its verdict and cause:\n%s", progress.String())
	}
}
