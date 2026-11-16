// Design: docs/labs/pppoe-interop.md -- refuse misleading product-lab evidence.
// Related: product_capacity.go, product_state.go -- source/path and wire ownership.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

func productCapacityFrames(t *testing.T) [][]byte {
	t.Helper()
	frames := productDiscoveryFrames(t)
	_, client := productClient(100)
	mac, err := net.ParseMAC(client)
	if err != nil {
		t.Fatal(err)
	}
	service := []byte{1, 1, 0, 8, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'}
	name := []byte{1, 2, 0, 10, 'z', 'e', '-', 'v', 'p', 'p', '-', 'b', 'n', 'g'}
	unique := []byte{1, 3, 0, 4, 0, 0, 0, 101}
	for index, frame := range frames {
		if index == 1 || index == 3 {
			copy(frame[:6], mac)
		} else {
			copy(frame[6:12], mac)
		}
		frame = frame[:20]
		var tags []byte
		switch index {
		case 0, 2:
			tags = append(tags, service...)
		case 1:
			tags = append(tags, name...)
			tags = append(tags, service...)
		case 3:
			tags = append(tags, name...)
			tags = append(tags, 2, 2, 0, 0)
		}
		tags = append(tags, unique...)
		binary.BigEndian.PutUint16(frame[16:18], 0)
		binary.BigEndian.PutUint16(frame[18:20], uint16(len(tags)))
		frames[index] = append(frame, tags...)
	}
	return frames
}

// The same current client must reach PADR and receive Ze's zero-SID resource
// refusal. Neither an unsuccessful process nor somebody else's PADS is proof.
func TestProductCapacityRequiresCorrelatedPhysicalRefusal(t *testing.T) {
	_, client := productClient(100)
	check := func(frames [][]byte) error {
		return productCapacityRefusal(productDiscoveryPCAP(t, frames), "02:00:00:00:00:01", client, "00000065")
	}
	if err := check(productCapacityFrames(t)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func([][]byte) [][]byte
	}{
		{"no dial", func([][]byte) [][]byte { return nil }},
		{"no PADR", func(frames [][]byte) [][]byte { return append(frames[:2], frames[3:]...) }},
		{"no refusal", func(frames [][]byte) [][]byte { return frames[:3] }},
		{"foreign AC", func(frames [][]byte) [][]byte { frames[3][11] = 2; return frames }},
		{"wrong AC name", func(frames [][]byte) [][]byte {
			frames[3] = bytes.Replace(frames[3], []byte("ze-vpp-bng"), []byte("foreign-ac"), 1)
			return frames
		}},
		{"accepted SID", func(frames [][]byte) [][]byte { frames[3][17] = 17; return frames }},
		{"service error", func(frames [][]byte) [][]byte {
			frames[3] = bytes.Replace(frames[3], []byte{2, 2, 0, 0}, []byte{2, 1, 0, 0}, 1)
			return frames
		}},
		{"old transaction", func(frames [][]byte) [][]byte { frames[3][len(frames[3])-1] = 100; return frames }},
		{"truncated response", func(frames [][]byte) [][]byte { frames[3] = frames[3][:len(frames[3])-1]; return frames }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := check(test.change(productCapacityFrames(t))); err == nil {
				t.Fatal("accepted an uncorrelated or absent capacity refusal")
			}
		})
	}
	if !productCapacityReason(`level=WARN msg="pppoe: session ID exhausted" error="limit" reason=session-id-exhausted`) {
		t.Fatal("refused actual exhaustion producer")
	}
	for _, log := range []string{"exit status 1", "Timeout waiting for PADS packets", "pppoe: session ID exhausted reason=native-add-failed", "pppoe: session ID exhausted reason=session-id-exhausted-extra", "reason=session-id-exhausted msg=unrelated"} {
		if productCapacityReason(log) {
			t.Fatal("unrelated failure counted as capacity enforcement")
		}
	}
}

const productOwnedPathFixture = `10.20.0.2/32 fib:0 index:9 locks:1
  ze-pppoe refs:1
    path-list:[4] locks:1 flags:shared, uPRF-list:0
      path:[5] pl-index:4 ip4 weight=1 pref=0 attached-nexthop: oper-flags:resolved,
        10.20.0.2 pppoe_session0 (p2p)
 forwarding: unicast-ip4-chain
  [@0]: dpo-load-balance: [proto:ip4 index:9 buckets:1 uRPF:0 to:[0:0]]
`

// The interface must belong to the named source's own configured path. Tokens
// in a foreign source, another prefix or the selected forwarding DPO do not.
func TestProductRouteRequiresSourceSpecificPath(t *testing.T) {
	check := func(text string) error { return productHostRoute(text, "10.20.0.2", "pppoe_session0") }
	if err := check(productOwnedPathFixture); err != nil {
		t.Fatal(err)
	}
	foreign := strings.Replace(productOwnedPathFixture, "ze-pppoe", "api", 1)
	wrongOwned := strings.Replace(productOwnedPathFixture, "pppoe_session0", "pppoe_session9", 1)
	// A correct path from another source remains permitted, but cannot fill an
	// incorrect owned path. Reverse source order exercises both boundaries.
	foreignPath := foreign[strings.Index(foreign, "  api refs:"):strings.Index(foreign, " forwarding:")]
	wrongPath := wrongOwned[strings.Index(wrongOwned, "  ze-pppoe refs:"):strings.Index(wrongOwned, " forwarding:")]
	header := "10.20.0.2/32 fib:0 index:9 locks:1\n"
	for _, text := range []string{
		header + wrongPath + foreignPath + " forwarding: pppoe_session0\n",
		header + foreignPath + wrongPath + " forwarding: pppoe_session0\n",
		strings.Replace(productOwnedPathFixture, "10.20.0.2/32", "10.20.0.3/32", 1) + foreign,
		foreign,
		wrongOwned,
		strings.Replace(productOwnedPathFixture, "attached-nexthop:", "recursive:", 1),
		strings.Replace(productOwnedPathFixture, "pref=0", "pref=5", 1),
		strings.Replace(productOwnedPathFixture, "pppoe_session0", "pppoe_session01", 1),
		"10.20.0.2/32 fib:0 index:9\n  ze-pppoe refs:1\n forwarding: pppoe_session0\n",
	} {
		if err := check(text); err == nil {
			t.Fatalf("accepted a foreign or unbound source/path: %s", text)
		}
	}
	validOwned, _, present := strings.Cut(productOwnedPathFixture, " forwarding:")
	if !present {
		t.Fatal("owned fixture omitted its forwarding-display boundary")
	}
	if err := check(validOwned + foreignPath + " forwarding: foreign-path\n"); err != nil {
		t.Fatalf("foreign source displaced valid owned witness: %v", err)
	}
}

// Match the measured iputils MTU field, rather than a packet count or an
// unrelated occurrence of the same digits. Whitespace around '=' is allowed.
func TestProductMTURequiresExactMeasuredBoundary(t *testing.T) {
	for _, text := range []string{"From 192.0.2.1 Frag needed and DF set (mtu = 1492)", "message too long, mtu=1492"} {
		if err := productMTURefusal(text); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range []string{"", "1492 packets transmitted", "mtu=14920", "mtu = 1493", "device1492 unavailable", "mtu=1400, count1492"} {
		if productMTURefusal(text) == nil {
			t.Fatalf("unmeasured or wrong MTU accepted: %q", text)
		}
	}
}
