package fixture

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/dynamic-peer-negotiates-configured-families-trigger", delayedDaemonStopExtra3(5*time.Second))
	Register("plugin/gr-marker-expired-create", writeGRMarkerExtra3(1_000_000_000))
	Register("plugin/gr-marker-restart-create", writeGRMarkerExtra3(4_070_910_976))
	Register("plugin/mgmt-guard-web-dormant-insecure-warns-probe", webListenerProbeExtra3)
	Register("plugin/mgmt-guard-web-env-started-address-binds-probe", webListenerProbeExtra3)
	Register("plugin/peer-port-listener-direct-route-trigger", delayedDaemonStopExtra3(3*time.Second))
	Register("plugin/plugin-cli-debug", pluginCLIDebugExtra3)
	Register("plugin/rbac-ssh-only-enforced", rbacSSHOnlyExtra3)
	Register("plugin/ssh-cli-status-error-exit-code", sshCLIStatusExtra3)
	Register("plugin/ssh-pubkey-auth-setup", sshPubkeySetupExtra3)
	Register("plugin/ssh-pubkey-auth", sshPubkeyAuthExtra3)
}

func delayedDaemonStopExtra3(delay time.Duration) Driver {
	return func(ctx context.Context, _ []string) error {
		for _, name := range []string{"daemon.pid", "daemon.ready"} {
			if !Poll(ctx, 300, 100*time.Millisecond, func() bool {
				_, err := os.Stat(name)
				return err == nil
			}) {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("wait for %s", name)
			}
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		raw, err := os.ReadFile("daemon.pid")
		if err != nil {
			return fmt.Errorf("read daemon pid: %w", err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			return fmt.Errorf("parse daemon pid: %w", err)
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return fmt.Errorf("stop daemon %d: %w", pid, err)
		}
		return nil
	}
}

func writeGRMarkerExtra3(timestamp int64) Driver {
	return func(_ context.Context, _ []string) error {
		if err := os.MkdirAll(filepath.Join("meta", "bgp"), 0o755); err != nil {
			return fmt.Errorf("create marker directory: %w", err)
		}
		marker := []byte{
			byte(timestamp >> 56), byte(timestamp >> 48), byte(timestamp >> 40), byte(timestamp >> 32),
			byte(timestamp >> 24), byte(timestamp >> 16), byte(timestamp >> 8), byte(timestamp),
		}
		if err := os.WriteFile(filepath.Join("meta", "bgp", "gr-marker"), marker, 0o644); err != nil {
			return fmt.Errorf("write GR marker: %w", err)
		}
		return nil
	}
}

func fixturePortExtra3(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("expected one port argument, got %d", len(args))
	}
	if _, err := strconv.ParseUint(args[0], 10, 16); err != nil {
		return "", fmt.Errorf("invalid port %q: %w", args[0], err)
	}
	return args[0], nil
}

func waitExtra3(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func environmentExtra3(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func webListenerProbeExtra3(ctx context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	address := net.JoinHostPort("127.0.0.1", port)
	var lastErr error
	if !Poll(ctx, 120, 250*time.Millisecond, func() bool {
		conn, dialErr := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "tcp", address)
		lastErr = dialErr
		if dialErr == nil {
			_ = conn.Close()
			return true
		}
		return false
	}) {
		return fmt.Errorf("web server never listened on %s: %w", address, lastErr)
	}
	return nil
}

func passwordCredentialsExtra3(port, username, password string) sshclient.Credentials {
	return sshclient.Credentials{Host: "127.0.0.1", Port: port, Username: username, Auth: password}
}

func waitSSHCommandExtra3(ctx context.Context, creds sshclient.Credentials, command string) (string, error) {
	var output string
	var lastErr error
	if !Poll(ctx, 75, 200*time.Millisecond, func() bool {
		output, lastErr = sshclient.ExecCommand(creds, command)
		return lastErr == nil
	}) {
		return "", fmt.Errorf("SSH command %q never succeeded: %w", command, lastErr)
	}
	return output, nil
}

func rawPasswordSSHCommandExtra3(port, username, password, command string) (string, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         2 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", port), config)
	if err != nil {
		return "", fmt.Errorf("connect SSH: %w", err)
	}
	defer client.Close() //nolint:errcheck // fixture cleanup
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck // fixture cleanup
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	runErr := session.Run(command)
	return stdout.String() + stderr.String(), runErr
}

func rbacSSHOnlyExtra3(ctx context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	creds := passwordCredentialsExtra3(port, "noc", "noc-secret")
	output, err := waitSSHCommandExtra3(ctx, creds, "show version")
	if err != nil {
		return fmt.Errorf("show version should be allowed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "OK: 'show version' allowed: %s\n", output)

	message, err := rawPasswordSSHCommandExtra3(port, "noc", "noc-secret", "clear interface counters")
	if err == nil {
		return errors.New("'clear interface counters' must be refused")
	}
	fmt.Fprintf(os.Stderr, "DENIED-OUTPUT: %s\n", message)
	if !strings.Contains(message, "command restricted by access control") {
		return fmt.Errorf("refusal must name access control: %s", message)
	}
	fmt.Fprintln(os.Stderr, "OK: refusal names access control")
	if strings.Contains(message, "error: error:") {
		return fmt.Errorf("doubled error prefix in: %s", message)
	}
	if !strings.Contains(message, "error: command restricted by access control") {
		return fmt.Errorf("refusal did not carry one error prefix: %s", message)
	}
	fmt.Fprintln(os.Stderr, "OK: single error prefix")

	message, err = rawPasswordSSHCommandExtra3(port, "noc", "noc-secret", "show no-such-command-anywhere")
	if err == nil {
		return errors.New("nonexistent command unexpectedly succeeded")
	}
	if !strings.Contains(message, "unknown command") {
		return fmt.Errorf("nonexistent command did not report unknown command: %s", message)
	}
	fmt.Fprintln(os.Stderr, "OK: nonexistent command reports unknown command")
	return nil
}

func sshCLIStatusExtra3(ctx context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	configDir, err := os.MkdirTemp("", "ssh-cli-status-extra-3-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // fixture cleanup

	initCommand := exec.CommandContext(ctx, "ze", "init")
	initCommand.Env = environmentExtra3(map[string]string{"ZE_CONFIG_DIR": configDir})
	initCommand.Stdin = strings.NewReader(fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%s\n", port))
	if output, initErr := initCommand.CombinedOutput(); initErr != nil {
		return fmt.Errorf("ze init: %w\n%s", initErr, output)
	}
	run := func(commandText string) (string, error) {
		command := exec.CommandContext(ctx, "ze", "cli", "-c", commandText)
		command.Env = environmentExtra3(map[string]string{
			"ZE_CONFIG_DIR":   configDir,
			"ZE_SSH_PASSWORD": "testpass",
		})
		output, commandErr := command.CombinedOutput()
		return strings.TrimSpace(string(output)), commandErr
	}
	if output, err := run("request as112 healthcheck"); err == nil {
		return fmt.Errorf("request as112 healthcheck exited 0 for StatusError: %s", output)
	}
	fmt.Fprintln(os.Stderr, "OK: 'request as112 healthcheck' exited non-zero as expected (Status:StatusError mapped to exit code)")
	if output, err := run("show version"); err != nil {
		return fmt.Errorf("show version should exit 0: %w\n%s", err, output)
	}
	fmt.Fprintln(os.Stderr, "OK: 'show version' exited 0 as expected")
	fmt.Fprintln(os.Stderr, "OK: all ssh-cli-status-error-exit-code tests passed")
	return nil
}

func pluginCLIDebugExtra3(ctx context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	creds := passwordCredentialsExtra3(port, "admin", "testpass")
	if err := waitExtra3(ctx, time.Second); err != nil {
		return err
	}
	var session *sshclient.ProtocolSession
	var lastErr error
	if !Poll(ctx, 75, 200*time.Millisecond, func() bool {
		session, lastErr = sshclient.OpenProtocolSession(creds, "plugin protocol")
		return lastErr == nil
	}) {
		return fmt.Errorf("open plugin protocol SSH session: %w", lastErr)
	}
	defer session.Close() //nolint:errcheck // fixture cleanup

	plugin := sdk.NewWithIO("bgp-cli-debug", io.NopCloser(session.Stdout), session.Stdin)
	defer plugin.Close() //nolint:errcheck // fixture cleanup
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	plugin.OnStarted(func(_ context.Context) error {
		go func() {
			defer cancel()
			_, commands, commandErr := plugin.DispatchCommand(runCtx, "show command list")
			if commandErr != nil {
				result <- fmt.Errorf("dispatch show command list: %w", commandErr)
				return
			}
			if !strings.Contains(string(commands), `"commands"`) {
				result <- fmt.Errorf("show command list omitted commands: %s", commands)
				return
			}
			_, help, helpErr := plugin.DispatchCommand(runCtx, "help")
			if helpErr != nil {
				result <- fmt.Errorf("dispatch help: %w", helpErr)
				return
			}
			if !strings.Contains(string(help), "show command list") {
				result <- fmt.Errorf("help omitted show command list: %s", help)
				return
			}
			fmt.Fprintln(os.Stderr, "OK: handshake completed")
			fmt.Fprintln(os.Stderr, "OK: dispatch-command returned command data")
			fmt.Fprintln(os.Stderr, "OK: help command returned command descriptions")
			fmt.Fprintln(os.Stderr, "OK: plugin debug shell test passed")
			result <- nil
		}()
		return nil
	})
	runErr := plugin.Run(runCtx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		if scenarioErr != nil {
			return scenarioErr
		}
		if runErr != nil && runCtx.Err() == nil {
			return runErr
		}
		return nil
	default:
		if runErr == nil {
			return errors.New("plugin protocol ended before the handshake scenario ran")
		}
		return runErr
	}
}

const sshPubkeyConfigExtra3 = `bgp {
	peer peer1 {
		connection { remote { ip 127.0.0.1 } local { ip 127.0.0.1; accept false } }
		session { asn { local 65533; remote 65533 } }
	}
}
system {
	authentication {
		user keyuser {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile admin
			public-keys testkey { type ssh-ed25519; key %s }
		}
	}
}
environment { ssh { enabled true; server main { ip 127.0.0.1; port %s } } }
`

func sshPubkeySetupExtra3(_ context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ed25519 key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return fmt.Errorf("create SSH signer: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "fixture")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	if err := os.WriteFile("ssh-pubkey-testkey", pem.EncodeToMemory(block), 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	fields := strings.Fields(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if len(fields) < 2 {
		return errors.New("marshal public key returned no key data")
	}
	config := fmt.Sprintf(sshPubkeyConfigExtra3, fields[1], port)
	if err := os.WriteFile("ssh-pubkey.conf", []byte(config), 0o644); err != nil {
		return fmt.Errorf("write SSH pubkey config: %w", err)
	}
	return nil
}

func dialPublicKeyExtra3(ctx context.Context, port, username string, signer ssh.Signer) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         2 * time.Second,
	}
	var client *ssh.Client
	var lastErr error
	if !Poll(ctx, 75, 200*time.Millisecond, func() bool {
		client, lastErr = ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", port), config)
		return lastErr == nil
	}) {
		return nil, lastErr
	}
	return client, nil
}

func sshPubkeyAuthExtra3(ctx context.Context, args []string) error {
	port, err := fixturePortExtra3(args)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile("ssh-pubkey-testkey")
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	if err := waitExtra3(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	client, err := dialPublicKeyExtra3(ctx, port, "keyuser", signer)
	if err != nil {
		return fmt.Errorf("matching public key did not authenticate: %w", err)
	}
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("create matching-key session: %w", err)
	}
	_, commandErr := session.CombinedOutput("show bgp peer list")
	_ = session.Close()
	_ = client.Close()
	if commandErr != nil {
		return fmt.Errorf("matching-key command failed: %w", commandErr)
	}
	fmt.Fprintln(os.Stderr, "OK: public key authentication succeeded")

	_, wrongPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate wrong key: %w", err)
	}
	wrongSigner, err := ssh.NewSignerFromKey(wrongPrivate)
	if err != nil {
		return fmt.Errorf("create wrong signer: %w", err)
	}
	wrongConfig := &ssh.ClientConfig{
		User:            "keyuser",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(wrongSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         2 * time.Second,
	}
	wrong, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", port), wrongConfig)
	if err == nil {
		_ = wrong.Close()
		return errors.New("wrong public key authenticated")
	}
	fmt.Fprintln(os.Stderr, "OK: wrong key rejected")
	fmt.Fprintln(os.Stderr, "OK: all SSH public key auth tests passed")
	return nil
}
