// Design: docs/architecture/testing/interop.md -- a contained read-path panic, judged by FRR and BIRD.
// Related: check_panic_cease.go -- the predicate and the daemon selection these tests read

package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// zeContainer answers the ze peer scenarioPeers builds for a scenario holding
// the given extra files.
func zeContainer(t *testing.T, extra ...string) (interoplab.PeerConfig, error) {
	t.Helper()
	scenario := t.TempDir()
	writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
	for _, name := range extra {
		writeFixture(t, filepath.Join(scenario, name), "\n")
	}
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}
	peers, err := scenarioPeers(t.TempDir(), scenario, "fixture", network)
	if err != nil {
		return interoplab.PeerConfig{}, err
	}
	index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == "ze" })
	if index < 0 {
		t.Fatalf("no ze peer in %v", peers)
	}
	return peers[index], nil
}

// VALIDATES: a scenario holding ze-le-daemon starts the ze_le daemon under
// tini, and a scenario without it starts the shipped one through the image's
// own entrypoint.
// PREVENTS: bgp-panic-cease running the shipped daemon, where the injection
// does not exist and the trigger file is ignored; or every other scenario
// running the ze_le build instead of what ships.
func TestZeLeDaemonOnlyWhereTheScenarioAsks(t *testing.T) {
	ze, err := zeContainer(t, zeLeDaemonFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ze.Arguments, " "); !strings.Contains(got, dockerEntrypointFlag+" tini") {
		t.Errorf("ze's arguments %q do not start tini as the entrypoint", got)
	}
	if len(ze.Command) < 2 || ze.Command[0] != "--" || ze.Command[1] != zeLeDaemonPath {
		t.Errorf("ze's command %q does not start %s", ze.Command, zeLeDaemonPath)
	}

	shipped, err := zeContainer(t)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(shipped.Command, zeLeDaemonPath) {
		t.Errorf("a scenario without %s starts the ze_le daemon: %q", zeLeDaemonFile, shipped.Command)
	}
	if slices.Contains(shipped.Arguments, dockerEntrypointFlag) {
		t.Errorf("a scenario without %s overrides the entrypoint: %q", zeLeDaemonFile, shipped.Arguments)
	}
}

// VALIDATES: a scenario holding both ze-le-daemon and ze-reload.conf is
// refused with both names, rather than silently running the shipped daemon.
func TestZeLeDaemonRefusesReloadScenario(t *testing.T) {
	_, err := zeContainer(t, zeLeDaemonFile, zeReloadConfigFile)
	if err == nil {
		t.Fatal("a scenario holding both files was accepted")
	}
	for _, name := range []string{zeLeDaemonFile, zeReloadConfigFile} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal %q does not name %s", err, name)
		}
	}
}

// VALIDATES: only a RECEIVED Cease with subcode 0 satisfies the scenario.
// PREVENTS: a green run on a Cease ze never sent (FRR's own), on a Cease with
// another subcode, or on a session that closed with no NOTIFICATION at all.
func TestFRRReceivedCeaseUnspecific(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dueTo  string
		reason string
		want   bool
	}{
		{"unspecific", "BGP Notification receive", "Cease/Unspecific", true},
		{"bare cease", "BGP Notification receive", "Cease", true},
		{"sent, not received", "BGP Notification send", "Cease/Unspecific", false},
		{"out of resources", "BGP Notification receive", "Cease/Out of Resources", false},
		{"administrative reset", "BGP Notification receive", "Cease/Administrative Reset", false},
		{"no notification", "Peer closed the session", "", false},
		{"hold timer", "BGP Notification receive", "Hold Timer Expired", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := map[string]any{"lastResetDueTo": tc.dueTo, "lastNotificationReason": tc.reason}
			if got := frrReceivedCeaseUnspecific(peer); got != tc.want {
				t.Errorf("frrReceivedCeaseUnspecific(%q, %q) = %v, want %v", tc.dueTo, tc.reason, got, tc.want)
			}
		})
	}
}
