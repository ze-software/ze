// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA default-route origination.
// Related: nssa.go -- applyNSSADefaults, the per-area NSSA default reconciler.
// Related: rfc3101_nssa_ac14_16_test.go -- the plain border-router default (P-clear) positive.
//
// VALIDATES: RFC 3101 Section 2.4 on the NSSA border router's Type-7 default: every input
// that makes an internal NSSA router originate a P-set default (a redistributed default
// whose source is marked nssa-propagate, and the area's default-originate) is present, and
// the border router's default still carries the P-bit clear.
// PREVENTS: a border router whose default follows the import policy an internal router
// uses, which would have another NSSA border router translate the default into a Type-5.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC3101-2.4-4 negative -- RFC 3101 Section 2.4: "The Type-7 default LSA
// originated by an NSSA border router must have the P-bit clear." The inputs are pushed
// toward the violation: the NSSA has default-originate set and the default route is
// redistributed from a source marked nssa-propagate, the pair that makes an internal NSSA
// router ask for a P-set default. applyNSSADefaults on the border router still installs
// its Type-7 default in the NSSA, and that LSA's P-bit is clear, so a border router that
// took the internal router's propagate decision fails here.
func TestRFC3101BorderRouterDefaultStaysPClearUnderPropagatingImport(t *testing.T) {
	// Goal: the border-router P-clear rule wins over every P-set trigger. Method: a
	// backbone plus NSSA engine (a border router), the retained default-route import a
	// redistribution leaves behind, then the reconciler; read the LSA it installs.
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{`+
		`"0.0.0.0":{"area-id":"0.0.0.0"},`+
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa","nssa":{"default-originate":"true"}}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.6"}}},`+
		`"redistribute":{"connected":{"source":"connected","nssa-propagate":"true"}}}}`)
	self := ridOf("10.0.10.9")
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	originate := false
	for _, a := range eng.cfg.Areas {
		if a.AreaID == nssa {
			originate = a.NSSADefaultOriginate
		}
	}
	require.True(t, originate, "the NSSA's default-originate leaf is parsed")
	require.True(t, externalPropagate(eng.cfg, "connected", false),
		"the redistribution source is marked nssa-propagate")
	defaultPrefix, _ := eng.defaultRoute()
	eng.externalImports = map[netip.Prefix]externalImport{
		defaultPrefix: {source: "connected", areas: []types.AreaID{nssa}},
	}

	eng.applyNSSADefaults()

	require.Equal(t, 1, selfNSSACount(eng, nssa, self), "the border router originates its Type-7 default")
	lsa, ok := eng.lsdb.LookupLSA(nssa, types.LSAKey{Type: types.LSTypeNSSA, AdvertisingRouter: self})
	require.True(t, ok)
	assert.False(t, lsa.Header.Options.Has(types.OptionNP),
		"the border router's Type-7 default is P-clear even when the import asks for propagation")
}
