// Design: docs/features/interfaces.md -- Netlink interface monitor
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vishvananda/netlink"
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
	eventBus     ze.EventBus
	done         chan struct{}
	stopCh       chan struct{}
	stopFn       sync.Once
	started      atomic.Bool
	known        sync.Map // map[int]struct{} — known link indices
	linkNames    sync.Map // map[int]string — index to name, updated by link events
	knownRouters sync.Map // map[neighKey]struct{} — neighbors seen with NTF_ROUTER
}

// neighKey identifies a neighbor entry for NTF_ROUTER tracking.
type neighKey struct {
	linkIndex int
	ip        string
}

func newMonitor(eb ze.EventBus) *monitor {
	return &monitor{
		eventBus: eb,
		done:     make(chan struct{}),
		stopCh:   make(chan struct{}),
	}
}

func (m *monitor) start() error {
	if !m.started.CompareAndSwap(false, true) {
		return errIfaceMonitorAlreadyStarted
	}

	linkCh := make(chan netlink.LinkUpdate, 64)
	addrCh := make(chan netlink.AddrUpdate, 64)
	neighCh := make(chan netlink.NeighUpdate, 64)

	if err := netlink.LinkSubscribe(linkCh, m.stopCh); err != nil {
		m.started.Store(false)
		return fmt.Errorf("iface monitor: link subscribe: %w", err)
	}
	if err := netlink.AddrSubscribe(addrCh, m.stopCh); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: addr subscribe: %w", err)
	}
	if err := netlink.NeighSubscribe(neighCh, m.stopCh); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: neigh subscribe: %w", err)
	}

	m.seedLinkNames()

	go m.run(linkCh, addrCh, neighCh)
	return nil
}

// seedLinkNames fills the index->name cache with the links that ALREADY exist.
//
// LinkSubscribe delivers only CHANGES, so without this the cache starts empty
// and stays empty for every interface that predates the monitor. handleAddrUpdate
// resolves an address event's interface through that cache and returns early on a
// miss (see "unknown link index for addr event"), so every address event on such
// an interface was silently dropped.
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
// `m.known` is deliberately NOT seeded here: it gates the `created` event, and
// this restores address events only, without changing create/down semantics.
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
	<-m.done
}

func (m *monitor) run(linkCh <-chan netlink.LinkUpdate, addrCh <-chan netlink.AddrUpdate, neighCh <-chan netlink.NeighUpdate) {
	defer close(m.done)
	for {
		select {
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
		if _, seen := m.known.LoadOrStore(idx, struct{}{}); !seen {
			m.emit(ifaceevents.EventCreated, linkEventPayload{
				Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
			})
			return
		}
		if isLinkUp(attrs) {
			m.emit(ifaceevents.EventUp, stateEventPayload{Name: name, Index: idx})
		} else {
			m.emit(ifaceevents.EventDown, stateEventPayload{Name: name, Index: idx})
		}
	case unix.RTM_DELLINK:
		m.known.Delete(idx)
		m.linkNames.Delete(idx)
		// Interface deletion maps to (interface, down): there is no
		// separate "deleted" event type in the stream registry. Down is
		// the closest semantic match (link is no longer operational).
		m.emit(ifaceevents.EventDown, linkEventPayload{
			Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
		})
	}
}

func (m *monitor) handleAddrUpdate(au netlink.AddrUpdate) {
	if au.LinkAddress.IP == nil {
		return
	}
	if au.Flags&unix.IFA_F_TENTATIVE != 0 && au.LinkAddress.IP.To4() == nil {
		return
	}

	nameVal, ok := m.linkNames.Load(au.LinkIndex)
	if !ok {
		logger().Debug("iface monitor: unknown link index for addr event",
			"index", au.LinkIndex)
		return
	}
	ifaceName, isStr := nameVal.(string)
	if !isStr {
		return
	}
	parent, unit, _ := resolveVLANUnit(ifaceName)
	addr := au.LinkAddress.IP.String()
	ones, _ := au.LinkAddress.Mask.Size()

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
	m.emit(eventType, addrEventPayload{
		Name:         parent,
		Unit:         unit,
		Index:        au.LinkIndex,
		Address:      addr,
		PrefixLength: ones,
		Family:       fam,
		Origin:       origin,
	})
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
