// Design: docs/architecture/api/json-format.md -- one key per family in "nlri"
// RFC: rfc/short/rfc7606.md -- Section 5.1, NLRI fields in any combination

package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Valid attribute values (no flags, code or length) for the combinations below.
var (
	// AFI 2, SAFI 1, next hop 2001:db8::1, reserved, NLRI 2001:db8::/32.
	mergeReachV6 = []byte{0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 32, 0x20, 0x01, 0x0d, 0xb8}
	// AFI 2, SAFI 1, withdrawn 2001:db8:1::/48.
	mergeUnreachV6 = []byte{0, 2, 1, 48, 0x20, 0x01, 0x0d, 0xb8, 0, 1}
	// AFI 1, SAFI 1, withdrawn 10.1.0.0/16.
	mergeUnreachV4 = []byte{0, 1, 1, 16, 10, 1}
	// AFI 1, SAFI 1, next hop 192.0.2.1, reserved, NLRI 10.0.0.0/24.
	mergeReachV4 = []byte{0, 1, 1, 4, 192, 0, 2, 1, 0, 24, 10, 0, 0}
)

// mpAttr wraps an MP attribute value as a whole optional attribute.
func mpAttr(code byte, value []byte) []byte {
	return append([]byte{0x80, code, byte(len(value))}, value...)
}

// updateJSON renders an UPDATE body (withdrawn, ORIGIN and NEXT_HOP 192.0.2.9
// plus extra attributes, body NLRI) through the JSON fast path.
func updateJSON(t *testing.T, withdrawn, extra, nlriField []byte) string {
	t.Helper()

	attrs := []byte{0x40, 1, 1, 0, 0x40, 3, 4, 192, 0, 2, 9}
	attrs = append(attrs, extra...)
	body := []byte{byte(len(withdrawn) >> 8), byte(len(withdrawn))}
	body = append(body, withdrawn...)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlriField...)
	wireUpdate := wireu.NewWireUpdate(body, testEncodingContext())
	attrsWire, err := wireUpdate.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, AttrsWire: attrsWire, WireUpdate: wireUpdate}
	return string(appendParsedUpdateJSONDirect(nil, testPeer(), msg, nil, rpc.DirectionReceived, "update"))
}

