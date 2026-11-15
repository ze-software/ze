// Design: docs/architecture/testing/interop.md -- preserve numbered checker failures
// Related: check_watchdog_withdraw_value.go -- bespoke route operation loop

package bgp

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestWatchdogRouteFailureStopsOperations cancels the first FRR query instead
// of waiting for its retry budget. The checker must return that numbered,
// network-scoped failure and issue no later route query or pool command.
func TestWatchdogRouteFailureStopsOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lab := &watchdogRouteFailureLab{cancel: cancel}
	err := checkWatchdogWithdrawValueRoutes(ctx, &interoplab.CheckContext{
		Lab: lab,
		Network: interoplab.Network{
			IPv4: netip.MustParsePrefix("172.31.77.0/24"),
			IPv6: netip.MustParsePrefix("fd00:77::/64"),
		},
	})
	if err == nil {
		t.Fatal("the first failed operation was accepted")
	}
	if _, reported := errors.AsType[*scenarioFailure](err); !reported {
		t.Fatalf("failure lost its scenario marker: %v", err)
	}
	const prefix = "scenario watchdog-withdraw-value-frr assertion 1: selected networks IPv4=172.31.77.0/24 IPv6=fd00:77::/64:"
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("failure lost its scenario, assertion index or selected networks: %v", err)
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("failure lost the canceled first-query cause: %v", err)
	}
	want := []string{"frr vtysh -c show bgp neighbor 172.31.77.2"}
	if !slices.Equal(lab.queries, want) {
		t.Fatalf("queries = %v, want only the first operation %v", lab.queries, want)
	}
	if lab.execs != 0 {
		t.Fatalf("issued %d pool commands after the failed first operation", lab.execs)
	}
}

// watchdogRouteFailureLab records executable operations. Diagnostic log reads
// use noEvidenceLab, so failure reporting cannot invent additional route reads.
type watchdogRouteFailureLab struct {
	noEvidenceLab
	cancel  context.CancelFunc
	queries []string
	execs   int
}

func (lab *watchdogRouteFailureLab) Query(ctx context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	lab.queries = append(lab.queries, peer+" "+strings.Join(command, " "))
	lab.cancel()
	return "", ctx.Err()
}

func (lab *watchdogRouteFailureLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	lab.execs++
	return interoplab.CommandResult{}, errNoPeerEvidence
}
