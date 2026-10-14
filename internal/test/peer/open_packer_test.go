// Tests that ze-peer's passive-role OPEN (encodeOpen) is written by the one
// Optional Parameters packer and the one OPEN writer.
//
// VALIDATES: AC-1p of spec-bgp-open-session-rfc-defects: encodeOpen's bytes
// equal the bytes its own framing wrote before it moved onto
// capability.PackOptionalParams and message.Open.WriteTo, for one type-2
// parameter, several type-2 parameters beside a raw parameter, and parameters
// above 255 octets; the 4096-octet OPEN refusal is kept, and parameters above
// 65535 octets get the packer's size error.
// PREVENTS: a second Optional Parameters packer or envelope writer in the test
// peer drifting from the one ze sends with.

package peer

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// passiveOpenGoldenExtended is encodeOpen's output for the "parameters above
// 255 octets" case, captured from encodeOpen at commit a659c76398, before it
// moved onto the packer.
const passiveOpenGoldenExtended = "ffffffffffffffffffffffffffffffff022201045ba0ffff01020305ffff02020200fcc8fa000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200fcc9fa0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007000101"

// bigCapabilityParams is n type-2 parameters, each one 252-octet capability TLV.
func bigCapabilityParams(n int) []openParam {
	params := make([]openParam, 0, n)
	for i := range n {
		params = append(params, openParam{kind: paramTypeCapability, caps: [][]byte{capTLV(byte(200+i%50), make([]byte, 250)...)}})
	}
	return params
}

// TestPassiveOpenUsesOnePacker compares encodeOpen's bytes with goldens
// captured from encodeOpen's own framing, before the move onto the one packer.
// Each golden was read from HEAD a659c76398 by running encodeOpen over the same
// list, so a byte that moved is a byte the move changed.
func TestPassiveOpenUsesOnePacker(t *testing.T) {
	cases := []struct {
		name   string
		params []openParam
		golden string
	}{
		{
			name:   "one type-2 parameter",
			params: []openParam{{kind: paramTypeCapability, caps: [][]byte{capTLV(1, 0, 1, 0, 1), capTLV(65, 0, 0, 0xfd, 0xe8)}}},
			golden: "ffffffffffffffffffffffffffffffff002b01045ba0ffff010203050e020c01040001000141040000fde8",
		},
		{
			name: "several type-2 parameters and a raw parameter",
			params: []openParam{
				{kind: paramTypeCapability, caps: [][]byte{capTLV(1, 0, 1, 0, 1)}},
				{kind: 7, raw: []byte{0xde, 0xad, 0xbe}},
				{kind: paramTypeCapability, caps: [][]byte{capTLV(2), capTLV(65, 0, 0, 0xfd, 0xe8)}},
				{kind: paramTypeCapability, caps: [][]byte{capTLV(unknownCapabilityCode, []byte(unknownCapabilityValue)...)}},
			},
			golden: "ffffffffffffffffffffffffffffffff004201045ba0ffff010203052502060104000100010703deadbe0208020041040000fde8020c420a6c6f72656d697073756d",
		},
		{
			name:   "parameters above 255 octets",
			params: append(bigCapabilityParams(2), openParam{kind: 7, raw: []byte{0x01}}),
			golden: passiveOpenGoldenExtended,
		},
		{
			name:   "no parameter",
			params: nil,
			golden: "ffffffffffffffffffffffffffffffff001d01045ba0ffff0102030500",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			open, err := encodeOpen([]byte{4}, newOpenIdentity(4200000000, 0x01020305), tc.params)
			if err != nil {
				t.Fatalf("encodeOpen: %v", err)
			}
			if got := hex.EncodeToString(open); got != tc.golden {
				t.Fatalf("OPEN bytes moved:\n got  %s\n want %s", got, tc.golden)
			}
		})
	}
}

// TestPassiveOpenRefusesBySize proves each oversized list gets its own error.
// Seventeen 255-octet parameters fit the packer (4335 octets) and break the
// 4096-octet OPEN, so the RFC 4271 refusal answers; 260 of them (66300 octets)
// exceed what the RFC 9072 two-octet length states, so the packer answers
// before any OPEN is sized.
func TestPassiveOpenRefusesBySize(t *testing.T) {
	_, err := encodeOpen([]byte{4}, newOpenIdentity(65001, 1), bigCapabilityParams(17))
	if err == nil {
		t.Fatal("an OPEN above 4096 octets must be refused")
	}
	if !strings.Contains(err.Error(), "4096") {
		t.Fatalf("the refusal names the 4096-octet ceiling: %v", err)
	}
	if errors.Is(err, capability.ErrOptionalParamsTooLarge) {
		t.Fatalf("parameters under 65535 octets are not the packer's refusal: %v", err)
	}

	_, err = encodeOpen([]byte{4}, newOpenIdentity(65001, 1), bigCapabilityParams(260))
	if !errors.Is(err, capability.ErrOptionalParamsTooLarge) {
		t.Fatalf("parameters above 65535 octets get the packer's size error, got %v", err)
	}
}
