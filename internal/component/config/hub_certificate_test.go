// VALIDATES: the two trust leaves parse and reach the structs the managed server and
// the managed client read -- plugin/hub/server/certificate and
// plugin/hub/client/certificate-fingerprint (spec-managed-server-hardening AC-1).
// PREVENTS: a leaf that validates in YANG and is dropped in extraction, which leaves
// the hub on a self-signed certificate and the client with no pin while the operator
// reads their configured value back from `show configuration`.

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var hubTrustConfig = `plugin {
    hub {
        server central {
            ip 10.0.0.1
            port 1790
            certificate fleet-hub
            client edge-01 {
                secret edge01-secret-that-is-at-least-32ch
            }
        }
        client edge-01 {
            host 10.0.0.1
            port 1790
            secret edge01-secret-that-is-at-least-32ch
            certificate-fingerprint ` + strings.Repeat("ab", 32) + `
        }
    }
}
`

// TestHubTrustLeavesReachTheStructs parses a config that names a hub certificate
// and pins its fingerprint, then reads both back through ExtractHubConfig.
//
// MUTATION: drop either assignment in extractHubServerConfig /
// extractHubClientConfig and the matching assertion fails on the zero value.
func TestHubTrustLeavesReachTheStructs(t *testing.T) {
	result, err := LoadConfig(hubTrustConfig, filepath.Join(t.TempDir(), "hub-trust.conf"), nil)
	require.NoError(t, err)

	hub, err := ExtractHubConfig(result.Tree)
	require.NoError(t, err)

	require.Len(t, hub.Servers, 1)
	assert.Equal(t, "fleet-hub", hub.Servers[0].Certificate)

	require.Len(t, hub.Clients, 1)
	assert.Equal(t, strings.Repeat("ab", 32), hub.Clients[0].CertificateFingerprint)
}

// TestHubFingerprintRejectsNonHex proves the leaf's pattern is enforced: a
// truncated or non-hex fingerprint is a typo that would otherwise leave the
// client refusing every certificate the hub can present.
func TestHubFingerprintRejectsNonHex(t *testing.T) {
	bad := strings.Replace(hubTrustConfig, strings.Repeat("ab", 32), "not-a-fingerprint", 1)
	_, err := LoadConfig(bad, filepath.Join(t.TempDir(), "hub-bad.conf"), nil)
	require.Error(t, err)
}

// TestHubRefusesDisagreeingCertificates: two blocks that accept managed clients
// and name different certificates are a config the managed server cannot honor
// -- it serves one certificate for both. Refusing at load beats serving the
// wrong one to half the fleet.
//
// MUTATION: delete the checkManagedCertificateAgreement call in
// ExtractHubConfig and this fails: the config loads and the second name is
// silently dropped.
func TestHubRefusesDisagreeingCertificates(t *testing.T) {
	tree := NewTree()
	pluginContainer := NewTree()
	tree.SetContainer("plugin", pluginContainer)
	hubContainer := NewTree()
	pluginContainer.SetContainer("hub", hubContainer)

	block := func(cert string) *Tree {
		serverTree := NewTree()
		serverTree.Set("ip", "127.0.0.1")
		serverTree.Set("port", "1790")
		serverTree.Set("certificate", cert)
		clientTree := NewTree()
		clientTree.Set("secret", "edge01-secret-that-is-at-least-32ch")
		serverTree.AddListEntry("client", "edge-01", clientTree)
		return serverTree
	}
	hubContainer.AddListEntry("server", "north", block("fleet-hub"))
	hubContainer.AddListEntry("server", "south", block("other-hub"))

	_, err := ExtractHubConfig(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one certificate")
}
