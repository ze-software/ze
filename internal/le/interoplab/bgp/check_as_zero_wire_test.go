// Design: docs/architecture/testing/interop.md -- AS-zero checker vacuity controls.
package bgp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

// TestASZeroRawFixtureFields pins the exact OLD-width and compatibility ASN
// encodings independently of the fixture builder's branch choices.
func TestASZeroRawFixtureFields(t *testing.T) {
	ze := netip.MustParseAddr("172.31.71.2")
	want := map[byte][2]string{
		2:  {"0202fde9fdfc", "0202fde90000"},
		17: {"02020000fde900011170", "02020000fde900000000"},
		7:  {"fdfcc0000209", "0000c0000209"},
		18: {"00011170c0000209", "00000000c0000209"},
	}
	for _, input := range asZeroWireInputs(ze) {
		for polarity, body := range [][]byte{input.clean, input.zero} {
			update := decodeSpeakerUpdate(body)
			if update.decodeErr != nil {
				t.Fatal(update.decodeErr)
			}
			found := 0
			for _, attr := range update.attributes {
				if attr.code != input.code {
					continue
				}
				found++
				if got := hex.EncodeToString(attr.value); got != want[input.code][polarity] {
					t.Fatalf("attribute %d polarity %d: %s", input.code, polarity, got)
				}
			}
			if found != 1 {
				t.Fatalf("attribute %d occurs %d times", input.code, found)
			}
		}
	}
}

// TestASZeroRawWireOracle rejects leaks even if FRR would reject them itself.
// These are independent oracle fixtures; a real native run remains required.
func TestASZeroRawWireOracle(t *testing.T) {
	ze := netip.MustParseAddr("172.31.71.2")
	native := netip.MustParseAddr("172.31.71.3")
	good := asZeroWireTestFrames(ze, native, false)
	text := asZeroWireTestText(t, good)
	if complete, err := judgeASZeroWire(text, ze, native, false); err != nil || !complete {
		t.Fatalf("good interval: complete=%v error=%v", complete, err)
	}
	for ordinal, input := range asZeroWireInputs(ze) {
		t.Run(input.prefix, func(t *testing.T) {
			bad := asZeroWireTestFrames(ze, native, false)
			bad[6+ordinal] = speakerMessage(bgpUpdate, input.zero)
			if _, err := judgeASZeroWire(asZeroWireTestText(t, bad), ze, native, false); err == nil {
				t.Fatal("AS-zero announcement passed in place of its held-route withdrawal")
			}
		})
	}
	for _, test := range []struct {
		name   string
		frames func() [][]byte
	}{
		{"missing OPEN", func() [][]byte { return good[1:] }},
		{"missing KEEPALIVE", func() [][]byte { return append(append([][]byte{}, good[:1]...), good[2:]...) }},
		{"missing announcement", func() [][]byte { return append(append([][]byte{}, good[:2]...), good[3:]...) }},
		{"missing withdrawal", func() [][]byte { return append(append([][]byte{}, good[:6]...), good[7:]...) }},
		{"replacement OPEN", func() [][]byte { return append(append([][]byte{}, good[:2]...), good...) }},
		{"notification", func() [][]byte {
			return append(append([][]byte{}, good[:2]...), speakerMessage(bgpNotification, []byte{6, 0}))
		}},
		{"malformed header", func() [][]byte { f := asZeroWireTestFrames(ze, native, false); f[2][16] = 1; return f }},
		{"wrong speaker", func() [][]byte { f := asZeroWireTestFrames(ze, native, false); f[0][21]++; return f }},
		{"wrong identifier", func() [][]byte { f := asZeroWireTestFrames(ze, native, false); f[0][27]++; return f }},
		{"unexpected Ze ASN4", func() [][]byte {
			f := asZeroWireTestFrames(ze, native, false)
			f[0] = asZeroWireTestOpen(ze, 65001, true)
			return f
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := judgeASZeroWire(asZeroWireTestText(t, test.frames()), ze, native, false); err == nil {
				t.Fatal("invalid interval passed")
			}
		})
	}
	for _, partial := range []string{"", text[:len(text)-2], asZeroWireTestText(t, good[:len(good)-1])} {
		if complete, _ := judgeASZeroWire(partial, ze, native, false); complete {
			t.Fatal("incomplete capture passed")
		}
	}
	rewritten := strings.Replace(text, `"original":`, `"delivered":"00","original":`, 1)
	if _, err := judgeASZeroWire(rewritten, ze, native, false); err == nil {
		t.Fatal("relay rewrite passed")
	}
	if complete, err := judgeASZeroWire(asZeroWireTestText(t, asZeroWireTestFrames(ze, native, true)), ze, native, true); err != nil || !complete {
		t.Fatalf("native handshake: complete=%v error=%v", complete, err)
	}
	badNative := asZeroWireTestFrames(ze, native, true)
	badNative[0][31] = 70
	if _, err := judgeASZeroWire(asZeroWireTestText(t, badNative), ze, native, true); err == nil {
		t.Fatal("native ASN4 capability was not required")
	}
}

