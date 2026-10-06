// Design: docs/architecture/testing/interop.md -- container-free checker branches.
// Related: check_extended_message.go -- real FRR execution; these tests do not claim interop.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestExtendedOpenRewrite proves the relay changes capability 6 alone, preserving
// the local original and every other optional parameter across both directions.
func TestExtendedOpenRewrite(t *testing.T) {
	for _, original := range []bool{false, true} {
		for _, delivered := range []bool{false, true} {
			frame := extendedTestOpen(original)
			before := bytes.Clone(frame)
			var target [4096]byte
			// RFC 8654 Section 3: the fixture rewrites just the empty code6 TLV.
			octets, found, err := rewriteExtendedOpen(target[:], frame, delivered)
			if err != nil {
				t.Fatal(err)
			}
			if found != original {
				t.Fatal("original capability permission lost")
			}
			if !bytes.Equal(frame, before) {
				t.Fatal("rewrite mutated the original local OPEN")
			}
			if !bytes.Equal(target[:octets], extendedTestOpen(delivered)) {
				t.Fatal("rewrite changed more than code6 or failed to change it")
			}
		}
	}
	valid := extendedTestOpen(true)
	for cut := range valid {
		var target [4096]byte
		// RFC 8654 Section 3: incomplete capability envelopes are not evidence.
		if _, _, err := rewriteExtendedOpen(target[:], valid[:cut], true); err == nil {
			t.Fatalf("truncated OPEN at %d accepted", cut)
		}
	}
	for name, frame := range map[string][]byte{
		"not OPEN":            speakerKeepalive(),
		"parameter header":    extendedTestOpenBody([]byte{2}),
		"parameter length":    extendedTestOpenBody([]byte{2, 4, 6, 0}),
		"capability header":   extendedTestOpenBody([]byte{2, 1, 6}),
		"capability value":    extendedTestOpenBody([]byte{2, 2, 6, 2}),
		"nonempty code6":      extendedTestOpenBody([]byte{2, 3, 6, 1, 0}),
		"extended parameters": extendedTestOpenBody(append([]byte{255, 1}, make([]byte, 253)...)),
	} {
		t.Run(name, func(t *testing.T) {
			var target [4096]byte
			// RFC 8654 Section 3: reject malformed or unsupported OPEN envelopes.
			if _, _, err := rewriteExtendedOpen(target[:], frame, true); err == nil {
				t.Fatal("invalid OPEN accepted")
			}
		})
	}
	// RFC 8654 Section 3: a caller-owned buffer must fit the rewritten OPEN.
	if _, _, err := rewriteExtendedOpen(make([]byte, 29), valid, true); !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("short buffer: %v", err)
	}
}

// TestExtendedFrameBoundaries uses one-byte reads to distinguish BGP wire length
// from TCP segmentation, including 4096, 4097 and the maximum 65535-octet frame.
func TestExtendedFrameBoundaries(t *testing.T) {
	for _, size := range []int{19, 4096, 4097, 65535} {
		frame := speakerMessage(bgpUpdate, make([]byte, size-19))
		var target [65535]byte
		// RFC 4271 Section 4.1 and RFC 8654 Section 2: framed rather than packet I/O.
		octets, err := readExtendedFrame(iotest.OneByteReader(bytes.NewReader(frame)), target[:])
		if err != nil {
			t.Fatal(err)
		}
		if octets != size || !bytes.Equal(target[:octets], frame) {
			t.Fatal("fragmented frame changed")
		}
	}
	for name, frame := range map[string][]byte{
		"empty":            nil,
		"truncated header": make([]byte, 18),
		"bad marker":       make([]byte, 19),
		"short length":     append(bytes.Repeat([]byte{255}, 16), 0, 18, 2),
		"truncated body":   append(bytes.Repeat([]byte{255}, 16), 0, 20, 2),
	} {
		t.Run(name, func(t *testing.T) {
			var target [65535]byte
			// RFC 4271 Section 4.1: incomplete envelopes fail closed.
			if _, err := readExtendedFrame(bytes.NewReader(frame), target[:]); err == nil {
				t.Fatal("invalid frame accepted")
			}
		})
	}
	for _, size := range []int{18, 19} {
		// RFC 8654 Section 2: reject a destination shorter than the actual frame.
		if _, err := readExtendedFrame(bytes.NewReader(speakerMessage(bgpUpdate, []byte{0})), make([]byte, size)); !errors.Is(err, io.ErrShortBuffer) {
			t.Fatalf("short destination: %v", err)
		}
	}
}

