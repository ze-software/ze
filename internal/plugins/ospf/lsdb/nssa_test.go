// VALIDATES: spec-ospf-11 RFC 3101 sec 2 -- OriginateNSSA builds a Type 7 NSSA-LSA in
// the NSSA area store (body = Type 5 body: mask / E-bit+metric / forwarding address /
// tag), carries the P (propagate) bit in the LSA-header Options, never leaks into the
// AS-wide Type 5 store, and PurgeNSSA MaxAge-purges it while the private
// self-LSA count tracks the per-area Type 7 inventory.
// PREVENTS: regressions where a Type 7 lands in the wrong store, loses the P-bit, drops
// the forwarding address, or a withdraw leaves a stale Type 7.
package lsdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func nssaKey(network [4]byte, router types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSTypeNSSA, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
}

func TestOSPFType7Origination(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")
	nssa := area("0.0.0.1")

	_, ok := db.OriginateNSSA(nssa, router, ip4("10.70.0.0"), ip4("255.255.0.0"), false, 25, ip4("10.0.0.9"), 42, true)
	require.True(t, ok, "OriginateNSSA returned false")

	lsa, ok := db.LookupLSA(nssa, nssaKey(ip4("10.70.0.0"), router))
	require.True(t, ok, "Type 7 must be in the NSSA area store")
	assert.True(t, lsa.Header.Options.Has(types.OptionNP), "P-bit set when propagate=true")
	body, err := lsa.DecodeExternal()
	require.NoError(t, err)
	assert.Equal(t, ip4("255.255.0.0"), body.NetworkMask)
	assert.False(t, body.ExternalType2, "type-1 external (E-bit clear)")
	assert.Equal(t, uint32(25), body.Metric)
	assert.Equal(t, ip4("10.0.0.9"), body.ForwardingAddr, "non-zero intra-NSSA forwarding address preserved")
	assert.Equal(t, uint32(42), body.ExternalRouteTag)

	// Type 7 is NSSA-scoped: it must never enter the AS-wide Type 5 store.
	_, ok = db.LookupLSA(types.BackboneArea, types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4("10.70.0.0")), AdvertisingRouter: router})
	assert.False(t, ok, "Type 7 must not leak into the AS-wide Type 5 store")

	assert.Equal(t, 1, db.selfNSSACount(nssa, router), "one self Type 7 in the NSSA")

	// A P=0 origination clears the P-bit (not translated).
	_, ok = db.OriginateNSSA(nssa, router, ip4("10.71.0.0"), ip4("255.255.255.0"), false, 10, ip4("10.0.0.9"), 0, false)
	require.True(t, ok)
	lsa0, ok := db.LookupLSA(nssa, nssaKey(ip4("10.71.0.0"), router))
	require.True(t, ok)
	assert.False(t, lsa0.Header.Options.Has(types.OptionNP), "P-bit clear when propagate=false")
	assert.Equal(t, 2, db.selfNSSACount(nssa, router))
}

func TestOSPFType7Withdraw(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")
	nssa := area("0.0.0.1")

	db.OriginateNSSA(nssa, router, ip4("10.70.0.0"), ip4("255.255.0.0"), false, 25, ip4("10.0.0.9"), 0, true)
	require.Equal(t, 1, db.selfNSSACount(nssa, router))

	assert.True(t, db.PurgeNSSA(nssa, router, ip4("10.70.0.0")), "an originated Type 7 is reported removed")
	assert.Equal(t, 0, db.selfNSSACount(nssa, router), "no non-purged Type 7 after withdraw")
	assert.False(t, db.PurgeNSSA(nssa, router, ip4("10.99.0.0")), "purging a never-originated Type 7 is a no-op")
}

func TestOSPFType7FloodScope(t *testing.T) {
	// A Type 7 originated into an NSSA is eligible to flood only out an interface in
	// that same NSSA, never out a backbone/normal or different-area interface.
	nssa := area("0.0.0.1")
	inNSSA := InterfaceInfo{AreaID: nssa, AreaType: AreaTypeNSSA}
	if !eligibleInterface(inNSSA, nssa, types.LSTypeNSSA) {
		t.Fatal("Type 7 not eligible out its own NSSA interface")
	}
	backbone := InterfaceInfo{AreaID: types.BackboneArea, AreaType: AreaTypeNormal}
	if eligibleInterface(backbone, nssa, types.LSTypeNSSA) {
		t.Fatal("Type 7 leaked out a backbone interface")
	}
}
