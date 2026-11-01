package ribevents

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestValidationRouteJSONRoundTrip proves a validation-change batch survives
// the JSON a plugin-process subscriber receives (bgp-rr and bgp-rs subscribe
// through the SDK on both transports). Method: marshal a batch holding a CIDR
// path and a non-CIDR key whose NLRI bytes are not valid UTF-8, decode it, and
// compare every field; the keys MUST be kebab-case.
func TestValidationRouteJSONRoundTrip(t *testing.T) {
	routes := []ValidationRoute{
		{
			Peer:   netip.MustParseAddr("192.0.2.1"),
			Family: family.IPv4Unicast,
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			PathID: 7,
		},
		{
			Peer:   netip.MustParseAddr("2001:db8::1"),
			Family: family.IPv6Unicast,
			NLRI:   "\x01\x18\xc0\x00\x02\xff\xfe",
		},
	}
	encoded, err := json.Marshal(routes)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"peer-address"`, `"family"`, `"prefix"`, `"path-id"`, `"nlri"`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("encoded batch lacks key %s: %s", key, encoded)
		}
	}
	var decoded []ValidationRoute
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal %s: %v", encoded, err)
	}
	if len(decoded) != len(routes) {
		t.Fatalf("decoded %d routes, want %d", len(decoded), len(routes))
	}
	for i := range routes {
		if decoded[i] != routes[i] {
			t.Errorf("route %d: decoded %+v, want %+v", i, decoded[i], routes[i])
		}
	}
}

// TestValidationRouteJSONRefusesBadNLRI proves a decoder never invents a key:
// an nlri that is not hex refuses the whole route.
func TestValidationRouteJSONRefusesBadNLRI(t *testing.T) {
	var route ValidationRoute
	err := json.Unmarshal([]byte(`{"peer-address":"192.0.2.1","family":"ipv4/unicast","nlri":"zz"}`), &route)
	if err == nil {
		t.Fatalf("decoded %+v from a non-hex nlri, want an error", route)
	}
}

// TestDecodeValidationChange proves the envelope reader answers each case
// apart: the event decodes to its batch, another event or a text line is not
// matched, and a matched event whose batch does not decode is refused.
func TestDecodeValidationChange(t *testing.T) {
	batch, err := json.Marshal([]ValidationRoute{{
		Peer: netip.MustParseAddr("192.0.2.1"), Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.0.0.0/8"), PathID: 3,
	}})
	if err != nil {
		t.Fatal(err)
	}
	event := `{"namespace":"` + Namespace + `","event":"` + EventValidationChange + `","payload":` + string(batch) + `}`
	routes, matched, err := DecodeValidationChange(event)
	if err != nil || !matched || len(routes) != 1 || routes[0].PathID != 3 {
		t.Fatalf("event: routes %+v matched %v err %v", routes, matched, err)
	}
	for _, other := range []string{
		`{"namespace":"bgp-gr","event":"retention","payload":{}}`,
		`{"namespace":"` + Namespace + `","event":"best-change","payload":{}}`,
		"peer 192.0.2.1 remote as 65001 state up\n",
	} {
		if _, matched, err := DecodeValidationChange(other); matched || err != nil {
			t.Errorf("%q: matched %v err %v, want neither", other, matched, err)
		}
	}
	bad := `{"namespace":"` + Namespace + `","event":"` + EventValidationChange + `","payload":[{"nlri":"zz"}]}`
	if _, matched, err := DecodeValidationChange(bad); !matched || err == nil {
		t.Fatalf("malformed batch: matched %v err %v, want matched and refused", matched, err)
	}
}
