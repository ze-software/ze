// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.

package runner

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// startDarwinGroupLeader starts script as the leader of its own process group,
// outside the runner's start path, so the test owns every signal it sends.
func startDarwinGroupLeader(t *testing.T, script string) *ownedProcess {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return &ownedProcess{cmd: cmd, done: make(chan struct{})}
}

// TestDarwinZombieOnlyGroupCleanupIsBenign proves the defect at the kernel.
// Method: a leader that exits alone, observed with waitid(WNOWAIT), leaves a
// group of one zombie. XNU answers the real group SIGKILL with EPERM; the
// owner must read that as nothing left to kill, not as a lifecycle failure.
func TestDarwinZombieOnlyGroupCleanupIsBenign(t *testing.T) {
	p := startDarwinGroupLeader(t, "exit 0")
	var killErr error
	p.groupSignal = func(pid int, sig syscall.Signal) error {
		killErr = syscall.Kill(pid, sig)
		return killErr
	}
	observeErr := p.beforeReap()
	if err := p.cmd.Wait(); err != nil {
		t.Fatalf("reap leader: %v", err)
	}
	if !errors.Is(killErr, syscall.EPERM) {
		t.Fatalf("group SIGKILL over a zombie leader = %v, want XNU's EPERM: the premise of this test changed", killErr)
	}
	if observeErr != nil {
		t.Fatalf("zombie-only group cleanup = %v, want nil", observeErr)
	}
}

// TestDarwinGroupEPERMWithLiveMemberFails proves a live member keeps the
// error. Method: the leader exits and leaves a live sleep in its group; the
// signal is replaced by a refusal, which an unprivileged test cannot obtain
// from the kernel, so the real kern.proc.pgrp listing must find the sleep.
func TestDarwinGroupEPERMWithLiveMemberFails(t *testing.T) {
	p := startDarwinGroupLeader(t, "sleep 30 & exit 0")
	p.groupSignal = func(int, syscall.Signal) error { return syscall.EPERM }
	observeErr := p.beforeReap()
	if err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL); err != nil {
		t.Errorf("kill the live member this test left: %v", err)
	}
	if err := p.cmd.Wait(); err != nil {
		t.Fatalf("reap leader: %v", err)
	}
	if !errors.Is(observeErr, syscall.EPERM) {
		t.Fatalf("cleanup with a live member = %v, want the EPERM kept", observeErr)
	}
	if !strings.Contains(observeErr.Error(), "live member") {
		t.Fatalf("cleanup with a live member = %v, want the live member named", observeErr)
	}
}

// TestDarwinExitedGroupSignalErr covers each verdict over a presented group
// listing. Method: a stub lister stands in for kern.proc.pgrp.
func TestDarwinExitedGroupSignalErr(t *testing.T) {
	zombie := unix.KinfoProc{}
	zombie.Proc.P_pid = 100
	zombie.Proc.P_stat = darwinZombieStat
	live := unix.KinfoProc{}
	live.Proc.P_pid = 101
	live.Proc.P_stat = 2 // SRUN in bsd/sys/proc.h.
	// exiting is the member killpg1 skips once proc_prepareexit hid it from
	// proc_find: still SRUN, already P_WEXIT. This is the state the timed-out
	// grandchild of TestForegroundHelperWithAGrandchildTimesOutOnTime is in.
	exiting := unix.KinfoProc{}
	exiting.Proc.P_pid = 102
	exiting.Proc.P_stat = 2
	exiting.Proc.P_flag = darwinExitingFlag
	// flagged is live with an unrelated p_flag bit (P_CONTROLT), so only the
	// exit bit, not any set flag, can excuse a member.
	flagged := unix.KinfoProc{}
	flagged.Proc.P_pid = 103
	flagged.Proc.P_stat = 2
	flagged.Proc.P_flag = 0x00000002
	listErr := errors.New("sysctl refused")
	tests := []struct {
		name    string
		signal  error
		members []unix.KinfoProc
		list    error
		wantErr []error
		wantNil bool
	}{
		{name: "zombies only", signal: syscall.EPERM, members: []unix.KinfoProc{zombie}, wantNil: true},
		{name: "empty group", signal: syscall.EPERM, wantNil: true},
		{name: "exiting member", signal: syscall.EPERM, members: []unix.KinfoProc{zombie, exiting}, wantNil: true},
		{name: "live beside exiting", signal: syscall.EPERM, members: []unix.KinfoProc{exiting, live}, wantErr: []error{syscall.EPERM}},
		{name: "live with other flag", signal: syscall.EPERM, members: []unix.KinfoProc{flagged}, wantErr: []error{syscall.EPERM}},
		{name: "live member", signal: syscall.EPERM, members: []unix.KinfoProc{zombie, live}, wantErr: []error{syscall.EPERM}},
		{name: "listing failed", signal: syscall.EPERM, list: listErr, wantErr: []error{syscall.EPERM, listErr}},
		{name: "other error", signal: syscall.EINVAL, members: []unix.KinfoProc{zombie}, wantErr: []error{syscall.EINVAL}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listed := false
			got := darwinExitedGroupSignalErr(100, tt.signal, func(pgid int) ([]unix.KinfoProc, error) {
				listed = true
				if pgid != 100 {
					t.Errorf("listed group %d, want 100", pgid)
				}
				return tt.members, tt.list
			})
			if tt.wantNil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			for _, want := range tt.wantErr {
				if !errors.Is(got, want) {
					t.Errorf("got %v, want it to wrap %v", got, want)
				}
			}
			if errors.Is(tt.signal, syscall.EPERM) != listed {
				t.Errorf("listed = %v, want a listing only for EPERM", listed)
			}
		})
	}
}
