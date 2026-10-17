package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func testBridge(t *testing.T) *bridge {
	t.Helper()
	// Unit tests must not autoload a real backend. Integration and failure
	// tests explicitly load their own backend before constructing the bridge.
	if firewall.GetBackend() == nil {
		name := "flowspec-unit-" + t.Name()
		backend := &lowerOnlyCanonicalBackend{}
		require.NoError(t, firewall.RegisterBackend(name, func() (firewall.Backend, error) { return backend, nil }))
		// Registered first, so it runs after the CloseBackend cleanup below.
		t.Cleanup(func() {
			if err := firewall.UnregisterBackendForTest(name); err != nil {
				t.Errorf("UnregisterBackendForTest: %v", err)
			}
		})
		require.NoError(t, firewall.LoadBackend(name))
		t.Cleanup(func() {
			_ = firewall.RegisterTables("flowspec", nil)
			_ = firewall.CloseBackend()
		})
	}
	return newBridge(slogutil.DiscardLogger())
}

func flowWire(t *testing.T, fam family.Family, components ...flowspec.FlowComponent) []byte {
	t.Helper()
	fs := flowspec.NewFlowSpec(fam)
	for _, component := range components {
		require.NoError(t, fs.AddComponent(component))
	}
	return fs.Bytes()
}

func selectedFixture(t *testing.T, prefix string, communities []byte, components ...flowspec.FlowComponent) *ribevents.FlowSpecChange {
	t.Helper()
	all := []flowspec.FlowComponent{flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix(prefix))}
	all = append(all, components...)
	return &ribevents.FlowSpecChange{Family: flowFamily(), NLRI: flowWire(t, flowFamily(), all...), ExtendedCommunities: communities}
}

var discardTrafficRate = []byte{0x80, 6, 0xfd, 0xe9, 0, 0, 0, 0}

// tcpFlagsComponent builds a TCP flags component (Type 9) whose matches carry
// no operator bits, through the constructor the config builder uses.
func tcpFlagsComponent(flags ...uint8) flowspec.FlowComponent {
	matches := make([]flowspec.FlowMatch, len(flags))
	for i, f := range flags {
		matches[i] = flowspec.FlowMatch{Value: uint64(f)}
	}
	return flowspec.NewFlowTCPFlagsMatchComponent(matches)
}

// packetLengthComponent builds a packet length component (Type 10) of
// equality matches, through the constructor the config builder uses.
func packetLengthComponent(lengths ...uint16) flowspec.FlowComponent {
	matches := make([]flowspec.FlowMatch, len(lengths))
	for i, l := range lengths {
		matches[i] = flowspec.FlowMatch{Op: flowspec.FlowOpEqual, Value: uint64(l)}
	}
	return flowspec.NewFlowNumericComponent(flowspec.FlowPacketLength, matches)
}
