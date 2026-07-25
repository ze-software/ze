// Design: plan/learned/818-flow-export-1-counter-export.md -- sFlow v5 datagram encoder

package sflow

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

const (
	// Version is the sFlow v5 datagram version number.
	Version = 5

	// AddressTypeIPv4 is the sFlow v5 address type for IPv4 agent addresses.
	AddressTypeIPv4 = 1
	// AddressTypeIPv6 is the sFlow v5 address type for IPv6 agent addresses.
	AddressTypeIPv6 = 2

	// HeaderSizeIPv4 is the sFlow datagram header size with an IPv4 agent address (28 bytes).
	HeaderSizeIPv4 = 28

	// HeaderSizeIPv6 is the sFlow datagram header size with an IPv6 agent address (40 bytes).
	HeaderSizeIPv6 = 40
)

// HeaderSize returns the datagram header size for the given agent address.
func HeaderSize(addr netip.Addr) int {
	if addr.Is6() {
		return HeaderSizeIPv6
	}
	return HeaderSizeIPv4
}

// WriteDatagramHeader writes an sFlow v5 datagram header into buf at off.
// Returns the new offset after the header.
func WriteDatagramHeader(buf []byte, off int, agentAddr netip.Addr, subAgentID, seqNum, uptime, numSamples uint32) int {
	// sFlow v5: version = 5
	binary.BigEndian.PutUint32(buf[off:], Version)
	off += 4

	if agentAddr.Is6() {
		// sFlow v5: address_type = 2 (IPv6)
		binary.BigEndian.PutUint32(buf[off:], AddressTypeIPv6)
		off += 4
		a := agentAddr.As16()
		copy(buf[off:], a[:])
		off += 16
	} else {
		// sFlow v5: address_type = 1 (IPv4)
		binary.BigEndian.PutUint32(buf[off:], AddressTypeIPv4)
		off += 4
		a := agentAddr.As4()
		copy(buf[off:], a[:])
		off += 4
	}

	binary.BigEndian.PutUint32(buf[off:], subAgentID)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], seqNum)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], uptime)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], numSamples)
	off += 4

	return off
}

// WriteCounterDatagrams encodes one or more sFlow v5 datagrams containing
// counter samples for the given interfaces. Each datagram is written into
// buf (which must be at least MaxDatagramSize bytes). When the next counter
// sample would overflow the datagram, a copy is made and a new datagram
// is started.
//
// seqNums maps ifIndex to the per-source sequence number. Updated in place.
// datagramSeq is the starting per-agent datagram sequence number.
//
// Returns a slice of completed datagram byte slices and the next datagram
// sequence number.
func WriteCounterDatagrams(buf []byte, agentAddr netip.Addr, subAgentID, datagramSeq, uptime uint32, ifaces []flowexport.InterfaceCounters, seqNums map[uint32]uint32) ([][]byte, uint32) {
	if len(ifaces) == 0 {
		return nil, datagramSeq
	}

	hdrSize := HeaderSize(agentAddr)
	var datagrams [][]byte

	off := 0
	numSamplesOff := 0
	var sampleCount uint32

	startDatagram := func() {
		off = 0
		sampleCount = 0
		// Write header with num_samples = 0; backfill later.
		off = WriteDatagramHeader(buf, off, agentAddr, subAgentID, datagramSeq, uptime, 0)
		// Save offset of num_samples for backfill (last 4 bytes of header).
		numSamplesOff = off - 4
	}

	flushDatagram := func() {
		// sFlow v5: backfill num_samples at saved offset
		binary.BigEndian.PutUint32(buf[numSamplesOff:], sampleCount)
		// Copy the datagram bytes out of the reusable buffer.
		dg := make([]byte, off)
		copy(dg, buf[:off])
		datagrams = append(datagrams, dg)
		datagramSeq++
	}

	startDatagram()

	for i := range ifaces {
		c := &ifaces[i]

		// Check if this counter sample fits in the current datagram.
		sampleSize := CounterSampleSize()
		if off+sampleSize > flowexport.MaxDatagramSize && sampleCount > 0 {
			flushDatagram()
			startDatagram()
		}

		// Overflow protection: single sample larger than datagram
		// (should not happen with 1400 byte datagrams and ~116 byte samples,
		// but guard against it).
		if hdrSize+sampleSize > flowexport.MaxDatagramSize {
			continue
		}

		seq := seqNums[c.IfIndex]
		seq++
		seqNums[c.IfIndex] = seq

		off = WriteCounterSample(buf, off, c.IfIndex, seq, c)
		sampleCount++
	}

	if sampleCount > 0 {
		flushDatagram()
	}

	return datagrams, datagramSeq
}
