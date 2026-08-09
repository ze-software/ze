//go:build linux

// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Conntrack netlink reader

package conntrack

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Reader reads conntrack entries via netlink. It supports periodic table
// dumps via ConntrackTableList. Destroy event monitoring for immediate
// final-record export can be added via raw netlink subscription to
// NFNLGRP_CONNTRACK_DESTROY in a future iteration.
type Reader struct {
	mu     sync.Mutex
	handle *netlink.Handle
	closed bool
}

// NewReader creates a conntrack reader with a netlink handle for table
// dumps.
func NewReader() (*Reader, error) {
	h, err := netlink.NewHandle()
	if err != nil {
		return nil, err
	}
	return &Reader{handle: h}, nil
}

// Dump returns all conntrack entries from the kernel table.
// Returns entries for both IPv4 and IPv6.
func (r *Reader) Dump() ([]FlowEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, errors.New("conntrack: reader closed")
	}

	var entries []FlowEntry

	for _, family := range []netlink.InetFamily{unix.AF_INET, unix.AF_INET6} {
		flows, err := r.handle.ConntrackTableList(netlink.ConntrackTable, family)
		if err != nil {
			return entries, err
		}

		for _, f := range flows {
			e, ok := convertFlow(f)
			if !ok {
				continue
			}
			entries = append(entries, e)
		}
	}

	return entries, nil
}

// convertFlow translates a vishvananda ConntrackFlow into a FlowEntry.
// Returns false if the entry cannot be converted (e.g., missing addresses).
func convertFlow(f *netlink.ConntrackFlow) (FlowEntry, bool) {
	srcAddr, ok := ipToAddr(f.Forward.SrcIP)
	if !ok {
		return FlowEntry{}, false
	}
	dstAddr, ok := ipToAddr(f.Forward.DstIP)
	if !ok {
		return FlowEntry{}, false
	}

	entry := FlowEntry{
		SrcAddr:  srcAddr,
		DstAddr:  dstAddr,
		SrcPort:  f.Forward.SrcPort,
		DstPort:  f.Forward.DstPort,
		Protocol: f.Forward.Protocol,
		Bytes:    f.Forward.Bytes + f.Reverse.Bytes,
		Packets:  f.Forward.Packets + f.Reverse.Packets,
		Mark:     f.Mark,
	}

	// Capture the TCP connection state where the kernel reported it
	// (CTA_PROTOINFO_TCP_STATE). A SYN flood leaves many entries stuck in
	// SYN_SENT/SYN_RECV; the DDoS characterizer classifies on that dominance.
	if tcp, ok := f.ProtoInfo.(*netlink.ProtoInfoTCP); ok && tcp != nil {
		entry.TCPState = tcp.State
	}

	if f.TimeStart > 0 {
		entry.StartTime = time.Unix(0, int64(f.TimeStart))
	}
	if f.TimeStop > 0 {
		entry.LastSeen = time.Unix(0, int64(f.TimeStop))
	} else {
		entry.LastSeen = time.Now()
	}

	return entry, true
}

func ipToAddr(ip net.IP) (netip.Addr, bool) {
	if ip == nil {
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// Close releases the netlink handle.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	if r.handle != nil {
		r.handle.Close()
		r.handle = nil
	}
	return nil
}
