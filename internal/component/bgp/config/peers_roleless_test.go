// Design: docs/guide/bgp-role.md -- a peer that declares no RFC 9234 role
package bgpconfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The tests below drive the peer pipeline from the config text an operator
// types. The role container reaches the schema through the plugin
// registrations loader_test.go links (plugin/all), so these tests run under
// `ze_core ze_bgp`.

// loadPeerConfig parses a config and runs the whole peer pipeline over it,
// which is the path daemon startup, `ze config validate` and `ze doctor` all
// take.
func loadPeerConfig(t *testing.T, text string) error {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err, "the config must parse: this test is about the pipeline, not the schema")
	_, err = PeersFromConfigTree(tree)
	return err
}

// TestPeerWithNoRoleIsAcceptedAndWarnedOnce loads three peers that declare no
// role and captures what the config logger wrote.
//
// VALIDATES: AC-34. The config loads, and ONE line reports the whole set with
// the exact count.
// PREVENTS: one warning per peer, which buries the fact it exists to report on
// a router carrying a thousand sessions.
func TestPeerWithNoRoleIsAcceptedAndWarnedOnce(t *testing.T) {
	var logged bytes.Buffer
	previous := configLogger
	configLogger = func() *slog.Logger {
		return slogutil.LoggerWithOutput("bgp.config", "warn", &logged)
	}
	t.Cleanup(func() { configLogger = previous })

	text := `
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
	require.NoError(t, loadPeerConfig(t, text), "a peer with no role is accepted")

	lines := strings.Count(strings.TrimSpace(logged.String()), "\n") + 1
	assert.Equal(t, 1, lines, "one aggregated line, never one per peer:\n%s", logged.String())
	assert.Contains(t, logged.String(), "peers=2",
		"the count covers the two eBGP peers and leaves the iBGP session out")
	assert.Contains(t, logged.String(), "peer-a")
	assert.Contains(t, logged.String(), "peer-b")
	assert.NotContains(t, logged.String(), "ibgp-a",
		"an RFC 9234 role describes an eBGP relationship, so an iBGP session is not unbound")
}

// TestFilterInstanceNameStripsEveryPrefixForm checks the four chain-ref forms
// resolve to one instance name.
//
// VALIDATES: a chain ref finds its policy registry entry whichever form the
// operator wrote and whichever form canonicalization left behind.
// PREVENTS: a ref resolved in the plain form and missed in the canonical one,
// or the reverse.
func TestFilterInstanceNameStripsEveryPrefixForm(t *testing.T) {
	for _, ref := range []string{
		"NO-TRANSIT",
		"reject-asn:NO-TRANSIT",
		"bgp-filter-path-asn:NO-TRANSIT",
	} {
		assert.Equal(t, "NO-TRANSIT", filterInstanceName(ref))
	}
}
