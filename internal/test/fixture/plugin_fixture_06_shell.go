package fixture

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

const fixture06AnswerFrameEnv = sshclient.EnvAnswerFrame + "=" + sshclient.AnswerFrameEnabled

type fixture06ExecAnswerCase struct {
	name           string
	command        string
	wantedExit     int
	plainErrSubstr string
	unorderedRows  bool
	wantJSON       bool
}

func init() {
	Register("plugin/exec-answer-unconditional", fixture06ExecAnswerDriver)
}

func fixture06ExecAnswerDriver(ctx context.Context, _ []string) error {
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return fmt.Errorf("signal fixture readiness: %w", err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate SSH key: %w", err)
	}
	privateBlock, err := ssh.MarshalPrivateKey(privateKey, "ze functional fixture")
	if err != nil {
		return fmt.Errorf("marshal SSH private key: %w", err)
	}
	if err := os.WriteFile("testkey", pem.EncodeToMemory(privateBlock), 0o600); err != nil {
		return fmt.Errorf("write SSH private key: %w", err)
	}
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("marshal SSH public key: %w", err)
	}
	publicFields := strings.Fields(string(ssh.MarshalAuthorizedKey(sshPublicKey)))
	if len(publicFields) != 2 {
		return fmt.Errorf("authorized key has %d fields, want 2", len(publicFields))
	}
	if err := os.WriteFile("testkey.pub", ssh.MarshalAuthorizedKey(sshPublicKey), 0o600); err != nil {
		return fmt.Errorf("write SSH public key: %w", err)
	}

	config := fmt.Sprintf(`bgp {
	router-id 203.0.113.253;
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
		user keyuser {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
			profile [ admin ]
			public-keys testkey {
				type ssh-ed25519
				key %s
			}
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
`, publicFields[1])
	if err := os.WriteFile("exec-answer-unconditional.conf", []byte(config), 0o600); err != nil {
		return fmt.Errorf("write daemon config: %w", err)
	}

	logFile, err := os.OpenFile("daemon.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create daemon log: %w", err)
	}
	defer logFile.Close() //nolint:errcheck // fixture teardown

	daemon := exec.CommandContext(ctx, "ze", "start", "exec-answer-unconditional.conf")
	daemon.Stdout = os.Stdout
	daemon.Stderr = logFile
	daemon.Env = append(os.Environ(), "ze_test_bgp_port="+strconv.Itoa(10000+os.Getpid()%50000))
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	var daemonErr error
	daemonDone := make(chan struct{})
	go func() {
		daemonErr = daemon.Wait()
		close(daemonDone)
	}()
	defer fixture06StopDaemon(daemon, daemonDone)

	addressPattern := regexp.MustCompile(`127\.0\.0\.1:(\d+)`)
	var sshPort string
	var daemonExited bool
	// The wait ends on the logged SSH address or on the daemon's exit. Its
	// 30-second bound is not a boot-time guess: a loaded machine has taken
	// more than ten seconds to start the daemon, and the runner's 45-second
	// timeout cancels ctx first.
	if !Poll(ctx, 150, 200*time.Millisecond, func() bool {
		select {
		case <-daemonDone:
			daemonExited = true
			return true
		default:
		}
		logData, readErr := os.ReadFile("daemon.log")
		if readErr != nil {
			return false
		}
		match := addressPattern.FindSubmatch(logData)
		if len(match) == 2 {
			sshPort = string(match[1])
			return true
		}
		return false
	}) || sshPort == "" {
		logData, _ := os.ReadFile("daemon.log")
		if daemonExited {
			if daemonErr != nil {
				return fmt.Errorf("daemon exited before SSH server started: %w\n%s", daemonErr, logData)
			}
			return fmt.Errorf("daemon exited before SSH server started: <nil>\n%s", logData)
		}
		return fmt.Errorf("SSH server did not start (no address in daemon.log)\n%s", logData)
	}
	fmt.Fprintf(os.Stderr, "SSH port: %s\n", sshPort)

	if err := fixture06Wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}

	cases := []fixture06ExecAnswerCase{
		{name: "document", command: cmdShowVersion, wantedExit: 0},
		{name: "json", command: cmdShowVersion + " | json", wantedExit: 0, wantJSON: true},
		{name: "streamed", command: "system command list | ndjson", wantedExit: 0, unorderedRows: true},
		{name: valueUnknown, command: "shwo bgp peers", wantedExit: 1, plainErrSubstr: "unknown command"},
		{name: outcomeFailed, command: "show pki certificate name Жé", wantedExit: 1, plainErrSubstr: "pki: certificate Жé not found"},
	}
	for _, tc := range cases {
		plainBody, plainStderr, _, _, err := fixture06RunSSHExec(ctx, sshPort, tc.name+"-plain", tc.command, tc.wantedExit, false)
		if err != nil {
			return err
		}
		if err := fixture06CheckPlainResult(tc, plainBody, plainStderr); err != nil {
			return err
		}
		framedBody, framedStderr, bodyPath, framePath, err := fixture06RunSSHExec(ctx, sshPort, tc.name+"-framed", tc.command, tc.wantedExit, true)
		if err != nil {
			return err
		}
		if err := fixture06CheckBodyMatch(tc, plainBody, framedBody); err != nil {
			return err
		}
		if err := fixture06CheckFramedResult(ctx, tc, bodyPath, framePath, framedStderr); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "OK: plain -- plain SSH received clean output with zero answer frame lines")
	fmt.Fprintln(os.Stderr, "OK: opt-in -- a client sending ZE_ANSWER_FRAME=1 read every frame by arithmetic")
	return nil
}

