// Design: docs/architecture/testing/interop.md -- successor observer discrimination.
package bgp

import (
	"bytes"
	"encoding/json"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestGoBGPReloadNativePreparation reaches the actual rendered preparation entry,
// including its suffixed directory, rather than calling a role helper in isolation.
func TestGoBGPReloadNativePreparation(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	producer := filepath.Join(root, "test", "interop")
	for _, name := range []string{gobgpReloadOrdinary, gobgpReloadSecondary} {
		t.Run(name, func(t *testing.T) {
			if checkers()[name] == nil {
				t.Fatal("successor is not registered in the native registry")
			}
			prepared, err := prepareScenario(t.TempDir(), producer, "actual-suffixed-native", interoplab.PrepareContext{
				Source:  interoplab.ScenarioSource{Name: name, Directory: filepath.Join(producer, "scenarios", name)},
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.31.77.0/24")},
			})
			if prepared.Cleanup != nil {
				t.Cleanup(func() {
					if err := prepared.Cleanup(); err != nil {
						t.Error(err)
					}
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Peers) != 5 {
				t.Fatalf("native topology has %d peers, want five", len(prepared.Peers))
			}
			gobgp := prepared.Peers[0]
			if gobgp.Name != peerGoBGP {
				t.Fatal("GoBGP listener does not start before native active sources")
			}
			if gobgp.Ready == nil {
				t.Fatal("native listener has no CLI readiness barrier")
			}
			if !slices.Equal(gobgp.Ready.Command, []string{"gobgp", "global", "-j"}) {
				t.Fatalf("readiness is %v", gobgp.Ready.Command)
			}
			if !slices.Equal(gobgp.Ready.Contains, []string{`"asn":65005,`}) {
				t.Fatal("unconfigured successful GetBgp can release native source startup")
			}
			if !slices.Equal(gobgp.Arguments, []string{dockerEntrypointFlag, "tini"}) {
				t.Fatalf("signal parent is %v", gobgp.Arguments)
			}
			mounts := make(map[string]bool)
			for _, mount := range gobgp.Mounts {
				if !mount.ReadOnly {
					t.Fatal("native successor writes a checkout fixture mount")
				}
				mounts[mount.Target] = true
			}
			for _, phase := range gobgpReloadPhases[1:] {
				if !mounts[gobgpReloadMounted+"gobgp-reload-"+phase+".toml"] {
					t.Fatalf("phase %s does not reach the native container", phase)
				}
			}
		})
	}
}

// TestGoBGPReloadExactReceiver refuses unknown/partial documents as absence and
// checks prefix, AS_PATH, validity, MED and next hop in the same received path.
func TestGoBGPReloadExactReceiver(t *testing.T) {
	const valid = `{"prefix":"10.240.2.0/24","paths":[{"valid":true,"aspath":{"string":"65005 65001"},"metric":20,"nexthops":[{"ip":"172.30.0.5"}]}]}`
	if err := requireGoBGPReloadRoute(valid, "10.240.2.0/24", "65005 65001", "172.30.0.5", 20); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"empty": "{}", "null": "null", "error": `{"error":"unknown prefix"}`, "unrelated": `{"uptime":20}`,
		"wrong-prefix": strings.Replace(valid, "10.240.2.0/24", "10.240.3.0/24", 1),
		"wrong-source": strings.Replace(valid, "65005 65001", "65005 65004", 1),
		"wrong-hop":    strings.Replace(valid, "172.30.0.5", "172.30.0.4", 1),
		"stale-med":    strings.Replace(valid, `"metric":20`, `"metric":10`, 1),
		"invalid":      strings.Replace(valid, `"valid":true`, `"valid":false`, 1),
		"missing-med":  strings.Replace(valid, `"metric":20,`, "", 1),
		"empty-paths":  `{"prefix":"10.240.2.0/24","paths":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if requireGoBGPReloadRoute(input, "10.240.2.0/24", "65005 65001", "172.30.0.5", 20) == nil {
				t.Fatal("invalid receiver evidence passed")
			}
		})
	}
	if err := requireGoBGPReloadRoute("{}", "10.240.7.0/24", "", "", 0); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"null", `{"error":"unknown"}`, `{"paths":[]}`, valid, "[]"} {
		if requireGoBGPReloadRoute(input, "10.240.7.0/24", "", "", 0) == nil {
			t.Fatalf("unknown/present document %s proved absence", input)
		}
	}
}

// TestGoBGPReloadRetainedSource freezes every retained attribute while allowing
// only route age to advance; source identity is independently validated.
func TestGoBGPReloadRetainedSource(t *testing.T) {
	const source = `{"10.240.0.0/24":[{"nlri":{"prefix":"10.240.0.0/24"},"age":1,"peer-address":"172.30.0.2","attrs":[{"type":1,"value":0},{"type":2,"as_paths":[{"segment_type":2,"num":1,"asns":[65001]}]},{"type":3,"nexthop":"172.30.0.2"}]}]}`
	parse := func(input string) (map[string][]map[string]json.RawMessage, error) {
		return sourceGoBGPReloadPaths(input, "172.30.0.2", 65001, []string{"10.240.0.0/24"})
	}
	before, err := parse(source)
	if err != nil {
		t.Fatal(err)
	}
	after, err := parse(strings.Replace(source, `"age":1`, `"age":2`, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := sameGoBGPReloadSources(before, after); err != nil {
		t.Fatal(err)
	}
	changed, err := parse(strings.Replace(source, `"value":0`, `"value":2`, 1))
	if err != nil {
		t.Fatal(err)
	}
	if sameGoBGPReloadSources(before, changed) == nil {
		t.Fatal("retained source attribute mutation passed")
	}
	for _, input := range []string{
		`{"error":"unknown"}`, `{}`, strings.Replace(source, `"asns":[65001]`, `"asns":[65004]`, 1),
		strings.Replace(source, `"nexthop":"172.30.0.2"`, `"nexthop":"172.30.0.4"`, 1),
		strings.Replace(source, `"prefix":"10.240.0.0/24"`, `"prefix":"10.240.1.0/24"`, 1),
	} {
		if _, err := parse(input); err == nil {
			t.Fatalf("invalid source evidence passed: %s", input)
		}
	}
}

// TestGoBGPReloadNeverSentWire checks every frame, so a transient forbidden
// advertisement followed by a withdrawal cannot disappear behind a final RIB.
func TestGoBGPReloadNeverSentWire(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}
	gobgp := netip.MustParseAddr("172.30.0.5")
	flow := pcap.Flow{SourceAddr: gobgp, TargetAddr: netip.MustParseAddr("172.30.0.3"), SourcePort: 179, TargetPort: 40000}
	control := speakerMessage(bgpUpdate, []byte{0, 0, 0, 0, 24, 10, 240, 0})
	withdrawNever := speakerMessage(bgpUpdate, []byte{0, 4, 24, 10, 240, 7, 0, 0})
	announceNever := speakerMessage(bgpUpdate, []byte{0, 0, 0, 0, 24, 10, 240, 7})
	for _, frames := range [][]byte{withdrawNever, announceNever, append(bytes.Clone(control), withdrawNever...), append(bytes.Clone(announceNever), withdrawNever...)} {
		stream := pcap.Stream{Flow: flow, Bytes: frames}
		if requireGoBGPReloadFrames(&stream, gobgp, network) == nil {
			t.Fatal("never-sent wire control was violated without refusal")
		}
	}
	stream := pcap.Stream{Flow: flow, Bytes: control}
	if err := requireGoBGPReloadFrames(&stream, gobgp, network); err != nil {
		t.Fatal(err)
	}
	stream.Bytes = control[:len(control)-1]
	if requireGoBGPReloadFrames(&stream, gobgp, network) == nil {
		t.Fatal("truncated wire frame passed")
	}
}

// TestGoBGPReloadOriginalOpen rejects wrong native identity and a GoBGP offer
// of ADD-PATH while preserving a peer's harmless one-sided capability.
func TestGoBGPReloadOriginalOpen(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}
	gobgp := netip.MustParseAddr("172.30.0.5")
	open, err := speakerOpen(65005, 90, net.ParseIP(gobgp.String()), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireGoBGPReloadOpen(open, gobgp, gobgp, network); err != nil {
		t.Fatal(err)
	}
	wrong := bytes.Clone(open)
	wrong[27] = 3
	if requireGoBGPReloadOpen(wrong, gobgp, gobgp, network) == nil {
		t.Fatal("wrong original router ID passed")
	}
	wrong = bytes.Clone(open)
	wrong[21]--
	if requireGoBGPReloadOpen(wrong, gobgp, gobgp, network) == nil {
		t.Fatal("wrong original AS passed")
	}
	if requireGoBGPReloadOpen(open[:28], gobgp, gobgp, network) == nil {
		t.Fatal("truncated original OPEN passed")
	}
	withAddPath, err := speakerOpen(65005, 90, net.ParseIP(gobgp.String()), nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if requireGoBGPReloadOpen(withAddPath, gobgp, gobgp, network) == nil {
		t.Fatal("GoBGP ADD-PATH offer passed bounded scope")
	}
	peer := netip.MustParseAddr("172.30.0.3")
	oneSided, err := speakerOpen(65002, 90, net.ParseIP(peer.String()), nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireGoBGPReloadOpen(oneSided, peer, gobgp, network); err != nil {
		t.Fatal(err)
	}
}

// TestGoBGPReloadCounterFences distinguishes a policy-only outbound UPDATE from
// source refresh, a source UPDATE, a reset, or an original OPEN replacement.
func TestGoBGPReloadCounterFences(t *testing.T) {
	const document = `{"state":{"neighbor_address":"172.30.0.2","peer_asn":65001,"router_id":"172.30.0.2","session_state":6,"messages":{"received":{"open":1,"update":8},"sent":{"open":1,"update":1}},"remote_cap":[],"local_cap":[]},"timers":{"state":{"uptime":{"seconds":1}}},"afi_safis":[{"state":{}}]}`
	parse := func(input string) gobgpReloadPeer {
		peer, err := parseGoBGPReloadPeer(input, "172.30.0.2", 65001)
		if err != nil {
			t.Fatal(err)
		}
		return peer
	}
	before := parse(document)
	after := parse(document)
	after.State.Messages.Sent.Update++
	if err := sameGoBGPReloadSession(before, after, true); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*gobgpReloadPeer){
		func(peer *gobgpReloadPeer) { peer.State.Messages.Received.Update++ },
		func(peer *gobgpReloadPeer) { peer.State.Messages.Received.Refresh++ },
		func(peer *gobgpReloadPeer) { peer.State.Messages.Sent.Refresh++ },
		func(peer *gobgpReloadPeer) { peer.State.Messages.Received.Notification++ },
		func(peer *gobgpReloadPeer) { peer.State.Flops++ },
		func(peer *gobgpReloadPeer) { peer.Timers.State.Uptime = json.RawMessage(`{"seconds":2}`) },
		func(peer *gobgpReloadPeer) { peer.State.Remote = json.RawMessage(`[{"code":69}]`) },
	} {
		after := parse(document)
		mutation(&after)
		if sameGoBGPReloadSession(before, after, true) == nil {
			t.Fatal("changed source/session fence passed")
		}
	}
	for _, input := range []string{
		`{}`, `null`, `{"error":"unknown"}`,
		strings.Replace(document, `"open":1`, `"open":2`, 1),
		strings.Replace(document, `"session_state":6`, `"session_state":5`, 1),
		strings.Replace(document, `"peer_asn":65001`, `"peer_asn":65004`, 1),
	} {
		if _, err := parseGoBGPReloadPeer(input, "172.30.0.2", 65001); err == nil {
			t.Fatalf("invalid neighbor document passed: %s", input)
		}
	}
}
