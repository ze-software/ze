// Design: docs/architecture/core-design.md -- tc type translation

//go:build linux

package trafficnetlink

import (
	"fmt"
	"math"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/dscp"
)

const (
	ethPAll     = 0x0003
	ethPIP      = 0x0800
	ethPIPv6    = 0x86DD
	maxProtocol = 255
)

// tc filter priorities, one per link-layer protocol.
//
// The kernel keeps exactly ONE tcf_proto per (parent, priority) and that
// instance carries a single protocol. Adding a filter whose protocol differs
// from the one already bound to that priority is rejected with EINVAL:
// tcf_chain_tp_find (net/sched/cls_api.c) walks the chain and returns
// ERR_PTR(-EINVAL) when "tp->prio == prio" and "tp->protocol != protocol &&
// protocol"; tc_new_tfilter propagates that as the RTM_NEWTFILTER errno.
//
// Deriving the priority from the protocol makes the invariant "same priority
// => same protocol" hold by construction, for every class and every filter
// type on an interface, instead of depending on each call site picking a free
// number. Filters that DO share a protocol may share a priority: the kernel
// appends them to the existing tcf_proto in handle order.
//
// Lower runs first. An explicit fwmark is an administrative classification
// made upstream (firewall), so it outranks the per-family header matches.
const (
	prioMark uint16 = 1 // ETH_P_ALL  -- fw (fwmark) filters
	prioIPv4 uint16 = 2 // ETH_P_IP   -- u32 IPv4 header matches
	prioIPv6 uint16 = 3 // ETH_P_IPV6 -- u32 IPv6 header matches
)

// filterPriority returns the dedicated tc priority for a filter protocol.
//
// Fail-closed by design: an unmapped protocol yields ok=false rather than a
// default or zero priority. Falling back to a shared number would collide with
// another protocol's tcf_proto and surface as an opaque EINVAL from the kernel
// at apply time; priority 0 would additionally ask the kernel to auto-allocate,
// which collides with every other auto-allocated filter on the same parent.
func filterPriority(protocol uint16) (uint16, bool) {
	switch protocol {
	case ethPAll:
		return prioMark, true
	case ethPIP:
		return prioIPv4, true
	case ethPIPv6:
		return prioIPv6, true
	}
	return 0, false
}

// newFilterAttrs builds the netlink attributes for one filter, assigning the
// priority that belongs to its protocol.
func newFilterAttrs(linkIdx int, parentHandle uint32, protocol uint16) (netlink.FilterAttrs, error) {
	prio, ok := filterPriority(protocol)
	if !ok {
		return netlink.FilterAttrs{}, fmt.Errorf("no tc filter priority assigned for protocol 0x%04X", protocol)
	}
	return netlink.FilterAttrs{
		LinkIndex: linkIdx,
		Parent:    parentHandle,
		Priority:  prio,
		Protocol:  protocol,
	}, nil
}

// rootQdiscMajor is the major number of every tc handle this backend programs.
// Ze attaches exactly one root qdisc per link, at 1:0, and hangs every class off
// it as 1:<minor>, so the major is an invariant of the backend and not a
// parameter: no caller passes anything else, and one that did would be
// programming a qdisc tree this backend cannot reconcile.
const rootQdiscMajor = 1

// tcHandle builds the tc handle 1:<minor> under the root qdisc major.
//
// NOT named makeHandle or classHandle: the vendored
// netlink.MakeHandle(major, minor)
// (vendor/github.com/vishvananda/netlink/qdisc.go) is used in the same files
// and carries the identical formula, so a one-argument local `makeHandle`
// invites collapsing netlink.MakeHandle(1, 0) into makeHandle(1) -- which
// compiles and silently changes 1:0 into 1:1. `classHandle` is taken too:
// it is the parameter name in translateFilter/dscpFilters/protocolFilters/
// u32FilterPair, so a function of that name is shadowed inside each of them.
func tcHandle(minor uint32) uint32 {
	return (rootQdiscMajor << 16) | minor
}

