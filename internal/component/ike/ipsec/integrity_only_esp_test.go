// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model types
// Related: algorithm_support.go -- EncryptionImplementedESP, the guard under test
package ipsec

import (
	"strings"
	"testing"
)

// VALIDATES: the operator can select integrity-only ESP for an IKE-keyed Child SA, and
// only where the RFCs allow it. Method: ParseIPsecConfig is driven with an esp-group
// naming null beside every implemented hash, then with null and no hash, then with an
// ike-group naming null.
// PREVENTS: a null cipher reaching an IKE SA (RFC 7296 Section 5: "implementations MUST
// NOT negotiate ... ENCR_NULL as the IKE encryption algorithm."), an ESP proposal with
// neither confidentiality nor integrity (RFC 4303 Section 3.2: "both algorithms MUST NOT
// be simultaneously NULL."), and an ESP vocabulary that cannot name integrity-only ESP.
func TestIntegrityOnlyESPConfig(t *testing.T) {
	for _, hash := range supportedHashNames() {
		cfg, err := ParseIPsecConfig(makeESPTree("ESP-NULL", "", "", map[string][2]string{"1": {"null", hash}}))
		if err != nil {
			t.Errorf("esp-group null + %s refused: %v", hash, err)
			continue
		}
		p := cfg.ESPGroups["ESP-NULL"].Proposals[0]
		if p.Encryption != EncryptionNull || p.Hash.String() != hash {
			t.Errorf("esp-group null + %s parsed as %s + %s", hash, p.Encryption, p.Hash)
		}
	}

	err := parseErr(t, makeESPTree("ESP-NULL", "", "", map[string][2]string{"1": {"null", ""}}))
	if !strings.Contains(err.Error(), "hash is required") {
		t.Errorf("esp-group null without a hash: error %q, want the missing hash named", err)
	}

	err = parseErr(t, makeIKETree("IKE-NULL", ikeOpts{proposals: map[string]ikeProposalOpts{
		"1": {encryption: "null", hash: "sha256", dhGroup: "14"},
	}}))
	if !strings.Contains(err.Error(), `"null"`) {
		t.Errorf("ike-group null: error %q, want the null cipher named", err)
	}
	if strings.Contains(err.Error(), "null,") || strings.HasSuffix(err.Error(), "null)") {
		t.Errorf("ike-group null: error %q lists null among the IKE ciphers", err)
	}
}
