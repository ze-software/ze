// Design: docs/features/interfaces.md -- owned-macvlan registry semantics tests

package iface

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// resetDeviceOwners clears the owned-macvlan registry and gauge bookkeeping
// between tests, mirroring resetAddressOwners.
func resetDeviceOwners(t *testing.T) {
	t.Helper()
	deviceOwnerMu.Lock()
	deviceOwners = map[string]map[string]MacvlanSpec{}
	gaugeOwners = map[string]bool{}
	deviceOwnerMu.Unlock()
	setDeviceOwnerReconcileTrigger(nil)
	t.Cleanup(func() {
		deviceOwnerMu.Lock()
		deviceOwners = map[string]map[string]MacvlanSpec{}
		gaugeOwners = map[string]bool{}
		deviceOwnerMu.Unlock()
		setDeviceOwnerReconcileTrigger(nil)
	})
}

func macvlanSpec(name, parent, mac string) MacvlanSpec {
	return MacvlanSpec{Name: name, Parent: parent, MAC: mac}
}

// TestRegisterOwnedMacvlan_ConflictAcrossOwners verifies that a second owner
// cannot claim a device name already registered by a different owner.
//
// VALIDATES: AC-7 (second owner rejected naming the first; original intact).
// PREVENTS: two owners silently fighting over one device name.
func TestRegisterOwnedMacvlan_ConflictAcrossOwners(t *testing.T) {
	resetDeviceOwners(t)

	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-10", "eth0", "00:00:5e:00:01:0a")); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := RegisterOwnedMacvlan("o2", macvlanSpec("zv4-42-10", "eth1", "00:00:5e:00:01:0b"))
	if err == nil {
		t.Fatal("second owner should be rejected for the same device name")
	}
	if !strings.Contains(err.Error(), "o1") {
		t.Errorf("error %q should name the owning owner o1", err.Error())
	}
	// Original registration unchanged: still owned by o1 with the first spec.
	specs, owners := ownedMacvlans()
	if owners["zv4-42-10"] != "o1" {
		t.Errorf("device still owned by %q, want o1", owners["zv4-42-10"])
	}
	if specs["zv4-42-10"].Parent != "eth0" {
		t.Errorf("original spec mutated: parent=%q", specs["zv4-42-10"].Parent)
	}
}

// TestRegisterOwnedMacvlan_IdempotentReplace verifies same-owner re-register
// replaces the spec and does not create duplicate entries.
//
// VALIDATES: AC-12 (re-register same owner+name replaces; the desired set holds
// one entry).
func TestRegisterOwnedMacvlan_IdempotentReplace(t *testing.T) {
	resetDeviceOwners(t)

	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-10", "eth0", "00:00:5e:00:01:0a")); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Re-register the same owner+name with a changed MAC: replaces in place.
	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-10", "eth0", "00:00:5e:00:01:0b")); err != nil {
		t.Fatalf("re-register: %v", err)
	}
	specs, _ := ownedMacvlans()
	if len(specs) != 1 {
		t.Fatalf("want 1 desired device, got %d", len(specs))
	}
	if specs["zv4-42-10"].MAC != "00:00:5e:00:01:0b" {
		t.Errorf("re-register did not replace spec: MAC=%q", specs["zv4-42-10"].MAC)
	}
}

// TestUnregisterOwnedMacvlanPerName verifies per-name unregister semantics:
// each call removes exactly one device, and removing the last one empties the
// owner's desired set (and drops the owner entry). VRRP uses a per-instance
// owner that holds exactly one macvlan, so the singular per-name call is the
// only unregister the plugin needs.
//
// VALIDATES: AC-3 lifecycle -- per-name removal takes exactly one; the last
// removal empties the set.
func TestUnregisterOwnedMacvlanPerName(t *testing.T) {
	resetDeviceOwners(t)

	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-10", "eth0", "00:00:5e:00:01:0a")); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-11", "eth0", "00:00:5e:00:01:0b")); err != nil {
		t.Fatalf("register b: %v", err)
	}

	// Per-name removal takes exactly one.
	UnregisterOwnedMacvlan("o1", "zv4-42-10")
	specs, _ := ownedMacvlans()
	if len(specs) != 1 || specs["zv4-42-11"].Name != "zv4-42-11" {
		t.Fatalf("per-name unregister wrong result: %+v", specs)
	}

	// Removing the last device empties the set (and drops the owner entry).
	UnregisterOwnedMacvlan("o1", "zv4-42-11")
	specs, _ = ownedMacvlans()
	if len(specs) != 0 {
		t.Fatalf("unregister left %d devices", len(specs))
	}
}

