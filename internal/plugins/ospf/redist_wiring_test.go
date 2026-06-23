// VALIDATES: spec-ospf-10 engine ExternalInjector -- InjectExternal originates a
// Type 5 with the per-source metric/metric-type/tag from cfg.Redistribute (default
// metric 20 / type-2 when the source is unconfigured); WithdrawExternal MaxAge-purges.
// PREVENTS: regressions where redistribution ignores the configured external metric/
// type/tag, or a withdraw leaves a stale Type 5.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func newRedistEngine(t *testing.T, cfgJSON string) (*engine, types.RouterID) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(cfgJSON), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	return eng, cfg.RouterID
}

func externalBody(t *testing.T, eng *engine, rid types.RouterID, prefix string) (packet.ExternalLSA, bool) {
	t.Helper()
	pfx := netip.MustParsePrefix(prefix)
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(pfx.Addr().As4()), AdvertisingRouter: rid}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		return packet.ExternalLSA{}, false
	}
	body, err := lsa.DecodeExternal()
	require.NoError(t, err)
	return body, true
}

func selfNSSACount(eng *engine, area types.AreaID, router types.RouterID) int {
	count := 0
	for _, h := range eng.lsdb.Summary(area) {
		if h.Type == types.LSTypeNSSA && h.AdvertisingRouter == router && !h.Age.IsMaxAge() {
			count++
		}
	}
	return count
}

func TestEngineInjectExternalConfiguredParams(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"connected":{"source":"connected","metric":"33","metric-type":"type-1","tag":"7"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.5.0.0/24"), "connected"))
	body, ok := externalBody(t, eng, rid, "10.5.0.0/24")
	require.True(t, ok, "Type 5 originated for the injected prefix")
	assert.Equal(t, uint32(33), body.Metric, "metric from cfg.Redistribute")
	assert.False(t, body.ExternalType2, "metric-type type-1 -> E1 (ExternalType2 false)")
	assert.Equal(t, uint32(7), body.ExternalRouteTag, "route tag from cfg.Redistribute")
	assert.Equal(t, [4]byte{255, 255, 255, 0}, body.NetworkMask)
}

func TestEngineInjectExternalDefaultParams(t *testing.T) {
	// "static" is not enrolled in cfg.Redistribute, so the engine falls back to the
	// code default (metric 20, type-2).
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"connected":{"source":"connected"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("192.0.2.0/24"), "static"))
	body, ok := externalBody(t, eng, rid, "192.0.2.0/24")
	require.True(t, ok)
	assert.Equal(t, DefaultExternalMetric, body.Metric, "default external metric")
	assert.True(t, body.ExternalType2, "default metric-type type-2 -> E2")
}

func TestEngineInjectExternalMetricBoundary(t *testing.T) {
	// The AS-External-LSA metric is a 24-bit field; the last valid value
	// (0xFFFFFF = 16777215) is preserved, not truncated (AC-15 boundary).
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":{"source":"static","metric":"16777215"}}}}`)
	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.8.0.0/16"), "static"))
	body, ok := externalBody(t, eng, rid, "10.8.0.0/16")
	require.True(t, ok)
	assert.Equal(t, uint32(0xFFFFFF), body.Metric, "24-bit max metric preserved")
}

func TestEngineInjectExternalNSSAOnly(t *testing.T) {
	// A router attached ONLY to an NSSA (no normal/backbone interface) is a pure NSSA-
	// internal ASBR: it originates a Type 7 into the NSSA and NO Type 5 (it cannot inject
	// Type 5 directly; the translator does that).
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.3.1","areas":{"area":{"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.5"}}},"redistribute":{"connected":{"source":"connected"}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.9.0.0/24"), "connected"))
	assert.Equal(t, 1, selfNSSACount(eng, nssa, rid), "Type 7 originated into the attached NSSA")
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "an NSSA-only ASBR originates no Type 5")

	removed, err := eng.WithdrawExternal(netip.MustParsePrefix("10.9.0.0/24"))
	require.NoError(t, err)
	assert.True(t, removed)
	assert.Equal(t, 0, selfNSSACount(eng, nssa, rid), "Type 7 withdrawn")
}

func TestEngineInjectExternalNSSAandBackbone(t *testing.T) {
	// A router attached to BOTH the backbone and an NSSA originates the Type 5 AS-wide
	// (it can inject directly) and a Type 7 into the NSSA.
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.4.1","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0","area-type":"normal"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"}}},"redistribute":{"connected":{"source":"connected"}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.9.0.0/24"), "connected"))
	assert.Equal(t, 1, eng.lsdb.SelfExternalCount(rid), "Type 5 AS-wide (backbone attachment)")
	assert.Equal(t, 1, selfNSSACount(eng, nssa, rid), "Type 7 into the NSSA")
}

func TestEngineNSSADefaultOriginate(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.5.1","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa","default-cost":"7","nssa":{"default-originate":true}}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	eng.applyNSSADefaults()
	key := types.LSAKey{Type: types.LSTypeNSSA, LinkStateID: types.LinkStateID([4]byte{}), AdvertisingRouter: rid}
	lsa, ok := eng.lsdb.LookupLSA(nssa, key)
	require.True(t, ok, "NSSA ABR originates a Type 7 default")
	assert.False(t, lsa.Header.Options.Has(types.OptionNP), "an ABR-originated NSSA default is not translated (P=0)")
	body, err := lsa.DecodeExternal()
	require.NoError(t, err)
	assert.Equal(t, uint32(7), body.Metric, "default at the area default-cost")

	// Disabling default-originate withdraws the Type 7 default.
	offCfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.5.1","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"}}}}}`), nil)
	require.NoError(t, err)
	eng.setConfig(offCfg)
	eng.applyNSSADefaults()
	assert.Equal(t, 0, selfNSSACount(eng, nssa, rid), "NSSA default withdrawn when default-originate is off")
}

func TestEngineWithdrawExternal(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.6.0.0/16"), "connected"))
	require.Equal(t, 1, eng.lsdb.SelfExternalCount(rid))

	removed, err := eng.WithdrawExternal(netip.MustParsePrefix("10.6.0.0/16"))
	require.NoError(t, err)
	assert.True(t, removed, "an injected prefix is reported removed")
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(rid), "ASBR cleared after the last external is withdrawn")

	again, err := eng.WithdrawExternal(netip.MustParsePrefix("10.7.0.0/16"))
	require.NoError(t, err)
	assert.False(t, again, "withdrawing a never-injected prefix is a no-op")
}
