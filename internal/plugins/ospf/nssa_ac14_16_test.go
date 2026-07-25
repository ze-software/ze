// VALIDATES: spec-ospf-14 AC-14 (RFC 3101 §2.3: a totally-stubby/no-summary NSSA ABR MUST
// auto-originate a Type 7 default even without `default-originate`, while a regular NSSA stays
// operator-gated) and AC-16 (RFC 3101 §3.6: the engine suppresses and withdraws its translation
// when a strictly-higher-Router-ID translator already advertises an equivalent Type 5).
// PREVENTS: a totally-stubby NSSA's internal routers blackholing externals for lack of a default,
// and two overlapping translators injecting duplicate Type 5 LSAs for one network.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFNSSATotallyStubbyAutoDefault(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{"0.0.0.0":{"area-id":"0.0.0.0"},"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa","no-summary":"true"},"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.5"},"eth2":{"area":"0.0.0.6"}}}}}`)
	self := ridOf("10.0.10.9")
	stubby := types.AreaID{0, 0, 0, 5}  // totally-stubby NSSA (no-summary)
	regular := types.AreaID{0, 0, 0, 6} // regular NSSA, no default-originate
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: stubby}
	eng.running["eth2"] = interfaceConfig{Name: "eth2", AreaID: regular}

	eng.applyNSSADefaults()

	// RFC 3101 §2.3: a totally-stubby (no-summary) NSSA ABR MUST auto-originate the Type 7
	// default (0.0.0.0/0) even without `default-originate` -- the only Type 7 originated here.
	assert.Equal(t, 1, selfNSSACount(eng, stubby, self), "totally-stubby NSSA auto-originates a Type 7 default")
	// A regular NSSA without `default-originate` originates no default.
	assert.Equal(t, 0, selfNSSACount(eng, regular, self), "regular NSSA without default-originate originates no default")
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
