// VALIDATES: RFC 3101 Section 2.4 default origination for every attached NSSA,
// plus spec-ospf-14 AC-16 translator suppression by a higher-Router-ID Type 5.
// PREVENTS: regular NSSAs missing their required border-router default and
// overlapping translators injecting duplicate Type 5 LSAs.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestOSPFNSSABorderRouterDefaultsEveryArea verifies the Type-7 half of
// the mandatory NSSA border-router default behavior.
func TestOSPFNSSABorderRouterDefaultsEveryArea(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa","no-summary":"true"},"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"},"eth2":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	stubby := types.AreaID{0, 0, 0, 5}
	regular := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: stubby}
	eng.running["eth2"] = interfaceConfig{Name: "eth2", AreaID: regular}

	eng.applyNSSADefaults()

	assert.Equal(t, 0, selfNSSACount(eng, stubby, self), "no-summary NSSA uses a Type-3 default")
	// RFC requirement: RFC3101-2.4-5 positive -- a regular NSSA receives
	// a default-destination LSA without an operator gate.
	require.Equal(t, 1, selfNSSACount(eng, regular, self))
	key := types.LSAKey{Type: types.LSTypeNSSA, AdvertisingRouter: self}
	lsa, ok := eng.lsdb.LookupLSA(regular, key)
	require.True(t, ok)
	// RFC requirement: RFC3101-2.4-4 positive -- an NSSA border router
	// clears the P-bit on its Type-7 default.
	assert.False(t, lsa.Header.Options.Has(types.OptionNP))
}

// TestOSPFNSSAInternalRouterOriginatesNoBorderDefault verifies that the
// border-router requirement does not make an internal NSSA router originate it.
func TestOSPFNSSAInternalRouterOriginatesNoBorderDefault(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}

	eng.applyNSSADefaults()

	// RFC requirement: RFC3101-2.4-5 negative -- a router that is not an
	// NSSA border router does not originate the border-router default.
	assert.Equal(t, 0, selfNSSACount(eng, nssa, self), "internal NSSA router originates no border default")
}

func TestOSPFNSSANonBackboneMultiAreaRouterIsNotABR(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.1":{"area-id":"0.0.0.1"},"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.1"},"eth1":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.AreaID{0, 0, 0, 1}}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	eng.applyNSSADefaults()

	assert.Equal(t, 0, selfNSSACount(eng, nssa, self), "two non-backbone areas do not make an ABR")
}

func TestOSPFNSSADefaultWithdrawnFromRemovedArea(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	eng.applyNSSADefaults()
	require.Equal(t, 1, selfNSSACount(eng, nssa, self))

	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"}}}}}`), nil)
	require.NoError(t, err)
	eng.setConfig(cfg)
	delete(eng.running, "eth1")
	eng.applyNSSADefaults()

	assert.Equal(t, 0, selfNSSACount(eng, nssa, self), "removed NSSA default is withdrawn")
	assert.False(t, eng.lsdb.SelfIsASBR(self), "removed default no longer marks self as ASBR")
}

func TestOSPFNSSABackboneDownWithdrawsBorderDefault(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	eng.applyNSSADefaults()
	require.Equal(t, 1, selfNSSACount(eng, nssa, self))

	down := ospfiface.New(ospfiface.Config{
		Name: "eth0", RouterID: self, AreaID: types.BackboneArea,
		NetworkType: ospfiface.NetworkPointToPoint,
	}, &rfc5340Sender{}, ospfiface.NopMetrics())
	t.Cleanup(down.Stop)
	eng.interfaces["eth0"] = down
	eng.applyNSSADefaults()

	assert.Equal(t, 0, selfNSSACount(eng, nssa, self), "inactive backbone removes the ABR default")
}

func TestOSPFNSSAHigherRIDType5Suppresses(t *testing.T) {
	eng, nssa := nssaTransEngine(t, "10.0.6.9") // self, the only ABR -> elected translator
	self := ridOf("10.0.6.9")
	asbr := ridOf("10.0.6.2")
	const net = "10.25.0.0"

	// A P=1, non-zero-FA Type 7 self would normally translate.
	eng.lsdb.OriginateNSSA(nssa, asbr, ip4Of(net), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)

	// Control: as the highest-Router-ID translator, self translates it to a Type 5.
	// RFC requirement: RFC3101-3.2-2 positive -- as the highest-Router-ID translator, self
	// translates the Type-7 into a Type-5 (no equivalent higher-RID Type-5 exists to suppress it).
	eng.translateNSSA(transTime)
	require.Equal(t, 1, eng.lsdb.SelfExternalCount(self), "control: self translates when it is the highest-Router-ID translator")

	// A strictly-higher-Router-ID translator advertises an equivalent Type 5 for the network.
	higher := ridOf("10.0.6.250")
	_, _, _ = eng.lsdb.OriginateExternal(higher, ip4Of(net), ip4Of("255.255.0.0"), types.OptionE, false, 10, ip4Of("10.5.0.2"), 0)

	// RFC 3101 §3.6: self yields -- it must not also inject a Type 5, and withdraws its own.
	// RFC requirement: RFC3101-3.2-2 negative -- when a strictly-higher-Router-ID translator
	// advertises an equivalent Type-5, self suppresses (and withdraws) its duplicate translation.
	eng.translateNSSA(transTime)
	assert.Equal(t, 0, eng.lsdb.SelfExternalCount(self), "suppressed and withdrawn once a higher-Router-ID Type 5 appears")
}
