// VALIDATES: monitor reconcile attaches configured interfaces by resolved ifindex,
// detaches ones dropped from config, and degrades gracefully when an interface
// cannot be resolved.
// PREVENTS: leaked attachments on config change; a single missing interface
// aborting the whole reconcile (AC-12); attaching by name instead of ifindex.

package trafficusage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
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
	attached       map[int]string   // ifindex -> ifname (currently attached)
	closed         []int            // ifindexes closed, in order
	attaches       []fakeAttachCall // every successful Attach call, in order
	attempts       int              // every Attach call, failed ones included
	availableCalls int              // every Available call
	failOn         map[string]bool
	failWith       map[string]error // the error Attach answers for an interface
	available      error
	result         counts // counts every attachment reports from Counts()
}

func newFakeAttacher() *fakeAttacher {
	return &fakeAttacher{attached: map[int]string{}, failOn: map[string]bool{}, failWith: map[string]error{}}
}

func (f *fakeAttacher) Available() error {
	f.availableCalls++
	return f.available
}

func (f *fakeAttacher) Attach(ifindex int, ifname string, maxEntries uint32, trackIP bool) (attachment, error) {
	f.attempts++
	if err, ok := f.failWith[ifname]; ok {
		return nil, err
	}
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
		ingressIP:   map[netip.Addr]uint64{netip.MustParseAddr("10.0.0.1"): 2000},
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

	srcV4 := netip.MustParseAddr("10.0.0.1")
	snap := []counts{{
		ifname:    "eth0",
		ingressIP: map[netip.Addr]uint64{srcV4: 42},
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
	if obs.Flow.Src != srcV4 {
		t.Errorf("flow.src = %v, want %v", obs.Flow.Src, srcV4)
	}
}

// VALIDATES: AC-6 — IPv4 and IPv6 netip.Addr entries publish to Prometheus
// ze_traffic_usage_* series (including icmpv6 and ip6_ingress/ip6_egress map
// entries), flow into observation.Global() with IPv6 Flow.Src, and expire under
// stale-timeout.
func TestMonitorPublishesIPv6MetricsAndObservations(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	BindMetrics(reg)
	m := newMonitor(newFakeAttacher(), stubResolver(nil))
	m.cfg = &Config{Enabled: true, Interval: time.Second, StaleTimeout: 5 * time.Minute, MaxEntries: 1024, TrackIP: true}

	feed := observation.Global()
	var received []observation.Observation
	var mu sync.Mutex
	subID := feed.Subscribe("test-ipv6", func(obs observation.Observation) {
		mu.Lock()
		received = append(received, obs)
		mu.Unlock()
	})
	defer feed.Unsubscribe(subID)

	srcV4 := netip.MustParseAddr("10.0.0.1")
	srcV6 := netip.MustParseAddr("2001:db8::1")
	dstV6 := netip.MustParseAddr("2001:db8::2")
	t0 := time.Unix(2000, 0)

	snap := []counts{{
		ifname: "eth0",
		ingressPort: map[portProto]uint64{
			{port: 0, proto: 58}: 512,
		},
		ingressIP: map[netip.Addr]uint64{
			srcV4: 1024,
			srcV6: 4096,
		},
		egressIP: map[netip.Addr]uint64{
			dstV6: 2048,
		},
		mapEntries: map[string]int{
			"ip_ingress":  1,
			"ip6_ingress": 1,
			"ip6_egress":  1,
		},
	}}
	m.mu.Lock()
	m.publishLocked(snap, t0)
	m.mu.Unlock()

	scrape := func() string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
		reg.Handler().ServeHTTP(rec, req)
		return rec.Body.String()
	}

	body := scrape()
	for _, want := range []string{
		`ze_traffic_usage_ingress_bytes_total{interface="eth0",src_ip="10.0.0.1"} 1024`,
		`ze_traffic_usage_ingress_bytes_total{interface="eth0",src_ip="2001:db8::1"} 4096`,
		`ze_traffic_usage_egress_bytes_total{dst_ip="2001:db8::2",interface="eth0"} 2048`,
		`ze_traffic_usage_ingress_port_bytes_total{dst_port="0",interface="eth0",protocol="icmpv6"} 512`,
		`ze_traffic_usage_map_entries{interface="eth0",map="ip6_ingress"} 1`,
		`ze_traffic_usage_map_entries{interface="eth0",map="ip6_egress"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics missing %q; got:\n%s", want, body)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	var sawIPv6Obs bool
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, obs := range received {
			if obs.Kind == observation.KindSourceIP && obs.Feature == observation.FeatureRxBytes &&
				obs.Iface == "eth0" && obs.Flow.Src == srcV6 && obs.Value == 4096 {
				sawIPv6Obs = true
				break
			}
		}
		mu.Unlock()
		if sawIPv6Obs {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !sawIPv6Obs {
		t.Fatalf("observation.Global() did not receive IPv6 SourceIP observation for %s", srcV6)
	}

	// Past stale-timeout with no republish -> IPv6 and IPv4 series are deleted.
	m.mu.Lock()
	m.staleCleanupLocked(t0.Add(10 * time.Minute))
	remaining := len(m.lastSeen)
	m.mu.Unlock()
	if remaining != 0 {
		t.Errorf("stale series not cleaned: %d remain", remaining)
	}
	after := scrape()
	if strings.Contains(after, `src_ip="2001:db8::1"`) || strings.Contains(after, `dst_ip="2001:db8::2"`) {
		t.Errorf("stale IPv6 series still present after cleanup:\n%s", after)
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
