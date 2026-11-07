// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: process_group_darwin.go -- the Darwin EPERM this platform never needs.

//go:build !darwin

package runner

// exitedGroupSignalErr judges the group SIGKILL retire sends after the leader's
// exit was observed and before it is reaped. Linux's kill_pgrp signals a zombie
// member and answers success, and retire already accepts ESRCH, so here every
// other error is a member the owner failed to kill and reaches it unchanged.
func exitedGroupSignalErr(_ int, err error) error {
	return err
}
