// Design: docs/architecture/testing/interop.md -- fail-closed neighbor-flip observations.
// Related: check_rfc2545_neighbor_flip.go -- daemon/FRR scenario under test.
// RFC naming: untagged -- these tests validate the checker; its live scenario observes protocol behavior separately.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestRFC2545NeighborFlipMutationAcceptsSilentExit preserves the native ip
// command contract: silence is a successful mutation only after exit zero;
// execution errors and nonzero exits must stop the checker before observations.
func TestRFC2545NeighborFlipMutationAcceptsSilentExit(t *testing.T) {
	nativeFailure := errors.New("native execution unavailable")
	for name, test := range map[string]struct {
		result    interoplab.CommandResult
		failure   error
		wantError bool
	}{
		"silent success":    {},
		"nonzero exit":      {result: interoplab.CommandResult{ExitCode: 2, Stderr: "address exists"}, wantError: true},
		"execution failure": {failure: nativeFailure, wantError: true},
	} {
		t.Run(name, func(t *testing.T) {
			lab := &neighborFlipMutationLab{result: test.result, failure: test.failure}
			command := []string{"ip", "-6", ipObjectAddr, ipActionAdd, "fe80::2545:5642/64", ipOptionDevice, neighborFlipDevice, "nodad"}
			err := neighborFlipExec(t.Context(), lab, peerFRRSink, command)
			if (err != nil) != test.wantError {
				t.Fatalf("native mutation error = %v, want error=%t", err, test.wantError)
			}
			if test.failure != nil && !errors.Is(err, test.failure) {
				t.Fatalf("native execution failure lost: %v", err)
			}
			if lab.executions != 1 || lab.peer != peerFRRSink || !reflect.DeepEqual(lab.command, command) || len(lab.environment) != 0 {
				t.Fatalf("mutation did not execute its exact native command: %+v", lab)
			}
		})
	}
}

type neighborFlipMutationLab struct {
	interoplab.CheckerLab
	result      interoplab.CommandResult
	failure     error
	executions  int
	peer        string
	command     []string
	environment []interoplab.EnvironmentVariable
}

func (lab *neighborFlipMutationLab) Exec(_ context.Context, peer string, command []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	lab.executions++
	lab.peer, lab.command, lab.environment = peer, command, environment
	return lab.result, lab.failure
}

// TestRFC2545NeighborFlipKernelInput refuses unreadable, unresolved and wrong
// router identities instead of blaming the missing re-advertisement for them.
func TestRFC2545NeighborFlipKernelInput(t *testing.T) {
	global := netip.MustParseAddr("fd00:1e::f")
	linkLocal := netip.MustParseAddr("fe80::f")
	mac, err := net.ParseMAC("02:42:00:00:00:0f")
	if err != nil {
		t.Fatal(err)
	}
	const unique = `[{"dst":"fd00:1e::f","lladdr":"02:42:00:00:00:0f","state":["PERMANENT"]},{"dst":"fe80::f","lladdr":"02:42:00:00:00:0f","state":["STALE"]}]`
	ambiguous := strings.TrimSuffix(unique, "]") + `,{"dst":"fe80::2545:5642","lladdr":"02:42:00:00:00:0f","state":["PERMANENT"]}]`
	if err := requireNeighborFlipOutcome(unique, global, linkLocal, mac, network.LinkLocalFound); err != nil {
		t.Fatal(err)
	}
	if err := requireNeighborFlipOutcome(ambiguous, global, linkLocal, mac, network.LinkLocalAmbiguous); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"unknown":           `[]`,
		"unreadable":        `not JSON`,
		"unresolved global": strings.Replace(unique, `"PERMANENT"`, `"FAILED"`, 1),
		"wrong global MAC":  strings.Replace(unique, `02:42:00:00:00:0f`, `02:42:00:00:00:ff`, 1),
		"wrong link-local":  strings.Replace(unique, `fe80::f`, `fe80::99`, 1),
		"extra link-local":  ambiguous,
	} {
		t.Run(name, func(t *testing.T) {
			if requireNeighborFlipOutcome(output, global, linkLocal, mac, network.LinkLocalFound) == nil {
				t.Fatalf("accepted unusable kernel input: %s", output)
			}
		})
	}
}

