// Goal: prove an operator gets every BGP message out of a capture -- ze's own
// or a colleague's tcpdump -- in wire order and with its direction, and that a
// capture holding no BGP says so instead of printing nothing.
// Method: build captures with internal/core/pcap's own writer, which makes the
// round trip (ze writes, ze reads) the test rather than an assumption.

package cli

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/pcap"
)

// The two ends of the session every test capture describes. They carry
// different addresses, so no assertion about direction can pass by confusing
// one end with the other.
var (
	testPeer  = netip.MustParseAddr("192.0.2.1")
	testLocal = netip.MustParseAddr("192.0.2.2")
)

var testStamp = time.Unix(1_700_000_000, 0).UTC()

// receivedFlow runs from the peer to this host, and sentFlow the other way.
func receivedFlow() pcap.Flow {
	return pcap.Flow{SourceAddr: testPeer, TargetAddr: testLocal, SourcePort: bgpPort, TargetPort: bgpPort}
}

func sentFlow() pcap.Flow { return receivedFlow().Reverse() }

// keepalive is the 19 octets of a BGP KEEPALIVE (RFC 4271 Section 4.4).
func keepalive() []byte {
	msg := bytes.Repeat([]byte{0xFF}, 16)
	return append(msg, 0x00, 0x13, 0x04)
}

// openMessage is the canonical OPEN: version 4, ASN 65000, hold time 90,
// identifier 1.2.3.4, no optional parameters.
func openMessage() []byte {
	msg, err := hex.DecodeString("ffffffffffffffffffffffffffffffff001d0104fde8005a0102030400")
	if err != nil {
		panic("BUG: the canonical OPEN fixture is not hexadecimal")
	}
	return msg
}

// capturePart is one payload to write into a test capture. Several parts in one
// flow become consecutive TCP segments, so a message can be split across them
// and several messages can share one.
type capturePart struct {
	flow    pcap.Flow
	payload []byte
	offset  time.Duration
}

// buildCapture writes the parts as a LINKTYPE_RAW pcap, in the order given.
func buildCapture(t *testing.T, parts []capturePart) []byte {
	t.Helper()

	var file bytes.Buffer
	if err := pcap.WriteFileHeader(&file, 65535, pcap.LinkTypeRaw); err != nil {
		t.Fatalf("WriteFileHeader: %v", err)
	}
	framer := pcap.NewFramer()
	for i, part := range parts {
		if err := framer.WriteMessage(&file, testStamp.Add(part.offset), part.flow, part.payload, len(part.payload)); err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
	}
	return file.Bytes()
}

// run captures what a decode writes to the two output streams, so a test can
// assert on what an operator reads.
func run(t *testing.T, fn func() int) (stdout, stderr string, code int) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	code = fn()

	os.Stdout, os.Stderr = savedOut, savedErr
	_ = outW.Close()
	_ = errW.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, outR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, errR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return outBuf.String(), errBuf.String(), code
}

// writeCapture puts a capture on disk and returns its path.
func writeCapture(t *testing.T, file []byte) string {
	t.Helper()
	path := t.TempDir() + "/capture.pcap"
	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatalf("write capture: %v", err)
	}
	return path
}

func TestReadPcapFramesMessages(t *testing.T) {
	// A session opening: the peer's OPEN, this host's OPEN, then a KEEPALIVE
	// each way. The file order interleaves the two directions, which is what
	// the wire-order sort has to get right.
	file := buildCapture(t, []capturePart{
		{flow: receivedFlow(), payload: openMessage(), offset: 0},
		{flow: sentFlow(), payload: openMessage(), offset: 10 * time.Millisecond},
		{flow: receivedFlow(), payload: keepalive(), offset: 20 * time.Millisecond},
		{flow: sentFlow(), payload: keepalive(), offset: 30 * time.Millisecond},
	})
	path := writeCapture(t, file)

	stdout, stderr, code := run(t, func() int {
		return decodePcapInput([]string{path}, "", "", false)
	})
	if code != 0 {
		t.Fatalf("exit code %d, want 0. stderr: %s", code, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var headers []string
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			headers = append(headers, line)
		}
	}
	if len(headers) != 4 {
		t.Fatalf("got %d messages, want 4:\n%s", len(headers), stdout)
	}

	// Wire order: received, sent, received, sent, by the timestamps written.
	wantSource := []netip.Addr{testPeer, testLocal, testPeer, testLocal}
	for i, header := range headers {
		if !strings.Contains(header, wantSource[i].String()+":") {
			t.Errorf("message %d header %q does not name %s as its source", i, header, wantSource[i])
		}
	}
	if !strings.Contains(stdout, "KEEPALIVE") {
		t.Errorf("no KEEPALIVE decoded:\n%s", stdout)
	}
	if strings.Count(stdout, "KEEPALIVE") != 2 {
		t.Errorf("decoded %d KEEPALIVEs, want 2:\n%s", strings.Count(stdout, "KEEPALIVE"), stdout)
	}
}

