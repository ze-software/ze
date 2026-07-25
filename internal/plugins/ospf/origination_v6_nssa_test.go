// VALIDATES: spec-ospfv3-5 Part A -- OSPFv3 NSSA Type-7 redistribution.
// PREVENTS: v6 redistribution bypassing NSSA as Type-5, malformed OSPFv3 Type-7 P-bit/FA,
// and stale translated Type-5 LSAs after the Type-7 disappears.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func newV6RedistEngine(t *testing.T, cfgJSON string) (*engine, types.RouterID) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(cfgJSON), nil)
	require.NoError(t, err)
	eng := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv6Unicast)
	eng.setConfig(cfg)
	return eng, cfg.RouterID
}

func decodeV6External(t *testing.T, eng *engine, area types.AreaID, key types.LSAKey) (ospfv3packet.ExternalLSA, bool) {
	t.Helper()
	lsa, ok := eng.lsdb.LookupLSA(area, key)
	if !ok {
		return ospfv3packet.ExternalLSA{}, false
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	require.NoError(t, err)
	body, err := decoded.DecodeExternal()
	require.NoError(t, err)
	return body, true
}

func TestOSPFv6InjectExternalNSSAType7(t *testing.T) {
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.1","areas":{"area":{"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.9"}}},"redistribute":{"connected":{"source":"connected"}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
	prefix := netip.MustParsePrefix("2001:db8:9::/64")

	require.NoError(t, eng.InjectExternal(prefix, "connected"))
	lsid := eng.redistV6[prefix.Masked()]
	body, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, lsid))
	require.True(t, ok, "pure NSSA ASBR must originate an OSPFv3 NSSA-LSA")
	got, ok := v6PrefixToNetip(body.Prefix, afIPv6Unicast)
	require.True(t, ok)
	assert.Equal(t, prefix, got)
	_, type5 := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(rid, lsid))
	assert.False(t, type5, "pure NSSA ASBR must not originate an AS-wide Type-5")
}

func TestOSPFv6InjectExternalNormalType5(t *testing.T) {
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.4","redistribute":{"connected":{"source":"connected"}}}}`)
	prefix := netip.MustParsePrefix("2001:db8:5::/64")

	require.NoError(t, eng.InjectExternal(prefix, "connected"))
	lsid := eng.redistV6[prefix.Masked()]
	body, ok := decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(rid, lsid))
	require.True(t, ok, "non-NSSA ASBR keeps AS-wide Type-5 origination")
	got, ok := v6PrefixToNetip(body.Prefix, afIPv6Unicast)
	require.True(t, ok)
	assert.Equal(t, prefix, got)
}

func TestOSPFv6InjectExternalSurvivesNSSATranslation(t *testing.T) {
	// Regression for the redistribution/translation TOCTOU race (I1): translateNSSAV6 builds a
	// keep-set from redistV6 then FlushStaleSelfLSAs-purges any self AS-External not in it. A
	// just-injected Type-5 must be in that keep-set and survive the flush; v6InjectExternal and
	// translateNSSAV6 now share nssaMu so an injection cannot land in the snapshot->flush window.
	// (A concurrent reproduction needs internal timing hooks; this guards the keep-set invariant
	// and confirms the serialized path runs without deadlock.)
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.5","redistribute":{"connected":{"source":"connected"}}}}`)
	prefix := netip.MustParsePrefix("2001:db8:1a::/64")
	require.NoError(t, eng.InjectExternal(prefix, "connected"))
	lsid := eng.redistV6[prefix.Masked()]
	if _, ok := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(rid, lsid)); !ok {
		t.Fatal("injected AS-External-LSA missing right after injection")
	}

	// The per-second NSSA translation flush must NOT purge a legitimately-injected Type-5.
	eng.translateNSSA(time.Unix(0, 0))

	if _, ok := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(rid, lsid)); !ok {
		t.Fatal("translateNSSA purged a legitimately-injected AS-External-LSA (keep-set must include redistV6)")
	}
}

func TestOSPFv6NSSAType7PbitFA(t *testing.T) {
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.2"}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	lsid := v6SummaryLSID(42)
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:42::/64"), 0)
	require.True(t, ok)
	fa := netip.MustParseAddr("2001:db8:9::1").As16()

	require.True(t, eng.v6OriginateNSSALSA(nssa, rid, lsid, pfx, false, 33, fa, true, 7, true))
	body, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, lsid))
	require.True(t, ok)
	assert.True(t, ospfv3packet.NSSAPropagate(body), "P-bit is encoded in PrefixOptions")
	assert.True(t, body.HasForwardingAddr)
	assert.Equal(t, fa, body.ForwardingAddr)
	assert.Equal(t, uint32(7), body.ExternalRouteTag)

	twinLSID := v6SummaryLSID(43)
	require.True(t, eng.v6OriginateExternalLSA(rid, twinLSID, pfx, true, 20, 0))
	require.True(t, eng.v6OriginateNSSALSA(nssa, rid, twinLSID, pfx, false, 33, fa, true, 0, true))
	body, ok = decodeV6External(t, eng, nssa, v6NSSAKey(rid, twinLSID))
	require.True(t, ok)
	assert.False(t, ospfv3packet.NSSAPropagate(body), "local Type-5 twin clears the Type-7 P-bit")
}

