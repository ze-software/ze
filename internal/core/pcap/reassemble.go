// Design: docs/architecture/diagnostics/packet-capture.md -- TCP stream reassembly
// Detail: parse.go -- turning one record into one TCP segment
// Overview: reader.go -- the records this consumes

package pcap

import (
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"time"
)

// The bounds a crafted capture must not be able to exceed. A pcap handed to Ze
// is untrusted input, so the flow count and the bytes held for each flow are
// both capped, and a capture that reaches either is REPORTED rather than
// silently shortened.
const (
	// FlowMax is the number of TCP directions one reassembly holds. A BGP
	// session uses two, so this covers 128 sessions in one capture.
	FlowMax = 256
	// FlowBytesMax is the reassembled bytes one direction holds. It is 64
	// full-size extended BGP messages.
	FlowBytesMax = 4 << 20
)

// Stream is one contiguous run of bytes in one direction of one TCP
// conversation. A gap in the capture ends a run and starts the next one, so a
// caller never reads across bytes that were never seen.
type Stream struct {
	Flow  Flow
	Bytes []byte
	// marks records the capture timestamp of each contributing segment, at the
	// offset into Bytes where that segment starts. TimestampAt reads it.
	marks []mark
}

// mark is the capture timestamp of the segment that begins at offset.
type mark struct {
	offset    int
	timestamp time.Time
}

// TimestampAt returns the capture timestamp of the segment that carried the
// byte at offset. An offset past the end returns the last segment's timestamp,
// because that is the segment the caller was reading when it ran out.
func (s *Stream) TimestampAt(offset int) time.Time {
	if len(s.marks) == 0 {
		return time.Time{}
	}
	// The marks are sorted by offset, so the segment covering offset is the
	// last one that starts at or before it.
	index := sort.Search(len(s.marks), func(i int) bool { return s.marks[i].offset > offset })
	if index == 0 {
		return s.marks[0].timestamp
	}
	return s.marks[index-1].timestamp
}

// Gap names bytes a capture never held: a hole between two segments of one
// direction. A gap is reported rather than closed, because concatenating across
// one would present bytes that were never adjacent as though they were.
type Gap struct {
	Flow Flow
	// AfterOffset is how many bytes of the run were reassembled before the
	// hole.
	AfterOffset int
	// Bytes is how many bytes are missing.
	Bytes int
}

// Report says what a reassembly saw. Every count is filled even when no stream
// came out, so a caller can tell "this capture holds no TCP on that port" from
// "this file could not be read", and neither is reported as an empty success.
type Report struct {
	RecordsRead int
	// RecordsSkipped counts records the dissection refused: a link type with
	// no dissector, a frame that is not IP, a packet that is not TCP, a
	// fragment, or a header running past the captured bytes.
	RecordsSkipped int
	// FlowsSeen counts the TCP directions in the file, on any port.
	FlowsSeen int
	// FlowsSelected counts the directions whose source or target port matched
	// the selector.
	FlowsSelected int
	// FlowsDropped counts directions refused because FlowMax was already
	// reached.
	FlowsDropped int
	// BytesReassembled is the total length of every Stream returned.
	BytesReassembled int
	Gaps             []Gap
	// Truncated names the directions that reached FlowBytesMax. Their later
	// bytes were not held.
	Truncated []Flow
}

// String renders the report as one line for an operator who got no messages
// out, so the answer names what was examined instead of printing nothing.
func (r *Report) String() string {
	return fmt.Sprintf(
		"%d records read, %d skipped, %d TCP flows seen, %d selected, %d dropped past the flow limit, %d bytes reassembled, %d gaps",
		r.RecordsRead, r.RecordsSkipped, r.FlowsSeen, r.FlowsSelected, r.FlowsDropped, r.BytesReassembled, len(r.Gaps))
}

// heldSegment is one segment kept for reassembly, with its bytes copied out of
// the reader's buffer.
type heldSegment struct {
	// offset is the segment's position in the direction's byte stream,
	// relative to the flow's base sequence. It is computed as a signed
	// difference so a sequence that wrapped past 2^32 still orders correctly.
	offset    int
	data      []byte
	timestamp time.Time
}

// flowState is everything one direction accumulates while records are read.
type flowState struct {
	segments []heldSegment
	// base is the sequence number offset zero corresponds to. The SYN sets it
	// exactly; without one it is the first sequence seen, which a later
	// out-of-order segment can move backwards.
	base      uint32
	baseKnown bool
	// synchronized records that a SYN fixed the base, so no later segment
	// moves it.
	synchronized bool
	bytesHeld    int
	truncated    bool
}

