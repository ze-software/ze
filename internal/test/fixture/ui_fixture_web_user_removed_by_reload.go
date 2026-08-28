package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
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

const webUserRemovedByReloadName = "ui/web-user-removed-by-reload"

func init() {
	Register(webUserRemovedByReloadName, uiDriver(runWebUserRemovedByReload))
}

type webReloadHTTP struct {
	baseURL        string
	client         *http.Client
	loginClient    *http.Client
	transport      *http.Transport
	loginTransport *http.Transport
}

func newWebReloadHTTP(baseURL string) *webReloadHTTP {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // The fixture daemon uses its generated test certificate.
	}
	loginTransport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // The fixture daemon uses its generated test certificate.
		DisableKeepAlives: true,
	}
	return &webReloadHTTP{
		baseURL: baseURL,
		client:  &http.Client{Transport: transport},
		loginClient: &http.Client{
			Transport: loginTransport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		transport:      transport,
		loginTransport: loginTransport,
	}
}

func (h *webReloadHTTP) close() {
	h.transport.CloseIdleConnections()
	h.loginTransport.CloseIdleConnections()
}

func (h *webReloadHTTP) probe(ctx context.Context, headers http.Header) int {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, h.baseURL+"/show/environment/daemon/", nil)
	if err != nil {
		return 0
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func (h *webReloadHTTP) statusFor(ctx context.Context, user, password string) int {
	credential := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	headers := make(http.Header)
	headers.Set("Authorization", "Basic "+credential)
	return h.probe(ctx, headers)
}

func (h *webReloadHTTP) statusForCookie(ctx context.Context, token string) int {
	headers := make(http.Header)
	headers.Set("Cookie", "ze-session="+token)
	return h.probe(ctx, headers)
}

func (h *webReloadHTTP) loginCookie(ctx context.Context, user, password string) (string, bool) {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	body := "username=" + url.QueryEscape(user) + "&password=" + url.QueryEscape(password)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, h.baseURL+"/login", strings.NewReader(body))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.loginClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	for _, value := range resp.Header.Values("Set-Cookie") {
		if !strings.HasPrefix(value, "ze-session=") {
			continue
		}
		pair := strings.SplitN(value, ";", 2)[0]
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			return parts[1], true
		}
	}
	return "", false
}

type webReloadDaemon struct {
	cmd  *exec.Cmd
	done chan error
}

func startWebReloadDaemon(ctx context.Context, dir, configName, logPath string, env []string) (*webReloadDaemon, *os.File, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.CommandContext(ctx, "ze", "start", configName)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, nil, err
	}

	daemon := &webReloadDaemon{cmd: cmd, done: make(chan error, 1)}
	go func() {
		daemon.done <- cmd.Wait()
	}()
	return daemon, logFile, nil
}

func (d *webReloadDaemon) stop() {
	if d == nil || d.cmd == nil || d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	<-d.done
}

