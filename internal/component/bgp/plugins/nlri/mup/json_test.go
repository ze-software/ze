package mup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The JSON each fixture NLRI owes, copied from the `1:json:` expectation in
// test/exabgp-compat/encoding/conf-srv6-mup.ci and conf-srv6-mup-v3.ci with the
// members in the alphabetical order json.Marshal writes a map in. The `rd`
// member keeps the RFC 4364 type ze states; the ExaBGP bridge trims it
// (exabgpRD, internal/exabgp/bridge/bridge_event.go).
const (
	jsonISDv4 = `{"arch":1,"code":1,"name":"InterworkSegmentDiscoveryRoute",` +
		`"prefix_ip":"10.0.1.0","prefix_ip_len":24,` +
		`"raw":"0100010C0000006400000064180A0001","rd":"0:100:100"}`

	jsonDSDv4 = `{"arch":1,"code":2,"ip":"10.0.0.1","name":"DirectSegmentDiscoveryRoute",` +
		`"raw":"0100020C00000064000000640A000001","rd":"0:100:100"}`

	jsonT1STv4 = `{"arch":1,"code":3,"endpoint_ip":"10.0.0.1","endpoint_ip_len":32,` +
		`"name":"Type1SessionTransformedRoute","prefix_ip":"192.168.0.1","prefix_ip_len":32,` +
		`"qfi":"9","raw":"01000317000000640000006420C0A800010000303909200A000001",` +
		`"rd":"0:100:100","source_ip":"b''","source_ip_len":0,"teid":"12345"}`

	jsonT1STv4Source = `{"arch":1,"code":3,"endpoint_ip":"10.0.0.1","endpoint_ip_len":32,` +
		`"name":"Type1SessionTransformedRoute","prefix_ip":"192.168.0.2","prefix_ip_len":32,` +
		`"qfi":"9","raw":"0100031C000000640000006420C0A800020000303909200A000001200A000101",` +
		`"rd":"0:100:100","source_ip":"10.0.1.1","source_ip_len":32,"teid":"12345"}`

	jsonT2STv4 = `{"arch":1,"code":4,"endpoint_ip":"10.0.0.1","endpoint_len":64,` +
		`"name":"Type2SessionTransformedRoute",` +
		`"raw":"010004110000006400000064400A00000100003039","rd":"0:100:100","teid":"12345"}`

	jsonT2STv4NoTEID = `{"arch":1,"code":4,"endpoint_ip":"10.0.0.1","endpoint_len":32,` +
		`"name":"Type2SessionTransformedRoute",` +
		`"raw":"0100040D0000006400000064200A000001","rd":"0:100:100","teid":"0"}`

	jsonISDv6 = `{"arch":1,"code":1,"name":"InterworkSegmentDiscoveryRoute",` +
		`"prefix_ip":"2001::","prefix_ip_len":64,` +
		`"raw":"010001110000006400000064402001000000000000","rd":"0:100:100"}`

	jsonT1STv6 = `{"arch":1,"code":3,"endpoint_ip":"2001::1","endpoint_ip_len":128,` +
		`"name":"Type1SessionTransformedRoute","prefix_ip":"2001:db8:1:1::1","prefix_ip_len":128,` +
		`"qfi":"9","raw":"0100032F00000064000000648020010DB8000100010000000000000001000030390980` +
		`20010000000000000000000000000001","rd":"0:100:100","source_ip":"b''","source_ip_len":0,` +
		`"teid":"12345"}`

	jsonT2STv6 = `{"arch":1,"code":4,"endpoint_ip":"2001::1","endpoint_len":160,` +
		`"name":"Type2SessionTransformedRoute",` +
		`"raw":"0100041D0000006400000064A02001000000000000000000000000000100003039",` +
		`"rd":"0:100:100","teid":"12345"}`

	jsonUnknownType = `{"arch":1,"code":99,"parsed":false,"raw":"010063040A0B0C0D"}`
)

