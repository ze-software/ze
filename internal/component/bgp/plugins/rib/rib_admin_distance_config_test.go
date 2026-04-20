package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
)

// TestExtractAdminDistanceConfigPresent verifies that a bgp/admin-distance
// container with both fields populated is parsed correctly.
//
// VALIDATES: admin-distance reaches the RIB plugin through the Stage 2
// configure callback.
// PREVENTS: Admin-distance config silently dropped between the editor and
// the RIB plugin so the default 20/200 are always stamped.
func TestExtractAdminDistanceConfigPresent(t *testing.T) {
	jsonStr := `{"bgp":{"admin-distance":{"ebgp":40,"ibgp":180}}}`
	ebgp, ibgp := extractAdminDistanceConfig(jsonStr)
	assert.Equal(t, uint8(40), ebgp)
	assert.Equal(t, uint8(180), ibgp)
}

// TestExtractAdminDistanceConfigMissing verifies that an absent
// admin-distance container returns zero values so the caller retains
// its current defaults.
//
// VALIDATES: No admin-distance config means the RFC-free Cisco/Juniper
// default (20/200) stays in place.
// PREVENTS: Unintended override of admin distance on daemons that never
// configured it.
func TestExtractAdminDistanceConfigMissing(t *testing.T) {
	jsonStr := `{"bgp":{"router-id":"10.0.0.1"}}`
	ebgp, ibgp := extractAdminDistanceConfig(jsonStr)
	assert.Equal(t, uint8(0), ebgp)
	assert.Equal(t, uint8(0), ibgp)
}

// TestExtractAdminDistanceConfigDefault verifies that the YANG default
// values round-trip through the JSON tree correctly.
//
// VALIDATES: YANG default 20/200 survives the config serialize-then-parse
// pipeline.
// PREVENTS: Missing/zero values when the config has only default leaves.
func TestExtractAdminDistanceConfigDefault(t *testing.T) {
	jsonStr := `{"bgp":{"admin-distance":{"ebgp":20,"ibgp":200}}}`
	ebgp, ibgp := extractAdminDistanceConfig(jsonStr)
	assert.Equal(t, uint8(20), ebgp)
	assert.Equal(t, uint8(200), ibgp)
}

// TestExtractAdminDistanceConfigBoundary verifies the last-valid and
// first-invalid values for the YANG 1..255 range.
//
// VALIDATES: YANG range boundary handling — 1 and 255 accepted, 0 and 256
// rejected. Fractional floats are rejected (no silent truncation).
// PREVENTS: Silent truncation of max (255) or acceptance of 0/256, and
// silent truncation of a fractional float on the plugin-IPC path.
func TestExtractAdminDistanceConfigBoundary(t *testing.T) {
	tests := []struct {
		name string
		json string
		ebgp uint8
		ibgp uint8
	}{
		{"lower valid", `{"bgp":{"admin-distance":{"ebgp":1,"ibgp":1}}}`, 1, 1},
		{"upper valid", `{"bgp":{"admin-distance":{"ebgp":255,"ibgp":255}}}`, 255, 255},
		{"zero rejected", `{"bgp":{"admin-distance":{"ebgp":0,"ibgp":0}}}`, 0, 0},
		{"above max rejected", `{"bgp":{"admin-distance":{"ebgp":256,"ibgp":256}}}`, 0, 0},
		{"negative rejected", `{"bgp":{"admin-distance":{"ebgp":-1,"ibgp":-1}}}`, 0, 0},
		{"non-numeric rejected", `{"bgp":{"admin-distance":{"ebgp":"abc","ibgp":"xyz"}}}`, 0, 0},
		{"fractional rejected", `{"bgp":{"admin-distance":{"ebgp":20.5,"ibgp":199.9}}}`, 0, 0},
		{"integer-valued float accepted", `{"bgp":{"admin-distance":{"ebgp":20.0,"ibgp":200.0}}}`, 20, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ebgp, ibgp := extractAdminDistanceConfig(tt.json)
			assert.Equal(t, tt.ebgp, ebgp, "ebgp")
			assert.Equal(t, tt.ibgp, ibgp, "ibgp")
		})
	}
}