// Reassemble reads every record from source, keeps the TCP directions whose
// source or target port is port, and returns their byte streams in reassembly
// order together with a report of what was seen.
//
// port is a parameter rather than a constant because this package knows TCP and
// no application protocol; the caller that wants BGP passes 179.
//
// Out-of-order segments are placed by sequence number, a retransmission that
// repeats bytes already held is dropped, and a hole ends the run and opens a
// Gap. Both bounds above are enforced, and reaching either is recorded in the
// report rather than growing the memory.
func Reassemble(source io.Reader, port uint16) ([]Stream, *Report, error) {
	reader, err := NewReader(source)
	if err != nil {
		return nil, nil, err
	}

	report := &Report{}
	flows := make(map[Flow]*flowState)
	// order keeps the flows in the order they first appeared, so two runs over
	// one file return their streams in the same order.
	var order []Flow
	seen := make(map[Flow]struct{})

	var record Record
	for {
		err := reader.Next(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, report, err
		}
		report.RecordsRead++

		seg, err := dissect(reader.LinkType(), record.Data)
		if err != nil {
			report.RecordsSkipped++
			continue
		}
		if _, already := seen[seg.flow]; !already {
			seen[seg.flow] = struct{}{}
			report.FlowsSeen++
		}
		if seg.flow.SourcePort != port && seg.flow.TargetPort != port {
			continue
		}

		state := flows[seg.flow]
		if state == nil {
			if len(flows) == FlowMax {
				report.FlowsDropped++
				continue
			}
			state = &flowState{}
			flows[seg.flow] = state
			order = append(order, seg.flow)
			report.FlowsSelected++
		}
		state.add(seg, record.Timestamp)
	}

	streams := make([]Stream, 0, len(order))
	for _, flow := range order {
		state := flows[flow]
		if state.truncated {
			report.Truncated = append(report.Truncated, flow)
		}
		streams = append(streams, state.build(flow, report)...)
	}
	for i := range streams {
		report.BytesReassembled += len(streams[i].Bytes)
	}
	return streams, report, nil
}

// add records one segment against a direction, copying its bytes out of the
// reader's buffer and placing it relative to the direction's base sequence.
func (f *flowState) add(seg segment, timestamp time.Time) {
	// RFC 9293 Section 3.4: the SYN occupies one sequence number, so the first
	// data byte sits at the initial sequence number plus one. A SYN fixes the
	// base exactly, which is why it overrides a base guessed from data.
	if seg.synchronize {
		f.rebase(seg.sequence + 1)
		f.synchronized = true
	}
	if len(seg.data) == 0 {
		return
	}
	if !f.baseKnown {
		f.base, f.baseKnown = seg.sequence, true
	}

	if f.bytesHeld+len(seg.data) > FlowBytesMax {
		f.truncated = true
		return
	}

	data := make([]byte, len(seg.data))
	copy(data, seg.data)
	f.bytesHeld += len(data)
	f.segments = append(f.segments, heldSegment{
		offset:    int(int32(seg.sequence - f.base)), //nolint:gosec // the signed difference is what carries a wrapped sequence
		data:      data,
		timestamp: timestamp,
	})
}

// rebase moves the direction's zero point to sequence, shifting every segment
// already held. A SYN arriving after data does this.
func (f *flowState) rebase(sequence uint32) {
	if f.synchronized {
		return
	}
	if !f.baseKnown {
		f.base, f.baseKnown = sequence, true
		return
	}
	shift := int(int32(f.base - sequence)) //nolint:gosec // the signed difference is what carries a wrapped sequence
	if shift == 0 {
		return
	}
	f.base = sequence
	for i := range f.segments {
		f.segments[i].offset += shift
	}
}

// build turns a direction's held segments into contiguous streams, appending a
// Gap to the report for every hole between them.
func (f *flowState) build(flow Flow, report *Report) []Stream {
	if len(f.segments) == 0 {
		return nil
	}

	// A segment before the base means the capture held an earlier sequence
	// than the first one seen. Shift everything so no offset is negative.
	lowest := f.segments[0].offset
	for i := range f.segments {
		lowest = min(lowest, f.segments[i].offset)
	}
	if lowest < 0 {
		for i := range f.segments {
			f.segments[i].offset -= lowest
		}
	}

	sort.SliceStable(f.segments, func(i, j int) bool { return f.segments[i].offset < f.segments[j].offset })

	var streams []Stream
	current := Stream{Flow: flow}
	// start is the offset the current run begins at, so a Gap can report how
	// many bytes of that run preceded the hole.
	start := f.segments[0].offset
	end := start

	for i := range f.segments {
		seg := &f.segments[i]
		segEnd := seg.offset + len(seg.data)

		// A retransmission whose bytes are all held already adds nothing.
		if segEnd <= end {
			continue
		}
		if seg.offset > end {
			if len(current.Bytes) > 0 {
				streams = append(streams, current)
				report.Gaps = append(report.Gaps, Gap{
					Flow:        flow,
					AfterOffset: end - start,
					Bytes:       seg.offset - end,
				})
			}
			current = Stream{Flow: flow}
			start, end = seg.offset, seg.offset
		}

		// An overlapping retransmission contributes only the bytes past what
		// is already held.
		data := seg.data
		if seg.offset < end {
			data = data[end-seg.offset:]
		}
		current.marks = append(current.marks, mark{offset: len(current.Bytes), timestamp: seg.timestamp})
		current.Bytes = append(current.Bytes, data...)
		end = segEnd
	}

	if len(current.Bytes) > 0 {
		streams = append(streams, current)
	}
	return streams
}

// FlowFrom builds a Flow from two address-and-port pairs, for a caller that
// holds them as values rather than off a wire.
func FlowFrom(source, target netip.AddrPort) Flow {
	return Flow{
		SourceAddr: source.Addr(),
		TargetAddr: target.Addr(),
		SourcePort: source.Port(),
		TargetPort: target.Port(),
	}
}
