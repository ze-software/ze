// RFC: rfc/short/rfc9568.md -- VRRPv3 engine behavior (Sections 6.4, 7.1)
// RFC: rfc/short/rfc3768.md -- VRRPv2 engine behavior (Sections 6.4, 7.1)
//
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- per-instance worker: FSM executor + rx decode
//
// One worker goroutine owns one virtual router. It is the ONLY executor of the
// FSM's action values (spec-vrrp-2 is a pure function of events -> actions) and
// the only owner of the three clock.Timers, so the FSM itself never touches a
// clock, a socket, or the kernel.
//
// Timer staleness is handled by generation echo: every Start*Timer action
// carries a Gen; the worker echoes that Gen back in the matching *Expired event
// and the FSM drops an expiry whose Gen it no longer recognizes. That turns the
// classic "timer already fired while we were resetting it" race into a
// deterministic, testable rule (spec-vrrp-2 Timer Generations).
package vrrp

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// eventQueueDepth bounds the per-instance event queue. A VRRP instance handles
// roughly one advert per interval plus timer expiries, so this is orders of
// magnitude above steady state; it exists to absorb an advert flood without
// blocking the shared transport reader.
const eventQueueDepth = 64

// ownerPrefix namespaces this plugin's address-owner strings.
const ownerPrefix = "vrrp:"

// instance is one running virtual router: the FSM, its timers, its transport
// handle, and the kernel-side effects the FSM asks for.
type instance struct {
	// mu serializes every touch of spec and machine.
	//
	// fsm.Instance is documented single-threaded and holds no locks of its own,
	// but three goroutines reach an instance: its worker (timer expiries, rx
	// events), the config-apply caller (reconfigure), and a show handler
	// (snapshot). Serializing here preserves the FSM's one-caller-at-a-time
	// contract without pushing lock discipline into the pure state machine.
	// Held across action execution: an action is a socket write or a registry
	// call, never a callback into this instance, so it cannot deadlock.
	mu sync.Mutex

	spec GroupSpec
	key  transport.InstanceKey
	dev  string // macvlan device name (also the address-owner interface)
	own  string // address-owner string: "vrrp:<device>" (per-instance, see D-1)

	machine *fsm.Instance
	clk     clock.Clock
	deps    engineDeps

	events chan fsm.Event
	stop   chan struct{}
	done   chan struct{}

	// Timers are owned solely by this worker; nil when unarmed.
	masterDown clock.Timer
	advert     clock.Timer
	preempt    clock.Timer

	// started is true between a readiness-driven Startup and its Shutdown. It
	// gates the FSM so link churn cannot restart a running router, and so a
	// router never advertises from a parent it cannot actually serve.
	started bool

	// prio0Sent/prio0Received count Priority-0 advertisements. The transport
	// never parses payloads, so these are engine-owned (D-F).
	prio0Sent     uint64
	prio0Received uint64
}

// engineDeps are the seams the worker drives. Real implementations wrap the
// transport and the iface component; tests substitute fakes, which is what lets
// the whole action-execution contract be unit-tested off-Linux.
type engineDeps struct {
	sendAdvert     func(key transport.InstanceKey, priority uint8, intervalMs int) error
	updateAdvert   func(key transport.InstanceKey, params transport.AdvertParams) error
	announceMaster func(key transport.InstanceKey, vips []netip.Addr)
	installVIPs    func(dev, owner string, cidrs []string) error
	removeVIPs     func(owner string)
	recordRxError  func(key transport.InstanceKey, reason string)
	emitState      func(spec GroupSpec, from, to fsm.State, reason string)

	// setAcceptFilter publishes one instance's RFC 9568 Section 6.4.3
	// acceptance decision to the dataplane: accept false suppresses local
	// delivery for the virtual addresses, accept true withdraws the
	// suppression. The caller MUST call clearAcceptFilter when the instance
	// stops holding those addresses (acceptfilter.go).
	setAcceptFilter func(instanceOwner string, vips []netip.Addr, accept bool) error
	// clearAcceptFilter withdraws the instance's suppression entry. It MUST be
	// called after setAcceptFilter once the instance gives its addresses up, or
	// a drop rule outlives the state that asked for it.
	clearAcceptFilter func(instanceOwner string) error

	// parentReady reports whether the unit's device can host a virtual router:
	// operationally up, with an address of this family to source advertisements
	// from. Keyed on the PARENT, never on the macvlan: the kernel leaves a
	// macvlan's oper-state UP when its parent dies (measured, spec-vrrp-3 A-4
	// broken), so watching the macvlan would never notice a dead link.
	parentReady func(device, family string) bool
	// watchParent delivers a notification on every link change for the device.
	// The instance re-evaluates readiness from scratch on each one, so a coarse
	// "something changed" signal is enough and cannot go stale.
	watchParent func(device string) (<-chan struct{}, func())
	// refreshAddresses tells the transport to re-resolve the parent's source
	// address (RFC 9568 Section 7.2: adverts are sourced from the sending
	// interface's primary address, which the transport caches).
	refreshAddresses func(key transport.InstanceKey)
}

