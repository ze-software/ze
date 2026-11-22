// Design: docs/architecture/route-selection.md -- RFC 7611 ACCEPT_OWN config default and VPN-IP decision-process tests
// Related: rib_accept_own_config.go -- extractAcceptOwnConfig, entryHasAcceptOwn, isVPNIPFamily, compareAcceptOwn
// Related: bestpath.go -- compareBeforeMED, comparePairWithReason, multipathEqual
// Related: config_tx.go -- bgp/accept-own config transaction installation
// RFC 7611 Section 2.3 and Section 3 -- see rfc/short/rfc7611.md

package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func buildAcceptOwnTestAttrs(nextHop [4]byte, localPref uint32, asPath []uint32, withAcceptOwn bool) []byte {
	var out []byte
	// ORIGIN = IGP (0)
	out = append(out, 0x40, byte(attribute.AttrOrigin), 1, 0)
	// AS_PATH (4-byte AS_SEQUENCE)
	asLen := 2 + 4*len(asPath)
	out = append(out, 0x40, byte(attribute.AttrASPath), byte(asLen), byte(attribute.ASSequence), byte(len(asPath)))
	for _, asn := range asPath {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], asn)
		out = append(out, b[:]...)
	}
	// NEXT_HOP
	out = append(out, 0x40, byte(attribute.AttrNextHop), 4)
	out = append(out, nextHop[:]...)
	// LOCAL_PREF
	var lp [4]byte
	binary.BigEndian.PutUint32(lp[:], localPref)
	out = append(out, 0x40, byte(attribute.AttrLocalPref), 4)
	out = append(out, lp[:]...)
	if withAcceptOwn {
		// COMMUNITIES: ACCEPT_OWN (0xFFFF0001) + 65001:100
		var c [8]byte
		binary.BigEndian.PutUint32(c[0:4], uint32(attribute.CommunityAcceptOwn))
		binary.BigEndian.PutUint32(c[4:8], (65001<<16)|100)
		out = append(out, 0xC0, byte(attribute.AttrCommunity), 8)
		out = append(out, c[:]...)
	}
	return out
}

func vpnv6NLRI(label uint32, rd [8]byte, pfxBytes ...byte) []byte {
	var nlri []byte
	// Bit length = 24 (label) + 64 (RD) + 8*len(pfxBytes)
	bitLen := byte(24 + 64 + 8*len(pfxBytes))
	nlri = append(nlri, bitLen)
	// Label with BoS bit set.
	nlri = append(nlri, byte(label>>12), byte(label>>4), byte((label<<4)|1))
	nlri = append(nlri, rd[:]...)
	nlri = append(nlri, pfxBytes...)
	return nlri
}

func evpnType5NLRI(rd [8]byte) []byte {
	// Type 5 IP Prefix route (length 34 bytes: RD(8)+ESI(10)+EthTag(4)+PrefixLen(1)+IP(4)+GW(4)+Label(3)).
	nlri := make([]byte, 2+34)
	nlri[0] = 5
	nlri[1] = 34
	copy(nlri[2:10], rd[:])
	nlri[24] = 24
	copy(nlri[25:29], []byte{10, 20, 30, 0})
	return nlri
}

