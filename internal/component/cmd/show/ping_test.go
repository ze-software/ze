// Design: plan/spec-diag-5-active-probes.md -- ping argument parsing tests

package show

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingParseArgsValid(t *testing.T) {
	dest, count, timeout, err := parsePingArgs([]string{"127.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", dest.String())
	assert.Equal(t, defaultPingCount, count)
	assert.Equal(t, defaultPingTimeout, timeout)
}

func TestPingParseArgsWithCount(t *testing.T) {
	dest, count, _, err := parsePingArgs([]string{"10.0.0.1", "count", "3"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 3, count)
}

func TestPingParseArgsWithTimeout(t *testing.T) {
	_, _, timeout, err := parsePingArgs([]string{"10.0.0.1", "timeout", "2s"})
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, timeout)
}

func TestPingParseArgsMissingDest(t *testing.T) {
	_, _, _, err := parsePingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")
}

func TestPingParseArgsInvalidDest(t *testing.T) {
	_, _, _, err := parsePingArgs([]string{"not-an-ip"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination")
}

func TestPingParseArgsCountTooHigh(t *testing.T) {
	_, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "200"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be")
}

func TestPingParseArgsCountZero(t *testing.T) {
	_, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "0"})
	assert.Error(t, err)
}

func TestPingParseArgsTimeoutTooHigh(t *testing.T) {
	_, _, _, err := parsePingArgs([]string{"127.0.0.1", "timeout", "60s"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be")
}

func TestPingParseArgsIPv6(t *testing.T) {
	dest, _, _, err := parsePingArgs([]string{"::1"})
	require.NoError(t, err)
	assert.True(t, dest.Is6())
}

func TestICMPChecksum(t *testing.T) {
	pkt := buildICMPEcho(8, 1234, 0, []byte("test"))
	cs := icmpChecksum(pkt)
	if cs != 0 {
		t.Errorf("checksum of valid packet should verify to 0, got %d", cs)
	}
}
