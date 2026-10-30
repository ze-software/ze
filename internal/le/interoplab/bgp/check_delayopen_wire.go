// Design: docs/architecture/testing/interop.md -- complete native TCP/BGP DelayOpen history.
// Related: check_delayopen.go -- capture lifecycle and retained native observations.
package bgp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

const delayOpenPacketLimit = 4096

// delayOpenMessage timestamps the first arrival of every covered byte, so a
// segmented OPEN completes only after all bytes arrived, not on a retransmit.
type delayOpenMessage struct {
	Type            byte
	First, Complete time.Time
}

type delayOpenConnection struct {
	Flow              pcap.Flow // normalized Ze -> FRR, including both native TCP ports
	Initiator         netip.Addr
	Handshake, Closed time.Time
	Ze, FRR           []delayOpenMessage
	Packets           int
	syn               [2]time.Time
	sequence          [2]uint32
	firstPayload      [2]uint32
	hasPayload        [2]bool
	synACK            time.Time
	arrivals          [2][]delayOpenArrival
	acknowledged      [2][]delayOpenAcknowledgement
	finOffset         [2]int
	hasFIN            [2]bool
	finSeen           [2]time.Time
}

// Offsets are relative to ISN+1, after the SYN's sequence number. Each captured
// payload adds one range; the packet bound caps their total across all flows.
type delayOpenArrival struct {
	start, end int
	at         time.Time
}

// Only advancing ACKs are retained: an earlier, larger cumulative ACK already
// imposes the stronger coverage deadline. At most one is added per packet.
type delayOpenAcknowledgement struct {
	end int
	at  time.Time
}

