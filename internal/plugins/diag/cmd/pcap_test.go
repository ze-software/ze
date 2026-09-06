// Goal: prove the BGP capture ze writes is a file ze can read back, that its
// framing says what the bytes are, and that the BFD capture is byte-identical
// to what it was before the BGP export gained framing.
// Method: build the entries the reactor produces, export, then reassemble the
// result with internal/core/pcap. The round trip is the test; a writer whose
// output nothing parses is how the defect this fixes stayed invisible.

package cmd

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/pcap"
)

const (
	testPeerAddr  = "192.0.2.1"
	testLocalAddr = "192.0.2.2"
)

var testStamp = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// keepalive is the 19 octets of a BGP KEEPALIVE (RFC 4271 Section 4.4).
func keepalive() []byte {
	msg := bytes.Repeat([]byte{0xFF}, 16)
	return append(msg, 0x00, 0x13, 0x04)
}

// bgpMessage builds a whole message of the given wire length: the all-ones
// marker, the length, the type, then a body of the stated byte.
func bgpMessage(length int, msgType, fill byte) []byte {
	msg := make([]byte, length)
	for i := range 16 {
		msg[i] = 0xFF
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(length)) //nolint:gosec // the tests pass lengths well below 65535
	msg[18] = msgType
	for i := 19; i < length; i++ {
		msg[i] = fill
	}
	return msg
}

// entry builds one capture entry the way the reactor's snapshot does.
func entry(direction, peer, local string, offset time.Duration, data []byte) plugin.BGPRawCaptureEntry {
	return plugin.BGPRawCaptureEntry{
		Timestamp:   testStamp.Add(offset).UTC().Format(captureTimeLayout),
		Direction:   direction,
		PeerAddr:    peer,
		LocalAddr:   local,
		Data:        data,
		OriginalLen: len(data),
	}
}

// TestExportBGPPcapRoundTrip is AC-1 and AC-18 together: what ze captured comes
// back out of the file ze wrote, whole messages and directions included.
func TestExportBGPPcapRoundTrip(t *testing.T) {
	open := bgpMessage(29, 1, 0x11)
	update := bgpMessage(40, 2, 0x22)

	// The reactor's snapshot is newest-first, and the exporter walks it
	// backwards so the file reads oldest-first.
	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirOut, testPeerAddr, testLocalAddr, 30*time.Millisecond, keepalive()),
		entry(captureDirIn, testPeerAddr, testLocalAddr, 20*time.Millisecond, update),
		entry(captureDirOut, testPeerAddr, testLocalAddr, 10*time.Millisecond, open),
		entry(captureDirIn, testPeerAddr, testLocalAddr, 0, open),
	}

	file, err := exportBGPPcap(entries)
	if err != nil {
		t.Fatalf("exportBGPPcap: %v", err)
	}

	reader, err := pcap.NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatalf("the exported file is not a pcap: %v", err)
	}
	if reader.LinkType() != pcap.LinkTypeRaw {
		t.Errorf("link type = %d, want %d", reader.LinkType(), pcap.LinkTypeRaw)
	}

	streams, report, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("got %d streams, want one for each direction: %s", len(streams), report)
	}
	if len(report.Gaps) != 0 {
		t.Errorf("the file ze wrote reads back with %d gaps: %s", len(report.Gaps), report)
	}

	// The received direction carries the peer's OPEN then its UPDATE; the sent
	// direction carries this host's OPEN then its KEEPALIVE.
	byDirection := map[string][]byte{}
	for i := range streams {
		byDirection[streams[i].Flow.SourceAddr.String()] = streams[i].Bytes
	}
	if want := append(append([]byte{}, open...), update...); !bytes.Equal(byDirection[testPeerAddr], want) {
		t.Errorf("received direction = % x, want % x", byDirection[testPeerAddr], want)
	}
	if want := append(append([]byte{}, open...), keepalive()...); !bytes.Equal(byDirection[testLocalAddr], want) {
		t.Errorf("sent direction = % x, want % x", byDirection[testLocalAddr], want)
	}
}

