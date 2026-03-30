// Design: plan/spec-iface-0-umbrella.md — Netlink interface monitor
// Overview: iface.go — shared types and topic constants

package iface

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

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Monitor watches OS interface changes via netlink multicast and publishes
// events to the Bus. It is a long-lived goroutine, not per-event.
//
// Start MUST be called exactly once. Stop MUST be called after a successful
// Start to release resources. Stop is safe to call multiple times.
// If Start returns an error, Stop MUST NOT be called.
type Monitor struct {
	bus     ze.Bus
	done    chan struct{}
	stop    chan struct{}
	stopFn  sync.Once
	started atomic.Bool

	// known tracks interface indices seen via RTM_NEWLINK.
	// Used to distinguish create (new index) from state change (known index).
	known sync.Map // map[int]struct{}
}

// NewMonitor creates a monitor that publishes interface events to bus.
// Bus must not be nil.
func NewMonitor(bus ze.Bus) (*Monitor, error) {
	if bus == nil {
		return nil, errors.New("iface monitor: bus is nil")
	}
	return &Monitor{
		bus:  bus,
		done: make(chan struct{}),
		stop: make(chan struct{}),
	}, nil
}

// Start begins monitoring in a goroutine. Returns immediately.
// MUST call Stop to release resources after a successful Start.
// Must not be called more than once.
func (m *Monitor) Start() error {
	if !m.started.CompareAndSwap(false, true) {
		return errors.New("iface monitor: already started")
	}

	linkCh := make(chan netlink.LinkUpdate, 64)
	addrCh := make(chan netlink.AddrUpdate, 64)

	if err := netlink.LinkSubscribe(linkCh, m.stop); err != nil {
		return fmt.Errorf("iface monitor: link subscribe: %w", err)
	}
	if err := netlink.AddrSubscribe(addrCh, m.stop); err != nil {
		m.stopFn.Do(func() { close(m.stop) })
		return fmt.Errorf("iface monitor: addr subscribe: %w", err)
	}

	go m.run(linkCh, addrCh)
	return nil
}

// Stop closes the netlink socket and waits for the monitor goroutine to exit.
// Safe to call multiple times.
func (m *Monitor) Stop() {
	m.stopFn.Do(func() { close(m.stop) })
	<-m.done
}

// run is the long-lived monitor loop. Recovers from panics to prevent
// a misbehaving bus implementation from crashing the monitor silently.
func (m *Monitor) run(linkCh <-chan netlink.LinkUpdate, addrCh <-chan netlink.AddrUpdate) {
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
		}
	}
}

// safeHandleLinkUpdate wraps handleLinkUpdate with panic recovery.
func (m *Monitor) safeHandleLinkUpdate(lu netlink.LinkUpdate) {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface monitor: panic in link handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleLinkUpdate(lu)
}

// safeHandleAddrUpdate wraps handleAddrUpdate with panic recovery.
func (m *Monitor) safeHandleAddrUpdate(au netlink.AddrUpdate) {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface monitor: panic in addr handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleAddrUpdate(au)
}

// handleLinkUpdate processes a netlink link event.
//
// RTM_NEWLINK (unix.RTM_NEWLINK = 16) is sent by the kernel for both
// new interfaces AND state changes on existing interfaces.
// RTM_DELLINK (unix.RTM_DELLINK = 17) is sent only for deletion.
//
// We distinguish create from state-change by tracking known indices:
//   - New index + RTM_NEWLINK = TopicCreated
//   - Known index + RTM_NEWLINK = TopicUp or TopicDown (state change)
//   - RTM_DELLINK = TopicDeleted (index removed from tracking)
func (m *Monitor) handleLinkUpdate(lu netlink.LinkUpdate) {
	attrs := lu.Attrs()
	if attrs == nil {
		return
	}

	name := attrs.Name
	idx := attrs.Index

	switch lu.Header.Type {
	case unix.RTM_NEWLINK:
		if _, seen := m.known.LoadOrStore(idx, struct{}{}); !seen {
			// First time seeing this index: interface created.
			m.publishJSON(TopicCreated, LinkPayload{
				Name:  name,
				Type:  lu.Type(),
				Index: idx,
				MTU:   attrs.MTU,
			}, map[string]string{"name": name})
			return
		}
		// Known index: state change.
		if isLinkUp(attrs) {
			m.publishJSON(TopicUp, StatePayload{Name: name, Index: idx},
				map[string]string{"name": name})
		} else {
			m.publishJSON(TopicDown, StatePayload{Name: name, Index: idx},
				map[string]string{"name": name})
		}

	case unix.RTM_DELLINK:
		m.known.Delete(idx)
		m.publishJSON(TopicDeleted, LinkPayload{
			Name:  name,
			Type:  lu.Type(),
			Index: idx,
			MTU:   attrs.MTU,
		}, map[string]string{"name": name})
	}
}

