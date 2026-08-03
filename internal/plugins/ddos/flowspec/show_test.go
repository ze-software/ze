// VALIDATES: `show ddos flowspec` surfaces the responder's live upstream
// announcement state (announced / not, target vector, probing) via the
// registered RPC handler.
// PREVENTS: the FlowSpec mitigation state being unreachable from the CLI, and a
// regression that unregisters the handler.

package flowspec

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestShowDdosFlowspecNoResponder(t *testing.T) {
	activeResponder.Store(nil)

	resp, err := handleShowDdosFlowspec(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != false || m["active"] != false {
		t.Errorf("got %v, want enabled=false active=false", m)
	}
}

func TestShowDdosFlowspecActive(t *testing.T) {
	r := newResponder(DefaultConfig(), &fakeDispatcher{})
	// announce is the responder's writer of the announcement state: it keeps the
	// mu-guarded fields and the lock-free snapshot the show handler reads in
	// step. Poking the fields would leave the snapshot idle. It documents
	// "Caller holds r.mu", so the fixture holds it -- a fixture that models the
	// production writer wrongly stops being evidence about the production writer.
	r.mu.Lock()
	r.announce(ddosevent.VectorTuple{
		DstPrefix: netip.MustParsePrefix("198.51.100.0/24"), Proto: 17, DstPort: 53,
	}, r.cfg.Action, "test fixture")
	r.mu.Unlock()
	activeResponder.Store(r)
	t.Cleanup(func() { activeResponder.Store(nil) })

	resp, err := handleShowDdosFlowspec(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != true || m["active"] != true {
		t.Errorf("got %v, want enabled=true active=true", m)
	}
	if _, ok := m["target"]; !ok {
		t.Error("active mitigation must report a target")
	}
}
