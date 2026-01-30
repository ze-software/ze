package peer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPeerLoadExpectNewFormat verifies testpeer parses new .ci format.
//
// VALIDATES: LoadExpectFile handles new key=value format correctly.
// PREVENTS: testpeer failing to parse migrated .ci files.
func TestPeerLoadExpectNewFormat(t *testing.T) {
	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")

	ciContent := `option:file:path=test.conf
option:asn:value=65000
option:bind:value=ipv6
option:tcp_connections:value=2
option:open:value=send-unknown-capability
option:open:value=inspect-open-message
option:open:value=send-unknown-message
option:update:value=send-default-route
option:env:var=ze.log.bgp.server:value=debug
expect:bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect:bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02
expect:json:conn=1:seq=1:json={"type":"keepalive"}
cmd:api:conn=1:seq=1:text=update text nhop set 1.2.3.4
action:notification:conn=1:seq=2:text=session ending`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	expects, config, err := LoadExpectFile(ciFile)
	require.NoError(t, err)

	// Verify config options
	assert.Equal(t, 65000, config.ASN)
	assert.True(t, config.IPv6)
	assert.Equal(t, 2, config.TCPConnections)
	assert.True(t, config.SendUnknownCapability)
	assert.True(t, config.InspectOpenMessage)
	assert.True(t, config.SendUnknownMessage)
	assert.True(t, config.SendDefaultRoute)

	// Verify expects contain the BGP messages and actions (passed through unchanged)
	assert.Len(t, expects, 3, "should have bgp hex lines and notification action")

	// LoadExpectFile now passes through new format unchanged
	foundBGP1 := false
	foundBGP2 := false
	foundNotification := false
	for _, exp := range expects {
		if exp == "expect:bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304" {
			foundBGP1 = true
		}
		if exp == "expect:bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02" {
			foundBGP2 = true
		}
		if exp == "action:notification:conn=1:seq=2:text=session ending" {
			foundNotification = true
		}
	}

	assert.True(t, foundBGP1, "should have first BGP expect")
	assert.True(t, foundBGP2, "should have second BGP expect")
	assert.True(t, foundNotification, "should have notification action")
}
