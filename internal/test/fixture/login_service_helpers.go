// Design: docs/architecture/aaa-tacacs.md -- shared clients and daemon lifecycle for login-service functional fixtures.
// Related: login_service_doors.go -- per-surface login-service functional fixtures.
// Related: login_service_modes.go -- remote backend, reload, recovery, and mode fixtures.

package fixture

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
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

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// lsTestPasswordHash is the bcrypt cost-4 hash of lsTestPassword ("testpass"),
	// matching the published fixture credential across test/plugin/.
	lsTestPasswordHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" //nolint:gosec // published test fixture credential
	lsTestPassword     = "testpass"
	lsWrongPassword    = "wrongpass"
	lsRecoveryPassword = "recoverypass"

	lsLoopbackHost    = "127.0.0.1"
	lsCmdShowVersion  = "show version"
	lsCmdShowAudit    = "show audit"
	lsCmdShowConfig   = "show config running"
	lsWebProbePath    = "/show/environment/daemon/"
	lsWebLoginPath    = "/login"
	lsRESTExecutePath = "/api/v1/execute"
	lsMCPEndpointPath = "/mcp"
	lsMCPVersion      = "2026-07-28"
	lsHeaderAuth      = "Authorization"
	lsHeaderCT        = "Content-Type"
	lsHeaderWWWAuth   = "WWW-Authenticate"
	lsBearerPrefix    = "Bearer "
	lsCookiePrefix    = "ze-session="
	lsRefusedMetric   = "ze_aaa_login_service_refused_total"
	lsGateLogMsg      = "login refused by login-service gate"
	lsReloadComplete  = "sighup reload complete"
	lsDaemonLogName   = "daemon.log"
	lsPollDelay       = 150 * time.Millisecond
	lsPollAttempts    = 80
)

// lsWebResponse captures status, body, and issued ze-session cookie for one
// HTTPS request to Ze's web interface.
type lsWebResponse struct {
	status int
	body   string
	cookie string
}

// lsMCPResponse captures HTTP status, WWW-Authenticate header, and raw body
// from one MCP Streamable HTTP POST.
type lsMCPResponse struct {
	status  int
	wwwAuth string
	body    string
}

// lsManagedDaemon owns a child `ze start` process started by a fixture.
type lsManagedDaemon struct {
	cmd     *exec.Cmd
	done    chan error
	logFile *os.File
	logPath string
}

func lsRequirePort(raw string) (string, error) {
	port, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: %w", raw, err)
	}
	if port == 0 {
		return "", errors.New("port must be nonzero")
	}
	return raw, nil
}

func lsHostPort(port string) string {
	return net.JoinHostPort(lsLoopbackHost, port)
}

func lsWaitTCPReady(ctx context.Context, port, label string) error {
	address := lsHostPort(port)
	var lastErr error
	if !Poll(ctx, WaitAttempts(40, lsPollDelay, lsPollAttempts), lsPollDelay, func() bool {
		conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", address)
		lastErr = err
		if err != nil {
			return false
		}
		_ = conn.Close() //nolint:errcheck // readiness probe socket
		return true
	}) {
		return fmt.Errorf("%s did not listen on %s: %w", label, address, lastErr)
	}
	return nil
}

func lsDialSSHPassword(port, username, password string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         3 * time.Second,
	}
	return ssh.Dial("tcp", lsHostPort(port), cfg)
}

func lsRunSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close() //nolint:errcheck // fixture cleanup
	out, runErr := session.CombinedOutput(command)
	return string(out), runErr
}

func lsExecSSHPassword(port, username, password, command string) (string, error) {
	client, err := lsDialSSHPassword(port, username, password)
	if err != nil {
		return "", err
	}
	defer client.Close() //nolint:errcheck // fixture cleanup
	return lsRunSSHCommand(client, command)
}