// mupJSONCases pairs each wire fixture with the JSON both writers owe for it.
var mupJSONCases = []struct {
	name string
	afi  AFI
	wire string
	want string
}{
	{"isd ipv4", AFIIPv4, wireISDv4, jsonISDv4},
	{"dsd ipv4", AFIIPv4, wireDSDv4, jsonDSDv4},
	{"t1st ipv4 without source", AFIIPv4, wireT1STv4, jsonT1STv4},
	{"t1st ipv4 with source", AFIIPv4, wireT1STv4Source, jsonT1STv4Source},
	{"t2st ipv4", AFIIPv4, wireT2STv4, jsonT2STv4},
	{"t2st ipv4 without teid", AFIIPv4, wireT2STv4NoTEID, jsonT2STv4NoTEID},
	{"isd ipv6", AFIIPv6, wireISDv6, jsonISDv6},
	{"t1st ipv6", AFIIPv6, wireT1STv6, jsonT1STv6},
	{"t2st ipv6", AFIIPv6, wireT2STv6, jsonT2STv6},
	{"unknown route type", AFIIPv4, wireUnknownType, jsonUnknownType},
}

// TestMUPAppendJSON verifies the in-process fast path renders every route type.
//
// VALIDATES: AppendJSON writes the members ExaBGP names for each route type,
// with the TEID and QFI as strings, the lengths as numbers, and raw as
// uppercase hex of the whole NLRI.
// PREVENTS: the regression this package carried until 2026-09-20, where every
// MUP route rendered as arch-type, route-type and rd, so no reader could tell
// one route from another.
func TestMUPAppendJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range mupJSONCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseHex(t, tc.afi, tc.wire).AppendJSON(nil)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// TestMUPDecodeNLRIHex verifies the registry path renders every route type.
//
// VALIDATES: DecodeNLRIHex returns a map whose marshaled form is the JSON the
// ExaBGP compatibility fixtures pin.
// PREVENTS: the RPC decoder and the in-process decoder reporting a MUP route
// differently, where a reader cannot tell which one served it.
func TestMUPDecodeNLRIHex(t *testing.T) {
	t.Parallel()
	for _, tc := range mupJSONCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			family := "ipv4/mup"
			if tc.afi == AFIIPv6 {
				family = "ipv6/mup"
			}
			decoded, err := DecodeNLRIHex(family, tc.wire, false)
			require.NoError(t, err)
			got, err := json.Marshal(decoded)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// TestMUPJSONPathsAgree verifies the two writers cannot drift.
//
// VALIDATES: AppendJSON writes byte for byte what json.Marshal writes for the
// map mupToJSON builds, for every route type.
// PREVENTS: a member added to one writer and forgotten in the other, which
// would make the route's JSON depend on whether the plugin ran in process.
func TestMUPJSONPathsAgree(t *testing.T) {
	t.Parallel()
	for _, tc := range mupJSONCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mup := parseHex(t, tc.afi, tc.wire)
			marshaled, err := json.Marshal(mupToJSON(mup))
			require.NoError(t, err)
			assert.Equal(t, string(marshaled), string(mup.AppendJSON(nil)))
		})
	}
}

// TestMUPDecodeNLRIHexRefusesMalformed verifies the decoder fails closed.
//
// VALIDATES: DecodeNLRIHex returns an error, and no object, for a body whose
// declared length disagrees with the octets present.
// PREVENTS: a truncated MUP route reaching a reader as a route with zero
// values, which reads exactly like a real one.
func TestMUPDecodeNLRIHexRefusesMalformed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		wire string
	}{
		{"truncated body", "01000110"},
		{"isd prefix shorter than its length", "0100010B0000006400000064180A00"},
		{"t1st truncated endpoint address", "01000316000000640000006420C0A800010000303909200A0000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			decoded, err := DecodeNLRIHex("ipv4/mup", tc.wire, false)
			assert.Error(t, err)
			assert.Nil(t, decoded)
		})
	}
}
