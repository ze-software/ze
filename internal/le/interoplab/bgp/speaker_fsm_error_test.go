// Design: docs/architecture/testing/interop.md -- native readiness and explicit relay dispatch.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// A clean worker return is not proof that the control stayed live. Retain the
// failure before the second worker is stopped, even when the first has no error.
func TestFSMControlTailCompletionFailsClosed(t *testing.T) {
	for _, cause := range []error{nil, io.EOF, errors.New("native frame truncated")} {
		t.Run(fmt.Sprintf("%v", cause), func(t *testing.T) {
			var control fsmRelayCase
			for _, test := range fsmRelayCases {
				if test.subcode == 0 {
					control = test
				}
			}
			relay := &fsmRelay{report: fsmErrorTestReport(t, control)}
			path := filepath.Join(t.TempDir(), "control")
			if err := relay.endControlTail(path, cause); err == nil {
				t.Fatal("completed control tail returned healthy")
			}
			data, err := os.ReadFile(path + "-result.json")
			if err != nil {
				t.Fatal(err)
			}
			var retained fsmRelayReport
			if err := json.Unmarshal(data, &retained); err != nil {
				t.Fatal(err)
			}
			if retained.Problem == "" || judgeFSMRelay(retained, control) == nil {
				t.Fatal("completed control tail left a healthy retained report")
			}
		})
	}
}

// TestFSMRelayScenarioReadiness reads the real fixtures through the renderer
// and preparation path. A listening TCP port must not consume the relay's sole
// session before the native FRR neighbor configuration has become active.
func TestFSMRelayScenarioReadiness(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	producer := filepath.Join(root, "test", "interop")
	for _, test := range fsmRelayCases {
		t.Run(test.scenario, func(t *testing.T) {
			network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
			rendered := t.TempDir()
			rendering, err := renderScenario(filepath.Join(producer, "scenarios", test.scenario), rendered, network)
			if err != nil {
				t.Fatal(err)
			}
			peers, err := scenarioPeers(producer, rendered, "fsm-readiness", network, rendering)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 3 || peers[0].Name != "ze" || peers[1].Name != peerFRR || peers[2].Name != peerSpeaker {
				t.Fatalf("native relay startup order: %+v", peers)
			}
			if peers[1].Ready == nil {
				t.Fatal("FRR has no native configuration barrier")
			}
			if !slices.Equal(peers[1].Ready.Command, []string{cmdVtysh, "-c", "show bgp neighbor 172.31.71.10"}) {
				t.Fatalf("FRR barrier: %+v", peers[1].Ready)
			}
			if !slices.Contains(peers[1].Ready.Contains, "BGP state = Active (passive)") {
				t.Fatal("FRR readiness is only a listener probe")
			}
			if peers[0].Ready == nil || !slices.Contains(peers[0].Ready.Contains, `"172.31.71.10"`) {
				t.Fatal("Ze barrier does not name the configured relay")
			}
			if specialCheckers[test.scenario] == nil || speakerRelayRunners[test.personality] == nil {
				t.Fatal("scenario or native relay personality is unregistered")
			}
		})
	}
}

// TestFSMRelayPreservesGenericRelayDispatch makes an ordinary personality
// observable if selected. --relay-peer must retain the old generic relay
// behavior unless the personality explicitly registered a relay runner.
func TestFSMRelayPreservesGenericRelayDispatch(t *testing.T) {
	const name = "fsm-test-ordinary-relay-fallback"
	old, existed := speakerRunners[name]
	called := false
	speakerRunners[name] = func(speakerOptions, io.Writer) error { called = true; return errors.New("ordinary speaker selected") }
	t.Cleanup(func() {
		if existed {
			speakerRunners[name] = old
		} else {
			delete(speakerRunners, name)
		}
	})
	if speakerRelayRunners[name] != nil {
		t.Fatal("ordinary fixture unexpectedly registered a relay")
	}
	// Invalid addresses make the generic relay fail within its tiny lifetime;
	// no listener, server or actual peer is opened by this dispatch test.
	err := runSpeakerHelper([]string{"--connect", "invalid-address", "--relay-peer", "invalid-address", "--test", name,
		"--duration", "0.001", "--result", filepath.Join(t.TempDir(), "relay")}, io.Discard)
	if err == nil {
		t.Fatal("invalid generic relay unexpectedly succeeded")
	}
	if called {
		t.Fatal("ordinary speaker registration changed --relay-peer semantics")
	}
}