// fixture06RunSSHExec runs one command through real OpenSSH against the
// fixture daemon, either plain (no ZE_ANSWER_FRAME) or opting into answer
// framing (-o SendEnv=ZE_ANSWER_FRAME with ZE_ANSWER_FRAME=1).
func fixture06RunSSHExec(ctx context.Context, sshPort, prefix, command string, wantedExit int, framed bool) (body, stderr []byte, bodyPath, framePath string, err error) {
	bodyPath = prefix + ".body"
	framePath = prefix + ".frame"
	bodyFile, err := os.OpenFile(bodyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, nil, "", "", err
	}
	frameFile, err := os.OpenFile(framePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		bodyFile.Close() //nolint:errcheck // fixture teardown
		return nil, nil, "", "", err
	}
	args := []string{
		"-F", "/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "PasswordAuthentication=no",
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-i", "./testkey",
		"-p", sshPort,
	}
	clientEnv := fixture06WithoutEnv(os.Environ(), "SSH_AUTH_SOCK")
	clientEnv = fixture06WithoutEnv(clientEnv, sshclient.EnvAnswerFrame)
	if framed {
		args = append(args, "-o", "SendEnv="+sshclient.EnvAnswerFrame)
		clientEnv = append(clientEnv, fixture06AnswerFrameEnv)
	}
	args = append(args, "keyuser@127.0.0.1", command)

	client := exec.CommandContext(ctx, "ssh", args...) //nolint:gosec // the fixture chooses the program and its arguments
	client.Env = clientEnv
	client.Stdout = bodyFile
	client.Stderr = frameFile
	runErr := client.Run()
	bodyCloseErr := bodyFile.Close()
	frameCloseErr := frameFile.Close()
	if bodyCloseErr != nil {
		return nil, nil, "", "", errors.Join(bodyCloseErr, frameCloseErr)
	}
	if frameCloseErr != nil {
		return nil, nil, "", "", frameCloseErr
	}
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return nil, nil, "", "", fmt.Errorf("run SSH command %q (framed=%t): %w", command, framed, runErr)
		}
		exitCode = exitErr.ExitCode()
	}
	body, err = os.ReadFile(bodyPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, nil, "", "", err
	}
	stderr, err = os.ReadFile(framePath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, nil, "", "", err
	}
	if exitCode != wantedExit {
		return nil, nil, "", "", fmt.Errorf("%s (framed=%t) exited %d, want %d\n%s", command, framed, exitCode, wantedExit, stderr)
	}
	return body, stderr, bodyPath, framePath, nil
}

// fixture06CheckPlainResult verifies that a plain OpenSSH session (without
// ZE_ANSWER_FRAME) receives clean rendered stdout and zero answer frame lines
// on stderr.
func fixture06CheckPlainResult(tc fixture06ExecAnswerCase, body, stderr []byte) error {
	if tc.wantedExit != 0 {
		if len(body) != 0 {
			return fmt.Errorf("plain %s: expected empty stdout on failure, got %q", tc.name, body)
		}
		lines, plainLines, err := fixture06ReadFrame(stderr)
		if err != nil {
			return fmt.Errorf("plain %s: scan stderr: %w; stderr=%q", tc.name, err, stderr)
		}
		if len(lines) != 0 {
			return fmt.Errorf("plain %s: unexpected answer frame lines on stderr: %+v (stderr=%q)", tc.name, lines, stderr)
		}
		for _, line := range plainLines {
			if strings.HasPrefix(line, "wrn ") {
				return fmt.Errorf("plain %s: unexpected warning frame line on stderr: %q", tc.name, line)
			}
		}
		if !strings.Contains(string(stderr), tc.plainErrSubstr) {
			return fmt.Errorf("plain %s: stderr %q missing expected operator error %q", tc.name, stderr, tc.plainErrSubstr)
		}
		return nil
	}
	if len(stderr) != 0 {
		return fmt.Errorf("plain %s: expected empty stderr without %s, got %q", tc.name, sshclient.EnvAnswerFrame, stderr)
	}
	if len(body) == 0 {
		return fmt.Errorf("plain %s: expected non-empty stdout", tc.name)
	}
	if tc.wantJSON {
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			return fmt.Errorf("plain %s: stdout is not valid JSON: %w; stdout=%q", tc.name, err, body)
		}
		ver, ok := decoded["version"].(string)
		if !ok {
			return fmt.Errorf("plain %s: decoded JSON missing string version key: %+v", tc.name, decoded)
		}
		if ver == "" {
			return fmt.Errorf("plain %s: decoded JSON version is empty: %+v", tc.name, decoded)
		}
	}
	if tc.unorderedRows {
		rows := fixture06SortedNonEmptyLines(body)
		if len(rows) <= 256 {
			return fmt.Errorf("plain %s: expected >256 streamed rows, got %d", tc.name, len(rows))
		}
		for _, row := range rows {
			if !json.Valid([]byte(row)) {
				return fmt.Errorf("plain %s: streamed row is not valid JSON: %q", tc.name, row)
			}
		}
	}
	return nil
}

