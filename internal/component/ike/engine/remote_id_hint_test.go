// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- remote identity policy
// RFC: rfc/short/rfc7296.md -- Identification payloads (Section 3.5)

package engine

import (
	"crypto/x509/pkix"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// VALIDATES: the class-mismatch hint names the class the operator actually configured,
// for every class remote-id can hold, including the distinguished name.
// PREVENTS: the two-arm hint telling a DN-valued remote-id that refused an asserted
// ID_FQDN that ID_FQDN is among the types it accepts. That sentence sends the operator
// to change the peer to assert exactly what was just refused.
func TestClassMismatchHintNamesTheConfiguredClass(t *testing.T) {
	for _, tc := range []struct {
		name     string
		want     string
		asserted uint8
		expect   string
		reject   string
	}{
		{
			name:     "address-valued remote-id",
			want:     "10.0.0.1",
			asserted: wire.IDTypeFQDN,
			expect:   "ID_IPV4_ADDR",
			reject:   "ID_FQDN, ID_RFC822_ADDR",
		},
		{
			name:     "DN-valued remote-id",
			want:     "CN=branch,O=Example",
			asserted: wire.IDTypeFQDN,
			expect:   "ID_DER_ASN1_DN",
			reject:   "ID_FQDN, ID_RFC822_ADDR",
		},
		{
			name:     "text-valued remote-id",
			want:     "branch.example.com",
			asserted: wire.IDTypeDERASN1DN,
			expect:   "ID_FQDN, ID_RFC822_ADDR",
			reject:   "ID_DER_ASN1_DN alone",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hint := classMismatchHint(tc.want, &wire.PayloadID{
				IDType: tc.asserted,
				IDData: []byte(tc.want),
			})
			if !strings.Contains(hint, tc.expect) {
				t.Errorf("the hint does not name %s, which is what remote-id accepts: %s",
					tc.expect, hint)
			}
			if strings.Contains(hint, tc.reject) {
				t.Errorf("the hint names %s, which remote-id does NOT accept, so it "+
					"points the operator at the type just refused: %s", tc.reject, hint)
			}
			if !strings.Contains(hint, idTypeName(tc.asserted)) {
				t.Errorf("the hint does not say what the peer asserted: %s", hint)
			}
		})
	}
}

// VALIDATES: ID_DER_ASN1_DN is reported as an identity ze can compare, which is what
// assertedIdentity's own switch does.
// PREVENTS: the doc comment's claim that ze compares five types and does not compare a
// DN drifting back into the code as a refusal.
func TestAssertedIdentityComparesDistinguishedName(t *testing.T) {
	dn, want := cfmDN(t, pkix.Name{CommonName: "branch", Organization: []string{"Example"}})
	text, comparable := assertedIdentity(&wire.PayloadID{
		IDType: wire.IDTypeDERASN1DN,
		IDData: dn,
	})
	if !comparable {
		t.Fatal("an ID_DER_ASN1_DN payload was reported as one ze cannot compare")
	}
	if text != want {
		t.Errorf("the rendered DN is %q and RFC 4514 gives %q", text, want)
	}
	// The rendering is what remote-id is compared against, so the two must agree.
	if !remoteIDMatches(want, &wire.PayloadID{IDType: wire.IDTypeDERASN1DN, IDData: dn}) {
		t.Error("the rendered DN does not match the remote-id an operator would copy from it")
	}
}