// newInstance builds a worker. The caller starts it with run().
func newInstance(spec GroupSpec, key transport.InstanceKey, dev string, clk clock.Clock, deps engineDeps) *instance {
	return &instance{
		spec:    spec,
		key:     key,
		dev:     dev,
		own:     ownerString(dev),
		machine: fsm.New(clk),
		clk:     clk,
		deps:    deps,
		events:  make(chan fsm.Event, eventQueueDepth),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// ownerString is the address-owner identity for one instance.
//
// Per-instance (not a shared "vrrp" owner) because UnregisterOwnedAddresses
// removes an owner from EVERY interface it holds
// (internal/component/iface/address_owner.go:122-137) and is the sole populator
// of staleIfaces (:126-129), which is what guarantees the kernel prune pass. A
// single shared owner therefore could not drop one instance's VIPs without
// dropping all of them (spec-vrrp-5 D-1).
func ownerString(dev string) string {
	var tb textbuf.Buffer
	return tb.Str(ownerPrefix).Str(dev).String()
}

// send queues an event. A full queue drops the event and says so: VRRP is
// soft-state, so a dropped advert is indistinguishable from one the network
// lost and the next interval re-establishes the truth -- but a queue this deep
// filling up means the worker is wedged, which an operator must see.
func (in *instance) send(ev fsm.Event) {
	select {
	case in.events <- ev:
	case <-in.stop:
	default:
		logger().Warn("vrrp: instance event queue full, dropping event",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID, "family", in.spec.Family,
			"event", eventName(ev), "depth", eventQueueDepth)
	}
}

// eventName labels an event for logs without reflection on the hot path.
func eventName(ev fsm.Event) string {
	switch ev.(type) {
	case fsm.Startup:
		return "startup"
	case fsm.Shutdown:
		return "shutdown"
	case fsm.AdvertReceived:
		return "advert-received"
	case fsm.MasterDownExpired:
		return "master-down-expired"
	case fsm.AdvertTimerExpired:
		return "advert-timer-expired"
	case fsm.PreemptDelayExpired:
		return "preempt-delay-expired"
	case fsm.ConfigUpdated:
		return "config-updated"
	default:
		return "unknown"
	}
}

// run is the worker loop: one goroutine per instance, per
// ai/rules/goroutine-lifecycle.md (goroutine per lifecycle, not per event).
func (in *instance) run() {
	defer close(in.done)

	// Watch the parent for link changes before the first readiness check, so a
	// change racing startup is not missed.
	var changes <-chan struct{}
	if in.deps.watchParent != nil {
		ch, cancel := in.deps.watchParent(in.spec.ParentDevice)
		changes = ch
		defer cancel()
	}

	in.evaluateReadiness()
	for {
		select {
		case <-in.stop:
			in.stopInstance()
			return
		case <-changes:
			in.evaluateReadiness()
		case ev := <-in.events:
			in.dispatch(ev)
		}
	}
}

// evaluateReadiness starts or stops the virtual router to match the parent's
// usability. It is idempotent: the iface monitor fires on every link change, and
// re-deciding from scratch each time is what keeps link churn from restarting a
// healthy router.
func (in *instance) evaluateReadiness() {
	in.mu.Lock()
	defer in.mu.Unlock()

	// A link change can also mean the parent's addresses moved, and the
	// transport caches the advertisement source (RFC 9568 Section 7.2).
	if in.deps.refreshAddresses != nil {
		in.deps.refreshAddresses(in.key)
	}

	ready := true
	if in.deps.parentReady != nil {
		ready = in.deps.parentReady(in.spec.ParentDevice, in.spec.Family)
	}
	switch {
	case ready && !in.started:
		logger().Info("vrrp: parent usable, starting virtual router",
			"interface", in.spec.Interface, "device", in.spec.ParentDevice,
			"group", in.spec.Name, "vrid", in.spec.VRID, "family", in.spec.Family)
		in.startupLocked()
	case !ready && in.started:
		// The parent is gone: stop claiming a link we cannot serve. A Master
		// still owes its peers the Priority-0 resignation, which Shutdown emits
		// (RFC 9568 Section 6.4.3) -- it may not reach them if the link is
		// truly dead, but it costs nothing and saves a full master-down
		// interval when the cause was administrative.
		logger().Warn("vrrp: parent unusable, stopping virtual router",
			"interface", in.spec.Interface, "device", in.spec.ParentDevice,
			"group", in.spec.Name, "vrid", in.spec.VRID, "family", in.spec.Family)
		in.shutdownLocked()
	}
}

// startup dispatches Startup under mu. Kept for callers that force a start
// (tests); readiness drives it in production.
func (in *instance) startup() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.startupLocked()
}

// startupLocked starts the FSM. Building the config inside the lock matters:
// reading spec to construct the event and then locking would let a concurrent
// reconfigure change spec in between.
func (in *instance) startupLocked() {
	if in.started {
		return
	}
	in.started = true
	in.dispatchLocked(fsm.Startup{Config: in.fsmConfig()})
}

// shutdownLocked stops the FSM and cancels its timers.
func (in *instance) shutdownLocked() {
	if !in.started {
		return
	}
	in.started = false
	in.dispatchLocked(fsm.Shutdown{})
	in.cancelTimers()
}

// stopInstance is the worker's exit path.
func (in *instance) stopInstance() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.shutdownLocked()
	in.cancelTimers()
}

