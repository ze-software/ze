// Design: docs/functional-tests.md — process orchestration, a second daemon started on the first one's file

package fixture

// The barrier a .ci places between a daemon and a second daemon started on its
// file. Its body, and why it reads the process state rather than kill(pid, 0),
// are daemon_await_exit_fixture.go.
func init() {
	Register("daemon/await-exit", daemonAwaitExit)
}