// TestASZeroNativeEchoOracle retains the reverse-direction frames observed in
// the FRR 10.7.1 live run bgp-as-zero-raw-frr-2870687208. With neither solo nor
// sender-as-path-loop-detection configured, subgroup_announce_check can export
// learned eBGP bestpaths to the source. This stream witnesses the native session;
// exact Ze egress and native route installation/withdrawal are separate checks.
func TestASZeroNativeEchoOracle(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	native := netip.MustParseAddr("172.30.0.3")
	var good [][]byte
	for _, raw := range []string{
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF006D0104FDEA005AAC1E00035002060104000100010202020002024600020641040000FDEA02020600020645040001010102074C0500010100000210490E0C3061343162323935336133350002044002C0780209470700010180000000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0032020000001740010100500200080203FDEAFDE9FDFC400304AC1E0003180A4C00",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0044020000002940010100500200080203FDEAFDE95BA0400304AC1E0003D011000E02030000FDEA0000FDE900011170180A4C01",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003B020000002040010100500200080203FDEAFDE9FDFC400304AC1E0003C00706FDFCC0000209180A4C02",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0046020000002B40010100500200080203FDEAFDE9FDFC400304AC1E0003C007065BA0C0000209C0120800011170C0000209180A4C03",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A4C000000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A4C010000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A4C020000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A4C030000",
		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0032020000001740010100500200080203FDEAFDE9FDFC400304AC1E0003180A4CFF",
	} {
		frame, err := hex.DecodeString(raw)
		if err != nil {
			t.Fatal(err)
		}
		good = append(good, frame)
	}
	if complete, err := judgeASZeroWire(asZeroWireTestText(t, good), ze, native, true); err != nil || !complete {
		t.Fatalf("observed native echoes: complete=%v error=%v", complete, err)
	}
	for _, test := range []struct {
		name   string
		frames [][]byte
	}{
		{"missing handshake witness", append(append([][]byte{}, good[:1]...), good[2:]...)},
		{"truncated withdrawn section", append(append([][]byte{}, good[:2]...), speakerMessage(bgpUpdate, []byte{0, 5, 0, 0}))},
		{"malformed attribute envelope", append(append([][]byte{}, good[:2]...), speakerMessage(bgpUpdate, []byte{0, 0, 0, 1, 0x40}))},
		{"notification after echoes", append(append([][]byte{}, good...), speakerMessage(bgpNotification, []byte{6, 0}))},
		{"reopened native session", append(append([][]byte{}, good...), good[0])},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := judgeASZeroWire(asZeroWireTestText(t, test.frames), ze, native, true); err == nil {
				t.Fatal("invalid native stream passed")
			}
		})
	}
}

