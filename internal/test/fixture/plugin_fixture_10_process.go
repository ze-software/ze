package fixture

import (
	"bytes"
	"context"
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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	fixture10ProcessTimeout  = 15 * time.Second
	fixture10ShutdownTimeout = 5 * time.Second
)

type fixture10ProcessResult struct {
	stdout string
	stderr string
	code   int
}

func init() {
	Register("plugin/netlab-lab-profile", fixture10NetlabLabProfile)
	Register("plugin/pipe-review-remote-contracts", fixture10PipeReviewRemoteContracts)
	Register("plugin/plugin-command-document-too-wide", fixture10DocumentTooWide)
}

func fixture10Run(ctx context.Context, env map[string]string, stdin string, argv ...string) fixture10ProcessResult {
	commandCtx, cancel := context.WithTimeout(ctx, fixture10ProcessTimeout)
	defer cancel()
	command := exec.CommandContext(commandCtx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = fixture10Environment(env)
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if err != nil {
		code = 127
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		}
	}
	if commandCtx.Err() != nil {
		code = 124
		stderr.WriteString("process timed out after 15 seconds")
	}
	return fixture10ProcessResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func fixture10Environment(overrides map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for key, value := range overrides {
		prefix := key + "="
		for index := range slices.Backward(env) {
			if strings.HasPrefix(env[index], prefix) {
				env = append(env[:index], env[index+1:]...)
			}
		}
		env = append(env, prefix+value)
	}
	return env
}

func fixture10FreePort() (string, error) {
	// The listener closes on the next line, so there is nothing to cancel.
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return "", fmt.Errorf("a tcp listener answered %T, want *net.TCPAddr", listener.Addr())
	}
	port := strconv.Itoa(address.Port)
	if err := listener.Close(); err != nil {
		return "", err
	}
	return port, nil
}

func fixture10StartDaemon(ctx context.Context, configPath, logPath string, env map[string]string) (*exec.Cmd, *os.File, error) {
	logFile, err := os.Create(logPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, nil, err
	}
	command := exec.CommandContext(ctx, "ze", "start", configPath) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = fixture10Environment(env)
	command.Stdout = io.Discard
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		logFile.Close() //nolint:errcheck // the start failure below is the error worth reporting
		return nil, nil, err
	}
	return command, logFile, nil
}

func fixture10StopProcess(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(fixture10ShutdownTimeout):
		_ = command.Process.Kill()
		<-done
	}
}

func fixture10ReadLog(logFile *os.File, logPath string) string {
	if logFile != nil {
		_ = logFile.Sync()
	}
	data, _ := os.ReadFile(logPath) //nolint:gosec // the path is the fixture's own scratch file
	return string(data)
}

func fixture10PortFromLog(log string, expression *regexp.Regexp) int {
	match := expression.FindStringSubmatch(log)
	if len(match) != 2 {
		return 0
	}
	port, _ := strconv.Atoi(match[1])
	return port
}

