// RFC: rfc/short/rfc9568.md -- Section 7.2 (transmit identity), Constants (dst/proto)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- VRRP raw-socket transport orchestrator
//
// The transport is the byte pipe between the raw sockets and the VRRP engine
// (spec-vrrp-5). It owns a per-instance socket set (rx on the parent, tx + announce
// on the macvlan), a bounded delivery channel, transport-owned metrics, and the
// two LAN announcers. It NEVER inspects VRRP payload bytes: spec-vrrp-1 encodes /
// validates, spec-vrrp-2 times, this package moves bytes and builds the L2/ND
// announcement frames. The platform raw-socket details live behind the Backend
// interface (backend_linux.go / backend_other.go).
//
// VRRP transmit identity is split across two devices: the L2 identity (virtual
// MAC) comes from binding the tx socket to the macvlan; the L3 identity (parent
// primary IPv4 / macvlan link-local) is pinned per send because no kernel default
// supplies both at once.

package transport

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

const (
	// rxChannelDepth bounds the shared engine delivery channel. Far above any
	// legitimate VRRP rate, so overflow only happens under attack, where GTSM TTL
	// validation discards the flood downstream anyway (R-4).
	rxChannelDepth = 256
	// txBufLen sizes the reusable per-instance tx buffer. It must hold the largest
	// advert (MaxLenV3v6 = 264) plus a 60-byte max IPv4 header; 512 gives headroom
	// (boundary table; derived from packet.MaxLen* constants, Finding 17).
	txBufLen = 512
	// ipv4HeaderLen is the fixed IPv4 header (no options) prepended for IP_HDRINCL.
	ipv4HeaderLen = 20
)

// Announcement kinds (metric label + snapshot selector).
const (
	kindGARP = "garp"
	kindNA   = "na"
)

// Transport-owned ze_vrrp_packet_errors_total reason labels. Codec-validation
// reasons come from packet.Reason() via RecordRxError.
const (
	reasonRxOverflow    = "rx-overflow"
	reasonMalformedIPv4 = "malformed-ipv4"
	reasonSendError     = "send-error"
	reasonGARPSendError = "garp-send-error"
	reasonNASendError   = "na-send-error"
	reasonNoLinkLocal   = "no-link-local"
)

