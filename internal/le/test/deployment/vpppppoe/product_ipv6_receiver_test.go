// Design: docs/labs/pppoe-interop.md -- discriminate exact wire observations.
// Related: product_ipv6_response.go -- pure envelope and bounded-window judge.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
	"time"
)

func productFamilyEnvelopeFixture(ipv6 bool) (request, response []byte) {
	request = make([]byte, 22)
	copy(request[:6], []byte{2, 0, 0, 0, 0, 2})
	copy(request[6:12], []byte{2, 0, 0, 0, 0, 3})
	binary.BigEndian.PutUint16(request[12:14], 0x8864)
	request[14] = 0x11
	binary.BigEndian.PutUint16(request[16:18], 17)
	protocol, header := uint16(0x21), 20
	if ipv6 {
		protocol, header = 0x57, 40
	}
	binary.BigEndian.PutUint16(request[20:22], protocol)
	ip := make([]byte, header+16)
	if ipv6 {
		ip[0], ip[6], ip[7] = 0x60, 58, 64
		binary.BigEndian.PutUint16(ip[4:6], 16)
		a, b := netip.MustParseAddr("2001:db8::3").As16(), netip.MustParseAddr("2001:db8::4").As16()
		copy(ip[8:24], a[:])
		copy(ip[24:40], b[:])
	} else {
		ip[0], ip[8], ip[9] = 0x45, 64, 1
		binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
		copy(ip[12:16], []byte{198, 18, 0, 3})
		copy(ip[16:20], []byte{198, 18, 0, 4})
	}
	echo := ip[header:]
	copy(echo, []byte{8, 0, 0, 0, 0x70, 0x70, 0, 1})
	copy(echo[8:], "envelope")
	if ipv6 {
		echo[0] = 128
	}
	request = append(request, ip...)
	binary.BigEndian.PutUint16(request[18:20], uint16(2+len(ip)))
	productFamilyFixtureChecksums(request, ipv6)
	response = bytes.Clone(request)
	copy(response[:6], request[6:12])
	copy(response[6:12], request[:6])
	p := response[22:]
	if ipv6 {
		copy(p[8:24], ip[24:40])
		copy(p[24:40], ip[8:24])
		p[header] = 129
	} else {
		copy(p[12:16], ip[16:20])
		copy(p[16:20], ip[12:16])
		p[header] = 0
	}
	productFamilyFixtureChecksums(response, ipv6)
	return request, response
}

func productFamilyFixtureChecksums(frame []byte, ipv6 bool) {
	p := frame[22:]
	offset := 20
	if ipv6 {
		offset = 40
	}
	p[offset+2], p[offset+3] = 0, 0
	if ipv6 {
		pseudo := append([]byte(nil), p[8:40]...)
		pseudo = append(pseudo, 0, 0, 0, byte(len(p)-40), 0, 0, 0, 58)
		pseudo = append(pseudo, p[40:]...)
		binary.BigEndian.PutUint16(p[42:44], productFamilyChecksum(pseudo))
	} else {
		binary.BigEndian.PutUint16(p[offset+2:offset+4], productFamilyChecksum(p[offset:]))
		p[10], p[11] = 0, 0
		binary.BigEndian.PutUint16(p[10:12], productFamilyChecksum(p[:20]))
	}
}

func TestProductFamilyResponseRequiresExactIPv4Envelope(t *testing.T) {
	request, response := productFamilyEnvelopeFixture(false)
	if !productFamilyResponse(response, request) {
		t.Fatal("valid exact-session IPv4 echo reply refused")
	}
	if !productFamilyResponse(append(bytes.Clone(response), 0, 0), request) {
		t.Fatal("Ethernet padding beyond declared PPPoE payload refused")
	}
	options := append(bytes.Clone(response[:42]), 1, 1, 1, 0)
	options = append(options, response[42:]...)
	options[22] = 0x46
	binary.BigEndian.PutUint16(options[24:26], 40)
	binary.BigEndian.PutUint16(options[18:20], 42)
	options[32], options[33] = 0, 0
	binary.BigEndian.PutUint16(options[32:34], productFamilyChecksum(options[22:46]))
	if !productFamilyResponse(options, request) {
		t.Fatal("valid IPv4 options/IHL envelope refused")
	}
	for _, test := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"version", func(p []byte) { p[0] = 0x65 }},
		{"short IHL", func(p []byte) { p[0] = 0x44 }},
		{"long IHL", func(p []byte) { p[0] = 0x4f }},
		{"short TotalLength", func(p []byte) { binary.BigEndian.PutUint16(p[2:4], uint16(len(p)-1)) }},
		{"long TotalLength", func(p []byte) { binary.BigEndian.PutUint16(p[2:4], uint16(len(p)+1)) }},
		{"header checksum", func(p []byte) { p[10] ^= 1 }},
		{"transport checksum", func(p []byte) { p[22] ^= 1 }},
		{"fragment", func(p []byte) { p[6] = 0x20 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := bytes.Clone(response)
			test.mutate(broken[22:])
			// Every header semantic corruption except the checksum-specific case has
			// a valid IPv4 header checksum; its untouched ICMP remains valid as well.
			if test.name != "header checksum" && test.name != "transport checksum" {
				broken[32], broken[33] = 0, 0
				binary.BigEndian.PutUint16(broken[32:34], productFamilyChecksum(broken[22:42]))
			}
			if test.name != "transport checksum" && productFamilyChecksum(broken[42:]) != 0 {
				t.Fatal("fixture unintentionally corrupted ICMP")
			}
			if productFamilyResponse(broken, request) {
				t.Fatal("corrupt IPv4 envelope accepted as an echo reply")
			}
		})
	}
	for _, mutate := range []func([]byte){func(p []byte) { p[0] ^= 1 }, func(p []byte) { p[16] ^= 1 }, func(p []byte) { p[49] ^= 1 }} {
		broken := bytes.Clone(response)
		mutate(broken)
		if productFamilyResponse(broken, request) {
			t.Fatal("MAC/SID/echo correlation lost")
		}
	}
}

