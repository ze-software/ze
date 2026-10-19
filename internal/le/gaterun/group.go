// Design: docs/architecture/testing/qemu-integration.md -- the docker population's signalled stop
// Related: gaterun.go -- StreamContext, whose child heads the group this file awaits.

package gaterun

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// groupPollInterval is how often AwaitExit asks whether the group is empty.
const groupPollInterval = 20 * time.Millisecond

// groupListTimeout bounds the one ps call that names the members left.
const groupListTimeout = 5 * time.Second

// ProcessGroup is the process group a StreamContext child headed (Setpgid), so
// its id is the child's pid. Reaping the child does not end the group: a `go
// test` child's test binaries are members too, and after a SIGKILL of the
// group they may still be exiting when the leader is reaped. The zero value is
// a child that never started, and heads no group.
type ProcessGroup struct {
	leader int
}

// Leader answers the group id, and false for a child that never started.
func (g ProcessGroup) Leader() (int, bool) {
	return g.leader, g.leader > 0
}

// AwaitExit waits until no process of the group is left, for at most bound and
// never past ctx. It answers nil once the group is empty, and for a child that
// never started. When it stops waiting with members left, the error names
// their pids, or says why they could not be listed. A group that empties while
// those members are being listed is empty, so that answer is nil too.
//
// It polls kill(-group, 0), which POSIX answers with ESRCH once no member
// remains, on Linux and Darwin alike. EPERM means a member exists that this
// process may not signal, so it counts as present.
func (g ProcessGroup) AwaitExit(ctx context.Context, bound time.Duration) error {
	// leader 0 is the guard against kill(0, 0), which would probe this
	// process's own group and never answer ESRCH.
	if g.leader <= 0 {
		return nil
	}
	wait, cancel := context.WithTimeout(ctx, bound)
	defer cancel()

	ticker := time.NewTicker(groupPollInterval)
	defer ticker.Stop()

	for groupPresent(g.leader) {
		select {
		case <-wait.Done():
			return g.leftError(ctx, bound)
		case <-ticker.C:
		}
	}
	return nil
}

// groupPresent reports whether any process of group remains.
func groupPresent(group int) bool {
	return !errors.Is(syscall.Kill(-group, 0), syscall.ESRCH)
}

// leftError names the members still in the group after bound. The last
// member can exit between the final poll and the ps listing, so the group is
// asked again once the listing is in: empty then, it answers nil, since a
// warning would name a pid nothing holds any more.
func (g ProcessGroup) leftError(ctx context.Context, bound time.Duration) error {
	members, err := g.members(ctx)
	if !groupPresent(g.leader) {
		return nil
	}
	var text textbuf.Buffer
	text.Str("process group ").Int(int64(g.leader)).Str(" still holds ")
	if err != nil {
		return errors.New(text.Str("members that could not be listed (").Err(err).Str(") after ").
			Str(bound.String()).String())
	}
	if len(members) == 0 {
		return errors.New(text.Str("members that ps did not list (one that joined after the listing, or one hidden from this user) after ").
			Str(bound.String()).String())
	}
	text.Str("pids")
	for index, pid := range members {
		if index > 0 {
			text.Byte(',')
		}
		text.Byte(' ').Int(int64(pid))
	}
	return errors.New(text.Str(" after ").Str(bound.String()).String())
}

// members lists the pids whose process group is g, from `ps -A -o pid=,pgid=`,
// which Linux procps and the BSD ps of Darwin both accept. The poll and this
// list race a member that exits in between, so the list may be empty.
func (g ProcessGroup) members(ctx context.Context) ([]int, error) {
	list, cancel := context.WithTimeout(context.WithoutCancel(ctx), groupListTimeout)
	defer cancel()

	output, err := exec.CommandContext(list, "ps", "-A", "-o", "pid=,pgid=").Output()
	if err != nil {
		return nil, err
	}
	var members []int
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] != strconv.Itoa(g.leader) {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		members = append(members, pid)
	}
	return members, nil
}
