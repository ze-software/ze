//go:build linux

// Design: docs/functional-tests.md -- state-fenced namespace process ownership.
// Related: plugin_fixture_scoped_peer_prefix_linux.go -- the retained TCP topology.
// Related: reactor_address_watch.go -- addressSync refresh-before-listener ordering.
package fixture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"

	"github.com/vishvananda/netns"
)

var scopedPrefixSentinels = [2]string{"2001:db8:5731::1", "2001:db8:5732::1"}

// scopedPrefixLog preserves daemon output and recognizes only a complete
// listener-start line for a fresh configured sentinel. Safe for concurrent
// stdout/stderr copiers. Each line is bounded; overflow fails the fixture.
type scopedPrefixLog struct {
	mu        sync.Mutex
	sink      io.Writer
	line      [16384]byte
	used      int
	listeners [2]bool
	removals  [2]bool
}

func (o *scopedPrefixLog) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.sink.Write(data); err != nil {
		return 0, err
	}
	for _, value := range data {
		if value == '\n' {
			line := o.line[:o.used]
			if bytes.Contains(line, []byte(`msg="iface: address added, starting listeners"`)) {
				if bytes.Contains(line, []byte("subsystem=bgp.reactor ")) {
					for index, sentinel := range scopedPrefixSentinels {
						if bytes.Contains(line, []byte("address="+sentinel+" ")) {
							o.listeners[index] = true
						}
					}
				}
			}
			if bytes.Contains(line, []byte(`msg="iface: address removed, stopping listener"`)) {
				if bytes.Contains(line, []byte("subsystem=bgp.reactor ")) {
					for index, sentinel := range scopedPrefixSentinels {
						if bytes.Contains(line, []byte("address="+sentinel+" ")) {
							o.removals[index] = true
						}
					}
				}
			}
			o.used = 0
			continue
		}
		if o.used == len(o.line) {
			return 0, fmt.Errorf("scoped-prefix daemon output exceeded bounded line length")
		}
		o.line[o.used] = value
		o.used++
	}
	return len(data), nil
}

func (o *scopedPrefixLog) listener(index int) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.listeners[index]
}

func (o *scopedPrefixLog) removed(index int) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.removals[index]
}

// scopedPrefixRun keeps every source and the recipient alive until all three
// subject/control pairs and the final public status read are complete. No
// source withdrawal or reconnect is allowed to substitute for prefix refresh.
func scopedPrefixRun(ctx context.Context, plan *netnsRunPlan, path *clampedPath, orig netns.NsHandle) error {
	watched := &watchedProcesses{exits: make(chan processExit, 5)}
	defer watched.stop()
	var outputs [4]jointSubnetOutput
	if err := startJointSubnetPeer(ctx, plan.peers[0], path.router, orig, &outputs[0], watched); err != nil {
		return err
	}
	log := &scopedPrefixLog{sink: os.Stdout}
	command, err := scopedPrefixStartDaemon(ctx, plan, path.sender, orig, log)
	if err != nil {
		return err
	}
	watched.watch(command, "scoped-prefix speaker", true)
	for phase := range 3 {
		source := plan.peers[phase+1]
		if source.after == "" {
			return fmt.Errorf("scoped-prefix source phase %d needs an explicit previous-state fence", phase+1)
		}
		if err := scopedPrefixAwait(ctx, watched, func() (bool, error) {
			return peerFileExists(source.after)
		}); err != nil {
			return err
		}
		if phase == 0 {
			if err := scopedPrefixRemoveSentinels(path.sender); err != nil {
				return err
			}
			// MUST observe removal before a later addition can fence refresh.
			for index, sentinel := range scopedPrefixSentinels {
				if err := scopedPrefixAwait(ctx, watched, func() (bool, error) {
					return log.removed(index), nil
				}); err != nil {
					return fmt.Errorf("scoped-prefix no startup listener removal for %s: %w", sentinel, err)
				}
			}
		}
		if phase > 0 {
			sentinel := scopedPrefixSentinels[phase-1]
			if err := scopedPrefixChange(path.sender, phase == 2, sentinel); err != nil {
				return err
			}
			if err := scopedPrefixAwait(ctx, watched, func() (bool, error) {
				return log.listener(phase - 1), nil
			}); err != nil {
				return fmt.Errorf("scoped-prefix no post-refresh listener fence for %s: %w", sentinel, err)
			}
			netnsSay("REFRESH-FENCED: ", sentinel)
		}
		if err := startJointSubnetPeer(ctx, source, path.far, orig, &outputs[phase+1], watched); err != nil {
			return err
		}
		statusFile := scopedPrefixStatusFile + strconv.Itoa(phase+1)
		if err := scopedPrefixAwait(ctx, watched, func() (bool, error) {
			return peerFileExists(statusFile)
		}); err != nil {
			return fmt.Errorf("scoped-prefix phase %d wire/status fence: %w", phase+1, err)
		}
	}
	if err := scopedPrefixAwait(ctx, watched, func() (bool, error) {
		for index := range outputs {
			passed, err := outputs[index].verdict()
			if err != nil {
				return false, err
			}
			if !passed {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return err
	}
	// Stop joins all output copiers; a late rejection must still retract success.
	watched.stop()
	for index := range outputs {
		if _, err := outputs[index].verdict(); err != nil {
			return err
		}
	}
	netnsSay("WIRE-PASSED: scoped recipient prefix present/removed/restored on one TCP connection")
	return nil
}

// scopedPrefixAwait waits on a named state, while a child failure or the one
// owning deadline ends the wait. The poll interval is shared with the existing
// namespace fixture; it measures no protocol settling delay.
func scopedPrefixAwait(ctx context.Context, watched *watchedProcesses, predicate func() (bool, error)) error {
	var failure error
	ready := Poll(ctx, 1000, clampedPeerPoll, func() bool {
		select {
		case exit := <-watched.exits:
			watched.pending--
			if exit.err == nil {
				failure = fmt.Errorf("scoped-prefix %s exited before completion", exit.label)
			} else {
				failure = fmt.Errorf("scoped-prefix %s exited before completion: %w", exit.label, exit.err)
			}
			return true
		default:
		}
		complete, err := predicate()
		if err != nil {
			failure = err
			return true
		}
		return complete
	})
	if failure != nil {
		return failure
	}
	if !ready {
		return fmt.Errorf("scoped-prefix state fence was not reached before the owning deadline: %w", ctx.Err())
	}
	return nil
}

// scopedPrefixStartDaemon is startInNetns with bounded output observation. The
// caller MUST give the child to watchedProcesses, whose Wait joins its copiers.
func scopedPrefixStartDaemon(ctx context.Context, plan *netnsRunPlan, namespace *testNetns, orig netns.NsHandle, output io.Writer) (*exec.Cmd, error) {
	if err := netns.Set(namespace.ns); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, plan.command[0], plan.command[1:]...) //nolint:gosec // argv belongs to the runner's own fixture declaration
	command.Stdout = output
	command.Stderr = output
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	startErr := command.Start()
	if err := netns.Set(orig); err != nil {
		if startErr == nil {
			command.Process.Kill() //nolint:errcheck // lost namespace thread; stop the child
			command.Wait()         //nolint:errcheck // reap after forced termination
		}
		return nil, err
	}
	if startErr != nil {
		return nil, startErr
	}
	return command, nil
}