// TestRFC2545NeighborFlipForeignRoute requires the requested prefix and its
// complete pair, not a matching address detached from the subject route.
func TestRFC2545NeighborFlipForeignRoute(t *testing.T) {
	global := netip.MustParseAddr("fd00:1e::f")
	linkLocal := netip.MustParseAddr("fe80::f")
	const paired = `{"prefix":"2001:db8:5642::/48","paths":[{"nexthops":[{"scope":"global","ip":"fd00:1e::f"},{"scope":"link-local","ip":"fe80::f"}]}]}`
	const sole = `{"prefix":"2001:db8:5642::/48","paths":[{"nexthops":[{"scope":"global","ip":"fd00:1e::f"}]}]}`
	if err := requireNeighborFlipRoute(paired, neighborFlipSubject, global, linkLocal); err != nil {
		t.Fatal(err)
	}
	if err := requireNeighborFlipRoute(sole, neighborFlipSubject, global, netip.Addr{}); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"unanswered": "", "null": "null", "unrelated": `{}`,
		"wrong prefix": strings.Replace(paired, neighborFlipSubject, neighborFlipUniqueControl, 1),
		"wrong global": strings.Replace(paired, `fd00:1e::f`, `fd00:1e::2`, 1),
		"wrong local":  strings.Replace(paired, `fe80::f`, `fe80::99`, 1),
		"no pair":      sole,
		"no route":     `{"prefix":"2001:db8:5642::/48","paths":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if requireNeighborFlipRoute(output, neighborFlipSubject, global, linkLocal) == nil {
				t.Fatalf("accepted an unrelated or incomplete foreign route: %s", output)
			}
		})
	}
	if requireNeighborFlipRoute(paired, neighborFlipSubject, global, netip.Addr{}) == nil {
		t.Fatal("the global-only baseline accepted a paired next hop")
	}
}

// TestRFC2545NeighborFlipRetentionPrerequisite refuses a remote observation
// without the daemon's exact sent route. A failure here is not a trigger red.
func TestRFC2545NeighborFlipRetentionPrerequisite(t *testing.T) {
	global := netip.MustParseAddr("fd00:1e::f")
	const peer = "fd00:1e::3"
	// Exact subject row captured by ed3f0b43's first native sent-RIB query.
	// The other two native rows differ only in prefix.
	const route = `{"as-path":{"optional":false,"partial":false,"transitive":true,"value":[65001]},"direction":"sent","family":"ipv6/unicast","next-hop":"fd00:1e::f","origin":{"optional":false,"partial":false,"transitive":true,"value":"igp"},"peer":"fd00:1e::3","prefix":"2001:db8:5642::/48"}`
	answer := `[` + route + `]`
	if err := requireNeighborFlipRetained(answer, peer, global); err != nil {
		t.Fatal(err)
	}
	controls := strings.Replace(route, neighborFlipSubject, neighborFlipUniqueControl, 1) + `,` +
		strings.Replace(route, neighborFlipSubject, neighborFlipAmbigControl, 1)
	if err := requireNeighborFlipRetained(`[`+controls+`,`+route+`]`, peer, global); err != nil {
		t.Fatalf("the native three-row census must retain exactly one subject: %v", err)
	}
	for name, output := range map[string]string{
		"empty":             "",
		"null":              "null",
		"null row":          `[null]`,
		"malformed":         strings.TrimSuffix(answer, "]"),
		"no retained route": `[]`,
		"only controls":     `[` + controls + `]`,
		"wrong owner":       strings.Replace(answer, peer, "fd00:1e::4", 1),
		"received only":     strings.Replace(answer, `"sent"`, `"received"`, 1),
		"wrong family":      strings.Replace(answer, `ipv6/unicast`, `ipv4/unicast`, 1),
		"wrong prefix":      strings.Replace(answer, neighborFlipSubject, neighborFlipUniqueControl, 1),
		"wrong next hop":    strings.Replace(answer, `"next-hop":"fd00:1e::f"`, `"next-hop":"fd00:1e::99"`, 1),
		"missing owner":     strings.Replace(answer, `"peer":"fd00:1e::3",`, "", 1),
		"missing next hop":  strings.Replace(answer, `"next-hop":"fd00:1e::f",`, "", 1),
		"unparseable hop":   strings.Replace(answer, `fd00:1e::f`, `not-an-address`, 1),
		"duplicate":         `[` + route + `,` + route + `]`,
		"RPC envelope":      `{"routes":` + answer + `}`,
		"wrong row type":    `[42]`,
		"unrelated":         `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if requireNeighborFlipRetained(output, peer, global) == nil {
				t.Fatalf("accepted an unproven retained subject: %s", output)
			}
		})
	}
}

