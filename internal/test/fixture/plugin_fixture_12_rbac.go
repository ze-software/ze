package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// p12WebRBAC drives rbac-web-config-deny. Its two arguments are the
// runner-leased $PORT and $PORT2: the BGP port and the web listener port. A
// fixed web port collided with every other test that bound 18443 at the same
// time (as112-doh), so both ports come from the runner's lease, which outlasts
// this function and therefore the daemon it stops and reaps.
func p12WebRBAC(ctx context.Context, args []string) error {
	bgpPort, err := parsePort(args, 0)
	if err != nil {
		return err
	}
	webPort, err := parsePort(args, 1)
	if err != nil {
		return err
	}
	configDir, err := os.MkdirTemp("", "ze-web-rbac-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // best-effort fixture cleanup

	env := childEnvironment(os.Environ(), map[string]string{envConfigDir: configDir})
	if err := p12RunCommand(ctx, env, strings.NewReader("readonly\nreadonlypass\n127.0.0.1\n2222\n"), io.Discard, "init"); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}
	database := filepath.Join(configDir, "database")
	usernameFile := filepath.Join(configDir, "zefs-username")
	passwordFile := filepath.Join(configDir, "zefs-password")
	if err := os.WriteFile(usernameFile, []byte("readonly"), 0o600); err != nil {
		return err
	}
	if err := p12RunCommand(ctx, env, nil, io.Discard, "data", "--path", database, "write", "meta/ssh/{host}/{port}/username", usernameFile); err != nil {
		return fmt.Errorf("write zefs username: %w", err)
	}
	var password bytes.Buffer
	if err := p12RunCommand(ctx, env, nil, &password, "data", "--path", database, "cat", "meta/ssh/127.0.0.1/2222/password"); err != nil {
		return fmt.Errorf("read zefs password: %w", err)
	}
	if err := os.WriteFile(passwordFile, password.Bytes(), 0o600); err != nil {
		return err
	}
	if err := p12RunCommand(ctx, env, nil, io.Discard, "data", "--path", database, "write", "meta/ssh/{host}/{port}/password", passwordFile); err != nil {
		return fmt.Errorf("write zefs password: %w", err)
	}

	configPath := filepath.Join(configDir, "web-rbac-deny.conf")
	if err := os.WriteFile(configPath, []byte(p12WebRBACConfigFor(webPort)), 0o600); err != nil {
		return err
	}
	readyFile := filepath.Join(configDir, "ready")
	env = childEnvironment(env, map[string]string{
		envTestBGPPort:  strconv.Itoa(bgpPort),
		"ZE_READY_FILE": readyFile,
	})
	daemonLog, err := os.Create("daemon.log")
	if err != nil {
		return err
	}
	daemon := exec.CommandContext(ctx, "ze", "start", configPath) //nolint:gosec // the path is the fixture's own scratch file
	daemon.Env = env
	daemon.Stdout = os.Stdout
	daemon.Stderr = daemonLog
	if err := daemon.Start(); err != nil {
		daemonLog.Close() //nolint:errcheck // the start failure below is the error worth reporting
		return fmt.Errorf("start ze: %w", err)
	}
	done := make(chan struct{})
	go func() {
		_ = daemon.Wait()
		close(done)
	}()

	checkErr := p12WaitWebListener(ctx, done, readyFile, "daemon.log", webPort)
	if checkErr == nil {
		checkErr = p12WebRBACCheck(ctx, p12WebRBACBase(webPort))
	}
	_ = daemon.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = daemon.Process.Kill()
		<-done
	}
	_ = daemonLog.Close()
	return checkErr
}

