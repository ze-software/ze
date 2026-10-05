package bgp

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestMEDWholeSetRenderedNetwork reads the actual scenario and checks every
// injected frame byte. On a nondefault subnet only the NEXT_HOP address changes;
// C's rendered transport identity, wire next hop and checker expectations agree.
func TestMEDWholeSetRenderedNetwork(t *testing.T) {
	t.Parallel()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "test", "interop", "scenarios", medWholeSetScenario)
	original := medWholeSetReadFrames(t, filepath.Join(source, "inject.msg"))
	if len(original) != 3 {
		t.Fatalf("scenario input has %d frames, want two baseline paths and C", len(original))
	}
	for _, test := range []struct {
		name    string
		network string
		peer    string
		nextHop [4]byte
	}{
		{name: "default", network: "172.30.0.0/24", peer: "172.30.0.9", nextHop: [4]byte{172, 30, 0, 9}},
		{name: "nondefault", network: "172.31.71.0/24", peer: "172.31.71.9", nextHop: [4]byte{172, 31, 71, 9}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			network := interoplab.Network{IPv4: netip.MustParsePrefix(test.network)}
			target := t.TempDir()
			if err := renderScenario(source, target, network); err != nil {
				t.Fatal(err)
			}
			frames := medWholeSetReadFrames(t, filepath.Join(target, "inject.msg"))
			if len(frames) != len(original) {
				t.Fatalf("rendering changed the UPDATE population: %d", len(frames))
			}
			// The literal fixtures have 19-byte headers, four length octets,
			// four ORIGIN octets, AS_PATH lengths 13/13/9 and a three-byte
			// NEXT_HOP header. These offsets are independent of the renderer.
			for index, offset := range []int{43, 43, 39} {
				wanted := bytes.Clone(original[index])
				if !bytes.Equal(wanted[offset:offset+4], []byte{172, 30, 0, 9}) {
					t.Fatalf("fixture %d no longer places its NEXT_HOP at %d", index, offset)
				}
				copy(wanted[offset:offset+4], test.nextHop[:])
				if !bytes.Equal(frames[index], wanted) {
					t.Errorf("frame %d differs beyond the selected-network NEXT_HOP\ngot  %X\nwant %X", index, frames[index], wanted)
				}
			}
			config, err := os.ReadFile(filepath.Join(target, zeConfigFile))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(config), "remote { ip "+test.peer+"; }") {
				t.Fatalf("C transport configuration does not name %s", test.peer)
			}
			input := newMEDWholeSetCandidate(network.IPv4.Addr().As4(), 9, 65004, 1, 100)
			if input.peer.String() != test.peer {
				t.Errorf("C peer expectation=%s, transport=%s", input.peer, test.peer)
			}
			nextHop := netip.AddrFrom4([4]byte(frames[2][39:43]))
			if input.peer != nextHop {
				t.Errorf("C wire expectation=%s, next hop=%s", input.peer, nextHop)
			}
		})
	}
}

func medWholeSetReadFrames(t *testing.T, path string) [][]byte {
	t.Helper()
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var frames [][]byte
	for line := range strings.SplitSeq(string(input), "\n") {
		if !strings.HasPrefix(line, "action=send:") {
			continue
		}
		_, encoded, found := strings.Cut(line, ":hex=")
		if !found {
			t.Fatalf("input send has no literal frame: %s", line)
		}
		frame, err := hex.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	return frames
}

// TestRenderInjectedNextHopBoundaries protects deliberately malformed injector
// fixtures and opaque attributes while exercising the real scenario renderer.
func TestRenderInjectedNextHopBoundaries(t *testing.T) {
	t.Parallel()
	base, err := hex.DecodeString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003A020000001B4001010040020602010000FDEC400304AC1E00098004040000006400000003180A634D")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		alter  func([]byte) []byte
		render bool
	}{
		{name: "valid UPDATE", render: true},
		{name: "bad marker", alter: func(frame []byte) []byte { frame[0] = 0; return frame }},
		{name: "declared length mismatch", alter: func(frame []byte) []byte { frame[17]++; return frame }},
		{name: "not UPDATE", alter: func(frame []byte) []byte { frame[18] = 1; return frame }},
		{name: "malformed NEXT_HOP", alter: func(frame []byte) []byte { frame[38] = 3; return frame }},
		{name: "wrong NEXT_HOP flags", alter: func(frame []byte) []byte { frame[36] = 0x80; return frame }},
		{name: "nonlab NEXT_HOP", alter: func(frame []byte) []byte {
			copy(frame[39:43], []byte{192, 0, 2, 9})
			return frame
		}},
		{name: "opaque address-shaped bytes", render: true, alter: func(frame []byte) []byte {
			result := append(bytes.Clone(frame[:50]), 0xc0, 99, 4, 172, 30, 0, 9)
			result = append(result, frame[50:]...)
			result[17] += 7
			result[22] += 7
			return result
		}},
		{name: "truncated following attribute", alter: func(frame []byte) []byte {
			result := append(bytes.Clone(frame[:50]), 0x80)
			result = append(result, frame[50:]...)
			result[17]++
			result[22]++
			return result
		}},
		{name: "duplicate NEXT_HOP", alter: func(frame []byte) []byte {
			result := append(bytes.Clone(frame[:50]), 0x40, 3, 4, 172, 30, 0, 9)
			result = append(result, frame[50:]...)
			result[17] += 7
			result[22] += 7
			return result
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			frame := bytes.Clone(base)
			if test.alter != nil {
				frame = test.alter(frame)
			}
			source := t.TempDir()
			text := "# AC1E0009 in a comment is not an address field.\n" +
				"action=send:conn=1:seq=1:hex=" + strings.ToUpper(hex.EncodeToString(frame)) + "\n"
			if err := os.WriteFile(filepath.Join(source, "inject.msg"), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			target := t.TempDir()
			if err := renderScenario(source, target, interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}); err != nil {
				t.Fatal(err)
			}
			got := medWholeSetReadFrames(t, filepath.Join(target, "inject.msg"))
			want := bytes.Clone(frame)
			if test.render {
				copy(want[39:43], []byte{172, 31, 71, 9})
			}
			if len(got) != 1 {
				t.Fatalf("rendered %d frames, want one", len(got))
			}
			if !bytes.Equal(got[0], want) {
				t.Fatalf("rendered frame changed wrong bytes\ngot  %X\nwant %X", got[0], want)
			}
		})
	}
}