// TestRFC7611AcceptOwnConfigDefaultsDisabled proves RFC 7611 Section 2.3:
// ACCEPT_OWN handling is controlled by configuration (bgp/accept-own) and
// defaults to disabled (false) when unconfigured, omitted on reload, or
// explicitly set false, while enabling it activates Section 3 VPN-IP selection
// and invalid non-boolean values are refused.
//
// RFC requirement: RFC7611-2.3-1 positive -- unconfigured RIBManager and ConfigTx deliveries omitting bgp/accept-own (or setting it false, or removing accept-own: true on reload) default acceptOwn to false and do not apply ACCEPT_OWN preference during VPN-IP best-path selection.
// RFC requirement: RFC7611-2.3-1 negative -- configuring bgp/accept-own true enables ACCEPT_OWN handling (flipping VPN-IP best-path selection to prefer the ACCEPT_OWN route), rollback restores disabled, and non-boolean values are refused at Verify without altering the running state.
func TestRFC7611AcceptOwnConfigDefaultsDisabled(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	tx := r.newConfigTx(false)

	// Unconfigured default is disabled (false).
	require.False(t, r.acceptOwn.Load(), "unconfigured RIB must default accept-own to disabled")

	// Stage 2 Configure omitting accept-own keeps default false.
	require.NoError(t, tx.Configure(ribSections(`"router-id":"10.0.0.1"`)))
	require.False(t, r.acceptOwn.Load(), "omitted bgp/accept-own must default to false")

	// Populate two VPN-IPv4 paths for the same RD+prefix with equal LOCAL_PREF=100:
	//   peer 1: ACCEPT_OWN present, longer AS_PATH [65010, 65011, 65012]
	//   peer 2: no ACCEPT_OWN, shorter AS_PATH [65020]
	pe1 := netip.MustParseAddr("192.0.2.1")
	pe2 := netip.MustParseAddr("192.0.2.2")
	electionFixtureLocRIB(t, r, pe1, pe2)
	r.peerMu.Lock()
	r.peerMeta[1] = &peerMetadata{Address: pe1, PeerASN: 65000, LocalASN: 65000}
	r.peerMeta[2] = &peerMetadata{Address: pe2, PeerASN: 65000, LocalASN: 65000}
	pr1, err := storage.NewAddressPeerRIB(pe1)
	require.NoError(t, err)
	pr2, err := storage.NewAddressPeerRIB(pe2)
	require.NoError(t, err)
	r.bgpPeers[1] = pr1
	r.bgpPeers[2] = pr2
	r.peerMu.Unlock()

	key := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	pr1.Insert(vpnv4Family, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 1}, 100, []uint32{65010, 65011, 65012}, true), key)
	pr2.Insert(vpnv4Family, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 2}, 100, []uint32{65020}, false), key)

	// Positive (default disabled): shorter AS_PATH (peer 2) wins because ACCEPT_OWN is disabled.
	candsDefault := gatherCandidatesHeld(t, r, vpnv4Family, key, false)
	require.Len(t, candsDefault, 2)
	bestDefault := SelectBest(candsDefault)
	require.NotNil(t, bestDefault)
	require.Equal(t, pe2, bestDefault.PeerIP, "when accept-own defaults disabled, shorter AS_PATH must win over ACCEPT_OWN")

	// Negative 1: enabling bgp/accept-own true activates ACCEPT_OWN handling so peer 1 wins.
	require.NoError(t, tx.Verify(ribSections(`"router-id":"10.0.0.1","accept-own":true`)))
	require.NoError(t, tx.Apply())
	require.True(t, r.acceptOwn.Load())

	candsEnabled := gatherCandidatesHeld(t, r, vpnv4Family, key, false)
	require.Len(t, candsEnabled, 2)
	bestEnabled := SelectBest(candsEnabled)
	require.NotNil(t, bestEnabled)
	require.Equal(t, pe1, bestEnabled.PeerIP, "when accept-own is configured true, ACCEPT_OWN path must win")

	// Reload removing accept-own restores the RFC 7611 §2.3 default (false).
	require.NoError(t, tx.Verify(ribSections(`"router-id":"10.0.0.1"`)))
	require.NoError(t, tx.Apply())
	require.False(t, r.acceptOwn.Load(), "removing accept-own on reload must restore default false")

	// Rollback restores enabled (true).
	require.NoError(t, tx.Rollback())
	require.True(t, r.acceptOwn.Load())

	// Explicit false disables it again.
	require.NoError(t, tx.Verify(ribSections(`"router-id":"10.0.0.1","accept-own":"false"`)))
	require.NoError(t, tx.Apply())
	require.False(t, r.acceptOwn.Load())

	// Negative 2: non-boolean values are refused at Verify without changing running state.
	for _, invalid := range []string{`"maybe"`, `123`, `[]`} {
		err := tx.Verify(ribSections(`"accept-own":` + invalid))
		require.Error(t, err, "invalid accept-own %s must be refused", invalid)
		require.Contains(t, err.Error(), "accept-own")
		require.ErrorIs(t, tx.Apply(), sdk.ErrConfigApplyUnverified)
		require.False(t, r.acceptOwn.Load())
	}
}