func (m *Monitor) handleAddrUpdate(au netlink.AddrUpdate) {
	if au.LinkAddress.IP == nil {
		return
	}

	// Skip tentative IPv6 (DAD incomplete).
	if au.Flags&0x40 != 0 { // IFA_F_TENTATIVE
		return
	}

	link, err := netlink.LinkByIndex(au.LinkIndex)
	if err != nil {
		// Race: interface deleted between addr event and lookup, or netlink error.
		loggerPtr.Load().Debug("iface monitor: link lookup failed",
			"index", au.LinkIndex, "err", err)
		return
	}

	attrs := link.Attrs()
	if attrs == nil {
		return
	}

	ifaceName := attrs.Name
	parent, unit, _ := resolveVLANUnit(ifaceName)
	addr := au.LinkAddress.IP.String()
	ones, _ := au.LinkAddress.Mask.Size()

	family, ok := addrFamily(au.LinkAddress.String())
	if !ok {
		return
	}

	topic := addrUpdateToTopic(au.NewAddr)
	payload := AddrPayload{
		Name:         parent,
		Unit:         unit,
		Index:        au.LinkIndex,
		Address:      addr,
		PrefixLength: ones,
		Family:       family,
	}

	m.publishJSON(topic, payload, map[string]string{
		"name":    parent,
		"unit":    strconv.Itoa(unit),
		"address": addr,
		"family":  family,
	})
}

func (m *Monitor) publishJSON(topic string, payload any, metadata map[string]string) {
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("iface monitor: marshal failed",
			"topic", topic, "err", err)
		return
	}
	m.bus.Publish(topic, data, metadata)
}

// addrUpdateToTopic maps a netlink address update to a Bus topic.
func addrUpdateToTopic(isNew bool) string {
	if isNew {
		return TopicAddrAdded
	}
	return TopicAddrRemoved
}

// resolveVLANUnit resolves an OS interface name to a parent name and unit ID.
// VLAN subinterfaces (e.g., "eth0.100") resolve to parent "eth0", unit 100.
// Plain interfaces resolve to themselves with unit 0.
func resolveVLANUnit(name string) (parent string, unit int, isVLAN bool) {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 {
		// No dot, or dot at position 0 (e.g., ".100") -- not a valid VLAN name.
		return name, 0, false
	}

	suffix := name[idx+1:]
	vid, err := strconv.Atoi(suffix)
	if err != nil || vid < 0 {
		return name, 0, false
	}

	return name[:idx], vid, true
}

// isLinkUp returns true if a link should be considered operationally up.
//
// OperUp is the definitive "up" state. OperUnknown is treated as up when the
// admin flag (IFF_UP) is set, because many virtual interfaces (loopback, dummy,
// bridge, tun/tap) do not report operational state but are fully functional.
func isLinkUp(attrs *netlink.LinkAttrs) bool {
	if attrs.OperState == netlink.OperUp {
		return true
	}
	if attrs.OperState == netlink.OperUnknown {
		return attrs.RawFlags&unix.IFF_UP != 0
	}
	return false
}

// addrFamily returns "ipv4" or "ipv6" for a CIDR string.
func addrFamily(cidr string) (string, bool) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", false
	}
	if ip.To4() != nil {
		return "ipv4", true
	}
	return "ipv6", true
}
