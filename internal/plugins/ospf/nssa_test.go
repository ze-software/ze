// VALIDATES: spec-ospf-11 RFC 3101 sec 3.5/3.6 -- translator election (highest Router
// ID among candidate ABRs; always/never roles) and Type 7 -> Type 5 translation (P
// cleared, Advertising Router = translator, forwarding address / metric / tag preserved,
// withdrawn when the source Type 7 disappears; P=0 / zero-FA not translated; only the
// elected translator translates, so no duplicate Type 5).
// PREVENTS: regressions where every NSSA ABR translates (duplicate Type 5), a P=0 Type
// 7 is translated, the forwarding address is dropped, or a stale translation lingers.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// transTime is a fixed evaluation time for translation tests that do not exercise the
// stability grace.
var transTime = time.Unix(1000, 0)

func ridOf(s string) types.RouterID { return types.RouterID(netip.MustParseAddr(s).As4()) }
func ip4Of(s string) [4]byte        { return netip.MustParseAddr(s).As4() }

func TestOSPFNSSATranslatorElection(t *testing.T) {
	self := ridOf("10.0.0.5")
	higher := ridOf("10.0.0.9")
	lower := ridOf("10.0.0.1")

	// RFC requirement: RFC3101-3.1-2 positive -- a candidate ABR with the highest Router ID
	// among the reachable candidates is elected translator.
	assert.True(t, electNSSATranslator(self, translateRoleCandidate, []types.RouterID{self, lower}), "candidate with the highest Router ID translates")
	// RFC requirement: RFC3101-3.1-2 negative -- a candidate defers to a reachable candidate
	// ABR with a higher Router ID and is not elected.
	assert.False(t, electNSSATranslator(self, translateRoleCandidate, []types.RouterID{self, higher}), "candidate defers to a higher Router ID")
	assert.True(t, electNSSATranslator(self, translateRoleAlways, []types.RouterID{self, higher}), "always translates regardless of Router ID")
	assert.False(t, electNSSATranslator(self, translateRoleNever, []types.RouterID{self}), "never does not translate even as the only ABR")
	assert.True(t, electNSSATranslator(self, translateRoleCandidate, nil), "the only/first candidate translates")
}

// nssaTransEngine builds an engine attached to the backbone + an NSSA, with the
// running interfaces simulated (no real sockets).
func nssaTransEngine(t *testing.T, routerID string) (*engine, types.AreaID) {
	t.Helper()
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"`+routerID+`","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	return eng, nssa
}

func type5(t *testing.T, eng *engine, self types.RouterID, network string) (bool, [4]byte) {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4Of(network)), AdvertisingRouter: self}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		return false, [4]byte{}
	}
	body, err := lsa.DecodeExternal()
	require.NoError(t, err)
	return true, body.ForwardingAddr
}

func TestOSPFNSSATranslation(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.6.9") // high Router ID -> elected translator
	self := ridOf("10.0.6.9")
	asbr := ridOf("10.0.6.2")

	// A P=1, non-zero-FA Type 7 from an internal NSSA ASBR.
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.20.0.0"), ip4Of("255.255.0.0"), false, 33, ip4Of("10.5.0.2"), 7, true)
	eng.translateNSSA(transTime)

	ok, fa := type5(t, eng, self, "10.20.0.0")
	require.True(t, ok, "the elected translator re-originates a Type 5")
	assert.Equal(t, ip4Of("10.5.0.2"), fa, "the source forwarding address is preserved")

	// RFC 3101 sec 3.2: the translated Type 5 preserves metric / metric-type / route tag and
	// clears the P-bit (Type 5 has no NSSA P-bit; the LSA carries only OptionE).
	// RFC requirement: RFC3101-3.2-1 positive -- the translated Type-5 sets the advertising
	// router to the translator's Router ID (the lookup key uses self) and preserves the source
	// Type-7's mask/path-type/metric/forwarding-address/route-tag.
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4Of("10.20.0.0")), AdvertisingRouter: self}
	t5, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	require.True(t, ok, "translated Type 5 present in the AS-external store")
	assert.False(t, t5.Header.Options.Has(types.OptionNP), "translated Type 5 clears the NSSA P-bit")
	body, err := t5.DecodeExternal()
	require.NoError(t, err)
	assert.Equal(t, uint32(33), body.Metric, "source metric preserved")
	assert.False(t, body.ExternalType2, "source metric-type (E1) preserved")
	assert.Equal(t, uint32(7), body.ExternalRouteTag, "source route tag preserved")

	// Withdraw the source Type 7 -> the translated Type 5 is purged.
	// RFC requirement: RFC3101-3.2-1 negative -- when the source Type-7 is withdrawn, the
	// translated Type-5 is not retained; it is withdrawn (no translation is fabricated).
	eng.lsdb.PurgeNSSA(nssa, asbr, ip4Of("10.20.0.0"))
	eng.translateNSSA(transTime)
	require.Equal(t, 0, eng.lsdb.SelfExternalCount(self), "translation withdrawn once the source Type 7 is gone")
}

