// Design: docs/labs/pppoe-interop.md -- non-vacuous CHAP-only refusal observations.

//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
)

func chapOnlyRefusalFrames() []lifecycleFrame {
	return []lifecycleFrame{
		{source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, code: 0x65},
		{source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, protocol: 0xc021, code: 1, id: 9, data: []byte{1, 4, 5, 0xd4, 3, 5, 0xc2, 0x23, 5}},
		{source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, protocol: 0xc021, code: 3, id: 9, data: []byte{3, 4, 0xc0, 0x23}},
		{source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, protocol: 0xc021, code: 5, id: 10},
		{source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, protocol: 0xc021, code: 6, id: 10},
		{source: lifecycleZeMAC, target: lifecyclePeerMAC, sid: 42, code: 0xa7},
	}
}

// TestCHAPOnlyRefusalEvidence judges the exact refusal chain, including direction,
// session and Identifier. Empty state, a client PADT or post-Opened failure cannot pass.
func TestCHAPOnlyRefusalEvidence(t *testing.T) {
	if checkers()["pppoe-chap-only-pap-client"] == nil {
		t.Fatal("CHAP-only scenario has no registered checker")
	}
	if err := chapOnlyRefusalEvidence(chapOnlyRefusalFrames(), lifecycleZeMAC); err != nil {
		t.Fatal(err)
	}
	// pppd may send its own PADT after receiving Ze's. It is not counted as Ze's.
	withClientPADT := append(chapOnlyRefusalFrames(), lifecycleFrame{source: lifecyclePeerMAC, target: lifecycleZeMAC, sid: 42, code: 0xa7})
	if err := chapOnlyRefusalEvidence(withClientPADT, lifecycleZeMAC); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func([]lifecycleFrame) []lifecycleFrame
	}{
		{"empty capture", func([]lifecycleFrame) []lifecycleFrame { return nil }},
		{"missing PADS", func(f []lifecycleFrame) []lifecycleFrame { return f[1:] }},
		{"discovery refusal", func(f []lifecycleFrame) []lifecycleFrame { f[0].sid = 0; return f }},
		{"wrong PADS source", func(f []lifecycleFrame) []lifecycleFrame { f[0].source = lifecyclePeerMAC; return f }},
		{"second session", func(f []lifecycleFrame) []lifecycleFrame { return append(f, f[0]) }},
		{"missing CHAP request", func(f []lifecycleFrame) []lifecycleFrame { return append(f[:1], f[2:]...) }},
		{"missing PAP Nak", func(f []lifecycleFrame) []lifecycleFrame { return append(f[:2], f[3:]...) }},
		{"wrong Nak identifier", func(f []lifecycleFrame) []lifecycleFrame { f[2].id++; return f }},
		{"wrong Nak direction", func(f []lifecycleFrame) []lifecycleFrame {
			f[2].source, f[2].target = lifecycleZeMAC, lifecyclePeerMAC
			return f
		}},
		{"wrong session", func(f []lifecycleFrame) []lifecycleFrame { f[2].sid++; return f }},
		{"zero session", func(f []lifecycleFrame) []lifecycleFrame { f[2].sid = 0; return f }},
		{"wrong endpoints", func(f []lifecycleFrame) []lifecycleFrame { f[3].target = [6]byte{}; return f }},
		{"PAP offered", func(f []lifecycleFrame) []lifecycleFrame { f[1].data = []byte{3, 4, 0xc0, 0x23}; return f }},
		{"auth option dropped", func(f []lifecycleFrame) []lifecycleFrame { f[1].data = nil; return f }},
		{"LCP could open", func(f []lifecycleFrame) []lifecycleFrame { f[2].code = 2; return f }},
		{"PAP authentication", func(f []lifecycleFrame) []lifecycleFrame { f[4].protocol = pppProtocolPAP; return f }},
		{"CHAP authentication", func(f []lifecycleFrame) []lifecycleFrame { f[4].protocol = 0xc223; return f }},
		{"IPCP started", func(f []lifecycleFrame) []lifecycleFrame { f[4].protocol = 0x8021; return f }},
		{"missing TermReq", func(f []lifecycleFrame) []lifecycleFrame { return append(f[:3], f[4:]...) }},
		{"early TermReq", func(f []lifecycleFrame) []lifecycleFrame { f[2], f[3] = f[3], f[2]; return f }},
		{"early PADT", func(f []lifecycleFrame) []lifecycleFrame { f[3], f[5] = f[5], f[3]; return f }},
		{"missing PADT", func(f []lifecycleFrame) []lifecycleFrame { return f[:5] }},
		{"client PADT only", func(f []lifecycleFrame) []lifecycleFrame {
			f[5].source, f[5].target = lifecyclePeerMAC, lifecycleZeMAC
			return f
		}},
		{"duplicate Ze PADT", func(f []lifecycleFrame) []lifecycleFrame { return append(f, f[5]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := chapOnlyRefusalEvidence(tc.change(chapOnlyRefusalFrames()), lifecycleZeMAC); err == nil {
				t.Fatal("invalid policy refusal passed")
			}
		})
	}
}

