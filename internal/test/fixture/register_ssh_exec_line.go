// Design: docs/architecture/api/commands.md -- a piped line on the SSH exec route
// Related: ssh_exec_line.go -- the body of this driver

package fixture

// The driver test/plugin/monitor-pipe-chain-ssh.ci and
// test/plugin/monitor-rendering-key-keeps-selection.ci run: one line on an
// SSH exec channel, with session environment keys. Its body is
// ssh_exec_line.go.
func init() {
	Register("plugin/ssh-exec-line", sshExecLine)
}
