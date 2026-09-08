// Design: docs/architecture/diagnostics/packet-capture.md — reading a capture back
// Overview: decode.go — cmdDecode, which routes an input form here
// Related: internal/core/pcap — the reader and the TCP reassembly this drives

package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/pcap"
)

// inputKeywordPcap precedes a capture path, so args[0] is always a keyword and
// never a user identifier (`ai/rules/cli.md`). `ze bgp decode <hex>` keeps its
// bare argument, which no path can be confused with: a path is not hexadecimal.
const inputKeywordPcap = "pcap"

// bgpPort is the TCP port a BGP session uses (RFC 4271 Section 3). A capture is
// searched for flows with it at either end. A session on another port needs the
// port supplied, which this command does not offer.
const bgpPort = 179

// framedMessage is one whole BGP message lifted out of a reassembled stream.
type framedMessage struct {
	timestamp time.Time
	flow      pcap.Flow
	bytes     []byte
}

// decodePcapInput decodes every BGP message in a capture. args holds the
// positional words after the `pcap` keyword, which must be exactly one path or
// the stdin token.
//
// It returns the process exit code. A capture with no BGP in it exits non-zero
// with a line naming what was examined, because an operator reads empty output
// as "there is no BGP here" and that is the one answer this must never give
// silently (`ai/rules/evidence.md`).
func decodePcapInput(args []string, msgType, family string, outputJSON bool) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "error: %s needs a capture path, or %q for standard input\n", inputKeywordPcap, cliio.StdinToken)
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "error: %s takes one input, and %s given: %s\n",
			inputKeywordPcap, countWord(len(args)), strings.Join(quoteAll(args), " and "))
		return 1
	}

	source, err := cliio.OpenReader(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open %s: %v\n", args[0], err)
		return 1
	}
	defer func() { _ = source.Close() }()

	return decodePcapStream(source, args[0], msgType, family, outputJSON)
}

// decodePcapStream reassembles a capture and decodes every BGP message in it,
// in the order the messages completed on the wire.
func decodePcapStream(source io.Reader, name, msgType, family string, outputJSON bool) int {
	streams, report, err := pcap.Reassemble(source, bgpPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", name, err)
		return 1
	}

	var messages []framedMessage
	var skipped, incomplete int
	for i := range streams {
		framed, resynced, tail := frameMessages(&streams[i])
		messages = append(messages, framed...)
		skipped += resynced
		incomplete += tail
	}

	// Messages from the two directions interleave, so the file order is not the
	// wire order. Each message is timed by the segment that completed it, which
	// is the moment the receiver could read it.
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].timestamp.Before(messages[j].timestamp) })

	reportAnomalies(report, skipped, incomplete)

	if len(messages) == 0 {
		fmt.Fprintf(os.Stderr, "error: no BGP message found in %s: %s\n", name, report)
		return 1
	}

	failures := 0
	for i := range messages {
		m := &messages[i]
		if !outputJSON {
			fmt.Printf("# %s  %s\n", m.timestamp.Format(time.RFC3339Nano), m.flow)
		}
		output, err := decodeHexPacket(hexOf(m.bytes), msgType, family, outputJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: decode the message at %s from %s: %v\n",
				m.timestamp.Format(time.RFC3339Nano), m.flow, err)
			failures++
			continue
		}
		fmt.Println(output)
	}
	if failures > 0 {
		return 1
	}
	return 0
}

// reportAnomalies writes to standard error everything the reassembly could not
// account for. It is written whether or not messages came out, because a
// partial answer that looks whole is the failure this reporting exists to
// prevent.
func reportAnomalies(report *pcap.Report, skipped, incomplete int) {
	for _, gap := range report.Gaps {
		fmt.Fprintf(os.Stderr, "warning: %s is missing %d bytes after %d reassembled bytes; the stream is not read across the hole\n",
			gap.Flow, gap.Bytes, gap.AfterOffset)
	}
	for _, flow := range report.Truncated {
		fmt.Fprintf(os.Stderr, "warning: %s reached the %d-byte reassembly limit; its later bytes were not read\n", flow, pcap.FlowBytesMax)
	}
	if report.RecordsDropped > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d records past the %d-flow limit were not read\n", report.RecordsDropped, pcap.FlowMax)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d bytes carried no BGP header and were skipped to find the next message\n", skipped)
	}
	if incomplete > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d bytes at the end of a stream are a message the capture does not hold whole\n", incomplete)
	}
}