// neighborFlipObservationLab records the queried surface while retaining the
// shared fake lab's complete interface. It owns no process or background work.
type neighborFlipObservationLab struct {
	recordingLab
	peers    []string
	commands [][]string
}

func (lab *neighborFlipObservationLab) Query(ctx context.Context, peer string, command []string, environment []interoplab.EnvironmentVariable) (string, error) {
	lab.peers = append(lab.peers, peer)
	lab.commands = append(lab.commands, command)
	return lab.recordingLab.Query(ctx, peer, command, environment)
}

// TestRFC2545NeighborFlipWaitNeverResends proves the decisive wait remains a
// foreign observation even when it fails: no native API send can hide a gap.
func TestRFC2545NeighborFlipWaitNeverResends(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(fmt.Sprint(complete), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			output := `{"prefix":"2001:db8:5642::/48","paths":[{"nexthops":[{"scope":"global","ip":"fd00:1e::f"}]}]}`
			if complete {
				output = `{"prefix":"2001:db8:5642::/48","paths":[{"nexthops":[{"scope":"global","ip":"fd00:1e::f"},{"scope":"link-local","ip":"fe80::f"}]}]}`
			}
			lab := &neighborFlipObservationLab{output: output, afterQuery: cancel}
			err := waitNeighborFlipRoute(ctx, lab, neighborFlipSubject,
				netip.MustParseAddr("fd00:1e::f"), netip.MustParseAddr("fe80::f"))
			if (err == nil) != complete {
				t.Fatalf("foreign transition verdict = %v, want complete=%t", err, complete)
			}
			if !reflect.DeepEqual(lab.peers, []string{peerFRR}) {
				t.Fatalf("the wait queried %v; it must query FRR alone", lab.peers)
			}
			want := [][]string{{cmdVtysh, "-c", "show bgp ipv6 unicast " + neighborFlipSubject + " json"}}
			if !reflect.DeepEqual(lab.commands, want) {
				t.Fatalf("the wait ran %v, want exactly %v", lab.commands, want)
			}
		})
	}
}

// TestRFC2545NeighborFlipWaitFailureKeepsBothCauses ensures a validation
// failure remains inspectable without hiding the wait failure, and a nil
// optional verdict cannot erase cancellation or produce a nil-wrap diagnostic.
func TestRFC2545NeighborFlipWaitFailureKeepsBothCauses(t *testing.T) {
	verdict := errors.New("FRR still has the old next hop")
	combined := neighborFlipWaitFailure(context.DeadlineExceeded, verdict)
	if !errors.Is(combined, context.DeadlineExceeded) || !errors.Is(combined, verdict) {
		t.Fatalf("wait failure lost a cause: %v", combined)
	}
	unchanged := neighborFlipWaitFailure(context.Canceled, nil)
	require.Same(t, context.Canceled, unchanged, "nil verdict changed the wait failure")
}

// TestRFC2545NeighborFlipControlRefusesSubject protects the distinction
// between repeatable setup controls and the one-shot retained subject.
func TestRFC2545NeighborFlipControlRefusesSubject(t *testing.T) {
	lab := &recordingLab{}
	err := neighborFlipControl(t.Context(), lab, neighborFlipSubject,
		netip.MustParseAddr("fd00:1e::f"), netip.MustParseAddr("fe80::f"))
	if err == nil {
		t.Fatal("the repeated control admitted the subject prefix")
	}
	if lab.reads != 0 {
		t.Fatalf("the refused subject still ran %d commands", lab.reads)
	}
}
