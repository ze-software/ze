// Design: docs/features/interfaces.md -- Netlink interface monitor
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errIfaceMonitorAlreadyStarted = errors.New("iface monitor: already started")
	errIfaceNetlinkEventBusIsNil  = errors.New("iface-netlink: event bus is nil")
)

// monitor watches OS interface changes via netlink multicast and emits
// namespaced events on the EventBus. It is a long-lived goroutine, not
// per-event.
//
// start MUST be called exactly once. stop MUST be called after a successful
// start to release resources. stop is safe to call multiple times.
type monitor struct {
	eventBus ze.EventBus
	done     chan struct{}
	stopCh   chan struct{}
	stopFn   sync.Once
	started  atomic.Bool
	ready    chan struct{}
	replay   chan struct{} // Depth one: simultaneous replay requests coalesce.
	// addresses is owned by run and bounded by the addresses currently held
	// on observed interfaces. Removals and link deletion reclaim entries.
	addresses    map[addressKey]addrEventPayload
	known        sync.Map // map[int]struct{} — known link indices
	linkNames    sync.Map // map[int]string — index to name, updated by link events
	knownRouters sync.Map // map[neighKey]struct{} — neighbors seen with NTF_ROUTER
	// linkByIndex asks the kernel for a link the name cache does not hold yet
	// (handleAddrUpdate, resolveAddrLink). It is netlink.LinkByIndex outside
	// tests.
	linkByIndex func(index int) (netlink.Link, error)
	// ndSock is the RTNLGRP_ND_USEROPT socket; ndDone closes when its reader
	// ends (nduseropt_linux.go).
	ndSock *nl.NetlinkSocket
	ndDone chan struct{}
}

// neighKey identifies a neighbor entry, for NTF_ROUTER tracking here and for
// the neighbor watch (neighbor_watch_linux.go).
type neighKey struct {
	linkIndex int
	ip        string
}

type addressKey struct {
	index  int
	prefix netip.Prefix
}

func newMonitor(eb ze.EventBus) *monitor {
	return &monitor{
		eventBus:    eb,
		done:        make(chan struct{}),
		stopCh:      make(chan struct{}),
		ready:       make(chan struct{}),
		replay:      make(chan struct{}, 1),
		addresses:   make(map[addressKey]addrEventPayload),
		linkByIndex: netlink.LinkByIndex,
	}
}

// monitorReceiveBufferBytes is SO_RCVBUF for each of the three netlink sockets
// this monitor subscribes on. Without it they keep the kernel default, which is
// net.core.rmem_default, commonly 208 KiB and not sized for anything in
// particular.
//
// The kernel drops a multicast notification it cannot queue, and the loss
// happens before ze sees it: no amount of care on the reader side recovers a
// notification that was never delivered. A link that flaps rapidly, a bulk
// address change, or a neighbor table churning during a scan all produce
// bursts far faster than a Go channel hand-off, and a router that silently
// misses link-state transitions under exactly those conditions is a real
// operational defect rather than a test artifact.
//
// Measured on the arm64 QEMU VM on 2026-09-04: with the default buffer,
// test/plugin/iface-link-flap-during-commit lost 1054 and 207 notifications in
// two of three runs at 101 transitions per burst, which is well under the 401
// its own header records as where loss had previously been measured.
//
// One MiB, and not forced. SetReceiveBufferSize without the force flag asks for
// the size and the kernel clamps it to net.core.rmem_max, so a host with a
// smaller ceiling gets what it allows rather than an error. The forcing variant
// takes SO_RCVBUFFORCE, needs CAP_NET_ADMIN and overrides the sysctl, which is
// not a decision a monitor should make for the operator.
const monitorReceiveBufferBytes = 1 << 20

