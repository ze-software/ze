// Package sdk_test holds the SDK tests that reach the daemon's own component
// tree. They live outside package sdk because internal/component/pki imports
// its way back to this package, which an in-package test file may not do.
package sdk_test

import (
	"context"
	"crypto/tls"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The three tests below drive dialAndAuth over the REAL plugin rail: the hub
// acceptor issues its leaf from a pki root, hands that root to a child through
// the environment, and the SDK validates the chain against it. They are the
// end of the path spec-local-ca replaced the fingerprint pin on, so they check
// what the child does with the anchor it was given, and what it does with none.
//
// VALIDATES: spec-local-ca AC-2, AC-3, AC-4 and R-2.
// PREVENTS: the shipping posture this replaced, where an absent ze.plugin.cert.fp
// produced an InsecureSkipVerify config with no comparison, so a plugin wrote
// its token to whatever answered on the hub address.

// hubUnderTest starts a hub acceptor serving a leaf issued by a fresh
// certificate authority, and returns the acceptor with that authority's root.
func hubUnderTest(t *testing.T) (*pluginipc.PluginAcceptor, *zepki.Root) {
	t.Helper()
	root := sdkTestRoot(t)

	acceptor, err := zeplugin.NewHubAcceptor(nil, root, clock.RealClock{})
	require.NoError(t, err)
	t.Cleanup(acceptor.Stop)
	return acceptor, root
}

// sdkTestRoot generates one certificate authority in a temporary blob store,
// the way a daemon does on first start.
func sdkTestRoot(t *testing.T) *zepki.Root {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	root, err := zepki.LoadOrGenerateRoot(store)
	require.NoError(t, err)
	return root
}

// setPluginEnv writes the four transport variables a forked plugin reads, and
// clears them again when the test ends. rootPEM is what startExternal puts in
// ZE_PLUGIN_CA_PEM.
func setPluginEnv(t *testing.T, acceptor *pluginipc.PluginAcceptor, rootPEM string) {
	t.Helper()
	host, port, err := net.SplitHostPort(acceptor.Addr().String())
	require.NoError(t, err)

	token, err := acceptor.TokenForPlugin("sdk-dial-test")
	require.NoError(t, err)

	for key, value := range map[string]string{
		"ze.plugin.hub.host":  host,
		"ze.plugin.hub.port":  port,
		"ze.plugin.hub.token": token,
		"ze.plugin.ca.pem":    rootPEM,
	} {
		require.NoError(t, env.Set(key, value))
		t.Cleanup(func() { _ = env.Set(key, "") })
	}
}

// TestPluginDialValidatesTheChain: AC-2 -- a plugin connects to its hub by
// validating the hub's leaf against the root it was handed, and the hub sees
// the authenticated connection.
//
// MUTATION: drop RootCAs from ipc.TLSConfigWithRoot and this fails: the system
// pool cannot verify a leaf the daemon root issued.
func TestPluginDialValidatesTheChain(t *testing.T) {
	acceptor, root := hubUnderTest(t)
	setPluginEnv(t, acceptor, string(root.CertificatePEM()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := sdk.DialAndAuth(ctx, "sdk-dial-test")
	require.NoError(t, err, "a plugin holding the hub root was refused by its own hub")
	defer conn.Close() //nolint:errcheck // test cleanup

	accepted, err := acceptor.WaitForPlugin(ctx, "sdk-dial-test")
	require.NoError(t, err, "the hub did not authenticate the plugin that dialed it")
	require.NoError(t, accepted.Close())
}

// TestPluginDialRefusesAnUnknownIssuer: AC-3 -- a hub whose leaf the plugin's
// root did not issue ends the handshake, so the plugin's token is never
// written.
//
// MUTATION: set InsecureSkipVerify in ipc.TLSConfigWithRoot and this fails: the
// handshake completes against a hub the plugin cannot place.
func TestPluginDialRefusesAnUnknownIssuer(t *testing.T) {
	acceptor, _ := hubUnderTest(t)
	stranger := sdkTestRoot(t)
	setPluginEnv(t, acceptor, string(stranger.CertificatePEM()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := sdk.DialAndAuth(ctx, "sdk-dial-test")
	if err == nil {
		conn.Close() //nolint:errcheck,gosec // test cleanup on the failing path
		t.Fatal("a plugin completed a handshake with a hub its root did not issue for")
	}

	var verification *tls.CertificateVerificationError
	require.ErrorAs(t, err, &verification, "the refusal must name the verification failure")
}

// TestPluginDialRefusesWithNoAnchor: AC-4 and R-2 -- a plugin given no root
// refuses before it dials. This is the defect the certificate authority
// replaces: an empty ze.plugin.cert.fp used to yield InsecureSkipVerify.
//
// MUTATION: return a bare &tls.Config{} for an empty root in
// ipc.TLSConfigWithRoot and this fails: the dial proceeds and the hub records
// an authenticated plugin.
func TestPluginDialRefusesWithNoAnchor(t *testing.T) {
	acceptor, _ := hubUnderTest(t)
	setPluginEnv(t, acceptor, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := sdk.DialAndAuth(ctx, "sdk-dial-test")
	if err == nil {
		conn.Close() //nolint:errcheck,gosec // test cleanup on the failing path
		t.Fatal("a plugin with no trust anchor connected to its hub")
	}
	require.Contains(t, err.Error(), "trust anchor")

	// Nothing reached the hub: the refusal happened before the dial.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer waitCancel()
	if accepted, waitErr := acceptor.WaitForPlugin(waitCtx, "sdk-dial-test"); waitErr == nil {
		accepted.Close() //nolint:errcheck,gosec // test cleanup on the failing path
		t.Fatal("the hub authenticated a plugin that had no trust anchor")
	}
}