func lsDialSSHPublicKey(port, username string, signer ssh.Signer) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // isolated loopback fixture
		Timeout:         3 * time.Second,
	}
	return ssh.Dial("tcp", lsHostPort(port), cfg)
}

func lsGenEd25519KeyPair() (signer ssh.Signer, pubKeyData string, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("generate ed25519 key: %w", err)
	}
	signer, err = ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, "", fmt.Errorf("build SSH signer: %w", err)
	}
	fields := strings.Fields(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if len(fields) < 2 {
		return nil, "", errors.New("authorized key marshaling produced fewer than 2 fields")
	}
	return signer, fields[1], nil
}

func lsNewHTTPSClient() (*http.Client, func()) {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // fixture daemon self-signed cert
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client, transport.CloseIdleConnections
}

func lsWebURL(port, path string) string {
	var tb textbuf.Buffer
	return tb.Str("https://").Str(lsLoopbackHost).Byte(':').Str(port).Str(path).String()
}

func lsWebFormLogin(ctx context.Context, client *http.Client, port, username, password string) (lsWebResponse, error) {
	var tb textbuf.Buffer
	form := tb.Str("username=").Str(url.QueryEscape(username)).
		Str("&password=").Str(url.QueryEscape(password)).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, lsWebURL(port, lsWebLoginPath), strings.NewReader(form))
	if err != nil {
		return lsWebResponse{}, err
	}
	req.Header.Set(lsHeaderCT, "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return lsWebResponse{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return lsWebResponse{}, err
	}
	var cookie string
	for _, value := range resp.Header.Values("Set-Cookie") {
		if !strings.HasPrefix(value, lsCookiePrefix) {
			continue
		}
		pair, _, _ := strings.Cut(value, ";")
		if _, tok, ok := strings.Cut(pair, "="); ok {
			cookie = tok
			break
		}
	}
	return lsWebResponse{status: resp.StatusCode, body: string(rawBody), cookie: cookie}, nil
}

func lsWebBasicGet(ctx context.Context, client *http.Client, port, username, password, path string) (lsWebResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lsWebURL(port, path), http.NoBody)
	if err != nil {
		return lsWebResponse{}, err
	}
	cred := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set(lsHeaderAuth, "Basic "+cred)
	resp, err := client.Do(req)
	if err != nil {
		return lsWebResponse{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return lsWebResponse{}, err
	}
	return lsWebResponse{status: resp.StatusCode, body: string(rawBody)}, nil
}

func lsWebCookieGet(ctx context.Context, client *http.Client, port, cookie, path string) (lsWebResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lsWebURL(port, path), http.NoBody)
	if err != nil {
		return lsWebResponse{}, err
	}
	req.Header.Set("Cookie", lsCookiePrefix+cookie)
	resp, err := client.Do(req)
	if err != nil {
		return lsWebResponse{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return lsWebResponse{}, err
	}
	return lsWebResponse{status: resp.StatusCode, body: string(rawBody)}, nil
}

func lsRESTURL(port, path string) string {
	var tb textbuf.Buffer
	return tb.Str("http://").Str(lsLoopbackHost).Byte(':').Str(port).Str(path).String()
}

func lsRESTExecute(ctx context.Context, port, bearerValue, command string) (int, string, error) {
	payload, err := json.Marshal(map[string]string{fieldCommand: command})
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, lsRESTURL(port, lsRESTExecutePath), bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set(lsHeaderCT, mediaTypeJSON)
	req.Header.Set(lsHeaderAuth, lsBearerPrefix+bearerValue)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", err
	}
	return resp.StatusCode, string(raw), nil
}

