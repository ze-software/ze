// Design: docs/labs/pppoe-interop.md -- discriminate the physical discovery oracle.
// Related: product_discovery.go -- fixtures pin wire identity and complete exchanges.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

func productDiscoveryFrames(t *testing.T) [][]byte {
	t.Helper()
	server, err := net.ParseMAC("02:00:00:00:00:01")
	if err != nil {
		t.Fatal(err)
	}
	client, err := net.ParseMAC(productSessionFixture().MAC)
	if err != nil {
		t.Fatal(err)
	}
	// The AC-Name is ten octets; the explicit wire lengths are deliberate
	// fixture data so a broken producer cannot update its own expected bytes.
	tags := []byte{1, 2, 0, 10, 'z', 'e', '-', 'v', 'p', 'p', '-', 'b', 'n', 'g', 1, 1, 0, 8, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'}
	var result [][]byte
	for _, code := range []byte{0x09, 0x07, 0x19, 0x65} {
		from, to := client, server
		if code == 0x09 {
			to = net.HardwareAddr{255, 255, 255, 255, 255, 255}
		}
		if code == 0x07 || code == 0x65 {
			from, to = server, client
		}
		frame := append([]byte(nil), to...)
		frame = append(frame, from...)
		frame = append(frame, 0x88, 0x63, 0x11, code, 0, 0, 0, 0)
		if code == 0x07 || code == 0x65 {
			binary.BigEndian.PutUint16(frame[18:20], uint16(len(tags)))
			frame = append(frame, tags...)
		}
		if code == 0x65 {
			binary.BigEndian.PutUint16(frame[16:18], 17)
		}
		result = append(result, frame)
	}
	return result
}

func productDiscoveryPCAP(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	var wire bytes.Buffer
	if err := pcap.WriteFileHeader(&wire, 512, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	for _, frame := range frames {
		if err := pcap.WriteRecord(&wire, time.Unix(1, 0), frame, len(frame)); err != nil {
			t.Fatal(err)
		}
	}
	return wire.Bytes()
}

func TestProductDiscoveryRequiresPhysicalACAndWireSID(t *testing.T) {
	check := func(frames [][]byte) error {
		return productDiscovery(productDiscoveryPCAP(t, frames), "02:00:00:00:00:01", []productSession{productSessionFixture()})
	}
	if err := check(productDiscoveryFrames(t)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func([][]byte) [][]byte
	}{
		{"missing offer", func(frames [][]byte) [][]byte { return append(frames[:1], frames[2:]...) }},
		{"wrong AC name", func(frames [][]byte) [][]byte {
			frames[1] = bytes.Replace(frames[1], []byte("ze-vpp-bng"), []byte("foreign-ac"), 1)
			return frames
		}},
		{"wrong source MAC", func(frames [][]byte) [][]byte { frames[1][11] = 2; return frames }},
		{"wrong successful SID", func(frames [][]byte) [][]byte { frames[3][17] = 18; return frames }},
		{"error PADS", func(frames [][]byte) [][]byte { frames[3][17] = 0; return frames }},
		{"truncated tag", func(frames [][]byte) [][]byte { frames[1][19] = 255; return frames }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if check(test.change(productDiscoveryFrames(t))) == nil {
				t.Fatal("accepted incomplete, foreign or mismatched discovery")
			}
		})
	}
	frames := productDiscoveryFrames(t)
	for index, frame := range frames {
		tagged := append([]byte(nil), frame[:12]...)
		tagged = append(tagged, 0x81, 0, 0, 100, 0x88, 0x63)
		frames[index] = append(tagged, frame[14:]...)
	}
	if err := check(frames); err != nil {
		t.Fatalf("valid tagged discovery refused: %v", err)
	}
	if err := productDiscovery([]byte("usage: tcpdump"), "02:00:00:00:00:01", []productSession{productSessionFixture()}); err == nil {
		t.Fatal("a capture failure became wire evidence")
	}
	if err := productDiscoveryTags([]byte(strings.Repeat("\x00", 3))); err == nil {
		t.Fatal("truncated tag header accepted")
	}
}