// TestExtendedRelayPreservesPayloads drives the transport itself, not only its
// metadata predicate: only the OPEN is rewritten and captured UPDATE bytes match.
func TestExtendedRelayPreservesPayloads(t *testing.T) {
	frames := [][]byte{extendedTestOpen(false), speakerKeepalive(), extendedTestUpdate(1, true)}
	input := bytes.Join(frames, nil)
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	var target bytes.Buffer
	err := captureExtendedRelay(&target, bytes.NewReader(input), true, path)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("end of bounded fixture stream: %v", err)
	}
	want := bytes.Join([][]byte{extendedTestOpen(true), frames[1], frames[2]}, nil)
	if !bytes.Equal(target.Bytes(), want) {
		t.Fatal("relay modified UPDATE or KEEPALIVE bytes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 8654 Sections 3, 4 and 6: original and delivered permissions differ.
	capture, err := parseExtendedCapture(string(data), false, true)
	if err != nil {
		t.Fatal(err)
	}
	if capture.largeOctets != len(frames[2]) || capture.updatesLarge != 1 {
		t.Fatal("capture did not retain the actual extended frame")
	}
	if err := captureExtendedRelay(&target, bytes.NewReader(input), true, path); err == nil {
		t.Fatal("relay overwrote a previous session capture")
	}
	for name, writer := range map[string]io.Writer{"short": extendedShortWriter{}, "error": extendedErrorWriter{}} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.jsonl")
			if err := captureExtendedRelay(writer, bytes.NewReader(input), true, path); err == nil {
				t.Fatal("failed forwarding passed")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) != 0 {
				t.Fatal("capture claimed an unwritten frame")
			}
		})
	}
}