// translateQdisc converts a ze Qdisc to a netlink Qdisc.
func translateQdisc(q traffic.Qdisc, linkIdx int) (netlink.Qdisc, error) {
	if q.Type != traffic.QdiscTBF && q.RateBps != 0 {
		return nil, fmt.Errorf("root rate is only supported for tbf, got %s", q.Type)
	}
	attrs := netlink.QdiscAttrs{
		LinkIndex: linkIdx,
		Handle:    tcHandle(0), // 1:0 root handle
		Parent:    netlink.HANDLE_ROOT,
	}

	switch q.Type {
	case traffic.QdiscHTB:
		htb := netlink.NewHtb(attrs)
		htb.Defcls = findDefaultClassMinor(q)
		return htb, nil
	case traffic.QdiscHFSC:
		return &netlink.Hfsc{
			QdiscAttrs: attrs,
			Defcls:     uint16(findDefaultClassMinor(q)),
		}, nil
	case traffic.QdiscFQ:
		return &netlink.Fq{QdiscAttrs: attrs}, nil
	case traffic.QdiscFQCodel:
		return &netlink.FqCodel{QdiscAttrs: attrs}, nil
	case traffic.QdiscSFQ:
		return &netlink.Sfq{QdiscAttrs: attrs}, nil
	case traffic.QdiscTBF:
		if len(q.Classes) != 0 || q.DefaultClass != "" {
			return nil, fmt.Errorf("tbf is classless; set the root qdisc rate and remove classes and default-class")
		}
		rate, err := rateBytesPerSecond(q.RateBps)
		if err != nil {
			return nil, fmt.Errorf("tbf rate: %w", err)
		}
		// Limit is a byte count: one second of queued traffic. Buffer is
		// NOT bytes: TCA_TBF_PARMS carries the burst's transmission time in
		// scheduler ticks. Size a 125ms burst with room for a full packet.
		limit := max(rate, 2048)
		burst := max(rate/8, 2048)
		if limit > math.MaxUint32 {
			return nil, fmt.Errorf("tbf rate %d bps exceeds the one-second queue limit of %d bytes", q.RateBps, uint64(math.MaxUint32))
		}
		buffer, err := burstTicks(rate, uint32(burst))
		if err != nil {
			return nil, fmt.Errorf("tbf burst: %w", err)
		}
		return &netlink.Tbf{
			QdiscAttrs: attrs,
			Rate:       rate,
			Limit:      uint32(limit),
			Buffer:     buffer,
		}, nil
	case traffic.QdiscNetem:
		return &netlink.Netem{QdiscAttrs: attrs}, nil
	case traffic.QdiscPrio:
		return &netlink.Prio{QdiscAttrs: attrs, Bands: 3}, nil
	// Every qdisc above attaches at the root with handle 1:0, which is where a
	// shaping discipline belongs. These two attach at the ingress hook instead,
	// and they hold no classes. That hook belongs to the mirror and sampling
	// paths, which install clsact at ffff:0 with parent HANDLE_CLSACT
	// (iface/netlink/mirror_linux.go, clsactQdisc). The config surface offers
	// neither (ze-traffic-control-conf.yang), so this refusal is reachable only
	// from a caller that builds a Qdisc by hand. raiseQdiscType still names both
	// when this package READS an interface that carries one.
	case traffic.QdiscClsact, traffic.QdiscIngress:
		return nil, fmt.Errorf("qdisc type %v attaches at the ingress hook, not at the root, and is not configurable", q.Type)
	default:
		// Backend models are open inputs, including unset qdisc types.
		return nil, fmt.Errorf("unsupported qdisc type %v", q.Type)
	}
}

// rateBytesPerSecond refuses fractional bytes rather than silently changing a
// configured bit rate. Both the scheduler and the police action use bytes/s.
func rateBytesPerSecond(bps uint64) (uint64, error) {
	if bps == 0 || bps%8 != 0 {
		return 0, fmt.Errorf("rate %d bps must be positive and divisible by 8 for backend tc", bps)
	}
	return bps / 8, nil
}

// burstTicks checks BOTH narrowing conversions in netlink.Xmittime:
// transmission microseconds to uint32, then scheduler ticks to uint32.
func burstTicks(rate uint64, bytes uint32) (uint32, error) {
	micros := float64(netlink.TIME_UNITS_PER_SEC) * (float64(bytes) / float64(rate))
	tick := netlink.TickInUsec()
	if tick <= 0 || math.IsNaN(tick) || math.IsInf(tick, 0) {
		return 0, fmt.Errorf("cannot read a valid kernel scheduler clock")
	}
	if micros > math.MaxUint32 {
		return 0, fmt.Errorf("burst transmission time exceeds uint32 microseconds")
	}
	ticks := float64(uint32(micros)) * tick
	if ticks < 1 || ticks > math.MaxUint32 {
		return 0, fmt.Errorf("burst transmission time does not fit a positive uint32 scheduler tick count")
	}
	return netlink.Xmittime(rate, bytes), nil
}

