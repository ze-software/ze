// VALIDATES: monitor reconcile attaches configured interfaces by resolved ifindex,
// detaches ones dropped from config, and degrades gracefully when an interface
// cannot be resolved.
// PREVENTS: leaked attachments on config change; a single missing interface
// aborting the whole reconcile (AC-12); attaching by name instead of ifindex.

package trafficusage

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/observation"
)

type fakeAttachment struct {
	ifindex int
	ifname  string
	owner   *fakeAttacher
}

func (a *fakeAttachment) Counts() (counts, error) {
	c := a.owner.result
	c.ifname = a.ifname
	return c, nil
}

func (a *fakeAttachment) Close() error {
	// Model real detachment: the interface is no longer attached at the "kernel".
	delete(a.owner.attached, a.ifindex)
	a.owner.closed = append(a.owner.closed, a.ifindex)
	return nil
}

// fakeAttachCall records the parameters of one Attach call.
type fakeAttachCall struct {
	ifindex    int
	ifname     string
	maxEntries uint32
	trackIP    bool
}

type fakeAttacher struct {
	attached  map[int]string   // ifindex -> ifname (currently attached)
	closed    []int            // ifindexes closed, in order
	attaches  []fakeAttachCall // every Attach call, in order
	failOn    map[string]bool
	available error
	result    counts // counts every attachment reports from Counts()
}

func newFakeAttacher() *fakeAttacher {
	return &fakeAttacher{attached: map[int]string{}, failOn: map[string]bool{}}
}

func (f *fakeAttacher) Available() error { return f.available }

func (f *fakeAttacher) Attach(ifindex int, ifname string, maxEntries uint32, trackIP bool) (attachment, error) {
	if f.failOn[ifname] {
		return nil, errors.New("attach failed")
	}
	f.attaches = append(f.attaches, fakeAttachCall{ifindex, ifname, maxEntries, trackIP})
	f.attached[ifindex] = ifname
	return &fakeAttachment{ifindex: ifindex, ifname: ifname, owner: f}, nil
}

// ifcList builds InterfaceConfig entries with the given max-entries and track-ip
// for the named interfaces (the monitor only cares about the per-interface
// attach parameters).
func ifcList(maxEntries uint32, trackIP bool, names ...string) []InterfaceConfig {
	out := make([]InterfaceConfig, len(names))
	for i, n := range names {
		out[i] = InterfaceConfig{Name: n, MaxEntries: maxEntries, TrackIP: trackIP}
	}
	return out
}

// stubResolver resolves names from a fixed map; a mapped name is reported
// present and up.
func stubResolver(m map[string]int) ifaceResolver {
	return func(name string) (int, bool, bool) {
		i, ok := m[name]
		return i, ok, ok
	}
}

func TestReconcileAttachesConfigured(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))
	cfg := &Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")}

	if err := m.Reconcile(cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(fa.attached) != 2 || fa.attached[10] != "eth0" || fa.attached[11] != "eth1" {
		t.Fatalf("attached = %v, want {10:eth0, 11:eth1}", fa.attached)
	}
	// Reconcile is idempotent: applying the same config attaches nothing new.
	if err := m.Reconcile(cfg); err != nil {
		t.Fatalf("Reconcile (idempotent): %v", err)
	}
	if len(fa.attached) != 2 {
		t.Fatalf("attached after idempotent reconcile = %v, want 2 entries", fa.attached)
	}
}

func TestReconcileRemovesUnconfigured(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))

	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")})
	if err := m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, ok := fa.attached[11]; ok {
		t.Errorf("eth1 (ifindex 11) still attached after removal")
	}
	if len(fa.closed) != 1 || fa.closed[0] != 11 {
		t.Errorf("closed = %v, want [11]", fa.closed)
	}
}

func TestReconcileDisableDetachesAll(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	if err := m.Reconcile(&Config{Enabled: false}); err != nil {
		t.Fatalf("Reconcile disable: %v", err)
	}
	if len(fa.attached) != 0 {
		t.Errorf("attached after disable = %v, want empty", fa.attached)
	}
}

func TestReconcileSkipsUnresolved(t *testing.T) {
	fa := newFakeAttacher()
	// Only eth0 resolves; eth9 is absent from the system.
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	cfg := &Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth9")}
	if err := m.Reconcile(cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, ok := fa.attached[10]; !ok {
		t.Error("eth0 should be attached despite eth9 being unresolved")
	}
}

func TestReconcileContinuesOnAttachError(t *testing.T) {
	fa := newFakeAttacher()
	fa.failOn["eth0"] = true
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))
	cfg := &Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")}
	if err := m.Reconcile(cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, ok := fa.attached[11]; !ok {
		t.Error("eth1 should attach even though eth0 failed (graceful degradation, AC-12)")
	}
}

