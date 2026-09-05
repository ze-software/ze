// VALIDATES: the operator's read path for the RFC 3101 NSSA default route. An NSSA border
// router's default reaches `show ospf database nssa-external` with no `default-originate`
// leaf configured, an internal router's default appears there only with a usable
// forwarding address, and an OSPFv3 border router's default appears as an NSSA-LSA.
// PREVENTS: origination proven only at the engine boundary. The subview an operator reads
// filters on the LS Type string, so a default keyed with the wrong LS Type is invisible
// there while every engine-level assertion still passes.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// nssaSubviewLSAs returns the LSAs `show ospf database nssa-external` renders for one
// area, resolved the way the registered RPC handler resolves it: the command string picks
// the LS Type out of dbSubviewType, and databaseSnapshotByType filters the LSDB to it.
func nssaSubviewLSAs(t *testing.T, eng *engine, area types.AreaID) []ospflsdb.LSASnapshot {
	t.Helper()
	lsType, ok := dbSubviewType[cmdShowDatabaseNSSAExternal]
	require.True(t, ok, "%q is a registered database subview", cmdShowDatabaseNSSAExternal)
	payload := eng.databaseSnapshotByType(lsType)
	require.Len(t, payload, 1, "the subview renders one snapshot")
	snapshot, ok := payload[0].(ospflsdb.Snapshot)
	require.True(t, ok, "the subview payload is an LSDB snapshot")
	for _, a := range snapshot.Areas {
		if a.Area == area {
			return a.LSAs
		}
	}
	return nil
}

// nssaABRConfig is a dual-area router: the backbone plus one NSSA makes it an NSSA border
// router. It sets no `nssa { default-originate }` leaf, because RFC 3101 Section 2.4 gives
// a border router no operator gate.
func nssaABRConfig(nssaLeaves string) string {
	return `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{` +
		`"0.0.0.0":{"area-id":"0.0.0.0"},` +
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa","default-cost":"12"` + nssaLeaves + `}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.6"}}}}}`
}

// TestOSPFNSSAABRDefaultFunctional is AC-9: an operator configures an NSSA ABR, sets no
// default-originate leaf, and finds the default in the database subview.
func TestOSPFNSSAABRDefaultFunctional(t *testing.T) {
	eng, rid := newRedistEngine(t, nssaABRConfig(""))
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	eng.applyNSSADefaults()

	lsas := nssaSubviewLSAs(t, eng, nssa)
	require.Len(t, lsas, 1, "show ospf database nssa-external reports the border-router default")
	assert.Equal(t, "nssa", lsas[0].Type)
	assert.Equal(t, rid.String(), lsas[0].AdvertisingRouter)
	assert.Equal(t, "0.0.0.0", lsas[0].LinkStateID, "the default destination")
	assert.Empty(t, nssaSubviewLSAs(t, eng, types.BackboneArea),
		"a Type-7 LSA never reaches a non-NSSA area")
}

// TestOSPFNSSAInternalDefaultFunctional is AC-10: on an NSSA internal router the
// `default-originate` leaf produces a default only when a forwarding address is usable,
// which RFC 3101 Section 2.4 requires on the P-set LSA that leaf asks for.
func TestOSPFNSSAInternalDefaultFunctional(t *testing.T) {
	nssa := types.AreaID{0, 0, 0, 5}
	subview := func(addr netip.Addr, usable bool) []ospflsdb.LSASnapshot {
		eng, _ := newRedistEngine(t, nssaInternalOriginatorConfig(false))
		eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
		eng.forwardingAddress = func(string) (netip.Addr, bool) { return addr, usable }
		eng.applyNSSADefaults()
		return nssaSubviewLSAs(t, eng, nssa)
	}

	withAddress := subview(netip.MustParseAddr("192.0.2.1"), true)
	require.Len(t, withAddress, 1, "control: a usable forwarding address produces the P-set default")
	assert.Equal(t, "0.0.0.0", withAddress[0].LinkStateID)

	assert.Empty(t, subview(netip.Addr{}, false),
		"no usable forwarding address, so the operator sees no default")
}

// TestOSPFv3NSSAABRDefaultFunctional is the OSPFv3 twin of AC-9. The subview filters on the
// LS Type STRING, and types.LSType.String() renders 0x2007 and 0x0007 alike as "nssa", so
// the second assertion reads the LS Type itself: it is what tells an operator's `show`
// output apart from a default no conforming OSPFv3 peer would accept.
func TestOSPFv3NSSAABRDefaultFunctional(t *testing.T) {
	eng, rid, nssa := v6DualAreaNSSAABR(t, "")

	eng.applyNSSADefaults()

	lsas := nssaSubviewLSAs(t, eng, nssa)
	require.Len(t, lsas, 1, "show ospf database nssa-external reports the OSPFv3 border-router default")
	assert.Equal(t, "nssa", lsas[0].Type)
	assert.Equal(t, rid.String(), lsas[0].AdvertisingRouter)
	assert.Equal(t, 1, countAreaLSAsByType(eng, nssa, types.LSType(0x2007)),
		"the LSA behind that row is the 0x2007 NSSA-LSA (RFC 5340 App A.4.8)")
	assert.Equal(t, 0, countAreaLSAsByType(eng, nssa, types.LSTypeNSSA),
		"and not the OSPFv2 0x0007, which RFC 5340 App A.4.2.1 reads as link-local scope")
}