// fixture06CheckBodyMatch verifies that plain and opt-in framed SSH sessions
// produce identical stdout answers.
func fixture06CheckBodyMatch(tc fixture06ExecAnswerCase, plainBody, framedBody []byte) error {
	if tc.unorderedRows {
		plainRows := fixture06SortedNonEmptyLines(plainBody)
		framedRows := fixture06SortedNonEmptyLines(framedBody)
		if !slices.Equal(plainRows, framedRows) {
			return fmt.Errorf("%s: plain stdout rows (%d) differ from framed stdout rows (%d)", tc.name, len(plainRows), len(framedRows))
		}
		return nil
	}
	if !bytes.Equal(plainBody, framedBody) {
		return fmt.Errorf("%s: plain stdout %q differs from framed stdout %q", tc.name, plainBody, framedBody)
	}
	return nil
}

// fixture06CheckFramedResult verifies that an opt-in OpenSSH session
// (ZE_ANSWER_FRAME=1) receives the exact grammar-valid answer frame on stderr.
func fixture06CheckFramedResult(ctx context.Context, tc fixture06ExecAnswerCase, bodyPath, framePath string, framedStderr []byte) error {
	if len(framedStderr) == 0 {
		return fmt.Errorf("%s: received empty stderr despite %s", tc.command, fixture06AnswerFrameEnv)
	}
	if tc.wantJSON {
		lines, _, err := fixture06ReadFrame(framedStderr)
		if err != nil {
			return fmt.Errorf("framed %s: %w; frame=%q", tc.name, err, framedStderr)
		}
		if len(lines) != 2 {
			return fmt.Errorf("framed %s: want 2 frame lines (top,end), got %d (%+v)", tc.name, len(lines), lines)
		}
		head, tail := lines[0], lines[1]
		if head.kind != decoderKindTop {
			return fmt.Errorf("framed %s: first line kind=%q, want %q", tc.name, head.kind, decoderKindTop)
		}
		if head.item != shapeDoc {
			return fmt.Errorf("framed %s: head item=%q, want %q", tc.name, head.item, shapeDoc)
		}
		if head.keyCount != 0 {
			return fmt.Errorf("framed %s: head keyCount=%d, want 0", tc.name, head.keyCount)
		}
		if head.columnsCount != 0 {
			return fmt.Errorf("framed %s: head columnsCount=%d, want 0", tc.name, head.columnsCount)
		}
		if tail.kind != decoderKindEnd {
			return fmt.Errorf("framed %s: second line kind=%q, want %q", tc.name, tail.kind, decoderKindEnd)
		}
		if tail.count != 1 {
			return fmt.Errorf("framed %s: tail count=%d, want 1", tc.name, tail.count)
		}
		if tail.faults != 0 {
			return fmt.Errorf("framed %s: tail faults=%d, want 0", tc.name, tail.faults)
		}
		if tail.messageCount != 0 {
			return fmt.Errorf("framed %s: tail messageCount=%d, want 0", tc.name, tail.messageCount)
		}
		fmt.Fprintln(os.Stderr, "OK: json -- plain SSH emitted clean JSON and opt-in framed `top doc 0: 0:` and `end 1 0 0:`")
		return nil
	}
	return fixture06FrameCheck(ctx, []string{tc.name, framePath, bodyPath})
}

func fixture06SortedNonEmptyLines(data []byte) []string {
	var rows []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line != "" {
			rows = append(rows, line)
		}
	}
	slices.Sort(rows)
	return rows
}

func fixture06WithoutEnv(environment []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func fixture06StopDaemon(command *exec.Cmd, done <-chan struct{}) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_ = command.Process.Kill()
		<-done
	}
}
