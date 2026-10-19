// Design: docs/architecture/testing/qemu-integration.md -- the docker population and its verb
// Related: ../buildtag/buildtag.go -- the tree scan this verb's package set comes from.
// Related: gates.go -- the docker-peers and live-rpki rows.

package testintegration

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
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
)

func runDockerPeers(ctx context.Context, root string) (any, int) {
	argv, err := dockerPeersArgv(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runRequiringDocker(ctx, dockerPeersVerb, argv, root)
}

func runLiveRPKI(ctx context.Context, root string) (any, int) {
	return runRequiringDocker(ctx, liveRPKIVerb, liveRPKIArgv(), root)
}

// runRequiringDocker runs argv under the checkout's test toolchain with
// ZE_REQUIRE_DOCKER=1, so labtest fails rather than skips when Docker, or a
// live test's image, is absent. The child runs under ctx, the verb's signal
// root, so a SIGINT or SIGTERM kills its process group and the verb answers
// non-zero at once instead of waiting out the child's own bound.
func runRequiringDocker(ctx context.Context, verb string, argv []string, root string) (any, int) {
	tc, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return gaterun.RunContext(ctx, verb, argv, root,
		requireDockerEnvironment(tc.Environment(gotoolchain.EnvOptions{Test: true})))
}

// liveRPKIArgv runs the three live tests of the RPKI plugin.
func liveRPKIArgv() []string {
	return []string{goTool, "test", "-v", "-tags", "live", "-timeout", "180s", "-count=1",
		"./internal/component/bgp/plugins/rpki/...", "-run", "TestLive"}
}

// dockerPeersArgv derives the population from the tree: every package under
// internal/ and cmd/ holding a _test.go whose constraint names the docker tag.
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
