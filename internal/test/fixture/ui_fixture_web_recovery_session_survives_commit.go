package fixture

import (
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

	"github.com/ze-software/ze/internal/core/textbuf"
)

const webRecoverySessionFixture = "ui/web-recovery-session-survives-commit"

func init() {
	Register(webRecoverySessionFixture, uiDriver(runWebRecoverySessionSurvivesCommit))
}

type webRecoveryResponse struct {
	status *int
	body   string
}

type webRecoveryField struct {
	name  string
	value string
}

type webRecoveryClient struct {
	ctx     context.Context
	baseURL string
}

func runWebRecoverySessionSurvivesCommit(ctx context.Context) error {
	workDir, err := os.MkdirTemp("", "ze-web-recovery-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workDir) //nolint:errcheck // fixture cleanup

	zeDir := filepath.Join(workDir, "zefs")
	if err := os.Mkdir(zeDir, 0o700); err != nil {
		return fmt.Errorf("create ZE_CONFIG_DIR: %w", err)
	}

	webPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	baseURL := "https://127.0.0.1:" + strconv.Itoa(webPort)
	commonEnv := webRecoveryEnvironment(os.Environ(),
		"PWD="+workDir,
		"ZE_WEB_URL="+baseURL,
	)

	initCommand := exec.CommandContext(ctx, "ze", "init")
	initCommand.Dir = workDir
	initCommand.Env = webRecoveryEnvironment(commonEnv, "ZE_CONFIG_DIR="+zeDir)
	initCommand.Stdin = strings.NewReader("recovery-admin\nrecoverypass\n127.0.0.1\n2222\n")
	initCommand.Stdout = io.Discard
	initCommand.Stderr = os.Stderr
	if err := initCommand.Run(); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}

	configPath := filepath.Join(workDir, "web-recovery.conf")
	config := fmt.Sprintf(`environment {
	web {
		enabled true;
		server main { ip 127.0.0.1; port %d; }
	}
}
`, webPort)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write web-recovery.conf: %w", err)
	}

	readyPath := filepath.Join(workDir, "daemon.ready")
	if err := os.Remove(readyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale readiness marker: %w", err)
	}

	logPath := filepath.Join(workDir, "daemon.log")
	logFile, err := os.Create(logPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("create daemon.log: %w", err)
	}
	defer logFile.Close() //nolint:errcheck // fixture teardown

	daemon := exec.CommandContext(ctx, "ze", "start", "web-recovery.conf")
	daemon.Dir = workDir
	daemon.Env = webRecoveryEnvironment(commonEnv,
		"ZE_READY_FILE="+readyPath,
		"ZE_CONFIG_DIR="+zeDir,
		"ze.log.plugin.server=info",
	)
	daemon.Stdout = os.Stdout
	daemon.Stderr = logFile
	daemon.Cancel = func() error {
		if daemon.Process == nil {
			return nil
		}
		return daemon.Process.Signal(syscall.SIGTERM)
	}
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("ze start: %w", err)
	}

	daemonDone := make(chan error, 1)
	go func() {
		daemonDone <- daemon.Wait()
	}()
	defer func() {
		if daemon.Process != nil {
			_ = daemon.Process.Signal(syscall.SIGTERM)
		}
		<-daemonDone
	}()

	ready := Poll(ctx, 300, 100*time.Millisecond, func() bool {
		info, err := os.Stat(readyPath)
		return err == nil && info.Mode().IsRegular()
	})
	if !ready {
		fmt.Fprintln(os.Stderr, "FAIL: daemon never became ready, so no commit could reload it")
		webRecoveryPrintLog(logPath)
		return fmt.Errorf("daemon never became ready, so no commit could reload it")
	}

	client, err := newWebRecoveryClient(ctx, baseURL)
	if err != nil {
		return err
	}
	if err := webRecoveryExercise(client, logPath); err != nil {
		fmt.Fprintln(os.Stderr, "--- daemon.log ---")
		webRecoveryPrintLog(logPath)
		return err
	}
	return nil
}

func newWebRecoveryClient(ctx context.Context, baseURL string) (*webRecoveryClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ZE_WEB_URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.Port() == "" {
		return nil, fmt.Errorf("invalid ZE_WEB_URL %q", baseURL)
	}
	return &webRecoveryClient{ctx: ctx, baseURL: baseURL}, nil
}

