// Design: docs/guide/bgp-role.md -- a peer that declares no RFC 9234 role
// Detail: roleless.go -- warnPeersWithoutRole
// Detail: config_tx.go -- installRoleConfig and verifyRoleConfig, the two loggers of the warning
package role

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// threeRolelessPeers carries two eBGP peers and one iBGP peer, none with a
// role.
const threeRolelessPeers = `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    peer peer-a {
        connection { remote { ip 10.0.0.1 } local { ip auto } }
        session { asn { remote 65001 } }
    }
    peer peer-b {
        connection { remote { ip 10.0.0.2 } local { ip auto } }
        session { asn { remote 65002 } }
    }
    peer ibgp-a {
        connection { remote { ip 10.0.0.3 } local { ip auto } }
        session { asn { remote 65000 } }
    }
}
`

// deliveredSections builds the sections the engine delivers this plugin for a
// config text: the roots it owns and reads, lowered as the daemon lowers them.
func deliveredSections(t *testing.T, text string) []sdk.ConfigSection {
	t.Helper()
	tree := parsedRoleTree(t, text)
	sections, err := config.BuildPluginConfigSections(tree.ToPluginMap(), []string{configRootBGP})
	require.NoError(t, err)
	require.NotEmpty(t, sections, "the bgp section must be delivered")
	return sections
}

// assertOneRolelessLine checks the captured log holds ONE line naming the two
// eBGP peers, with the exact count, and leaves the iBGP session out.
func assertOneRolelessLine(t *testing.T, logged string) {
	t.Helper()
	lines := strings.Count(strings.TrimSpace(logged), "\n") + 1
	assert.Equal(t, 1, lines, "one aggregated line, never one per peer:\n%s", logged)
	assert.Contains(t, logged, "declare no RFC 9234 role")
	assert.Contains(t, logged, "peers=2",
		"the count covers the two eBGP peers and leaves the iBGP session out")
	assert.Contains(t, logged, "peer-a, peer-b")
	assert.NotContains(t, logged, "ibgp-a",
		"an RFC 9234 role describes an eBGP relationship, so an iBGP session is not unbound")
}

// TestPeerWithNoRoleIsAcceptedAndWarnedOnce installs a config whose three
// peers declare no role, through the parse and install the transaction runs at
// Stage 2 and at every reload apply, and captures what the plugin logged.
//
// VALIDATES: AC-34. The config is accepted, and ONE line reports the whole set
// with the exact count.
// PREVENTS: one warning per peer, which buries the fact it exists to report on
// a router carrying a thousand sessions.
func TestPeerWithNoRoleIsAcceptedAndWarnedOnce(t *testing.T) {
	previous := installedRoleConfig()
	previousLogger := logger()
	t.Cleanup(func() {
		loggerPtr.Store(slogutil.DiscardLogger())
		require.NoError(t, installRoleConfig(previous))
		loggerPtr.Store(previousLogger)
	})
	var logged bytes.Buffer
	ConfigureLogger(slogutil.LoggerWithOutput("bgp.role", "warn", &logged))

	cfg, found, err := parseRoleConfig(deliveredSections(t, threeRolelessPeers))
	require.NoError(t, err, "a peer with no role is accepted")
	require.True(t, found)
	require.NoError(t, installRoleConfig(cfg))

	assertOneRolelessLine(t, logged.String())
}

// TestValidateWarnsAboutPeersWithNoRole runs the in-process verifier `ze
// config validate` runs, which never starts the plugin, so logger() is still
// the discard logger there.
//
// VALIDATES: AC-34 on the offline path. The verifier accepts the config and
// logs the same one line through verifyLogger.
// PREVENTS: the warning vanishing from `ze config validate`, where an operator
// checks a config before an upgrade, because the plugin's own logger is
// configured only when the engine starts it.
func TestValidateWarnsAboutPeersWithNoRole(t *testing.T) {
	previous := verifyLogger
	t.Cleanup(func() { verifyLogger = previous })
	var logged bytes.Buffer
	verifyLogger = func() *slog.Logger {
		return slogutil.LoggerWithOutput("bgp.role", "warn", &logged)
	}

	require.NoError(t, verifyRoleConfig(deliveredSections(t, threeRolelessPeers)),
		"a peer with no role is accepted")

	assertOneRolelessLine(t, logged.String())
}

// TestVerifierIsRegistered reads the verifier back from the plugin registry
// `ze config validate` walks.
//
// VALIDATES: the registration carries the verifier, so validate runs it.
// PREVENTS: a verifier that exists and is never called.
func TestVerifierIsRegistered(t *testing.T) {
	previous := verifyLogger
	t.Cleanup(func() { verifyLogger = previous })
	var logged bytes.Buffer
	verifyLogger = func() *slog.Logger {
		return slogutil.LoggerWithOutput("bgp.role", "warn", &logged)
	}

	// The registration the validate walk reads (config.VerifyPluginConfig
	// calls each present root's InProcessConfigVerifier). The walk itself is
	// not driven here: this test binary links no bgp plugin, so its
	// dependency refusal would answer first.
	reg := registry.Lookup(rolePluginName)
	require.NotNil(t, reg)
	require.NotNil(t, reg.InProcessConfigVerifier)
	require.NoError(t, reg.InProcessConfigVerifier(deliveredSections(t, threeRolelessPeers)))
	assertOneRolelessLine(t, logged.String())
}
