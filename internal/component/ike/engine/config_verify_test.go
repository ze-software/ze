package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/pki"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// vpnSection wraps a JSON body as the "vpn" config section the plugin receives.
func vpnSection(body string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "vpn", Data: body}}
}

// VALIDATES: AC-8 -- validateIPsecSections, the plugin's OnConfigVerify body,
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

	err := validateIPsecSections(sections)

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

	err := validateIPsecSections(sections)

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

	if err := validateIPsecSections(sections); err != nil {
		t.Fatalf("a consistent config was rejected: %v", err)
	}
}

// VALIDATES: AC-8 -- an empty delivery verifies clean.
// PREVENTS: The gate failing when the vpn section is absent, which would break
// every config that does not use IPsec at all.
func TestValidateIPsecSectionsAcceptsNoVPNSection(t *testing.T) {
	if err := validateIPsecSections(nil); err != nil {
		t.Fatalf("an empty config delivery was rejected: %v", err)
	}
}

// testCADER returns a real self-signed CA certificate as base64 DER, which is
// the form pki.ParseConfig expects for a `ca` entry's `certificate` leaf
// (internal/component/pki/config.go parseCACert). A fake value would fail to
// parse, and the side-effect test below would then pass vacuously: pki.Load is
// only reached when the section parses.
func testCADER(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "verify-probe-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

// VALIDATES: config verification is side-effect free -- it must not install the
// candidate PKI into the live process-wide store.
// PREVENTS: A REJECTED commit leaving the daemon holding the config it refused.
// pki.Load swaps the whole store (internal/component/pki/store.go Load), so
// verifying through parseIPsecSections would adopt the candidate's certificates
// and then abort the transaction: a live tunnel whose CA was renamed in the
// rejected config would fail its next rekey, because buildPeerTLSConfig could no
// longer resolve the old name. The InProcessConfigVerifier contract in
// internal/component/plugin/registry/registry.go states the same requirement.
func TestValidateIPsecSectionsDoesNotMutatePKIStore(t *testing.T) {
	if pki.GetCA("verify-probe-ca") != nil {
		t.Fatal("test premise broken: verify-probe-ca is already in the live store")
	}

	sections := []sdk.ConfigSection{
		{Root: "pki", Data: `{"ca": {"verify-probe-ca": {"certificate": "` + testCADER(t) + `"}}}`},
		vpnSection(`{
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
		}`)[0],
	}

	// The vpn half is invalid (eap-tls with no ca-certificate), so this rejects.
	// The pki half is fully valid, so a side-effecting verify WOULD install it.
	if err := validateIPsecSections(sections); err == nil {
		t.Fatal("test premise broken: the candidate config was expected to be rejected")
	}

	if pki.GetCA("verify-probe-ca") != nil {
		t.Error("verification installed the candidate PKI into the live store: " +
			"a rejected commit must leave the running daemon untouched")
	}
}

// VALIDATES: certificate names are resolved against the CANDIDATE pki section,
// not the live store, so a config defining its own CA verifies clean.
// PREVENTS: Judging a new config by the old PKI, which would reject every commit
// that introduces a certificate and its first user together.
func TestValidateIPsecSectionsResolvesAgainstCandidatePKI(t *testing.T) {
	ca := testCADER(t)

	good := []sdk.ConfigSection{
		{Root: "pki", Data: `{"ca": {"corp-ca": {"certificate": "` + ca + `"}}}`},
		vpnSection(`{
		  "vpn": {
		  "ipsec": {
		    "site-to-site": {
		      "peer": {
		        "branch": {
		          "authentication": {"mode": "eap-tls", "ca-certificate": "corp-ca"}
		        }
		      }
		    }
		  }
		  }
		}`)[0],
	}
	if err := validateIPsecSections(good); err != nil {
		t.Fatalf("a peer whose CA is defined in the same delivery was rejected: %v", err)
	}

	bad := []sdk.ConfigSection{
		good[0],
		vpnSection(`{
		  "vpn": {
		  "ipsec": {
		    "site-to-site": {
		      "peer": {
		        "branch": {
		          "authentication": {"mode": "eap-tls", "ca-certificate": "nowhere-ca"}
		        }
		      }
		    }
		  }
		  }
		}`)[0],
	}
	err := validateIPsecSections(bad)
	if err == nil {
		t.Fatal("a ca-certificate absent from the candidate pki section was accepted")
	}
	if !strings.Contains(err.Error(), "nowhere-ca") {
		t.Errorf("error %q does not name the unresolvable CA", err)
	}
}
