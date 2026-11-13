// Design: docs/architecture/testing/interop.md -- OSPF redistribution failure evidence.
// Related: check_ospf_redistribute_default_diagnostics.go -- bounded read-only queries.
package bgp

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func TestOSPFRedistributeDiagnosticsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	lab := &ospfRedistributeDiagnosticLab{}
	output := ospfRedistributeDefaultDiagnostics(ctx, lab, "172.30.44.2", "172.30.44.3")
	if lab.contextError != nil {
		t.Fatalf("diagnostics inherited the failed check's cancellation: %v", lab.contextError)
	}
	if !lab.bounded {
		t.Fatal("diagnostic reads did not carry their own deadline")
	}
	for _, query := range []string{
		"frr vtysh -c show bgp ipv4 unicast json",
		"frr vtysh -c show bgp neighbor 172.30.44.2 json",
		"frr vtysh -c show bgp ipv4 unicast neighbors 172.30.44.2 routes json",
		"frr vtysh -c show bgp ipv4 unicast neighbors 172.30.44.2 received-routes json",
	} {
		if !slices.Contains(lab.queries, query) {
			t.Errorf("missing diagnostic query %q", query)
		}
	}
	for _, command := range []string{
		"show log recent component bgp.redistribute count 100",
		"show log recent component redistribute.orchestrator count 100",
		"show bgp peer 172.30.44.3 rib advertised",
		"show ospf route",
		"show rib candidates prefix " + ospfRedistributeDefaultPrefix,
	} {
		query := "ze " + strings.Join(zeCommand(command), " ")
		if !slices.Contains(lab.queries, query) {
			t.Errorf("missing diagnostic query %q", query)
		}
	}
	for _, evidence := range []string{"soft reconfiguration disabled", "actual consumer command", "logs error: log reader failed"} {
		if !strings.Contains(output, evidence) {
			t.Errorf("diagnostic output lost %q: %s", evidence, output)
		}
	}
}

type ospfRedistributeDiagnosticLab struct {
	noEvidenceLab
	queries      []string
	contextError error
	bounded      bool
}

func (lab *ospfRedistributeDiagnosticLab) Query(ctx context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	lab.contextError = errors.Join(lab.contextError, ctx.Err())
	deadline, ok := ctx.Deadline()
	lab.bounded = ok && time.Until(deadline) <= 15*time.Second
	query := peer + " " + strings.Join(command, " ")
	lab.queries = append(lab.queries, query)
	if strings.Contains(query, "received-routes") {
		return "", errors.New("soft reconfiguration disabled")
	}
	return "actual consumer command: " + query, nil
}

func (lab *ospfRedistributeDiagnosticLab) Logs(_ context.Context, _ string, _ int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, errors.New("log reader failed")
}
