// Design: docs/labs/pppoe-interop.md -- the VPP mechanism has independent wire peers.
package testvpppppoe

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/interoplab"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// The registered local command must reach the same declared action table
// the container helper and developer entry point consume.
func TestMechanismActionRegisters(t *testing.T) {
	if !command.HasLocal(leroot.CommandPath(area)) {
		t.Fatal("mechanism action is not registered")
	}
	if _, code := actions.Answer([]string{"not-an-action"}); code == 0 {
		t.Fatal("unknown mechanism action was accepted")
	}
}

// A shared Ethernet segment must not supply the subscriber's return path.
// The checker must first identify the real PPP interface and its negotiated
// address, rather than assume ppp0 or treat IPCP control state as kernel state.
func TestMechanismFindsNegotiatedClientInterface(t *testing.T) {
	device, err := clientPPPInterface(`[{"ifname":"ppp7","addr_info":[{"family":"inet","local":"10.20.0.2"}]}]`)
	if err != nil {
		t.Fatal(err)
	}
	if device != "ppp7" {
		t.Fatalf("negotiated interface %q", device)
	}
	for _, text := range []string{`[]`, `[{"ifname":"ppp0","addr_info":[{"family":"inet","local":"10.20.0.3"}]}]`, `[{"ifname":"ppp0","addr_info":[{"family":"inet","local":"10.20.0.2"}]},{"ifname":"ppp1","addr_info":[{"family":"inet","local":"10.20.0.2"}]}]`} {
		if _, err = clientPPPInterface(text); err == nil {
			t.Fatalf("unassigned or ambiguous subscriber accepted: %s", text)
		}
	}
}

// Kernel readback must confirm the installed specific return route, because
// the Ethernet connected route reaches the same traffic host by another path.
func TestMechanismRejectsEthernetReturnRoute(t *testing.T) {
	if err := routeUsesPPP(`[{"dev":"ppp7"}]`, "ppp7"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{`[{"dev":"eth0"}]`, `[]`, `[{"dev":"ppp7"},{"dev":"eth0"}]`} {
		if err := routeUsesPPP(text, "ppp7"); err == nil {
			t.Fatalf("reply route bypass accepted: %s", text)
		}
	}
}

// Old nonzero session counters can be negotiation noise. A controlled ping
// proves the round trip only when both actual counters increase from baseline.
func TestMechanismRequiresCounterIncreaseInBothDirections(t *testing.T) {
	before := packetCounters{received: 10, sent: 12}
	if err := counterIncrease(before, packetCounters{received: 12, sent: 14}); err != nil {
		t.Fatal(err)
	}
	for _, after := range []packetCounters{{received: 10, sent: 14}, {received: 12, sent: 12}, {received: 1, sent: 1}} {
		if err := counterIncrease(before, after); err == nil {
			t.Fatalf("unchanged or reset counters passed: %#v", after)
		}
	}
}

// The suite's existing bounded diagnostics must capture detached process and
// helper state before removing the container and this run's temporary files.
func TestMechanismPlanRetainsFailureEvidence(t *testing.T) {
	plan := mechanismPlan("/checkout", "/scratch/run", "fixture")
	wanted := map[string][]string{vppPeer: {"/evidence/vpp.log", "/evidence/control.error", "/evidence/control.state"}, clientPeer: {"/var/log/ppp/mechanism.log"}}
	for _, peer := range plan.Peers {
		for _, file := range wanted[peer.Name] {
			found := false
			for _, declared := range peer.LogFiles {
				if declared == file {
					found = true
				}
			}
			if !found {
				t.Fatalf("peer %s loses %s before cleanup", peer.Name, file)
			}
		}
	}
}

// Only a completed cat with ENOENT means no failure marker. A transport loss
// or a permission refusal must remain an error rather than read healthy state.
func TestMechanismControlStatePreservesProbeFailures(t *testing.T) {
	transport := errors.New("Docker transport disconnected")
	for _, test := range []struct {
		name, stderr string
		cause        error
	}{{name: "transport", cause: transport}, {name: "permission", stderr: "cat: /evidence/control.error: Permission denied"}} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				calls++
				return interoplab.ProcessResult{ExitCode: 1, Stderr: test.stderr}, test.cause
			}))
			_, err := readControlState(t.Context(), mechanismProbeLab{docker: docker})
			if err == nil {
				t.Fatal("failed marker probe was treated as an absent marker")
			}
			if calls != 1 {
				t.Fatalf("read state after failure probe: %d calls", calls)
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport cause lost: %v", err)
				}
			}
		})
	}
}