// TestFrameMessagesAcrossSegments covers AC-9: one message split over three TCP
// segments is decoded once, not three times and not dropped.
func TestFrameMessagesAcrossSegments(t *testing.T) {
	msg := openMessage()
	file := buildCapture(t, []capturePart{
		{flow: receivedFlow(), payload: msg[0:5]},
		{flow: receivedFlow(), payload: msg[5:12], offset: time.Millisecond},
		{flow: receivedFlow(), payload: msg[12:], offset: 2 * time.Millisecond},
	})

	streams, report, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1: %s", len(streams), report)
	}

	messages, resynced, tail := frameMessages(&streams[0])
	if len(messages) != 1 {
		t.Fatalf("framed %d messages, want 1", len(messages))
	}
	if !bytes.Equal(messages[0].bytes, msg) {
		t.Errorf("message = % x, want % x", messages[0].bytes, msg)
	}
	if resynced != 0 || tail != 0 {
		t.Errorf("resynced %d bytes and left %d trailing, want none of either", resynced, tail)
	}
	// The message is timed by the segment that completed it, which is the
	// third one, not the first.
	if want := testStamp.Add(2 * time.Millisecond); !messages[0].timestamp.Equal(want) {
		t.Errorf("timestamp = %s, want the completing segment's %s", messages[0].timestamp, want)
	}
}

// TestFrameThreeMessagesInOneSegment covers AC-10.
func TestFrameThreeMessagesInOneSegment(t *testing.T) {
	together := bytes.Join([][]byte{openMessage(), keepalive(), keepalive()}, nil)
	file := buildCapture(t, []capturePart{{flow: receivedFlow(), payload: together}})

	streams, _, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	messages, resynced, tail := frameMessages(&streams[0])
	if len(messages) != 3 {
		t.Fatalf("framed %d messages, want 3", len(messages))
	}
	if !bytes.Equal(messages[0].bytes, openMessage()) {
		t.Errorf("first message = % x, want the OPEN", messages[0].bytes)
	}
	if !bytes.Equal(messages[1].bytes, keepalive()) || !bytes.Equal(messages[2].bytes, keepalive()) {
		t.Errorf("messages 2 and 3 are not the two KEEPALIVEs")
	}
	if resynced != 0 || tail != 0 {
		t.Errorf("resynced %d bytes and left %d trailing, want none of either", resynced, tail)
	}
}

// TestFrameResyncsMidStream covers A-5: a capture started part-way through a
// message has no marker at offset zero, so framing resynchronizes on the
// all-ones marker rather than giving up or reading garbage.
func TestFrameResyncsMidStream(t *testing.T) {
	// The tail of a message the capture missed the start of, then a whole one.
	partial := openMessage()[7:]
	file := buildCapture(t, []capturePart{
		{flow: receivedFlow(), payload: partial},
		{flow: receivedFlow(), payload: keepalive(), offset: time.Millisecond},
	})

	streams, _, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	messages, resynced, _ := frameMessages(&streams[0])
	if len(messages) != 1 {
		t.Fatalf("framed %d messages, want 1: only the whole one is framable", len(messages))
	}
	if !bytes.Equal(messages[0].bytes, keepalive()) {
		t.Errorf("message = % x, want the KEEPALIVE", messages[0].bytes)
	}
	if resynced == 0 {
		t.Error("resynced 0 bytes, but the stream opens part-way through a message")
	}
}

