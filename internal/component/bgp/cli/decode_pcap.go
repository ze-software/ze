// Design: docs/architecture/diagnostics/packet-capture.md — reading a capture back
// Overview: decode_answer.go — answerDecode, which routes an input form here
// Related: internal/core/pcap — the reader and the TCP reassembly this drives

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/textbuf"
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
// A capture with no BGP in it is refused with a line naming what was
// examined, because an operator reads empty output as "there is no BGP here"
// and that is the one answer this must never give silently
// (`ai/rules/evidence.md`).
func decodePcapInput(args []string, request *decodeRequest, route command.Route) command.LocalAnswer {
	if len(args) == 0 {
		var tb textbuf.Buffer
		tb.Str("error: ").Str(inputKeywordPcap).Str(" needs a capture path, or ").
			Quoted(cliio.StdinToken).Str(" for standard input\n")
		return command.LocalRefusal(1, tb.String())
	}
	if len(args) > 1 {
		var tb textbuf.Buffer
		tb.Str("error: ").Str(inputKeywordPcap).Str(" takes one input, and ").Str(countWord(len(args))).
			Str(" given: ").Str(strings.Join(quoteAll(args), " and ")).Byte('\n')
		return command.LocalRefusal(1, tb.String())
	}
	if cliio.IsStdin(args[0]) && !route.CallerLocal() {
		return stdinRefusal(pathDecode + " " + inputKeywordPcap + " " + cliio.StdinToken)
	}
	// On the daemon route a capture is read only from where the daemon reads
	// configuration (command.CallerPath).
	readPath, err := command.CallerPath(route, pathDecode, []string{inputKeywordPcap, args[0]}, args[0])
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}

	source, err := cliio.OpenReader(readPath)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: open ").Str(args[0]).Str(": ").Err(err).Byte('\n')
		return command.LocalRefusal(1, tb.String())
	}
	defer func() { _ = source.Close() }()

	return decodePcapStream(source, args[0], request)
}

// decodePcapStream reassembles a capture and decodes every BGP message in it,
// in the order the messages completed on the wire. A message that does not
// decode stays in the answer with its reason, and the reason is also a
// Diagnostic line, which is where the operator has always read it.
func decodePcapStream(source io.Reader, name string, request *decodeRequest) command.LocalAnswer {
	streams, report, err := pcap.Reassemble(source, bgpPort)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: read ").Str(name).Str(": ").Err(err).Byte('\n')
		return command.LocalRefusal(1, tb.String())
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

	var diag textbuf.Buffer
	writeAnomalies(&diag, report, skipped, incomplete)

	if len(messages) == 0 {
		diag.Str("error: no BGP message found in ").Str(name).Str(": ").Str(report.String()).Byte('\n')
		return command.LocalRefusal(1, diag.String())
	}

	decoded := make([]decodedMessage, 0, len(messages))
	failures := 0
	for i := range messages {
		m := &messages[i]
		entry := decodedMessage{
			Time: m.timestamp.Format(time.RFC3339Nano),
			Flow: m.flow.String(),
			Hex:  hexOf(m.bytes),
		}
		event, err := decodeHexPacket(request.plugins, entry.Hex, request.msgType, request.family, true)
		if err != nil {
			diag.Str("error: decode the message at ").Str(entry.Time).Str(" from ").Str(entry.Flow).
				Str(": ").Err(err).Byte('\n')
			entry.Error = err.Error()
			decoded = append(decoded, entry)
			failures++
			continue
		}
		entry.Decoded = json.RawMessage(event)
		decoded = append(decoded, entry)
	}
	code := 0
	if failures > 0 {
		code = 1
	}
	return request.answer(inputFormPcap, decoded, code, diag.String())
}

// writeAnomalies writes into diag everything the reassembly could not account
// for. It is written whether or not messages came out, because a partial
// answer that looks whole is the failure this reporting exists to prevent.
func writeAnomalies(diag *textbuf.Buffer, report *pcap.Report, skipped, incomplete int) {
	for _, gap := range report.Gaps {
		diag.Str("warning: ").Str(gap.Flow.String()).Str(" is missing ").Int(int64(gap.Bytes)).
			Str(" bytes after ").Int(int64(gap.AfterOffset)).
			Str(" reassembled bytes; the stream is not read across the hole\n")
	}
	for _, flow := range report.Truncated {
		diag.Str("warning: ").Str(flow.String()).Str(" reached the ").Int(int64(pcap.FlowBytesMax)).
			Str("-byte reassembly limit; its later bytes were not read\n")
	}
	if report.RecordsDropped > 0 {
		diag.Str("warning: ").Int(int64(report.RecordsDropped)).Str(" records past the ").
			Int(int64(pcap.FlowMax)).Str("-flow limit were not read\n")
	}
	if skipped > 0 {
		diag.Str("warning: ").Int(int64(skipped)).
			Str(" bytes carried no BGP header and were skipped to find the next message\n")
	}
	if incomplete > 0 {
		diag.Str("warning: ").Int(int64(incomplete)).
			Str(" bytes at the end of a stream are a message the capture does not hold whole\n")
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
// each non-empty line, which is the form exabgp's decode accepts. The caller
// MUST have established that the route reads standard input.
//
// A line that fails decodes nothing and the walk continues, because an operator
// who pasted twenty lines wants the nineteen that worked. The exit code is
// non-zero when any line failed.
func decodeHexStdin(request *decodeRequest) command.LocalAnswer {
	data, err := cliio.ReadFile(cliio.StdinToken)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: read standard input: ").Err(err).Byte('\n')
		return command.LocalRefusal(1, tb.String())
	}

	lines := strings.Split(string(data), "\n")
	var diag textbuf.Buffer
	messages := make([]decodedMessage, 0, len(lines))
	decoded, failures := 0, 0
	for i, line := range lines {
		payload := strings.TrimSpace(line)
		if payload == "" {
			continue
		}
		if strings.HasPrefix(payload, "#") {
			continue
		}
		event, err := decodeHexPacket(request.plugins, payload, request.msgType, request.family, true)
		if err != nil {
			diag.Str("error: line ").Int(int64(i + 1)).Str(": ").Err(err).Byte('\n')
			messages = append(messages, decodedMessage{Hex: payload, Error: err.Error()})
			failures++
			continue
		}
		messages = append(messages, decodedMessage{Hex: payload, Decoded: json.RawMessage(event)})
		decoded++
	}

	if decoded == 0 && failures == 0 {
		return command.LocalRefusal(1, "error: standard input carried no hexadecimal message\n")
	}
	code := 0
	if failures > 0 {
		code = 1
	}
	return request.answer(inputFormStdin, messages, code, diag.String())
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