func webRecoveryExercise(client *webRecoveryClient, logPath string) error {
	token := client.waitForLogin("recovery-admin", "recoverypass")
	if err := webRecoveryRequire(token != nil,
		"the zefs break-glass admin logs in through the web login form"); err != nil {
		return err
	}

	response := client.setFormat(*token, "table")
	if err := webRecoveryRequire(webRecoveryStatusIs(response.status, http.StatusOK) && strings.Contains(response.body, `id="commit-bar"`),
		fmt.Sprintf("the session may edit before it commits (status %s)", webRecoveryStatusText(response.status))); err != nil {
		return err
	}

	response = client.commit(*token)
	if err := webRecoveryRequire(webRecoveryStatusIs(response.status, http.StatusOK) && !strings.Contains(response.body, "Commit failed"),
		fmt.Sprintf("the commit the operator asked for succeeds (status %s)", webRecoveryStatusText(response.status))); err != nil {
		return err
	}

	reloadStarted := Poll(client.ctx, 60, 500*time.Millisecond, func() bool {
		contents, err := os.ReadFile(logPath) //nolint:gosec // the path is the fixture's own scratch file
		return err == nil && strings.Contains(string(contents), "config reload started")
	})
	if err := webRecoveryRequire(reloadStarted,
		"the commit triggered the daemon reload the generation is decided by"); err != nil {
		return err
	}

	if err := webRecoveryRequire(strings.Contains(response.body, `id="commit-bar"`),
		"the commit answers with an editable commit bar, not a read-only one"); err != nil {
		return err
	}

	response = client.setFormat(*token, "json")
	if err := webRecoveryRequire(webRecoveryStatusIs(response.status, http.StatusOK) && strings.Contains(response.body, `id="commit-bar"`),
		fmt.Sprintf("the same session still edits after its own commit (status %s)", webRecoveryStatusText(response.status))); err != nil {
		return err
	}

	response = client.setFormat("not-a-session-token", "text")
	inventedTokenRefused := response.status != nil && *response.status != http.StatusOK
	if err := webRecoveryRequire(inventedTokenRefused,
		fmt.Sprintf("an invented session token is refused (status %s)", webRecoveryStatusText(response.status))); err != nil {
		return err
	}

	return nil
}

func (client *webRecoveryClient) waitForLogin(user, password string) *string {
	var held *string
	if !Poll(client.ctx, 60, 500*time.Millisecond, func() bool {
		held = client.loginCookie(user, password)
		return held != nil
	}) {
		return nil
	}
	return held
}

func (client *webRecoveryClient) loginCookie(user, password string) *string {
	requestContext, cancel := context.WithTimeout(client.ctx, 10*time.Second)
	defer cancel()

	body := webRecoveryEncodeForm([]webRecoveryField{
		{name: "username", value: user},
		{name: "password", value: password},
	})
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, client.baseURL+"/login", strings.NewReader(body))
	if err != nil {
		return nil
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient, transport := webRecoveryHTTPClient()
	defer transport.CloseIdleConnections()
	response, err := httpClient.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	if _, err := io.ReadAll(response.Body); err != nil {
		return nil
	}

	for _, value := range response.Header.Values("Set-Cookie") {
		if !strings.HasPrefix(value, "ze-session=") {
			continue
		}
		token := strings.TrimPrefix(value, "ze-session=")
		if before, _, found := strings.Cut(token, ";"); found {
			token = before
		}
		return &token
	}
	return nil
}

func (client *webRecoveryClient) setFormat(token, value string) webRecoveryResponse {
	return client.post(
		"/config/set/environment/cli/format/",
		[]webRecoveryField{{name: fieldLeaf, value: "default"}, {name: fieldValue, value: value}},
		map[string]string{"Cookie": "ze-session=" + token, "HX-Request": valueTrue},
	)
}

func (client *webRecoveryClient) commit(token string) webRecoveryResponse {
	return client.post(
		"/config/commit",
		nil,
		map[string]string{"Cookie": "ze-session=" + token, "HX-Request": valueTrue},
	)
}

func (client *webRecoveryClient) post(path string, fields []webRecoveryField, headers map[string]string) webRecoveryResponse {
	requestContext, cancel := context.WithTimeout(client.ctx, 30*time.Second)
	defer cancel()

	body := webRecoveryEncodeForm(fields)
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, client.baseURL+path, strings.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "transport: POST %s: %T: %v\n", path, err, err)
		return webRecoveryResponse{}
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", client.baseURL)
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	httpClient, transport := webRecoveryHTTPClient()
	defer transport.CloseIdleConnections()
	response, err := httpClient.Do(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transport: POST %s: %T: %v\n", path, err, err)
		return webRecoveryResponse{}
	}
	defer response.Body.Close() //nolint:errcheck // the body is read

	contents, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transport: POST %s: %T: %v\n", path, err, err)
		return webRecoveryResponse{}
	}
	status := response.StatusCode
	return webRecoveryResponse{status: &status, body: string(contents)}
}

func webRecoveryHTTPClient() (*http.Client, *http.Transport) {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // the product fixture uses its own generated test certificate
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client, transport
}

func webRecoveryEncodeForm(fields []webRecoveryField) string {
	var encoded textbuf.Buffer
	for index, field := range fields {
		if index != 0 {
			encoded.Byte('&')
		}
		encoded.Str(url.QueryEscape(field.name)).Byte('=').Str(url.QueryEscape(field.value))
	}
	return encoded.String()
}

func webRecoveryRequire(condition bool, message string) error {
	if !condition {
		fmt.Fprintln(os.Stderr, "FAIL: "+message)
		return fmt.Errorf("%s", message)
	}
	fmt.Fprintln(os.Stderr, "OK: "+message)
	return nil
}

func webRecoveryStatusIs(status *int, expected int) bool {
	return status != nil && *status == expected
}

func webRecoveryStatusText(status *int) string {
	if status == nil {
		return "None"
	}
	return strconv.Itoa(*status)
}

func webRecoveryEnvironment(base []string, assignments ...string) []string {
	replacements := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		key, _, _ := strings.Cut(assignment, "=")
		replacements[key] = assignment
	}

	environment := make([]string, 0, len(base)+len(assignments))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := replacements[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, assignments...)
	return environment
}

func webRecoveryPrintLog(path string) {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	_, _ = os.Stderr.Write(contents)
}
