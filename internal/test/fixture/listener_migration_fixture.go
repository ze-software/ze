// Design: docs/architecture/web-interface.md -- listener migration on reload
// Related: register_listener_migration.go -- the registration
// Related: lg_pki_fixture.go -- lgPKIDaemon, the daemon this scenario signals

package fixture

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// listenerMigrationIOTimeout bounds one dial or one request, so a listener that
// accepts and never answers fails the scenario rather than hanging it.
const listenerMigrationIOTimeout = 5 * time.Second

// listenerMigrationService is one management listener the reload moves.
type listenerMigrationService struct {
	name   string // the environment container and the report label
	banner string // the line the daemon prints when it binds, before the scheme
	tls    bool   // HTTPS: the web server always, the looking glass by default
	path   string // a path the service answers without credentials
	before int    // the port the daemon starts on
	after  int    // the port the reloaded config names
	status int    // the HTTP status the before port answered path with
}

// listenerMigrationConn is one connection held open across the reload, with
// the reader that owns whatever the last response left buffered.
type listenerMigrationConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// listenerMigration proves a SIGHUP reload that changes the web, looking-glass
// and MCP listen ports moves each listener with no restart.
//
// Method: start one daemon with the three services on three ports, open one
// connection to each and get an answer on it, rewrite the config with three new
// ports and SIGHUP. Then, for each service: the new port answers, the old port
// refuses a new connection, and the connection held across the reload still
// answers, because the migration closes the old listener and not the
// connections it had accepted.
func listenerMigration(ctx context.Context, _ []string) error {
	services := []*listenerMigrationService{
		{name: "web", banner: "web server listening on", tls: true, path: "/"},
		{name: "looking-glass", banner: "looking glass listening on", tls: true, path: lgPKIStatusPath},
		{name: "mcp", banner: "MCP server listening on", path: "/"},
	}
	for _, service := range services {
		var err error
		if service.before, err = availablePort09(); err != nil {
			return err
		}
		if service.after, err = availablePort09(); err != nil {
			return err
		}
	}

	root, err := os.MkdirTemp("", "listener-migration-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	daemon, err := startListenerMigrationDaemon(ctx, filepath.Join(root, "hub.conf"), services)
	if err != nil {
		return err
	}
	defer daemon.stop()

	held := make([]*listenerMigrationConn, len(services))
	for index, service := range services {
		conn, err := dialListenerMigration(ctx, service, service.before)
		if err != nil {
			return fmt.Errorf("before reload: %w", err)
		}
		defer conn.close()
		if service.status, err = conn.get(service.path); err != nil {
			return fmt.Errorf("before reload: %s on port %d: %w", service.name, service.before, err)
		}
		if service.status == http.StatusBadRequest {
			return fmt.Errorf("before reload: %s on port %d answered 400: the probe speaks the wrong protocol", service.name, service.before)
		}
		held[index] = conn
	}

	accepted, err := daemon.reload(ctx, listenerMigrationConfig(services, true))
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("the reload moving the listeners was refused:\n%s", daemon.output.String())
	}

	for index, service := range services {
		if err := assertListenerMigrated(ctx, service, held[index]); err != nil {
			return err
		}
		fmt.Println("OK: " + service.name + " moved from port " + strconv.Itoa(service.before) + " to " + strconv.Itoa(service.after) + "; new port and held connection answer " + service.path + " with " + strconv.Itoa(service.status))
	}
	return nil
}

// assertListenerMigrated checks one service after the reload: the new port
// answers, the old port refuses, and the connection held across it answers.
func assertListenerMigrated(ctx context.Context, service *listenerMigrationService, held *listenerMigrationConn) error {
	fresh, err := dialListenerMigration(ctx, service, service.after)
	if err != nil {
		return fmt.Errorf("after reload: %s new port: %w", service.name, err)
	}
	defer fresh.close()
	status, err := fresh.get(service.path)
	if err != nil {
		return fmt.Errorf("after reload: %s on new port %d: %w", service.name, service.after, err)
	}
	if status != service.status {
		return fmt.Errorf("after reload: %s on new port %d answered %d, the old port answered %d", service.name, service.after, status, service.status)
	}

	dialer := net.Dialer{Timeout: listenerMigrationIOTimeout}
	stale, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(service.before)))
	if err == nil {
		_ = stale.Close() //nolint:errcheck // the accept is the failure being reported
		return fmt.Errorf("after reload: %s old port %d still accepts connections", service.name, service.before)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("after reload: %s old port %d: want connection refused, got %w", service.name, service.before, err)
	}

	status, err = held.get(service.path)
	if err != nil {
		return fmt.Errorf("after reload: %s connection held from port %d: %w", service.name, service.before, err)
	}
	if status != service.status {
		return fmt.Errorf("after reload: %s connection held from port %d answered %d, before the reload %d", service.name, service.before, status, service.status)
	}
	return nil
}

