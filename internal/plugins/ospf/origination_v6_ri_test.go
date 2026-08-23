// VALIDATES: spec-ospf-ext-3 AC-3/AC-4/AC-9, A-3/A-4/A-7 -- the OSPFv3 Router Information LSA
// is originated as a native function-code-12 self-LSA through OriginateSelf, per scope: area
// (0xA00C) into each active area, AS (0xC00C) routed to the AS-wide store; the U-bit is set;
// and disabling RI MaxAge-flushes it via FlushStaleSelfLSAs (v6ManagedSelfTypes).
// PREVENTS: a v3 RI LSA that never installs, an AS-scope LSA mis-stored per area, a cleared
// U-bit, or a lingering LSA after disable.
package ospf

import (
	"strings"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// newV6RIEngine builds a fully-initialized OSPFv3 engine (ri metrics + trackers + lsdb) for
// RI origination tests. It uses the OSPFv2 backend transport only for construction; RI
// origination writes straight to the LSDB and never touches the wire in these unit tests.
func newV6RIEngine(t *testing.T) *engine {
	t.Helper()
	return newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv6Unicast)
}

// v6RIConfig parses a config whose OSPFv3 sub-config enables RI at the given scopes.
func v6RIConfig(t *testing.T, scopes ...string) ospfConfig {
	t.Helper()
	// Mirror what Tree.ToMap emits: a bare string at exactly one member, a JSON
	// array at two or more. Always emitting an array fed a shape no producer
	// makes, so the single-scope cases below exercised a path the daemon never
	// takes.
	scopeJSON := `["area","as"]`
	if len(scopes) == 1 {
		scopeJSON = `"` + scopes[0] + `"`
	} else if len(scopes) > 1 {
		parts := make([]string, len(scopes))
		for i, s := range scopes {
			parts[i] = `"` + s + `"`
		}
		scopeJSON = `[` + strings.Join(parts, ",") + `]`
	}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","address-family":{"ipv6":{`+
		`"router-information":{"enabled":true,"scope":`+scopeJSON+`},`+
		`"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parse v6 RI config: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatalf("no v6 sub-config parsed")
	}
	return *cfg.V6
}

func TestRIv3OriginateArea(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area"))
	router := types.RouterID{1, 1, 1, 1}
	area := types.BackboneArea
	keep := map[ospflsdb.SelfLSARef]struct{}{}

	if n := eng.v6OriginateRIScope(router, OpaqueScopeArea, area, keep); n == 0 {
		t.Fatalf("v6OriginateRIScope(area) originated nothing")
	}
	key := v6RIKey(router, ospfv3types.LSTypeRouterInformationArea, 0)
	lsa, ok := eng.lsdb.LookupLSA(area, key)
	if !ok {
		t.Fatalf("area RI LSA not installed (key type %#04x)", uint16(key.Type))
	}
	if lsa.Header.Type != types.LSType(ospfv3types.LSTypeRouterInformationArea) {
		t.Fatalf("installed type = %#04x, want 0xA00C", uint16(lsa.Header.Type))
	}
}

func TestRIv3OriginateASRouting(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "as"))
	router := types.RouterID{1, 1, 1, 1}
	keep := map[ospflsdb.SelfLSARef]struct{}{}

	if n := eng.v6OriginateRIScope(router, OpaqueScopeAS, types.BackboneArea, keep); n == 0 {
		t.Fatalf("v6OriginateRIScope(as) originated nothing")
	}
	key := v6RIKey(router, ospfv3types.LSTypeRouterInformationAS, 0)
	// A-4: the AS-scope RI LSA (0xC00C) routes to the AS-wide store, reachable via the backbone.
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("AS RI LSA not installed / not routed to AS store")
	}
	if !key.Type.ASWide() {
		t.Fatalf("AS RI type %#04x not classified AS-wide", uint16(key.Type))
	}
	if key.Type.ASExternal() {
		t.Fatalf("AS RI type %#04x wrongly classified as AS-External", uint16(key.Type))
	}
	if lsa.Header.Type != types.LSType(ospfv3types.LSTypeRouterInformationAS) {
		t.Fatalf("installed type = %#04x, want 0xC00C", uint16(lsa.Header.Type))
	}
}

func TestRIv3UBitSet(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area"))
	router := types.RouterID{1, 1, 1, 1}
	keep := map[ospflsdb.SelfLSARef]struct{}{}
	eng.v6OriginateRIScope(router, OpaqueScopeArea, types.BackboneArea, keep)

	key := v6RIKey(router, ospfv3types.LSTypeRouterInformationArea, 0)
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("area RI LSA not installed")
	}
	// RFC 7770 sec 2.2 / RFC 5340 sec 4.4.1: the U-bit (0x8000) must be set.
	if uint16(lsa.Header.Type)&0x8000 == 0 {
		t.Fatalf("RI LSA type %#04x has U-bit clear", uint16(lsa.Header.Type))
	}
}

func TestRIv3WithdrawFlushes(t *testing.T) {
	eng := newV6RIEngine(t)
	eng.setConfig(v6RIConfig(t, "area"))
	router := types.RouterID{1, 1, 1, 1}
	keep := map[ospflsdb.SelfLSARef]struct{}{}
	eng.v6OriginateRIScope(router, OpaqueScopeArea, types.BackboneArea, keep)
	key := v6RIKey(router, ospfv3types.LSTypeRouterInformationArea, 0)
	if lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key); !ok || lsa.Header.Age.IsMaxAge() {
		t.Fatalf("RI LSA not freshly installed before withdraw")
	}
	// Disable RI: v6OriginateSelf would pass an empty keep for the RI type, so the stale flush
	// (over v6ManagedSelfTypes, which includes the RI types) MaxAge-purges it.
	eng.lsdb.FlushStaleSelfLSAs(router, v6ManagedSelfTypes, map[ospflsdb.SelfLSARef]struct{}{})
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("RI LSA vanished entirely; want a MaxAge purge instance")
	}
	if !lsa.Header.Age.IsMaxAge() {
		t.Fatalf("RI LSA not MaxAge-flushed after disable (age %d)", lsa.Header.Age.Age())
	}
}
