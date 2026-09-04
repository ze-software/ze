package radius

import (
	"bytes"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/eap"
)

// radiusTree builds system/authentication/radius with the given inner tree.
func radiusTree(inner *config.Tree) *config.Tree {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()
	auth.SetContainer("radius", inner)
	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)
	return tree
}

// VALIDATES: ExtractConfig returns zero config for a nil tree.
// PREVENTS: nil pointer panic when the config tree is unavailable at Build.
func TestExtractRadiusConfigNilTree(t *testing.T) {
	cfg, err := ExtractConfig(nil)
	require.NoError(t, err)
	assert.False(t, cfg.HasServers())
}

// VALIDATES: ExtractConfig returns no servers for an empty tree.
// PREVENTS: false-positive HasServers when RADIUS is not configured (AC-2).
func TestExtractRadiusConfigEmptyTree(t *testing.T) {
	cfg, err := ExtractConfig(config.NewTree())
	require.NoError(t, err)
	assert.False(t, cfg.HasServers())
}

// VALIDATES: ExtractConfig parses servers, timeout, retries, source-address,
// profile-attribute and default-profile from the YANG subtree.
// PREVENTS: wrong field mapping from YANG to ExtractedConfig.
func TestExtractRadiusConfig(t *testing.T) {
	inner := config.NewTree()

	srv1 := config.NewTree()
	srv1.Set("port", "1812")
	srv1.Set("key", "secret-one")
	inner.AddListEntry("server", "10.0.0.1", srv1)

	srv2 := config.NewTree()
	srv2.Set("port", "1645")
	srv2.Set("key", "secret-two")
	inner.AddListEntry("server", "10.0.0.2", srv2)

	inner.Set("timeout", "7")
	inner.Set("retries", "2")
	inner.Set("source-address", "192.168.1.1")
	inner.Set("profile-attribute", "filter-id")
	inner.SetSlice("default-profile", []string{"read-only"})

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)

	require.True(t, cfg.HasServers())
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "10.0.0.1:1812", cfg.Servers[0].Address)
	assert.Equal(t, []byte("secret-one"), cfg.Servers[0].SharedKey)
	assert.Equal(t, "10.0.0.2:1645", cfg.Servers[1].Address)
	assert.Equal(t, []byte("secret-two"), cfg.Servers[1].SharedKey)
	assert.Equal(t, 7*time.Second, cfg.Timeout)
	assert.Equal(t, 2, cfg.Retries)
	assert.True(t, cfg.SourceAddress.Equal(net.ParseIP("192.168.1.1")))
	assert.Equal(t, uint8(AttrFilterID), cfg.ProfileAttr)
	assert.Equal(t, []string{"read-only"}, cfg.DefaultProfiles)
}

// VALIDATES: ExtractConfig applies the YANG defaults when leaves are absent.
// PREVENTS: zero timeout/retries and a missing profile attribute default.
func TestExtractRadiusConfigDefaults(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)

	require.True(t, cfg.HasServers())
	assert.Equal(t, "10.0.0.1:1812", cfg.Servers[0].Address, "default port 1812")
	assert.Equal(t, defaultTimeout, cfg.Timeout, "default timeout 3s")
	assert.Equal(t, defaultRetries, cfg.Retries, "default retries 3")
	assert.Equal(t, uint8(AttrFilterID), cfg.ProfileAttr, "default profile attr Filter-Id")
}

// VALIDATES: profile-attribute resolves to Filter-Id whatever the tree names.
// PREVENTS: a profile carrier RFC 2865 Section 5.25 forbids the client to read.
// RFC requirement: RFC2865-5.25-1 negative -- "class" in the tree is the value
// the enum used to carry, and it resolves to Filter-Id (11), so no configuration
// route reaches the Class attribute (25) as a locally interpreted profile name.
func TestExtractRadiusConfigProfileAttrNeverClass(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)
	inner.Set("profile-attribute", "class")

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, uint8(AttrFilterID), cfg.ProfileAttr)
}

// VALIDATES: ExtractConfig extracts the last-valid boundary values verbatim.
// PREVENTS: overflow/truncation of port 65535, timeout 60, retries 0 and 10.
func TestExtractRadiusConfigBoundaryValues(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("port", "65535")
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)
	inner.Set("timeout", "60")
	inner.Set("retries", "10")

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:65535", cfg.Servers[0].Address)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 10, cfg.Retries)

	inner.Set("retries", "0")
	cfg, err = ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Retries, "explicit retries 0 preserved by extraction")
}

