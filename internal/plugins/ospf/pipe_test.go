// VALIDATES: spec-ospf-ext-14 AC-19, R-7 -- every new show/debug command's snapshot is
// valid JSON, so the central dispatch's ApplyPipes can render it through json / ndjson /
// table / text / yaml / count, and resolve / origin can walk its IP-bearing fields.
// PREVENTS: a command whose snapshot fails to marshal (breaking every pipe operator).
package ospf

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func mustMarshal(t *testing.T, name string, v any) {
	t.Helper()
	if _, err := json.Marshal(v); err != nil {
		t.Fatalf("%s snapshot is not JSON-marshalable: %v", name, err)
	}
}

func TestNewCommandsPipeComplete(t *testing.T) {
	injectEnabled(t)
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	originateOpaque(t, eng, rid, 250, 1, wellFormedTLV(0x0001, []byte{1, 2, 3, 4}))
	res, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "2", "hex", "00010000"})
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	mustMarshal(t, "opaque-detail", eng.opaqueDetailSnapshot(OpaqueScopeArea))
	mustMarshal(t, "spf-explain", eng.spfExplainSnapshot())
	mustMarshal(t, "neighbor-detail", eng.neighborDetailSnapshot())
	mustMarshal(t, "interface-detail", eng.interfaceDetailSnapshot())
	mustMarshal(t, "inject-result", res)
}

func TestV3NewCommandsPipeComplete(t *testing.T) {
	injectEnabled(t)
	router := types.RouterID{10, 0, 0, 1}
	e := newV6DecodeEngine(t, router)
	originateV6Router(t, e, router, types.BackboneArea)
	res, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2009", "id", "1", "hex", "00000000"})
	if err != nil {
		t.Fatalf("v6 inject: %v", err)
	}

	full, err := e.v3DatabaseDetailSnapshot("", "")
	if err != nil {
		t.Fatalf("v3 database detail: %v", err)
	}
	mustMarshal(t, "v3-database-detail", full)
	mustMarshal(t, "v3-spf-explain", e.spfExplainSnapshot())
	mustMarshal(t, "v3-neighbor-detail", e.v3NeighborDetailSnapshot())
	mustMarshal(t, "v3-interface-detail", e.v3InterfaceDetailSnapshot())
	mustMarshal(t, "v3-inject-result", res)
}
