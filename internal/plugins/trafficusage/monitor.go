// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage monitor: reconcile, lifecycle, poller

package trafficusage

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// portProto is the key for the per-(port,protocol) byte maps. port is in host
// byte order (the BPF program applies ntohs); protocol is the raw IP protocol
// number. It mirrors the kernel struct port_proto_key {u16 port; u8 proto; u8 pad}.
type portProto struct {
	port  uint16
	proto uint8
}

// counts is one interface's accounting snapshot read from the BPF maps. IP keys
// are IPv4 or IPv6 addresses read from the per-family IP maps. Byte values are
// absolute cumulative totals.
type counts struct {
	ifname      string
	ingressIP   map[netip.Addr]uint64 // source IP -> bytes (track-ip only)
	egressIP    map[netip.Addr]uint64 // dest IP -> bytes (track-ip only)
	ingressPort map[portProto]uint64  // dest (port,proto) -> bytes
	egressPort  map[portProto]uint64  // source (port,proto) -> bytes
	mapEntries  map[string]int        // map name -> live entry count
}

// attachment is one interface's loaded and attached eBPF state.
type attachment interface {
	// Counts reads the BPF maps and returns the current absolute byte counters.
	Counts() (counts, error)
	// Close detaches the TCX links and closes the maps.
	Close() error
}

// attacher loads and attaches the ingress+egress eBPF programs for an interface.
// The production implementation is platform-specific (attach_linux.go /
// attach_other.go); tests inject a fake.
type attacher interface {
	// Available prepares this process to load eBPF programs and reports whether
	// it can. The monitor asks it only before attaching a configured interface,
	// and a non-nil error is that interface's outcome, classified like an
	// Attach error.
	Available() error
	// Attach builds, loads, and attaches ingress+egress programs to the given
	// ifindex, returning the live attachment.
	Attach(ifindex int, ifname string, maxEntries uint32, trackIP bool) (attachment, error)
}

// ifaceResolver resolves a ze interface name to its OS device: the ifindex,
// whether it is up, and whether it is present. Production uses iface.Resolve
// (which honors the os-name / mac-match selectors); tests inject a stub.
type ifaceResolver func(name string) (index int, up, present bool)