func fixture10NetlabLabProfile(ctx context.Context, _ []string) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return fmt.Errorf("ZE_REPO_ROOT is empty")
	}
	golden := filepath.Join(root, "contrib", "netlab", "golden", "r3.conf")
	original, err := os.ReadFile(golden) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read committed netlab render: %w", err)
	}
	validated := fixture10Run(ctx, nil, "", "ze", "config", "validate", golden)
	if validated.code != 0 {
		return fmt.Errorf("committed netlab render is invalid: %s%s", validated.stdout, validated.stderr)
	}
	fmt.Fprintln(os.Stderr, "OK: AC-9 -- ze config validate accepts contrib/netlab/golden/r3.conf")

	config := strings.ReplaceAll(string(original), "10.1.0.5", "127.0.0.1")
	config = strings.ReplaceAll(config, "10.1.0.6", "127.0.0.1")
	config = strings.Replace(config, "port 2222;", "port 0;", 1)
	if strings.Contains(config, "10.1.0.") {
		return fmt.Errorf("link addresses were not rewritten; the render changed shape")
	}
	if !strings.Contains(config, "port 0;") {
		return fmt.Errorf("SSH port was not rewritten")
	}
	if err := os.WriteFile("lab.conf", []byte(config), 0o600); err != nil {
		return err
	}
	// The second ^ anchors the plaintext-password line inside the user block.
	credential := regexp.MustCompile(`(?ms)^\s*user\s+([A-Za-z0-9_-]+)\s*\{.*?^\s*plaintext-password\s+"([^"]*)";`).FindStringSubmatch(config) //nolint:gocritic // the second ^ is a line anchor, and (?m) is set
	if len(credential) != 3 {
		return fmt.Errorf("render declares no lab credential")
	}
	bgpPort, err := fixture10FreePort()
	if err != nil {
		return err
	}
	daemonCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	daemon, logFile, err := fixture10StartDaemon(daemonCtx, "lab.conf", "daemon.log", map[string]string{envTestBGPPort: bgpPort})
	if err != nil {
		return err
	}
	defer func() {
		fixture10StopProcess(daemon)
		_ = logFile.Close()
	}()
	sshRE := regexp.MustCompile(`SSH server listening.*?127\.0\.0\.1:(\d+)`)
	sshPort := 0
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		sshPort = fixture10PortFromLog(fixture10ReadLog(logFile, "daemon.log"), sshRE)
		return sshPort != 0
	}) {
		return fmt.Errorf("daemon did not start an SSH server: %s", fixture10ReadLog(logFile, "daemon.log"))
	}
	configDir, err := os.MkdirTemp("", "ze-netlab-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // scratch cleanup on exit, so a removal failure changes no assertion
	remoteEnv := map[string]string{
		envConfigDir:   configDir,
		envSSHHost:     addrLoopback,
		envSSHPort:     strconv.Itoa(sshPort),
		envSSHPassword: credential[2],
	}
	peers := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", credential[1], "-c", "show bgp peer list")
	if peers.code != 0 {
		return fmt.Errorf("render credential cannot log in: %s%s\n%s", peers.stdout, peers.stderr, fixture10ReadLog(logFile, "daemon.log"))
	}
	if !strings.Contains(fixture10ReadLog(logFile, "daemon.log"), "SSH auth success") || !strings.Contains(fixture10ReadLog(logFile, "daemon.log"), "username="+credential[1]) {
		return fmt.Errorf("daemon did not authenticate request as %s", credential[1])
	}
	fmt.Fprintln(os.Stderr, "OK: AC-10 -- the render's own credential logs in over SSH")
	compact := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", credential[1], "-c", "show bgp peer list | json compact")
	if compact.code != 0 {
		return fmt.Errorf("netlab show command failed: %s%s", compact.stdout, compact.stderr)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(compact.stdout), &document); err != nil {
		return fmt.Errorf("netlab show output is not JSON: %w: %s", err, compact.stdout)
	}
	peerRows, ok := document["peers"].([]any)
	if !ok {
		if peerMap, mapOK := document["peers"].(map[string]any); mapOK {
			ok = len(peerMap) != 0
		}
	}
	if !ok || (peerRows != nil && len(peerRows) == 0) {
		return fmt.Errorf("parsed JSON lists no peer: %v", document)
	}
	fmt.Fprintln(os.Stderr, "OK: AC-10 -- show bgp peer list | json compact parses as JSON with a peer")
	return nil
}

func fixture10RequireRefusal(result fixture10ProcessResult, operator, reason string) error {
	combined := strings.ToLower(result.stdout + result.stderr)
	if result.code == 0 {
		return fmt.Errorf("%s refusal exited zero: %s", operator, combined)
	}
	if !strings.Contains(combined, operator) || !strings.Contains(combined, reason) {
		return fmt.Errorf("%s refusal did not name operator and %q: %s", operator, reason, combined)
	}
	if result.stdout != "" {
		return fmt.Errorf("%s refusal produced an answer before failing: %q", operator, result.stdout)
	}
	return nil
}

type fixture10PTYSession struct {
	file   *os.File
	cmd    *exec.Cmd
	chunks chan []byte
	once   sync.Once
}

func fixture10StartPTY(ctx context.Context, sshPort int) (*fixture10PTYSession, error) {
	command := exec.CommandContext(ctx, "ssh", "-tt", "-p", strconv.Itoa(sshPort), //nolint:gosec // the fixture chooses the program and its arguments
		"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no",
		"-o", "NumberOfPasswordPrompts=1", "-o", "ConnectTimeout=5", "operator@127.0.0.1")
	command.Env = fixture10Environment(map[string]string{envTerm: "xterm-256color"})
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 100})
	if err != nil {
		return nil, err
	}
	session := &fixture10PTYSession{file: terminal, cmd: command, chunks: make(chan []byte, 16)}
	go func() {
		defer close(session.chunks)
		buffer := make([]byte, 65536)
		for {
			count, readErr := terminal.Read(buffer)
			if count != 0 {
				copyOfChunk := append([]byte(nil), buffer[:count]...)
				session.chunks <- copyOfChunk
			}
			if readErr != nil {
				return
			}
		}
	}()
	return session, nil
}