func runWebUserRemovedByReload(ctx context.Context) error {
	workspace, err := os.MkdirTemp("", "web-user-removed-by-reload-")
	if err != nil {
		return err
	}
	configDir, err := os.MkdirTemp("", "web-user-removed-by-reload-config-")
	if err != nil {
		_ = os.RemoveAll(workspace)
		return err
	}

	var daemon *webReloadDaemon
	var daemonLog *os.File
	defer func() {
		if daemon != nil {
			daemon.stop()
		}
		if daemonLog != nil {
			_ = daemonLog.Close()
		}
		_ = os.RemoveAll(configDir)
		_ = os.RemoveAll(workspace)
	}()

	webPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	sshPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	baseURL := "https://127.0.0.1:" + strconv.Itoa(webPort)

	fixtureEnv := setWebReloadEnv(os.Environ(), "PWD", workspace)
	fixtureEnv = setWebReloadEnv(fixtureEnv, "ze_test_bgp_port", strconv.Itoa(bgpPort))
	fixtureEnv = setWebReloadEnv(fixtureEnv, "ZE_WEB_URL", baseURL)

	webHash, err := webReloadPasswordHash(ctx, workspace, fixtureEnv, "webuserpass")
	if err != nil {
		return err
	}
	keepHash, err := webReloadPasswordHash(ctx, workspace, fixtureEnv, "keepuserpass")
	if err != nil {
		return err
	}
	newHash, err := webReloadPasswordHash(ctx, workspace, fixtureEnv, "newuserpass")
	if err != nil {
		return err
	}

	usersBothPath := filepath.Join(workspace, "users-both.conf")
	usersKeptPath := filepath.Join(workspace, "users-kept.conf")
	configPath := filepath.Join(workspace, "web-user-reload.conf")
	readyPath := filepath.Join(workspace, "daemon.ready")
	logPath := filepath.Join(workspace, "daemon.log")

	usersBoth := fmt.Sprintf(`system {
	authentication {
		user webuser {
			password "%s"
		}
		user keepuser {
			password "%s"
		}
	}
}
`, webHash, keepHash)
	if err := os.WriteFile(usersBothPath, []byte(usersBoth), 0o666); err != nil {
		return err
	}

	usersKept := fmt.Sprintf(`system {
	authentication {
		user keepuser {
			password "%s"
		}
		user newuser {
			password "%s"
		}
	}
}
`, keepHash, newHash)
	if err := os.WriteFile(usersKeptPath, []byte(usersKept), 0o666); err != nil {
		return err
	}

	if err := writeWebReloadConfig(configPath, usersBothPath, webPort, sshPort); err != nil {
		return err
	}
	if err := os.Remove(readyPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	daemonEnv := setWebReloadEnv(fixtureEnv, "ZE_READY_FILE", readyPath)
	daemonEnv = setWebReloadEnv(daemonEnv, "ZE_CONFIG_DIR", configDir)
	daemon, daemonLog, err = startWebReloadDaemon(ctx, workspace, filepath.Base(configPath), logPath, daemonEnv)
	if err != nil {
		return err
	}

	httpCheck := newWebReloadHTTP(baseURL)
	defer httpCheck.close()

	if err := webReloadBefore(ctx, workspace, httpCheck); err != nil {
		return err
	}

	ready := pollWebReload(ctx, 201, 100*time.Millisecond, func() bool {
		info, statErr := os.Stat(readyPath)
		return statErr == nil && !info.IsDir()
	})
	if !ready {
		err := requireWebReload(false, "daemon never became ready, so the reload could not be signalled")
		dumpWebReloadLog(logPath, false)
		return err
	}

	if err := writeWebReloadConfig(configPath, usersKeptPath, webPort, sshPort); err != nil {
		return err
	}
	if err := daemon.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return err
	}

	if err := webReloadAfter(ctx, workspace, httpCheck); err != nil {
		dumpWebReloadLog(logPath, true)
		return err
	}
	return nil
}

func webReloadBefore(ctx context.Context, workspace string, check *webReloadHTTP) error {
	if err := requireWebReload(
		pollWebReload(ctx, 40, 500*time.Millisecond, func() bool {
			return check.statusFor(ctx, "webuser", "webuserpass") == http.StatusOK
		}),
		"webuser authenticates against the config that declares them",
	); err != nil {
		return err
	}
	if err := requireWebReload(
		check.statusFor(ctx, "keepuser", "keepuserpass") == http.StatusOK,
		"keepuser authenticates against the config that declares them",
	); err != nil {
		return err
	}
	if err := requireWebReload(
		check.statusFor(ctx, "webuser", "wrongpass") == http.StatusUnauthorized,
		"a wrong password is refused, so a 200 above means credentials were checked",
	); err != nil {
		return err
	}

	users := []struct {
		name     string
		password string
	}{
		{name: "webuser", password: "webuserpass"},
		{name: "keepuser", password: "keepuserpass"},
	}
	for _, user := range users {
		token, issued := check.loginCookie(ctx, user.name, user.password)
		if err := requireWebReload(issued, user.name+" logs in through the login form and is issued a session cookie"); err != nil {
			return err
		}
		if err := saveWebReloadCookie(workspace, user.name, token); err != nil {
			return err
		}
		if err := requireWebReload(
			check.statusForCookie(ctx, token) == http.StatusOK,
			user.name+"'s session cookie authenticates on its own, with no password",
		); err != nil {
			return err
		}
	}
	return nil
}

