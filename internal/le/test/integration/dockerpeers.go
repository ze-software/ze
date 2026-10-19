// Design: docs/architecture/testing/qemu-integration.md -- the docker population and its verb
// Related: ../buildtag/buildtag.go -- the tree scan this verb's package set comes from.
// Related: gates.go -- the docker-peers and live-rpki rows.
// Related: cleanup.go -- labCleaner, the orphan sweep a signalled run ends with.

package testintegration

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/test/buildtag"
)

const (
	// dockerPeersVerb runs every package test that starts a real peer container.
	dockerPeersVerb = "docker-peers"
	// dockerTag is the build tag of a package test that needs a Docker daemon
	// and no Ze kernel: it never joins `integration`, whose guest has no Docker.
	dockerTag = "docker"
	// requireDockerVariable turns a missing docker binary from a skip into a
	// failure, so a gating run cannot pass because Docker was absent.
	requireDockerVariable = "ZE_REQUIRE_DOCKER"
	// dockerPeersTimeout bounds the whole docker population.
	dockerPeersTimeout = "900s"
	// liveRPKIVerb runs the RPKI client against a real stayrtr serving real
	// data: Docker and the internet.
	liveRPKIVerb = "live-rpki"
	// signalSweepTimeout bounds the orphan sweep a signalled run ends with.
	// The signal already fired, so a second Ctrl-C does not reach this sweep:
	// the bound is what stops it.
	signalSweepTimeout = 60 * time.Second
	// groupExitTimeout bounds, inside signalSweepTimeout, the wait for the
	// killed group's test binaries to finish exiting before the sweep judges
	// their containers. A SIGKILLed process exits in milliseconds, so the bound
	// is for a stuck one, and leaves the sweep most of its own.
	groupExitTimeout = 10 * time.Second
)

func runDockerPeers(ctx context.Context, root string) (any, int) {
	argv, err := dockerPeersArgv(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runRequiringDocker(ctx, dockerPeersVerb, argv, root, interoplab.NewDocker())
}

func runLiveRPKI(ctx context.Context, root string) (any, int) {
	return runRequiringDocker(ctx, liveRPKIVerb, liveRPKIArgv(), root, interoplab.NewDocker())
}

// runRequiringDocker runs argv under the checkout's test toolchain with
// ZE_REQUIRE_DOCKER=1, so labtest fails rather than skips when Docker, or a
// live test's image, is absent. The child runs under ctx, the verb's signal
// root, so a SIGINT or SIGTERM kills its process group and the verb answers
// non-zero at once instead of waiting out the child's own bound.
//
// That SIGKILL skips every labtest t.Cleanup, so a signalled run ends with
// one orphan sweep through cleaner, the call `./le test integration cleanup`
// makes: the killed test binaries own their containers, so the sweep proves
// them gone and removes them in this invocation rather than at the next lab
// start. A child that ends on its own already ran its cleanups and gets none.
//
// The sweep waits for the child's process group to empty first: reaping `go
// test` does not reap the test binaries it started, and a binary still exiting
// reads as a live owner, whose containers the sweep would leave.
func runRequiringDocker(ctx context.Context, verb string, argv []string, root string, cleaner labCleaner) (any, int) {
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, code, group := gaterun.RunContext(ctx, verb, argv, root,
		requireDockerEnvironment(tc.Environment(gotoolchain.EnvOptions{Test: true})))
	if ctx.Err() == nil {
		return report, code
	}
	gaterun.Note(sweepAfterSignal(ctx, verb, cleaner, group, groupExitTimeout))
	return report, code
}

// sweepAfterSignal waits, for at most groupBound, until the killed child's
// process group is empty, then runs one non-dry orphan sweep, both on a
// context that ctx's cancellation does not reach, bounded together by
// signalSweepTimeout, and answers the operator's summary. A group still holding
// members at its bound is swept anyway, and the summary names the pids left,
// whose containers the sweep may have judged live. A sweep the daemon refuses
// is reported with the fix: neither changes the signalled run's non-zero answer.
func sweepAfterSignal(ctx context.Context, verb string, cleaner labCleaner, group gaterun.ProcessGroup, groupBound time.Duration) string {
	sweep, cancel := context.WithTimeout(context.WithoutCancel(ctx), signalSweepTimeout)
	defer cancel()

	left := group.AwaitExit(sweep, groupBound)
	report, err := cleaner.CleanupOrphans(sweep, false)
	var text textbuf.Buffer
	if left != nil {
		text.Str(verb).Str(": the killed tests had not all exited, so the sweep may have kept containers of").
			Str(" theirs as live-owned: ").Err(left).
			Str(". Fix: once they exit, run `./le test integration cleanup`\n")
	}
	if err != nil {
		return text.Str(verb).Str(": stopped by a signal, and the sweep of its lab resources failed (").Err(err).
			Str("), so its containers stay until a later sweep. Fix: run `./le test integration cleanup`").String()
	}
	return text.Str(verb).Str(": stopped by a signal; swept the lab resources its killed tests left:\n").
		Str(report.Text()).String()
}

// liveRPKIArgv runs the three live tests of the RPKI plugin.
func liveRPKIArgv() []string {
	return []string{goTool, "test", "-v", "-tags", "live", "-timeout", "180s", "-count=1",
		"./internal/component/bgp/plugins/rpki/...", "-run", "TestLive"}
}

// dockerPeersArgv derives the population from the tree: every package under
// internal/ and cmd/ holding a _test.go whose constraint needs the docker tag
// (buildtag.TestFile.Needs), so a `!docker` file never joins it.
// An empty population is refused, because a verb that runs nothing proves
// nothing.
func dockerPeersArgv(root string) ([]string, error) {
	packages, err := buildtag.Packages(root, dockerTag)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, errors.New("docker-peers: no package under internal/ or cmd/ holds a _test.go whose" +
			" //go:build line names `docker`, so this verb would prove nothing. Fix: restore the" +
			" docker-tagged tests (internal/le/interoplab/orphans_docker_test.go is one), or find the change" +
			" that removed them")
	}
	argv := make([]string, 0, 7+len(packages))
	argv = append(argv, goTool, "test", "-tags", dockerTag, "-count=1", "-timeout", dockerPeersTimeout)
	for _, dir := range packages {
		argv = append(argv, "./"+dir)
	}
	return argv, nil
}

// requireDockerEnvironment adds the switch that makes an absent docker binary
// fail rather than skip.
func requireDockerEnvironment(base []string) []string {
	return append(append(make([]string, 0, len(base)+1), base...), requireDockerVariable+"=1")
}