func p12RunCommand(ctx context.Context, env []string, stdin io.Reader, stdout io.Writer, args ...string) error {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = env
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// p12WebRBACListenerLine is the line the daemon prints once its web server has
// bound (startWebServer, cmd/ze/hub/service_web.go), before it writes the
// ready file.
const p12WebRBACListenerLine = "web server listening on "

// p12WebRBACBase is the URL of the web listener on the leased port.
func p12WebRBACBase(webPort int) string {
	return "https://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(webPort))
}

// p12WebRBACConfigFor is the daemon config with the web server on webPort.
func p12WebRBACConfigFor(webPort int) string {
	return p12WebRBACConfig + p12WebRBACEnvironment(webPort) + p12WebRBACBGP
}

// p12WebRBACEnvironment is the config's environment block: the one web server,
// on the leased port.
func p12WebRBACEnvironment(webPort int) string {
	return `environment {
	web {
		enabled true;
		server main {
			ip 127.0.0.1;
			port ` + strconv.Itoa(webPort) + `;
		}
	}
}
`
}

// p12WaitWebListener returns once the daemon has reported its web listener on
// the leased port. The daemon writes the ready file after startup completes,
// and it prints the listener line before that, once the web server has bound,
// so the ready file is the event and the line is what it reports. A daemon
// that started without its web server (a bind failure prints a warning and
// carries on) is refused here, naming the log, rather than polled.
func p12WaitWebListener(ctx context.Context, done <-chan struct{}, readyFile, logPath string, webPort int) error {
	ready := false
	for range WaitAttempts(75, daemonReadyDelay, daemonReadyFallback) {
		if _, err := os.Stat(readyFile); err == nil {
			ready = true
			break
		}
		select {
		case <-done:
			return fmt.Errorf("ze exited before it was ready; see %s", logPath)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(daemonReadyDelay):
		}
	}
	if !ready {
		return fmt.Errorf("ze did not write its ready file %s; see %s", readyFile, logPath)
	}
	log, err := os.ReadFile(logPath) //nolint:gosec // the fixture's own daemon log
	if err != nil {
		return err
	}
	want := p12WebRBACListenerLine + p12WebRBACBase(webPort) + "/"
	if !strings.Contains(string(log), want) {
		return fmt.Errorf("ze is ready but did not report %q in %s", want, logPath)
	}
	return nil
}

const p12WebRBACConfig = `system {
	authentication {
		user readonly {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ readweb ]
		}
	}
	authorization {
		profile readweb {
			run {
				default-action allow
			}
			edit {
				default-action deny
			}
		}
	}
}

`

const p12WebRBACBGP = `
bgp {
	router-id 1.2.3.4
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 1
				remote 1
			}
			router-id 1.2.3.4
			family {
				ipv4/unicast { prefix { maximum 10000; } }
			}
			capability {
				graceful-restart disable
			}
		}
		behavior {
			group-updates disable
		}
	}
}
`

func p12WebRBACCheck(ctx context.Context, base string) error {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // local fixture uses an ephemeral self-signed certificate
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	defer transport.CloseIdleConnections()

	request := func(method, path string, fields url.Values) (int, []byte, error) {
		var body io.Reader
		if fields != nil {
			body = strings.NewReader(fields.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, body)
		if err != nil {
			return 0, nil, err
		}
		req.SetBasicAuth("readonly", "testpass")
		if fields != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close() //nolint:errcheck // the fixture only reads the body, so a close failure changes no assertion
		data, readErr := io.ReadAll(resp.Body)
		return resp.StatusCode, data, readErr
	}

	// The listener is reported bound and startup is complete, so the show
	// page answers now: no poll.
	status, _, err := request(http.MethodGet, "/show/environment/daemon/", nil)
	if err != nil {
		return fmt.Errorf("web show page: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("web show page status=%d, want 2xx", status)
	}
	fmt.Fprintln(os.Stderr, "OK: read-only web user can view show page")

	probe := url.Values{fieldLeaf: {"bad/name"}, fieldValue: {"probe"}}
	if !Poll(ctx, 20, 500*time.Millisecond, func() bool {
		status, _, _ := request(http.MethodPost, "/config/set/environment/daemon/", probe)
		return status == http.StatusForbidden
	}) {
		return fmt.Errorf("web RBAC did not become active")
	}

	checks := []struct {
		path   string
		fields url.Values
		label  string
	}{
		{"/config/set/environment/daemon/", url.Values{fieldLeaf: {"zeuser"}, fieldValue: {"blocked-user"}}, "config set"},
		{"/config/add/bgp/peer/", url.Values{fieldName: {"blocked-peer"}}, "config add"},
		{"/config/delete/environment/daemon/", url.Values{fieldLeaf: {"zeuser"}}, "config delete"},
		{"/config/rename/bgp/peer/peer1/", url.Values{"new-key": {"peer2"}}, "config rename"},
		{"/config/commit/", url.Values{}, "config commit"},
		{"/config/discard/", url.Values{}, "config discard"},
		{"/cli/terminal", url.Values{fieldCommand: {"set zeuser blocked-terminal"}}, "terminal config set"},
		{"/cli", url.Values{fieldCommand: {"set zeuser blocked-cli"}, fieldPath: {"environment/daemon"}}, "integrated CLI config set"},
	}
	for _, check := range checks {
		status, _, err := request(http.MethodPost, check.path, check.fields)
		if err != nil {
			return fmt.Errorf("web %s: %w", check.label, err)
		}
		if status != http.StatusForbidden {
			return fmt.Errorf("web %s status=%d, want 403", check.label, status)
		}
		fmt.Fprintf(os.Stderr, "OK: read-only web %s denied\n", check.label)
	}

	status, body, err := request(http.MethodGet, "/show/environment/daemon/", nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("final show page status=%d", status)
	}
	for _, forbidden := range []string{"blocked-user", "blocked-terminal", "blocked-cli"} {
		if strings.Contains(string(body), forbidden) {
			return fmt.Errorf("denied config set changed rendered config: found %q", forbidden)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: denied web mutations left config unchanged")
	return nil
}
