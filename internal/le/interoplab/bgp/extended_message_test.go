// Design: docs/architecture/testing/interop.md -- container-free checker branches.
// Related: check_extended_message.go -- real FRR execution; these tests do not claim interop.
package bgp

import (
	"bytes"
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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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
	for cut := range len(valid) {
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
	text := extendedTestTranscript(t, frames, true)
	// RFC 8654 Sections 3, 4 and 6: local-original false is not rewritten-local true.
	capture, err := parseExtendedCapture(text, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if capture.largeOctets <= 4096 || !capture.control || capture.keepalives != 1 {
		t.Fatal("positive transcript evidence missing")
	}
	notification := speakerMessage(bgpNotification, []byte{1, 2, 0x12, 0xd7})
	notified := text + extendedTestTranscript(t, [][]byte{notification}, true)
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
		"duplicate OPEN":         text + extendedTestTranscript(t, [][]byte{extendedTestOpen(false)}, true),
		"no keepalive":           extendedTestTranscript(t, [][]byte{extendedTestOpen(false)}, true),
		"keepalive body":         extendedTestTranscript(t, [][]byte{extendedTestOpen(false), speakerMessage(bgpKeepalive, []byte{0})}, true),
		"unknown type":           text + extendedTestTranscript(t, [][]byte{speakerMessage(99, nil)}, true),
		"no OPEN":                extendedTestTranscript(t, [][]byte{speakerKeepalive()}, true),
		"trailing frame bytes":   extendedTestTranscript(t, [][]byte{append(extendedTestOpen(false), 0)}, true),
		"extended OPEN":          extendedTestTranscript(t, [][]byte{speakerMessage(bgpOpen, make([]byte, 4097-19))}, true),
		"duplicate notification": notified + extendedTestTranscript(t, [][]byte{notification}, true),
		"too many frames":        text + strings.Repeat(extendedTestTranscript(t, [][]byte{speakerKeepalive()}, true), 256),
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

// TestExtendedMessageScenarioWiring reads and renders every real fixture on a
// nondefault subnet, proving both existing sidecar slots launch the relay and
// retain their separate remote advertisements.
func TestExtendedMessageScenarioWiring(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
	for name, testCase := range extendedMessageCases {
		t.Run(name, func(t *testing.T) {
			rendered := t.TempDir()
			source := filepath.Join(root, "test", "interop", "scenarios", name)
			if err := renderScenario(source, rendered, network); err != nil {
				t.Fatal(err)
			}
			peers, err := scenarioPeers(filepath.Join(root, "test", "interop"), rendered, "extended-test", network)
			if err != nil {
				t.Fatal(err)
			}
			relays, foreign := 0, 0
			for _, peer := range peers {
				if peer.Name == peerFRR {
					foreign++
				}
				if peer.Name != peerSpeaker {
					if peer.Name != peerSpeaker2 {
						continue
					}
				}
				relays++
				options, err := parseSpeakerOptions(peer.Command[3:])
				if err != nil {
					t.Fatal(err)
				}
				if options.connect != "172.31.71.2:179" {
					t.Fatal("relay connects to the wrong Ze network")
				}
				if options.relayPeer != "172.31.71.3:179" {
					t.Fatal("relay does not connect to the real FRR container")
				}
				want := testCase.sourceRemote
				if peer.Name == peerSpeaker2 {
					want = testCase.sinkRemote
				}
				if options.relayExtended != want {
					t.Fatal("scenario's remote capability differs from its checker")
				}
			}
			if relays != 2 || foreign != 1 {
				t.Fatalf("got %d relays and %d FRR containers", relays, foreign)
			}
		})
	}
}

// RFC 8654 Section 3: "The BGP Extended Message Capability is a new BGP capability
// [RFC5492] defined with Capability Code 6 and Capability Length 0."
func extendedTestOpen(extended bool) []byte {
	parameters := []byte{99, 1, 42, 2, 6, 1, 4, 0, 1, 0, 1}
	if extended {
		parameters = append(parameters, 2, 2, 6, 0)
	}
	// RFC 8654 Section 3: preserve an unrelated optional parameter and MP TLV.
	return extendedTestOpenBody(parameters)
}

// RFC 4271 Section 4.2: "This 1-octet unsigned integer indicates the total length
// of the Optional Parameters field in octets."
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

func extendedTestTranscript(t *testing.T, frames [][]byte, delivered bool) string {
	t.Helper()
	var output strings.Builder
	encoder := json.NewEncoder(&output)
	for _, frame := range frames {
		row := extendedRelayFrame{Original: textbuf.StringHexUpper(frame)}
		if len(frame) >= 29 && frame[18] == bgpOpen {
			var target [4096]byte
			// RFC 8654 Section 3: compute the relay's expected OPEN-only rewrite.
			octets, _, err := rewriteExtendedOpen(target[:], frame, delivered)
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
