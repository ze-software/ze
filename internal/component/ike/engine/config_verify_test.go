package engine

import (
	"strings"
	"testing"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// vpnSection wraps a JSON body as the "vpn" config section the plugin receives.
func vpnSection(body string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "vpn", Data: body}}
}

// VALIDATES: AC-8 -- ValidateIPsecSections, the plugin's OnConfigVerify body,
// rejects an EAP-TLS peer that names no ca-certificate.
// PREVENTS: The check existing in ipsec.ValidatePKIRefs but never running. Before
// this wiring, ValidatePKIRefs, ValidateGroupRefs and ValidateRemoteAccess had no
// non-test caller anywhere in the repo, so every cross-reference they describe
// was unenforced (ai/rules/wiring-completeness.md).
func TestValidateIPsecSectionsRejectsEAPTLSWithoutCA(t *testing.T) {
	sections := vpnSection(`{
	  "vpn": {
	  "ipsec": {
	    "site-to-site": {
	      "peer": {
	        "branch": {
	          "authentication": {"mode": "eap-tls", "certificate": "client"}
	        }
	      }
	    }
	  }
	  }
	}`)

	err := ValidateIPsecSections(sections)

	if err == nil {
		t.Fatal("an eap-tls peer with no ca-certificate passed config verification")
	}
	if !strings.Contains(err.Error(), "ca-certificate") {
		t.Errorf("error %q does not explain the missing ca-certificate", err)
	}
}

// VALIDATES: AC-8 -- the group-reference validator now runs too.
// PREVENTS: Wiring only the PKI validator and leaving the other two orphaned.
func TestValidateIPsecSectionsRejectsUnknownGroupRef(t *testing.T) {
	sections := vpnSection(`{
	  "vpn": {
	  "ipsec": {
	    "site-to-site": {
	      "peer": {
	        "branch": {
	          "ike-group": "no-such-group",
	          "authentication": {"mode": "pre-shared-secret", "pre-shared-secret": "s3cret"}
	        }
	      }
	    }
	  }
	  }
	}`)

	err := ValidateIPsecSections(sections)

	if err == nil {
		t.Fatal("a peer naming an undefined ike-group passed config verification")
	}
	if !strings.Contains(err.Error(), "no-such-group") {
		t.Errorf("error %q does not name the undefined group", err)
	}
}

// VALIDATES: AC-8 -- a config with no cross-reference problems verifies clean.
// PREVENTS: The new gate rejecting valid configs, which would be worse than the
// gap it closes.
func TestValidateIPsecSectionsAcceptsConsistentConfig(t *testing.T) {
	sections := vpnSection(`{
	  "vpn": {
	  "ipsec": {
	    "site-to-site": {
	      "peer": {
	        "branch": {
	          "authentication": {"mode": "pre-shared-secret", "pre-shared-secret": "s3cret"}
	        }
	      }
	    }
	  }
	  }
	}`)

	if err := ValidateIPsecSections(sections); err != nil {
		t.Fatalf("a consistent config was rejected: %v", err)
	}
}

// VALIDATES: AC-8 -- an empty delivery verifies clean.
// PREVENTS: The gate failing when the vpn section is absent, which would break
// every config that does not use IPsec at all.
func TestValidateIPsecSectionsAcceptsNoVPNSection(t *testing.T) {
	if err := ValidateIPsecSections(nil); err != nil {
		t.Fatalf("an empty config delivery was rejected: %v", err)
	}
}
