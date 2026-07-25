// VALIDATES: spec-ospf-ext-14 AC-1/AC-3/AC-24 -- `show ospf database opaque-<scope>
// detail` renders each opaque LSA's header + a decoded body: a registered typed decoder
// yields named content; with no decoder the ext-1 generic TLV iterator yields
// (type,length,value-hex) rows; a malformed body renders as raw hex, bumps
// ze_ospf_debug_decode_errors_total, and never panics.
// PREVENTS: an opaque detail view that shows raw hex when a decoder exists, that panics on
// a malformed body, or that fails to count a decode error.
package ospf

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// countingRegistry is a metrics.Registry that tallies CounterVec .Inc() by series name, so
// a test can assert an error/injection counter moved. Shared by the ext-14 debug tests.
type countingRegistry struct {
	metrics.NopRegistry
	mu     sync.Mutex
	counts map[string]int
}

func newCountingRegistry() *countingRegistry { return &countingRegistry{counts: map[string]int{}} }

func (r *countingRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return &countingVec{r: r, name: name}
}

func (r *countingRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return &countingGaugeVec{r: r, name: name}
}

func (r *countingRegistry) get(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[name]
}

type countingVec struct {
	r    *countingRegistry
	name string
}

func (v *countingVec) With(_ ...string) metrics.Counter {
	return &dbgCounter{r: v.r, name: v.name}
}
func (v *countingVec) Delete(_ ...string) bool { return false }

type dbgCounter struct {
	r    *countingRegistry
	name string
}

func (c *dbgCounter) Inc() { c.Add(1) }
func (c *dbgCounter) Add(f float64) {
	c.r.mu.Lock()
	c.r.counts[c.name] += int(f)
	c.r.mu.Unlock()
}

type countingGaugeVec struct {
	r    *countingRegistry
	name string
}

func (v *countingGaugeVec) With(_ ...string) metrics.Gauge {
	return &countingGauge{r: v.r, name: v.name}
}
func (v *countingGaugeVec) Delete(_ ...string) bool { return false }

type countingGauge struct {
	r    *countingRegistry
	name string
}

func (g *countingGauge) Set(f float64) { g.r.mu.Lock(); g.r.counts[g.name] = int(f); g.r.mu.Unlock() }
func (g *countingGauge) Inc()          { g.Add(1) }
func (g *countingGauge) Dec()          { g.Add(-1) }
func (g *countingGauge) Add(f float64) { g.r.mu.Lock(); g.r.counts[g.name] += int(f); g.r.mu.Unlock() }

// withDebugMetrics swaps the process-global debug metric set for a counting one for the
// duration of a test, bypassing the production sync.Once.
func withDebugMetrics(t *testing.T) *countingRegistry {
	t.Helper()
	rec := newCountingRegistry()
	old := debugMetrics.Load()
	debugMetrics.Store(newDebugMetrics(rec))
	t.Cleanup(func() { debugMetrics.Store(old) })
	return rec
}

// wellFormedTLV builds one 4-byte-aligned opaque TLV (type, value).
func wellFormedTLV(typ uint16, value []byte) []byte {
	out := []byte{byte(typ >> 8), byte(typ), byte(len(value) >> 8), byte(len(value))}
	out = append(out, value...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

func originateOpaque(t *testing.T, eng *engine, rid types.RouterID, opaqueType uint8, id uint32, body []byte) {
	t.Helper()
	_, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: rid, OpaqueType: opaqueType, OpaqueID: id,
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea,
		Options: types.OptionO, Body: body,
	})
	if !ok {
		t.Fatalf("OriginateOpaque(type=%d id=%d) not installed", opaqueType, id)
	}
}

func opaqueDetailRowsOf(t *testing.T, eng *engine) []opaqueDetailLSA {
	t.Helper()
	out := eng.opaqueDetailSnapshot(OpaqueScopeArea)
	for _, v := range out {
		if db, ok := v.(opaqueDetailDatabase); ok {
			return db.OpaqueDetail
		}
	}
	t.Fatalf("opaqueDetailSnapshot did not contain an opaqueDetailDatabase: %#v", out)
	return nil
}

func TestDecodeFallbackNoDecoder(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	body := wellFormedTLV(0x0001, []byte{1, 2, 3, 4})
	originateOpaque(t, eng, rid, 250, 7, body) // Private-Use type, no decoder registered

	rows := opaqueDetailRowsOf(t, eng)
	if len(rows) != 1 {
		t.Fatalf("opaque detail rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Decoder != "generic" {
		t.Fatalf("decoder = %q, want generic", r.Decoder)
	}
	if len(r.TLVs) != 1 || r.TLVs[0].Type != 1 || r.TLVs[0].Length != 4 || r.TLVs[0].ValueHex != "01020304" {
		t.Fatalf("generic TLVs = %+v", r.TLVs)
	}
	if r.OpaqueType != 250 || r.OpaqueID != 7 {
		t.Fatalf("opaque type/id = %d/%d", r.OpaqueType, r.OpaqueID)
	}
	if !r.LocalOriginated {
		t.Fatalf("self-originated opaque LSA should be marked local (AC-23)")
	}
}

// TestTEDatabaseView: spec-ospf-ext-14 AC-1/AC-5 -- once the ext-2 TE consumer wires its
// decoder into the detail registry, a stored TE opaque LSA (Opaque Type 1) renders as a
// typed decode (Router-Address / Link sub-TLVs), not raw hex.
func TestTEDatabaseView(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.9"}}`)
	_ = registerTEConsumer(eng) // registers the TE detail decoder (idempotent; ignore consumer dup)
	body := packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{9, 9, 9, 9}}.Encode()
	_, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: rid, OpaqueType: packet.TEOpaqueType, OpaqueID: 0,
		Scope: types.LSTypeOpaqueArea, Area: types.BackboneArea, Options: types.OptionO, Body: body,
	})
	if !ok {
		t.Fatalf("OriginateOpaque(TE) not installed")
	}
	rows := eng.opaqueDetailRows(OpaqueScopeArea)
	found := false
	for _, r := range rows {
		if r.OpaqueType == packet.TEOpaqueType && r.Decoder == "traffic-engineering" && r.Decoded != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("TE opaque LSA was not typed-decoded via the registry: %+v", rows)
	}
}

func TestOpaqueDecodeTypedDecoder(t *testing.T) {
	const testType uint8 = 251
	registerOpaqueDetailDecoder(testType, "debug-test", func(body []byte) (any, error) {
		return map[string]int{"len": len(body)}, nil
	})
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.2"}}`)
	originateOpaque(t, eng, rid, testType, 3, wellFormedTLV(0x0002, []byte{9, 9}))

	rows := opaqueDetailRowsOf(t, eng)
	if len(rows) != 1 || rows[0].Decoder != "debug-test" || rows[0].Decoded == nil {
		t.Fatalf("typed decode row = %+v", rows[0])
	}
}

func TestOpaqueDecodeMalformed(t *testing.T) {
	rec := withDebugMetrics(t)
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.3"}}`)
	// Claims a 8-byte value but supplies 2: DecodeOpaqueTLVs returns ErrLength.
	malformed := []byte{0x00, 0x01, 0x00, 0x08, 0x01, 0x02}
	originateOpaque(t, eng, rid, 252, 1, malformed)

	rows := opaqueDetailRowsOf(t, eng)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if !rows[0].Malformed || rows[0].BodyHex == "" {
		t.Fatalf("malformed body should render raw hex: %+v", rows[0])
	}
	if rec.get("ze_ospf_debug_decode_errors_total") == 0 {
		t.Fatalf("decode-error metric not incremented")
	}
}
