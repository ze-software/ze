package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// retryTestIKEGroup is an IKE group offering two Diffie-Hellman groups, so a test can
// name one Ze proposed and one it did not.
func retryTestIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "ike-retry",
		Proposals: []ipsec.IKEProposal{
			{Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256, DHGroup: 14},
			{Number: 2, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA256, DHGroup: 19},
		},
	}
}

// VALIDATES: the INVALID_KE_PAYLOAD body is read as exactly two octets, big endian.
// PREVENTS: a one-octet read that would turn group 4096 into group 0, and a body of any
// other length being accepted (RFC 7296 Section 1.3, "two octets of data ... the
// accepted Diffie-Hellman group number in big endian order").
func TestParseInvalidKEGroupReadsTwoOctetsBigEndian(t *testing.T) {
	g, ok := parseInvalidKEGroup([]byte{0x00, 0x13})
	if !ok {
		t.Fatal("a well-formed two-octet body was refused")
	}
	if g != 19 {
		t.Errorf("group = %d, want 19; the octets were read in the wrong order", g)
	}
}

// VALIDATES: every malformed or out-of-range body is refused, and the caller cannot
// mistake a refusal for group 0.
// PREVENTS: a forged notify steering Ze onto a group it cannot build, and the zero-value
// trap where a truncated body parses as "no DH" (ai/rules/evidence.md).
func TestParseInvalidKEGroupFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"one octet", []byte{0x0e}},
		{"three octets", []byte{0x00, 0x0e, 0x00}},
		{"group zero", []byte{0x00, 0x00}},
		{"above the uint8 range", []byte{0x10, 0x00}},
		{"above the valid range", []byte{0x00, 0xff}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if g, ok := parseInvalidKEGroup(tc.data); ok {
				t.Errorf("parseInvalidKEGroup accepted %s and returned group %d", tc.name, g)
			}
		})
	}
}

// VALIDATES: a group is accepted only when this node proposed it.
// PREVENTS: the security hole the RFC leaves implicit -- the notify is unauthenticated,
// so without this check a forged one picks Ze's Diffie-Hellman group.
func TestGroupIsProposedOnlyForConfiguredGroups(t *testing.T) {
	g := retryTestIKEGroup()
	for _, want := range []ipsec.DHGroup{14, 19} {
		if !groupIsProposed(g, want) {
			t.Errorf("group %d was configured but read as unproposed", want)
		}
	}
	for _, unwanted := range []ipsec.DHGroup{1, 2, 5, 20, 31} {
		if groupIsProposed(g, unwanted) {
			t.Errorf("group %d was never proposed but read as proposed", unwanted)
		}
	}
}

// VALIDATES: the retry cause is a typed value whose zero is invalid.
// PREVENTS: an unset cause reading as a real one and driving a retry nobody asked for
// (ai/rules/go-standards.md).
func TestRetryCauseZeroIsInvalid(t *testing.T) {
	var zero retryCause
	if zero != retryCauseNone {
		t.Fatal("the zero retryCause must be the invalid one")
	}
	if zero.String() != "unset" {
		t.Errorf("the zero cause names itself %q, which reads like a real cause", zero.String())
	}
	if retryCookie.String() == retryInvalidKE.String() {
		t.Error("the two real causes share a name, so a metric label cannot tell them apart")
	}
}
