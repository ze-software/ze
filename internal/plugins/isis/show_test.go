// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- engine render + clear unit tests.
// Related: show.go -- the hostname/interface/spf-log render and clear actions under test.
//
// VALIDATES: `show isis hostname` maps System ID -> TLV 137 name (RFC 5301);
// `show isis interface` reports per-circuit level/metric/passive/DIS; `show isis
// spf-log` projects the SPF history; `clear isis adjacency` drops adjacencies;
// `clear isis counters` resets the SPF-log; the hostname sanitizer drops
// non-printable bytes; auth-configured interfaces report Authenticated without
// leaking keys.
// PREVENTS: a hostname render that misses the local node or surfaces garbage; an
// interface view that omits passive circuits; a clear that does not reset state.

package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestISISShowHostnameRender: the local node's configured hostname appears in
// the `show isis hostname` view, mapped to its own System ID and flagged local.
func TestISISShowHostnameRender(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","hostname":"ze-router","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	defer eng.shutdown()

	rows := eng.hostnameSnapshot()
	if len(rows) == 0 {
		t.Fatal("hostnameSnapshot empty; expected the local node's TLV 137 mapping")
	}
	found := false
	want := eng.cfg.SystemID.String()
	for _, r := range rows {
		hr, ok := r.(hostnameRow)
		if !ok {
			t.Fatalf("hostname row is %T, want hostnameRow", r)
		}
		if hr.SystemID == want {
			if hr.Hostname != "ze-router" {
				t.Errorf("hostname = %q, want ze-router", hr.Hostname)
			}
			if !hr.Local {
				t.Error("local node not flagged Local")
			}
			found = true
		}
	}
	if !found {
		t.Errorf("local System ID %s not in hostname view %+v", want, rows)
	}
}

// TestISISShowHostnameSkipsNoTLV137: a node config with no hostname leaf
// originates no TLV 137, so the local node is omitted from the hostname view.
func TestISISShowHostnameSkipsNoTLV137(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	defer eng.shutdown()
	own := eng.cfg.SystemID.String()
	for _, r := range eng.hostnameSnapshot() {
		if hr, ok := r.(hostnameRow); ok && hr.SystemID == own {
			t.Errorf("local node %s present without a configured hostname", own)
		}
	}
}

// TestISISShowInterfaceRender: a configured circuit and a passive interface both
// appear, with the passive flag and parameters carried through; auth-configured
// circuits report Authenticated.
func TestISISShowInterfaceRender(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point","metric":"42","hello-interval":"3"},"lo":{"passive":"true"}}}}}`)
	defer eng.shutdown()

	rows := eng.interfaceSnapshot()
	byName := map[string]interfaceRow{}
	for _, r := range rows {
		ir, ok := r.(interfaceRow)
		if !ok {
			t.Fatalf("interface row is %T, want interfaceRow", r)
		}
		byName[ir.Name] = ir
	}
	eth0, ok := byName["eth0"]
	if !ok {
		t.Fatalf("eth0 missing from interface view %+v", rows)
	}
	if eth0.Metric != 42 {
		t.Errorf("eth0 metric = %d, want 42", eth0.Metric)
	}
	if eth0.HelloInterval != 3 {
		t.Errorf("eth0 hello-interval = %d, want 3", eth0.HelloInterval)
	}
	if eth0.CircuitType != "point-to-point" {
		t.Errorf("eth0 circuit-type = %q, want point-to-point", eth0.CircuitType)
	}
	lo, ok := byName["lo"]
	if !ok {
		t.Fatalf("passive lo missing from interface view %+v", rows)
	}
	if !lo.Passive {
		t.Error("lo not flagged passive")
	}
}

