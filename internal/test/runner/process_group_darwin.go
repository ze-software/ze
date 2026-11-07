// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: process_owner.go -- retire, the post-exit group signal this judges.

package runner

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// darwinZombieStat is XNU's p_stat for an exited, unreaped process
// (bsd/sys/proc.h: "#define SZOMB 5").
const darwinZombieStat = 5

// darwinExitingFlag is the extern_proc p_flag bit for a process that is past
// the point of exit (bsd/sys/proc.h: "#define P_WEXIT 0x00002000"). XNU's
// fill_externproc sets it from the kernel's P_LEXIT, which x/sys/unix omits.
const darwinExitingFlag = 0x00002000

// exitedGroupSignalErr judges the group SIGKILL retire sends after waitid
// observed the leader's exit and before Cmd.Wait reaps it. It MUST be called
// only there: the unreaped leader keeps the group, so the group still exists.
//
// XNU's killpg1 (bsd/kern/kern_sig.c) skips every member whose p_stat is
// SZOMB ("return !(p->p_flag & P_SYSTEM) && p->p_stat != SZOMB;"), then
// answers "error = (karg.nfound > 0 ? 0 : (posix ? EPERM : ESRCH));". Go's
// syscall.Kill passes posix=1, so a group whose only members are zombies,
// the observed leader among them, answers EPERM where Linux answers success.
// EPERM is also what a live member that refuses the signal produces, so the
// group is listed, and a member counts as gone only when darwinMemberGone
// says so. A live member, or a listing that failed, keeps the error.
func exitedGroupSignalErr(pgid int, err error) error {
	return darwinExitedGroupSignalErr(pgid, err, listDarwinProcessGroup)
}

// darwinExitedGroupSignalErr takes the group listing as a parameter so a test
// can present a live member that refused the signal, which an unprivileged
// test cannot create.
func darwinExitedGroupSignalErr(pgid int, err error, list func(int) ([]unix.KinfoProc, error)) error {
	if !errors.Is(err, syscall.EPERM) {
		return err
	}
	members, listErr := list(pgid)
	if listErr != nil {
		return errors.Join(err, fmt.Errorf("list process group %d after EPERM: %w", pgid, listErr))
	}
	for i := range members {
		if !darwinMemberGone(&members[i]) {
			return fmt.Errorf("process group %d keeps live member %d: %w", pgid, members[i].Proc.P_pid, err)
		}
	}
	return nil
}

// darwinMemberGone reports a member no signal can stop, because it is already
// a zombie or already exiting. The exiting case is not a race to tolerate but
// one killpg1 itself creates: proc_prepareexit moves a dying process to the
// zombie list and marks it P_REF_DEAD ("will not be visible via proc_find")
// well before proc_exit sets SZOMB, and pgrp_iterate drops every member
// proc_find cannot see, so killpg answers EPERM while the listing still shows
// p_stat SRUN. Both callers of proc_prepareexit (exit_with_reason and
// proc_exit) set P_LEXIT first, so that member always carries P_WEXIT here.
// A member that refuses the signal and is not exiting has neither mark.
func darwinMemberGone(member *unix.KinfoProc) bool {
	if member.Proc.P_stat == darwinZombieStat {
		return true
	}
	return member.Proc.P_flag&darwinExitingFlag != 0
}

// listDarwinProcessGroup reads kern.proc.pgrp, which XNU's sysctl_prochandle
// fills from both the live and the zombie process lists.
func listDarwinProcessGroup(pgid int) ([]unix.KinfoProc, error) {
	return unix.SysctlKinfoProcSlice("kern.proc.pgrp", pgid)
}