// TestExtendedMessageCheckerBranches pins both polarities of every foreign-wire
// verdict without Docker; the named scenario runs remain the interoperability proof.
func TestExtendedMessageCheckerBranches(t *testing.T) {
	frames := [][]byte{extendedTestOpen(false), speakerKeepalive(), extendedTestUpdate(1, true), extendedTestUpdate(2, false)}
	text := extendedTestTranscript(t, frames)
	// RFC 8654 Sections 3, 4 and 6: local-original false is not rewritten-local true.
	capture, err := parseExtendedCapture(text, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if capture.largeOctets <= 4096 || !capture.control || capture.keepalives != 1 {
		t.Fatal("positive transcript evidence missing")
	}
	notification := speakerMessage(bgpNotification, []byte{1, 2, 0x12, 0xd7})
	notified := text + extendedTestTranscript(t, [][]byte{notification})
	// RFC 8654 Sections 4 and 5: retain the actual error for the exact verdict.
	capture, err = parseExtendedCapture(notified, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(capture.notification, notification) {
		t.Fatal("capture lost the NOTIFICATION")
	}
	for name, output := range map[string]string{
		"empty":                  "",
		"invalid JSON":           "{",
		"bad hex":                `{"original":"XYZ"}`,
		"short frame":            `{"original":"FF"}`,
		"rewritten UPDATE":       text + `{"original":"` + textbuf.StringHexUpper(extendedTestUpdate(2, false)) + `","delivered":"00"}`,
		"duplicate OPEN":         text + extendedTestTranscript(t, [][]byte{extendedTestOpen(false)}),
		"no keepalive":           extendedTestTranscript(t, [][]byte{extendedTestOpen(false)}),
		"keepalive body":         extendedTestTranscript(t, [][]byte{extendedTestOpen(false), speakerMessage(bgpKeepalive, []byte{0})}),
		"unknown type":           text + extendedTestTranscript(t, [][]byte{speakerMessage(99, nil)}),
		"no OPEN":                extendedTestTranscript(t, [][]byte{speakerKeepalive()}),
		"trailing frame bytes":   extendedTestTranscript(t, [][]byte{append(extendedTestOpen(false), 0)}),
		"extended OPEN":          extendedTestTranscript(t, [][]byte{speakerMessage(bgpOpen, make([]byte, 4097-19))}),
		"duplicate notification": notified + extendedTestTranscript(t, [][]byte{notification}),
		"too many frames":        text + strings.Repeat(extendedTestTranscript(t, [][]byte{speakerKeepalive()}), 256),
	} {
		t.Run(name, func(t *testing.T) {
			// RFC 8654 Sections 3, 4 and 6: missing or invalid evidence cannot pass.
			if _, err := parseExtendedCapture(output, false, true); err == nil {
				t.Fatal("invalid capture passed")
			}
		})
	}
	// RFC 8654 Section 4: relay-added advertisement cannot stand in for local OPEN.
	if _, err := parseExtendedCapture(text, true, true); err == nil {
		t.Fatal("rewritten permission passed as local advertisement")
	}
	// RFC 8654 Section 3: the delivered OPEN must match the chosen remote view.
	if _, err := parseExtendedCapture(text, false, false); err == nil {
		t.Fatal("wrong delivered capability passed")
	}
	for _, frame := range [][]byte{nil, make([]byte, 22), speakerMessage(bgpUpdate, []byte{255, 255, 0, 0}), speakerMessage(bgpUpdate, []byte{0, 0, 255, 255}), speakerMessage(bgpUpdate, []byte{0, 0, 0, 0, 33}), speakerMessage(bgpUpdate, []byte{0, 0, 0, 0, 24, 10})} {
		// RFC 4271 Section 4.3: truncation must not make an attribute look like NLRI.
		if extendedFramePrefix(frame, 1) {
			t.Fatal("malformed UPDATE passed as the target prefix")
		}
	}
	// RFC 4271 Section 4.3: another prefix is not evidence for the target route.
	if extendedFramePrefix(extendedTestUpdate(2, true), 1) {
		t.Fatal("wrong prefix passed")
	}
}

// TestExtendedRejectionBranches requires the precise error plus its offending
// wire length, and a foreign session generation that really closed.
func TestExtendedRejectionBranches(t *testing.T) {
	frame := speakerMessage(bgpNotification, []byte{1, 2, 0x12, 0xd7})
	// RFC 8654 Sections 4 and 5: exact Bad Message Length evidence.
	if err := requireExtendedRejection(frame, 0x12d7); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{18, 19, 20, 21, 22} {
		bad := bytes.Clone(frame)
		bad[offset]++
		// RFC 8654 Section 4: a different error or offending message must fail.
		if err := requireExtendedRejection(bad, 0x12d7); err == nil {
			t.Fatalf("wrong error at offset %d passed", offset)
		}
	}
	// RFC 8654 Section 4: no absent notification passes on a disconnect alone.
	if err := requireExtendedRejection(nil, 0x12d7); err == nil {
		t.Fatal("missing notification passed")
	}
	for _, state := range []struct {
		output string
		closed bool
		valid  bool
	}{
		{`{"peer":{"bgpState":"Idle","connectionsEstablished":1}}`, true, true},
		{`{"peer":{"bgpState":"Established","connectionsEstablished":1}}`, false, true},
		{`{"peer":{"bgpState":"Idle","connectionsEstablished":2}}`, false, false},
		{`{"peer":{"connectionsEstablished":1}}`, false, false},
		{`{}`, false, false},
		{`{`, false, false},
	} {
		closed, err := extendedFRRSessionClosed(state.output, "peer")
		if (err == nil) != state.valid || closed != state.closed {
			t.Fatalf("closed=%v err=%v for %s", closed, err, state.output)
		}
	}
}

// TestExtendedFRRRouteBranches refuses a local FRR route, wrong peer, wrong
// AS_PATH, missing communities, duplicates and an invalid received path.
func TestExtendedFRRRouteBranches(t *testing.T) {
	communities := make([]string, extendedCommunities)
	var community textbuf.Buffer
	for index := range communities {
		communities[index] = community.Reset().Str("65002:8654:").Int(int64(index + 1)).String()
	}
	path := map[string]any{
		"valid": true, "peer": map[string]string{"peerId": "172.30.0.11"},
		"aspath":         map[string]string{"string": "65001 65002"},
		"largeCommunity": map[string]any{"list": communities},
	}
	route := map[string]any{"prefix": extendedLargePrefix, "paths": []any{path}}
	encode := func() string {
		t.Helper()
		data, err := json.Marshal(route)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := requireExtendedFRRRoute(encode(), extendedLargePrefix, "172.30.0.11", true); err != nil {
		t.Fatal(err)
	}
	if err := requireExtendedFRRRoute(encode(), extendedLargePrefix, "172.30.0.11", false); err != nil {
		t.Fatal(err)
	}
	for field, invalid := range map[string]any{
		"valid":          false,
		"peer":           map[string]string{"peerId": "0.0.0.0"},
		"aspath":         map[string]string{"string": "65002"},
		"largeCommunity": map[string]any{"list": communities[:399]},
	} {
		old := path[field]
		path[field] = invalid
		if err := requireExtendedFRRRoute(encode(), extendedLargePrefix, "172.30.0.11", true); err == nil {
			t.Fatalf("invalid %s passed", field)
		}
		path[field] = old
	}
	communities[399] = communities[398]
	if err := requireExtendedFRRRoute(encode(), extendedLargePrefix, "172.30.0.11", true); err == nil {
		t.Fatal("duplicate community passed")
	}
	for _, output := range []string{"{", "{}", `{"prefix":"10.86.0.0/24","paths":[]}`, `{"prefix":"10.86.1.0/24","paths":[]}`} {
		if err := requireExtendedFRRRoute(output, extendedLargePrefix, "172.30.0.11", true); err == nil {
			t.Fatal("missing foreign route passed")
		}
	}
}

// TestExtendedRelayOptions pins the existing speaker entry point and bounded
// relay-specific arguments without requiring synthetic ASN/router-ID options.
func TestExtendedRelayOptions(t *testing.T) {
	args := []string{"--connect", "172.30.0.2:179", "--relay-peer", "172.30.0.3:179", "--result", extendedCaptureBase, "--relay-extended=true", "--duration", "180"}
	options, err := parseSpeakerOptions(args)
	if err != nil || !options.relayExtended {
		t.Fatalf("relay entry point: %+v %v", options, err)
	}
	for _, arguments := range [][]string{
		{"--relay-peer", "172.30.0.3:179"},
		{"--connect", "172.30.0.2:179", "--relay-peer", "172.30.0.3:179"},
		append(slices.Clone(args), "--duration", "0"),
		append(slices.Clone(args), "--duration", "301"),
	} {
		if _, err := parseSpeakerOptions(arguments); err == nil {
			t.Fatal("unbounded or incomplete relay options accepted")
		}
	}
	for name := range extendedMessageCases {
		if checkers()[name] == nil {
			t.Fatalf("scenario %s has no checker", name)
		}
	}
}

// TestExtendedRelayPreparationRequiresNativePeers exercises the preparation
// consumer: an absent native endpoint cannot produce a runnable relay plan.
func TestExtendedRelayPreparationRequiresNativePeers(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
	relays := []extendedRelayPeer{
		{peer: interoplab.PeerConfig{Name: peerSpeaker, Host: 10}, destination: peerFRR},
		{peer: interoplab.PeerConfig{Name: peerSpeaker2, Host: 11}, destination: peerFRRSink},
	}
	for _, peers := range [][]interoplab.PeerConfig{
		nil,
		{{Name: "ze"}},
		{{Name: peerFRR}},
		{{Name: "ze"}, {Name: peerFRR}},
		{{Name: "ze"}, {Name: peerFRRSink}},
	} {
		if _, err := prepareExtendedRelayPeers(peers, relays, network); err == nil {
			t.Fatal("relay preparation admitted an absent native endpoint")
		}
	}
}

// TestExtendedRelayGoBGPPreparation keeps FRR-specific readiness keyed to the
// parsed destination, not merely the presence or spelling of --relay-peer.
func TestExtendedRelayGoBGPPreparation(t *testing.T) {
	for _, option := range []string{"--relay-peer 172.31.71.5:179", "--relay-peer=172.31.71.5:179"} {
		t.Run(option, func(t *testing.T) {
			scenario := t.TempDir()
			writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
			writeFixture(t, filepath.Join(scenario, "frr.conf"), "router bgp 65002\n")
			writeFixture(t, filepath.Join(scenario, "gobgp.toml"), "[global.config]\n")
			writeFixture(t, filepath.Join(scenario, "speaker-args"), option+"\n--duration 180\n--result /tmp/extended\n")
			network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
			peers, err := scenarioPeers(t.TempDir(), scenario, "gobgp-relay-test", network)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 4 || peers[0].Name != "ze" || peers[1].Name != peerSpeaker ||
				peers[2].Name != peerFRR || peers[3].Name != peerGoBGP {
				t.Fatalf("GoBGP relay startup order changed: %+v", peers)
			}
			if len(peers[0].Ready.Contains) != 0 || len(peers[2].Ready.Contains) != 0 ||
				!slices.Equal(peers[2].Ready.Command, []string{cmdVtysh, "-c", "show version"}) {
				t.Fatal("GoBGP relay installed FRR-specific readiness probes")
			}
		})
	}
}

// TestExtendedFRRRouteQueriesConsumer proves the decoded acceptance evidence is
// read from the independent sink, not the producer's locally originated route.
func TestExtendedFRRRouteQueriesConsumer(t *testing.T) {
	lab := &extendedRouteLab{
		output: `{"prefix":"10.86.0.0/24","paths":[{"valid":true,"peer":{"peerId":"172.31.71.11"},"aspath":{"string":"65001 65002"}}]}`,
	}
	check := &interoplab.CheckContext{
		Lab:     lab,
		Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")},
	}
	if err := waitExtendedFRRRoute(t.Context(), check, extendedBaselinePrefix, false); err != nil {
		t.Fatal(err)
	}
	if lab.peer != peerFRRSink || !slices.Equal(lab.command,
		[]string{cmdVtysh, "-c", "show bgp ipv4 unicast 10.86.0.0/24 json"}) {
		t.Fatalf("decoded route queried from %s with %v", lab.peer, lab.command)
	}
}

type extendedRouteLab struct {
	recordingLab
	peer    string
	command []string
}

func (lab *extendedRouteLab) Query(ctx context.Context, peer string, command []string, environment []interoplab.EnvironmentVariable) (string, error) {
	lab.peer = peer
	lab.command = command
	return lab.recordingLab.Query(ctx, peer, command, environment)
}

// TestExtendedFailureDiagnostics retains empty captures, read failures and both
// peers' raw evidence, even after cancellation, under one bounded deadline.
func TestExtendedFailureDiagnostics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lab := &extendedDiagnosticLab{t: t}
	output := extendedFailureDiagnostics(ctx, lab)
	for _, peer := range []string{peerSpeaker, peerSpeaker2} {
		for _, suffix := range []string{"-result.json", "-ze.jsonl", "-frr.jsonl"} {
			if !strings.Contains(output, peer+": "+extendedCaptureBase+suffix) {
				t.Fatalf("missing %s %s", peer, suffix)
			}
		}
	}
	for _, want := range []string{"raw partial capture", "stderr: missing file", "query error: read failed", "exit: 1", "logs unavailable"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostics lost %q: %s", want, output)
		}
	}
	if lab.reads != 8 {
		t.Fatalf("read %d files, want all four from both peers", lab.reads)
	}
	failed := &recordingLab{failure: errors.New("transport unavailable")}
	if output := extendedFailureDiagnostics(ctx, failed); !strings.Contains(output, "logs error: transport unavailable") {
		t.Fatal("log collection failure disappeared")
	}
}

type extendedDiagnosticLab struct {
	noEvidenceLab
	t        *testing.T
	reads    int
	deadline time.Time
}

func (lab *extendedDiagnosticLab) Exec(ctx context.Context, _ string, command []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	lab.t.Helper()
	if err := ctx.Err(); err != nil {
		lab.t.Fatal("diagnostics inherited cancellation", err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		lab.t.Fatal("diagnostics have no deadline")
	}
	if time.Until(deadline) > 15*time.Second {
		lab.t.Fatal("diagnostics exceeded the shared deadline")
	}
	if lab.deadline.IsZero() {
		lab.deadline = deadline
	} else if !lab.deadline.Equal(deadline) {
		lab.t.Fatal("diagnostic commands reset the shared deadline")
	}
	lab.reads++
	if command[1] == extendedCaptureBase+"-result.json" {
		return interoplab.CommandResult{ExitCode: 1, Stderr: "missing file"}, errors.New("read failed")
	}
	if command[1] == extendedCaptureBase+"-ze.jsonl" {
		return interoplab.CommandResult{}, nil
	}
	return interoplab.CommandResult{Stdout: "raw partial capture"}, nil
}

func (*extendedDiagnosticLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, nil
}

// RFC 8654 Section 3: "The BGP Extended Message Capability is a new BGP capability
// [RFC5492] defined with Capability Code 6 and Capability Length 0.".
func extendedTestOpen(extended bool) []byte {
	parameters := []byte{99, 1, 42, 2, 6, 1, 4, 0, 1, 0, 1}
	if extended {
		parameters = append(parameters, 2, 2, 6, 0)
	}
	// RFC 8654 Section 3: preserve an unrelated optional parameter and MP TLV.
	return extendedTestOpenBody(parameters)
}

// RFC 4271 Section 4.2: "This 1-octet unsigned integer indicates the total length
// of the Optional Parameters field in octets.".
func extendedTestOpenBody(parameters []byte) []byte {
	body := []byte{4, 0xfd, 0xe9, 0, 9, 172, 30, 0, 2, byte(len(parameters))}
	return speakerMessage(bgpOpen, append(body, parameters...))
}

// RFC 4271 Section 4.3: "This variable length field contains a list of IP address
// prefixes." The synthetic attribute only controls framing size in predicate tests;
// the runtime scenario's UPDATE is encoded by FRR, not this helper.
func extendedTestUpdate(subnet byte, large bool) []byte {
	attributes := 0
	if large {
		attributes = 4 + 12*extendedCommunities
	}
	body := make([]byte, 4+attributes+4)
	binary.BigEndian.PutUint16(body[2:4], uint16(attributes))
	if large {
		body[4], body[5] = 0xd0, 32
		binary.BigEndian.PutUint16(body[6:8], uint16(attributes-4))
	}
	copy(body[4+attributes:], []byte{24, 10, 86, subnet})
	return speakerMessage(bgpUpdate, body)
}

func extendedTestTranscript(t *testing.T, frames [][]byte) string {
	t.Helper()
	var output strings.Builder
	encoder := json.NewEncoder(&output)
	for _, frame := range frames {
		row := extendedRelayFrame{Original: textbuf.StringHexUpper(frame)}
		if len(frame) >= 29 && frame[18] == bgpOpen {
			var target [4096]byte
			// RFC 8654 Section 3: compute the relay's expected OPEN-only rewrite.
			octets, _, err := rewriteExtendedOpen(target[:], frame, true)
			if err == nil {
				row.Delivered = textbuf.StringHexUpper(target[:octets])
			}
		}
		if err := encoder.Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	return output.String()
}

type extendedShortWriter struct{}

func (extendedShortWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

type extendedErrorWriter struct{}

func (extendedErrorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
