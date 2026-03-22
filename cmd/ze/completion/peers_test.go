package completion

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCompletionPeersOutput verifies the formatting of peer selector completions.
//
// VALIDATES: AC-8 — output contains peer names, IPs, and as<N> values.
// VALIDATES: AC-11 — ASN values are deduplicated across peers.
// PREVENTS: Completion output with wrong format or missing selectors.
func TestCompletionPeersOutput(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"upstream","remote-as":65001,"state":"Established"},` +
		`"10.0.0.2":{"name":"downstream","remote-as":65002,"state":"Established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Each peer produces 3 entries (name + IP + ASN) = 6 total
	assert.Len(t, lines, 6, "expected 6 completion lines (2 peers x 3 entries)")

	// Verify name entries
	assert.Contains(t, out, "upstream\tpeer name (10.0.0.1 AS 65001)")
	assert.Contains(t, out, "downstream\tpeer name (10.0.0.2 AS 65002)")

	// Verify IP entries
	assert.Contains(t, out, "10.0.0.1\tpeer ip (upstream AS 65001)")
	assert.Contains(t, out, "10.0.0.2\tpeer ip (downstream AS 65002)")

	// Verify ASN entries
	assert.Contains(t, out, "as65001\tpeer asn (upstream 10.0.0.1)")
	assert.Contains(t, out, "as65002\tpeer asn (downstream 10.0.0.2)")
}

// TestCompletionPeersASNDedup verifies that shared ASNs produce one completion entry.
//
// VALIDATES: AC-11 — two peers with same ASN produce one as<N> entry.
// PREVENTS: Duplicate ASN entries cluttering completion output.
func TestCompletionPeersASNDedup(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"ibgp-a","remote-as":65000,"state":"Established"},` +
		`"10.0.0.2":{"name":"ibgp-b","remote-as":65000,"state":"Established"},` +
		`"10.0.1.1":{"name":"ebgp","remote-as":65001,"state":"Established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()

	// Count as65000 entries — should be exactly 1 despite 2 peers with that ASN
	count := strings.Count(out, "as65000\t")
	assert.Equal(t, 1, count, "as65000 should appear exactly once (deduplicated)")

	// as65001 should appear once
	count = strings.Count(out, "as65001\t")
	assert.Equal(t, 1, count, "as65001 should appear exactly once")
}

// TestCompletionPeersNoName verifies output for peers without configured names.
//
// VALIDATES: Peers without names still produce IP and ASN entries.
// PREVENTS: Missing completions for unnamed peers.
func TestCompletionPeersNoName(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"remote-as":65001,"state":"Established"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// No name entry, just IP + ASN = 2 entries
	assert.Len(t, lines, 2, "expected 2 completion lines (IP + ASN, no name)")

	assert.Contains(t, out, "10.0.0.1\tpeer ip (AS 65001)")
	assert.Contains(t, out, "as65001\tpeer asn (10.0.0.1)")
}

// TestCompletionPeersEmptyJSON verifies graceful handling of empty peer list.
//
// VALIDATES: Empty peer list produces no output and returns 0.
// PREVENTS: Crash or error on empty peer list.
func TestCompletionPeersEmptyJSON(t *testing.T) {
	var buf strings.Builder
	code := formatPeerCompletions(&buf, `{"peers":{}}`)
	assert.Equal(t, 0, code)
	assert.Empty(t, buf.String())
}

// TestCompletionPeersInvalidJSON verifies graceful handling of invalid JSON.
//
// VALIDATES: AC-9 — invalid input produces no output and returns 0.
// PREVENTS: Crash on malformed daemon response.
func TestCompletionPeersInvalidJSON(t *testing.T) {
	var buf strings.Builder
	code := formatPeerCompletions(&buf, "not json")
	assert.Equal(t, 0, code)
	assert.Empty(t, buf.String())
}

// TestCompletionPeersASNZero verifies output for peers with ASN 0 (reserved).
//
// VALIDATES: RemoteAS=0 produces "as0" in completion output.
// PREVENTS: Empty or missing ASN string for zero-value ASN.
func TestCompletionPeersASNZero(t *testing.T) {
	jsonData := `{"peers":{` +
		`"10.0.0.1":{"name":"misconfigured","remote-as":0,"state":"Idle"}` +
		`}}`

	var buf strings.Builder
	code := formatPeerCompletions(&buf, jsonData)
	assert.Equal(t, 0, code)

	out := buf.String()
	assert.Contains(t, out, "as0\tpeer asn (misconfigured 10.0.0.1)")
	assert.Contains(t, out, "10.0.0.1\tpeer ip (misconfigured AS 0)")
}

// TestCompletionPeersDispatch verifies the peers subcommand is reachable via Run.
//
// VALIDATES: "ze completion peers" dispatch works.
// PREVENTS: peers being wired to internal function but not to Run.
func TestCompletionPeersDispatch(t *testing.T) {
	// Will return 0 even without daemon (graceful fallback)
	code := Run([]string{"peers"})
	assert.Equal(t, 0, code)
}

// TestShellScriptsIncludeCompletionPeers verifies all 4 shell scripts call
// ze completion peers for dynamic peer selector completion.
//
// VALIDATES: AC-10 — shell completion after "peer" includes dynamic selectors.
// PREVENTS: Shell scripts missing the peer selector integration.
func TestShellScriptsIncludeCompletionPeers(t *testing.T) {
	shells := []struct {
		name string
		gen  string
	}{
		{"bash", "bash"},
		{"zsh", "zsh"},
		{"fish", "fish"},
		{"nushell", "nushell"},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			var buf strings.Builder
			code := generate(sh.gen, &buf)
			assert.Equal(t, 0, code)
			assert.Contains(t, buf.String(), "completion peers",
				"%s script should call 'ze completion peers' for dynamic peer selectors", sh.name)
		})
	}
}