// TestNonBGPFlowsIgnored covers AC-12.
func TestNonBGPFlowsIgnored(t *testing.T) {
	web := pcap.Flow{SourceAddr: testPeer, TargetAddr: testLocal, SourcePort: 443, TargetPort: 40000}
	file := buildCapture(t, []capturePart{
		{flow: web, payload: []byte("GET / HTTP/1.1\r\n\r\n")},
		{flow: receivedFlow(), payload: keepalive(), offset: time.Millisecond},
		{flow: web.Reverse(), payload: []byte("HTTP/1.1 200 OK\r\n\r\n"), offset: 2 * time.Millisecond},
	})
	path := writeCapture(t, file)

	stdout, stderr, code := run(t, func() int {
		return decodePcapInput([]string{path}, "", "", false)
	})
	if code != 0 {
		t.Fatalf("exit code %d, want 0. stderr: %s", code, stderr)
	}
	if strings.Count(stdout, "KEEPALIVE") != 1 {
		t.Errorf("decoded %d KEEPALIVEs, want 1:\n%s", strings.Count(stdout, "KEEPALIVE"), stdout)
	}
	if strings.Contains(stdout, "HTTP") {
		t.Errorf("non-BGP bytes reached the output:\n%s", stdout)
	}
}

// TestNoBGPFoundIsAnError covers AC-14 and R-4: silence is never a successful
// decode, and the answer names what was examined.
func TestNoBGPFoundIsAnError(t *testing.T) {
	web := pcap.Flow{SourceAddr: testPeer, TargetAddr: testLocal, SourcePort: 443, TargetPort: 40000}
	file := buildCapture(t, []capturePart{{flow: web, payload: []byte("nothing to decode")}})
	path := writeCapture(t, file)

	stdout, stderr, code := run(t, func() int {
		return decodePcapInput([]string{path}, "", "", false)
	})
	if code == 0 {
		t.Errorf("exit code 0 for a capture with no BGP in it, want non-zero")
	}
	if stdout != "" {
		t.Errorf("wrote %q to standard output, want nothing", stdout)
	}
	for _, want := range []string{"1 records read", "1 TCP flows seen", "0 selected"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the error does not report %q:\n%s", want, stderr)
		}
	}
}

// TestPathAndStdinRejected covers AC-17: the rejection happens at argument
// parsing and names both inputs, so no internal stdin-claimed error reaches the
// operator.
func TestPathAndStdinRejected(t *testing.T) {
	_, stderr, code := run(t, func() int {
		return decodePcapInput([]string{"session.pcap", "-"}, "", "", false)
	})
	if code == 0 {
		t.Error("exit code 0 for a path and a dash together, want non-zero")
	}
	if !strings.Contains(stderr, `"session.pcap"`) || !strings.Contains(stderr, `"-"`) {
		t.Errorf("the error does not name both inputs:\n%s", stderr)
	}
	if strings.Contains(stderr, "claimed") {
		t.Errorf("the operator got the internal stdin-claimed error:\n%s", stderr)
	}
}

// TestPcapWithNoPathRejected proves the keyword alone is refused with a message
// naming what is missing.
func TestPcapWithNoPathRejected(t *testing.T) {
	_, stderr, code := run(t, func() int {
		return decodePcapInput(nil, "", "", false)
	})
	if code == 0 {
		t.Error("exit code 0 for the pcap keyword with no path, want non-zero")
	}
	if !strings.Contains(stderr, "capture path") {
		t.Errorf("the error does not say a path is missing:\n%s", stderr)
	}
}

// TestDecodePcapStdinMatchesPath covers AC-15: a capture piped in decodes
// identically to the same capture given as a path.
func TestDecodePcapStdinMatchesPath(t *testing.T) {
	file := buildCapture(t, []capturePart{
		{flow: receivedFlow(), payload: openMessage()},
		{flow: sentFlow(), payload: keepalive(), offset: time.Millisecond},
	})
	path := writeCapture(t, file)

	fromPath, _, code := run(t, func() int {
		return decodePcapInput([]string{path}, "", "", false)
	})
	if code != 0 {
		t.Fatalf("path form exited %d, want 0", code)
	}

	restore := cliio.SwapStreams(bytes.NewReader(file), io.Discard)
	defer restore()
	fromStdin, stderr, code := run(t, func() int {
		return decodePcapInput([]string{cliio.StdinToken}, "", "", false)
	})
	if code != 0 {
		t.Fatalf("stdin form exited %d, want 0. stderr: %s", code, stderr)
	}
	if fromStdin != fromPath {
		t.Errorf("stdin output differs from the path output:\nstdin:\n%s\npath:\n%s", fromStdin, fromPath)
	}
}

