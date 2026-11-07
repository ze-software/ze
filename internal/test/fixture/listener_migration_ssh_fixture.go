// Design: docs/guide/configuration.md -- SSH listeners: all or none, and on reload
// Related: register_listener_migration.go -- the registration
// Related: listener_migration_fixture.go -- the web, looking-glass and MCP sibling
// Related: lg_pki_fixture.go -- lgPKIDaemon, the daemon this scenario signals

package fixture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sshIdentificationPrefix opens the identification line an SSH-2.0 server
// sends as soon as it accepts a connection (RFC 4253 Section 4.2), before the
// client says anything. Reading it proves an SSH server owns the socket.
const sshIdentificationPrefix = "SSH-2.0-"

// listenerMigrationSSH proves a SIGHUP reload that moves the SSH listener from
// 127.0.0.1 to ::1 (AC-12 of the home-CPE system defects spec) takes effect in
// the running daemon, through the production wiring: the hub's attachSSH hands
// the built server to the listener migrator, and the reload's buildChanges moves
// it.
//
// The two ports are the runner's lease ($PORT and $PORT2), so a sibling test
// never collides with them.
//
// Method: start one daemon with SSH on 127.0.0.1, wait until that address sends
// an SSH identification line, rewrite the config to name ::1 on another port and
// SIGHUP. Once the daemon prints the accepted verdict, ::1 on the new port must
// send the identification line and 127.0.0.1 on the old port must refuse the
// connection. Every wait is on state (a line read, a verdict printed), never on
// a duration.
func listenerMigrationSSH(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: reload/listener-migration-ssh <port-before> <port-after>, the two ports the runner leased, got %d arguments", len(args))
	}
	before, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("port-before %q: %w", args[0], err)
	}
	after, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("port-after %q: %w", args[1], err)
	}
	oldAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(before))
	newAddress := net.JoinHostPort("::1", strconv.Itoa(after))

	root, err := os.MkdirTemp("", "listener-migration-ssh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	daemon, err := startSSHMigrationDaemon(ctx, filepath.Join(root, "hub.conf"), sshMigrationConfig("127.0.0.1", before), oldAddress)
	if err != nil {
		return err
	}
	defer daemon.stop()

	accepted, err := daemon.reload(ctx, sshMigrationConfig("::1", after))
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("the reload moving the SSH listener from %s to %s was refused:\n%s", oldAddress, newAddress, daemon.output.String())
	}

	if err := readSSHIdentification(ctx, newAddress); err != nil {
		return fmt.Errorf("after reload: the SSH listener on the new address: %w\n%s", err, daemon.output.String())
	}

	dialer := net.Dialer{Timeout: listenerMigrationIOTimeout}
	stale, err := dialer.DialContext(ctx, "tcp", oldAddress)
	if err == nil {
		_ = stale.Close() //nolint:errcheck // the accept is the failure being reported
		return fmt.Errorf("after reload: SSH still accepts connections on the old address %s", oldAddress)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("after reload: SSH old address %s: want connection refused, got %w", oldAddress, err)
	}

	fmt.Println("OK: ssh moved from " + oldAddress + " to " + newAddress + "; the new address sends the SSH identification and the old one refuses")
	return nil
}

// sshMigrationConfig is the daemon config: one local user, so SSH has an
// authenticator, and SSH listening on ip and port.
func sshMigrationConfig(ip string, port int) string {
	var tb textbuf.Buffer
	tb.Str("system {\n\tauthentication {\n\t\tuser admin {\n")
	// bcrypt cost-4 hash of "testpass", the hash test/plugin/authz-allow.ci uses.
	tb.Str("\t\t\tpassword \"$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO\"\n")
	tb.Str("\t\t}\n\t}\n}\n\nenvironment {\n\tssh {\n\t\tenabled true\n")
	tb.Str("\t\tserver main {\n\t\t\tip ").Str(ip).Str(";\n\t\t\tport ").Int(int64(port)).Str(";\n\t\t}\n\t}\n}\n")
	return tb.String()
}

// startSSHMigrationDaemon writes the config, starts ze, and returns once
// address sends an SSH identification line.
func startSSHMigrationDaemon(ctx context.Context, path, config, address string) (*lgPKIDaemon, error) {
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return nil, err
	}
	configDir, err := os.MkdirTemp("", "listener-migration-ssh-daemon-")
	if err != nil {
		return nil, err
	}
	daemon := &lgPKIDaemon{output: &lockedBuffer{}, path: path, configDir: configDir}
	command := exec.CommandContext(ctx, "ze", actionStart, path) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = miscEnvironment(map[string]string{envConfigDir: configDir})
	command.Stdout = daemon.output
	command.Stderr = daemon.output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(configDir) //nolint:errcheck // the start already failed
		return nil, err
	}
	daemon.command = command
	daemon.done = make(chan error, 1)
	// Lifecycle goroutine (one-time Process.Wait bridge): Wait must be called
	// exactly once, and stop() reads the channel it sends to.
	go daemon.reap()

	attempts := lgPKIStartAttempts()
	ready := Poll(ctx, attempts, lgPKIPollDelay, func() bool {
		return readSSHIdentification(ctx, address) == nil
	})
	if !ready {
		daemon.stop()
		waited := time.Duration(attempts) * lgPKIPollDelay
		return nil, fmt.Errorf("SSH never answered on %s within %s:\n%s", address, waited, daemon.output.String())
	}
	return daemon, nil
}

// readSSHIdentification connects to address and reads the server's first line.
// It answers nil only when that line is an SSH-2.0 identification.
func readSSHIdentification(ctx context.Context, address string) error {
	dialer := net.Dialer{Timeout: listenerMigrationIOTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial %s: %w", address, err)
	}
	defer conn.Close() //nolint:errcheck // probe teardown

	if err := conn.SetDeadline(time.Now().Add(listenerMigrationIOTimeout)); err != nil {
		return err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read the identification line from %s: %w", address, err)
	}
	if !strings.HasPrefix(line, sshIdentificationPrefix) {
		return fmt.Errorf("%s answered %q, want a line opening %q", address, strings.TrimSpace(line), sshIdentificationPrefix)
	}
	return nil
}
