// VALIDATES: `show ddos local` surfaces the responder's live on-host mitigation
// state (installed / not, and the target vector) via the registered RPC handler.
// PREVENTS: the on-host drop state being unreachable from the CLI, and a
// regression that unregisters the handler.

package local

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestShowDdosLocalNoResponder(t *testing.T) {
	activeResponder.Store(nil)

	resp, err := handleShowDdosLocal(nil, nil)
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

func TestShowDdosLocalActive(t *testing.T) {
	r := newResponder(DefaultConfig(), nil)
	r.active = true
	r.target = ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 6, DstPort: 80}
	activeResponder.Store(r)
	t.Cleanup(func() { activeResponder.Store(nil) })

	resp, err := handleShowDdosLocal(nil, nil)
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