// VALIDATES: the RADIUS shared secret never appears in log output produced
// while building the backend (AC-8).
// PREVENTS: leaking the secret into logs even though the YANG leaf is sensitive.
func TestRadiusSecretNotLogged(t *testing.T) {
	const secret = "top-secret-radius-key"
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", secret)
	inner.AddListEntry("server", "127.0.0.1", srv)

	contrib, err := radiusBackend{}.Build(buildParamsWithTree(radiusTree(inner), logger))
	require.NoError(t, err)
	if contrib.Close != nil {
		t.Cleanup(func() { _ = contrib.Close() })
	}
	assert.NotContains(t, buf.String(), secret, "shared secret must never be logged")
}

// TestExtractConfigAuthMethod covers the extraction side of the auth-method
// leaf. The schema side, where an unknown word is refused at config load with
// the leaf path and the permitted values, is
// TestRadiusAuthMethodEnumRefusesUnknownValue in internal/component/config.
//
// VALIDATES: AC-1 and AC-2 -- an absent leaf and an explicit `pap` both extract
// to AuthMethodPAP. AC-3 -- `chap` extracts to AuthMethodCHAP. AC-8 -- a word
// the schema does not define fails the whole extraction rather than silently
// selecting a credential the operator did not choose. spec-radius-admin-eap
// AC-1 -- `eap-md5` and `eap-mschapv2` extract to their own methods, and each
// names the EAP Type its peer session runs.
// PREVENTS: an unknown auth-method defaulting to PAP behind an operator who
// wrote `chap` with a typo, which would send the password in a recoverable form
// while the config file says otherwise. Also an eap word that parses and then
// selects no EAP Type, which would run the PAP credential under an EAP name.
func TestExtractConfigAuthMethod(t *testing.T) {
	withMethod := func(value string) *config.Tree {
		inner := config.NewTree()
		srv := config.NewTree()
		srv.Set("key", "secret")
		inner.AddListEntry("server", "10.0.0.1", srv)
		if value != "" {
			inner.Set("auth-method", value)
		}
		return radiusTree(inner)
	}

	cfg, err := ExtractConfig(withMethod(""))
	require.NoError(t, err)
	assert.Equal(t, AuthMethodPAP, cfg.AuthMethod, "an absent leaf keeps the shipped PAP behavior")

	cfg, err = ExtractConfig(withMethod("pap"))
	require.NoError(t, err)
	assert.Equal(t, AuthMethodPAP, cfg.AuthMethod)

	cfg, err = ExtractConfig(withMethod("chap"))
	require.NoError(t, err)
	assert.Equal(t, AuthMethodCHAP, cfg.AuthMethod)
	assert.Equal(t, "chap", cfg.AuthMethod.String())

	cfg, err = ExtractConfig(withMethod("eap-md5"))
	require.NoError(t, err)
	assert.Equal(t, AuthMethodEAPMD5, cfg.AuthMethod)
	assert.Equal(t, "eap-md5", cfg.AuthMethod.String())
	eapType, isEAP := cfg.AuthMethod.EAPType()
	assert.True(t, isEAP)
	assert.Equal(t, eap.TypeMD5Challenge, eapType)

	cfg, err = ExtractConfig(withMethod("eap-mschapv2"))
	require.NoError(t, err)
	assert.Equal(t, AuthMethodEAPMSCHAPv2, cfg.AuthMethod)
	assert.Equal(t, "eap-mschapv2", cfg.AuthMethod.String())
	eapType, isEAP = cfg.AuthMethod.EAPType()
	assert.True(t, isEAP)
	assert.Equal(t, eap.TypeMSCHAPv2, eapType)

	// The two password credentials run no EAP conversation.
	for _, method := range []AuthMethod{AuthMethodPAP, AuthMethodCHAP} {
		_, isEAP = method.EAPType()
		assert.False(t, isEAP, "%s runs no EAP conversation", method)
	}

	// A word the schema does not define. "mschapv2" is chosen because it is a
	// real method name and one letter of intent away from eap-mschapv2, so a
	// parser that matched loosely would accept it.
	_, err = ExtractConfig(withMethod("mschapv2"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mschapv2")
	assert.Contains(t, err.Error(), "pap, chap, eap-md5 or eap-mschapv2")
}