// delayOpenHistory uses the shared pcap reassembler for segmentation and
// retransmissions. The additional Ethernet/IPv4 walk records TCP handshake and
// close flags (which Reassemble deliberately does not expose). The fixture's
// capture filter admits IPv4 TCP/179 only; no packet may be silently skipped.
func delayOpenHistory(raw []byte, ze netip.Addr) ([]*delayOpenConnection, error) {
	reader, err := pcap.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return nil, errors.New("DelayOpen capture is not Ethernet")
	}
	byFlow := make(map[pcap.Flow]*delayOpenConnection)
	var history []*delayOpenConnection
	var record pcap.Record
	var previous time.Time
	for count := 0; ; count++ {
		err := reader.Next(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if count >= delayOpenPacketLimit-1 {
			return nil, errors.New("DelayOpen capture reached packet bound")
		}
		if record.Timestamp.IsZero() || record.Timestamp.Before(previous) {
			return nil, errors.New("DelayOpen capture timestamps missing or reversed")
		}
		previous = record.Timestamp
		frame := record.Data
		if record.OriginalLen != len(frame) || len(frame) < 54 {
			return nil, errors.New("truncated DelayOpen Ethernet frame")
		}
		if binary.BigEndian.Uint16(frame[12:14]) != 0x0800 {
			return nil, errors.New("non-IPv4 packet in DelayOpen capture")
		}
		ip := frame[14:]
		header := int(ip[0]&15) * 4
		total := int(binary.BigEndian.Uint16(ip[2:4]))
		if ip[0]>>4 != 4 || header < 20 || total < header+20 || total > len(ip) || ip[9] != 6 || binary.BigEndian.Uint16(ip[6:8])&0x3fff != 0 {
			return nil, errors.New("invalid or fragmented DelayOpen IPv4/TCP packet")
		}
		tcp := ip[header:total]
		tcpHeader := int(tcp[12]>>4) * 4
		if tcpHeader < 20 || tcpHeader > len(tcp) {
			return nil, errors.New("invalid DelayOpen TCP header")
		}
		source := netip.AddrFrom4([4]byte(ip[12:16]))
		target := netip.AddrFrom4([4]byte(ip[16:20]))
		flow := pcap.Flow{SourceAddr: source, TargetAddr: target, SourcePort: binary.BigEndian.Uint16(tcp[:2]), TargetPort: binary.BigEndian.Uint16(tcp[2:4])}
		if flow.SourcePort != 179 && flow.TargetPort != 179 {
			return nil, errors.New("non-BGP TCP packet in DelayOpen capture")
		}
		direction := 0
		if source != ze {
			if target != ze {
				return nil, errors.New("DelayOpen capture does not involve Ze")
			}
			flow = flow.Reverse()
			direction = 1
		}
		connection := byFlow[flow]
		if connection == nil {
			if len(history) >= pcap.FlowMax/2 {
				return nil, errors.New("DelayOpen connection bound reached")
			}
			connection = &delayOpenConnection{Flow: flow}
			byFlow[flow] = connection
			history = append(history, connection)
		}
		connection.Packets++
		seq, ack, flags := binary.BigEndian.Uint32(tcp[4:8]), binary.BigEndian.Uint32(tcp[8:12]), tcp[13]
		if flags&2 != 0 {
			if !connection.syn[direction].IsZero() && connection.sequence[direction] != seq {
				return nil, errors.New("TCP tuple reused: cannot merge DelayOpen session generations")
			}
			if connection.syn[direction].IsZero() {
				connection.syn[direction], connection.sequence[direction] = record.Timestamp, seq
			}
			if flags&16 == 0 {
				connection.Initiator = source
			} else {
				if connection.syn[1-direction].IsZero() || ack != connection.sequence[1-direction]+1 {
					return nil, errors.New("incomplete DelayOpen TCP handshake: SYN-ACK lacks SYN")
				}
				connection.synACK = record.Timestamp
			}
		}
		if flags&18 == 16 && connection.Handshake.IsZero() && source == connection.Initiator && !connection.synACK.IsZero() && ack == connection.sequence[1-direction]+1 && seq == connection.sequence[direction]+1 {
			connection.Handshake = record.Timestamp
		}
		if len(tcp) > tcpHeader {
			if connection.Handshake.IsZero() {
				return nil, errors.New("BGP payload without complete captured TCP handshake")
			}
			if !connection.hasPayload[direction] || int32(seq-connection.firstPayload[direction]) < 0 {
				connection.firstPayload[direction] = seq
			}
			connection.hasPayload[direction] = true
			offset := int(int32(seq - (connection.sequence[direction] + 1))) //nolint:gosec // bounded signed TCP sequence delta handles uint32 wrap
			end := offset + len(tcp) - tcpHeader
			if offset < 0 || end > pcap.FlowBytesMax {
				return nil, errors.New("DelayOpen TCP payload exceeds bounded sequence window")
			}
			connection.arrivals[direction] = append(connection.arrivals[direction], delayOpenArrival{start: offset, end: end, at: record.Timestamp})
		}
		if !connection.Handshake.IsZero() {
			if flags&16 != 0 {
				ackOffset := int(int32(ack - (connection.sequence[1-direction] + 1))) //nolint:gosec // bounded signed TCP sequence delta handles uint32 wrap
				if ackOffset < 0 || ackOffset > pcap.FlowBytesMax+1 {
					return nil, errors.New("DelayOpen TCP ACK exceeds bounded sequence window")
				}
				acknowledged := connection.acknowledged[1-direction]
				if ackOffset > 0 && (len(acknowledged) == 0 || ackOffset > acknowledged[len(acknowledged)-1].end) {
					connection.acknowledged[1-direction] = append(acknowledged, delayOpenAcknowledgement{end: ackOffset, at: record.Timestamp})
				}
			}
			if flags&1 != 0 {
				finOffset := int(int32(seq-(connection.sequence[direction]+1))) + len(tcp) - tcpHeader //nolint:gosec // FIN follows payload in the same bounded TCP sequence window
				if finOffset < 0 || finOffset > pcap.FlowBytesMax || (connection.hasFIN[direction] && connection.finOffset[direction] != finOffset) {
					return nil, errors.New("inconsistent or out-of-window DelayOpen TCP FIN")
				}
				if !connection.hasFIN[direction] {
					connection.finSeen[direction] = record.Timestamp
				}
				connection.hasFIN[direction], connection.finOffset[direction] = true, finOffset
			}
		}
		if flags&5 != 0 && connection.Closed.IsZero() {
			connection.Closed = record.Timestamp
		}
	}
	for _, connection := range history {
		for direction := range 2 {
			if connection.hasPayload[direction] && connection.firstPayload[direction] != connection.sequence[direction]+1 {
				return nil, errors.New("missing initial TCP payload in DelayOpen capture")
			}
			first := delayOpenFirstArrivals(connection.arrivals[direction])
			connection.arrivals[direction] = first
			end := 0
			var payloadComplete time.Time
			for _, arrival := range first {
				if arrival.start != end {
					return nil, errors.New("incomplete DelayOpen reassembly: missing TCP payload interval")
				}
				end = arrival.end
				if arrival.at.After(payloadComplete) {
					payloadComplete = arrival.at
				}
			}
			ackLimit := end
			if connection.hasFIN[direction] {
				if connection.finOffset[direction] != end {
					return nil, errors.New("incomplete DelayOpen reassembly: FIN proves missing TCP payload")
				}
				if payloadComplete.After(connection.finSeen[direction]) {
					return nil, errors.New("incomplete DelayOpen reassembly: FIN precedes captured TCP payload")
				}
				// RFC 9293: FIN consumes one sequence number, but no payload
				// byte. Only a captured FIN permits that extra acknowledged unit.
				ackLimit++
			}
			// Advancing ACK offsets and first-arrival intervals are both
			// ordered by sequence, so one pass computes every prefix's
			// completion time without re-scanning bytes or intervals.
			rangeIndex := 0
			var prefixComplete time.Time
			for _, acknowledged := range connection.acknowledged[direction] {
				if acknowledged.end > ackLimit {
					return nil, errors.New("incomplete DelayOpen reassembly: ACK proves missing TCP payload or FIN")
				}
				for rangeIndex < len(first) && first[rangeIndex].start < min(acknowledged.end, end) {
					if first[rangeIndex].at.After(prefixComplete) {
						prefixComplete = first[rangeIndex].at
					}
					rangeIndex++
				}
				if prefixComplete.After(acknowledged.at) {
					return nil, errors.New("incomplete DelayOpen reassembly: ACK precedes captured TCP payload")
				}
				if acknowledged.end > end && connection.finSeen[direction].After(acknowledged.at) {
					return nil, errors.New("incomplete DelayOpen reassembly: ACK precedes captured TCP FIN")
				}
			}
		}
	}
	streams, report, err := pcap.Reassemble(bytes.NewReader(raw), 179)
	if err != nil {
		return nil, err
	}
	if report.RecordsRead == 0 || report.RecordsSkipped != 0 || report.RecordsDropped != 0 || len(report.Gaps) != 0 || len(report.Truncated) != 0 {
		return nil, fmt.Errorf("incomplete DelayOpen reassembly: %s", report.String())
	}
	var buffer [65535]byte
	for index := range streams {
		stream := &streams[index]
		flow, reverse := stream.Flow, stream.Flow.SourceAddr != ze
		if reverse {
			flow = flow.Reverse()
		}
		connection := byFlow[flow]
		if connection == nil {
			return nil, errors.New("reassembled stream has no TCP history")
		}
		direction := 0
		if reverse {
			direction = 1
		}
		for offset := 0; offset < len(stream.Bytes); {
			length, err := readExtendedFrame(bytes.NewReader(stream.Bytes[offset:]), buffer[:])
			if err != nil {
				return nil, fmt.Errorf("incomplete DelayOpen BGP frame: %w", err)
			}
			// The live peers can send initial EORs or a collision Cease as
			// well as the measured OPEN/KEEPALIVE pair.
			switch buffer[18] {
			case bgpOpen:
				if length < 29 || buffer[19] != 4 {
					return nil, errors.New("malformed captured BGP OPEN")
				}
			case bgpKeepalive:
				if length != bgpHeaderLength {
					return nil, errors.New("malformed captured BGP KEEPALIVE")
				}
			case bgpNotification:
				if length < 21 {
					return nil, errors.New("malformed captured BGP NOTIFICATION")
				}
			case bgpUpdate:
				if length < 23 {
					return nil, errors.New("malformed captured BGP UPDATE")
				}
			case 5: // ROUTE-REFRESH, RFC 2918 Section 2.
				if length != 23 {
					return nil, errors.New("malformed captured BGP ROUTE-REFRESH")
				}
			default: // Open wire type: unsupported values cannot prove timing.
				return nil, fmt.Errorf("unknown captured BGP message type %d", buffer[18])
			}
			first, complete, err := delayOpenArrivalTimes(connection.arrivals[direction], offset, offset+length)
			if err != nil {
				return nil, err
			}
			message := delayOpenMessage{Type: buffer[18], First: first, Complete: complete}
			if reverse {
				connection.FRR = append(connection.FRR, message)
			} else {
				connection.Ze = append(connection.Ze, message)
			}
			offset += length
		}
	}
	return history, nil
}