func lsNewGRPCClient(port string) (*grpc.ClientConn, error) {
	return grpc.NewClient(lsHostPort(port), grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func lsGRPCExecute(ctx context.Context, conn *grpc.ClientConn, bearerValue, command string) (*zepb.CommandResponse, codes.Code, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", lsBearerPrefix+bearerValue)
	client := zepb.NewZeServiceClient(conn)
	resp, err := client.Execute(reqCtx, &zepb.CommandRequest{Command: command})
	if err != nil {
		st := status.Convert(err)
		return nil, st.Code(), st.Message(), err
	}
	return resp, codes.OK, "", nil
}

func lsGRPCStreamOpen(ctx context.Context, conn *grpc.ClientConn, bearerValue, command string) (*zepb.CommandResponse, codes.Code, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", lsBearerPrefix+bearerValue)
	client := zepb.NewZeServiceClient(conn)
	stream, err := client.Stream(reqCtx, &zepb.CommandRequest{Command: command})
	if err != nil {
		st := status.Convert(err)
		return nil, st.Code(), st.Message(), err
	}
	msg, recvErr := stream.Recv()
	if recvErr != nil {
		st := status.Convert(recvErr)
		return nil, st.Code(), st.Message(), recvErr
	}
	return msg, codes.OK, "", nil
}

func lsMCPToolCall(ctx context.Context, port, token, command string) (lsMCPResponse, error) {
	bodyObj := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    lsMCPVersion,
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
				"io.modelcontextprotocol/clientInfo": map[string]any{
					"name":    "ze-login-service-fixture",
					"version": lsMCPVersion,
				},
			},
			"name": "ze_execute",
			"arguments": map[string]string{
				fieldCommand: command,
			},
		},
	}
	encoded, err := json.Marshal(bodyObj)
	if err != nil {
		return lsMCPResponse{}, err
	}
	var tb textbuf.Buffer
	endpoint := tb.Str("http://").Str(lsLoopbackHost).Byte(':').Str(port).Str(lsMCPEndpointPath).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return lsMCPResponse{}, err
	}
	req.Header.Set(lsHeaderCT, mediaTypeJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", lsMCPVersion)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "ze_execute")
	if token != "" {
		req.Header.Set(lsHeaderAuth, lsBearerPrefix+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lsMCPResponse{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return lsMCPResponse{}, err
	}
	return lsMCPResponse{
		status:  resp.StatusCode,
		wwwAuth: resp.Header.Get(lsHeaderWWWAuth),
		body:    string(raw),
	}, nil
}

func lsScrapePrometheus(ctx context.Context, port string) (string, error) {
	var tb textbuf.Buffer
	endpoint := tb.Str("http://").Str(lsLoopbackHost).Byte(':').Str(port).Str("/metrics").String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // body fully read below
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics endpoint returned HTTP %d", resp.StatusCode)
	}
	return string(raw), nil
}

// lsRefusedCounterValue returns the Prometheus value of
// ze_aaa_login_service_refused_total{service="<service>"} or -1 if absent.
func lsRefusedCounterValue(metricsText, service string) (float64, bool, error) {
	var tb textbuf.Buffer
	prefix := tb.Str(lsRefusedMetric).Str(`{service="`).Str(service).Str(`"} `).String()
	for line := range strings.SplitSeq(metricsText, "\n") {
		rest, ok := strings.CutPrefix(line, prefix)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return 0, false, fmt.Errorf("parse metric %q: %w", line, err)
		}
		return val, true, nil
	}
	return 0, false, nil
}

func lsRunCLI(ctx context.Context, configDir, port, username, password, command string) (string, error) {
	args := []string{areaCLI, "--remote", lsHostPort(port)}
	if username != "" {
		args = append(args, "--user", username)
	}
	args = append(args, "-c", command)
	cmd := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // fixture controls all arguments
	cmd.Env = childEnvironment(os.Environ(), map[string]string{
		envConfigDir:   configDir,
		envSSHPassword: password,
	})
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func lsInitStore(ctx context.Context, dir, username, password, sshPort string) error {
	var tb textbuf.Buffer
	stdin := tb.Str(username).Byte('\n').
		Str(password).Byte('\n').
		Str(lsLoopbackHost).Byte('\n').
		Str(sshPort).Byte('\n').String()
	cmd := exec.CommandContext(ctx, "ze", "init")
	cmd.Env = childEnvironment(os.Environ(), map[string]string{
		envConfigDir: dir,
	})
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ze init: %w\n%s", err, out)
	}
	return nil
}