func TestProductFamilyResponseRequiresExactIPv6Envelope(t *testing.T) {
	request, response := productFamilyEnvelopeFixture(true)
	if !productFamilyResponse(response, request) {
		t.Fatal("valid exact-session IPv6 echo reply refused")
	}
	for _, test := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"version", func(p []byte) { p[0] = 0x40 }},
		{"short PayloadLength", func(p []byte) { binary.BigEndian.PutUint16(p[4:6], 15) }},
		{"long PayloadLength", func(p []byte) { binary.BigEndian.PutUint16(p[4:6], 17) }},
		{"next header", func(p []byte) { p[6] = 17 }},
		{"source", func(p []byte) { p[23] ^= 1 }},
		{"transport checksum", func(p []byte) { p[42] ^= 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := bytes.Clone(response)
			test.mutate(broken[22:])
			if productFamilyResponse(broken, request) {
				t.Fatal("corrupt IPv6 envelope accepted as an echo reply")
			}
		})
	}
	for size := range response {
		if productFamilyResponse(response[:size], request) {
			t.Fatal("truncated IPv6 reply accepted")
		}
	}
}

func TestProductFamilyObservationDistinguishesBudgetFromFullWindow(t *testing.T) {
	request, response := productFamilyEnvelopeFixture(false)
	start := time.Unix(123, 0)
	for _, test := range []struct {
		name       string
		packets    bool
		wantBudget bool
	}{{"full observation window", false, false}, {"early unrelated-packet exhaustion", true, true}} {
		t.Run(test.name, func(t *testing.T) {
			now := start
			reads := 0
			wire, err := productFamilyObserve(func() time.Time { return now }, start.Add(time.Second), func(wait time.Duration) ([]byte, error) {
				reads++
				now = now.Add(wait)
				if test.packets {
					return []byte{1}, nil
				}
				return nil, nil
			}, request, 2)
			if !errors.Is(err, errProductFamilyObservationBudget) && test.wantBudget {
				t.Fatalf("budget exhaustion claimed silence: wire=%x err=%v", wire, err)
			}
			if !test.wantBudget && (err != nil || wire != nil || now.Before(start.Add(time.Second))) {
				t.Fatalf("full window=%v wire=%x err=%v", now, wire, err)
			}
			if test.wantBudget && (reads != 2 || !now.Before(start.Add(time.Second))) {
				t.Fatal("budget fixture did not stop before complete observation window")
			}
		})
	}
	now := start
	got, err := productFamilyObserve(func() time.Time { return now }, start.Add(time.Second), func(time.Duration) ([]byte, error) { return response, nil }, request, 2)
	if err != nil || !bytes.Equal(got, response) {
		t.Fatalf("valid response lost: %x %v", got, err)
	}
	transport := errors.New("reader failed")
	if _, err := productFamilyObserve(func() time.Time { return now }, start.Add(time.Second), func(time.Duration) ([]byte, error) { return nil, transport }, request, 2); !errors.Is(err, transport) {
		t.Fatal("reader error became a successful negative")
	}
}

func FuzzProductFamilyResponseEnvelope(f *testing.F) {
	request4, response4 := productFamilyEnvelopeFixture(false)
	request6, response6 := productFamilyEnvelopeFixture(true)
	f.Add(response4)
	f.Add(response6)
	f.Add([]byte{0x45})
	f.Fuzz(func(t *testing.T, frame []byte) {
		_ = productFamilyResponse(frame, request4)
		_ = productFamilyResponse(frame, request6)
	})
}