// A genuine missing error marker permits reading a complete positive state.
func TestMechanismControlStateAcceptsMissingMarker(t *testing.T) {
	calls := 0
	docker := interoplab.NewFixtureDocker(mechanismProcess(func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
		calls++
		if calls == 1 {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "cat: /evidence/control.error: No such file or directory"}, nil
		}
		return interoplab.ProcessResult{Stdout: `{"bounds-barrier":true}`}, nil
	}))
	state, err := readControlState(t.Context(), mechanismProbeLab{docker: docker})
	if err != nil {
		t.Fatal(err)
	}
	if !state.BoundsBarrier {
		t.Fatal("positive state was not read")
	}
	if calls != 2 {
		t.Fatalf("state probe made %d calls", calls)
	}
}

type mechanismProcess func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error)

func (run mechanismProcess) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	return run(ctx, command)
}

type mechanismProbeLab struct {
	interoplab.CheckerLab
	docker *interoplab.Docker
}

func (lab mechanismProbeLab) Exec(ctx context.Context, peer string, args []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return lab.docker.Exec(ctx, peer, args, environment)
}

// The scratch peer accepts one complete tagged PADI and refuses a declared
// payload that exceeds the actual frame instead of echoing unreceived bytes.
func TestMechanismDiscoveryChecksDeclaredLength(t *testing.T) {
	frame := []byte{255, 255, 255, 255, 255, 255, 2, 0, 0, 0, 0, 1, 0x81, 0, 0, 100, 0x88, 0x63, 0x11, 9, 0, 0, 0, 4, 1, 1, 0, 0}
	decoded, err := parseDiscovery(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.vlan) != 4 {
		t.Fatal("VLAN envelope was lost")
	}
	if len(decoded.tags) != 4 {
		t.Fatal("complete Service-Name tag was lost")
	}
	frame[23] = 5
	if _, err = parseDiscovery(frame); err == nil {
		t.Fatal("truncated declared discovery length was accepted")
	}
}

// Fuzzing exercises only the bounded scratch parser, without opening sockets.
func FuzzMechanismDiscovery(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{255, 255, 255, 255, 255, 255, 2, 0, 0, 0, 0, 1, 0x88, 0x63, 0x11, 9, 0, 0, 0, 4, 1, 1, 0, 0})
	f.Fuzz(func(t *testing.T, frame []byte) {
		decoded, err := parseDiscovery(frame)
		if err == nil {
			if len(decoded.tags) > len(frame) {
				t.Fatal("parser returned unreceived tag bytes")
			}
		}
	})
}

// The MTU oracle must run on a different peer from the PPP address owner;
// otherwise Linux can answer its own address without using VPP at all.
func TestMechanismPlanHasIndependentTrafficPeer(t *testing.T) {
	plan := mechanismPlan("/checkout", "/scratch/run", "fixture")
	if len(plan.Peers) != 3 {
		t.Fatalf("mechanism has %d peers, want separate VPP, pppd and traffic peers", len(plan.Peers))
	}
	seen := map[string]bool{}
	for _, peer := range plan.Peers {
		if seen[peer.Name] {
			t.Fatalf("duplicate peer %q", peer.Name)
		}
		seen[peer.Name] = true
		for _, argument := range peer.Arguments {
			if argument == "--privileged" {
				t.Fatal("mechanism uses an unrestricted privileged container")
			}
		}
	}
	for _, name := range []string{vppPeer, clientPeer, trafficPeer} {
		if !seen[name] {
			t.Fatalf("mechanism has no %q peer", name)
		}
	}
}

