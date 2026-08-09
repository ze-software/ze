// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- conntrack destroy-event parser
// Related: destroy_linux.go -- NFNLGRP_CONNTRACK_DESTROY multicast listener

package conntrack

import (
	"encoding/binary"
	"net/netip"
	"time"

	"github.com/mdlayher/netlink"
)

// ctnetlink (CTA_*) attribute types from the kernel
// include/uapi/linux/netfilter/nfnetlink_conntrack.h. A conntrack event message
// is an nfgenmsg header (4 bytes) followed by these nested attributes. All
// numeric values are big-endian on the wire (NLA_F_NET_BYTEORDER); addresses
// are raw network-order bytes. mdlayher's AttributeDecoder strips the nested /
// byte-order flags from Type(), so the bare constants match.
const (
	ctaTupleOrig     = 1
	ctaCountersOrig  = 9
	ctaCountersReply = 10
	ctaTimestamp     = 20

	ctaTupleIP    = 1
	ctaTupleProto = 2

	ctaIPv4Src = 1
	ctaIPv4Dst = 2
	ctaIPv6Src = 3
	ctaIPv6Dst = 4

	ctaProtoNum     = 1
	ctaProtoSrcPort = 2
	ctaProtoDstPort = 3

	ctaCountersPackets = 1
	ctaCountersBytes   = 2

	ctaTimestampStart = 1
	ctaTimestampStop  = 2

	// nfgenmsgLen is the size of the netfilter generic message header
	// (family u8, version u8, res_id u16) that precedes the attributes.
	nfgenmsgLen = 4
)

// parseConntrackEvent decodes a ctnetlink destroy message body into a
// FlowEntry. The 5-tuple comes from CTA_TUPLE_ORIG; byte/packet counts are the
// sum of the original and reply directions (matching the periodic-dump path in
// reader_linux.go). Returns false when the message carries no usable tuple
// (e.g. an event for a protocol we did not record, or a malformed buffer).
func parseConntrackEvent(data []byte) (FlowEntry, bool) {
	if len(data) < nfgenmsgLen {
		return FlowEntry{}, false
	}

	ad, err := netlink.NewAttributeDecoder(data[nfgenmsgLen:])
	if err != nil {
		return FlowEntry{}, false
	}
	// Every numeric ctnetlink value is big-endian; nested decoders inherit
	// this, so addresses (read via Bytes) are untouched while ports/counters
	// decode correctly.
	ad.ByteOrder = binary.BigEndian

	var (
		e                 FlowEntry
		haveTuple         bool
		fwdBytes, fwdPkts uint64
		revBytes, revPkts uint64
		tStart, tStop     uint64
	)

	for ad.Next() {
		switch ad.Type() {
		case ctaTupleOrig:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				parseTuple(nad, &e)
				haveTuple = true
				return nil
			})
		case ctaCountersOrig:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				fwdBytes, fwdPkts = parseCounters(nad)
				return nil
			})
		case ctaCountersReply:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				revBytes, revPkts = parseCounters(nad)
				return nil
			})
		case ctaTimestamp:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				for nad.Next() {
					switch nad.Type() {
					case ctaTimestampStart:
						tStart = nad.Uint64()
					case ctaTimestampStop:
						tStop = nad.Uint64()
					}
				}
				return nil
			})
		}
	}

	if ad.Err() != nil || !haveTuple || !e.SrcAddr.IsValid() || !e.DstAddr.IsValid() {
		return FlowEntry{}, false
	}

	e.Bytes = fwdBytes + revBytes
	e.Packets = fwdPkts + revPkts
	if tStart > 0 {
		e.StartTime = time.Unix(0, int64(tStart))
	}
	if tStop > 0 {
		e.LastSeen = time.Unix(0, int64(tStop))
	} else {
		e.LastSeen = time.Now()
	}
	return e, true
}

// parseTuple fills the 5-tuple (addresses, protocol, ports) from a CTA_TUPLE_*
// nested attribute. Addresses are normalized with Unmap so IPv4 flows land in
// the IPv4 enrichment trie rather than as v4-mapped v6.
func parseTuple(ad *netlink.AttributeDecoder, e *FlowEntry) {
	for ad.Next() {
		switch ad.Type() {
		case ctaTupleIP:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				for nad.Next() {
					switch nad.Type() {
					case ctaIPv4Src, ctaIPv6Src:
						if a, ok := netip.AddrFromSlice(nad.Bytes()); ok {
							e.SrcAddr = a.Unmap()
						}
					case ctaIPv4Dst, ctaIPv6Dst:
						if a, ok := netip.AddrFromSlice(nad.Bytes()); ok {
							e.DstAddr = a.Unmap()
						}
					}
				}
				return nil
			})
		case ctaTupleProto:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				for nad.Next() {
					switch nad.Type() {
					case ctaProtoNum:
						e.Protocol = nad.Uint8()
					case ctaProtoSrcPort:
						e.SrcPort = nad.Uint16()
					case ctaProtoDstPort:
						e.DstPort = nad.Uint16()
					}
				}
				return nil
			})
		}
	}
}

// parseCounters reads CTA_COUNTERS_BYTES / CTA_COUNTERS_PACKETS from a
// CTA_COUNTERS_* nested attribute.
func parseCounters(ad *netlink.AttributeDecoder) (bytes, packets uint64) {
	for ad.Next() {
		switch ad.Type() {
		case ctaCountersBytes:
			bytes = ad.Uint64()
		case ctaCountersPackets:
			packets = ad.Uint64()
		}
	}
	return bytes, packets
}