// Monitor orchestrates per-interface eBPF attachments and (from Phase 5) the
// metrics poller. It is platform-neutral: all kernel interaction is behind the
// attacher interface.
type Monitor struct {
	att     attacher
	resolve ifaceResolver

	// remedy names the fix for a permission refusal. Production binds
	// privilege.CapabilityRemedy; a test stands in its own.
	remedy func(missing ...string) string

	mu       sync.Mutex
	cfg      *Config
	attached map[string]attachedIface // by interface name
	outcomes map[string]attachOutcome // by configured interface name: what the last attempt established
	lastSeen map[seriesID]seriesEntry // published series -> last-seen + deleter

	// poller lifecycle
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// attachedIface is a live attachment plus the config parameters it was built
// with, so reconcile can detect a track-ip or max-entries change (which require
// rebuilding the eBPF program) and re-attach.
type attachedIface struct {
	att        attachment
	trackIP    bool
	maxEntries uint32
}

// attachOutcome is what the last attempt established about one configured
// interface: the state, the ifindex and program parameters it was reached
// with, and for a failure the kernel's answer and, for a permission refusal,
// the remedy. The doctor check reads it; a change of it is logged once.
type attachOutcome struct {
	state      attachState
	ifindex    int
	trackIP    bool
	maxEntries uint32
	cause      string
	remedy     string
}

// settled reports whether a failure is one a retry on the same interface and
// program cannot change: the process's capabilities and the kernel's TCX
// support are fixed for the life of the process. A snapshot tick skips such an
// interface; a configuration apply retries it.
func (o attachOutcome) settled(ifindex int, ifc InterfaceConfig) bool {
	if o.state != attachStateRefused && o.state != attachStateAbsent {
		return false
	}
	if o.ifindex != ifindex {
		return false
	}
	return o.trackIP == ifc.TrackIP && o.maxEntries == ifc.MaxEntries
}

// seriesID identifies one published metric series for stale tracking: a metric
// family and up to three label values.
type seriesID struct {
	family  string
	a, b, c string
}

// seriesEntry records when a series was last published and how to delete it.
type seriesEntry struct {
	seen time.Time
	del  func()
}

// newMonitor creates a monitor bound to an attacher and an ifindex resolver.
// A permission refusal's fix comes from privilege.CapabilityRemedy.
func newMonitor(att attacher, resolve ifaceResolver) *Monitor {
	return &Monitor{
		att:      att,
		resolve:  resolve,
		remedy:   privilege.CapabilityRemedy,
		attached: make(map[string]attachedIface),
		outcomes: make(map[string]attachOutcome),
		lastSeen: make(map[seriesID]seriesEntry),
	}
}

// Reconcile records a new configuration and immediately drives the attached set
// toward it, resolving each configured interface to an ifindex. It is idempotent
// and degrades gracefully -- an interface that cannot be resolved or attached is
// recorded, logged once, and skipped, leaving the others unaffected (AC-12). It
// runs on OnConfigure and OnConfigApply, so it is the startup and reload check:
// it retries every failure, a settled one included. The outcomes are what the
// traffic-usage-attach doctor check reports; it always returns nil.
func (m *Monitor) Reconcile(cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.reconcileLocked(m.resolve, true)
	return nil
}

// onSnapshot is the iface rate-tracker callback (~1 Hz). It is a periodic tick
// that re-resolves every configured interface through the shared resolver, so an
// interface going down or disappearing is detached and one coming back up is
// re-attached (AC-11). It does not retry a settled failure (attachOutcome.settled):
// a refusal or a missing TCX attach point would answer the same every second.
// The snapshot slice is unused: resolution goes through
// iface.Resolve to honor the os-name / mac-match selectors, and the resolver's
// cache is invalidated by the same link events that drive this callback.
func (m *Monitor) onSnapshot(_ []iface.InterfaceInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconcileLocked(m.resolve, false)
}

// reconcileLocked attaches configured interfaces that are present and up, and
// detaches interfaces that are unconfigured, absent, or down. lookup reports an
// interface's (ifindex, up, present). retrySettled retries a failure a retry on
// the same interface cannot change. It records each configured interface's
// outcome, and asks the attacher whether eBPF can run here only when an
// interface is to be attached, so a disabled traffic-usage probes nothing.
// Caller must hold m.mu.
func (m *Monitor) reconcileLocked(lookup func(name string) (idx int, up, present bool), retrySettled bool) {
	desired := make(map[string]InterfaceConfig)
	if m.cfg != nil && m.cfg.Enabled {
		for _, ifc := range m.cfg.Interfaces {
			desired[ifc.Name] = ifc
		}
	}

	for name, a := range m.attached {
		ifc, want := desired[name]
		_, up, present := lookup(name)
		// Detach when no longer desired, absent, or down -- or when track-ip /
		// max-entries changed, since those parameters are baked into the loaded
		// eBPF program and only a rebuild (detach + re-attach below) applies them.
		stale := want && (a.trackIP != ifc.TrackIP || a.maxEntries != ifc.MaxEntries)
		if want && present && up && !stale {
			continue
		}
		if err := a.att.Close(); err != nil {
			logger().Warn("traffic-usage: detach failed", "interface", name, "error", err)
		}
		delete(m.attached, name)
		// Drop the interface's metric series now rather than waiting for the
		// stale-timeout (which may be 0/disabled): a detached interface is no
		// longer accounted, and on re-attach its eBPF maps start fresh so the old
		// values would be stale. Series are republished on the next poll if the
		// interface is still desired and comes back up.
		m.deleteInterfaceSeriesLocked(name)
	}

	// An interface no longer configured has no outcome to report.
	for name := range m.outcomes {
		if _, want := desired[name]; !want {
			delete(m.outcomes, name)
		}
	}

	// availability is asked once per pass, and only once an interface is to
	// be attached; availabilityAsked guards it, because a nil availability is
	// also the answer "eBPF can run here".
	var availability error
	availabilityAsked := false
	for name, ifc := range desired {
		if _, ok := m.attached[name]; ok {
			continue
		}
		idx, up, present := lookup(name)
		if !present {
			m.recordLocked(name, attachOutcome{state: attachStateMissing})
			continue
		}
		if !up {
			m.recordLocked(name, attachOutcome{state: attachStateDown, ifindex: idx})
			continue
		}
		if previous, ok := m.outcomes[name]; ok && !retrySettled && previous.settled(idx, ifc) {
			continue
		}
		if !availabilityAsked {
			availability = m.att.Available()
			availabilityAsked = true
		}
		err := availability
		var att attachment
		if err == nil {
			att, err = m.att.Attach(idx, name, ifc.MaxEntries, ifc.TrackIP)
		}
		if err != nil {
			m.recordLocked(name, m.failedOutcome(idx, ifc, err))
			continue
		}
		m.attached[name] = attachedIface{att: att, trackIP: ifc.TrackIP, maxEntries: ifc.MaxEntries}
		m.recordLocked(name, attachOutcome{state: attachStateAttached, ifindex: idx, trackIP: ifc.TrackIP, maxEntries: ifc.MaxEntries})
		logger().Info("traffic-usage: attached", "interface", name, "ifindex", idx, "track-ip", ifc.TrackIP)
	}
}

// failedOutcome builds the outcome of a failed attach from the kernel's answer.
// Only a permission refusal carries a remedy: it is the one failure a change
// of the daemon's capabilities clears.
func (m *Monitor) failedOutcome(idx int, ifc InterfaceConfig, err error) attachOutcome {
	outcome := attachOutcome{
		state:      classifyAttachError(err),
		ifindex:    idx,
		trackIP:    ifc.TrackIP,
		maxEntries: ifc.MaxEntries,
		cause:      err.Error(),
	}
	if outcome.state == attachStateRefused {
		outcome.remedy = m.remedy("CAP_BPF", "CAP_NET_ADMIN")
	}
	return outcome
}

// recordLocked stores an interface's outcome and logs it when it differs from
// the one recorded before, so a failure that repeats every snapshot is logged
// once per change rather than once a second. Each failure line carries the
// code the doctor check reports it under. Caller must hold m.mu.
func (m *Monitor) recordLocked(name string, outcome attachOutcome) {
	previous, known := m.outcomes[name]
	m.outcomes[name] = outcome
	if known && previous == outcome {
		return
	}
	switch outcome.state {
	case attachStateAttached, attachStateDown:
	case attachStateMissing:
		logger().Warn("traffic-usage: configured interface not found; it is not accounted until it appears",
			"interface", name, "code", codeInterfaceMissing)
	case attachStateRefused:
		logger().Error("traffic-usage: the kernel refused permission to load or attach the accounting programs; the interface is not accounted",
			"interface", name, "ifindex", outcome.ifindex, "error", outcome.cause, "fix", outcome.remedy, "code", codeAttachRefused)
	case attachStateAbsent:
		logger().Warn("traffic-usage: this kernel cannot attach eBPF TCX programs; the interface is not accounted",
			"interface", name, "ifindex", outcome.ifindex, "error", outcome.cause, "code", codeTCXAbsent)
	case attachStateUnknown, attachStateUnspecified:
		logger().Error("traffic-usage: attach failed",
			"interface", name, "ifindex", outcome.ifindex, "error", outcome.cause, "code", codeAttachUnknown)
	}
}

// Snapshot reads the current counters for every attached interface. Used by the
// `show traffic-usage` handler.
func (m *Monitor) Snapshot() []counts {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *Monitor) snapshotLocked() []counts {
	out := make([]counts, 0, len(m.attached))
	for name, a := range m.attached {
		c, err := a.att.Counts()
		if err != nil {
			logger().Warn("traffic-usage: read counts failed", "interface", name, "error", err)
			continue
		}
		c.ifname = name
		out = append(out, c)
	}
	return out
}

// Start launches the metrics poller. It is idempotent.
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})
	m.mu.Unlock()
	go m.pollLoop()
}