func (m *monitor) start() error {
	if !m.started.CompareAndSwap(false, true) {
		return errIfaceMonitorAlreadyStarted
	}

	linkCh := make(chan netlink.LinkUpdate, 64)
	addrCh := make(chan netlink.AddrUpdate, 64)
	neighCh := make(chan netlink.NeighUpdate, 64)

	if err := netlink.LinkSubscribeWithOptions(linkCh, m.stopCh, netlink.LinkSubscribeOptions{
		ReceiveBufferSize: monitorReceiveBufferBytes,
	}); err != nil {
		m.started.Store(false)
		return fmt.Errorf("iface monitor: link subscribe: %w", err)
	}
	// ListExisting dumps every address the kernel already holds through the
	// same subscription, as addr-added events, before the changes that follow.
	// The subscription alone delivers only CHANGES, and the monitor starts
	// after boot-time applyConfig has assigned the operator's addresses, so
	// without the dump no consumer heard of any address that predated it: the
	// connected plugin published no prefix for them, sysrib's resolver could
	// prove no gateway inside them, and a gateway member of a multipath group
	// beside a device-only member was dropped from the group. Zebra reads the
	// same table at startup for the same reason. The dump is requested on the
	// subscribed socket, so an address changed while it runs is never missed;
	// it may be announced twice, which consumers of addr-added already meet
	// when the kernel re-announces an address. The dumped updates wait in
	// addrCh until run starts, after seedLinkNames, so each one resolves its
	// interface name.
	if err := netlink.AddrSubscribeWithOptions(addrCh, m.stopCh, netlink.AddrSubscribeOptions{
		ListExisting:      true,
		ReceiveBufferSize: monitorReceiveBufferBytes,
	}); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: addr subscribe: %w", err)
	}
	if err := netlink.NeighSubscribeWithOptions(neighCh, m.stopCh, netlink.NeighSubscribeOptions{
		ReceiveBufferSize: monitorReceiveBufferBytes,
	}); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: neigh subscribe: %w", err)
	}
	if err := m.startNDUserOpts(); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: nd user option subscribe: %w", err)
	}

	m.seedLinkNames()

	go m.run(linkCh, addrCh, neighCh)
	// StartMonitor MUST return only after the request subscription is live.
	<-m.ready
	return nil
}

// seedLinkNames fills the index->name cache with the links that ALREADY exist.
//
// LinkSubscribe delivers only CHANGES, so without this the cache starts empty
// and stays empty for every interface that predates the monitor. handleAddrUpdate
// resolves an address event's interface through that cache. Before a miss asked
// the kernel (resolveAddrLink), it returned early, so every address event on
// such an interface was silently dropped; the seed still saves the dumped
// addresses one lookup each.
//
// That is not a cosmetic gap. The interface component starts its monitor AFTER
// boot-time applyConfig has already created the configured interfaces
// (internal/component/iface/register.go:430 logs "interface config applied", the
// monitor starts after it), so the dropped set was exactly the operator's own
// interfaces. The config-transaction settlement waiter for an address add blocks
// on that `interface/addr-added` event (internal/component/iface/operation.go:57-65,
// 5s timeout), so a SIGHUP reload changing an interface address always timed out
// and rolled back: the address could not be changed by reload at all. Reproduced
// in QEMU before this fix as
//
//	config apply partial failure: operation interface-add-address-zdiag0-10.77.0.2_24
//	settlement timeout waiting for interface/addr-added 10.77.0.2 after 5s
//
// Seeded AFTER the subscriptions are live so a link appearing between the two is
// still delivered by the subscription; a duplicate Store is harmless.
//
// `m.known` remains empty so the first update still announces each interface.
// handleLinkUpdate also emits that update's carrier state.
func (m *monitor) seedLinkNames() {
	links, err := listLinks()
	if err != nil {
		logger().Warn("iface monitor: seed link cache failed; address events for pre-existing interfaces will be dropped", "error", err)
		return
	}
	for _, l := range links {
		attrs := l.Attrs()
		if attrs == nil {
			continue
		}
		m.linkNames.Store(attrs.Index, attrs.Name)
	}
}

func (m *monitor) stop() {
	if !m.started.Load() {
		return
	}
	m.stopFn.Do(func() { close(m.stopCh) })
	if m.ndSock != nil {
		m.ndSock.Close()
		<-m.ndDone
	}
	<-m.done
}

func (m *monitor) run(linkCh <-chan netlink.LinkUpdate, addrCh <-chan netlink.AddrUpdate, neighCh <-chan netlink.NeighUpdate) {
	defer close(m.done)
	unsubscribe := ifaceevents.AddressReplayRequest.Subscribe(m.eventBus, m.requestReplay)
	defer unsubscribe()
	close(m.ready)
	// This owner runs until stop or a producer closes. All address changes
	// and replay publications are serialized here, including link deletion.
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}
		select {
		case <-m.stopCh:
			return
		case <-m.replay:
			for _, payload := range m.addresses {
				m.emit(ifaceevents.EventAddrAdded, payload)
			}
		case lu, ok := <-linkCh:
			if !ok {
				return
			}
			m.safeHandleLinkUpdate(lu)
		case au, ok := <-addrCh:
			if !ok {
				return
			}
			m.safeHandleAddrUpdate(au)
		case nu, ok := <-neighCh:
			if !ok {
				return
			}
			m.safeHandleNeighUpdate(nu)
		}
	}
}

