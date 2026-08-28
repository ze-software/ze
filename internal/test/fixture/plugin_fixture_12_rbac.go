package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

func p12WebRBAC(ctx context.Context, _ []string) error {
	configDir, err := os.MkdirTemp("", "ze-web-rbac-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // best-effort fixture cleanup

	env := p12SetEnv(os.Environ(), "ZE_CONFIG_DIR", configDir)
	if err := p12RunCommand(ctx, env, strings.NewReader("readonly\nreadonlypass\n127.0.0.1\n2222\n"), io.Discard, "ze", "init"); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}
	database := filepath.Join(configDir, "database.zefs")
	usernameFile := filepath.Join(configDir, "zefs-username")
	passwordFile := filepath.Join(configDir, "zefs-password")
	if err := os.WriteFile(usernameFile, []byte("readonly"), 0o600); err != nil {
		return err
	}
	if err := p12RunCommand(ctx, env, nil, io.Discard, "ze", "data", "--path", database, "write", "meta/ssh/{host}/{port}/username", usernameFile); err != nil {
		return fmt.Errorf("write zefs username: %w", err)
	}
	var password bytes.Buffer
	if err := p12RunCommand(ctx, env, nil, &password, "ze", "data", "--path", database, "cat", "meta/ssh/127.0.0.1/2222/password"); err != nil {
		return fmt.Errorf("read zefs password: %w", err)
	}
	if err := os.WriteFile(passwordFile, password.Bytes(), 0o600); err != nil {
		return err
	}
	if err := p12RunCommand(ctx, env, nil, io.Discard, "ze", "data", "--path", database, "write", "meta/ssh/{host}/{port}/password", passwordFile); err != nil {
		return fmt.Errorf("write zefs password: %w", err)
	}

	configPath := filepath.Join(configDir, "web-rbac-deny.conf")
	if err := os.WriteFile(configPath, []byte(p12WebRBACConfig), 0o600); err != nil {
		return err
	}
	env = p12SetEnv(env, "ze_test_bgp_port", strconv.Itoa(12000+os.Getpid()%40000))
	daemonLog, err := os.Create("daemon.log")
	if err != nil {
		return err
	}
	daemon := exec.Command("ze", "start", configPath) //nolint:gosec // fixed fixture executable and arguments
	daemon.Env = env
	daemon.Stdout = os.Stdout
	daemon.Stderr = daemonLog
	if err := daemon.Start(); err != nil {
		daemonLog.Close() //nolint:errcheck
		return fmt.Errorf("start ze: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- daemon.Wait() }()

	checkErr := p12WebRBACCheck(ctx)
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

func p12SetEnv(current []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(current)+1)
	for _, entry := range current {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func p12RunCommand(ctx context.Context, env []string, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // fixed fixture commands
	command.Env = env
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = os.Stderr
	return command.Run()
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

environment {
	web {
		enabled true;
		server main {
			ip 127.0.0.1;
			port 18443;
		}
	}
}

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

func p12WebRBACCheck(ctx context.Context) error {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // local fixture uses an ephemeral self-signed certificate
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	defer transport.CloseIdleConnections()

	request := func(method, path string, fields url.Values) (int, []byte, error) {
		var body io.Reader
		if fields != nil {
			body = strings.NewReader(fields.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, "https://127.0.0.1:18443"+path, body)
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
		defer resp.Body.Close() //nolint:errcheck
		data, readErr := io.ReadAll(resp.Body)
		return resp.StatusCode, data, readErr
	}

	if !Poll(ctx, 20, 500*time.Millisecond, func() bool {
		status, _, err := request(http.MethodGet, "/show/environment/daemon/", nil)
		return err == nil && status >= 200 && status < 300
	}) {
		return fmt.Errorf("web show page did not become available")
	}
	fmt.Fprintln(os.Stderr, "OK: read-only web user can view show page")

	probe := url.Values{"leaf": {"bad/name"}, "value": {"probe"}}
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
		{"/config/set/environment/daemon/", url.Values{"leaf": {"zeuser"}, "value": {"blocked-user"}}, "config set"},
		{"/config/add/bgp/peer/", url.Values{"name": {"blocked-peer"}}, "config add"},
		{"/config/delete/environment/daemon/", url.Values{"leaf": {"zeuser"}}, "config delete"},
		{"/config/rename/bgp/peer/peer1/", url.Values{"new-key": {"peer2"}}, "config rename"},
		{"/config/commit/", url.Values{}, "config commit"},
		{"/config/discard/", url.Values{}, "config discard"},
		{"/cli/terminal", url.Values{"command": {"set zeuser blocked-terminal"}}, "terminal config set"},
		{"/cli", url.Values{"command": {"set zeuser blocked-cli"}, "path": {"environment/daemon"}}, "integrated CLI config set"},
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