func TestOSPFv6NSSAWithdrawPurges(t *testing.T) {
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.3","areas":{"area":{"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.9"}}},"redistribute":{"connected":{"source":"connected"}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
	prefix := netip.MustParsePrefix("2001:db8:99::/64")
	require.NoError(t, eng.InjectExternal(prefix, "connected"))
	lsid := eng.redistV6[prefix.Masked()]

	removed, err := eng.WithdrawExternal(prefix)
	require.NoError(t, err)
	require.True(t, removed)
	lsa, ok := eng.lsdb.LookupLSA(nssa, v6NSSAKey(rid, lsid))
	require.True(t, ok, "withdraw leaves a MaxAge purge instance")
	assert.True(t, lsa.Header.Age.IsMaxAge())
}

func TestOSPFv6TranslateNSSAToType5(t *testing.T) {
	eng, self := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa","nssa":{"translate-role":"always"}}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.9"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	asbr := types.RouterID{10, 0, 9, 2}
	lsid := v6SummaryLSID(77)
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:77::/64"), 0)
	require.True(t, ok)
	pfx.Options |= ospfv3types.OptPrefixP
	fa := netip.MustParseAddr("2001:db8:9::2").As16()
	body := ospfv3packet.ExternalLSA{ExternalType2: true, Metric: 44, Prefix: pfx, ForwardingAddr: fa, HasForwardingAddr: true, ExternalRouteTag: 123, HasRouteTag: true}
	installV6NSSAForTest(t, eng, nssa, asbr, lsid, body, 1, ospfv3types.InitialSequenceNumber)

	eng.translateNSSA(transTime)
	translated, ok := decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(self, lsid))
	require.True(t, ok, "elected NSSA ABR translates P=1 Type-7 to Type-5")
	assert.False(t, ospfv3packet.NSSAPropagate(translated), "translated Type-5 clears P-bit")
	assert.Equal(t, fa, translated.ForwardingAddr)
	assert.Equal(t, uint32(44), translated.Metric)
	assert.Equal(t, uint32(123), translated.ExternalRouteTag)

	pfxNoP := pfx
	pfxNoP.Options &^= ospfv3types.OptPrefixP
	installV6NSSAForTest(t, eng, nssa, asbr, lsid, ospfv3packet.ExternalLSA{Metric: 44, Prefix: pfxNoP, ForwardingAddr: fa, HasForwardingAddr: true}, ospfv3types.MaxAge, ospfv3types.InitialSequenceNumber.Next())
	eng.translateNSSA(transTime.Add(time.Second))
	purged, ok := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(self, lsid))
	require.True(t, ok, "translation withdrawal leaves a MaxAge purge instance")
	assert.True(t, purged.Header.Age.IsMaxAge())
}

func TestOSPFv6NSSANonCandidateDoesNotWedge(t *testing.T) {
	eng, self := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.9.1","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.9":{"area-id":"0.0.0.9","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.9"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 9}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	higher := types.RouterID{10, 0, 9, 99}
	rtr := ospfv3packet.RouterLSA{
		Flags:   ospfv3packet.RouterFlagB,
		Options: ospfv3types.OptV6 | ospfv3types.OptR | ospfv3types.OptN,
	}
	require.True(t, eng.lsdb.Install(nssa, v6SelfLSA(ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:               1,
			Type:              ospfv3types.LSTypeRouter,
			AdvertisingRouter: ospfv3types.RouterID(higher),
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		Router: &rtr,
	})))

	asbr := types.RouterID{10, 0, 9, 2}
	lsid := v6SummaryLSID(79)
	pfx, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:79::/64"), 0)
	require.True(t, ok)
	pfx.Options |= ospfv3types.OptPrefixP
	fa := netip.MustParseAddr("2001:db8:9::2").As16()
	installV6NSSAForTest(t, eng, nssa, asbr, lsid, ospfv3packet.ExternalLSA{Metric: 44, Prefix: pfx, ForwardingAddr: fa, HasForwardingAddr: true}, 1, ospfv3types.InitialSequenceNumber)

	eng.translateNSSA(transTime)
	_, ok = decodeV6External(t, eng, types.BackboneArea, v6ExternalKey(self, lsid))
	require.True(t, ok, "a higher ABR with Nt clear must not suppress the v6 translator")
}

func installV6NSSAForTest(t *testing.T, eng *engine, area types.AreaID, adv types.RouterID, lsid types.LinkStateID, body ospfv3packet.ExternalLSA, age ospfv3types.LSAge, seq ospfv3types.LSSequenceNumber) {
	t.Helper()
	lsa := v6SelfLSA(ospfv3packet.LSA{
		Header:   ospfv3packet.LSAHeader{Age: age, Type: ospfv3types.LSTypeNSSA, LinkStateID: ospfv3types.LinkStateID(lsid), AdvertisingRouter: ospfv3types.RouterID(adv), Sequence: seq},
		External: &body,
	})
	require.True(t, eng.lsdb.Install(area, lsa))
}
