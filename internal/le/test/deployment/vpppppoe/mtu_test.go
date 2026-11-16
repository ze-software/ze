// Design: docs/labs/pppoe-interop.md -- distinguish the VPP MTU from the peer MTU.
// Related: mtu.go -- independent 1400-byte boundary and bounded restoration.
package testvpppppoe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The lower boundary is meaningful only when the measured peer can receive
// the packet just above it. Missing or ambiguous measurements refuse first.
func TestMechanismMTUProbeRequiresIndependentClientMTU(t *testing.T) {
	for _, output := range []string{
		`[{"ifname":"ppp0","mtu":1400}]`,
		`[{"ifname":"ppp0","mtu":1399}]`,
		`[{"ifname":"ppp0"}]`,
		`[{"ifname":"ppp1","mtu":1492}]`,
		`[{"ifname":"ppp0","mtu":1492},{"ifname":"ppp0","mtu":1492}]`,
		`[]`, `not JSON`,
	} {
		t.Run(output, func(t *testing.T) {
			steps := []mtuProbeStep{{arguments: "docker exec client ip -j link show dev ppp0", result: interoplab.ProcessResult{Stdout: output}}}
			lab, complete := mtuProbeFixture(t, steps)
			if err := checkIndependentMTU(t.Context(), lab, "ppp0"); err == nil {
				t.Fatal("probe accepted a peer that does not independently exceed 1400")
			}
			complete()
		})
	}
}

// The command sequence changes only VPP, measures both sides of its lower
// boundary, and restores 1492 before returning to the unchanged assertions.
func TestMechanismMTUProbeControlsVPPBoundary(t *testing.T) {
	steps := mtuProbeSteps()
	lab, complete := mtuProbeFixture(t, steps)
	if err := checkIndependentMTU(t.Context(), lab, "ppp0"); err != nil {
		t.Fatal(err)
	}
	complete()
}

// Pinned VPP prints L3/IP4/IP6/MPLS. Only one exact session row with the
// requested second value proves the command took effect, never another MTU.
func TestMechanismMTUProbeRequiresExactVPPReadback(t *testing.T) {
	for _, test := range []struct {
		name, output string
		index        int
	}{
		{name: "missing initial session", index: 1, output: "host-eth0 1 up 1492/1492/1492/1492"},
		{name: "ambiguous initial session", index: 1, output: "pppoe_session0 7 up 0/1492/0/0\npppoe_session0 8 up 0/1492/0/0"},
		{name: "invalid tuple", index: 1, output: "pppoe_session0 7 up 1492/1492/0"},
		{name: "L3 is not IP4", index: 3, output: "pppoe_session0 7 up 1400/1492/1400/1400"},
		{name: "unapplied lower MTU", index: 3, output: "pppoe_session0 7 up 0/1492/0/0"},
		{name: "ambiguous lower session", index: 3, output: "pppoe_session0 7 up 0/1400/0/0\npppoe_session0 8 up 0/1400/0/0"},
		{name: "unrestored MTU", index: 7, output: "pppoe_session0 7 up 0/1400/0/0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			steps := mtuProbeSteps()
			steps[test.index].result.Stdout = test.output
			switch test.index {
			case 1:
				steps = steps[:2]
			case 3:
				steps = append(steps[:4], steps[6:]...)
			}
			lab, complete := mtuProbeFixture(t, steps)
			if err := checkIndependentMTU(t.Context(), lab, "ppp0"); err == nil {
				t.Fatal("probe accepted a missing, ambiguous or incorrect VPP MTU")
			}
			complete()
		})
	}
}

// An MTU refusal requires ping's completed exit 1 and the measured 1400
// boundary. Transport failure, exit 2 and unrelated numbers do not prove it.
func TestMechanismMTUProbeRequiresTypedPingRefusal(t *testing.T) {
	transport := errors.New("Docker disconnected")
	for _, test := range []struct {
		name   string
		result interoplab.ProcessResult
		cause  error
	}{
		{name: "forwarded", result: interoplab.ProcessResult{}},
		{name: "process failure", result: interoplab.ProcessResult{ExitCode: 2, Stdout: "mtu = 1400"}},
		{name: "transport failure", result: interoplab.ProcessResult{ExitCode: 1, Stdout: "mtu = 1400"}, cause: transport},
		{name: "timeout without MTU", result: interoplab.ProcessResult{ExitCode: 1, Stdout: "100% packet loss"}},
		{name: "peer MTU", result: interoplab.ProcessResult{ExitCode: 1, Stdout: "Frag needed and DF set (mtu = 1492)"}},
		{name: "unrelated number", result: interoplab.ProcessResult{ExitCode: 1, Stdout: "1400 packets lost"}},
		{name: "larger MTU", result: interoplab.ProcessResult{ExitCode: 1, Stdout: "mtu = 14000"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			steps := mtuProbeSteps()
			steps[5].result, steps[5].cause = test.result, test.cause
			lab, complete := mtuProbeFixture(t, steps)
			err := checkIndependentMTU(t.Context(), lab, "ppp0")
			if err == nil {
				t.Fatal("an unrelated failure counted as VPP MTU enforcement")
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport cause lost: %v", err)
				}
			}
			complete()
		})
	}
}

