// Design: docs/architecture/core-design.md -- BMP message types
//
// Related: bmp.go -- plugin lifecycle and session handling
// Related: header.go -- common and per-peer header encode/decode
// Related: tlv.go -- TLV encode/decode

package bmp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Peer Up fixed fields: LocalAddress(16) + LocalPort(2) + RemotePort(2) = 20.
const peerUpFixedSize = 20

// BGP message header size (marker + length + type).
const bgpHeaderSize = 19

// RFC 4271 Section 4: maximum BGP message size.
const bgpMaxMsgSize = 4096

var (
	errShortMsg     = errors.New("bmp: message too short")
	errBadMsgType   = errors.New("bmp: unknown message type")
	errShortPeerUp  = errors.New("bmp: peer up too short")
	errShortBGPOpen = errors.New("bmp: BGP OPEN too short")
)

// Initiation represents a BMP Initiation message (Type 4).
// RFC 7854 Section 4.3: no per-peer header.
type Initiation struct {
	TLVs []TLV
}

// Termination represents a BMP Termination message (Type 5).
// RFC 7854 Section 4.5: no per-peer header.
type Termination struct {
	TLVs []TLV
}

// PeerUp represents a BMP Peer Up Notification (Type 3).
// RFC 7854 Section 4.10.
type PeerUp struct {
	Peer            PeerHeader
	LocalAddress    [16]byte
	LocalPort       uint16
	RemotePort      uint16
	SentOpenMsg     []byte
	ReceivedOpenMsg []byte
	InfoTLVs        []TLV
}

// PeerDown represents a BMP Peer Down Notification (Type 2).
// RFC 7854 Section 4.9.
type PeerDown struct {
	Peer   PeerHeader
	Reason uint8
	Data   []byte // NOTIFICATION PDU or FSM event code, depending on reason
}

// Peer Down reason codes (RFC 7854 Section 4.9).
const (
	PeerDownLocalNotify   uint8 = 1
	PeerDownLocalNoNotify uint8 = 2
	PeerDownRemoteNotify  uint8 = 3
	PeerDownRemoteNoData  uint8 = 4
	PeerDownDeconfigured  uint8 = 5
)

// RouteMonitoring represents a BMP Route Monitoring message (Type 0).
// RFC 7854 Section 4.6: per-peer header + BGP UPDATE.
type RouteMonitoring struct {
	Peer      PeerHeader
	BGPUpdate []byte // raw BGP UPDATE message (including BGP header)
}

// StatisticsReport represents a BMP Statistics Report (Type 1).
// RFC 7854 Section 4.8.
type StatisticsReport struct {
	Peer  PeerHeader
	Stats []StatEntry
}

// StatEntry is a single statistics counter.
type StatEntry struct {
	Type  uint16
	Value []byte // 8 bytes (global) or 11 bytes (per-AFI/SAFI)
}

// RouteMirroring represents a BMP Route Mirroring message (Type 6).
// RFC 7854 Section 4.7.
type RouteMirroring struct {
	Peer PeerHeader
	TLVs []TLV
}

// DecodeMsg decodes a complete BMP message from buf.
// The buf must contain exactly one complete message (common header length bytes).
func DecodeMsg(buf []byte) (any, error) {
	ch, n, err := DecodeCommonHeader(buf, 0)
	if err != nil {
		return nil, err
	}
	if int(ch.Length) > len(buf) {
		return nil, fmt.Errorf("%w: header says %d, have %d", errShortMsg, ch.Length, len(buf))
	}
	end := int(ch.Length)
	off := n

	switch ch.Type { //nolint:exhaustive // BMP has exactly 7 types, handled below + error
	case MsgInitiation:
		return decodeInitiation(buf, off, end)
	case MsgTermination:
		return decodeTermination(buf, off, end)
	case MsgPeerUpNotify:
		return decodePeerUp(buf, off, end)
	case MsgPeerDownNotify:
		return decodePeerDown(buf, off, end)
	case MsgRouteMonitoring:
		return decodeRouteMonitoring(buf, off, end)
	case MsgStatisticsReport:
		return decodeStatisticsReport(buf, off, end)
	case MsgRouteMirroring:
		return decodeRouteMirroring(buf, off, end)
	}
	return nil, fmt.Errorf("%w: %d", errBadMsgType, ch.Type)
}

func decodeInitiation(buf []byte, off, end int) (*Initiation, error) {
	tlvs, err := DecodeTLVs(buf, off, end)
	if err != nil {
		return nil, fmt.Errorf("initiation: %w", err)
	}
	return &Initiation{TLVs: tlvs}, nil
}

func decodeTermination(buf []byte, off, end int) (*Termination, error) {
	tlvs, err := DecodeTLVs(buf, off, end)
	if err != nil {
		return nil, fmt.Errorf("termination: %w", err)
	}
	return &Termination{TLVs: tlvs}, nil
}