// TestOwnedDeviceGaugeTracksRegistry verifies the ze_iface_owned_devices gauge
// follows register/unregister per owner.
//
// VALIDATES: story 5 -- gauge per owner follows register/unregister.
// PREVENTS: a stale gauge series after an owner releases its devices.
func TestOwnedDeviceGaugeTracksRegistry(t *testing.T) {
	resetDeviceOwners(t)

	reg := newCapturingGaugeRegistry()
	bindMetricsRegistry(reg)
	t.Cleanup(func() { ifaceMetricsPtr.Store(nil) })

	gauge := reg.ownedDevicesVec

	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-10", "eth0", "00:00:5e:00:01:0a")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if got := gauge.value("o1"); got != 1 {
		t.Errorf("gauge o1 = %v, want 1", got)
	}
	if err := RegisterOwnedMacvlan("o1", macvlanSpec("zv4-42-11", "eth0", "00:00:5e:00:01:0b")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if got := gauge.value("o1"); got != 2 {
		t.Errorf("gauge o1 = %v, want 2", got)
	}
	UnregisterOwnedMacvlan("o1", "zv4-42-10")
	UnregisterOwnedMacvlan("o1", "zv4-42-11")
	if _, present := gauge.gauges["o1"]; present {
		t.Errorf("gauge o1 series should be deleted after owner release")
	}
}

// capturingGaugeRegistry is a metrics.Registry that records the vectors
// bindMetricsRegistry creates, so a test can read back per-label values.
//
// CounterVec returns a working vector rather than nil on purpose: a nil stored
// in ifaceMetrics is a panic waiting for the first counter increment, and the
// increments run from goroutines no test controls (link_queue.go).
type capturingGaugeRegistry struct {
	ownedDevicesVec *capturingGaugeVec
	counterVecs     map[string]*capturingCounterVec
}

func newCapturingGaugeRegistry() *capturingGaugeRegistry {
	return &capturingGaugeRegistry{counterVecs: map[string]*capturingCounterVec{}}
}

func (r *capturingGaugeRegistry) Counter(string, string) metrics.Counter { return nil }
func (r *capturingGaugeRegistry) Gauge(string, string) metrics.Gauge     { return nil }
func (r *capturingGaugeRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	v := newCapturingCounterVec()
	r.counterVecs[name] = v
	return v
}
func (r *capturingGaugeRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	v := &capturingGaugeVec{gauges: map[string]*capturingGauge{}}
	if name == "ze_iface_owned_devices" {
		r.ownedDevicesVec = v
	}
	return v
}
func (r *capturingGaugeRegistry) Histogram(string, string, []float64) metrics.Histogram { return nil }
func (r *capturingGaugeRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nil
}

type capturingGaugeVec struct {
	gauges map[string]*capturingGauge
}

func (v *capturingGaugeVec) With(labels ...string) metrics.Gauge {
	key := labels[0]
	if _, ok := v.gauges[key]; !ok {
		v.gauges[key] = &capturingGauge{}
	}
	return v.gauges[key]
}

func (v *capturingGaugeVec) Delete(labels ...string) bool {
	_, ok := v.gauges[labels[0]]
	delete(v.gauges, labels[0])
	return ok
}

func (v *capturingGaugeVec) value(key string) float64 {
	if g, ok := v.gauges[key]; ok {
		return g.v
	}
	return -1
}

type capturingGauge struct{ v float64 }

func (g *capturingGauge) Set(v float64) { g.v = v }
func (g *capturingGauge) Inc()          { g.v++ }
func (g *capturingGauge) Dec()          { g.v-- }
func (g *capturingGauge) Add(d float64) { g.v += d }
