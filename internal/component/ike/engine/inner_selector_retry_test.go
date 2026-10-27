// VALIDATES: a stranded-record retry is serialized with itself and with claim
// (innerSelectorRegistry.retryMu), and the removal command a stranded record names is
// the dataplane's own (strandedCommand).
// PREVENTS: a retry deleting the policy a Child SA installed while the retry's removal
// was in flight, on the same peer (XFRM owner check passes) or another one (VPP checks
// no owner), and an `ip xfrm` command handed to the operator of a VPP dataplane.

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// xfrmCommandDP is failingRemoveDP that names its removal commands as the XFRM backend
// does.
type xfrmCommandDP struct {
	failingRemoveDP
}

func (x *xfrmCommandDP) PolicyDeleteCommand(p dataplane.SPParams) (string, error) {
	return dataplane.XFRMPolicyDeleteCommand(&p), nil
}

// vppCommandDP is failingRemoveDP that names its removal commands in the VPP backend's
// form, or answers commandErr when the case sets it.
type vppCommandDP struct {
	failingRemoveDP
	commandErr error
}

const vppCommandFixture = "vppctl ipsec policy del spd 1 priority -2000 outbound action protect sa 4"

func (v *vppCommandDP) PolicyDeleteCommand(dataplane.SPParams) (string, error) {
	if v.commandErr != nil {
		return "", v.commandErr
	}
	return vppCommandFixture, nil
}

// gatedRemoveDP is a dataplane that keeps the live policies by selector, answers EIO
// for the first fails removals, and, once armed, holds the next removal until release
// is closed. It checks no owner on removal, as the VPP backend does not.
type gatedRemoveDP struct {
	mockDP
	mu      sync.Mutex
	fails   int
	armed   bool
	entered chan struct{}
	release chan struct{}
	live    map[spdSelector]string
}

func newGatedRemoveDP(fails int) *gatedRemoveDP {
	return &gatedRemoveDP{
		fails: fails, entered: make(chan struct{}), release: make(chan struct{}),
		live: map[spdSelector]string{},
	}
}

func (g *gatedRemoveDP) InstallSA(dataplane.SAParams) error { return nil }

func (g *gatedRemoveDP) InstallPolicy(p dataplane.SPParams) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.live[selectorOf(&p)] = p.Owner
	return nil
}

func (g *gatedRemoveDP) RemovePolicyParams(p dataplane.SPParams) error {
	g.mu.Lock()
	if g.fails > 0 {
		g.fails--
		g.mu.Unlock()
		return syscall.EIO
	}
	if g.armed {
		g.armed = false
		g.mu.Unlock()
		close(g.entered)
		<-g.release
		g.mu.Lock()
	}
	delete(g.live, selectorOf(&p))
	g.mu.Unlock()
	return nil
}

// owns reports whether every policy of child is live under owner.
func (g *gatedRemoveDP) owns(child *ChildSA, owner string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range append(childPolicies(child, dataplane.SADirOut), childPolicies(child, dataplane.SADirIn)...) {
		if got, ok := g.live[selectorOf(&p)]; !ok || got != owner {
			return false
		}
	}
	return true
}

// TestStrandedRetryDoesNotRemoveFreshPolicy: a retry whose removal is in flight must not
// delete the policy a Child SA installs meanwhile. Method: peer a's Child SA for
// 10.1.2.3/32 is removed while removals fail, so its record is stranded; a retry starts
// and its first removal is held; a Child SA of the same peer (the XFRM owner check
// passes) or of peer b (VPP checks no owner) negotiates the same address and installs
// on the same dataplane; the held removal is then let go. Once both finish, the new Child
// SA's policies must still be live under its peer, and the registry must hold only it.
func TestStrandedRetryDoesNotRemoveFreshPolicy(t *testing.T) {
	cases := []struct{ name, owner, remote string }{
		{name: "same peer", owner: "a", remote: "198.18.0.7"},
		{name: "another peer", owner: "b", remote: "198.18.0.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := slogutil.DiscardLogger()
			reg := newInnerSelectorRegistry()
			dp := newGatedRemoveDP(2)
			first := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			clearLedgerAfter(t, first)
			removeChildSA(first, dp, log)
			if got := reg.holdings(); got != 1 {
				t.Fatalf("setup: registry holds %d Child SAs after both removals failed, want 1", got)
			}

			dp.mu.Lock()
			dp.armed = true
			dp.mu.Unlock()
			retried := make(chan struct{})
			go func() {
				defer close(retried)
				reg.retryStranded()
			}()
			<-dp.entered

			sa := innerPeerSA(t, reg, tc.owner, tc.remote)
			tsi := tsPayload(t, wire.PayloadTypeTSi, "10.1.2.3/32")
			tsr := tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")
			ifID, err := resolveIfID(&sa.PeerCfg)
			if err != nil {
				t.Fatalf("resolveIfID: %v", err)
			}
			type result struct {
				child *ChildSA
				err   error
			}
			installed := make(chan result, 1)
			go func() {
				if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
					installed <- result{err: err}
					return
				}
				child, err := createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress,
					ifID, dp, slogutil.DiscardLogger())
				installed <- result{child: child, err: err}
			}()
			// Without the serialization the install finishes inside this window, before
			// the held removal returns; with it the install waits for the retry.
			var got result
			early := false
			select {
			case got = <-installed:
				early = true
			case <-time.After(200 * time.Millisecond):
			}
			close(dp.release)
			<-retried
			if !early {
				got = <-installed
			}
			if got.err != nil {
				t.Fatalf("peer %s after the retry released a's record: %v", tc.owner, got.err)
			}
			clearLedgerAfter(t, got.child)
			if !dp.owns(got.child, tc.owner) {
				t.Fatalf("the retry's removal took peer %s's fresh policy out of the dataplane", tc.owner)
			}
			if n := reg.holdings(); n != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer %s's)", n, tc.owner)
			}
		})
	}
}

