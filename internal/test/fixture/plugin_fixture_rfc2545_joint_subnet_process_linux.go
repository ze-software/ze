//go:build linux

// Design: docs/functional-tests.md -- state-fenced native BGP fixtures.
// Related: plugin_fixture_rfc2545_joint_subnet_linux.go -- topology owner.
// Related: internal/test/peer/reject.go -- successful and rejection markers.
package fixture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/test/peer"
)

// jointSubnetOutput observes the peer's existing success/rejection protocol.
// Safe for concurrent use by child stdout/stderr copiers and the fixture owner.
// One bounded line buffer suffices: no transcript grows with the peer lifetime.
type jointSubnetOutput struct {
	mu       sync.Mutex
	line     [8192]byte
	used     int
	success  bool
	rejected bool
}

// Write preserves the peer output for the outer runner and tracks complete lines.
func (o *jointSubnetOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	written, err := os.Stdout.Write(data)
	if err != nil {
		return written, err
	}
	for _, value := range data {
		if value == '\n' {
			line := o.line[:o.used]
			if bytes.Equal(line, []byte("successful")) {
				o.success = true
			}
			if bytes.Contains(line, []byte(peer.RejectionMarker)) {
				o.rejected = true
			}
			o.used = 0
			continue
		}
		if o.used == len(o.line) {
			o.rejected = true
			return 0, fmt.Errorf("joint-subnet peer output exceeded bounded line length")
		}
		o.line[o.used] = value
		o.used++
	}
	return len(data), nil
}

// verdict reads both flags under the same lock; a late rejection retracts success.
func (o *jointSubnetOutput) verdict() (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.rejected {
		return false, fmt.Errorf("joint-subnet peer rejected recipient wire output")
	}
	return o.success, nil
}

// runJointSubnetPeers keeps both real peer processes lingering until the
// recipient has asserted the subject AND the subsequent control UPDATE. The
// source cannot close early and invalidate a queued announcement. The owner
// MUST stop/reap every child, including on setup errors; watchedProcesses owns
// one waiter per child and stop joins them all before final rejection checks.
func runJointSubnetPeers(ctx context.Context, plan *netnsRunPlan, path *clampedPath, orig netns.NsHandle) error {
	if len(plan.peers) != 2 {
		return fmt.Errorf("joint-subnet needs exactly recipient and announcer peers")
	}
	if plan.peers[0].namespace != "router" {
		return fmt.Errorf("joint-subnet first peer must be the recipient")
	}
	if plan.peers[0].after != "" {
		return fmt.Errorf("joint-subnet recipient must start immediately")
	}
	if plan.peers[1].namespace != "far" {
		return fmt.Errorf("joint-subnet second peer must be the announcer")
	}
	if plan.peers[1].after == "" {
		return fmt.Errorf("joint-subnet announcer must await recipient establishment")
	}
	watched := &watchedProcesses{exits: make(chan processExit, 3)}
	defer func() {
		if watched.pending != 0 {
			watched.stop()
		}
	}()
	var outputs [2]jointSubnetOutput
	if err := startJointSubnetPeer(ctx, plan.peers[0], path.router, orig, &outputs[0], watched); err != nil {
		return err
	}
	command, err := startCommand(ctx, plan, path.sender, orig)
	if err != nil {
		return err
	}
	watched.watch(command, "joint-subnet speaker", true)
	started := false
	poll := time.NewTicker(clampedPeerPoll)
	defer poll.Stop()
	for {
		if !started {
			ready, err := peerFileExists(plan.peers[1].after)
			if err != nil {
				return err
			}
			if ready {
				if err := startJointSubnetPeer(ctx, plan.peers[1], path.far, orig, &outputs[1], watched); err != nil {
					return err
				}
				started = true
			}
		}
		complete := started
		for i := range outputs {
			passed, err := outputs[i].verdict()
			if err != nil {
				return err
			}
			complete = complete && passed
		}
		if complete {
			// Kill/reap joins the output copiers too, so the final check cannot
			// miss a rejection already read from either peer before teardown.
			watched.stop()
			for i := range outputs {
				if _, err := outputs[i].verdict(); err != nil {
					return err
				}
			}
			netnsSay("WIRE-PASSED: recipient subject and completion fence; both peers held open")
			return nil
		}
		select {
		case exit := <-watched.exits:
			watched.pending--
			return fmt.Errorf("joint-subnet %s exited before the completion fence: %v", exit.label, exit.err)
		case <-poll.C:
		case <-ctx.Done():
			return fmt.Errorf("joint-subnet setup or completion deadline (not a protocol verdict): %w", ctx.Err())
		}
	}
}

// startJointSubnetPeer uses the same locked-thread namespace inheritance as
// startInNetns, adding only a bounded reader of the existing peer verdict lines.
// The caller MUST hand each started child to watchedProcesses and stop it.
func startJointSubnetPeer(ctx context.Context, declaration netnsPeer, namespace *testNetns, orig netns.NsHandle, output *jointSubnetOutput, watched *watchedProcesses) error {
	if err := netns.Set(namespace.ns); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "le", "test", lePeerVerb, declaration.script)
	command.Stdout = output
	command.Stderr = output
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	startErr := command.Start()
	if err := netns.Set(orig); err != nil {
		if startErr == nil {
			command.Process.Kill() //nolint:errcheck // lost namespace thread; terminate child
			command.Wait()         //nolint:errcheck // reaping after forced termination
		}
		return err
	}
	if startErr != nil {
		return startErr
	}
	watched.watch(command, declaration.namespace+" "+declaration.script, false)
	return nil
}