func TestReconcileReattachesOnTrackIPChange(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))

	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	if len(fa.attaches) != 1 || fa.attaches[0].trackIP {
		t.Fatalf("first attach = %+v, want one attach with track-ip=false", fa.attaches)
	}

	// Toggling track-ip on the still-configured interface must rebuild the eBPF
	// program: detach the old attachment and re-attach with track-ip=true. The
	// flag is baked into the loaded program, so a skip would silently never
	// produce per-IP metrics (the bug this guards against).
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, true, "eth0")})
	if len(fa.attaches) != 2 {
		t.Fatalf("attaches = %d, want 2 (re-attach on track-ip change)", len(fa.attaches))
	}
	if !fa.attaches[1].trackIP {
		t.Error("re-attach track-ip = false, want true")
	}
	if len(fa.closed) != 1 || fa.closed[0] != 10 {
		t.Errorf("closed = %v, want [10] (old attachment detached before re-attach)", fa.closed)
	}

	// Re-applying the same config must not churn (idempotent).
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, true, "eth0")})
	if len(fa.attaches) != 2 {
		t.Errorf("attaches = %d after idempotent reconcile, want 2", len(fa.attaches))
	}
}

func TestReconcileReattachesOnMaxEntriesChange(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(4096, false, "eth0")})
	if len(fa.attaches) != 2 || fa.attaches[1].maxEntries != 4096 {
		t.Fatalf("expected re-attach with max-entries=4096, got attaches=%+v", fa.attaches)
	}
}

// onSnapshot no longer reads the snapshot slice -- the lifecycle is
// now driven by re-resolving each configured name via the injected resolver
// (iface.Resolve in production, honoring os-name/mac-match). The down/up/absent
// coverage below is preserved, driven through a mutable stub resolver instead of
// crafted InterfaceInfo slices.
func TestOnInterfaceDownReattach(t *testing.T) {
	fa := newFakeAttacher()
	up := true
	resolve := func(name string) (int, bool, bool) {
		if name != "eth0" {
			return 0, false, false
		}
		return 10, up, true
	}
	m := newMonitor(fa, resolve)
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	if _, ok := fa.attached[10]; !ok {
		t.Fatal("eth0 should be attached after reconcile")
	}

	// Interface goes down -> the 1 Hz tick re-resolves and detaches it.
	up = false
	m.onSnapshot(nil)
	if _, ok := fa.attached[10]; ok {
		t.Error("eth0 should be detached when it goes down (AC-11)")
	}

	// Interface comes back up -> re-attach; a second tick must not double-attach.
	up = true
	m.onSnapshot(nil)
	m.onSnapshot(nil)
	if _, ok := fa.attached[10]; !ok {
		t.Error("eth0 should be re-attached when it comes back up (AC-11)")
	}
}

func TestOnSnapshotDetachesAbsent(t *testing.T) {
	fa := newFakeAttacher()
	present := true
	resolve := func(name string) (int, bool, bool) {
		if name == "eth0" && present {
			return 10, true, true
		}
		return 0, false, false
	}
	m := newMonitor(fa, resolve)
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0")})
	// eth0 disappears entirely (deleted) -> resolver reports absent -> detach.
	present = false
	m.onSnapshot(nil)
	if _, ok := fa.attached[10]; ok {
		t.Error("eth0 should be detached when it disappears")
	}
}

func TestPublishAndStaleCleanup(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{Enabled: true, Interval: time.Second, StaleTimeout: 5 * time.Minute, MaxEntries: 1024, TrackIP: true}
	t0 := time.Unix(1000, 0)

	snap := []counts{{
		ifname:      "eth0",
		ingressPort: map[portProto]uint64{{port: 443, proto: 6}: 1000},
		ingressIP:   map[uint32]uint64{0x0100000a: 2000}, // 10.0.0.1
		mapEntries:  map[string]int{"port_ingress": 1},
	}}
	m.publishLocked(snap, t0)
	if len(m.lastSeen) != 3 {
		t.Fatalf("lastSeen = %d series, want 3 (port, ip, map-entries)", len(m.lastSeen))
	}

	// Republish keeps the series alive.
	m.publishLocked(snap, t0.Add(time.Minute))
	m.staleCleanupLocked(t0.Add(time.Minute))
	if len(m.lastSeen) != 3 {
		t.Errorf("series cleaned while still fresh: %d remain", len(m.lastSeen))
	}

	// Past the stale-timeout with no republish -> every series removed.
	m.staleCleanupLocked(t0.Add(10 * time.Minute))
	if len(m.lastSeen) != 0 {
		t.Errorf("stale series not cleaned: %d remain", len(m.lastSeen))
	}
}