// TestExportBGPPcapSequenceAdvances is AC-3: two messages in one direction
// advance the sequence by the first one's length, so neither reads as a
// retransmission. The proof is that the reassembly joins them with no gap; a
// repeated sequence would make the second overlap the first.
func TestExportBGPPcapSequenceAdvances(t *testing.T) {
	first := bgpMessage(29, 1, 0xA1)
	second := bgpMessage(35, 2, 0xB2)

	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirIn, testPeerAddr, testLocalAddr, 10*time.Millisecond, second),
		entry(captureDirIn, testPeerAddr, testLocalAddr, 0, first),
	}
	file, err := exportBGPPcap(entries)
	if err != nil {
		t.Fatalf("exportBGPPcap: %v", err)
	}

	streams, report, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1: %s", len(streams), report)
	}
	want := append(append([]byte{}, first...), second...)
	if !bytes.Equal(streams[0].Bytes, want) {
		t.Errorf("stream is %d bytes, want the %d bytes of both messages end to end", len(streams[0].Bytes), len(want))
	}
}

// TestExportBGPPcapIPv6 is AC-5: an IPv6 peer is written, not skipped, and the
// same link type carries it.
func TestExportBGPPcapIPv6(t *testing.T) {
	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirIn, "2001:db8::1", "2001:db8::2", 0, keepalive()),
	}
	file, err := exportBGPPcap(entries)
	if err != nil {
		t.Fatalf("exportBGPPcap: %v", err)
	}

	streams, report, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1: %s", len(streams), report)
	}
	if streams[0].Flow.SourceAddr.Is4() {
		t.Errorf("source address %s was framed as IPv4", streams[0].Flow.SourceAddr)
	}
	if !bytes.Equal(streams[0].Bytes, keepalive()) {
		t.Errorf("stream = % x, want the KEEPALIVE", streams[0].Bytes)
	}
}

// TestExportBGPPcapTruncated is AC-6: a message longer than the ring slot is
// exported with the true original length declared, so a reader shows truncation
// rather than a malformed packet.
func TestExportBGPPcapTruncated(t *testing.T) {
	const originalLen = 9000
	captured := bgpMessage(4096, 2, 0xCC)
	binary.BigEndian.PutUint16(captured[16:18], originalLen)

	entries := []plugin.BGPRawCaptureEntry{{
		Timestamp:   testStamp.UTC().Format(captureTimeLayout),
		Direction:   captureDirIn,
		PeerAddr:    testPeerAddr,
		LocalAddr:   testLocalAddr,
		Data:        captured,
		OriginalLen: originalLen,
	}}

	file, err := exportBGPPcap(entries)
	if err != nil {
		t.Fatalf("exportBGPPcap: %v", err)
	}

	reader, err := pcap.NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var record pcap.Record
	if err := reader.Next(&record); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if want := pcap.IPv4HeaderLen + pcap.TCPHeaderLen + len(captured); len(record.Data) != want {
		t.Errorf("captured bytes = %d, want %d", len(record.Data), want)
	}
	if want := pcap.IPv4HeaderLen + pcap.TCPHeaderLen + originalLen; record.OriginalLen != want {
		t.Errorf("original length = %d, want %d", record.OriginalLen, want)
	}
}

// TestExportBGPPcapUnmatchedPeerNamesTheUnknown proves an entry the reactor
// could not match to a peer still exports, with the unspecified address in
// place of the local end, rather than failing or inventing a host.
func TestExportBGPPcapUnmatchedPeerNamesTheUnknown(t *testing.T) {
	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirIn, testPeerAddr, "invalid IP", 0, keepalive()),
	}
	file, err := exportBGPPcap(entries)
	if err != nil {
		t.Fatalf("exportBGPPcap: %v", err)
	}
	streams, _, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1", len(streams))
	}
	if !streams[0].Flow.TargetAddr.IsUnspecified() {
		t.Errorf("target address = %s, want the unspecified address", streams[0].Flow.TargetAddr)
	}
}