// judgeDelayOpen checks EVERY observed connection, not merely the eventual
// winner. RFC 4271 8.2.2 Connect/Active permit OPEN on expiry or after receipt
// of peer OPEN; the early path sends OPEN followed by KEEPALIVE. The 20ms
// tolerance covers the final ACK's wire timestamp versus local TCP completion,
// not a fraction of either configured seconds-long delay.
func judgeDelayOpen(history []*delayOpenConnection, peer netip.Addr, zeDelay, frrDelay time.Duration, early bool) (string, error) {
	const timestampTolerance = 20 * time.Millisecond
	winners, overlaps, completed := 0, 0, 0
	for _, connection := range history {
		if connection.Flow.TargetAddr != peer || connection.Handshake.IsZero() {
			continue
		}
		completed++
		var zeOpen, frrOpen *delayOpenMessage
		for index := range connection.Ze {
			message := &connection.Ze[index]
			if message.Type == bgpOpen {
				if zeOpen != nil {
					return "", errors.New("multiple Ze OPENs on one native connection")
				}
				zeOpen = message
			}
		}
		for index := range connection.FRR {
			message := &connection.FRR[index]
			if message.Type == bgpOpen {
				if frrOpen != nil {
					return "", errors.New("multiple FRR OPENs on one native connection")
				}
				frrOpen = message
			}
		}
		if zeOpen != nil && zeOpen.First.Sub(connection.Handshake)+timestampTolerance < zeDelay && (frrOpen == nil || zeOpen.First.Before(frrOpen.Complete)) {
			return "", fmt.Errorf("premature Ze OPEN on %+v: handshake=%s Ze=%s FRR=%v delay=%s", connection.Flow, connection.Handshake, zeOpen.First, frrOpen, zeDelay)
		}
		if frrOpen != nil && frrOpen.First.Sub(connection.Handshake)+timestampTolerance < frrDelay && (zeOpen == nil || frrOpen.First.Before(zeOpen.Complete)) {
			return "", fmt.Errorf("premature FRR OPEN on %+v: handshake=%s FRR=%s delay=%s", connection.Flow, connection.Handshake, frrOpen.First, frrDelay)
		}
		for _, other := range history {
			if other == connection || other.Flow.TargetAddr != peer || other.Handshake.IsZero() || other.Initiator == connection.Initiator {
				continue
			}
			if other.Handshake.After(connection.Handshake) && (connection.Closed.IsZero() || connection.Closed.After(other.Handshake)) {
				overlaps++
			}
		}
		if !connection.Closed.IsZero() {
			continue
		}
		if zeOpen == nil || frrOpen == nil {
			return "", fmt.Errorf("additional or incomplete live DelayOpen connection lacks OPEN exchange: %s", connection.Flow)
		}
		if len(connection.Ze) < 2 || len(connection.FRR) < 2 || connection.Ze[0].Type != bgpOpen || connection.Ze[1].Type != bgpKeepalive || connection.FRR[0].Type != bgpOpen || connection.FRR[1].Type != bgpKeepalive {
			return "", errors.New("native winner lacks OPEN then KEEPALIVE in both directions")
		}
		if early {
			if !frrOpen.Complete.Before(zeOpen.First) || zeOpen.First.Sub(connection.Handshake) >= zeDelay-timestampTolerance {
				return "", errors.New("early OPEN did not release Ze before its DelayOpen expiry")
			}
		} else if zeOpen.First.Before(connection.Handshake.Add(zeDelay-timestampTolerance)) || !zeOpen.Complete.Before(frrOpen.First) {
			return "", errors.New("both-delayed winner does not show Ze timer expiry before FRR OPEN")
		}
		winners++
	}
	if winners != 1 {
		return "", fmt.Errorf("DelayOpen peer %s has %d complete live wire winners, want one", peer, winners)
	}
	return fmt.Sprintf("peer=%s completed TCP connections=%d live OPEN/KEEPALIVE winner=1 opposite-initiator overlaps=%d; overlap is observed native collision opportunity, not proof of an internal arbitration order", peer, completed, overlaps), nil
}