func (session *fixture10PTYSession) close() {
	session.once.Do(func() {
		_ = session.file.Close()
		fixture10StopProcess(session.cmd)
	})
}

func fixture10PTYCommand(ctx context.Context, sshPort int, command, marker string) (string, error) {
	session, err := fixture10StartPTY(ctx, sshPort)
	if err != nil {
		return "", err
	}
	defer session.close()
	transcript := ""
	readUntil := func(predicate func(string) bool, description string) error {
		timer := time.NewTimer(fixture10ProcessTimeout)
		defer timer.Stop()
		for !predicate(transcript) {
			select {
			case chunk, open := <-session.chunks:
				if !open {
					return fmt.Errorf("%s; transcript=%q", description, transcript)
				}
				transcript += string(chunk)
			case <-timer.C:
				return fmt.Errorf("%s; transcript=%q", description, transcript)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	if err := readUntil(func(text string) bool { return strings.Contains(strings.ToLower(text), "password:") }, "OpenSSH did not request password"); err != nil {
		return "", err
	}
	_, _ = session.file.WriteString("testpass\r")
	if err := readUntil(func(text string) bool { return strings.Contains(strings.ToLower(text), "welcome to ze") }, "authenticated PTY did not render hub model"); err != nil {
		return "", err
	}
	switchAt := len(transcript)
	_, _ = session.file.WriteString("exit\r")
	if err := readUntil(func(text string) bool { return strings.Contains(text[switchAt:], "ze> ") }, "hub model did not enter operational mode"); err != nil {
		return "", err
	}
	commandAt := len(transcript)
	_, _ = session.file.WriteString(command + "\r")
	if err := readUntil(func(text string) bool {
		return strings.Contains(strings.ToLower(text[commandAt:]), strings.ToLower(marker))
	}, "PTY command did not render marker "+marker); err != nil {
		return "", err
	}
	return strings.ToLower(transcript[commandAt:]), nil
}

func fixture10PipeReviewRemoteContracts(ctx context.Context, _ []string) error {
	work, err := os.MkdirTemp("", "ze-pipe-review-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck // best-effort fixture cleanup
	stateDir := filepath.Join(work, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		return err
	}
	configPath := filepath.Join(work, "pipe-entry.conf")
	config := `system {
    authentication {
        user operator {
            password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
            profile [ pipe-entry ]
        }
    }
    authorization {
        profile pipe-entry {
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
    ssh {
        enabled true
        server main {
            ip 127.0.0.1;
            port 0;
        }
    }
    web {
        enabled true
        server main {
            ip 127.0.0.1;
            port 0;
        }
    }
}
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return err
	}
	daemonCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	logPath := filepath.Join(work, "daemon.log")
	daemon, logFile, err := fixture10StartDaemon(daemonCtx, configPath, logPath, map[string]string{envConfigDir: stateDir})
	if err != nil {
		return err
	}
	defer func() {
		fixture10StopProcess(daemon)
		_ = logFile.Close()
	}()
	sshRE := regexp.MustCompile(`SSH server listening.*?address=127\.0\.0\.1:(\d+)`)
	webRE := regexp.MustCompile(`web server listening on https://127\.0\.0\.1:(\d+)/`)
	sshPort, webPort := 0, 0
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		log := fixture10ReadLog(logFile, logPath)
		sshPort = fixture10PortFromLog(log, sshRE)
		webPort = fixture10PortFromLog(log, webRE)
		return sshPort != 0 && webPort != 0
	}) {
		return fmt.Errorf("SSH/web servers did not report ports: %s", fixture10ReadLog(logFile, logPath))
	}
	if sshPort == webPort {
		return fmt.Errorf("SSH and web listeners reused port %d", sshPort)
	}
	remoteEnv := map[string]string{
		envConfigDir: stateDir, envSSHHost: addrLoopback,
		envSSHPort: strconv.Itoa(sshPort), envSSHPassword: valueTestPassword,
	}
	oversized := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "operator", "-c", "show bgp rib | last 257")
	if err := fixture10RequireRefusal(oversized, "last", "at most 256"); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(oversized.stderr), "257") {
		return fmt.Errorf("last refusal did not name supplied 257: %s", oversized.stderr)
	}
	surplus := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "operator", "-c", "show bgp rib | count extra")
	if err := fixture10RequireRefusal(surplus, "count", "does not accept an argument"); err != nil {
		return err
	}
	oneShot := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "operator", "-c", "show version | log")
	if err := fixture10RequireRefusal(oneShot, "log", "streaming command"); err != nil {
		return err
	}
	sshSave := filepath.Join(work, "ssh-must-not-exist.json")
	sshRefusal := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "operator", "-c", "show version | save "+sshSave)
	if err := fixture10RequireRefusal(sshRefusal, "save", "daemon"); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(sshRefusal.stderr), "refused") {
		return fmt.Errorf("SSH save refusal did not state refusal: %s", sshRefusal.stderr)
	}
	if _, err := os.Stat(sshSave); !os.IsNotExist(err) {
		return fmt.Errorf("SSH refused save still created %s", sshSave)
	}
	for _, control := range [][2]string{
		{"monitor ping 127.0.0.1 | log", "--- monitor ping 127.0.0.1 | log (Esc to stop) ---"},
		{"monitor traceroute 127.0.0.1 | log", "--- monitor traceroute | log (Esc to stop) ---"},
	} {
		if _, err := fixture10PTYCommand(ctx, sshPort, control[0], control[1]); err != nil {
			return err
		}
	}
	ptyCases := []struct {
		command, path, forbidden string
	}{
		{"show version | save " + filepath.Join(work, "pty-must-not-exist.json"), filepath.Join(work, "pty-must-not-exist.json"), ""},
		{"monitor ping 127.0.0.1 | save " + filepath.Join(work, "pty-ping-must-not-exist.json"), filepath.Join(work, "pty-ping-must-not-exist.json"), "monitoring ping"},
		{"monitor traceroute 127.0.0.1 | save " + filepath.Join(work, "pty-traceroute-must-not-exist.json"), filepath.Join(work, "pty-traceroute-must-not-exist.json"), "monitoring traceroute"},
	}
	for _, test := range ptyCases {
		answer, err := fixture10PTYCommand(ctx, sshPort, test.command, "refused")
		if err != nil {
			return err
		}
		if !strings.Contains(answer, "save") || !strings.Contains(answer, "daemon") || (test.forbidden != "" && strings.Contains(answer, test.forbidden)) {
			return fmt.Errorf("PTY refusal contract failed: %q", answer)
		}
		if _, err := os.Stat(test.path); !os.IsNotExist(err) {
			return fmt.Errorf("PTY refused save still created %s", test.path)
		}
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // the fixture dials the daemon's own self-signed test certificate
	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
	baseURL := "https://127.0.0.1:" + strconv.Itoa(webPort)
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("operator:testpass"))
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/ping", http.NoBody)
		request.Header.Set("Authorization", auth)
		response, requestErr := client.Do(request)
		if requestErr != nil {
			return false
		}
		defer response.Body.Close() //nolint:errcheck // the fixture only reads the body, so a close failure changes no assertion
		return response.StatusCode == http.StatusOK
	}) {
		return fmt.Errorf("web server did not become ready: %s", fixture10ReadLog(logFile, logPath))
	}
	webSave := filepath.Join(work, "web-must-not-exist.json")
	form := url.Values{fieldCommand: {"show version | save " + webSave}, fieldMode: {"operational"}}.Encode()
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/cli/terminal", strings.NewReader(form))
	request.Header.Set("Authorization", auth)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // the fixture only reads the body, so a close failure changes no assertion
	var webResponse map[string]any
	if err := json.NewDecoder(response.Body).Decode(&webResponse); err != nil {
		return err
	}
	output, _ := webResponse["output"].(string)
	lowerOutput := strings.ToLower(output)
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(output, "pipe error:") || !strings.Contains(lowerOutput, "save") || !strings.Contains(lowerOutput, "refused") || !strings.Contains(lowerOutput, "daemon") {
		return fmt.Errorf("web save refusal contract failed: status=%d response=%v", response.StatusCode, webResponse)
	}
	if _, err := os.Stat(webSave); !os.IsNotExist(err) {
		return fmt.Errorf("web refused save still created %s", webSave)
	}
	fmt.Fprintln(os.Stdout, "OK: reviewed remote pipe contracts") //nolint:errcheck // progress output
	return nil
}

