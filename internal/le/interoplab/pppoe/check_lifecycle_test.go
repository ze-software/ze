// Design: docs/architecture/testing/interop.md -- non-vacuous lifecycle observations.
package pppoe

import (
	"bytes"
	"crypto/md5" // #nosec G501 -- CHAP test vectors.
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

var lifecycleZeMAC = [6]byte{2, 0, 0, 0, 0, 1}
var lifecyclePeerMAC = [6]byte{2, 0, 0, 0, 0, 2}

func lifecycleCHAPExchange(id byte) []lifecycleFrame {
	value := []byte{1, 2, 3, id}
	input := append([]byte{id}, []byte(pppdPassword)...)
	input = append(input, value...)
	digest := md5.Sum(input) // #nosec G401 -- the required CHAP-MD5 test vector.
	response := append([]byte{16}, digest[:]...)
	response = append(response, pppdUsername...)
	return []lifecycleFrame{
		{source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, protocol: 0xc223, code: 1, id: id, data: append([]byte{4}, value...)},
		{source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, protocol: 0xc223, code: 2, id: id, data: response},
		{source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, protocol: 0xc223, code: 3, id: id},
	}
}

// TestRechallengeEvidence pins exact direction, session, digest and correlation;
// an initial login or an unrelated success cannot stand in for reauthentication.
func TestRechallengeEvidence(t *testing.T) {
	cases := []struct {
		name   string
		change func([]lifecycleFrame) []lifecycleFrame
	}{
		{"missing second exchange", func(f []lifecycleFrame) []lifecycleFrame { return f[:3] }},
		{"missing response", func(f []lifecycleFrame) []lifecycleFrame { return append(f[:1], f[2:]...) }},
		{"wrong digest", func(f []lifecycleFrame) []lifecycleFrame { f[1].data[1] ^= 1; return f }},
		{"wrong username", func(f []lifecycleFrame) []lifecycleFrame { f[1].data[17] = 'x'; return f }},
		{"wrong response identifier", func(f []lifecycleFrame) []lifecycleFrame { f[1].id++; return f }},
		{"wrong success identifier", func(f []lifecycleFrame) []lifecycleFrame { f[2].id++; return f }},
		{"wrong direction", func(f []lifecycleFrame) []lifecycleFrame { f[1].source = lifecyclePeerMAC; return f }},
		{"new session", func(f []lifecycleFrame) []lifecycleFrame { f[3].sid++; return f }},
		{"reused identifier", func(f []lifecycleFrame) []lifecycleFrame { f[3].id = f[0].id; return f }},
		{"reused value", func(f []lifecycleFrame) []lifecycleFrame { f[3].data = bytes.Clone(f[0].data); return f }},
		{"failure", func(f []lifecycleFrame) []lifecycleFrame { f[2].code = 4; return f }},
		{"renegotiation", func(f []lifecycleFrame) []lifecycleFrame {
			return append(f, lifecycleFrame{sid: 42, protocol: 0x8021, code: 1})
		}},
		{"padt", func(f []lifecycleFrame) []lifecycleFrame { return append(f, lifecycleFrame{sid: 42, code: 0xa7}) }},
		{"empty challenge", func(f []lifecycleFrame) []lifecycleFrame { f[0].data[0] = 0; return f }},
		{"truncated challenge", func(f []lifecycleFrame) []lifecycleFrame { f[0].data[0] = 255; return f }},
	}
	good := append(lifecycleCHAPExchange(2), lifecycleCHAPExchange(3)...)
	if err := rechallengeEvidence(good, lifecycleZeMAC); err != nil {
		t.Fatal(err)
	}
	// A capture can start between the earlier Response and Success; those
	// packets must not count toward the two fully observed exchanges.
	preamble := lifecycleCHAPExchange(1)[1:]
	if err := rechallengeEvidence(append(preamble, good...), lifecycleZeMAC); err != nil {
		t.Fatal(err)
	}
	if err := rechallengeEvidence(nil, lifecycleZeMAC); err == nil {
		t.Fatal("empty capture passed")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frames := append(lifecycleCHAPExchange(2), lifecycleCHAPExchange(3)...)
			if err := rechallengeEvidence(tc.change(frames), lifecycleZeMAC); err == nil {
				t.Fatal("invalid reauthentication passed")
			}
		})
	}
}

