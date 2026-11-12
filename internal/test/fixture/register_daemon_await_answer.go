// Design: docs/functional-tests.md — process orchestration, waiting for a daemon state over SSH

package fixture

// The barrier a .ci places between a daemon event it cannot order and the
// steps that assert on it over SSH. Its body is daemon_await_answer_fixture.go.
func init() {
	Register("daemon/await-answer", daemonAwaitAnswer)
}