// A failing positive control cannot be interpreted as a working boundary;
// restoration still runs and cannot erase either failure.
func TestMechanismMTUProbeRestoresAfterPositiveFailure(t *testing.T) {
	positiveFailure := errors.New("positive ping transport failed")
	restoreFailure := errors.New("restore transport failed")
	steps := mtuProbeSteps()
	steps[4].cause = positiveFailure
	steps[6].cause = restoreFailure
	steps = append(steps[:5], steps[6])
	lab, complete := mtuProbeFixture(t, steps)
	err := checkIndependentMTU(t.Context(), lab, "ppp0")
	for _, cause := range []error{positiveFailure, restoreFailure} {
		if !errors.Is(err, cause) {
			t.Fatalf("probe lost %v: %v", cause, err)
		}
	}
	complete()
}

// Cancellation can follow an applied MTU even when the set command loses its
// answer. The compensating set/readback use a fresh, bounded context.
func TestMechanismMTUProbeRestoresAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	steps := mtuProbeSteps()
	steps[2].cause = context.Canceled
	steps[2].before = func(context.Context) { cancel() }
	steps = append(steps[:3], steps[6:]...)
	for index := 3; index < len(steps); index++ {
		steps[index].before = func(ctx context.Context) {
			if ctx.Err() != nil {
				t.Fatalf("restore inherited cancellation: %v", ctx.Err())
			}
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("restore has no deadline")
			}
			if time.Until(deadline) > controlCommandBudget {
				t.Fatal("restore exceeds the existing command budget")
			}
		}
	}
	lab, complete := mtuProbeFixture(t, steps)
	if err := checkIndependentMTU(ctx, lab, "ppp0"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
	complete()
}

type mtuProbeStep struct {
	arguments string
	result    interoplab.ProcessResult
	cause     error
	before    func(context.Context)
}

func mtuProbeSteps() []mtuProbeStep {
	return []mtuProbeStep{
		{arguments: "docker exec client ip -j link show dev ppp0", result: interoplab.ProcessResult{Stdout: `[{"ifname":"ppp0","mtu":1492}]`}},
		{arguments: "docker exec vpp vppctl -s /run/vpp/cli.sock show interface pppoe_session0", result: interoplab.ProcessResult{Stdout: "Name Idx State MTU (L3/IP4/IP6/MPLS) Counter Count\npppoe_session0 7 up 0/1492/0/0 rx packets 2"}},
		{arguments: "docker exec vpp vppctl -s /run/vpp/cli.sock set interface mtu ip4 1400 pppoe_session0"},
		{arguments: "docker exec vpp vppctl -s /run/vpp/cli.sock show interface pppoe_session0", result: interoplab.ProcessResult{Stdout: "pppoe_session0 7 up 0/1400/0/0 rx packets 2"}},
		{arguments: "docker exec traffic ping -n -c 1 -W 3 -M do -s 1372 " + peerAddress},
		{arguments: "docker exec traffic ping -n -c 1 -W 3 -M do -s 1373 " + peerAddress, result: interoplab.ProcessResult{ExitCode: 1, Stdout: "Frag needed and DF set (mtu = 1400)"}},
		{arguments: "docker exec vpp vppctl -s /run/vpp/cli.sock set interface mtu ip4 1492 pppoe_session0"},
		{arguments: "docker exec vpp vppctl -s /run/vpp/cli.sock show interface pppoe_session0", result: interoplab.ProcessResult{Stdout: "pppoe_session0 7 up 0/1492/0/0 rx packets 3"}},
	}
}

func mtuProbeFixture(t *testing.T, steps []mtuProbeStep) (mechanismProbeLab, func()) {
	t.Helper()
	calls := 0
	docker := interoplab.NewFixtureDocker(mechanismProcess(func(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
		if calls == len(steps) {
			t.Fatalf("unexpected command: %v", command.Arguments)
		}
		step := steps[calls]
		calls++
		if arguments := strings.Join(command.Arguments, " "); arguments != step.arguments {
			t.Fatalf("command %d = %s, want %s", calls, arguments, step.arguments)
		}
		if step.before != nil {
			step.before(ctx)
		}
		return step.result, step.cause
	}))
	return mechanismProbeLab{docker: docker}, func() {
		t.Helper()
		if calls != len(steps) {
			t.Fatalf("probe executed %d commands, want %d", calls, len(steps))
		}
	}
}