func lsStartDaemon(ctx context.Context, workDir, configPath string, extraEnv map[string]string) (*lsManagedDaemon, error) {
	logPath := filepath.Join(workDir, lsDaemonLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // fixture scratch log
	if err != nil {
		return nil, err
	}
	overrides := map[string]string{
		"ze.log.aaa":      logLevelInfo,
		"ze.log.hub.ssh":  logLevelInfo,
		"ze.log.web.auth": logLevelInfo,
	}
	for k, v := range extraEnv {
		overrides[k] = v
	}
	cmd := exec.CommandContext(ctx, "ze", "start", configPath) //nolint:gosec // fixture controls path
	cmd.Dir = workDir
	cmd.Env = childEnvironment(os.Environ(), overrides)
	cmd.Stdout = io.Discard
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	daemon := &lsManagedDaemon{
		cmd:     cmd,
		done:    make(chan error, 1),
		logFile: logFile,
		logPath: logPath,
	}
	go daemon.waitProcess()
	return daemon, nil
}

func (d *lsManagedDaemon) waitProcess() {
	d.done <- d.cmd.Wait()
}

func (d *lsManagedDaemon) stop() {
	if d == nil {
		return
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = syscall.Kill(-d.cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-d.done:
			timer.Stop()
		case <-timer.C:
			_ = syscall.Kill(-d.cmd.Process.Pid, syscall.SIGKILL)
			<-d.done
		}
		d.cmd = nil
	}
	if d.logFile != nil {
		_ = d.logFile.Close()
		d.logFile = nil
	}
}

func (d *lsManagedDaemon) logText() string {
	if d == nil {
		return ""
	}
	if d.logFile != nil {
		_ = d.logFile.Sync()
	}
	raw, err := os.ReadFile(d.logPath) //nolint:gosec // fixture scratch log
	if err != nil {
		return "unreadable daemon log: " + err.Error()
	}
	return string(raw)
}

func (d *lsManagedDaemon) reloadAndWait(ctx context.Context, wantGeneration int) error {
	if d == nil {
		return errors.New("cannot reload nil daemon")
	}
	if d.cmd == nil {
		return errors.New("cannot reload stopped daemon")
	}
	if d.cmd.Process == nil {
		return errors.New("cannot reload unstarted daemon")
	}
	if err := d.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP: %w", err)
	}
	if !Poll(ctx, WaitAttempts(40, lsPollDelay, lsPollAttempts), lsPollDelay, func() bool {
		return strings.Count(d.logText(), lsReloadComplete) >= wantGeneration
	}) {
		return fmt.Errorf("daemon did not finish reload generation %d:\n%s", wantGeneration, d.logText())
	}
	return nil
}

func lsWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func lsAssertAuditContains(ctx context.Context, port, adminUser, adminPass, actor, surface string) error {
	out, err := lsExecSSHPassword(port, adminUser, adminPass, lsCmdShowAudit)
	if err != nil {
		return fmt.Errorf("run %q as %s: %w", lsCmdShowAudit, adminUser, err)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.Contains(line, actor) {
			continue
		}
		if !strings.Contains(line, "auth-fail") {
			continue
		}
		if surface != "" && !strings.Contains(line, surface) {
			continue
		}
		return nil
	}
	return fmt.Errorf("show audit missing auth-fail for actor=%q surface=%q:\n%s", actor, surface, out)
}

// lsWaitSSHAndRESTReady waits until both SSH and REST listeners accept TCP.
func lsWaitSSHAndRESTReady(ctx context.Context, sshPort, restPort string) error {
	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return err
	}
	return lsWaitTCPReady(ctx, restPort, "REST")
}