// TestExportBGPPcapRefusesABadTimestamp proves a timestamp the exporter cannot
// read is named rather than silently written as the epoch.
func TestExportBGPPcapRefusesABadTimestamp(t *testing.T) {
	entries := []plugin.BGPRawCaptureEntry{{
		Timestamp: "not a time",
		Direction: captureDirIn,
		PeerAddr:  testPeerAddr,
		LocalAddr: testLocalAddr,
		Data:      keepalive(),
	}}
	if _, err := exportBGPPcap(entries); err == nil {
		t.Fatal("exportBGPPcap accepted an unparsable timestamp")
	}
}

// TestExportBFDPcapUnchanged is AC-7: the BFD capture is byte-identical to what
// the shared exporter wrote before the BGP one gained IP and TCP framing. The
// expected bytes are built here by hand, so the test states the old format
// rather than asking the new code whether it agrees with itself.
func TestExportBFDPcapUnchanged(t *testing.T) {
	first := []byte{0x20, 0xC0, 0x03, 0x18}
	second := []byte{0x21, 0xC0, 0x03, 0x18, 0xAA}

	// The offsets are whole seconds: captureTimeLayout carries no fractional
	// digits, so a sub-second offset would arrive back as the same instant.
	// The precision loss is a defect of its own, recorded in
	// plan/journal/mtime-granularity-stamp.md, and this test asserts what the
	// code does rather than working around it.
	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirOut, "", "", time.Second, second),
		entry(captureDirIn, "", "", 0, first),
	}

	got, err := exportBFDPcap(entries)
	if err != nil {
		t.Fatalf("exportBFDPcap: %v", err)
	}

	var want bytes.Buffer
	hdr := make([]byte, 24)
	binary.LittleEndian.PutUint32(hdr[0:4], 0xa1b2c3d4)
	binary.LittleEndian.PutUint16(hdr[4:6], 2)
	binary.LittleEndian.PutUint16(hdr[6:8], 4)
	binary.LittleEndian.PutUint32(hdr[16:20], 4096)
	binary.LittleEndian.PutUint32(hdr[20:24], 101)
	want.Write(hdr)
	for i, data := range [][]byte{first, second} {
		ts := testStamp.Add(time.Duration(i) * time.Second)
		rec := make([]byte, 16)
		binary.LittleEndian.PutUint32(rec[0:4], uint32(ts.Unix()))            //nolint:gosec // a fixed test timestamp
		binary.LittleEndian.PutUint32(rec[4:8], uint32(ts.Nanosecond()/1000)) //nolint:gosec // below one million
		binary.LittleEndian.PutUint32(rec[8:12], uint32(len(data)))           //nolint:gosec // a slice length
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(data)))          //nolint:gosec // a slice length
		want.Write(rec)
		want.Write(data)
	}

	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("the BFD capture changed.\ngot:  % x\nwant: % x", got, want.Bytes())
	}
}

// TestExportBFDPcapCarriesNoTCPFraming is the other half of AC-7: no port-179
// framing reached the UDP protocol's export.
func TestExportBFDPcapCarriesNoTCPFraming(t *testing.T) {
	entries := []plugin.BGPRawCaptureEntry{
		entry(captureDirIn, testPeerAddr, testLocalAddr, 0, keepalive()),
	}
	file, err := exportBFDPcap(entries)
	if err != nil {
		t.Fatalf("exportBFDPcap: %v", err)
	}

	streams, report, err := pcap.Reassemble(bytes.NewReader(file), bgpPort)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 0 {
		t.Errorf("the BFD export produced %d port-%d TCP flows, want none", len(streams), bgpPort)
	}
	if report.FlowsSeen != 0 {
		t.Errorf("the BFD export produced %d TCP flows, want none", report.FlowsSeen)
	}
}
