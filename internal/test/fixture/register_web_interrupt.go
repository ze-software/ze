// Design: docs/architecture/web-interface.md — the standalone web server's two-step Ctrl+C

package fixture

// The operator's Ctrl+C at `ze start --web-only`, once or twice. Its body, and
// why it holds a connection and waits for the listener to close between the
// two signals, are web_interrupt_fixture.go.
func init() {
	Register("web/interrupt", webInterrupt)
}