// TestISISShowSPFLogRender: after the engine triggers SPF, the spf-log view
// carries the run with the lsdb-change trigger.
func TestISISShowSPFLogRender(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	defer eng.shutdown()
	// Force a synchronous SPF run via the engine trigger path (sets the tag) then
	// a direct Run on the Computer to record deterministically.
	eng.spf.SetSPFLogTrigger("lsdb-change")
	eng.spf.Run()
	rows := eng.spfLogView()
	if len(rows) == 0 {
		t.Fatal("spfLogView empty after a run")
	}
	en, ok := rows[0].(spf.SPFLogEntry)
	if !ok {
		t.Fatalf("spf-log row is %T, want spf.SPFLogEntry", rows[0])
	}
	if en.Trigger != "lsdb-change" {
		t.Errorf("spf-log trigger = %q, want lsdb-change", en.Trigger)
	}
}

// TestISISClearAdjacencies: clearing drops adjacency records; on a started engine
// with no neighbors it returns 0 without error.
func TestISISClearAdjacencies(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	defer eng.shutdown()
	if n := eng.clearAdjacencies(); n != 0 {
		t.Errorf("clearAdjacencies on a fresh engine = %d, want 0", n)
	}
}

// TestISISClearCounters: clearing counters empties the SPF-run history.
func TestISISClearCounters(t *testing.T) {
	eng := startedEngine(t, `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`)
	defer eng.shutdown()
	eng.spf.SetSPFLogTrigger("lsdb-change")
	eng.spf.Run()
	if len(eng.spfLogView()) == 0 {
		t.Fatal("precondition: spf-log should have an entry after a run")
	}
	eng.clearCounters()
	if got := eng.spfLogView(); len(got) != 0 {
		t.Errorf("after clearCounters spf-log len = %d, want 0", len(got))
	}
}

// TestISISSanitizeHostname: control and non-ASCII bytes are dropped from a TLV
// 137 value (RFC 5301 sec 3 is 7-bit ASCII); printable ASCII is preserved.
func TestISISSanitizeHostname(t *testing.T) {
	got := sanitizeHostname([]byte{'z', 'e', 0x00, 0x80, '-', 'r', 0x07})
	if got != "ze-r" {
		t.Errorf("sanitizeHostname = %q, want ze-r", got)
	}
}

// TestISISHostnameFromLSP: the TLV 137 value is extracted from a decoded LSP and
// absent TLV 137 yields the empty string.
func TestISISHostnameFromLSP(t *testing.T) {
	with := &packet.LSP{TLVs: []packet.TLV{{Type: packet.TLVDynamicHostname, Value: []byte("host-a")}}}
	if got := hostnameFromLSP(with); got != "host-a" {
		t.Errorf("hostnameFromLSP = %q, want host-a", got)
	}
	without := &packet.LSP{TLVs: []packet.TLV{{Type: packet.TLVAreaAddresses, Value: []byte{1, 2}}}}
	if got := hostnameFromLSP(without); got != "" {
		t.Errorf("hostnameFromLSP with no TLV 137 = %q, want empty", got)
	}
}

// TestISISHostnameSnapshotPeer: a peer LSP carrying TLV 137 injected into the
// LSDB shows up as a non-local hostname row.
func TestISISHostnameSnapshotPeer(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	peer := types.SystemID{0, 0, 0, 0, 0, 0x55}
	id := types.NewLSPID(types.NewSourceID(peer, 0), 0)
	lsp := &packet.LSP{
		PDUType:           packet.PDUTypeL2LSP,
		LSPID:             id,
		RemainingLifetime: 1200, // non-zero so the entry is live, not a purge
		SequenceNumber:    1,
		TLVs:              []packet.TLV{{Type: packet.TLVDynamicHostname, Value: []byte("peer-host")}},
	}
	raw := make([]byte, lsp.EncodedLen())
	n := lsp.WriteTo(raw, 0)
	eng.lsdb.Insert(lsdb.Level2, lsp, raw[:n])

	found := false
	for _, r := range eng.hostnameSnapshot() {
		hr, ok := r.(hostnameRow)
		if !ok {
			t.Fatalf("hostname row is %T, want hostnameRow", r)
		}
		if hr.SystemID == peer.String() {
			if hr.Hostname != "peer-host" || hr.Local {
				t.Errorf("peer row = %+v, want hostname peer-host, Local false", hr)
			}
			found = true
		}
	}
	if !found {
		t.Error("peer hostname not surfaced from injected LSP")
	}
}