var (
	// ErrNoLinkLocal is returned by a v6 InstanceHandle.SendAdvert while the macvlan
	// has no link-local source yet (the DAD window right after device creation).
	// The transport counts it {reason=no-link-local} and returns no upward error;
	// the next FSM timer tick retries (R-2).
	ErrNoLinkLocal = errors.New("vrrp/transport: macvlan has no link-local source yet")
	// ErrInstanceNotOpen is returned for an operation on an unknown instance key.
	ErrInstanceNotOpen = errors.New("vrrp/transport: instance not open")
	// errNoParentV4 signals the parent unit has no configured IPv4 address.
	errNoParentV4 = errors.New("vrrp/transport: parent interface has no IPv4 address")
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() { loggerPtr.Store(slogutil.DiscardLogger()) }

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger configures the transport logger. Called by the VRRP plugin
// (spec-vrrp-5) at startup. Defaults to a discard logger so unit tests are quiet.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// resolveIfaceAddresses is the overridable seam for parent address resolution
// (ospf backend_linux.go:31 pattern), so the source-selection path is testable
// without a live resolver.
var resolveIfaceAddresses = iface.Addresses

// InstanceSpec identifies one VRRP instance's transport needs. The macvlan device
// and virtual MAC are produced by spec-vrrp-3 and passed BY NAME; the transport
// never creates devices.
type InstanceSpec struct {
	Family        uint8   // packet.V4 or packet.V6 (independent VRID namespaces)
	VRID          uint8   // 1..255
	Parent        string  // parent logical interface (unit) name: rx bind + v4 source
	MacvlanDevice string  // macvlan OS device name: tx bind (virtual MAC egress)
	VirtualMAC    [6]byte // 00:00:5e:00:0{1|2}:{vrid}
}

// key derives the InstanceKey (also the metric label set) from the spec.
func (s InstanceSpec) key() InstanceKey {
	return InstanceKey{Interface: s.Parent, VRID: s.VRID, Family: s.Family}
}

// InstanceKey identifies an open instance for every post-open call. Its fields are
// exactly the interface/vrid/family metric labels.
type InstanceKey struct {
	Interface string
	VRID      uint8
	Family    uint8
}

// AdvertParams are the FSM-driven advertisement parameters (spec-vrrp-2). VRID and
// Family come from the InstanceSpec, not here.
type AdvertParams struct {
	Version         uint8
	Priority        uint8
	AdverIntervalMS uint32
	VIPs            []netip.Addr
}

// CounterSnapshot is a per-instance read-back for spec-vrrp-5 `show vrrp
// statistics` / `clear vrrp statistics` (Finding 7). Prio-0 sent/received counters
// are ENGINE-owned (D-F): the transport never parses payloads.
type CounterSnapshot struct {
	AdvertsSent       uint64
	AdvertsReceived   uint64
	AnnouncementsGARP uint64
	AnnouncementsNA   uint64
	PacketErrors      map[string]uint64
}

// RxItem is one received VRRP datagram delivered to the engine: the IP-header
// facts (packet.RxMeta, consumed by packet.Decode), the copied VRRP payload, and
// the instance whose socket received it.
//
// Key is essential, not decorative: every instance opens its OWN rx socket bound
// to the parent and joined to the VRRP group, so one advertisement on the wire
// is delivered once PER INSTANCE on that parent. Without the key the engine
// could not tell the copies apart and would feed each instance every copy,
// inflating the receive counters and re-running the state machine on duplicates.
type RxItem struct {
	Key     InstanceKey
	Meta    packet.RxMeta
	Payload []byte
}

// rxSink is the readLoop's delivery target: it forwards a received datagram to the
// engine channel without blocking (drop-on-overflow, AC-7) and counts deliveries
// and overflow drops. A blocked readLoop would stop draining the kernel buffer for
// all traffic on that socket, so a counted drop is strictly safer (R-4).
type rxSink struct {
	key      InstanceKey
	ch       chan<- RxItem
	counters *instanceCounters
}

// deliver performs the non-blocking send with drop-on-overflow. It never blocks
// the readLoop. Returns true when the item was delivered.
func (s rxSink) deliver(item RxItem) bool {
	item.Key = s.key
	select {
	case s.ch <- item:
		s.counters.advertReceived()
		return true
	default:
		s.counters.packetError(reasonRxOverflow)
		return false
	}
}

// InstanceHandle is one open per-instance socket set. Implementations are
// platform-specific (backend_linux.go); tests substitute a fake. The handle owns
// the rx readLoop goroutine (started by the backend, stopped by Close) and
// delivers received datagrams through the rxSink passed to Backend.OpenInstance.
type InstanceHandle interface {
	// SendAdvert transmits a prepared advertisement. For IPv4 frame is a full
	// IP_HDRINCL datagram (IPv4 header + VRRP message); for IPv6 frame is the VRRP
	// message and the handle pins the source via an IPV6_PKTINFO cmsg. Returns
	// ErrNoLinkLocal (v6) when the macvlan has no link-local source yet.
	SendAdvert(frame []byte) error
	// SendAnnounce transmits a prepared announcement frame on the family-appropriate
	// announce socket (AF_PACKET GARP for v4, raw ICMPv6 NA for v6).
	SendAnnounce(frame []byte) error
	// Close releases all sockets and stops the readLoop goroutine.
	Close() error
}

// Backend opens per-instance socket sets. The Linux implementation opens the raw
// proto-112 rx/tx sockets plus the family-appropriate announce socket; the
// non-Linux stub returns an unsupported-platform error.
type Backend interface {
	OpenInstance(spec InstanceSpec, sink rxSink) (InstanceHandle, error)
}

// instance is the transport's per-instance bookkeeping around an InstanceHandle.
type instance struct {
	spec     InstanceSpec
	handle   InstanceHandle
	counters *instanceCounters
	ann      *announcer

	mu        sync.Mutex // guards txBuf/txLen/v4Src/lastParams
	txBuf     []byte
	txLen     int
	v4Src     netip.Addr
	lastParam AdvertParams
	hasParam  bool
}

// Transport is the VRRP raw-socket transport orchestrator.
type Transport struct {
	backend    Backend
	metricsPtr atomic.Pointer[transportMetrics]

	mu        sync.Mutex
	instances map[InstanceKey]*instance
	deliver   chan RxItem
}

// New constructs a Transport over backend. Production passes NewBackend(); tests
// pass a fake.
func New(backend Backend) *Transport {
	t := &Transport{
		backend:   backend,
		instances: make(map[InstanceKey]*instance),
		deliver:   make(chan RxItem, rxChannelDepth),
	}
	t.metricsPtr.Store(nopTransportMetrics())
	return t
}

// SetMetrics wires the Prometheus registry (spec-vrrp-5 ConfigureMetrics). The
// per-instance counters read the registry through an atomic pointer, so a swap
// after OpenInstance is picked up without a stale capture.
func (t *Transport) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	t.metricsPtr.Store(newTransportMetrics(reg))
	t.mu.Lock()
	t.updateSocketsGaugeLocked()
	t.mu.Unlock()
}

