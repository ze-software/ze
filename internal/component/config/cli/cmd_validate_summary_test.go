//go:build ze_bgp

// Design: docs/guide/command-reference.md -- the `ze config validate -v` peer summary.
package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summaryConfig puts two peers in a group that carries their remote AS, one of
// them overriding it and turning connect off, beside a standalone peer with its
// own local AS, and a grouped peer that no layer gives a remote AS.
const summaryConfig = `bgp {
    router-id 192.0.2.254
    session { asn { local 64496 } }
    group edge {
        session { asn { remote 64500 } }
        peer edge-a {
            connection { remote { ip 192.0.2.1 } local { ip 192.0.2.254 } }
        }
        peer edge-b {
            connection { remote { ip 192.0.2.2; connect false } local { ip 192.0.2.254 } }
            session { asn { remote 64501 } }
        }
    }
    group bare {
        peer bare-a {
            connection { remote { ip 192.0.2.3 } local { ip 192.0.2.254 } }
        }
    }
    peer solo {
        connection { remote { ip 192.0.2.9 } local { ip 192.0.2.254 } }
        session { asn { remote 64502; local 64497 } }
    }
}
`

// TestValidateVerboseSummaryResolvesGroupInheritance runs `ze config validate -v`
// on a config whose peers take their remote AS from a group, and reads the
// summary it prints.
//
// VALIDATES: each peer line names the peer, its address and the remote AS the
// daemon would use after group inheritance, its group, and its connect mode; a
// peer the daemon skips says so instead of printing an AS; the global local-as
// is read from session/asn/local.
// PREVENTS: the summary reading leaves the schema does not have (remote/as,
// remote/ip) and printing an empty address and AS0 for every peer, which an
// operator reads as a config that sets no AS.
func TestValidateVerboseSummaryResolvesGroupInheritance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.conf")
	require.NoError(t, os.WriteFile(path, []byte(summaryConfig), 0o600))

	code, out := captureValidateStdout(t, []string{"-v", path})
	require.Equal(t, 0, code, out)

	assert.Contains(t, out, "  local-as:  64496\n", out)
	assert.Contains(t, out, "  peers: 4\n", out)
	assert.Contains(t, out, "  - bare-a: incomplete, the daemon does not start it: missing required session > asn > remote\n", out)
	assert.Contains(t, out, "  - edge-a 192.0.2.1 AS64500 group edge\n", out)
	assert.Contains(t, out, "  - edge-b 192.0.2.2 AS64501 group edge (connect disabled)\n", out)
	assert.Contains(t, out, "  - solo 192.0.2.9 AS64502\n", out)
	assert.NotContains(t, out, "AS0", out)

	// The structured summary carries the same resolved values, in name order,
	// and names the skipped peer apart rather than giving it zero fields.
	result := runValidation(summaryConfig, "summary.conf")
	require.True(t, result.Valid, "%+v", result.Diagnostics)
	require.NotNil(t, result.Config)
	assert.Equal(t, uint32(64496), result.Config.LocalAS)
	assert.Equal(t, 4, result.Config.Peers)
	assert.Equal(t, []incompletePeer{{Name: "bare-a", Reason: "missing required session > asn > remote"}}, result.Config.IncompletePeers)
	require.Len(t, result.Config.PeerDetails, 3)
	assert.Equal(t, peerSummary{Name: "edge-a", Group: "edge", Address: "192.0.2.1", PeerAS: 64500, LocalAS: 64496, Connect: true, Accept: true}, result.Config.PeerDetails[0])
	assert.Equal(t, peerSummary{Name: "edge-b", Group: "edge", Address: "192.0.2.2", PeerAS: 64501, LocalAS: 64496, Connect: false, Accept: true}, result.Config.PeerDetails[1])
	assert.Equal(t, peerSummary{Name: "solo", Address: "192.0.2.9", PeerAS: 64502, LocalAS: 64497, Connect: true, Accept: true}, result.Config.PeerDetails[2])
}

// TestValidateVerboseSummaryNamesTheSkipReason runs `ze config validate -v` on
// a config whose peers each lack a different setting the peer parser requires.
//
// VALIDATES: each skipped peer is listed with the parser's own reason (here a
// missing remote AS and a missing remote ip), not one fixed guess.
// PREVENTS: the summary telling an operator to set a remote AS on a peer whose
// actual gap is its remote ip.
func TestValidateVerboseSummaryNamesTheSkipReason(t *testing.T) {
	const conf = `bgp {
    router-id 192.0.2.254
    session { asn { local 64496 } }
    peer no-remote-as {
        connection { remote { ip 192.0.2.3 } }
    }
    peer no-ip {
        session { asn { remote 64501; local 64496 } }
    }
}
`
	path := filepath.Join(t.TempDir(), "skipped.conf")
	require.NoError(t, os.WriteFile(path, []byte(conf), 0o600))

	code, out := captureValidateStdout(t, []string{"-v", path})
	require.Equal(t, 0, code, out)
	assert.Contains(t, out, "  - no-remote-as: incomplete, the daemon does not start it: missing required session > asn > remote\n", out)
	assert.Contains(t, out, "  - no-ip: incomplete, the daemon does not start it: missing required connection > remote > ip\n", out)
	assert.Contains(t, out, "  peers: 2\n", out)
}

// captureValidateStdout runs cmdValidate and returns its exit code and what it
// wrote to stdout.
func captureValidateStdout(t *testing.T, args []string) (int, string) {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	done := make(chan []byte)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r) //nolint:errcheck // test pipe
		done <- buf.Bytes()
	}()
	code := cmdValidate(args)
	w.Close() //nolint:errcheck // test pipe
	os.Stdout = saved
	return code, string(<-done)
}
