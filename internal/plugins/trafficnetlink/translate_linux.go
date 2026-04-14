// Design: docs/architecture/core-design.md -- tc type translation

//go:build linux

package trafficnetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// makeHandle builds a tc handle from major:minor parts.
func makeHandle(major, minor uint32) uint32 {
	return (major << 16) | minor
}

// translateQdisc converts a ze Qdisc to a netlink Qdisc.
func translateQdisc(q traffic.Qdisc, linkIdx int) (netlink.Qdisc, error) {
	attrs := netlink.QdiscAttrs{
		LinkIndex: linkIdx,
		Handle:    makeHandle(1, 0), // 1:0 root handle
		Parent:    netlink.HANDLE_ROOT,
	}

	switch q.Type {
	case traffic.QdiscHTB:
		return &netlink.Htb{
			QdiscAttrs: attrs,
			Defcls:     findDefaultClassMinor(q),
			DirectQlen: nil,
		}, nil
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
		// TBF requires rate/limit/buffer from config. The Qdisc struct does not
		// carry TBF-specific fields yet; if the first class has a rate, use it.
		// Otherwise the kernel will reject the qdisc with EINVAL.
		rate := uint64(1_000_000) // 1 Mbps fallback
		if len(q.Classes) > 0 && q.Classes[0].Rate > 0 {
			rate = q.Classes[0].Rate
		}
		return &netlink.Tbf{
			QdiscAttrs: attrs,
			Rate:       rate,
			Limit:      uint32(rate / 8),  // ~1 second of burst at configured rate
			Buffer:     uint32(rate / 64), // ~125ms of burst
		}, nil
	case traffic.QdiscNetem:
		return &netlink.Netem{QdiscAttrs: attrs}, nil
	case traffic.QdiscPrio:
		return &netlink.Prio{QdiscAttrs: attrs, Bands: 3}, nil
	case traffic.QdiscClsact:
		return &netlink.Clsact{QdiscAttrs: attrs}, nil
	case traffic.QdiscIngress:
		return &netlink.Ingress{QdiscAttrs: attrs}, nil
	}
	return nil, fmt.Errorf("unsupported qdisc type %v", q.Type)
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
		Handle:    makeHandle(1, minor),
		Parent:    parentHandle,
	}

	switch qt {
	case traffic.QdiscHTB:
		return &netlink.HtbClass{
			ClassAttrs: attrs,
			Rate:       tc.Rate,
			Ceil:       ceilOrRate(tc),
			Prio:       uint32(tc.Priority),
		}, nil
	case traffic.QdiscHFSC:
		return &netlink.HfscClass{
			ClassAttrs: attrs,
		}, nil
	}
	return nil, fmt.Errorf("unsupported classful qdisc type %v", qt)
}

func ceilOrRate(tc traffic.TrafficClass) uint64 {
	if tc.Ceil > 0 {
		return tc.Ceil
	}
	return tc.Rate
}

// translateFilter converts a ze TrafficFilter to a netlink Filter.
func translateFilter(f traffic.TrafficFilter, linkIdx int, parentHandle, classHandle uint32) (netlink.Filter, error) {
	attrs := netlink.FilterAttrs{
		LinkIndex: linkIdx,
		Parent:    parentHandle,
		Priority:  1,
		Protocol:  0x0003, // ETH_P_ALL
	}

	switch f.Type {
	case traffic.FilterMark:
		// fw filter matches packets whose nfmark equals the filter handle.
		attrs.Handle = f.Value
		return &netlink.FwFilter{
			FilterAttrs: attrs,
			ClassId:     classHandle,
			Mask:        0xFFFFFFFF,
		}, nil
	case traffic.FilterDSCP, traffic.FilterProtocol:
		// u32 filter for DSCP/protocol matching.
		return &netlink.U32{
			FilterAttrs: attrs,
			ClassId:     classHandle,
		}, nil
	}
	return nil, fmt.Errorf("unsupported filter type %v", f.Type)
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