// TestStrandedRetryHoldsClaim: a claim made while a retry's removal is in flight waits
// for the retry, so it does not adopt a policy the retry then removes. Method: peer a's
// record is stranded; a retry starts and its first removal is held; a Child SA of peer a
// for the same address claims the registry directly and installs its policies, as
// installChildSA does after its own retry; the removal is then let go. The new Child
// SA's policies must still be live.
func TestStrandedRetryHoldsClaim(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := newGatedRemoveDP(2)
	first := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, first)
	removeChildSA(first, dp, log)
	// The replacement is built on a registry of its own, so building it claims nothing
	// on reg; only the claim below does.
	next := mustInnerChild(t, innerPeerSA(t, newInnerSelectorRegistry(), "a", "198.18.0.7"), &mockDP{},
		[]string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	installs := append(childPolicies(next, dataplane.SADirOut), childPolicies(next, dataplane.SADirIn)...)

	dp.mu.Lock()
	dp.armed = true
	dp.mu.Unlock()
	retried := make(chan struct{})
	go func() {
		defer close(retried)
		reg.retryStranded()
	}()
	<-dp.entered
	claimed := make(chan error, 1)
	go func() {
		if err := reg.beginInstall(next, "a", innerSelectorPairsOf(next)); err != nil {
			claimed <- err
			return
		}
		for i := range installs {
			if err := dp.InstallPolicy(installs[i]); err != nil {
				reg.finishInstall(next, nil)
				claimed <- err
				return
			}
		}
		reg.finishInstall(next, installs)
		claimed <- nil
	}()
	var err error
	early := false
	select {
	case err = <-claimed:
		early = true
	case <-time.After(200 * time.Millisecond):
	}
	close(dp.release)
	<-retried
	if !early {
		err = <-claimed
	}
	if err != nil {
		t.Fatalf("claim of peer a's replacement: %v", err)
	}
	if !dp.owns(next, "a") {
		t.Fatal("the retry's removal took the replacement's fresh policy out of the dataplane")
	}
}

// TestStrandedRecordNamesDataplaneCommand: the warning and the refusal name the
// removal command of the dataplane that holds the policy. Method: peer a's removals fail
// on a dataplane naming VPP commands, on one naming none, and on one whose naming fails;
// the Warn's commands and peer b's refusal must carry the VPP command, or say the
// dataplane names none, and never an `ip xfrm` command.
func TestStrandedRecordNamesDataplaneCommand(t *testing.T) {
	cases := []struct {
		name string
		dp   dataplane.Dataplane
		want string
	}{
		{name: "vpp", dp: &vppCommandDP{err: syscall.EIO}, want: vppCommandFixture},
		{name: "no command", dp: &failingRemoveDP{err: syscall.EIO}, want: "the dataplane names no command that removes the policy dir out src 0.0.0.0/0 dst 10.1.2.3/32"},
		{
			name: "naming fails",
			dp:   &vppCommandDP{err: syscall.EIO, commandErr: errors.New("two entries share it")},
			want: "the dataplane names no command that removes the policy dir out src 0.0.0.0/0 dst 10.1.2.3/32: two entries share it",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logged bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logged, nil))
			reg := newInnerSelectorRegistry()
			child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), tc.dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			clearLedgerAfter(t, child)
			removeChildSA(child, tc.dp, log)
			if !strings.Contains(logged.String(), tc.want) {
				t.Fatalf("warning names no %q:\n%s", tc.want, logged.String())
			}
			if strings.Contains(logged.String(), "ip xfrm") {
				t.Fatalf("warning names an ip xfrm command on a dataplane that is not XFRM:\n%s", logged.String())
			}
			_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			mustInnerRefused(t, err, "peer b while a's policy cannot be removed")
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal names no %q: %v", tc.want, err)
			}
			if strings.Contains(err.Error(), "ip xfrm") {
				t.Fatalf("refusal names an ip xfrm command on a dataplane that is not XFRM: %v", err)
			}
		})
	}
}

// TestStrandedFixesNameNoSingleTool: the fix lines beside a stranded refusal name the
// tool of each dataplane rather than one backend's command. Method: read both constants.
func TestStrandedFixesNameNoSingleTool(t *testing.T) {
	for _, fix := range []string{childRefusedFixResponderStranded, childResponseFixStranded} {
		if strings.Contains(fix, "ip xfrm policy delete") {
			t.Fatalf("fix prescribes the XFRM command alone: %q", fix)
		}
		if !strings.Contains(fix, "vppctl") || !strings.Contains(fix, "ip xfrm") {
			t.Fatalf("fix names neither dataplane's tool: %q", fix)
		}
	}
}

// TestStrandedCountSkipsScan: the retry finds nothing to do without scanning when no
// record is stranded, and the count follows strand, settle and release. Method: strand
// a record, check the count, retry with removals succeeding, check it is zero.
func TestStrandedCountSkipsScan(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &flakyRemoveDP{err: syscall.EIO, fails: 2}
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	if work := reg.strandedWork(); work != nil {
		t.Fatalf("a live Child SA counted as stranded: %d", len(work))
	}
	removeChildSA(child, dp, log)
	if reg.stranded != 1 {
		t.Fatalf("stranded count %d after a removal left policies, want 1", reg.stranded)
	}
	reg.retryStranded()
	if reg.stranded != 0 {
		t.Fatalf("stranded count %d after the retry released the record, want 0", reg.stranded)
	}
	if got := reg.holdings(); got != 0 {
		t.Fatalf("registry holds %d Child SAs after the retry, want 0", got)
	}
}