func decodePeerUp(buf []byte, off, end int) (*PeerUp, error) {
	peer, n, err := DecodePeerHeader(buf, off)
	if err != nil {
		return nil, fmt.Errorf("peer up: %w", err)
	}
	off += n

	if end-off < peerUpFixedSize {
		return nil, fmt.Errorf("%w: need %d fixed bytes, have %d", errShortPeerUp, peerUpFixedSize, end-off)
	}

	pu := &PeerUp{Peer: peer}
	copy(pu.LocalAddress[:], buf[off:off+16])
	pu.LocalPort = binary.BigEndian.Uint16(buf[off+16 : off+18])
	pu.RemotePort = binary.BigEndian.Uint16(buf[off+18 : off+20])
	off += peerUpFixedSize

	// Sent OPEN: BGP header (19 bytes) + optional params.
	sentOpen, n, err := extractBGPOpen(buf, off, end)
	if err != nil {
		return nil, fmt.Errorf("peer up sent open: %w", err)
	}
	pu.SentOpenMsg = sentOpen
	off += n

	// Received OPEN.
	recvOpen, n, err := extractBGPOpen(buf, off, end)
	if err != nil {
		return nil, fmt.Errorf("peer up received open: %w", err)
	}
	pu.ReceivedOpenMsg = recvOpen
	off += n

	// Optional trailing TLVs (RFC 9736).
	if off < end {
		tlvs, err := DecodeTLVs(buf, off, end)
		if err != nil {
			return nil, fmt.Errorf("peer up info tlvs: %w", err)
		}
		pu.InfoTLVs = tlvs
	}

	return pu, nil
}

// extractBGPOpen extracts a BGP OPEN message from buf.
// Returns the raw bytes and number of bytes consumed.
func extractBGPOpen(buf []byte, off, end int) ([]byte, int, error) {
	if end-off < bgpHeaderSize {
		return nil, 0, fmt.Errorf("%w: need %d bytes for BGP header", errShortBGPOpen, bgpHeaderSize)
	}
	// BGP message length is at offset 16-17 (after 16-byte marker).
	msgLen := int(binary.BigEndian.Uint16(buf[off+16 : off+18]))
	if msgLen < bgpHeaderSize {
		return nil, 0, fmt.Errorf("%w: BGP length %d", errShortBGPOpen, msgLen)
	}
	// RFC 4271 Section 4: BGP messages are at most 4096 bytes.
	if msgLen > bgpMaxMsgSize {
		return nil, 0, fmt.Errorf("%w: BGP length %d exceeds max %d", errShortBGPOpen, msgLen, bgpMaxMsgSize)
	}
	if off+msgLen > end {
		return nil, 0, fmt.Errorf("%w: BGP OPEN %d bytes, only %d available", errShortBGPOpen, msgLen, end-off)
	}
	return buf[off : off+msgLen], msgLen, nil
}

func decodePeerDown(buf []byte, off, end int) (*PeerDown, error) {
	peer, n, err := DecodePeerHeader(buf, off)
	if err != nil {
		return nil, fmt.Errorf("peer down: %w", err)
	}
	off += n

	if off >= end {
		return nil, fmt.Errorf("%w: no reason byte", errShortMsg)
	}

	pd := &PeerDown{
		Peer:   peer,
		Reason: buf[off],
	}
	off++

	if off < end {
		pd.Data = buf[off:end]
	}

	return pd, nil
}

func decodeRouteMonitoring(buf []byte, off, end int) (*RouteMonitoring, error) {
	peer, n, err := DecodePeerHeader(buf, off)
	if err != nil {
		return nil, fmt.Errorf("route monitoring: %w", err)
	}
	off += n

	return &RouteMonitoring{
		Peer:      peer,
		BGPUpdate: buf[off:end],
	}, nil
}

func decodeStatisticsReport(buf []byte, off, end int) (*StatisticsReport, error) {
	peer, n, err := DecodePeerHeader(buf, off)
	if err != nil {
		return nil, fmt.Errorf("statistics report: %w", err)
	}
	off += n

	// Stats count (4 bytes).
	if end-off < 4 {
		return nil, fmt.Errorf("%w: no stats count", errShortMsg)
	}
	count := binary.BigEndian.Uint32(buf[off : off+4])
	off += 4

	// Cap pre-allocation: each stat is at least TLVHeaderSize bytes,
	// so the maximum possible entries is bounded by remaining buffer.
	maxFromBuf := uint32(end-off) / TLVHeaderSize
	stats := make([]StatEntry, 0, min(count, maxFromBuf))
	for i := uint32(0); i < count && off < end; i++ {
		if end-off < TLVHeaderSize {
			return nil, fmt.Errorf("%w: stat entry %d truncated", errShortMsg, i)
		}
		se := StatEntry{
			Type: binary.BigEndian.Uint16(buf[off : off+2]),
		}
		length := binary.BigEndian.Uint16(buf[off+2 : off+4])
		off += TLVHeaderSize
		if off+int(length) > end {
			return nil, fmt.Errorf("%w: stat value %d truncated", errShortMsg, i)
		}
		se.Value = buf[off : off+int(length)]
		off += int(length)
		stats = append(stats, se)
	}

	return &StatisticsReport{Peer: peer, Stats: stats}, nil
}