// pollLoop reads the maps and publishes metrics every cfg.Interval, re-reading
// the interval each cycle so a reload takes effect on the next tick.
func (m *Monitor) pollLoop() {
	defer close(m.doneCh)
	for {
		m.mu.Lock()
		interval := defaultInterval
		if m.cfg != nil && m.cfg.Interval > 0 {
			interval = m.cfg.Interval
		}
		m.mu.Unlock()

		timer := time.NewTimer(interval)
		select {
		case <-m.stopCh:
			timer.Stop()
			return
		case <-timer.C:
			m.poll()
		}
	}
}

func (m *Monitor) poll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.publishLocked(m.snapshotLocked(), now)
	m.staleCleanupLocked(now)
}

// stopPoller signals the poller to stop and waits for it to exit.
func (m *Monitor) stopPoller() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	done := m.doneCh
	m.mu.Unlock()
	<-done
}

// Stop halts the poller, detaches every interface, and releases all resources.
func (m *Monitor) Stop() {
	m.stopPoller()
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, a := range m.attached {
		if err := a.att.Close(); err != nil {
			logger().Warn("traffic-usage: detach failed", "interface", name, "error", err)
		}
		delete(m.attached, name)
	}
}

// publishLocked sets every metric series from the snapshot and records each as
// last-seen at now. Per-IP series are present only when track-ip populated the
// IP maps. Caller holds m.mu.
func (m *Monitor) publishLocked(snap []counts, now time.Time) {
	pm := metricsPtr.Load()
	if pm == nil {
		return
	}
	feed := observation.Global()
	for _, c := range snap {
		ifname := c.ifname
		for k, v := range c.ingressPort {
			port, proto := portStr(k.port), protoName(k.proto)
			pm.ingressPortBytes.With(ifname, port, proto).Set(float64(v))
			m.lastSeen[seriesID{"port_in", ifname, port, proto}] = seriesEntry{now, func() { pm.ingressPortBytes.Delete(ifname, port, proto) }}
		}
		for k, v := range c.egressPort {
			port, proto := portStr(k.port), protoName(k.proto)
			pm.egressPortBytes.With(ifname, port, proto).Set(float64(v))
			m.lastSeen[seriesID{"port_out", ifname, port, proto}] = seriesEntry{now, func() { pm.egressPortBytes.Delete(ifname, port, proto) }}
		}
		for k, v := range c.ingressIP {
			ip := k.String()
			pm.ingressBytes.With(ifname, ip).Set(float64(v))
			m.lastSeen[seriesID{"ip_in", ifname, ip, ""}] = seriesEntry{now, func() { pm.ingressBytes.Delete(ifname, ip) }}
			obs := observation.Observation{
				Kind:    observation.KindSourceIP,
				Iface:   ifname,
				Feature: observation.FeatureRxBytes,
				Value:   float64(v),
				At:      now,
			}
			obs.Flow.Src = k
			feed.Publish(obs)
		}
		for k, v := range c.egressIP {
			ip := k.String()
			pm.egressBytes.With(ifname, ip).Set(float64(v))
			m.lastSeen[seriesID{"ip_out", ifname, ip, ""}] = seriesEntry{now, func() { pm.egressBytes.Delete(ifname, ip) }}
		}
		for mapName, n := range c.mapEntries {
			pm.mapEntries.With(ifname, mapName).Set(float64(n))
			m.lastSeen[seriesID{"map", ifname, mapName, ""}] = seriesEntry{now, func() { pm.mapEntries.Delete(ifname, mapName) }}
		}
	}
}

