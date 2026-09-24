package rpki

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ze-software/ze/internal/component/pki"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Candidate verification must neither adopt a new trust anchor nor poison the
// running client when a candidate references an unusable identity. Each check
// finishes with a real handshake under the pre-existing live PKI generation.
func TestRTRTLSCandidatePKIIsIsolated(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprintf("rejected=%t", reject), func(t *testing.T) {
			live := newRTRTLSFixture(t)
			candidate := newRTRTLSFixture(t)
			installRTRTLSPKI(t, live.store)
			peer := startRTRTLSPeer(t, candidate.serverConfig(tls.VersionTLS13))
			name := "router"
			if reject {
				name = "absent-router"
			}
			sections := []sdk.ConfigSection{
				{Root: configRootBGP, Data: fmt.Sprintf(`{"bgp":{"rpki":{"cache-server":{"127.0.0.1":{"port":"%d","tls":{"ca-certificate":"cache-ca","certificate":%q,"server-name":"cache.rtr.test"}}}}}}`, peer.port, name)},
				{Root: configRootPKI, Data: rtrPKISection(t, candidate.store)},
			}
			cfg, err := parseRPKISections(sections, nil)
			if reject {
				if err == nil {
					t.Fatal("candidate referencing a missing router identity was accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				server := cfg.CacheServers[0]
				session := newTestRTRSession(t, server.Address, server.Port, server.Preference, server.SourceAddress,
					newROACache(), newASPACache(), make(chan struct{}))
				session.tlsSettings, session.pkiConfig = server.TLS, cfg.pkiConfig
				if err := session.syncOnce(); err != nil {
					t.Fatalf("authenticate with candidate identity and intermediate chain: %v", err)
				}
				if session.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
					t.Fatal("candidate PKI could not authenticate usable RTR data")
				}
			}
			peer.stop()
			livePeer := startRTRTLSPeer(t, live.serverConfig(tls.VersionTLS12))
			liveSession := parsedRTRTLSSession(t, "127.0.0.1", livePeer.port, rtrTLSConfigLeaves(), make(chan struct{}))
			if err := liveSession.syncOnce(); err != nil {
				t.Fatalf("candidate verification changed live PKI: %v", err)
			}
			livePeer.stop()
			if liveSession.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
				t.Fatal("live PKI no longer authenticates the original cache")
			}
		})
	}
}

func rtrPKISection(t *testing.T, store *pki.PKIConfig) string {
	t.Helper()
	cas := make(map[string]any, len(store.CACerts))
	for name, ca := range store.CACerts {
		cas[name] = map[string]any{"certificate": base64.StdEncoding.EncodeToString(ca.Raw)}
	}
	certificates := make(map[string]any, len(store.Certificates))
	for name, certificate := range store.Certificates {
		key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		intermediates := make([]string, len(certificate.RawIntermediates))
		for i, intermediate := range certificate.RawIntermediates {
			intermediates[i] = base64.StdEncoding.EncodeToString(intermediate)
		}
		certificates[name] = map[string]any{
			"certificate":  base64.StdEncoding.EncodeToString(certificate.Raw),
			"intermediate": intermediates,
			"private":      map[string]any{"key": base64.StdEncoding.EncodeToString(key)},
		}
	}
	data, err := json.Marshal(map[string]any{"pki": map[string]any{"ca": cas, "certificate": certificates}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