// TestOSPFNSSATranslationStoreFull pins the NSSA-translation sibling of the ospf-10
// store-full finding: when OriginateExternal rejects a translated Type 5 because the
// AS-external store is at capacity, applyTranslations must NOT count it
// (ze_ospf_nssa_translations_total) and must NOT record it as translated. Recording it (the
// bug) over-reports the metric and lets a translation that never reached the backbone
// masquerade as done; skipping it lets a later tick re-attempt and count it once the store
// frees. The recorded-as-translated symptom is the buggy-vs-fixed discriminator here.
func TestOSPFNSSATranslationStoreFull(t *testing.T) {
	savedCap := ospflsdb.MaxASExternalLSAs
	ospflsdb.MaxASExternalLSAs = 1 // a single unrelated Type 5 fills the AS-external store
	defer func() { ospflsdb.MaxASExternalLSAs = savedCap }()

	eng, nssa := nssaTransEngine(t, "10.0.6.9") // high Router ID -> elected translator
	self := ridOf("10.0.6.9")
	asbr := ridOf("10.0.6.2")

	// Fill the AS-external store to capacity with an unrelated self Type 5.
	_, _, err := eng.lsdb.OriginateExternal(self, ip4Of("10.200.0.0"), ip4Of("255.255.0.0"), types.OptionE, true, 20, ip4Of("0.0.0.0"), 0)
	require.NoError(t, err, "the first Type 5 fits the size-1 store")

	// A P=1, non-zero-FA Type 7 the elected translator would translate -- but the store is
	// now full, so OriginateExternal returns ErrExternalStoreFull for the translated Type 5.
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.20.0.0"), ip4Of("255.255.0.0"), false, 33, ip4Of("10.5.0.2"), 7, true)
	eng.translateNSSA(transTime)

	if ok, _ := type5(t, eng, self, "10.20.0.0"); ok {
		t.Fatalf("translated Type 5 must not be installed when the AS-external store is full")
	}
	if _, recorded := eng.translations[ip4Of("10.20.0.0")]; recorded {
		t.Fatalf("a store-full-rejected translation must not be recorded as translated " +
			"(recording it over-counts the metric and hides the never-installed Type 5)")
	}
}

func TestOSPFNSSAPbitNotTranslated(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.6.9")
	self := ridOf("10.0.6.9")
	asbr := ridOf("10.0.6.2")

	// RFC requirement: RFC3101-3.2-1 negative -- a P=0 Type-7 and a P=1 Type-7 with a zero
	// forwarding address are not translated into a Type-5 (there is nothing to preserve).
	// P=0 Type 7 -> not translated.
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.21.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, false)
	// P=1 but zero forwarding address -> not translatable.
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.22.0.0"), ip4Of("255.255.0.0"), false, 10, [4]byte{}, 0, true)
	eng.translateNSSA(transTime)

	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(self), "neither a P=0 nor a zero-FA Type 7 is translated")
}