func lifecycleTermination() []lifecycleFrame {
	at := time.Unix(100, 0)
	return []lifecycleFrame{
		{at: at, source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, protocol: 0xc021, code: 5, id: 9},
		{at: at.Add(time.Millisecond), source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, protocol: 0xc021, code: 6, id: 9},
		{at: at.Add(3*time.Second + time.Millisecond), source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, code: 0xa7},
	}
}

// TestPeerTerminateEvidence refuses the old happy-path escape: a client's PADT
// cannot replace Ze's delayed tlf PADT, even when the session table empties.
func TestPeerTerminateEvidence(t *testing.T) {
	if err := terminateEvidence(lifecycleTermination(), lifecycleZeMAC, 42); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func([]lifecycleFrame) []lifecycleFrame
	}{
		{"no PADT", func(f []lifecycleFrame) []lifecycleFrame { return f[:2] }},
		{"no request", func(f []lifecycleFrame) []lifecycleFrame { return f[1:] }},
		{"no ack", func(f []lifecycleFrame) []lifecycleFrame { return append(f[:1], f[2:]...) }},
		{"early PADT", func(f []lifecycleFrame) []lifecycleFrame {
			f[2].at = f[0].at.Add(3*time.Second - time.Microsecond)
			return f
		}},
		{"client PADT", func(f []lifecycleFrame) []lifecycleFrame { f[2].source = lifecyclePeerMAC; return f }},
		{"wrong ack", func(f []lifecycleFrame) []lifecycleFrame { f[1].id++; return f }},
		{"wrong session", func(f []lifecycleFrame) []lifecycleFrame { f[2].sid++; return f }},
		{"duplicate PADT", func(f []lifecycleFrame) []lifecycleFrame { return append(f, f[2]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := terminateEvidence(tc.change(lifecycleTermination()), lifecycleZeMAC, 42); err == nil {
				t.Fatal("invalid teardown passed")
			}
		})
	}
}

// TestLifecycleFrames validates capture framing independently of product codecs,
// including padding and both lengths that delimit a PPP control packet.
func TestLifecycleFrames(t *testing.T) {
	raw := make([]byte, 60)
	copy(raw[:6], lifecycleZeMAC[:])
	copy(raw[6:12], lifecyclePeerMAC[:])
	binary.BigEndian.PutUint16(raw[12:14], 0x8864)
	raw[14] = 0x11
	binary.BigEndian.PutUint16(raw[16:18], 42)
	binary.BigEndian.PutUint16(raw[18:20], 8)
	copy(raw[20:], []byte{0xc2, 0x23, 1, 9, 0, 6, 1, 0x55})
	capture := func(packet []byte, original int, link uint32) []byte {
		t.Helper()
		var buf bytes.Buffer
		if err := pcap.WriteFileHeader(&buf, 1600, link); err != nil {
			t.Fatal(err)
		}
		if err := pcap.WriteRecord(&buf, time.Unix(100, 0), packet, original); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	frames, err := lifecycleFrames(capture(raw, len(raw), pcap.LinkTypeEthernet))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0].data, []byte{1, 0x55}) || frames[0].sid != 42 {
		t.Fatalf("padding leaked or header changed: %+v", frames)
	}
	for _, mutate := range []func([]byte){
		func(p []byte) { p[19] = 255 },
		func(p []byte) { p[25] = 5 },
		func(p []byte) { p[14] = 0 },
	} {
		bad := bytes.Clone(raw)
		mutate(bad)
		if _, err := lifecycleFrames(capture(bad, len(bad), pcap.LinkTypeEthernet)); err == nil {
			t.Fatal("malformed packet passed")
		}
	}
	if _, err := lifecycleFrames(capture(raw, len(raw)+1, pcap.LinkTypeEthernet)); err == nil {
		t.Fatal("truncation passed")
	}
	if _, err := lifecycleFrames(capture(raw, len(raw), 101)); err == nil {
		t.Fatal("non-Ethernet capture passed")
	}
}