// The staged helper is mounted by its task-owned path; the lab must not
// create another source tree or mount a host-wide temporary directory.
func TestMechanismPlanMountsOnlyOwnedInputs(t *testing.T) {
	work := "/scratch/run"
	plan := mechanismPlan("/checkout", work, "fixture")
	for _, peer := range plan.Peers {
		for _, mount := range peer.Mounts {
			if mount.Source != work && mount.Source != filepath.Join(work, "le") {
				t.Fatalf("peer %s mounts unrelated path %q", peer.Name, mount.Source)
			}
		}
	}
}

// An LCP Configure-Ack repeats the received Identifier and options exactly;
// malformed lengths are refused rather than converted into an acknowledgment.
func TestMechanismLCPAckEchoesRequest(t *testing.T) {
	request := []byte{0xc0, 0x21, 1, 42, 0, 8, 1, 4, 5, 0xd4}
	ack, err := controlAck(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(ack) != len(request) {
		t.Fatalf("ack length %d differs from request %d", len(ack), len(request))
	}
	for index := range request {
		want := request[index]
		if index == 2 {
			want = 2
		}
		if ack[index] != want {
			t.Fatalf("ack byte %d = %d, want %d", index, ack[index], want)
		}
	}
	if request[2] != 1 {
		t.Fatal("ack construction mutated the received request")
	}
}

// The control helper's acknowledgment requires a complete declared packet.
// Dropping every frame cannot satisfy the valid-request test beside this one.
func TestMechanismLCPAckRejectsTruncatedRequest(t *testing.T) {
	for _, request := range [][]byte{{}, {0xc0, 0x21}, {0xc0, 0x21, 1, 1, 0, 8}, {0xc0, 0x21, 2, 1, 0, 4}} {
		if _, err := controlAck(request); err == nil {
			t.Fatalf("malformed Configure-Request acknowledged: %x", request)
		}
	}
}

// Interface counters can share the first line with interface metadata; both
// directions must be measured above zero rather than inferred from a ping.
func TestMechanismReadsBothVPPPacketCounters(t *testing.T) {
	if err := interfaceTraffic("pppoe_session0 3 up 1492/0/0/0 rx packets 2\n tx packets 2\n"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"", "rx packets 0\n tx packets 2\n", "rx packets 2\n tx packets 0\n", "rx packets unknown\n tx packets 2\n"} {
		if err := interfaceTraffic(text); err == nil {
			t.Fatalf("unmeasured or idle counters passed: %q", text)
		}
	}
}

// The LCP host interface is read from VPP's declared pair rather than guessed
// from the first TAP name present in an unrelated interface dump.
func TestMechanismResolvesControlPair(t *testing.T) {
	name, err := controlVPPName("itf-pair: [0] host-eth0 tap3 cp0 12 type tap\n", controlInterface)
	if err != nil {
		t.Fatal(err)
	}
	if name != "tap3" {
		t.Fatalf("control VPP name %q", name)
	}
	if _, err = controlVPPName("itf-pair: [0] host-eth0 tap3 other 12 type tap\n", controlInterface); err == nil {
		t.Fatal("unrelated pair was accepted")
	}
}

// Counterfactual VPP inputs must be immutable image IDs; a mutable tag cannot
// establish which producer was broken while the helper remains current.
func TestMechanismImageRequiresCompleteDigest(t *testing.T) {
	if !immutableImage("sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
		t.Fatal("valid image identity refused")
	}
	for _, image := range []string{"ze-vpp-pppoe-lab", "sha256:short", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg"} {
		if immutableImage(image) {
			t.Fatalf("mutable or malformed identity accepted: %q", image)
		}
	}
}