// findDefaultClassMinor returns the minor handle number for the default class.
func findDefaultClassMinor(q traffic.Qdisc) uint32 {
	for i, c := range q.Classes {
		if c.Name == q.DefaultClass {
			return uint32(i + 1)
		}
	}
	return 0
}

// translateClass converts a ze TrafficClass to a netlink Class.
func translateClass(qt traffic.QdiscType, tc traffic.TrafficClass, linkIdx int, parentHandle, minor uint32) (netlink.Class, error) {
	attrs := netlink.ClassAttrs{
		LinkIndex: linkIdx,
		Handle:    tcHandle(minor),
		Parent:    parentHandle,
	}

	switch qt {
	case traffic.QdiscHTB:
		rate, err := rateBytesPerSecond(tc.Rate)
		if err != nil {
			return nil, err
		}
		ceil, err := rateBytesPerSecond(ceilOrRate(tc))
		if err != nil {
			return nil, err
		}
		if ceil < rate {
			return nil, fmt.Errorf("htb ceil must be >= rate")
		}
		// NewHtbClass accepts bits/s and also sizes the required token
		// buffers. Validate its narrowing arithmetic before calling it.
		hz := netlink.Hz()
		if hz <= 0 {
			return nil, fmt.Errorf("cannot read a valid kernel scheduler frequency")
		}
		for _, bytesPerSecond := range [...]uint64{rate, ceil} {
			burst := float64(bytesPerSecond)/hz + 1600
			if burst > math.MaxUint32 {
				return nil, fmt.Errorf("htb burst exceeds uint32 bytes")
			}
			if _, err := burstTicks(bytesPerSecond, uint32(burst)); err != nil {
				return nil, fmt.Errorf("htb burst: %w", err)
			}
		}
		return netlink.NewHtbClass(attrs, netlink.HtbClassAttrs{
			Rate: tc.Rate,
			Ceil: ceilOrRate(tc),
			Prio: uint32(tc.Priority),
		}), nil
	case traffic.QdiscHFSC:
		return &netlink.HfscClass{
			ClassAttrs: attrs,
		}, nil
	case traffic.QdiscFQ, traffic.QdiscFQCodel, traffic.QdiscSFQ,
		traffic.QdiscTBF, traffic.QdiscNetem, traffic.QdiscPrio,
		traffic.QdiscClsact, traffic.QdiscIngress:
		return nil, fmt.Errorf("qdisc type %v is classless and cannot have classes", qt)
	default:
		// Backend models are open inputs, including unset qdisc types.
		return nil, fmt.Errorf("unsupported classful qdisc type %v", qt)
	}
}

func ceilOrRate(tc traffic.TrafficClass) uint64 {
	if tc.Ceil > 0 {
		return tc.Ceil
	}
	return tc.Rate
}

// translateFilter converts a ze TrafficFilter to netlink Filters.
// DSCP and protocol filters produce two u32 filters (IPv4 + IPv6).
func translateFilter(f traffic.TrafficFilter, linkIdx int, parentHandle, classHandle uint32) ([]netlink.Filter, error) {
	switch f.Type {
	case traffic.FilterMark:
		attrs, err := newFilterAttrs(linkIdx, parentHandle, ethPAll)
		if err != nil {
			return nil, err
		}
		attrs.Handle = f.Value
		return []netlink.Filter{&netlink.FwFilter{
			FilterAttrs: attrs,
			ClassId:     classHandle,
			Mask:        0xFFFFFFFF,
		}}, nil
	case traffic.FilterDSCP:
		if f.Value > dscp.MaxValue {
			return nil, fmt.Errorf("dscp value %d out of range (0-%d)", f.Value, dscp.MaxValue)
		}
		return dscpFilters(f.Value, linkIdx, parentHandle, classHandle)
	case traffic.FilterProtocol:
		if f.Value > maxProtocol {
			return nil, fmt.Errorf("protocol value %d out of range (0-%d)", f.Value, maxProtocol)
		}
		return protocolFilters(f.Value, linkIdx, parentHandle, classHandle)
	default:
		// Backend models are open inputs, including unset filter types.
		return nil, fmt.Errorf("unsupported filter type %v", f.Type)
	}
}