// frameMessages cuts whole BGP messages out of one reassembled stream.
//
// A capture started mid-session begins part-way through a message, so the walk
// resynchronizes on the all-ones marker RFC 4271 Section 4.1 requires rather
// than assuming a message starts at offset zero. It returns the messages, the
// bytes skipped to resynchronize, and the bytes of a trailing message the
// capture does not hold whole.
func frameMessages(stream *pcap.Stream) (messages []framedMessage, resynced, tail int) {
	data := stream.Bytes
	offset := 0
	for offset < len(data) {
		start := resynchronize(data, offset)
		if start < 0 {
			resynced += len(data) - offset
			return messages, resynced, tail
		}
		resynced += start - offset

		header, err := message.ParseHeader(data[start:])
		if err != nil {
			// Fewer than 19 bytes left, or a length below the header. The
			// marker was there, so this is the tail of a truncated capture
			// rather than something to resynchronize past.
			return messages, resynced, tail + len(data) - start
		}
		if start+int(header.Length) > len(data) {
			return messages, resynced, tail + len(data) - start
		}

		messages = append(messages, framedMessage{
			// The message is timed by the segment carrying its last byte,
			// which is when a receiver could read it whole.
			timestamp: stream.TimestampAt(start + int(header.Length) - 1),
			flow:      stream.Flow,
			bytes:     data[start : start+int(header.Length)],
		})
		offset = start + int(header.Length)
	}
	return messages, resynced, tail
}

// resynchronize returns the offset of the next BGP marker at or after start, or
// -1 when the stream holds no further marker. RFC 4271 Section 4.1 fixes the
// marker at 16 octets of all ones, which is what makes a mid-stream capture
// framable at all.
func resynchronize(data []byte, start int) int {
	for offset := start; offset+message.MarkerLen <= len(data); offset++ {
		if isMarker(data[offset:]) {
			return offset
		}
	}
	return -1
}

// isMarker reports whether data opens with the 16-octet all-ones BGP marker.
func isMarker(data []byte) bool {
	for i := range message.MarkerLen {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}

// hexOf renders bytes as the lowercase hexadecimal string decodeHexPacket
// takes, which keeps that function's signature and its single decode path.
func hexOf(data []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(data)*2)
	for _, b := range data {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

// decodeHexStdin decodes hexadecimal messages read from standard input, one for
// each non-empty line, which is the form exabgp's decode accepts.
//
// A line that fails decodes nothing and the walk continues, because an operator
// who pasted twenty lines wants the nineteen that worked. The exit code is
// non-zero when any line failed.
func decodeHexStdin(msgType, family string, outputJSON bool) int {
	data, err := cliio.ReadFile(cliio.StdinToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read standard input: %v\n", err)
		return 1
	}

	lines := strings.Split(string(data), "\n")
	decoded, failures := 0, 0
	for i, line := range lines {
		payload := strings.TrimSpace(line)
		if payload == "" || strings.HasPrefix(payload, "#") {
			continue
		}
		output, err := decodeHexPacket(payload, msgType, family, outputJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: line %d: %v\n", i+1, err)
			failures++
			continue
		}
		fmt.Println(output)
		decoded++
	}

	if decoded == 0 && failures == 0 {
		fmt.Fprintf(os.Stderr, "error: standard input carried no hexadecimal message\n")
		return 1
	}
	if failures > 0 {
		return 1
	}
	return 0
}

// countWord spells a small count for an error message, so "1 were given" never
// reaches a reader.
func countWord(n int) string {
	if n == 1 {
		return "1 was"
	}
	return fmt.Sprintf("%d were", n)
}

// quoteAll quotes each word, so an error naming two inputs shows where each one
// begins and ends.
func quoteAll(words []string) []string {
	out := make([]string, len(words))
	for i, word := range words {
		out[i] = fmt.Sprintf("%q", word)
	}
	return out
}
