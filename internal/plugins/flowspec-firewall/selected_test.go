package flowspecfirewall

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func TestSelectedFlowSpecReplacesActionsAndWithdraws(t *testing.T) {
	const backendName = "flowspec-selected-test"
	backend := &lowerOnlyCanonicalBackend{}
	require.NoError(t, firewall.RegisterBackend(backendName, func() (firewall.Backend, error) { return backend, nil }))
	require.NoError(t, firewall.LoadBackend(backendName))
	t.Cleanup(func() {
		_ = firewall.RegisterTables("flowspec", nil)
		_ = firewall.CloseBackend()
	})
	bridge := testBridge(t)
	change := ribevents.FlowSpecChange{
		Family:              family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
		NLRI:                []byte{5, 1, 24, 10, 1, 0},
		ExtendedCommunities: []byte{0x80, 6, 0, 0, 0, 0, 0, 0},
	}
	bridge.handleSelected(&change)
	require.True(t, selectedKernelAction[firewall.Drop](backend.flushed))
	change.ExtendedCommunities = []byte{0x80, 9, 0, 0, 0, 0, 0, 46}
	bridge.handleSelected(&change)
	require.False(t, selectedKernelAction[firewall.Drop](backend.flushed), "an attribute-only replacement removes the previous discard")
	require.True(t, selectedKernelAction[firewall.SetDSCP](backend.flushed))

	// An unknown generic transitive EC is not an unknown traffic action.
	change.ExtendedCommunities = []byte{0x80, 0x99, 0, 0, 0, 0, 0, 1}
	bridge.handleSelected(&change)
	require.True(t, selectedKernelAction[firewall.Accept](backend.flushed))
	require.False(t, selectedKernelAction[firewall.SetDSCP](backend.flushed))

	// A known redirect is unperformable. It must remove the prior rule,
	// rather than preserving a different action under the same NLRI key.
	change.ExtendedCommunities = []byte{0x80, 8, 0xfd, 0xe9, 0, 0, 0, 1}
	bridge.handleSelected(&change)
	require.False(t, selectedTablePresent(backend.flushed))
	change.ExtendedCommunities = nil
	bridge.handleSelected(&change)
	require.True(t, selectedKernelAction[firewall.Accept](backend.flushed))
	change.Withdraw = true
	bridge.handleSelected(&change)
	require.False(t, selectedTablePresent(backend.flushed))
}

func selectedKernelAction[A firewall.Action](tables []firewall.Table) bool {
	for _, table := range tables {
		if table.Name != tableName {
			continue
		}
		for _, chain := range table.Chains {
			for _, term := range chain.Terms {
				for _, action := range term.Actions {
					if _, ok := action.(A); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

func selectedTablePresent(tables []firewall.Table) bool {
	for _, table := range tables {
		if table.Name == tableName {
			return true
		}
	}
	return false
}

// Native length octets frame the rule rather than identify it. Replacement
// and withdrawal must work even when the peer changes the length encoding.
func TestSelectedFlowSpecLengthEncodingDoesNotChangeIdentity(t *testing.T) {
	b := testBridge(t)
	change := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	b.handleSelected(change)
	change.NLRI = append([]byte{0xf0, change.NLRI[0]}, change.NLRI[1:]...)
	change.ExtendedCommunities = []byte{0x80, 9, 0, 0, 0, 0, 0, 46}
	b.handleSelected(change)
	require.False(t, selectedKernelAction[firewall.Drop](b.rules.buildTable()))
	require.True(t, selectedKernelAction[firewall.SetDSCP](b.rules.buildTable()))
	change.NLRI = change.NLRI[1:]
	change.Withdraw = true
	b.handleSelected(change)
	require.Nil(t, b.rules.buildTable())
}

// Partial action bytes cannot turn an unperformable replacement into normal
// forwarding, and the previously installed action must leave with it.
func TestSelectedFlowSpecMalformedActionsRemovePreviousRule(t *testing.T) {
	b := testBridge(t)
	change := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	b.handleSelected(change)
	require.True(t, selectedKernelAction[firewall.Drop](b.rules.buildTable()))
	change.ExtendedCommunities = []byte{0x80, 8, 0, 0}
	b.handleSelected(change)
	require.Nil(t, b.rules.buildTable())
}

// A single limiter cannot enforce both byte and packet rates. Refusing the
// complete rule prevents the last community from silently choosing one unit.
func TestSelectedFlowSpecRefusesConflictingRateDimensions(t *testing.T) {
	b := testBridge(t)
	bytes := []byte{0x80, 6, 0, 0, 0x44, 0x7a, 0, 0}
	packets := []byte{0x80, 12, 0, 0, 0x44, 0x7a, 0, 0}
	change := selectedFixture(t, "10.1.0.0/24", bytes)
	b.handleSelected(change)
	require.True(t, selectedKernelAction[firewall.Limit](b.rules.buildTable()))
	for _, communities := range [][]byte{
		append(append([]byte(nil), bytes...), packets...),
		append(append([]byte(nil), packets...), bytes...),
	} {
		change.ExtendedCommunities = communities
		b.handleSelected(change)
		require.Nil(t, b.rules.buildTable())
	}
}

// A code-25 redirect is an action, not an absent eight-octet community.
// Replacing an enforceable action with it must remove the previous rule.
func TestSelectedFlowSpecIPv6ActionsRemovePreviousRule(t *testing.T) {
	b := testBridge(t)
	change := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
	b.handleSelected(change)
	require.True(t, selectedKernelAction[firewall.Drop](b.rules.buildTable()))
	redirect, err := attribute.FlowSpecRedirectToIPv6(netip.MustParseAddr("2001:db8::1"))
	require.NoError(t, err)
	change.ExtendedCommunities = nil
	change.IPv6ExtendedCommunities = redirect[:]
	b.handleSelected(change)
	require.Nil(t, b.rules.buildTable())

	unknown := attribute.IPv6ExtendedCommunity{0, 0x99}
	change.IPv6ExtendedCommunities = unknown[:]
	b.handleSelected(change)
	require.True(t, selectedKernelAction[firewall.Accept](b.rules.buildTable()))
	change.IPv6ExtendedCommunities = redirect[:19]
	b.handleSelected(change)
	require.Nil(t, b.rules.buildTable())
}

// RFC 8956 Section 6.1 fixes the complete action type at 0x000d.
func TestSelectedFlowSpecIPv6RouteTargetRedirectRefused(t *testing.T) {
	wire, err := hex.DecodeString("000d20010db80000000000000000000000010064")
	require.NoError(t, err)
	typed, err := attribute.ParseIPv6ExtendedCommunities(wire)
	require.NoError(t, err)
	encoded := make([]byte, typed.Len())
	typed.WriteTo(encoded, 0)
	for _, input := range [][]byte{wire, encoded} {
		b := testBridge(t)
		change := selectedFixture(t, "10.1.0.0/24", discardTrafficRate)
		b.handleSelected(change)
		require.True(t, selectedKernelAction[firewall.Drop](b.rules.buildTable()))
		change.ExtendedCommunities = nil
		change.IPv6ExtendedCommunities = input
		b.handleSelected(change)
		require.Nil(t, b.rules.buildTable(), "an unsupported redirect must remove the stale discard, not install accept")
	}
}