// shutdown stops the worker and waits for its Shutdown actions (including the
// Priority-0 advertisement a Master owes its peers) to be executed.
func (in *instance) shutdown() {
	close(in.stop)
	<-in.done
}

// dispatch runs one event through the FSM and executes the resulting actions in
// order. Ordering matters: the FSM emits, for example, InstallVIPs before
// AnnounceFailover so the address exists before it is advertised to the link.
func (in *instance) dispatch(ev fsm.Event) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.dispatchLocked(ev)
}

// dispatchLocked is dispatch's body for callers that already hold mu.
func (in *instance) dispatchLocked(ev fsm.Event) {
	for _, act := range in.machine.Handle(ev) {
		in.execute(act)
	}
}

// execute performs one action's side effect.
func (in *instance) execute(act fsm.Action) {
	switch a := act.(type) {
	case fsm.SendAdvert:
		in.doSendAdvert(a.Priority, a.AdvertIntervalMs)
	case fsm.SendAdvertZeroPriority:
		// RFC 9568 Section 6.4.3: on Shutdown the Active router sends an
		// ADVERTISEMENT with Priority 0 so Backups promote immediately instead
		// of waiting out the master-down interval.
		in.doSendAdvert(0, int(in.spec.AdvertIntervalMs))
		in.prio0Sent++
	case fsm.InstallVIPs:
		in.doInstallVIPs(a.VIPs)
	case fsm.RemoveVIPs:
		in.doRemoveVIPs()
	case fsm.AnnounceFailover:
		in.deps.announceMaster(in.key, in.spec.VIPs)
	case fsm.StartMasterDownTimer:
		in.masterDown = in.armTimer(in.masterDown, a.Duration, func() {
			in.send(fsm.MasterDownExpired{Gen: a.Gen})
		})
	case fsm.StartAdvertTimer:
		in.advert = in.armTimer(in.advert, a.Interval, func() {
			in.send(fsm.AdvertTimerExpired{Gen: a.Gen})
		})
	case fsm.StartPreemptDelayTimer:
		in.preempt = in.armTimer(in.preempt, a.Duration, func() {
			in.send(fsm.PreemptDelayExpired{Gen: a.Gen})
		})
	case fsm.StopPreemptDelayTimer:
		stopTimer(in.preempt)
		in.preempt = nil
	case fsm.StopTimers:
		in.cancelTimers()
	case fsm.EmitStateChange:
		in.deps.emitState(in.spec, a.From, a.To, a.Reason)
	default:
		// The action set is closed (fsm.Action is a sealed interface), so this
		// is unreachable unless spec-vrrp-2 grows an action without teaching the
		// executor about it. Say so rather than dropping it silently.
		logger().Error("vrrp: unhandled FSM action, feature will not work",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID)
	}
}