func (m *monitor) requestReplay() {
	select {
	case <-m.stopCh:
		return
	default:
	}
	select {
	case m.replay <- struct{}{}:
	default: // One pending replay already covers this request.
	}
}

func (m *monitor) safeHandleLinkUpdate(lu netlink.LinkUpdate) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("iface monitor: panic in link handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleLinkUpdate(lu)
}

func (m *monitor) safeHandleAddrUpdate(au netlink.AddrUpdate) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("iface monitor: panic in addr handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleAddrUpdate(au)
}

func (m *monitor) safeHandleNeighUpdate(nu netlink.NeighUpdate) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("iface monitor: panic in neigh handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleNeighUpdate(nu)
}

// linkEventPayload is the JSON payload emitted on (interface, created/down)
// events. The Unit field is always 0 for link events (these describe the
// physical/parent interface; per-unit events use addrEventPayload).
type linkEventPayload struct {
	Name  string `json:"name"`
	Unit  int    `json:"unit"`
	Type  string `json:"type"`
	Index int    `json:"index"`
	MTU   int    `json:"mtu"`
}

// stateEventPayload is the JSON payload emitted on (interface, up/down).
type stateEventPayload struct {
	Name  string `json:"name"`
	Unit  int    `json:"unit"`
	Index int    `json:"index"`
}

// addrEventPayload is the JSON payload emitted on (interface, addr-added/addr-removed).
// Matches the addr-handler shape in bgp/reactor/reactor_iface.go.
type addrEventPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
	// Origin classifies the address as static/slaac/temporary/dynamic from the
	// kernel IFA_F_* flags (AC-6), so a consumer observing the addr event stream
	// can tell a SLAAC/RA-assigned address from an operator-configured one.
	Origin string `json:"origin,omitempty"`
}

func (m *monitor) handleLinkUpdate(lu netlink.LinkUpdate) {
	attrs := lu.Attrs()
	if attrs == nil {
		return
	}
	name := attrs.Name
	idx := attrs.Index

	m.linkNames.Store(idx, name)

	switch lu.Header.Type {
	case unix.RTM_NEWLINK:
		parent, unit, _ := resolveVLANUnit(name)
		for key, payload := range m.addresses {
			if key.index != idx {
				continue
			}
			identityUnchanged := payload.Name == parent && payload.Unit == unit
			if identityUnchanged {
				continue
			}
			previous := payload
			payload.Name = parent
			payload.Unit = unit
			m.addresses[key] = payload
			// A rename changes the consumer's address identity. Withdraw the
			// old identity before announcing and retaining its replacement.
			m.emit(ifaceevents.EventAddrRemoved, previous)
			m.emit(ifaceevents.EventAddrAdded, payload)
		}
		if _, seen := m.known.LoadOrStore(idx, struct{}{}); !seen {
			m.emit(ifaceevents.EventCreated, linkEventPayload{
				Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
			})
		}
		// A link can predate this monitor. Its first observed update can be
		// the failure itself, so announcing it must not consume its state.
		if isLinkUp(attrs) {
			m.emit(ifaceevents.EventUp, stateEventPayload{Name: name, Index: idx})
		} else {
			m.emit(ifaceevents.EventDown, stateEventPayload{Name: name, Index: idx})
		}
	case unix.RTM_DELLINK:
		m.known.Delete(idx)
		m.linkNames.Delete(idx)
		for key, payload := range m.addresses {
			if key.index == idx {
				delete(m.addresses, key)
				m.emit(ifaceevents.EventAddrRemoved, payload)
			}
		}
		// Interface deletion maps to (interface, down): there is no
		// separate "deleted" event type in the stream registry. Down is
		// the closest semantic match (link is no longer operational).
		m.emit(ifaceevents.EventDown, linkEventPayload{
			Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
		})
	}
}

