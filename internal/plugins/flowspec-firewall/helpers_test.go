package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
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