// TestExtractAdminDistanceConfigStringNumber verifies that values
// serialized as numeric strings (some tree->JSON paths) still parse.
//
// VALIDATES: Extractor robustness against config serialization formats.
// PREVENTS: Plugin silently falling back to defaults when the round-trip
// shape changes.
func TestExtractAdminDistanceConfigStringNumber(t *testing.T) {
	jsonStr := `{"bgp":{"admin-distance":{"ebgp":"30","ibgp":"150"}}}`
	ebgp, ibgp := extractAdminDistanceConfig(jsonStr)
	assert.Equal(t, uint8(30), ebgp)
	assert.Equal(t, uint8(150), ibgp)
}

// TestExtractAdminDistanceConfigPartial verifies that only one of the two
// fields being set returns 0 for the missing one, so the caller preserves
// the existing default for the absent side.
//
// VALIDATES: Per-field extraction independence.
// PREVENTS: A single-field override accidentally clobbering the other.
func TestExtractAdminDistanceConfigPartial(t *testing.T) {
	ebgp, ibgp := extractAdminDistanceConfig(`{"bgp":{"admin-distance":{"ebgp":50}}}`)
	assert.Equal(t, uint8(50), ebgp)
	assert.Equal(t, uint8(0), ibgp)

	ebgp, ibgp = extractAdminDistanceConfig(`{"bgp":{"admin-distance":{"ibgp":180}}}`)
	assert.Equal(t, uint8(0), ebgp)
	assert.Equal(t, uint8(180), ibgp)
}

// TestAdminDistanceDefaults verifies that a freshly constructed RIBManager
// has the classical Cisco/Juniper defaults pre-loaded.
//
// VALIDATES: NewRIBManager seeds 20/200 so any best-path mirror fired
// before Stage 2 configure delivery stamps sane values.
// PREVENTS: AdminDistance atomic zero-value (0) reaching locrib and
// beating every other protocol.
func TestAdminDistanceDefaults(t *testing.T) {
	r := newTestRIBManager(t)
	assert.Equal(t, uint32(20), r.adminDistanceEBGP.Load())
	assert.Equal(t, uint32(200), r.adminDistanceIBGP.Load())
}

// TestAdminDistanceAppliedEBGP verifies that a configured eBGP distance
// is stamped onto the locrib.Path for a route learned from an external
// peer.
//
// VALIDATES: Configured admin-distance/ebgp flows from the atomic into
// locrib.Path.AdminDistance on the best-path mirror.
// PREVENTS: Config changes being silently ignored by checkBestPathChange.
func TestAdminDistanceAppliedEBGP(t *testing.T) {
	r := newTestRIBManager(t)
	r.adminDistanceEBGP.Store(40)

	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	_, ok := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok)

	best, found := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	require.True(t, found)
	assert.Equal(t, uint8(40), best.AdminDistance,
		"eBGP distance must match configured value, not hardcoded 20")
}

// TestAdminDistanceAppliedIBGP verifies that a configured iBGP distance
// is stamped when the peer and local ASN match.
//
// VALIDATES: Configured admin-distance/ibgp flows through checkBestPathChange.
// PREVENTS: eBGP path (isEBGP==true branch) being the only one honored.
func TestAdminDistanceAppliedIBGP(t *testing.T) {
	r := newTestRIBManager(t)
	r.adminDistanceIBGP.Store(150)

	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	peerAddr := "192.0.2.2"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65000, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 1, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 2})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	_, ok := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok)

	best, found := loc.Best(fam, netip.MustParsePrefix("10.1.0.0/24"))
	require.True(t, found)
	assert.Equal(t, uint8(150), best.AdminDistance,
		"iBGP distance must match configured value, not hardcoded 200")
}
