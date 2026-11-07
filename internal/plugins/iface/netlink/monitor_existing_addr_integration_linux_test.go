//go:build integration && linux

// Design: docs/architecture/core-design.md -- connected prefixes in the shared Loc-RIB
// Related: monitor_linux.go -- start, which asks the address subscription to dump existing addresses

package ifacenetlink

import (
	"encoding/json"
	"testing"
	"time"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestMonitorAnnouncesAddressesThatPredateIt checks that an address assigned
// before the monitor starts reaches the bus as addr-added. The method is the
// real kernel: a dummy link gets its address first, then the monitor starts on
// a collecting bus, and the test waits for the event naming that address.
//
// PREVENTS: the boot-time blind spot. The interface component starts its
// monitor after boot-time applyConfig has assigned the operator's addresses,
// and a subscription delivers only changes, so no consumer heard of them: the
// connected plugin published no prefix, sysrib could prove no gateway inside
// one, and a gateway member beside a device-only member left its multipath
// group. Removing ListExisting from start makes this test red.
func TestMonitorAnnouncesAddressesThatPredateIt(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const link = "mon-predate"
	const address = "192.0.2.77"
	b := &netlinkBackend{}
	if err := b.CreateDummy(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.DeleteInterface(link); err != nil {
			t.Error(err)
		}
	})
	if err := b.AddAddress(link, address+"/24"); err != nil {
		t.Fatal(err)
	}

	m, bus := newTestMonitor()
	if err := m.start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.stop)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if announced(t, bus, link, address) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %s event for %s on %s, which predates the monitor", ifaceevents.EventAddrAdded, address, link)
}

// announced reports whether the bus recorded addr-added for address on link.
func announced(t *testing.T, bus *collectingEventBus, link, address string) bool {
	t.Helper()
	for _, ev := range bus.snapshot() {
		if ev.EventType != ifaceevents.EventAddrAdded {
			continue
		}
		data, ok := ev.Payload.(string)
		if !ok {
			t.Fatalf("addr-added payload is %T, want the JSON string the monitor emits", ev.Payload)
		}
		var p addrEventPayload
		if err := json.Unmarshal([]byte(data), &p); err != nil {
			t.Fatalf("addr-added payload %q: %v", data, err)
		}
		if p.Name == link && p.Address == address {
			return true
		}
	}
	return false
}