func decodeRouteMirroring(buf []byte, off, end int) (*RouteMirroring, error) {
	peer, n, err := DecodePeerHeader(buf, off)
	if err != nil {
		return nil, fmt.Errorf("route mirroring: %w", err)
	}
	off += n

	tlvs, err := DecodeTLVs(buf, off, end)
	if err != nil {
		return nil, fmt.Errorf("route mirroring: %w", err)
	}
	return &RouteMirroring{Peer: peer, TLVs: tlvs}, nil
}

// --- Encoding ---

// WriteInitiation writes a complete Initiation message into buf at off.
// Returns total bytes written.
func WriteInitiation(buf []byte, off int, init *Initiation) int {
	// Skip-and-backfill: reserve common header, write payload, backfill length.
	start := off
	off += CommonHeaderSize
	off += WriteTLVs(buf, off, init.TLVs)
	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgInitiation})
	return totalLen
}

// WriteTermination writes a complete Termination message into buf at off.
// Returns total bytes written.
func WriteTermination(buf []byte, off int, term *Termination) int {
	start := off
	off += CommonHeaderSize
	off += WriteTLVs(buf, off, term.TLVs)
	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgTermination})
	return totalLen
}

// WritePeerUp writes a complete Peer Up message into buf at off.
// Returns total bytes written.
func WritePeerUp(buf []byte, off int, pu *PeerUp) int {
	start := off
	off += CommonHeaderSize
	off += WritePeerHeader(buf, off, pu.Peer)

	copy(buf[off:off+16], pu.LocalAddress[:])
	binary.BigEndian.PutUint16(buf[off+16:off+18], pu.LocalPort)
	binary.BigEndian.PutUint16(buf[off+18:off+20], pu.RemotePort)
	off += peerUpFixedSize

	copy(buf[off:], pu.SentOpenMsg)
	off += len(pu.SentOpenMsg)

	copy(buf[off:], pu.ReceivedOpenMsg)
	off += len(pu.ReceivedOpenMsg)

	if len(pu.InfoTLVs) > 0 {
		off += WriteTLVs(buf, off, pu.InfoTLVs)
	}

	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgPeerUpNotify})
	return totalLen
}

// WritePeerDown writes a complete Peer Down message into buf at off.
// Returns total bytes written.
func WritePeerDown(buf []byte, off int, pd *PeerDown) int {
	start := off
	off += CommonHeaderSize
	off += WritePeerHeader(buf, off, pd.Peer)
	buf[off] = pd.Reason
	off++
	if len(pd.Data) > 0 {
		copy(buf[off:], pd.Data)
		off += len(pd.Data)
	}
	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgPeerDownNotify})
	return totalLen
}

// WriteRouteMonitoring writes a complete Route Monitoring message into buf at off.
// Returns total bytes written.
func WriteRouteMonitoring(buf []byte, off int, rm *RouteMonitoring) int {
	start := off
	off += CommonHeaderSize
	off += WritePeerHeader(buf, off, rm.Peer)
	copy(buf[off:], rm.BGPUpdate)
	off += len(rm.BGPUpdate)
	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgRouteMonitoring})
	return totalLen
}

// WriteStatisticsReport writes a complete Statistics Report into buf at off.
// Returns total bytes written.
func WriteStatisticsReport(buf []byte, off int, sr *StatisticsReport) int {
	start := off
	off += CommonHeaderSize
	off += WritePeerHeader(buf, off, sr.Peer)

	// Stats count.
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(sr.Stats)))
	off += 4

	for i := range sr.Stats {
		binary.BigEndian.PutUint16(buf[off:off+2], sr.Stats[i].Type)
		binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(len(sr.Stats[i].Value)))
		copy(buf[off+4:], sr.Stats[i].Value)
		off += TLVHeaderSize + len(sr.Stats[i].Value)
	}

	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgStatisticsReport})
	return totalLen
}

// WriteRouteMirroring writes a complete Route Mirroring message into buf at off.
// Returns total bytes written.
func WriteRouteMirroring(buf []byte, off int, rm *RouteMirroring) int {
	start := off
	off += CommonHeaderSize
	off += WritePeerHeader(buf, off, rm.Peer)
	off += WriteTLVs(buf, off, rm.TLVs)
	totalLen := off - start
	WriteCommonHeader(buf, start, CommonHeader{Version: Version, Length: uint32(totalLen), Type: MsgRouteMirroring})
	return totalLen
}