func delayOpenCaptureComplete(log string) (int, error) {
	captured, dropped := -1, -1
	for line := range strings.SplitSeq(log, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		value, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if strings.Join(fields[1:], " ") == "packets captured" {
			captured = value
		}
		if strings.Join(fields[1:], " ") == "packets dropped by kernel" {
			dropped = value
		}
	}
	if captured <= 0 || captured >= delayOpenPacketLimit || dropped != 0 {
		return captured, fmt.Errorf("incomplete DelayOpen capture: captured=%d dropped=%d; capture log: %s", captured, dropped, log)
	}
	return captured, nil
}

// delayOpenFirstArrivals inserts only previously uncovered intervals. Input is
// capture-time ordered, so an overlap retains its earlier timestamp regardless
// of how a later retransmission segments the same bytes. There are at most two
// new boundaries per input range, hence at most 2*delayOpenPacketLimit entries.
func delayOpenFirstArrivals(arrivals []delayOpenArrival) []delayOpenArrival {
	first := make([]delayOpenArrival, 0, 2*len(arrivals))
	for _, arrival := range arrivals {
		start := arrival.start
		index := sort.Search(len(first), func(index int) bool { return first[index].end > start })
		for start < arrival.end {
			if index == len(first) || first[index].start >= arrival.end {
				first = slices.Insert(first, index, delayOpenArrival{start: start, end: arrival.end, at: arrival.at})
				break
			}
			if start < first[index].start {
				first = slices.Insert(first, index, delayOpenArrival{start: start, end: first[index].start, at: arrival.at})
				index++
			}
			start = max(start, first[index].end)
			index++
		}
	}
	return first
}

func delayOpenArrivalTimes(arrivals []delayOpenArrival, start, end int) (time.Time, time.Time, error) {
	index := sort.Search(len(arrivals), func(index int) bool { return arrivals[index].end > start })
	if index == len(arrivals) {
		return time.Time{}, time.Time{}, errors.New("BGP message has no captured TCP byte arrivals")
	}
	first, complete := arrivals[index].at, arrivals[index].at
	for start < end && index < len(arrivals) {
		arrival := arrivals[index]
		if arrival.start > start {
			return time.Time{}, time.Time{}, errors.New("BGP message spans missing TCP byte arrivals")
		}
		if arrival.at.Before(first) {
			first = arrival.at
		}
		if arrival.at.After(complete) {
			complete = arrival.at
		}
		start = arrival.end
		index++
	}
	if start < end {
		return time.Time{}, time.Time{}, errors.New("BGP message ends beyond captured TCP byte arrivals")
	}
	return first, complete, nil
}