func TestStaleCleanupPerInterface(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{
		Enabled: true,
		Interfaces: []InterfaceConfig{
			{Name: "fast", StaleTimeout: time.Minute, MaxEntries: 1024},
			{Name: "slow", StaleTimeout: time.Hour, MaxEntries: 1024},
		},
	}
	t0 := time.Unix(1000, 0)
	m.publishLocked([]counts{
		{ifname: "fast", ingressPort: map[portProto]uint64{{port: 80, proto: 6}: 1}},
		{ifname: "slow", ingressPort: map[portProto]uint64{{port: 80, proto: 6}: 1}},
	}, t0)
	if len(m.lastSeen) != 2 {
		t.Fatalf("lastSeen = %d, want 2", len(m.lastSeen))
	}

	// 2 minutes later: fast (1m timeout) expires, slow (1h timeout) survives --
	// each series honors its own interface's stale-timeout.
	m.staleCleanupLocked(t0.Add(2 * time.Minute))
	seen := map[string]bool{}
	for id := range m.lastSeen {
		seen[id.a] = true
	}
	if seen["fast"] {
		t.Error("fast interface series should be cleaned (1m timeout, 2m elapsed)")
	}
	if !seen["slow"] {
		t.Error("slow interface series should survive (1h timeout)")
	}
}

func TestReconcileDeletesSeriesOnRemoval(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(map[string]int{"eth0": 10}))
	// stale-timeout 0 (cleanup disabled): without per-detach deletion, a removed
	// interface's series would leak forever.
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, StaleTimeout: 0, Interfaces: ifcList(1024, false, "eth0")})
	m.mu.Lock()
	m.publishLocked([]counts{{ifname: "eth0", ingressPort: map[portProto]uint64{{port: 80, proto: 6}: 1}}}, time.Unix(1000, 0))
	had := len(m.lastSeen)
	m.mu.Unlock()
	if had == 0 {
		t.Fatal("expected published series for eth0 before removal")
	}

	// Remove eth0 from config -> its series must be dropped immediately, not
	// linger until a (disabled) stale-timeout.
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, StaleTimeout: 0, Interfaces: ifcList(1024, false, "eth9")})
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.lastSeen {
		if id.a == "eth0" {
			t.Errorf("eth0 series should be deleted on removal (stale-timeout 0), found %+v", id)
		}
	}
}

// VALIDATES: AC-4 — trafficusage publishes per-source-IP observations to the feed.
func TestTrafficUsagePublishesSourceObs(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{Enabled: true, Interval: time.Second}

	feed := observation.Global()
	var received []observation.Observation
	var mu sync.Mutex
	subID := feed.Subscribe("test", func(obs observation.Observation) {
		mu.Lock()
		received = append(received, obs)
		mu.Unlock()
	})
	defer feed.Unsubscribe(subID)

	snap := []counts{{
		ifname:    "eth0",
		ingressIP: map[uint32]uint64{0x0100000a: 42}, // 10.0.0.1 in LE
	}}
	m.mu.Lock()
	m.publishLocked(snap, time.Now())
	m.mu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("no observations published")
	}
	obs := received[0]
	if obs.Kind != observation.KindSourceIP {
		t.Errorf("kind = %v, want SourceIP", obs.Kind)
	}
	if obs.Feature != observation.FeatureRxBytes {
		t.Errorf("feature = %v, want RxBytes", obs.Feature)
	}
	if obs.Value != 42 {
		t.Errorf("value = %f, want 42", obs.Value)
	}
	if obs.Iface != "eth0" {
		t.Errorf("iface = %q, want eth0", obs.Iface)
	}
}

func TestStaleCleanupDisabled(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{Enabled: true, StaleTimeout: 0} // 0 disables cleanup
	t0 := time.Unix(1000, 0)
	m.publishLocked([]counts{{ifname: "eth0", ingressPort: map[portProto]uint64{{port: 80, proto: 6}: 100}}}, t0)
	m.staleCleanupLocked(t0.Add(24 * time.Hour))
	if len(m.lastSeen) == 0 {
		t.Error("stale-timeout=0 must disable cleanup, but series were removed")
	}
}

func TestTrackIPToggle(t *testing.T) {
	BindMetrics(metrics.NewPrometheusRegistry())
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{Enabled: true, TrackIP: false}
	t0 := time.Unix(1000, 0)
	// track-ip off: the attacher leaves the IP maps nil, so only port and
	// map-entries series are published -- never per-IP (AC-17).
	m.publishLocked([]counts{{
		ifname:      "eth0",
		ingressPort: map[portProto]uint64{{port: 80, proto: 6}: 100},
		mapEntries:  map[string]int{"port_ingress": 1},
	}}, t0)
	for id := range m.lastSeen {
		if id.family == "ip_in" || id.family == "ip_out" {
			t.Errorf("per-IP series published with track-ip off: %+v", id)
		}
	}
	if len(m.lastSeen) == 0 {
		t.Error("expected port/map-entries series to be published")
	}
}

func TestStopClosesAll(t *testing.T) {
	fa := newFakeAttacher()
	m := newMonitor(fa, stubResolver(map[string]int{"eth0": 10, "eth1": 11}))
	_ = m.Reconcile(&Config{Enabled: true, Interval: time.Second, Interfaces: ifcList(1024, false, "eth0", "eth1")})
	m.Stop()
	if len(fa.closed) != 2 {
		t.Errorf("closed = %v, want 2 attachments closed on Stop", fa.closed)
	}
}
