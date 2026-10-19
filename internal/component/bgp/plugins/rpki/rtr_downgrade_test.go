//go:build live

package rpki

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLiveRTRv2DowngradeToV1 starts a stayrtr container (protocol v1 default),
// connects with our RTR client (starts at v2), and verifies the v2 -> error 4 ->
// v1 fallback path works and VRPs are synced successfully.
//
// VALIDATES: RTR v2 negotiation with v1 fallback against a real server.
// PREVENTS: Version downgrade logic broken by ASPA v2 changes.
func TestLiveRTRv2DowngradeToV1(t *testing.T) {
	t.Log("starting stayrtr container...")
	port := startStayRTR(t, "rpki-downgrade")

	cache := newROACache()
	stopCh := make(chan struct{})
	session := newTestRTRSession(t, "127.0.0.1", port, 100, "", cache, newASPACache(), stopCh)
	session.retryInterval = 5 * time.Second

	done := make(chan struct{})
	go func() {
		newCacheGroup([]*RTRSession{session}, stopCh).Run()
		close(done)
	}()

	t.Log("waiting for v2->v1 downgrade and VRP sync...")
	require.Eventually(t, func() bool {
		v4, _ := cache.Count()
		return v4 > 0
	}, 90*time.Second, 2*time.Second, "VRPs should populate via v2->v1 downgrade")

	close(stopCh)
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("session did not exit")
	}

	assert.Equal(t, uint8(1), session.version, "session should have downgraded to v1")

	v4, v6 := cache.Count()
	assert.Greater(t, v4, 100_000, "expected >100K IPv4 VRPs")
	assert.Greater(t, v6, 10_000, "expected >10K IPv6 VRPs")
	t.Logf("v2->v1 downgrade: synced %d IPv4 + %d IPv6 VRPs", v4, v6)
}