// TestDecodeHexStdinMultipleLines covers AC-16: several hex messages, one for
// each line, decoded in order, which is what exabgp's decode accepts.
func TestDecodeHexStdinMultipleLines(t *testing.T) {
	input := strings.Join([]string{
		hex.EncodeToString(openMessage()),
		"# a comment line is skipped",
		"",
		hex.EncodeToString(keepalive()),
		hex.EncodeToString(keepalive()),
	}, "\n")

	restore := cliio.SwapStreams(strings.NewReader(input), io.Discard)
	defer restore()
	stdout, stderr, code := run(t, func() int {
		return decodeHexStdin("", "", false)
	})
	if code != 0 {
		t.Fatalf("exit code %d, want 0. stderr: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 3 {
		t.Fatalf("decoded %d lines of output, want at least one for each of the 3 messages:\n%s", len(lines), stdout)
	}
	if strings.Count(stdout, "KEEPALIVE") != 2 {
		t.Errorf("decoded %d KEEPALIVEs, want 2:\n%s", strings.Count(stdout, "KEEPALIVE"), stdout)
	}
	if !strings.Contains(stdout, "65000") {
		t.Errorf("the OPEN's ASN is absent from the output:\n%s", stdout)
	}
}

// TestDecodeHexStdinReportsABadLine proves one unusable line neither stops the
// rest nor passes silently.
func TestDecodeHexStdinReportsABadLine(t *testing.T) {
	input := "zzzz not hex\n" + hex.EncodeToString(keepalive()) + "\n"

	restore := cliio.SwapStreams(strings.NewReader(input), io.Discard)
	defer restore()
	stdout, stderr, code := run(t, func() int {
		return decodeHexStdin("", "", false)
	})
	if code == 0 {
		t.Error("exit code 0 with a line that failed, want non-zero")
	}
	if !strings.Contains(stderr, "line 1") {
		t.Errorf("the error does not name the failing line:\n%s", stderr)
	}
	if !strings.Contains(stdout, "KEEPALIVE") {
		t.Errorf("the good line was not decoded:\n%s", stdout)
	}
}

// TestDecodeHexStdinEmptyIsAnError proves an empty pipe reports rather than
// exiting 0 with no output.
func TestDecodeHexStdinEmptyIsAnError(t *testing.T) {
	restore := cliio.SwapStreams(strings.NewReader("\n\n  \n"), io.Discard)
	defer restore()
	_, stderr, code := run(t, func() int {
		return decodeHexStdin("", "", false)
	})
	if code == 0 {
		t.Error("exit code 0 for an empty input, want non-zero")
	}
	if !strings.Contains(stderr, "no hexadecimal message") {
		t.Errorf("the error does not say what was missing:\n%s", stderr)
	}
}

// TestDecodePcapReportsAGap proves a hole in the capture reaches the operator
// rather than being closed silently (AC-13).
func TestDecodePcapReportsAGap(t *testing.T) {
	// Two records whose sequence numbers leave a hole: the framer advances by
	// the original length, so declaring a longer original than the payload
	// written opens exactly the gap a lost segment leaves.
	var file bytes.Buffer
	if err := pcap.WriteFileHeader(&file, 65535, pcap.LinkTypeRaw); err != nil {
		t.Fatalf("WriteFileHeader: %v", err)
	}
	framer := pcap.NewFramer()
	if err := framer.WriteMessage(&file, testStamp, receivedFlow(), keepalive(), len(keepalive())+40); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if err := framer.WriteMessage(&file, testStamp.Add(time.Millisecond), receivedFlow(), keepalive(), len(keepalive())); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	path := writeCapture(t, file.Bytes())

	stdout, stderr, code := run(t, func() int {
		return decodePcapInput([]string{path}, "", "", false)
	})
	if code != 0 {
		t.Fatalf("exit code %d, want 0: both runs still hold a whole message. stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "missing 40 bytes") {
		t.Errorf("the gap was not reported:\n%s", stderr)
	}
	if strings.Count(stdout, "KEEPALIVE") != 2 {
		t.Errorf("decoded %d KEEPALIVEs, want 2 (one on each side of the hole):\n%s", strings.Count(stdout, "KEEPALIVE"), stdout)
	}
}