func (t *Transport) updateSocketsGaugeLocked() {
	t.metricsPtr.Load().socketsOpen.Set(float64(len(t.instances)))
}

// Receive returns the bounded channel of received datagrams delivered to the
// engine. Each item is (RxMeta, payload) after IPv4-header stripping (v4) or cmsg
// extraction (v6); the engine calls packet.Decode.
func (t *Transport) Receive() <-chan RxItem { return t.deliver }

func (t *Transport) lookup(key InstanceKey) *instance {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.instances[key]
}

// OpenInstance opens the socket set for a configured group, starts its readLoop
// and announcer, and registers it. Called by the engine (spec-vrrp-5 OnStarted).
func (t *Transport) OpenInstance(spec InstanceSpec) (InstanceKey, error) {
	key := spec.key()
	counters := newInstanceCounters(&t.metricsPtr, spec.Parent, spec.VRID, spec.Family)
	sink := rxSink{key: key, ch: t.deliver, counters: counters}

	handle, err := t.backend.OpenInstance(spec, sink)
	if err != nil {
		return InstanceKey{}, err
	}

	inst := &instance{
		spec:     spec,
		handle:   handle,
		counters: counters,
		txBuf:    make([]byte, txBufLen),
	}
	inst.ann = newAnnouncer(inst.frameBuilder(), handle.SendAnnounce, inst.reportAnnounce)

	// Resolve the IPv4 source now (re-resolved on UpdateAdvert / address events).
	if spec.Family == packet.V4 {
		if src, rerr := resolveParentPrimaryV4(spec.Parent); rerr == nil {
			inst.v4Src = src
		} else {
			logger().Warn("vrrp/transport: parent IPv4 source unresolved", "interface", spec.Parent, "err", rerr)
		}
	}

	inst.ann.start()
	t.mu.Lock()
	t.instances[key] = inst
	t.updateSocketsGaugeLocked()
	t.mu.Unlock()
	return key, nil
}

// CloseInstance closes one instance's sockets and stops its goroutines.
func (t *Transport) CloseInstance(key InstanceKey) error {
	t.mu.Lock()
	inst := t.instances[key]
	if inst == nil {
		t.mu.Unlock()
		return nil
	}
	delete(t.instances, key)
	t.updateSocketsGaugeLocked()
	t.mu.Unlock()
	return inst.shutdown()
}