// staleCleanupLocked deletes metric series not seen within their interface's
// stale-timeout. Each series is keyed by interface name (seriesID.a), so the
// per-interface timeout applies; a series whose interface is no longer
// configured falls back to the global stale-timeout. A timeout of 0 disables
// cleanup for that interface. Caller holds m.mu.
func (m *Monitor) staleCleanupLocked(now time.Time) {
	if m.cfg == nil {
		return
	}
	timeouts := make(map[string]time.Duration, len(m.cfg.Interfaces))
	for _, ifc := range m.cfg.Interfaces {
		timeouts[ifc.Name] = ifc.StaleTimeout
	}
	for id, e := range m.lastSeen {
		timeout, ok := timeouts[id.a]
		if !ok {
			timeout = m.cfg.StaleTimeout // removed interface: global fallback
		}
		if timeout <= 0 {
			continue
		}
		if e.seen.Before(now.Add(-timeout)) {
			e.del()
			delete(m.lastSeen, id)
		}
	}
}

// deleteInterfaceSeriesLocked removes every published metric series belonging to
// an interface (keyed by interface name in seriesID.a) and its lastSeen
// bookkeeping. Called on detach so the series disappear immediately instead of
// lingering until the stale-timeout. Caller holds m.mu.
func (m *Monitor) deleteInterfaceSeriesLocked(name string) {
	for id, e := range m.lastSeen {
		if id.a == name {
			e.del()
			delete(m.lastSeen, id)
		}
	}
}

// portStr renders a port number as a metric label value.
func portStr(p uint16) string { return textbuf.StringUint(uint64(p)) }

// activeMonitor is the running monitor for the current configuration, or nil
// when the plugin is idle (no traffic-usage section). The show handler reads it.
var activeMonitor atomic.Pointer[Monitor]

// getMonitor returns the active monitor, or nil if the plugin is unconfigured.
func getMonitor() *Monitor { return activeMonitor.Load() }

// resolveBinding resolves a ze interface name to its OS device via the shared
// iface resolver, which honors the os-name / mac-match selectors. It returns the
// OS ifindex, whether the device is up, and whether it is present. Returns
// (0,false,false) when no backend is loaded or the device is absent.
func resolveBinding(name string) (index int, up, present bool) {
	b, err := iface.Resolve(name)
	if err != nil || b.Ifindex <= 0 {
		return 0, false, false
	}
	return b.Ifindex, b.State == "up", true
}