func webReloadAfter(ctx context.Context, workspace string, check *webReloadHTTP) error {
	if err := requireWebReload(
		pollWebReload(ctx, 40, 500*time.Millisecond, func() bool {
			return check.statusFor(ctx, "newuser", "newuserpass") == http.StatusOK
		}),
		"newuser authenticates, so the reload read the rewritten config",
	); err != nil {
		return err
	}
	if err := requireWebReload(
		check.statusFor(ctx, "webuser", "webuserpass") == http.StatusUnauthorized,
		"webuser is refused once the reload removes them from the config",
	); err != nil {
		return err
	}
	if err := requireWebReload(
		check.statusFor(ctx, "keepuser", "keepuserpass") == http.StatusOK,
		"keepuser still authenticates, so the reload removed one user and not the listener",
	); err != nil {
		return err
	}

	webToken, err := loadWebReloadCookie(workspace, "webuser")
	if err != nil {
		return err
	}
	if err := requireWebReload(
		check.statusForCookie(ctx, webToken) == http.StatusUnauthorized,
		"webuser's session cookie is refused once the reload removes them from the config",
	); err != nil {
		return err
	}

	keepToken, err := loadWebReloadCookie(workspace, "keepuser")
	if err != nil {
		return err
	}
	return requireWebReload(
		check.statusForCookie(ctx, keepToken) == http.StatusOK,
		"keepuser's session cookie still works, so the reload ended one session and not every session",
	)
}

func webReloadPasswordHash(ctx context.Context, dir string, env []string, password string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(password + "\n")
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w", err)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func writeWebReloadConfig(configPath, usersPath string, webPort, sshPort int) error {
	users, err := os.ReadFile(usersPath)
	if err != nil {
		return err
	}
	remainder := fmt.Sprintf(`
environment {
	ssh {
		enabled true;
		server main { ip 127.0.0.1; port %d; }
	}
	web {
		enabled true;
		server main { ip 127.0.0.1; port %d; }
	}
}

bgp {
	router-id 1.2.3.4
	peer peer1 {
		connection {
			remote { ip 127.0.0.1; }
			local { ip 127.0.0.1; accept false; }
		}
		session {
			asn { local 1; remote 1; }
			router-id 1.2.3.4
			family { ipv4/unicast { prefix { maximum 10000; } } }
			capability { graceful-restart disable; }
		}
		behavior { group-updates disable; }
	}
}
`, sshPort, webPort)
	contents := make([]byte, 0, len(users)+len(remainder))
	contents = append(contents, users...)
	contents = append(contents, remainder...)
	return os.WriteFile(configPath, contents, 0o666)
}

func saveWebReloadCookie(workspace, user, token string) error {
	return os.WriteFile(filepath.Join(workspace, "cookie-"+user+".txt"), []byte(token), 0o666)
}

func loadWebReloadCookie(workspace, user string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(workspace, "cookie-"+user+".txt"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(contents)), nil
}

func requireWebReload(condition bool, message string) error {
	if !condition {
		fmt.Fprintln(os.Stderr, "FAIL: "+message)
		return fmt.Errorf("%s", message)
	}
	fmt.Fprintln(os.Stderr, "OK: "+message)
	return nil
}

func pollWebReload(ctx context.Context, attempts int, delay time.Duration, observe func() bool) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		if observe() {
			return true
		}
		if attempt+1 == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false
		case <-timer.C:
		}
	}
	return false
}

func dumpWebReloadLog(path string, withHeader bool) {
	if withHeader {
		fmt.Fprintln(os.Stderr, "--- daemon.log ---")
	}
	contents, err := os.ReadFile(path)
	if err == nil {
		_, _ = os.Stderr.Write(contents)
	}
}

func setWebReloadEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
