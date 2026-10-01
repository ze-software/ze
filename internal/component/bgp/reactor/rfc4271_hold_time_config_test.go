// Design: docs/architecture/config/syntax.md -- per-peer `timer { receive-hold-time }`
// Related: config.go -- parsePeerFromTree reads the timer container
// Related: rfc4271_test.go -- the PeerSettings-level hold-time units

package reactor

import (
	"net/netip"
	"testing"
	"time"
)

// rfc4271HoldTimePeerTree is one peer's config tree, with a timer container
// carrying receive-hold-time when holdSeconds is not empty and no timer
// container at all when it is.
func rfc4271HoldTimePeerTree(remote, holdSeconds string) map[string]any {
	tree := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": remote},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
	}
	if holdSeconds != "" {
		tree["timer"] = map[string]any{"receive-hold-time": holdSeconds}
	}
	return tree
}

// rfc4271SessionHoldTime parses one peer tree the way the config loader does
// and answers the HoldTimer value of the session Ze builds for it.
func rfc4271SessionHoldTime(t *testing.T, name, remote, holdSeconds string) time.Duration {
	t.Helper()
	settings, err := parsePeerFromTree(name, rfc4271HoldTimePeerTree(remote, holdSeconds), 65000, 0)
	if err != nil {
		t.Fatalf("parse peer %s: %v", name, err)
	}
	if settings.Address != netip.MustParseAddr(remote) {
		t.Fatalf("peer %s address = %v, want %s", name, settings.Address, remote)
	}
	return NewSession(settings).timers.HoldTime()
}

// TestRFC4271HoldTimeConfiguredPerPeerFromConfig drives the operator's per-peer
// `timer { receive-hold-time }` through parsePeerFromTree to the session's
// HoldTimer.
//
// VALIDATES: two peers parsed from config with 30 and 240 seconds each build a
// session whose HoldTimer holds its own value.
// PREVENTS: a hold time that can be set only below the config entry point, or
// one config value applied to every neighbor.
//
// RFC requirement: RFC4271-10-1 positive -- the HoldTimer is configurable per peer from
// operator config: peer A's `timer { receive-hold-time 30 }` gives A's session a 30 s
// HoldTimer and peer B's 240 gives B's a 240 s one, both parsed by parsePeerFromTree.
func TestRFC4271HoldTimeConfiguredPerPeerFromConfig(t *testing.T) {
	if got := rfc4271SessionHoldTime(t, "peer-a", "10.0.0.1", "30"); got != 30*time.Second {
		t.Errorf("peer-a HoldTimer = %v, want 30s from its config", got)
	}
	if got := rfc4271SessionHoldTime(t, "peer-b", "10.0.0.2", "240"); got != 240*time.Second {
		t.Errorf("peer-b HoldTimer = %v, want 240s from its config", got)
	}
}

// TestRFC4271HoldTimeUnconfiguredPeerKeepsTheDefault is the negative polarity:
// a peer whose config carries no hold time is not given another peer's value,
// and a value RFC 4271 forbids is refused rather than installed.
//
// VALIDATES: a peer with no timer container, parsed after peers configured with
// 30 and 240 seconds, gets DefaultReceiveHoldTime; a receive-hold-time of 2 is
// refused by parsePeerFromTree.
// PREVENTS: a per-peer setting that leaks to its neighbors, and a config path
// that installs any number it is given.
//
// RFC requirement: RFC4271-10-1 negative -- the setting is per peer and only the
// operator's: a peer parsed with no `timer` container, after two peers configured with
// 30 and 240 seconds, gets the default HoldTimer (DefaultReceiveHoldTime), and
// `receive-hold-time 2` is refused by parsePeerFromTree rather than installed.
func TestRFC4271HoldTimeUnconfiguredPeerKeepsTheDefault(t *testing.T) {
	rfc4271SessionHoldTime(t, "peer-a", "10.0.0.1", "30")
	rfc4271SessionHoldTime(t, "peer-b", "10.0.0.2", "240")
	if got := rfc4271SessionHoldTime(t, "peer-c", "10.0.0.3", ""); got != DefaultReceiveHoldTime {
		t.Errorf("peer-c HoldTimer = %v, want the default %v", got, DefaultReceiveHoldTime)
	}
	if DefaultReceiveHoldTime == 30*time.Second || DefaultReceiveHoldTime == 240*time.Second {
		t.Fatalf("DefaultReceiveHoldTime %v equals a configured value; the test cannot tell them apart", DefaultReceiveHoldTime)
	}

	if _, err := parsePeerFromTree("peer-d", rfc4271HoldTimePeerTree("10.0.0.4", "2"), 65000, 0); err == nil {
		t.Error("receive-hold-time 2 was accepted; RFC 4271 allows only 0 or at least 3 seconds")
	}
}