// doSendAdvert re-encodes from the action's fields and sends. The advert is
// never cached across sends: a priority or interval change must reach the wire
// on the very next advertisement (holo bug 8, spec R-5).
func (in *instance) doSendAdvert(priority uint8, intervalMs int) {
	params := transport.AdvertParams{
		Version:         in.spec.Version,
		Priority:        priority,
		AdverIntervalMS: uint32(intervalMs),
		VIPs:            in.spec.VIPs,
	}
	if err := in.deps.updateAdvert(in.key, params); err != nil {
		logger().Warn("vrrp: prepare advertisement failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID, "family", in.spec.Family, "error", err)
		return
	}
	if err := in.deps.sendAdvert(in.key, priority, intervalMs); err != nil {
		logger().Warn("vrrp: send advertisement failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID, "family", in.spec.Family, "error", err)
	}
}

// doInstallVIPs registers the virtual addresses on the macvlan through the
// iface address-owner registry, which reconciles them onto the kernel device,
// and publishes the acceptance decision RFC 9568 Section 6.4.3 attaches to them:
// this router accepts packets addressed to a virtual address only if it is the
// address owner or Accept_Mode is True, and MUST NOT accept them otherwise.
// EffectiveAcceptMode (groups.go) is exactly that condition, with the Section
// 6.1 ownership exemption already folded in.
//
// The filter goes in FIRST, so the kernel never holds the address ahead of the
// rule that governs what it accepts.
//
// A filter that fails to apply is an operator-visible error and does NOT stop
// the addresses being installed. The same section requires this router to answer
// ARP and Neighbor Solicitations for those addresses, and on Linux both follow
// from the address being present, so withholding it would answer one MUST NOT by
// breaking two MUSTs and take the gateway down with it.
func (in *instance) doInstallVIPs(vips []netip.Addr) {
	accept := in.spec.EffectiveAcceptMode()
	if err := in.deps.setAcceptFilter(in.own, vips, accept); err != nil {
		logger().Error("vrrp: set accept-mode dataplane filter failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID,
			"device", in.dev, "accept", accept, "error", err)
	}
	if err := in.deps.installVIPs(in.dev, in.own, in.spec.vipCIDRs(vips)); err != nil {
		logger().Error("vrrp: install virtual addresses failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID, "device", in.dev, "error", err)
	}
}

// doRemoveVIPs deregisters this instance's owner.
//
// Unregister (not an empty re-registration) because it is the only call that
// marks the interface stale and so guarantees the kernel address is pruned
// (address_owner.go:126-129); an empty registration would leave the VIP live on
// a Backup, which is the split-brain the whole protocol exists to prevent.
//
// The acceptance filter is withdrawn after the addresses, the reverse of
// doInstallVIPs, so the rule is never given up ahead of the address it governs.
// It is withdrawn at all because a drop rule that outlives this instance would
// silence an address the next Active router is allowed to answer on.
func (in *instance) doRemoveVIPs() {
	in.deps.removeVIPs(in.own)
	if err := in.deps.clearAcceptFilter(in.own); err != nil {
		logger().Error("vrrp: withdraw accept-mode dataplane filter failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID,
			"device", in.dev, "error", err)
	}
}

// armTimer replaces any existing timer with a fresh one. The old timer is
// stopped first; a late callback from it is harmless because the FSM rejects an
// expiry whose Gen no longer matches.
func (in *instance) armTimer(existing clock.Timer, d time.Duration, fire func()) clock.Timer {
	stopTimer(existing)
	return in.clk.AfterFunc(d, fire)
}

// stopTimer cancels a timer if one is armed. Callers assign nil to the field
// themselves, which keeps "armed" representable as a non-nil field.
func stopTimer(t clock.Timer) {
	if t != nil {
		t.Stop()
	}
}

func (in *instance) cancelTimers() {
	stopTimer(in.masterDown)
	stopTimer(in.advert)
	stopTimer(in.preempt)
	in.masterDown, in.advert, in.preempt = nil, nil, nil
}

// fsmConfig projects the config spec onto the FSM's view.
func (in *instance) fsmConfig() fsm.Config {
	return fsm.Config{
		Version:          in.spec.Version,
		IsOwner:          in.spec.IsOwner,
		Priority:         in.spec.EffectivePriority(),
		Preempt:          in.spec.Preempt,
		PreemptDelayMs:   int(in.spec.PreemptDelaySeconds) * 1000,
		AdvertIntervalMs: int(in.spec.AdvertIntervalMs),
		LocalPrimaryIP:   in.primaryIP(),
		VIPs:             in.spec.VIPs,
		AcceptMode:       in.spec.EffectiveAcceptMode(),
	}
}

// primaryIP is the tie-break operand (RFC 9568 Section 6.4.3): the source
// address this router advertises from. For IPv6 that is the first virtual
// address, which the verifier guarantees is the link-local one; for IPv4 the
// transport resolves the parent's primary address at send time, and the FSM only
// needs a stable comparison operand, so the first VIP serves for both.
func (in *instance) primaryIP() netip.Addr {
	if len(in.spec.VIPs) > 0 {
		return in.spec.VIPs[0]
	}
	return netip.Addr{}
}

// reconfigure re-applies config to a running instance without restarting it.
//
// Applied synchronously under mu rather than queued as an event: the FSM must
// see the new config and the instance's spec change together, or an advert sent
// in between would carry a priority from one config and VIPs from the other.
func (in *instance) reconfigure(spec GroupSpec) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.spec = spec
	in.dispatchLocked(fsm.ConfigUpdated{Config: in.fsmConfig()})
}

// onPacket decodes one raw datagram addressed to this instance's family and
// feeds the FSM.
//
// The ENGINE owns Decode (not the transport) because Decode needs the group
// table for its VRID lookup, which only the engine has (spec-vrrp-5 D-B).
func (in *instance) onPacket(item transport.RxItem) {
	in.mu.Lock()
	defer in.mu.Unlock()

	adv, err := packet.Decode(item.Payload, item.Meta, in.lookup)
	if err != nil {
		in.deps.recordRxError(in.key, packet.Reason(err))
		return
	}
	// An accepted packet that matched only the RFC 9568 message-only checksum
	// (not the RFC 5798 pseudo-header form ze and the deployed base send) is
	// still counted, so operators can see a strict-RFC-9568 peer on the segment
	// (spec-vrrp-1 dual-accept).
	if adv.MsgOnlyChecksum {
		in.deps.recordRxError(in.key, packet.ReasonMsgOnlyChecksum)
	}
	// RFC 3768 Section 7.1: a VRRPv2 receiver MUST discard an advertisement
	// whose address list differs from its own. VRRPv3 dropped the requirement,
	// so this is v2-only. It lives here because only the engine holds both the
	// configured VIPs and the decoded ones.
	if in.spec.Version == versionV2 && !in.addressListMatches(adv) {
		in.deps.recordRxError(in.key, packet.ReasonAddressList)
		return
	}
	if adv.Priority == 0 {
		in.prio0Received++
	}
	in.send(fsm.AdvertReceived{
		Priority:   adv.Priority,
		SrcIP:      item.Meta.Src,
		IntervalMs: int(adv.AdverIntervalMS),
		VIPCount:   adv.VIPCount(),
	})
}

// addressListMatches compares an advertisement's address list with the
// configured one (RFC 3768 Section 7.1, v2 only).
func (in *instance) addressListMatches(adv packet.Advertisement) bool {
	if adv.VIPCount() != len(in.spec.VIPs) {
		return false
	}
	for i := range in.spec.VIPs {
		if adv.VIPAt(i) != in.spec.VIPs[i] {
			return false
		}
	}
	return true
}

// lookup answers packet.Decode's VRID probe for this instance.
func (in *instance) lookup(vrid uint8) (packet.Local, bool) {
	if vrid != in.spec.VRID {
		return packet.Local{}, false
	}
	return packet.Local{Version: in.spec.Version, AdverIntervalMS: in.spec.AdvertIntervalMs}, true
}

// snapshot is the show-command view of this instance. Called from the show
// handler's goroutine, so it takes mu like every other reader of spec/machine.
func (in *instance) snapshot() instanceView {
	in.mu.Lock()
	defer in.mu.Unlock()

	s := in.machine.Snapshot()
	return instanceView{
		Interface:            in.spec.Interface,
		Unit:                 in.spec.Unit,
		Family:               in.spec.Family,
		Group:                in.spec.Name,
		VRID:                 in.spec.VRID,
		Device:               in.dev,
		State:                viewState(s.State),
		Version:              s.Version,
		Priority:             in.spec.Priority,
		EffectivePriority:    s.Priority,
		IsOwner:              s.IsOwner,
		Preempt:              s.Preempt,
		AcceptMode:           s.AcceptMode,
		ConfiguredIntervalMs: s.ConfiguredIntervalMs,
		ActiveIntervalMs:     s.ActiveIntervalMs,
		VIPs:                 addrStrings(in.spec.VIPs),
		LastAdvertSource:     addrString(s.LastAdvertSrc),
		Since:                s.Since,
	}
}

// statistics is the counters view: the transport's wire counters merged with
// the Priority-0 counts this engine owns (the transport never parses payloads,
// so it cannot see a Priority-0 advert) and the FSM's derived timers.
func (in *instance) statistics(snapshot func(transport.InstanceKey) (transport.CounterSnapshot, bool)) statisticsView {
	in.mu.Lock()
	defer in.mu.Unlock()

	s := in.machine.Snapshot()
	v := statisticsView{
		Interface:            in.spec.Interface,
		Unit:                 in.spec.Unit,
		Family:               in.spec.Family,
		Group:                in.spec.Name,
		VRID:                 in.spec.VRID,
		State:                viewState(s.State),
		PriorityZeroSent:     in.prio0Sent,
		PriorityZeroReceived: in.prio0Received,
		// Microseconds, not milliseconds: a valid v3 skew is sub-millisecond
		// (78.125us at priority 254 / 10ms), so ms would render it as 0 (D-G).
		SkewTimeMicroseconds:   s.SkewTime.Microseconds(),
		MasterDownMicroseconds: s.MasterDownInterval.Microseconds(),
	}
	if snapshot != nil {
		if c, ok := snapshot(in.key); ok {
			v.AdvertsSent = c.AdvertsSent
			v.AdvertsReceived = c.AdvertsReceived
			v.AnnouncementsGARP = c.AnnouncementsGARP
			v.AnnouncementsNA = c.AnnouncementsNA
			v.PacketErrors = c.PacketErrors
		}
	}
	return v
}

// clearCounters resets this instance's counters. State is deliberately
// untouched: `clear ... statistics` must never perturb the protocol.
func (in *instance) clearCounters(reset func(transport.InstanceKey)) {
	in.mu.Lock()
	defer in.mu.Unlock()

	in.prio0Sent, in.prio0Received = 0, 0
	if reset != nil {
		reset(in.key)
	}
}

func addrStrings(addrs []netip.Addr) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
