// Design: docs/architecture/ike/ipsec-14-responder.md -- authenticator EAP-TLS identity wiring
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- peer EAP-TLS server-identity wiring
// RFC: rfc/short/rfc5216.md -- Section 5.3, peer and server identity authorization
//
// VALIDATES: validateIPsecSections enforces ValidateEAPTLSIdentities after
// ValidatePKIRefs; eapTLSServerConfig forwards AllowedIdentities and fails
// closed when empty; buildPeerTLSConfig forwards explicit server-identity or
// unambiguous FQDN remote-address and fails closed with code
// doctor-ipsec-eap-tls-identity when neither exists; resumptionFor invalidates
// tickets on identity policy edits; recordEAPTLSAuthentication logs peer-id /
// server-id.
// PREVENTS: identity authorization existing in internal/core/eap while config
// validation or engine builders fail to wire or enforce it.

package engine

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	wiringCertName   = "crl-cert"
	wiringCAName     = "crl-ca"
	wiringServerFQDN = "vpn.example.com"
	wiringRemoteIP   = "192.0.2.1"
)

// TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints verifies that
// validateIPsecSections (used by ze config validate, commit, and reload) runs
// ValidateEAPTLSIdentities after ValidatePKIRefs.
func TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints(t *testing.T) {
	t.Run("respond peer without allowed-identities is refused", func(t *testing.T) {
		sections := vpnSection(`{
		  "vpn": {
		    "ipsec": {
		      "site-to-site": {
		        "peer": {
		          "branch": {
		            "connection-type": "respond",
		            "authentication": {
		              "mode": "eap-tls",
		              "certificate": "server",
		              "ca-certificate": "corp-ca"
		            }
		          }
		        }
		      }
		    }
		  }
		}`)
		err := validateIPsecSections(sections)
		if err == nil {
			t.Fatal("validateIPsecSections accepted a respond eap-tls peer without allowed-identities")
		}
		if !strings.Contains(err.Error(), "allowed-identities") {
			t.Errorf("error %q does not name allowed-identities", err)
		}
	})

	t.Run("initiate peer targeting IP without server-identity is refused", func(t *testing.T) {
		sections := vpnSection(`{
		  "vpn": {
		    "ipsec": {
		      "site-to-site": {
		        "peer": {
		          "hq": {
		            "connection-type": "initiate",
		            "remote-address": "192.0.2.1",
		            "authentication": {
		              "mode": "eap-tls",
		              "certificate": "client",
		              "ca-certificate": "corp-ca"
		            }
		          }
		        }
		      }
		    }
		  }
		}`)
		err := validateIPsecSections(sections)
		if err == nil {
			t.Fatal("validateIPsecSections accepted an initiate eap-tls peer with IP remote-address and no server-identity")
		}
		if !strings.Contains(err.Error(), "server-identity") {
			t.Errorf("error %q does not name server-identity", err)
		}
	})

	t.Run("missing ca-certificate still reports ValidatePKIRefs before ValidateEAPTLSIdentities", func(t *testing.T) {
		sections := vpnSection(`{
		  "vpn": {
		    "ipsec": {
		      "site-to-site": {
		        "peer": {
		          "branch": {
		            "connection-type": "respond",
		            "authentication": {
		              "mode": "eap-tls",
		              "certificate": "server"
		            }
		          }
		        }
		      }
		    }
		  }
		}`)
		err := validateIPsecSections(sections)
		if err == nil {
			t.Fatal("validateIPsecSections accepted an eap-tls peer without ca-certificate")
		}
		if !strings.Contains(err.Error(), "ca-certificate") {
			t.Errorf("error %q does not report missing ca-certificate first", err)
		}
	})
}

// TestEAPTLSServerConfigWiresAllowedIdentities verifies that eapTLSServerConfig
// forwards AllowedIdentities, fails closed when empty, resumptionFor invalidates
// on edits, and recordEAPTLSAuthentication logs peer-id.
func TestEAPTLSServerConfigWiresAllowedIdentities(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "id-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "id-wiring-server", 10, ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, [][]byte{crlWiringList(t, ca, caKey, 999)})

	store := eap.NewResumption(time.Now, true)
	sa := &SA{
		PeerName: "branch",
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionRespond,
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
				Additions: &ipsec.AuthAdditions{
					AllowedIdentities: []string{"client.example.com", "@corp.example"},
				},
			},
		},
		Resumption: store,
	}

	serverCfg, err := eapTLSServerConfig(sa)
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	want := []string{"client.example.com", "@corp.example"}
	if !slices.Equal(serverCfg.AllowedIdentities, want) {
		t.Fatalf("AllowedIdentities = %v, want %v", serverCfg.AllowedIdentities, want)
	}

	// Empty allowed-identities must fail closed at runtime too.
	emptySA := &SA{
		PeerName: "branch-empty",
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionRespond,
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
			},
		},
		Resumption: store,
	}
	if _, err := eapTLSServerConfig(emptySA); err == nil {
		t.Fatal("eapTLSServerConfig accepted an EAP-TLS responder peer with empty allowed-identities")
	}

	// Editing allowed-identities must invalidate the resumption store (RFC 9190 Section 5.7).
	const peerName = "identity-resumption-responder"
	t.Cleanup(func() { forgetResumption(peerName) })
	before := resumptionFor(peerName, sa.PeerCfg.Auth)
	edited := sa.PeerCfg.Auth
	edited.Additions = &ipsec.AuthAdditions{AllowedIdentities: []string{"other.example.com"}}
	if after := resumptionFor(peerName, edited); after == before {
		t.Fatal("changing allowed-identities kept the same resumption store")
	}

	// recordEAPTLSAuthentication logs peer-id on responder SAs.
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	recordEAPTLSAuthentication(sa, false, "client.example.com", log)
	if !strings.Contains(buf.String(), "peer-id=client.example.com") {
		t.Errorf("responder log %q does not contain peer-id=client.example.com", buf.String())
	}
}