// nlriFamilies decodes the event's "nlri" object key by key, failing when a
// family key appears twice: encoding/json keeps the last copy of a duplicate
// key, which is how a consumer loses the first.
func nlriFamilies(t *testing.T, out string) map[string][]map[string]any {
	t.Helper()

	var event struct {
		BGP struct {
			Update struct {
				NLRI json.RawMessage `json:"nlri"`
			} `json:"update"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(out), &event); err != nil {
		t.Fatalf("the event is not JSON: %v\n%s", err, out)
	}
	decoder := json.NewDecoder(bytes.NewReader(event.BGP.Update.NLRI))
	if _, err := decoder.Token(); err != nil {
		t.Fatalf("nlri is not an object: %v\n%s", err, out)
	}
	families := map[string][]map[string]any{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatalf("nlri key: %v\n%s", err, out)
		}
		key, ok := token.(string)
		if !ok {
			t.Fatalf("nlri key %v is not a string\n%s", token, out)
		}
		if _, dup := families[key]; dup {
			t.Fatalf("the event states family %q twice, so a reader keeps only one copy:\n%s", key, out)
		}
		var operations []map[string]any
		if err := decoder.Decode(&operations); err != nil {
			t.Fatalf("family %q: %v\n%s", key, err, out)
		}
		families[key] = operations
	}
	return families
}

// operationOf returns the operation of family with the given action, failing
// when there is none.
func operationOf(t *testing.T, families map[string][]map[string]any, family, action, out string) map[string]any {
	t.Helper()

	for _, operation := range families[family] {
		if operation["action"] == action {
			return operation
		}
	}
	t.Fatalf("family %q has no %q operation:\n%s", family, action, out)
	return nil
}

// TestOneFamilyKeyPerUpdate renders UPDATEs carrying two NLRI fields of one
// family and decodes the "nlri" object key by key.
//
// VALIDATES: every family is one key whose array holds both operations, the
// add naming its route and the del naming its route, and an MP_UNREACH_NLRI
// that does not split still carries the error member beside the announce.
// PREVENTS: the fast path writing MP_REACH_NLRI, MP_UNREACH_NLRI and the body
// fields as separate members, so an UPDATE carrying two of them for one family
// stated the key twice and every map-based reader (bgp event parseFamilyOps,
// the ExaBGP bridge) kept only the last copy and lost the announce.
// RFC 7606 Section 5.1: "Since older BGP speakers may not implement these
// restrictions, an implementation MUST still be prepared to receive these
// fields in any position or combination." The test reads each key, so a
// repeated one fails it.
func TestOneFamilyKeyPerUpdate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		withdrawn []byte
		extra     []byte
		nlri      []byte
		family    string
		add, del  string
		delError  bool
	}{
		{
			name:   "mp_reach_ipv6_and_mp_unreach_ipv6",
			extra:  append(mpAttr(14, mergeReachV6), mpAttr(15, mergeUnreachV6)...),
			family: "ipv6/unicast", add: "2001:db8::/32", del: "2001:db8:1::/48",
		},
		{
			name:   "body_ipv4_and_mp_unreach_ipv4",
			extra:  mpAttr(15, mergeUnreachV4),
			nlri:   []byte{24, 10, 0, 0},
			family: "ipv4/unicast", add: "10.0.0.0/24", del: "10.1.0.0/16",
		},
		{
			name:      "mp_reach_ipv4_and_body_withdrawn",
			withdrawn: []byte{16, 10, 2},
			extra:     mpAttr(14, mergeReachV4),
			family:    "ipv4/unicast", add: "10.0.0.0/24", del: "10.2.0.0/16",
		},
		{
			name:   "mp_reach_ipv6_and_unsplittable_mp_unreach_ipv6",
			extra:  append(mpAttr(14, mergeReachV6), mpAttr(15, splitErrUnreachV6)...),
			family: "ipv6/unicast", add: "2001:db8::/32", delError: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := updateJSON(t, tt.withdrawn, tt.extra, tt.nlri)
			families := nlriFamilies(t, out)
			if len(families[tt.family]) != 2 {
				t.Fatalf("family %q holds %d operations, want 2:\n%s", tt.family, len(families[tt.family]), out)
			}
			add := operationOf(t, families, tt.family, "add", out)
			if !strings.Contains(stringOf(add["nlri"]), tt.add) {
				t.Fatalf("the add operation lacks %s:\n%s", tt.add, out)
			}
			del := operationOf(t, families, tt.family, "del", out)
			if tt.delError {
				if del["error"] == nil || del["parsed"] != false || del["raw"] != "40FDAB" {
					t.Fatalf("the del operation lacks the error member:\n%s", out)
				}
				return
			}
			if !strings.Contains(stringOf(del["nlri"]), tt.del) {
				t.Fatalf("the del operation lacks %s:\n%s", tt.del, out)
			}
		})
	}
}

// stringOf renders a decoded JSON value for a containment check.
func stringOf(v any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// TestBodyIPv4SplitErrorIsRendered renders UPDATEs whose body NLRI or
// Withdrawn Routes field is malformed through the JSON fast path.
//
// VALIDATES: the ipv4/unicast operation carries "error", "parsed":false and
// the whole field as uppercase "raw", in place of its nlri list, the same
// convention as the MP arms.
// PREVENTS: appendIPv4PrefixesFromWire stopping at the first malformed prefix
// with no word, so the event listed only the routes before it (or none) and
// read as a complete announce or withdrawal.
// RFC 4271 Section 4.3 defines the field: "Network Layer Reachability
// Information: This variable length field contains a list of IP address
// prefixes." A list cut short is not that list.
func TestBodyIPv4SplitErrorIsRendered(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		withdrawn []byte
		nlri      []byte
		action    string
		raw       string
	}{
		// A /24 with two of its three octets.
		{name: "nlri_truncated", nlri: []byte{24, 0x0a, 0x0b}, action: "add", raw: "180A0B"},
		// 10.0.0.0/8, then a /24 with one octet: the first route is no answer.
		{name: "nlri_truncated_after_a_route", nlri: []byte{8, 0x0a, 24, 0x0b}, action: "add", raw: "080A180B"},
		// A prefix length past 32.
		{name: "nlri_prefix_too_long", nlri: []byte{33, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}, action: "add", raw: "210A0B0C0D0E"},
		{name: "withdrawn_truncated", withdrawn: []byte{24, 0x0a, 0x0b}, action: "del", raw: "180A0B"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := updateJSON(t, tt.withdrawn, nil, tt.nlri)
			families := nlriFamilies(t, out)
			operation := operationOf(t, families, "ipv4/unicast", tt.action, out)
			if _, listed := operation["nlri"]; listed {
				t.Fatalf("the operation lists routes for a field that did not split:\n%s", out)
			}
			if operation["error"] == nil || operation["parsed"] != false || operation["raw"] != tt.raw {
				t.Fatalf("the operation lacks the error member with raw %s:\n%s", tt.raw, out)
			}
		})
	}
}

// TestBodyIPv4AddPathShortRead walks ADD-PATH body fields that end inside a
// Path Identifier, or right after one, through appendIPv4PrefixesFromWire.
//
// VALIDATES: both short reads return nlri.ErrShortRead, worded with the offset
// of the entry that broke, as wireu.ParseNLRIs words it.
// PREVENTS: the walk stopping at the cut with no error, so the routes before it
// read as the whole field.
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets." An entry holds the four octets and a length.
func TestBodyIPv4AddPathShortRead(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
		want string
	}{
		// Three of the four Path Identifier octets.
		{name: "path_id_cut", data: []byte{0, 0, 7}, want: "parsing NLRI at offset 0"},
		// Path 7 10.0.0.0/8, then two Path Identifier octets.
		{name: "path_id_cut_after_a_route", data: []byte{0, 0, 0, 7, 8, 10, 0, 0}, want: "parsing NLRI at offset 6"},
		// A whole Path Identifier and no prefix length.
		{name: "length_missing", data: []byte{0, 0, 0, 7}, want: "parsing NLRI at offset 0"},
		// Path 7 10.0.0.0/8, then a whole Path Identifier and no prefix length.
		{name: "length_missing_after_a_route", data: []byte{0, 0, 0, 7, 8, 10, 0, 0, 0, 9}, want: "parsing NLRI at offset 6"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := appendIPv4PrefixesFromWire(nil, tt.data, true)
			if err == nil {
				t.Fatalf("appendIPv4PrefixesFromWire(% x) returned no error", tt.data)
			}
			if !errors.Is(err, nlri.ErrShortRead) {
				t.Fatalf("appendIPv4PrefixesFromWire(% x) = %v, want nlri.ErrShortRead", tt.data, err)
			}
			if !strings.HasPrefix(err.Error(), tt.want) {
				t.Fatalf("appendIPv4PrefixesFromWire(% x) = %q, want it to start %q", tt.data, err, tt.want)
			}
		})
	}
}
