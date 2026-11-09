// Design: docs/architecture/testing/interop.md -- a checker removes lab plumbing that would mask what Ze sends.

package bgp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// defaultRouteLab answers one Exec with a fixed result and records its argv.
type defaultRouteLab struct {
	noEvidenceLab
	result interoplab.CommandResult
	err    error
	argv   []string
}

func (l *defaultRouteLab) Exec(_ context.Context, _ string, argv []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.argv = argv
	return l.result, l.err
}

// TestRemoveContainerDefaultRouteRunsIPRouteDelete requires the helper to run
// `ip route del default` in the peer, to accept a container that has no
// default route (Docker.Exec's error for a non-zero exit, with "No such
// process" on stderr), and to report any other failure.
//
// MUTATION: (internal/le/interoplab/bgp/check_container_routes.go,
// `	if strings.Contains(result.Stderr, "No such process") || strings.Contains(err.Error(), "No such process") {`,
// `	if false && strings.Contains(result.Stderr, "No such process") {`): a
// container with no default route fails the checker.
// MUTATION: (internal/le/interoplab/bgp/check_container_routes.go,
// `	return fmt.Errorf("remove the Docker default route from %s: %w", peer, err)`,
// `	return nil`): a refused deletion passes silently.
func TestRemoveContainerDefaultRouteRunsIPRouteDelete(t *testing.T) {
	cases := []struct {
		name    string
		result  interoplab.CommandResult
		err     error
		refuses bool
	}{
		{"deleted", interoplab.CommandResult{}, nil, false},
		{"already absent", interoplab.CommandResult{Stderr: "RTNETLINK answers: No such process\n", ExitCode: 2}, errors.New("docker exec: exit 2"), false},
		{"refused", interoplab.CommandResult{Stderr: "RTNETLINK answers: Operation not permitted\n", ExitCode: 2}, errors.New("docker exec: exit 2"), true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			lab := &defaultRouteLab{result: test.result, err: test.err}
			err := removeContainerDefaultRoute(t.Context(), lab, peerFRR)
			if !slices.Equal(lab.argv, []string{"ip", "route", "del", "default"}) {
				t.Fatalf("ran %q, want ip route del default", lab.argv)
			}
			if test.refuses && err == nil {
				t.Fatal("a refused deletion was accepted")
			}
			if !test.refuses && err != nil {
				t.Fatalf("deletion refused: %v", err)
			}
		})
	}
}

// TestNSSADefaultDiagnosticKeepsCauseAndAddsRoutes requires the assertion-2
// diagnostic to keep the wait's error in the chain and to append what FRR and
// the kernel hold, so a failed lab run names the route that won.
//
// MUTATION: (internal/le/interoplab/bgp/check_container_routes.go,
// `	return fmt.Errorf("%w; FRR default routes: %s; kernel routes: %s", cause, frrDefault, kernel)`,
// `	return fmt.Errorf("%v; FRR default routes: %s; kernel routes: %s", cause, frrDefault, kernel)`):
// the cause leaves the chain, and errors.Is fails.
func TestNSSADefaultDiagnosticKeepsCauseAndAddsRoutes(t *testing.T) {
	cause := errors.New("wait timed out")
	lab := &defaultRouteLab{result: interoplab.CommandResult{Stdout: "default via 172.30.0.1 dev eth0\n"}}
	err := nssaDefaultDiagnostic(t.Context(), lab, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("the wait's error left the chain: %v", err)
	}
	if !strings.Contains(err.Error(), "default via 172.30.0.1 dev eth0") {
		t.Fatalf("the diagnostic lost the route output: %v", err)
	}
}