func (m *monitor) handleAddrUpdate(au netlink.AddrUpdate) {
	ip, ok := netip.AddrFromSlice(au.LinkAddress.IP)
	if !ok {
		return
	}
	ip = ip.Unmap()
	if ip.IsUnspecified() {
		return
	}
	if ip.IsMulticast() {
		return
	}
	ones, bits := au.LinkAddress.Mask.Size()
	if bits != ip.BitLen() {
		return
	}
	key := addressKey{index: au.LinkIndex, prefix: netip.PrefixFrom(ip, ones)}
	if !ip.Is4() {
		if au.Flags&(unix.IFA_F_TENTATIVE|unix.IFA_F_DADFAILED) != 0 {
			if payload, exists := m.addresses[key]; exists {
				delete(m.addresses, key)
				m.emit(ifaceevents.EventAddrRemoved, payload)
			}
			return
		}
	}

	ifaceName, ok := m.resolveAddrLink(au.LinkIndex, key.prefix)
	if !ok {
		return
	}
	parent, unit, _ := resolveVLANUnit(ifaceName)
	addr := ip.String()

	fam, ok := addrFamily(au.LinkAddress.String())
	if !ok {
		return
	}

	origin := addrOrigin(au.LinkAddress.IP.To4() == nil, au.Flags)
	// Observe kernel-autoconfigured (SLAAC/RA) address churn distinctly (AC-6).
	// This rides the monitor's existing coalesced event path (R-6), so a high-RA
	// network does not need a separate observer.
	if origin == originSlaac || origin == originTemporary {
		logger().Debug("iface monitor: kernel-autoconfigured (SLAAC) address",
			"interface", ifaceName, "address", addr, "origin", origin, "added", au.NewAddr,
			"valid-lft", normalizeLifetime(au.ValidLft), "preferred-lft", normalizeLifetime(au.PreferedLft))
	}

	eventType := addrUpdateToEventType(au.NewAddr)
	payload := addrEventPayload{
		Name:         parent,
		Unit:         unit,
		Index:        au.LinkIndex,
		Address:      addr,
		PrefixLength: ones,
		Family:       fam,
		Origin:       origin,
	}
	if au.NewAddr {
		m.addresses[key] = payload
	} else {
		delete(m.addresses, key)
	}
	m.emit(eventType, payload)
}

// resolveAddrLink names the interface an address event belongs to.
//
// Links and addresses arrive on two netlink sockets, each read by its own
// goroutine (LinkSubscribeWithOptions, AddrSubscribeWithOptions in start), and
// run selects between the two channels at random. The kernel announces a new
// link before its first address, but nothing keeps that order between the
// sockets: an `ip link add`, `ip link set up`, `ip addr add` sequence can put
// the address in front of run while the link's RTM_NEWLINK still waits on the
// other channel. The cache then misses for a link that exists, and dropping the
// address there lost it for good: it was never retained, so no replay restored
// it, and the kernel sends no second RTM_NEWADDR. The connected plugin then
// never offered the prefix (test/plugin/rib-import-central-bogons under load).
//
// On a miss the kernel is asked for the link, which is the authority the cache
// copies. The answer is stored, so the RTM_NEWLINK that follows still announces
// the link (m.known is untouched) and finds the address identity unchanged. A
// link the kernel no longer holds took its addresses with it: its RTM_DELLINK
// is still queued, and the address is not announced. Any other lookup failure
// loses the address, so reportAddressUnattributed logs and raises it.
func (m *monitor) resolveAddrLink(index int, prefix netip.Prefix) (string, bool) {
	if nameVal, cached := m.linkNames.Load(index); cached {
		name, isStr := nameVal.(string)
		return name, isStr
	}
	link, err := m.linkByIndex(index)
	if err != nil {
		if _, gone := errors.AsType[netlink.LinkNotFoundError](err); gone {
			logger().Debug("iface monitor: address event for a deleted link", "index", index)
			return "", false
		}
		reportAddressUnattributed(index, prefix, err)
		return "", false
	}
	attrs := link.Attrs()
	if attrs == nil {
		return "", false
	}
	m.linkNames.Store(index, attrs.Name)
	return attrs.Name, true
}