func fixture10DocumentTooWide(ctx context.Context, _ []string) error {
	work, err := os.MkdirTemp("", "ze-document-too-wide-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	config := `plugin {
	external record-plugin {
		run "ze-test record-plugin"
		encoder json
	}
}
bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.2
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65533
				remote 65533
			}
		}
	}
}
system {
	authentication {
		user admin {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
		}
	}
	authorization {
		profile admin {
			run {
				default-action allow
			}
			edit {
				default-action allow
			}
		}
	}
}
environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 0;
		}
	}
}
`
	configPath := filepath.Join(work, "plugin-command-document-too-wide.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return err
	}
	bgpPort, err := fixture10FreePort()
	if err != nil {
		return err
	}
	daemonCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	logPath := filepath.Join(work, "daemon.log")
	daemon, logFile, err := fixture10StartDaemon(daemonCtx, configPath, logPath, map[string]string{
		envTestBGPPort: bgpPort,
		envConfigDir:   work,
	})
	if err != nil {
		return err
	}
	defer func() {
		fixture10StopProcess(daemon)
		_ = logFile.Close()
	}()
	sshRE := regexp.MustCompile(`127\.0\.0\.1:(\d+)`)
	sshPort := 0
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		sshPort = fixture10PortFromLog(fixture10ReadLog(logFile, logPath), sshRE)
		return sshPort != 0
	}) {
		return fmt.Errorf("SSH server did not start: %s", fixture10ReadLog(logFile, logPath))
	}
	configDir, err := os.MkdirTemp("", "ze-document-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir) //nolint:errcheck // scratch cleanup on exit, so a removal failure changes no assertion
	initInput := fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%d\n", sshPort)
	initResult := fixture10Run(ctx, map[string]string{envConfigDir: configDir}, initInput, "ze", "init")
	if initResult.code != 0 {
		return fmt.Errorf("initialize isolated CLI state: exit=%d stdout=%s stderr=%s", initResult.code, initResult.stdout, initResult.stderr)
	}
	remoteEnv := map[string]string{
		envConfigDir: configDir, envSSHHost: addrLoopback,
		envSSHPort: strconv.Itoa(sshPort), envSSHPassword: valueTestPassword,
	}
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		result := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "admin", "-c", "system command list | raw")
		return result.code == 0 && strings.Contains(result.stdout, "show test records document")
	}) {
		return fmt.Errorf("record plugin command never appeared: %s", fixture10ReadLog(logFile, logPath))
	}
	result := fixture10Run(ctx, remoteEnv, "", "ze", "cli", "--user", "admin", "-c", "show test records document | raw")
	if result.code == 0 {
		return fmt.Errorf("an answer that delivered no row reported success")
	}
	if strings.Contains(result.stderr, "exceeds maximum size") {
		return fmt.Errorf("document line was written unmeasured and transport refused it: %s", result.stderr)
	}
	if result.stdout == "" {
		return fmt.Errorf("no payload reached operator: %s", result.stderr)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &document); err != nil {
		return fmt.Errorf("decode rejected answer: %w", err)
	}
	if len(document) != 2 || document["data"] == nil || document["errors"] == nil {
		return fmt.Errorf("answer carries wrong keys: %v", document)
	}
	data, ok := document["data"].([]any)
	if !ok || len(data) != 0 {
		return fmt.Errorf("answer data=%v, want empty list", document["data"])
	}
	errorsList, ok := document["errors"].([]any)
	if !ok || len(errorsList) != 1 {
		return fmt.Errorf("answer errors=%v, want one", document["errors"])
	}
	fault, ok := errorsList[0].(map[string]any)
	if !ok || len(fault) != 4 {
		return fmt.Errorf("rejected row has wrong shape: %v", errorsList[0])
	}
	for _, key := range []string{"encoded-bytes", "limit-bytes", fieldMessage, "record"} {
		if _, found := fault[key]; !found {
			return fmt.Errorf("rejected row missing %s: %v", key, fault)
		}
	}
	if fault["message"] != "answer record does not fit one wire message" || fixture10Number(fault["record"]) != 1 || fixture10Number(fault["limit-bytes"]) != 16777216 {
		return fmt.Errorf("rejected document fault mismatch: %v", fault)
	}
	minimumWidth := 2 * (16777216 * 5 / 8)
	encoded := fixture10Number(fault["encoded-bytes"])
	if encoded < minimumWidth {
		return fmt.Errorf("rejected line is %d bytes, one row wide rather than whole document", encoded)
	}
	fmt.Fprintf(os.Stderr, "OK: the document of %d bytes was refused and the answer still ended\n", encoded)
	return nil
}
