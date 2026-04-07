// Design: docs/features/interfaces.md -- Netlink interface monitor
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// monitor watches OS interface changes via netlink multicast and publishes
// events to the Bus. It is a long-lived goroutine, not per-event.
//
// start MUST be called exactly once. stop MUST be called after a successful
// start to release resources. stop is safe to call multiple times.
type monitor struct {
	bus     ze.Bus
	done    chan struct{}
	stopCh  chan struct{}
	stopFn  sync.Once
	started atomic.Bool
	known   sync.Map // map[int]struct{}
}

func newMonitor(bus ze.Bus) *monitor {
	return &monitor{
		bus:    bus,
		done:   make(chan struct{}),
		stopCh: make(chan struct{}),
	}
}

func (m *monitor) start() error {
	if !m.started.CompareAndSwap(false, true) {
		return fmt.Errorf("iface monitor: already started")
	}

	linkCh := make(chan netlink.LinkUpdate, 64)
	addrCh := make(chan netlink.AddrUpdate, 64)

	if err := netlink.LinkSubscribe(linkCh, m.stopCh); err != nil {
		m.started.Store(false)
		return fmt.Errorf("iface monitor: link subscribe: %w", err)
	}
	if err := netlink.AddrSubscribe(addrCh, m.stopCh); err != nil {
		m.stopFn.Do(func() { close(m.stopCh) })
		m.started.Store(false)
		return fmt.Errorf("iface monitor: addr subscribe: %w", err)
	}

	go m.run(linkCh, addrCh)
	return nil
}

func (m *monitor) stop() {
	if !m.started.Load() {
		return
	}
	m.stopFn.Do(func() { close(m.stopCh) })
	<-m.done
}

func (m *monitor) run(linkCh <-chan netlink.LinkUpdate, addrCh <-chan netlink.AddrUpdate) {
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

func (m *monitor) safeHandleLinkUpdate(lu netlink.LinkUpdate) {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface monitor: panic in link handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleLinkUpdate(lu)
}

func (m *monitor) safeHandleAddrUpdate(au netlink.AddrUpdate) {
	defer func() {
		if r := recover(); r != nil {
			loggerPtr.Load().Error("iface monitor: panic in addr handler",
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	m.handleAddrUpdate(au)
}

func (m *monitor) handleLinkUpdate(lu netlink.LinkUpdate) {
	attrs := lu.Attrs()
	if attrs == nil {
		return
	}
	name := attrs.Name
	idx := attrs.Index

	switch lu.Header.Type {
	case unix.RTM_NEWLINK:
		if _, seen := m.known.LoadOrStore(idx, struct{}{}); !seen {
			m.publishJSON(iface.TopicCreated, iface.LinkPayload{
				Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
			}, map[string]string{"name": name})
			return
		}
		if isLinkUp(attrs) {
			m.publishJSON(iface.TopicUp, iface.StatePayload{Name: name, Index: idx},
				map[string]string{"name": name})
		} else {
			m.publishJSON(iface.TopicDown, iface.StatePayload{Name: name, Index: idx},
				map[string]string{"name": name})
		}
	case unix.RTM_DELLINK:
		m.known.Delete(idx)
		m.publishJSON(iface.TopicDeleted, iface.LinkPayload{
			Name: name, Type: lu.Type(), Index: idx, MTU: attrs.MTU,
		}, map[string]string{"name": name})
	}
}

func (m *monitor) handleAddrUpdate(au netlink.AddrUpdate) {
	if au.LinkAddress.IP == nil {
		return
	}
	if au.Flags&unix.IFA_F_TENTATIVE != 0 {
		return
	}

	link, err := netlink.LinkByIndex(au.LinkIndex)
	if err != nil {
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

	fam, ok := addrFamily(au.LinkAddress.String())
	if !ok {
		return
	}

	topic := addrUpdateToTopic(au.NewAddr)
	payload := iface.AddrPayload{
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

func (m *monitor) publishJSON(topic string, payload any, metadata map[string]string) {
	data, err := json.Marshal(payload)
	if err != nil {
		loggerPtr.Load().Debug("iface monitor: marshal failed", "topic", topic, "err", err)
		return
	}
	m.bus.Publish(topic, data, metadata)
}

func addrUpdateToTopic(isNew bool) string {
	if isNew {
		return iface.TopicAddrAdded
	}
	return iface.TopicAddrRemoved
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
		return "ipv4", true
	}
	return "ipv6", true
}

// StartMonitor and StopMonitor implement the iface.Backend interface.

func (b *netlinkBackend) StartMonitor(bus ze.Bus) error {
	if bus == nil {
		return fmt.Errorf("iface-netlink: bus is nil")
	}
	b.mon = newMonitor(bus)
	return b.mon.start()
}

func (b *netlinkBackend) StopMonitor() {
	if b.mon != nil {
		b.mon.stop()
		b.mon = nil
	}
}