// TestBuildPeerTLSConfigWiresServerIdentity verifies that buildPeerTLSConfig
// forwards explicit server-identity or unambiguous FQDN remote-address, fails
// closed with code doctor-ipsec-eap-tls-identity when unconstrained,
// resumptionFor invalidates on edits, and recordEAPTLSAuthentication logs server-id.
func TestBuildPeerTLSConfigWiresServerIdentity(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "id-wiring-peer-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "id-wiring-client", 11, ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, [][]byte{crlWiringList(t, ca, caKey, 999)})

	store := eap.NewResumption(time.Now, true)
	explicitSA := &SA{
		PeerName:    "hq-explicit",
		IsInitiator: true,
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionInitiate,
			RemoteAddress:  wiringRemoteIP,
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
				Additions: &ipsec.AuthAdditions{
					ServerIdentity: wiringServerFQDN,
				},
			},
		},
		Resumption: store,
	}

	peerCfg := buildPeerTLSConfig(explicitSA, slogutil.DiscardLogger())
	if peerCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for explicit server-identity")
	}
	if peerCfg.ServerIdentity != wiringServerFQDN {
		t.Fatalf("ServerIdentity = %q, want %q", peerCfg.ServerIdentity, wiringServerFQDN)
	}

	// Explicit server-identity overrides an FQDN remote-address.
	overrideSA := &SA{
		PeerName:    "hq-override",
		IsInitiator: true,
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionInitiate,
			RemoteAddress:  "gw.example.com",
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
				Additions: &ipsec.AuthAdditions{
					ServerIdentity: wiringServerFQDN,
				},
			},
		},
		Resumption: store,
	}
	overrideCfg := buildPeerTLSConfig(overrideSA, slogutil.DiscardLogger())
	if overrideCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil when server-identity overrides FQDN remote-address")
	}
	if overrideCfg.ServerIdentity != wiringServerFQDN {
		t.Fatalf("override ServerIdentity = %q, want %q", overrideCfg.ServerIdentity, wiringServerFQDN)
	}

	// Unambiguous FQDN remote-address acts as implicit server-identity fallback.
	fqdnSA := &SA{
		PeerName:    "hq-fqdn",
		IsInitiator: true,
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionInitiate,
			RemoteAddress:  wiringServerFQDN,
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
			},
		},
		Resumption: store,
	}
	fqdnCfg := buildPeerTLSConfig(fqdnSA, slogutil.DiscardLogger())
	if fqdnCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for unambiguous FQDN remote-address")
	}
	if fqdnCfg.ServerIdentity != wiringServerFQDN {
		t.Fatalf("FQDN fallback ServerIdentity = %q, want %q", fqdnCfg.ServerIdentity, wiringServerFQDN)
	}

	// Unconstrained IP peer must fail closed and log code=doctor-ipsec-eap-tls-identity.
	unconstrainedSA := &SA{
		PeerName:    "hq-ip-only",
		IsInitiator: true,
		PeerCfg: ipsec.SiteToSitePeer{
			ConnectionType: ipsec.ConnectionInitiate,
			RemoteAddress:  wiringRemoteIP,
			Auth: ipsec.AuthConfig{
				Mode:              ipsec.AuthEAPTLS,
				Certificate:       wiringCertName,
				CACertificate:     wiringCAName,
				SessionResumption: true,
			},
		},
		Resumption: store,
	}
	var warnBuf bytes.Buffer
	warnLog := slog.New(slog.NewTextHandler(&warnBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if cfg := buildPeerTLSConfig(unconstrainedSA, warnLog); cfg != nil {
		t.Fatal("buildPeerTLSConfig accepted an unconstrained IP EAP-TLS peer with no server-identity")
	}
	if !strings.Contains(warnBuf.String(), "code="+codeDoctorIPsecEAPTLSIdentity) {
		t.Errorf("warn log %q does not carry code=%s", warnBuf.String(), codeDoctorIPsecEAPTLSIdentity)
	}

	// Editing server-identity must invalidate the resumption store.
	const peerName = "identity-resumption-initiator"
	t.Cleanup(func() { forgetResumption(peerName) })
	before := resumptionFor(peerName, explicitSA.PeerCfg.Auth)
	edited := explicitSA.PeerCfg.Auth
	edited.Additions = &ipsec.AuthAdditions{ServerIdentity: "vpn2.example.com"}
	if after := resumptionFor(peerName, edited); after == before {
		t.Fatal("changing server-identity kept the same resumption store")
	}

	// recordEAPTLSAuthentication logs server-id on initiator SAs.
	var infoBuf bytes.Buffer
	infoLog := slog.New(slog.NewTextHandler(&infoBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	recordEAPTLSAuthentication(explicitSA, false, wiringServerFQDN, infoLog)
	if !strings.Contains(infoBuf.String(), "server-id="+wiringServerFQDN) {
		t.Errorf("initiator log %q does not contain server-id=%s", infoBuf.String(), wiringServerFQDN)
	}
}