// TestASZeroNativeControlOracle prevents a dead decoder or wrong session from
// satisfying the valid twin. The OLD-width AS4_PATH twin also requires native
// reconstruction of the non-mappable 70000, not mere prefix acceptance.
func TestASZeroNativeControlOracle(t *testing.T) {
	good := `{"prefix":"10.76.0.0/24","paths":[{"valid":true,"peer":{"peerId":"172.31.71.10"},"aspath":{"string":"65001 65020"}}]}`
	if err := requireASZeroFRRPath(good, "10.76.0.0/24", "172.31.71.10", "65001 65020", 0); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"{}", strings.Replace(good, `"valid":true`, `"valid":false`, 1), strings.Replace(good, "65001 65020", "65001 0", 1), strings.Replace(good, "172.31.71.10", "172.31.71.11", 1), strings.Replace(good, "10.76.0.0/24", "10.76.1.0/24", 1)} {
		if err := requireASZeroFRRPath(bad, "10.76.0.0/24", "172.31.71.10", "65001 65020", 0); err == nil {
			t.Fatalf("invalid native observation passed: %s", bad)
		}
	}
	as4 := strings.Replace(good, "65001 65020", "65001 70000", 1)
	if err := requireASZeroFRRPath(as4, "10.76.0.0/24", "172.31.71.10", "65001 70000", 0); err != nil {
		t.Fatal(err)
	}
	if err := requireASZeroFRRPath(good, "10.76.0.0/24", "172.31.71.10", "65001 70000", 0); err == nil {
		t.Fatal("unreconstructed AS4_PATH passed")
	}
}

// TestASZeroNativeAggregatorOracle requires the foreign decoder's result for
// both aggregator twins, including AS4 reconstruction beyond AS_TRANS.
func TestASZeroNativeAggregatorOracle(t *testing.T) {
	for _, asn := range []uint32{65020, 70000} {
		good := fmt.Sprintf(`{"prefix":"10.76.2.0/24","paths":[{"valid":true,"peer":{"peerId":"172.31.71.10"},"aspath":{"string":"65001 65020"},"aggregatorAs":%d,"aggregatorId":"192.0.2.9"}]}`, asn)
		judge := func(text string) error {
			return requireASZeroFRRPath(text, "10.76.2.0/24", "172.31.71.10", "65001 65020", asn)
		}
		if err := judge(good); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{
			strings.Replace(good, fmt.Sprintf(`,"aggregatorAs":%d`, asn), "", 1),
			strings.Replace(good, fmt.Sprintf(`"aggregatorAs":%d`, asn), `"aggregatorAs":23456`, 1),
			strings.Replace(good, fmt.Sprintf(`"aggregatorAs":%d`, asn), `"aggregatorAs":0`, 1),
			strings.Replace(good, `,"aggregatorId":"192.0.2.9"`, "", 1),
			strings.Replace(good, `"aggregatorId":"192.0.2.9"`, `"aggregatorId":"192.0.2.10"`, 1),
		} {
			if err := judge(bad); err == nil {
				t.Fatalf("missing or incorrect native aggregator passed: %s", bad)
			}
		}
	}
}

func asZeroWireTestFrames(ze, native netip.Addr, fromNative bool) [][]byte {
	address, asn := ze, uint16(65001)
	if fromNative {
		address, asn = native, 65002
	}
	frames := [][]byte{asZeroWireTestOpen(address, asn, fromNative), speakerMessage(bgpKeepalive, nil)}
	if fromNative {
		return frames
	}
	for _, input := range asZeroWireInputs(ze) {
		frames = append(frames, speakerMessage(bgpUpdate, input.clean))
	}
	for ordinal := range 4 {
		frames = append(frames, speakerMessage(bgpUpdate, asZeroWireWithdrawal(byte(ordinal))))
	}
	return append(frames, speakerMessage(bgpUpdate, asZeroWireBody(0, 255, false, ze)))
}

func asZeroWireTestOpen(address netip.Addr, asn uint16, asn4 bool) []byte {
	router := address.As4()
	open := []byte{4, byte(asn >> 8), byte(asn), 0, 90}
	open = append(open, router[:]...)
	if asn4 {
		open = append(open, 8, 2, 6, 65, 4, 0, 0, byte(asn>>8), byte(asn))
	} else {
		open = append(open, 0)
	}
	return speakerMessage(bgpOpen, open)
}

func asZeroWireTestText(t *testing.T, frames [][]byte) string {
	t.Helper()
	var text bytes.Buffer
	encoder := json.NewEncoder(&text)
	for _, frame := range frames {
		if err := encoder.Encode(extendedRelayFrame{Original: hex.EncodeToString(frame)}); err != nil {
			t.Fatal(err)
		}
	}
	return text.String()
}
