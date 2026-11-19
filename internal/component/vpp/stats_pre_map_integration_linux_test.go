//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- physical admission before stats mapping
// Related: govpp_stats_identity_linux_test.go -- real Unixpacket SCM_RIGHTS fixture
package vpp

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"

	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

// Malformed or absent segment data from a KNOWN FOREIGN original Unix peer
// cannot erase that custody and become an ordinary optional-stats failure.
// The same actual peer with the same unavailable bytes remains degraded-ready;
// this independently preserves ordinary unavailable-statistics behavior.
func TestVPPPhysicalStatsChecksPeerBeforeUnavailableMapping(t *testing.T) {
	for _, same := range []bool{false, true} {
		for _, segment := range []bool{false, true} {
			name := "foreign"
			if same {
				name = "same"
			}
			if segment {
				name += "-malformed"
			} else {
				name += "-absent"
			}
			t.Run(name, func(t *testing.T) {
				manager := runtimeStatsTestManager(t)
				if !same {
					t.Chdir(t.TempDir())
					startPeerProcess(t, "foreign-api.sock")
					manager.connector.conn = &physicalTestConnection{peer: capturePeerSocket(t, "foreign-api.sock")}
				}
				file, err := os.CreateTemp(t.TempDir(), "empty-stats")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				})
				path, done, _ := startStatsIdentityPeer(t, func(connection *net.UnixConn) error {
					var rights []byte
					if segment {
						rights = syscall.UnixRights(int(file.Fd()))
					}
					_, _, writeErr := connection.WriteMsgUnix([]byte{1}, rights, nil)
					if writeErr != nil {
						// A refused foreign peer may close before this intentionally unusable
						// segment packet is sent. Same-peer transfer must succeed.
						if same {
							return writeErr
						}
						return nil
					}
					return nil // EOF bounds absent-descriptor receive; zero-size fd fails mmap.
				})
				manager.settings.Stats.SocketPath = path
				events := 0
				setVPPEventBus(runtimeStatsTestBus{emit: func(_, event string, _ any) {
					if event != vppevents.EventConnected {
						t.Error(event)
					}
					events++
				}})
				lease, err := manager.activateRuntimeStatsGeneration(t.Context(), 7, func(path string) (runtimeStatsClient, error) { return connectStats(path, manager.connector, 7) })
				if lease != nil {
					lease.Close()
					t.Fatal("unusable native statistics retained a provider")
				}
				requireStatsIdentityPeer(t, done)
				if same {
					if err != nil {
						t.Fatal("same-peer unavailable statistics incorrectly blocked service", err)
					}
					if !manager.ready || events != 1 {
						t.Fatal("same-peer unavailable positive lost degraded readiness")
					}
				} else {
					if !errors.Is(err, errVppStatsPeerMismatch) {
						t.Fatal("foreign unusable segment erased original peer mismatch", err)
					}
					if manager.ready || events != 0 {
						t.Fatal("known foreign unavailable statistics granted readiness")
					}
				}
				if GetActiveStatsProvider() != nil {
					t.Fatal("unusable original statistics were published")
				}
			})
		}
	}
}

// The production opener retains its entry epoch even if activation observes a
// replacement before the real socket hook. Epoch refusal must not degrade into
// a successful ready receipt for that replacement.
func TestVPPPhysicalStatsHookRetainsOriginalEpoch(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	path, done, _ := startStatsIdentityPeer(t, func(*net.UnixConn) error { return nil })
	manager.settings.Stats.SocketPath = path
	manager.connector.mu.Lock()
	manager.connector.generation = 8
	manager.connector.mu.Unlock()
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { events++ }})
	lease, err := manager.activateRuntimeStatsGeneration(t.Context(), 8, func(path string) (runtimeStatsClient, error) { return connectStats(path, manager.connector, 7) })
	if lease != nil {
		lease.Close()
		t.Fatal("original-epoch refusal retained a provider")
	}
	requireStatsIdentityPeer(t, done)
	if !errors.Is(err, errVppGenerationChanged) {
		t.Fatal("pre-map original epoch was lost", err)
	}
	if manager.ready || events != 0 {
		t.Fatal("original epoch failure granted replacement readiness")
	}
	if GetActiveStatsProvider() != nil {
		t.Fatal("original epoch failure published statistics")
	}
}
