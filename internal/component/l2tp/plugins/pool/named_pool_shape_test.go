// VALIDATES: a named pool an operator writes in the config is present at
// runtime, in the shape the config pipeline actually delivers it, and a
// subscriber whose RADIUS Framed-Pool names it gets an address from it.
// PREVENTS: every named pool being lost. Tree.ToMap renders a YANG list as a
// map of list key to entry at every count, never as an array, so the []any
// assertion the parser used found nothing whatever the operator wrote. The
// named table then stayed nil and handleIPRequest refused every session whose
// profile named a pool: config accepted at commit and refused at connect. The
// existing coverage missed it because its fixtures fed an array of entries each
// carrying a "name" field, which no producer emits.

package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestParseFullPoolConfigReadsNamedPoolMap feeds the keyed-map shape at one and
// at two entries. One entry is not a weaker case here: the list shape does not
// vary with count, so either count discriminates.
func TestParseFullPoolConfigReadsNamedPoolMap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       string
		wantPools  []string
		wantTotals []uint32
	}{
		{
			name:       "one named pool",
			data:       `{"l2tp":{"pool":{"named-pool":{"gold":{"gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5"}}}}}`,
			wantPools:  []string{"gold"},
			wantTotals: []uint32{5},
		},
		{
			name: "two named pools",
			data: `{"l2tp":{"pool":{"named-pool":{` +
				`"gold":{"gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5"},` +
				`"silver":{"gateway":"10.2.0.254","start":"10.2.0.1","end":"10.2.0.3"}}}}}`,
			wantPools:  []string{"gold", "silver"},
			wantTotals: []uint32{5, 3},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseFullPoolConfig(tc.data)
			require.NoError(t, err)
			require.True(t, result.found, "a named pool alone makes the config found")
			require.Len(t, result.namedPools, len(tc.wantPools))

			for i, name := range tc.wantPools {
				pool, ok := result.namedPools[name]
				require.True(t, ok, "pool %q is keyed by its config key", name)
				total, _, _ := pool.stats()
				assert.Equal(t, tc.wantTotals[i], total, "pool %q size", name)
			}
		})
	}
}

// TestParseFullPoolConfigReadsNamedIPv6PoolMap is the IPv6 half of the same
// defect: the prefix-delegation pools were lost by the same assertion.
func TestParseFullPoolConfigReadsNamedIPv6PoolMap(t *testing.T) {
	data := `{"l2tp":{"pool":{"named-ipv6-pool":{"v6-gold":{"block":"2001:db8:aa00::/40","delegation-length":48}}}}}`

	result, err := parseFullPoolConfig(data)
	require.NoError(t, err)
	require.True(t, result.found)
	require.Len(t, result.namedV6Pools, 1)

	gold, ok := result.namedV6Pools["v6-gold"]
	require.True(t, ok, "the v6 pool is keyed by its config key")
	total, _, _ := gold.stats()
	assert.Equal(t, uint32(256), total)
}

// TestParseFullPoolConfigRejectsAnEmptyPoolName keeps the fail-closed guard the
// array-shaped parser had. A keyed list carries its name as the map key, so an
// empty key is the only way a pool can be nameless, and a nameless pool no
// Framed-Pool value can ever select must be an error rather than a silent drop.
func TestParseFullPoolConfigRejectsAnEmptyPoolName(t *testing.T) {
	data := `{"l2tp":{"pool":{"named-pool":{"":{"gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5"}}}}}`

	_, err := parseFullPoolConfig(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "named-pool requires a name")
}

// TestHandleIPRequestAcceptsFramedPoolFromKeyedList runs the whole chain the
// operator's config takes, from the delivered JSON into the pool table and out
// through the address request a real subscriber makes. Building namedPools by
// hand would prove the lookup works while leaving the table empty in every
// running daemon, which is how this defect stayed live.
func TestHandleIPRequestAcceptsFramedPoolFromKeyedList(t *testing.T) {
	data := `{"l2tp":{"pool":{` +
		`"ipv4":{"gateway":"10.0.0.254","start":"10.0.0.1","end":"10.0.0.10"},` +
		`"named-pool":{"gold":{"gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5","dns-primary":"1.1.1.1"}}}}}`

	result, err := parseFullPoolConfig(data)
	require.NoError(t, err)

	p := &poolPlugin{}
	p.pool = result.defaultPool
	p.namedPools = result.namedPools

	const tunnelID, sessionID = 21, 22
	l2tp.StoreSessionMetadata(tunnelID, sessionID, &l2tp.AuthMetadata{FramedPool: "gold"})
	defer l2tp.ClearSessionMetadata(tunnelID, sessionID)

	got := p.handle(ppp.EventIPRequest{TunnelID: tunnelID, SessionID: sessionID, Family: ppp.AddressFamilyIPv4})

	require.True(t, got.Accept, "reject reason: %s", got.Reason)
	assert.Equal(t, netip.MustParseAddr("10.1.0.254"), got.Local, "gateway comes from the named pool")
	assert.Equal(t, netip.MustParseAddr("10.1.0.1"), got.Peer, "address comes from the named pool range")
	assert.Equal(t, netip.MustParseAddr("1.1.1.1"), got.DNSPrimary)

	_, defaultAllocated, _ := result.defaultPool.stats()
	assert.Zero(t, defaultAllocated, "the default pool must not serve a Framed-Pool session")
}