// Close closes every instance and stops all goroutines (component shutdown).
func (t *Transport) Close() {
	t.mu.Lock()
	insts := make([]*instance, 0, len(t.instances))
	for k, inst := range t.instances {
		insts = append(insts, inst)
		delete(t.instances, k)
	}
	t.updateSocketsGaugeLocked()
	t.mu.Unlock()

	for _, inst := range insts {
		if err := inst.shutdown(); err != nil {
			logger().Warn("vrrp/transport: close instance", "interface", inst.spec.Parent, "vrid", inst.spec.VRID, "err", err)
		}
	}
}

// shutdown stops the announcer and closes the socket set (which stops the
// readLoop). Announcer first so no in-flight burst races the socket close.
func (inst *instance) shutdown() error {
	inst.ann.close()
	return inst.handle.Close()
}

// UpdateAdvert re-encodes the advertisement into the per-instance tx buffer so a
// later SendAdvert never uses stale parameters (holo bug 8). The IPv4 source is
// re-resolved here (staleness point without per-send cost).
func (t *Transport) UpdateAdvert(key InstanceKey, params AdvertParams) error {
	inst := t.lookup(key)
	if inst == nil {
		return ErrInstanceNotOpen
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.spec.Family == packet.V4 {
		inst.resolveV4SrcLocked()
	}
	return inst.encodeLocked(params)
}

// SendAdvert transmits the prepared advertisement (Adver_Timer fire / prio-0
// shutdown, spec-vrrp-2). A v6 send with no link-local is skipped and counted; any
// other send error is counted and returned.
func (t *Transport) SendAdvert(key InstanceKey) error {
	inst := t.lookup(key)
	if inst == nil {
		return ErrInstanceNotOpen
	}
	inst.mu.Lock()
	if inst.txLen == 0 {
		inst.mu.Unlock()
		return nil
	}
	// Hold the lock across the send so UpdateAdvert cannot re-encode the shared
	// buffer mid-flight; Sendto/Sendmsg copy before returning.
	err := inst.handle.SendAdvert(inst.txBuf[:inst.txLen])
	inst.mu.Unlock()

	switch {
	case err == nil:
		inst.counters.advertSent()
		return nil
	case errors.Is(err, ErrNoLinkLocal):
		inst.counters.packetError(reasonNoLinkLocal)
		return nil
	default:
		inst.counters.packetError(reasonSendError)
		return err
	}
}

// v6SourceWarmer is implemented by the Linux handle so the orchestrator can
// resolve and cache the macvlan link-local ON THE ENGINE'S GOROUTINE before the
// announcer worker (which only does socket I/O) uses it. Resolution is a netlink
// query and netlink sockets land in the calling THREAD's network namespace, so
// doing it at the entry point also keeps netns-scoped tests (and any future
// multi-netns embedding) correct.
// The method is unexported because the only implementation is in this package.
// It was WarmV6Source() until 3c9644e15 unexported the implementing method and
// left this declaration alone: nothing satisfied the interface after that, the
// comma-ok assertion below simply stopped matching, and the warm silently never
// ran. The compile-time check under it is what makes that unfixable in silence
// again -- a type assertion cannot report a break, so the break has to be a
// build error instead.
// The compile-time check that the Linux handle still satisfies this lives in
// backend_linux.go, beside the only implementation: linuxInstance exists only
// under the linux build tag and this file carries none.
type v6SourceWarmer interface{ warmV6Source() }

// AnnounceMaster enqueues a GARP/NA burst for the given VIPs on a Master
// transition (spec-vrrp-2). Announcers fire ONLY on this explicit engine call,
// never on rx input, so a forged advert cannot make this node emit GARP/NA.
// For a v6 instance the NA source (macvlan link-local) is resolved here, on the
// caller's goroutine; the worker only transmits.
func (t *Transport) AnnounceMaster(key InstanceKey, vips []netip.Addr) {
	inst := t.lookup(key)
	if inst == nil {
		return
	}
	if w, ok := inst.handle.(v6SourceWarmer); ok && inst.spec.Family == packet.V6 {
		w.warmV6Source()
	}
	if !inst.ann.enqueue(vips) {
		logger().Warn("vrrp/transport: announce queue full, dropping burst", "interface", key.Interface, "vrid", key.VRID)
	}
}

// RefreshParentAddresses re-resolves the IPv4 source after a parent address-change
// event and re-encodes the last advert so the next SendAdvert carries the new
// source. No-op for a v6 instance (its source is the macvlan link-local, pinned at
// send time by the handle).
func (t *Transport) RefreshParentAddresses(key InstanceKey) {
	inst := t.lookup(key)
	if inst == nil || inst.spec.Family != packet.V4 {
		return
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	inst.resolveV4SrcLocked()
	if inst.hasParam {
		if err := inst.encodeLocked(inst.lastParam); err != nil {
			logger().Warn("vrrp/transport: re-encode after address change", "interface", key.Interface, "err", err)
		}
	}
}

// RecordRxError counts a codec-validation reason the engine (spec-vrrp-5)
// discovered via packet.Reason(), keeping ze_vrrp_packet_errors_total single-owner
// (holo bug 9). The transport itself counts its transport-level reasons directly.
func (t *Transport) RecordRxError(key InstanceKey, reason string) {
	inst := t.lookup(key)
	if inst == nil || reason == "" {
		return
	}
	inst.counters.packetError(reason)
}

// CounterSnapshot returns a per-instance read-back for show/clear vrrp statistics.
func (t *Transport) CounterSnapshot(key InstanceKey) (CounterSnapshot, bool) {
	inst := t.lookup(key)
	if inst == nil {
		return CounterSnapshot{}, false
	}
	return inst.counters.snapshot(), true
}

// ResetCounters zeroes one instance's snapshot counters (clear vrrp statistics).
// The Prometheus series are monotonic and left untouched.
func (t *Transport) ResetCounters(key InstanceKey) {
	inst := t.lookup(key)
	if inst == nil {
		return
	}
	inst.counters.reset()
}

// resolveV4SrcLocked re-resolves the parent primary IPv4; on error the prior
// source is kept so a transient resolver failure does not blank the source.
func (inst *instance) resolveV4SrcLocked() {
	if src, err := resolveParentPrimaryV4(inst.spec.Parent); err == nil {
		inst.v4Src = src
	}
}

// encodeLocked builds the tx buffer from params. For IPv4 it prepends the
// IP_HDRINCL header and software-fills the VRRP checksum (message-only); for IPv6
// it writes the VRRP message only and leaves the checksum zero for the kernel to
// fill via IPV6_CHECKSUM. inst.mu must be held.
func (inst *instance) encodeLocked(p AdvertParams) error {
	adv := packet.Advertisement{
		Version:         p.Version,
		Family:          inst.spec.Family,
		VRID:            inst.spec.VRID,
		Priority:        p.Priority,
		AdverIntervalMS: p.AdverIntervalMS,
		VIPs:            p.VIPs,
	}
	if err := adv.Validate(); err != nil {
		return err
	}
	if inst.spec.Family == packet.V4 {
		var src4 [4]byte
		if inst.v4Src.Is4() {
			src4 = inst.v4Src.As4()
		}
		hdr := buildIPv4Header(inst.txBuf, src4, packet.MulticastV4.As4())
		n := adv.WriteTo(inst.txBuf, hdr)
		// RFC 9568 Section 5.2.8: v3/IPv4 checksum is message-only (no pseudo-header);
		// a v4 src makes FillChecksum select the message-only path.
		packet.FillChecksum(inst.txBuf, hdr, n, inst.v4Src, packet.MulticastV4)
		inst.txLen = hdr + n
	} else {
		n := adv.WriteTo(inst.txBuf, 0)
		inst.txLen = n
	}
	inst.lastParam = p
	inst.hasParam = true
	return nil
}

// frameBuilder returns the family-appropriate announcement-frame builder capturing
// the virtual MAC. A VIP of the wrong family produces no frame (defensive filter).
func (inst *instance) frameBuilder() frameBuilder {
	vmac := inst.spec.VirtualMAC
	if inst.spec.Family == packet.V6 {
		return func(vip netip.Addr, buf []byte) (int, bool) {
			if !vip.Is6() {
				return 0, false
			}
			return BuildNA(buf, vmac, vip.As16()), true
		}
	}
	return func(vip netip.Addr, buf []byte) (int, bool) {
		if !vip.Is4() {
			return 0, false
		}
		return buildGARP(buf, vmac, vip.As4()), true
	}
}

// reportAnnounce counts an announcement send result: success bumps
// announcements_sent{kind}; failure bumps packet_errors{reason=*-send-error},
// except the macvlan-has-no-link-local skip which counts {reason=no-link-local}
// (AC-10 semantics; the next Master transition re-announces).
func (inst *instance) reportAnnounce(err error) {
	kind, sendErr := kindGARP, reasonGARPSendError
	if inst.spec.Family == packet.V6 {
		kind, sendErr = kindNA, reasonNASendError
	}
	if err != nil {
		if errors.Is(err, ErrNoLinkLocal) {
			sendErr = reasonNoLinkLocal
		}
		inst.counters.packetError(sendErr)
		logger().Debug("vrrp/transport: announce send failed", "interface", inst.spec.Parent, "vrid", inst.spec.VRID, "err", err)
		return
	}
	inst.counters.announcement(kind)
}

// buildIPv4Header writes the 20-byte IPv4 header prepended under IP_HDRINCL and
// returns ipv4HeaderLen. Total length, identification, and header checksum are
// left zero for the kernel to fill (raw(7), A-3).
//
//	RFC 9568 Section 5.1.1.3 / RFC 3768 Section 5.2.3: the TTL MUST be 255.
//	RFC 9568 Constants / RFC 3768 Section 5.2.2: the protocol number is 112.
func buildIPv4Header(buf []byte, src, dst [4]byte) int {
	buf[0] = 0x45 // IP version 4, IHL 5 (no options)
	// TOS 0xc0 = DSCP CS6 (Internetwork Control), matching keepalived/holo so
	// adverts are prioritized like other control-plane traffic (operational
	// precedent; RFC 9568 does not mandate a TOS).
	buf[1] = 0xc0
	buf[2], buf[3] = 0, 0 // total length: kernel fills under IP_HDRINCL (A-3)
	buf[4], buf[5] = 0, 0 // identification: kernel fills (A-3)
	buf[6], buf[7] = 0, 0 // flags + fragment offset: not fragmented
	buf[8] = 255          // RFC 9568 Section 5.1.1.3: TTL MUST be 255
	buf[9] = packet.ProtoNumber
	buf[10], buf[11] = 0, 0 // header checksum: kernel fills under IP_HDRINCL (A-3)
	copy(buf[12:16], src[:])
	copy(buf[16:20], dst[:])
	return ipv4HeaderLen
}

// resolveParentPrimaryV4 returns the parent unit's first configured IPv4 address
// (its "primary", ospf interfaceIPv4 precedent). RFC 9568 Section 7.2 / RFC 3768
// Section 7.2: the advert source IP is the sending interface's primary IPv4.
func resolveParentPrimaryV4(name string) (netip.Addr, error) {
	addrs, err := resolveIfaceAddresses(name)
	if err != nil {
		return netip.Addr{}, err
	}
	for _, a := range addrs {
		if a.Family != "" && a.Family != "ipv4" {
			continue
		}
		ip, perr := netip.ParseAddr(a.Address)
		if perr == nil && ip.Is4() {
			return ip, nil
		}
	}
	return netip.Addr{}, errNoParentV4
}
