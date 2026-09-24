// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sFlow v5 flow sample encoding
// Related: flow_adapter.go -- assembles flow_sample datagrams using these record writers

package sflow

import (
	"encoding/binary"
	"net/netip"
)

const (
	// DataFormatFlowSampleExpanded is flow_sample_expanded (enterprise 0, format 3).
	DataFormatFlowSampleExpanded = 0x00000003

	// DataFormatSampledHeader is the sampled_header flow record (enterprise 0, format 1).
	DataFormatSampledHeader = 0x00000001

	// DataFormatExtendedGateway is the extended_gateway flow record (enterprise 0, format 1003).
	DataFormatExtendedGateway = 0x000003EB

	// HeaderProtocolEthernet is the header_protocol value for Ethernet (IEEE 802.3).
	HeaderProtocolEthernet uint32 = 1
)

// flowSampleHeaderSize includes the expanded sample tag and length.
func flowSampleHeaderSize() int {
	return 52
}

// writeFlowSample writes a flow_sample_expanded record header into buf at off.
// Input and output are full-width ifIndex values; zero means unknown.
// The caller MUST write flow records immediately after the header, then call
// backfillFlowSample to set sample_length and flow_records count.
//
// Returns the offset after the header. The returned sampleLengthOff and
// numRecordsOff are positions for backfill.
//
// sFlow v5 Section 5, flow_sample_expanded, byte offsets:
//
//	0 format | 4 length | 8 sequence | 12 source type | 16 source index
//	20 rate | 24 pool | 28 drops | 32 input format | 36 input index
//	40 output format | 44 output index | 48 record count | 52 records
func writeFlowSample(buf []byte, off int, seqNum, sourceID, rate, pool, drops, input, output uint32) (newOff, sampleLengthOff, numRecordsOff int) {
	// sFlow v5 Section 5: "An agent must not mix compact/expanded encodings."
	// Linux ifIndex values can exceed 24 bits, so every sample uses expanded encoding.
	binary.BigEndian.PutUint32(buf[off:], DataFormatFlowSampleExpanded)
	off += 4

	// Skip-and-backfill: sample_length
	sampleLengthOff = off
	off += 4

	// sFlow v5: per-source sequence number
	binary.BigEndian.PutUint32(buf[off:], seqNum)
	off += 4

	// Expanded source ID: type 0 (ifIndex), then the full-width index.
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], sourceID)
	off += 4

	// sFlow v5: sampling_rate (1-in-N)
	binary.BigEndian.PutUint32(buf[off:], rate)
	off += 4

	// sFlow v5: sample_pool (total packets that could have been sampled)
	binary.BigEndian.PutUint32(buf[off:], pool)
	off += 4

	// sFlow v5: drops (packets dropped due to resource limits)
	binary.BigEndian.PutUint32(buf[off:], drops)
	off += 4

	// Expanded input: format 0 (single interface), then the full-width index.
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], input)
	off += 4

	// FlowSample.Output is an ifIndex, not packed discard or multicast metadata.
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], output)
	off += 4

	// Skip-and-backfill: flow_records count
	numRecordsOff = off
	off += 4

	return off, sampleLengthOff, numRecordsOff
}

// backfillFlowSample fills in the sample_length and flow_records count
// after all flow records have been written. The caller MUST call this after
// writeFlowSample and before sending the sample.
func backfillFlowSample(buf []byte, sampleLengthOff, numRecordsOff, endOff int, numRecords uint32) {
	// sample_length = bytes from after the sample_length field to end
	sampleLength := uint32(endOff - sampleLengthOff - 4)
	binary.BigEndian.PutUint32(buf[sampleLengthOff:], sampleLength)
	binary.BigEndian.PutUint32(buf[numRecordsOff:], numRecords)
}

