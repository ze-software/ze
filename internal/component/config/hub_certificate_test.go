// VALIDATES: the two trust leaves parse and reach the structs the managed server and
// the managed client read -- plugin/hub/server/certificate and plugin/hub/client/ca --
// and the retired certificate-fingerprint leaf is refused with its replacement named
// (spec-managed-server-hardening AC-1, spec-local-ca AC-5).
// PREVENTS: a leaf that validates in YANG and is dropped in extraction, which leaves
// the hub on a certificate nothing issued and the client with no anchor while the
// operator reads their configured value back from `show configuration`. And a config
// still spelling the pin loading as if it did something.

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
            ca fleet-hub-root
        }
    }
}
`

// TestHubTrustLeavesReachTheStructs parses a config that names a hub certificate
// and the certificate authority the client anchors on, then reads both back
// through ExtractHubConfig.
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
	assert.Equal(t, "fleet-hub-root", hub.Clients[0].CA)
}

// TestHubCARejectsAnUnusableName proves the leaf's pattern is enforced. The
// value names a pki ca entry, and an entry name carries no spaces or slashes,
// so a value that holds one is a typo the client would otherwise fail on at
// its first connection.
func TestHubCARejectsAnUnusableName(t *testing.T) {
	bad := strings.Replace(hubTrustConfig, "ca fleet-hub-root", `ca "fleet hub/root"`, 1)
	_, err := LoadConfig(bad, filepath.Join(t.TempDir(), "hub-bad.conf"), nil)
	require.Error(t, err)
}

// TestFingerprintConfigIsRefused: AC-5 -- a config still carrying the retired
// certificate-fingerprint leaf is refused, and the refusal names the
// certificate authority that replaced it. Ze rewrites no file, so the operator
// needs the replacement spelling in the error.
//
// MUTATION: drop the certificate-fingerprint row from retiredKeywords and this
// fails on the message: the config is still refused, but as a bare unknown
// field that tells the operator nothing about what to write.
func TestFingerprintConfigIsRefused(t *testing.T) {
	retired := strings.Replace(hubTrustConfig, "ca fleet-hub-root",
		"certificate-fingerprint "+strings.Repeat("ab", 32), 1)

	_, err := LoadConfig(retired, filepath.Join(t.TempDir(), "hub-retired.conf"), nil)
	require.Error(t, err, "a config carrying the retired pin must be refused, not silently ignored")
	assert.Contains(t, err.Error(), "certificate-fingerprint")
	assert.Contains(t, err.Error(), "ca <pki-ca-name>")
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