func TestOSPFNSSATranslatorStability(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.7.9") // initially the only ABR -> elected
	self := ridOf("10.0.7.9")
	asbr := ridOf("10.0.7.2")
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.30.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)

	t0 := time.Unix(2000, 0)
	eng.translateNSSA(t0)
	require.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "elected translator translates")

	// A higher-Router-ID translator-candidate ABR appears (Nt-bit set): self loses the
	// election but keeps translating for the stability interval (default 40s), so a
	// transient flap opens no Type 5 gap.
	eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: ridOf("10.0.7.99"), ABR: true, NSSATranslator: true})
	eng.translateNSSA(t0.Add(10 * time.Second))
	assert.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "keeps translating during the stability grace")

	// Past the stability interval (grace started at +10s), self yields and withdraws.
	eng.translateNSSA(t0.Add(60 * time.Second))
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(self), "yields translation after the stability interval elapses")
}

// TestEngineNSSATranslationSkipsRedistributed is the regression for the shared Type 5
// key: a router that both redistributes a network AND is the NSSA translator must NOT
// translate a peer's Type 7 for that same network (RFC 3101 §3.6 keeps the local Type
// 5), and a peer's Type 7 withdrawal must never purge the redistributed Type 5.
func TestEngineNSSATranslationSkipsRedistributed(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.8.9") // R: ABR + elected translator
	self := ridOf("10.0.8.9")
	q := ridOf("10.0.8.2")

	// R redistributes network X as a Type 5 (Forwarding Address 0).
	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.50.0.0/24"), "connected"))
	require.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "R's redistributed Type 5 for X")

	// Q (an NSSA-internal ASBR) advertises a P=1 Type 7 for the SAME network X.
	eng.lsdb.OriginateNSSA(nssa, q, ip4Of("10.50.0.0"), ip4Of("255.255.255.0"), false, 5, ip4Of("10.5.0.2"), 0, true)
	eng.translateNSSA(transTime)

	require.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "translator does not also translate a network it redistributes")
	body, ok := externalBody(t, eng, self, "10.50.0.0/24")
	require.True(t, ok)
	assert.Equal(t, [4]byte{}, body.ForwardingAddr, "the surviving Type 5 is R's redistribution (FA 0), not the translation's FA")

	// Q withdraws its Type 7 -> R's redistributed Type 5 must survive (no blackhole).
	eng.lsdb.PurgeNSSA(nssa, q, ip4Of("10.50.0.0"))
	eng.translateNSSA(transTime)
	assert.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "redistributed Type 5 survives the peer Type 7 withdrawal")
}

func TestOSPFNSSANoTranslateWhenNotElected(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.6.1") // low Router ID
	self := ridOf("10.0.6.1")
	asbr := ridOf("10.0.6.2")

	// A higher-Router-ID translator-candidate ABR (Nt-bit set) is present in the NSSA ->
	// self (candidate) must not translate.
	eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: ridOf("10.0.6.9"), ABR: true, NSSATranslator: true})
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.23.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
	eng.translateNSSA(transTime)

	// RFC requirement: RFC3101-3.1-2 negative -- with a reachable higher-Router-ID Nt-capable
	// ABR present, a candidate self is not elected and does not translate.
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(self), "a non-elected candidate ABR does not translate (no duplicate Type 5)")
}

// TestOSPFNSSANonCandidateDoesNotWedge is the RFC 3101 §3.5 regression for the translator
// election: a higher-Router-ID ABR that advertises NO Nt-bit (e.g. configured `translate
// never`) is not a translator candidate, so it must not suppress a willing lower-Router-ID
// candidate. Before the Nt-bit filter the higher RID alone wedged translation off,
// blackholing the NSSA's externals outside the area.
func TestOSPFNSSANonCandidateDoesNotWedge(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.6.1") // low Router ID, candidate
	self := ridOf("10.0.6.1")
	asbr := ridOf("10.0.6.2")

	// A higher-Router-ID ABR present but NOT a translator candidate (Nt-bit clear).
	eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: ridOf("10.0.6.9"), ABR: true})
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of("10.24.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
	eng.translateNSSA(transTime)

	// RFC requirement: RFC3101-3.1-2 positive -- election counts only Nt-advertising (candidate)
	// ABRs, so a willing lower-Router-ID candidate is elected despite a higher-Router-ID
	// non-candidate ABR.
	assert.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "the willing lower-Router-ID candidate translates despite a higher-RID non-candidate ABR")
}