func dscpFilters(dscp uint32, linkIdx int, parentHandle, classHandle uint32) ([]netlink.Filter, error) {
	// IPv4: TOS byte at IP header offset 0, byte 1. DSCP = top 6 bits.
	v4Key := nl.TcU32Key{
		Val:  dscp << 18, // (dscp << 2) << 16: DSCP shifted to TOS, then to byte 1 in 32-bit word
		Mask: 0x00FC0000, // top 6 bits of byte 1
		Off:  0,
	}
	// IPv6: traffic class spans version/TC/flow-label word at offset 0.
	// DSCP occupies bits 27-22 of the 32-bit big-endian word.
	v6Key := nl.TcU32Key{
		Val:  dscp << 22,
		Mask: 0x0FC00000,
		Off:  0,
	}
	return u32FilterPair(v4Key, v6Key, linkIdx, parentHandle, classHandle)
}

func protocolFilters(proto uint32, linkIdx int, parentHandle, classHandle uint32) ([]netlink.Filter, error) {
	// IPv4: protocol byte at IP header offset 8, byte 1 (TTL, Protocol, Checksum).
	v4Key := nl.TcU32Key{
		Val:  proto << 16,
		Mask: 0x00FF0000,
		Off:  8,
	}
	// IPv6: next header byte at offset 4, byte 2 (Payload Length, NH, Hop Limit).
	v6Key := nl.TcU32Key{
		Val:  proto << 8,
		Mask: 0x0000FF00,
		Off:  4,
	}
	return u32FilterPair(v4Key, v6Key, linkIdx, parentHandle, classHandle)
}

// u32FilterPair builds the IPv4 and IPv6 halves of one match. The two halves
// carry different link-layer protocols, so they MUST land on different tc
// priorities: sharing one is what the kernel rejects with EINVAL (see the
// filterPriority doc). filterPriority guarantees that here.
func u32FilterPair(v4Key, v6Key nl.TcU32Key, linkIdx int, parentHandle, classHandle uint32) ([]netlink.Filter, error) {
	v4Attrs, err := newFilterAttrs(linkIdx, parentHandle, ethPIP)
	if err != nil {
		return nil, err
	}
	v6Attrs, err := newFilterAttrs(linkIdx, parentHandle, ethPIPv6)
	if err != nil {
		return nil, err
	}
	return []netlink.Filter{
		&netlink.U32{
			FilterAttrs: v4Attrs,
			ClassId:     classHandle,
			Sel: &nl.TcU32Sel{
				Flags: nl.TC_U32_TERMINAL,
				Nkeys: 1,
				Keys:  []nl.TcU32Key{v4Key},
			},
		},
		&netlink.U32{
			FilterAttrs: v6Attrs,
			ClassId:     classHandle,
			Sel: &nl.TcU32Sel{
				Flags: nl.TC_U32_TERMINAL,
				Nkeys: 1,
				Keys:  []nl.TcU32Key{v6Key},
			},
		},
	}, nil
}

// raiseQdiscType maps a netlink Qdisc to a ze QdiscType.
func raiseQdiscType(q netlink.Qdisc) traffic.QdiscType {
	switch q.(type) {
	case *netlink.Htb:
		return traffic.QdiscHTB
	case *netlink.Hfsc:
		return traffic.QdiscHFSC
	case *netlink.Fq:
		return traffic.QdiscFQ
	case *netlink.FqCodel:
		return traffic.QdiscFQCodel
	case *netlink.Sfq:
		return traffic.QdiscSFQ
	case *netlink.Tbf:
		return traffic.QdiscTBF
	case *netlink.Netem:
		return traffic.QdiscNetem
	case *netlink.Prio:
		return traffic.QdiscPrio
	case *netlink.Clsact:
		return traffic.QdiscClsact
	case *netlink.Ingress:
		return traffic.QdiscIngress
	}
	return traffic.QdiscHTB // fallback
}