// writeSampledHeader writes a sampled_header flow record into buf at off.
// This is the most common flow record: the first N bytes of the sampled packet.
//
// sFlow v5: sampled_header = enterprise 0, format 1.
// XDR encoding: header bytes are prefixed with a 4-byte count and padded to
// a 4-byte boundary.
func writeSampledHeader(buf []byte, off int, frameLength, stripped uint32, header []byte) int {
	// Record data_format
	binary.BigEndian.PutUint32(buf[off:], DataFormatSampledHeader)
	off += 4

	// Skip-and-backfill: record_length
	recordLengthOff := off
	off += 4

	// header_protocol: Ze samples Ethernet frames only.
	binary.BigEndian.PutUint32(buf[off:], HeaderProtocolEthernet)
	off += 4

	// frame_length (original frame size on wire)
	binary.BigEndian.PutUint32(buf[off:], frameLength)
	off += 4

	// stripped (bytes stripped before sampling, e.g. preamble/FCS)
	binary.BigEndian.PutUint32(buf[off:], stripped)
	off += 4

	// header: XDR variable-length opaque
	// 4-byte count + data + padding to 4-byte boundary
	hdrLen := uint32(len(header))
	binary.BigEndian.PutUint32(buf[off:], hdrLen)
	off += 4

	copy(buf[off:], header)
	off += len(header)

	// XDR padding to 4-byte boundary
	pad := (4 - len(header)%4) % 4
	for i := range pad {
		buf[off+i] = 0
	}
	off += pad

	// Backfill record_length
	recordLength := uint32(off - recordLengthOff - 4)
	binary.BigEndian.PutUint32(buf[recordLengthOff:], recordLength)

	return off
}

// writeExtendedGateway writes an extended_gateway flow record into buf at off.
// Contains BGP context: AS path, communities, local preference, next-hop.
//
// sFlow v5: extended_gateway = enterprise 0, format 1003.
//
// PRECONDITION: buf must hold the whole record. The record is variable length:
// fixed fields (~32 bytes incl. nexthop) + 4*len(dstASPath) + 4*len(communities).
// This is not wired in production yet; when the BGP AS-path enrichment path
// calls it, the caller MUST cap dstASPath/communities to the remaining datagram
// space (see EncodeFlowSample's flowSampleHeaderSize bound) before invoking it,
// since the writes here are not individually bounds-checked.
func writeExtendedGateway(buf []byte, off int, nextHop netip.Addr, agentAS, srcAS, srcPeerAS uint32, dstASPath, communities []uint32, localPref uint32) int {
	// Record data_format
	binary.BigEndian.PutUint32(buf[off:], DataFormatExtendedGateway)
	off += 4

	// Skip-and-backfill: record_length
	recordLengthOff := off
	off += 4

	// nexthop address (XDR address: type + addr)
	if nextHop.Is6() {
		binary.BigEndian.PutUint32(buf[off:], AddressTypeIPv6)
		off += 4
		a := nextHop.As16()
		copy(buf[off:], a[:])
		off += 16
	} else {
		binary.BigEndian.PutUint32(buf[off:], AddressTypeIPv4)
		off += 4
		a := nextHop.As4()
		copy(buf[off:], a[:])
		off += 4
	}

	// as (agent's AS number)
	binary.BigEndian.PutUint32(buf[off:], agentAS)
	off += 4

	// src_as (source AS from longest-prefix match)
	binary.BigEndian.PutUint32(buf[off:], srcAS)
	off += 4

	// src_peer_as (peer AS the route was learned from)
	binary.BigEndian.PutUint32(buf[off:], srcPeerAS)
	off += 4

	// dst_as_path: XDR variable-length array of AS path segments.
	// Simplified: single segment of type AS_SEQUENCE containing all ASNs.
	// XDR: num_segments (4), then per segment: type (4) + count (4) + ASNs
	if len(dstASPath) > 0 {
		binary.BigEndian.PutUint32(buf[off:], 1) // num_segments = 1
		off += 4
		binary.BigEndian.PutUint32(buf[off:], 2) // segment type 2 = AS_SEQUENCE
		off += 4
		binary.BigEndian.PutUint32(buf[off:], uint32(len(dstASPath)))
		off += 4
		for _, asn := range dstASPath {
			binary.BigEndian.PutUint32(buf[off:], asn)
			off += 4
		}
	} else {
		binary.BigEndian.PutUint32(buf[off:], 0) // num_segments = 0
		off += 4
	}

	// communities: XDR variable-length array
	binary.BigEndian.PutUint32(buf[off:], uint32(len(communities)))
	off += 4
	for _, c := range communities {
		binary.BigEndian.PutUint32(buf[off:], c)
		off += 4
	}

	// localpref
	binary.BigEndian.PutUint32(buf[off:], localPref)
	off += 4

	// Backfill record_length
	recordLength := uint32(off - recordLengthOff - 4)
	binary.BigEndian.PutUint32(buf[recordLengthOff:], recordLength)

	return off
}
