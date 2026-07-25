// VALIDATES: `show ddos` and `show ddos incidents` surface the live incident
// store -- status counts and the incident ring are returned via the registered
// in-process RPC handlers.
// PREVENTS: the incident store staying unreachable from the CLI (the previously
// dead store.list()), and a regression that unregisters the handlers.

package observe

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

func TestShowDdosNoStore(t *testing.T) {
	activeStore.Store(nil)

	resp, err := handleShowDdos(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != false {
		t.Errorf("enabled = %v, want false", m["enabled"])
	}

	resp, err = handleShowDdosIncidents(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, ok = resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", resp.Data)
	}
	if m["enabled"] != false {
		t.Errorf("incidents enabled = %v, want false", m["enabled"])
	}
}

func TestShowDdosWithActiveIncident(t *testing.T) {
	s := newStore(10, time.Minute)
	s.open(&ddosevent.AttackDetected{
		Interface: "eth0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.0/24"), Proto: 17, DstPort: 53},
		Family:    ddosevent.FamilyReflection,
		PeakRxPps: 1_000_000,
		PeakRxBps: 8_000_000_000,
	})
	activeStore.Store(s)
	t.Cleanup(func() { activeStore.Store(nil) })

	statusResp, err := handleShowDdos(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	status, ok := statusResp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", statusResp.Data)
	}
	if status["enabled"] != true {
		t.Errorf("enabled = %v, want true", status["enabled"])
	}
	if status["active-attacks"] != 1 {
		t.Errorf("active-attacks = %v, want 1", status["active-attacks"])
	}
	if status["incidents"] != 1 {
		t.Errorf("incidents = %v, want 1", status["incidents"])
	}

	listResp, err := handleShowDdosIncidents(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := listResp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("Data is %T, want plugin.Map", listResp.Data)
	}
	incs, ok := list["incidents"].([]incident)
	if !ok {
		t.Fatalf("incidents is %T, want []incident", list["incidents"])
	}
	if len(incs) != 1 {
		t.Fatalf("got %d incidents, want 1", len(incs))
	}
	if incs[0].Family != ddosevent.FamilyReflection || !incs[0].Active {
		t.Errorf("incident = %+v, want reflection/active", incs[0])
	}
}
