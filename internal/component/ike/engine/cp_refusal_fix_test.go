// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// Related: create_child.go -- childRefusedResponderFix; transport_mode.go -- decideResponderTransportMode
// VALIDATES: a virtual-IP refusal names its own fix, and a pool-bound peer asking for
// transport mode is answered in tunnel mode.
// PREVENTS: a log that tells the operator to fix proposals for a missing CFG_REQUEST, and
// a declined transport request that still refuses the tunnel answer.

package engine

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestChildRefusedFixNamesVirtualIPCauses checks that each virtual-IP refusal, wrapped
// as assignVirtualIP wraps it, selects its own next step and not the proposal advice.
func TestChildRefusedFixNamesVirtualIPCauses(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want []string
	}{
		{"FAILED_CP_REQUIRED", errCPRequired, []string{"FAILED_CP_REQUIRED", "vips = 0.0.0.0", "delete virtual-ip-pool", "ze signal reload"}},
		{"INTERNAL_ADDRESS_FAILURE", fmt.Errorf("%w: pool %q: %w", errNoAddressAssigned, "rw", errTSUnusable),
			[]string{"INTERNAL_ADDRESS_FAILURE", "widen range or range6", "add the requested family", "ze signal reload"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := childRefusedResponderFix(fmt.Errorf("ike auth: %w", tc.err))
			if got == childRefusedFixResponder {
				t.Fatalf("the %s refusal fell to the proposal advice %q", tc.name, got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("the %s fix %q does not name %q", tc.name, got, w)
				}
			}
		})
	}
}

// TestPoolBoundTransportRequestAnsweredInTunnelMode narrows a pool-bound peer's Child SA
// request that asked for transport mode, after decideResponderTransportMode declined it.
// The answer is the tunnel-mode one: the lease host against the configured local prefix.
func TestPoolBoundTransportRequestAnsweredInTunnelMode(t *testing.T) {
	vip := cpVIP(t, cpPool())
	sa := testSAWithGCMKeys(t)
	sa.PeerCfg.VirtualIP = &ipsec.VirtualIPBinding{Pool: "rw"}
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, "192.168.0.0/16"),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustNet(t, "10.10.0.0/24"),
		RemotePort:   ipsec.AnyPort(),
	}}
	lease := leaseOnto(t, vip, sa)
	sa.UseTransportMode = decideResponderTransportMode(sa, true) == modeTransport
	if sa.UseTransportMode {
		t.Fatal("a pool-bound peer's transport request was accepted, want declined")
	}

	err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0"), tsPayload(t, wire.PayloadTypeTSr, "192.168.0.0/16"), nil)
	if err != nil {
		t.Fatalf("the declined transport request was refused: %v, want a tunnel-mode answer", err)
	}
	want := []string{lease.String() + "/32 <-> 192.168.0.0/16"}
	if got := scopeText(sa.NegotiatedPairs); !slices.Equal(got, want) {
		t.Fatalf("answered %v, want %v", got, want)
	}
}