// TestCHAPOnlyAuthOption refuses malformed and duplicate options, and cannot
// confuse MRU, a CHAP variant or an absent option with a PAP Nak.
func TestCHAPOnlyAuthOption(t *testing.T) {
	want := []byte{3, 4, 0xc0, 0x23}
	for _, options := range [][]byte{{1}, {3, 0}, {3, 1}, {3, 5, 0xc0, 0x23}, {3, 4, 0xc0, 0x23, 3, 4, 0xc0, 0x23}} {
		if _, err := chapOnlyAuthOption(options, want); err == nil {
			t.Fatalf("malformed options accepted: %x", options)
		}
	}
	for _, options := range [][]byte{nil, {1, 4, 5, 0xd4}, {3, 5, 0xc2, 0x23, 5}} {
		matched, err := chapOnlyAuthOption(options, want)
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Fatalf("non-PAP option accepted as PAP: %x", options)
		}
	}
	matched, err := chapOnlyAuthOption([]byte{1, 4, 5, 0xd4, 3, 4, 0xc0, 0x23}, want)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatal("PAP option after MRU was not found")
	}
}

// TestCHAPOnlyRefusalCapture feeds actual Ethernet records through the shared
// lifecycle parser so PAP traffic cannot disappear between decoding and judgement.
func TestCHAPOnlyRefusalCapture(t *testing.T) {
	for _, pap := range []bool{false, true} {
		var capture bytes.Buffer
		if err := pcap.WriteFileHeader(&capture, 1600, pcap.LinkTypeEthernet); err != nil {
			t.Fatal(err)
		}
		frames := chapOnlyRefusalFrames()
		if pap {
			frames[4].protocol = pppProtocolPAP
		}
		for i, frame := range frames {
			payload := 0
			if frame.protocol != 0 {
				payload = 6 + len(frame.data)
			}
			raw := make([]byte, 20+payload)
			copy(raw[:6], frame.target[:])
			copy(raw[6:12], frame.source[:])
			etherType := uint16(0x8863)
			if frame.protocol != 0 {
				etherType = etherTypePPPoESession
			}
			binary.BigEndian.PutUint16(raw[12:14], etherType)
			raw[14] = 0x11
			raw[15] = frame.code
			binary.BigEndian.PutUint16(raw[16:18], frame.sid)
			binary.BigEndian.PutUint16(raw[18:20], uint16(payload)) // #nosec G115 -- these fixed fixtures have payloads below 16 octets.
			if frame.protocol != 0 {
				raw[15] = 0
				binary.BigEndian.PutUint16(raw[20:22], frame.protocol)
				raw[22], raw[23] = frame.code, frame.id
				binary.BigEndian.PutUint16(raw[24:26], uint16(4+len(frame.data))) // #nosec G115 -- bounded fixed test fixture.
				copy(raw[26:], frame.data)
			}
			if err := pcap.WriteRecord(&capture, time.Unix(100+int64(i), 0), raw, len(raw)); err != nil {
				t.Fatal(err)
			}
		}
		decoded, err := lifecycleFrames(capture.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		err = chapOnlyRefusalEvidence(decoded, lifecycleZeMAC)
		if pap {
			if err == nil {
				t.Fatal("captured PAP packet escaped the refusal checker")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestCHAPOnlyRefusalCause requires the explanatory record, not just a generic
// failure or unrelated lines mentioning PAP and the allowed method separately.
func TestCHAPOnlyRefusalCause(t *testing.T) {
	good := "ppp: authentication policy refused peer reason=auth-method-refused: peer suggested Auth-Protocol 0xc023 (pap); allowed [chap-md5]; configure the peer"
	if err := chapOnlyRefusalCause(good); err != nil {
		t.Fatal(err)
	}
	for _, log := range []string{
		"",
		"authentication failed",
		"auth-method-refused: peer suggested Auth-Protocol 0xc023 (pap)\nallowed [chap-md5]",
		"auth-method-refused: peer suggested Auth-Protocol 0xc023 (pap); allowed [chap-md5 pap]",
		"auth-method-refused: peer suggested Auth-Protocol 0xc223 (chap-md5); allowed [chap-md5]",
	} {
		if err := chapOnlyRefusalCause(log); err == nil {
			t.Fatalf("unexplained refusal accepted: %s", log)
		}
	}
}