// handleNeighUpdate processes neighbor table changes. It emits router
// discovery/loss events when the NTF_ROUTER flag appears or disappears
// on a neighbor entry. Only IPv6 link-local neighbors are relevant.
//
// RFC 4861: the kernel sets NTF_ROUTER when it receives a Router Advertisement
// from a neighbor. The flag is cleared when the router sends an RA with
// lifetime=0 or the neighbor entry transitions to NUD_FAILED/deleted.
func (m *monitor) handleNeighUpdate(nu netlink.NeighUpdate) {
	if nu.IP == nil || nu.IP.To4() != nil {
		return // only IPv6 neighbors
	}
	if !nu.IP.IsLinkLocalUnicast() {
		return // only link-local (fe80::) routers are default route gateways
	}

	nameVal, ok := m.linkNames.Load(nu.LinkIndex)
	if !ok {
		return
	}
	ifaceName, isStr := nameVal.(string)
	if !isStr {
		return
	}

	nk := neighKey{linkIndex: nu.LinkIndex, ip: nu.IP.String()}
	isRouter := nu.Flags&netlink.NTF_ROUTER != 0
	isDeleted := nu.Type == unix.RTM_DELNEIGH
	isFailed := nu.State == unix.NUD_FAILED

	if isDeleted || isFailed {
		// Only emit loss if we previously saw this neighbor as a router.
		if _, wasRouter := m.knownRouters.LoadAndDelete(nk); wasRouter {
			m.emit(ifaceevents.EventRouterLost, iface.RouterEventPayload{
				Name:     ifaceName,
				RouterIP: nu.IP.String(),
			})
		}
		return
	}

	if isRouter {
		if _, already := m.knownRouters.LoadOrStore(nk, struct{}{}); !already {
			m.emit(ifaceevents.EventRouterDiscovered, iface.RouterEventPayload{
				Name:     ifaceName,
				RouterIP: nu.IP.String(),
			})
		}
	} else {
		// NTF_ROUTER cleared (e.g., router sent RA with lifetime=0).
		// Only emit if we previously tracked this neighbor as a router.
		if _, wasRouter := m.knownRouters.LoadAndDelete(nk); wasRouter {
			m.emit(ifaceevents.EventRouterLost, iface.RouterEventPayload{
				Name:     ifaceName,
				RouterIP: nu.IP.String(),
			})
		}
	}
}

// emit marshals the payload and emits it on the interface namespace.
func (m *monitor) emit(eventType string, payload any) {
	if m.eventBus == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		logger().Debug("iface monitor: marshal failed", "event", eventType, "err", err)
		return
	}
	if _, err := m.eventBus.Emit(ifaceevents.Namespace, eventType, string(data)); err != nil {
		logger().Debug("iface monitor: emit failed", "event", eventType, "err", err)
	}
}

func addrUpdateToEventType(isNew bool) string {
	if isNew {
		return ifaceevents.EventAddrAdded
	}
	return ifaceevents.EventAddrRemoved
}

func resolveVLANUnit(name string) (parent string, unit int, isVLAN bool) {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 {
		return name, 0, false
	}
	suffix := name[idx+1:]
	vid, err := strconv.Atoi(suffix)
	if err != nil || vid < 0 {
		return name, 0, false
	}
	return name[:idx], vid, true
}

func isLinkUp(attrs *netlink.LinkAttrs) bool {
	if attrs.OperState == netlink.OperUp {
		return true
	}
	if attrs.OperState == netlink.OperUnknown {
		return attrs.RawFlags&unix.IFF_UP != 0
	}
	return false
}

func addrFamily(cidr string) (string, bool) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", false
	}
	if ip.To4() != nil {
		return "ipv4", true //nolint:goconst // AFI label; see ifacenetlink.go for siblings
	}
	return "ipv6", true //nolint:goconst // AFI label; see ifacenetlink.go for siblings
}

// StartMonitor and StopMonitor implement the iface.Backend interface.

func (b *netlinkBackend) StartMonitor(eb ze.EventBus) error {
	if eb == nil {
		return errIfaceNetlinkEventBusIsNil
	}
	b.mon = newMonitor(eb)
	return b.mon.start()
}

func (b *netlinkBackend) StopMonitor() {
	if b.mon != nil {
		b.mon.stop()
		b.mon = nil
	}
}

// Ensure iface is referenced so goimports does not remove the import when
// the Backend interface evolves (the StartMonitor signature change above
// already removed the last direct iface reference from this file's tree).
var _ = iface.TopicPrefix