// listenerMigrationConfig is the daemon config: one user the web server can
// authenticate, and the three services on their before or after ports.
func listenerMigrationConfig(services []*listenerMigrationService, moved bool) string {
	var tb textbuf.Buffer
	tb.Str("system {\n\tauthentication {\n\t\tuser webadmin {\n")
	// bcrypt cost-4 hash of "testpass", the hash test/plugin/authz-allow.ci uses.
	tb.Str("\t\t\tpassword \"$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO\"\n")
	tb.Str("\t\t\tprofile [ admin ]\n\t\t}\n\t}\n}\n\nenvironment {\n")
	for _, service := range services {
		port := service.before
		if moved {
			port = service.after
		}
		tb.Str("\t").Str(service.name).Str(" {\n\t\tenabled true\n")
		tb.Str("\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").Int(int64(port)).Str(";\n\t\t}\n\t}\n")
	}
	tb.Str("}\n")
	return tb.String()
}

// startListenerMigrationDaemon writes the config, starts ze, and returns once
// every service has announced its listener on its before port.
func startListenerMigrationDaemon(ctx context.Context, path string, services []*listenerMigrationService) (*lgPKIDaemon, error) {
	if err := os.WriteFile(path, []byte(listenerMigrationConfig(services, false)), 0o600); err != nil {
		return nil, err
	}
	configDir, err := os.MkdirTemp("", "listener-migration-daemon-")
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

	needles := make([]string, 0, len(services))
	for _, service := range services {
		needles = append(needles, service.banner+" ")
	}
	attempts := lgPKIStartAttempts()
	ready := Poll(ctx, attempts, lgPKIPollDelay, func() bool {
		output := daemon.output.String()
		for index, service := range services {
			if !strings.Contains(output, needles[index]) {
				return false
			}
			if !strings.Contains(output, "127.0.0.1:"+strconv.Itoa(service.before)+"/") {
				return false
			}
		}
		return true
	})
	if !ready {
		daemon.stop()
		waited := time.Duration(attempts) * lgPKIPollDelay
		return nil, fmt.Errorf("the daemon did not announce all three listeners within %s:\n%s", waited, daemon.output.String())
	}
	return daemon, nil
}

// dialListenerMigration opens one connection to a service, over TLS for the
// web server and the looking glass. Both certificates are self-signed, and the scenario asserts
// which listener answers, not who it is, so the client does not verify it.
func dialListenerMigration(ctx context.Context, service *listenerMigrationService, port int) (*listenerMigrationConn, error) {
	dialer := net.Dialer{Timeout: listenerMigrationIOTimeout}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial %s on port %d: %w", service.name, port, err)
	}
	if service.tls {
		client := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // self-signed listener; the scenario asserts reachability, not identity
		if err := client.HandshakeContext(ctx); err != nil {
			_ = conn.Close() //nolint:errcheck // the handshake already failed
			return nil, fmt.Errorf("TLS handshake with %s on port %d: %w", service.name, port, err)
		}
		conn = client
	}
	return &listenerMigrationConn{conn: conn, reader: bufio.NewReader(conn)}, nil
}

// get sends one keep-alive GET on the held connection and answers the status.
// Any HTTP status is an answer: the question is whether this listener serves.
func (c *listenerMigrationConn) get(path string) (int, error) {
	if err := c.conn.SetDeadline(time.Now().Add(listenerMigrationIOTimeout)); err != nil {
		return 0, err
	}
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1"+path, http.NoBody) //nolint:noctx // the deadline belongs to the connection
	if err != nil {
		return 0, err
	}
	request.Header.Set("Connection", "keep-alive")
	if err := request.Write(c.conn); err != nil {
		return 0, fmt.Errorf("write %s: %w", path, err)
	}
	response, err := http.ReadResponse(c.reader, request)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		_ = response.Body.Close() //nolint:errcheck // the read already failed
		return 0, err
	}
	if err := response.Body.Close(); err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (c *listenerMigrationConn) close() {
	_ = c.conn.Close() //nolint:errcheck // fixture teardown
}