// TestRFC7611DecisionStepOnlyForVPNIPRoutes proves RFC 7611 Section 3:
// when ACCEPT_OWN is enabled by configuration, the post-LOCAL_PREF ACCEPT_OWN
// preference step MUST ONLY be invoked during best-path selection of VPN-IP
// routes (RFC 4364: ipv4/vpn and ipv6/vpn), and MUST NOT be invoked for
// non-VPN-IP routes such as imported IPv4/IPv6 unicast routes in a VRF or EVPN.
//
// RFC requirement: RFC7611-3-1 positive -- with bgp/accept-own enabled, for VPN-IPv4 and VPN-IPv6 routes with tied LOCAL_PREF, the candidate carrying ACCEPT_OWN beats a shorter-AS_PATH candidate without ACCEPT_OWN (explained as BestStepAcceptOwn and excluded from equal-cost multipath), while higher LOCAL_PREF still beats ACCEPT_OWN.
// RFC requirement: RFC7611-3-1 negative -- with bgp/accept-own enabled, for non-VPN-IP families (IPv4 unicast, IPv6 unicast representing imported IP routes in a VRF, and EVPN), a candidate carrying ACCEPT_OWN is NOT preferred over a non-ACCEPT_OWN candidate with a shorter AS_PATH (explained as BestStepASPathLen, never BestStepAcceptOwn), and equal-attribute IP unicast paths remain multipath-eligible.
func TestRFC7611DecisionStepOnlyForVPNIPRoutes(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	tx := r.newConfigTx(false)
	require.NoError(t, tx.Configure(ribSections(`"router-id":"10.0.0.1","accept-own":true`)))
	require.True(t, r.acceptOwn.Load())

	pe1 := netip.MustParseAddr("192.0.2.1")
	pe2 := netip.MustParseAddr("192.0.2.2")
	electionFixtureLocRIB(t, r, pe1, pe2)
	r.peerMu.Lock()
	r.peerMeta[1] = &peerMetadata{Address: pe1, PeerASN: 65000, LocalASN: 65000}
	r.peerMeta[2] = &peerMetadata{Address: pe2, PeerASN: 65000, LocalASN: 65000}
	pr1, err := storage.NewAddressPeerRIB(pe1)
	require.NoError(t, err)
	pr2, err := storage.NewAddressPeerRIB(pe2)
	require.NoError(t, err)
	r.bgpPeers[1] = pr1
	r.bgpPeers[2] = pr2
	r.peerMu.Unlock()

	// Positive cases: RFC 4364 VPN-IPv4 and VPN-IPv6 routes invoke the ACCEPT_OWN step after LOCAL_PREF.
	vpnv6Fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	vpnCases := []struct {
		name string
		fam  family.Family
		key  []byte
	}{
		{"ipv4_vpn", vpnv4Family, vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)},
		{"ipv6_vpn", vpnv6Fam, vpnv6NLRI(200, vpnRouteKeyRD, 0x20, 0x01, 0x0d, 0xb8)},
	}
	for _, tc := range vpnCases {
		t.Run("positive_"+tc.name, func(t *testing.T) {
			// Equal LOCAL_PREF (100): peer 1 has ACCEPT_OWN + longer AS_PATH [65010, 65011, 65012];
			// peer 2 has no ACCEPT_OWN + shorter AS_PATH [65020].
			pr1.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 1}, 100, []uint32{65010, 65011, 65012}, true), tc.key)
			pr2.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 2}, 100, []uint32{65020}, false), tc.key)

			cands := gatherCandidatesHeld(t, r, tc.fam, tc.key, false)
			require.Len(t, cands, 2)
			best := SelectBest(cands)
			require.NotNil(t, best)
			require.Equal(t, pe1, best.PeerIP, "VPN-IP route with ACCEPT_OWN must beat shorter AS_PATH on tied LOCAL_PREF")

			exp := SelectBestExplain(gatherCandidatesHeld(t, r, tc.fam, tc.key, false))
			require.NotNil(t, exp)
			require.Equal(t, pe1, exp.Winner.PeerIP)
			require.Len(t, exp.Steps, 1)
			require.Equal(t, BestStepAcceptOwn, exp.Steps[0].Step)

			// Equal AS_PATH too: multipath must NOT treat ACCEPT_OWN and non-ACCEPT_OWN VPN-IP paths as equal cost.
			pr1.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 1}, 100, []uint32{65020}, true), tc.key)
			primary, siblings := SelectMultipath(gatherCandidatesHeld(t, r, tc.fam, tc.key, false), 4, false)
			require.NotNil(t, primary)
			require.Equal(t, pe1, primary.PeerIP)
			require.Empty(t, siblings, "non-ACCEPT_OWN VPN-IP path is not equal-cost with ACCEPT_OWN path")

			// Higher LOCAL_PREF (200) on peer 2 STILL beats peer 1's ACCEPT_OWN (step is AFTER LOCAL_PREF).
			pr2.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 2}, 200, []uint32{65020, 65021, 65022}, false), tc.key)
			bestLP := SelectBest(gatherCandidatesHeld(t, r, tc.fam, tc.key, false))
			require.NotNil(t, bestLP)
			require.Equal(t, pe2, bestLP.PeerIP, "higher LOCAL_PREF must beat ACCEPT_OWN")
		})
	}

	// Negative cases: non-VPN-IP families (imported IPv4/IPv6 unicast routes in a VRF, and EVPN)
	// MUST NOT invoke the ACCEPT_OWN step even when bgp/accept-own is true and ACCEPT_OWN is attached.
	nonVPNCases := []struct {
		name string
		fam  family.Family
		key  []byte
	}{
		{"ipv4_unicast_vrf_imported", family.IPv4Unicast, []byte{24, 198, 51, 100}},
		{"ipv6_unicast_vrf_imported", family.IPv6Unicast, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}},
		{"l2vpn_evpn", evpnFamily, evpnType5NLRI(vpnRouteKeyRD)},
	}
	for _, tc := range nonVPNCases {
		t.Run("negative_"+tc.name, func(t *testing.T) {
			// Equal LOCAL_PREF (100): peer 1 carries ACCEPT_OWN with longer AS_PATH [65010, 65011, 65012];
			// peer 2 has no ACCEPT_OWN with shorter AS_PATH [65020].
			pr1.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 1}, 100, []uint32{65010, 65011, 65012}, true), tc.key)
			pr2.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 2}, 100, []uint32{65020}, false), tc.key)

			cands := gatherCandidatesHeld(t, r, tc.fam, tc.key, false)
			require.Len(t, cands, 2)
			best := SelectBest(cands)
			require.NotNil(t, best)
			require.Equal(t, pe2, best.PeerIP, "ACCEPT_OWN step MUST NOT be invoked for %s", tc.fam)

			exp := SelectBestExplain(gatherCandidatesHeld(t, r, tc.fam, tc.key, false))
			require.NotNil(t, exp)
			require.Equal(t, pe2, exp.Winner.PeerIP)
			require.Len(t, exp.Steps, 1)
			require.Equal(t, BestStepASPathLen, exp.Steps[0].Step, "non-VPN-IP route must be decided by AS_PATH, never ACCEPT_OWN")

			// With equal AS_PATH too, non-VPN-IP paths remain equal-cost for multipath (ACCEPT_OWN ignored).
			pr1.Insert(tc.fam, buildAcceptOwnTestAttrs([4]byte{192, 0, 2, 1}, 100, []uint32{65020}, true), tc.key)
			primary, siblings := SelectMultipath(gatherCandidatesHeld(t, r, tc.fam, tc.key, false), 4, false)
			require.NotNil(t, primary)
			require.Len(t, siblings, 1, "non-VPN-IP paths with identical non-ACCEPT_OWN attributes must remain multipath equal-cost")
		})
	}
}
