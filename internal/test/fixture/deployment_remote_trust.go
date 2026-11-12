// Design: docs/guide/operations.md -- standalone CLI with verified remote trust.
// Related: docs/architecture/config/environment.md -- caller-side trust and aliases.

package fixture

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

func init() {
	Register("ui/deployment-remote-trust", deploymentRemoteTrust)
}

func deploymentRemoteTrust(ctx context.Context, args []string) (resultErr error) {
	if len(args) != 1 {
		return errors.New("remote trust fixture requires the runner's SSH port")
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil || port == 0 {
		return errors.New("remote trust fixture requires a nonzero runner SSH port from 1 through 65535")
	}
	ctx, cancel := context.WithTimeout(ctx, deploymentBudget(90*time.Second))
	defer cancel()

	root, err := os.MkdirTemp(".", "rt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // Remove only this fixture's private runner-owned directory.
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	serverDir, clientDir, downloadDir := filepath.Join(root, "s"), filepath.Join(root, "c"), filepath.Join(root, "d")
	for _, dir := range []string{serverDir, clientDir, downloadDir, filepath.Join(serverDir, "r"), filepath.Join(clientDir, "r"), filepath.Join(clientDir, "home"), filepath.Join(clientDir, "home", ".ssh"), filepath.Join(clientDir, "trusted hosts")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
	}
	source, err := uiZEBinary()
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	// PATH contains runner shims. Link the resolved regular executable, never
	// a symlink whose eventual config lookup would return to the original bin.
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() {
		return errors.New("runner DUT is not a regular executable")
	}
	binary := filepath.Join(downloadDir, "ze")
	// A hardlink changes neither the source inode nor its contents and avoids
	// copying the executable when DUT and worker scratch share a filesystem.
	// Its parent is neither bin nor sbin: this CLI has no inferred store.
	linkErr := os.Link(source, binary)
	if errors.Is(linkErr, syscall.EXDEV) {
		if err := remoteTrustCopyBinary(ctx, source, binary, sourceInfo); err != nil {
			return fmt.Errorf("copy runner DUT across filesystems: %w", err)
		}
	} else if linkErr != nil {
		return fmt.Errorf("hardlink runner DUT into private standalone directory: %w", linkErr)
	}
	linkedInfo, err := os.Lstat(binary)
	if err != nil {
		return err
	}
	if !linkedInfo.Mode().IsRegular() || linkedInfo.Size() != sourceInfo.Size() {
		return errors.New("standalone DUT is not a regular executable of the expected size")
	}
	if linkErr == nil && !os.SameFile(sourceInfo, linkedInfo) {
		return errors.New("standalone DUT hardlink does not identify the resolved runner executable")
	}

	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	password := hex.EncodeToString(secret)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			text := strings.ReplaceAll(resultErr.Error(), password, "<redacted-password>")
			resultErr = errors.New(strings.ReplaceAll(text, string(hash), "<redacted-password-hash>"))
		}
	}()
	address := net.JoinHostPort("127.0.0.2", strconv.FormatUint(port, 10))
	configuration := fmt.Sprintf(`environment {
    ssh {
        enabled true;
        server proof { ip 127.0.0.2; port %d; }
    }
}
system {
    dns { resolv-conf-path %s; }
    authentication {
        user remote-proof {
            password "%s";
            profile [ admin ];
        }
    }
    authorization {
        profile admin { run { default-action allow; } edit { default-action allow; } }
    }
}
`, port, config.QuoteIfNeeded(filepath.Join(serverDir, "resolv.conf")), hash)
	configPath, readyPath := filepath.Join(serverDir, "ze.conf"), filepath.Join(serverDir, "ready")
	if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
		return err
	}
	serverEnv := remoteTrustEnvironment(os.Environ(), map[string]string{envConfigDir: serverDir, envReadyFile: readyPath, "HOME": serverDir, envXDGRuntimeDir: filepath.Join(serverDir, "r")})
	var daemonOutput lockedBuffer
	daemon, err := remoteTrustStartDaemon(ctx, source, serverDir, configPath, serverEnv, &daemonOutput)
	if err != nil {
		return err
	}
	// Stop joins the runner's identity-preserving sole reaper before any
	// scratch removal or result publication. Cleanup failure fails the proof.
	defer func() {
		if err := daemon.Stop(syscall.SIGTERM, deploymentBudget(fixture10ShutdownTimeout), 0); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("stop private remote daemon: %w\n%s", err, daemonOutput.String()))
		}
		if resultErr == nil {
			fmt.Println("verified standalone remote trust and password rejection")
		}
	}()
	var readinessErr error
	if !Poll(ctx, 900*runner.ChildParallelFactor(), 100*time.Millisecond, func() bool {
		if readinessErr = remoteTrustDaemonAlive(daemon, &daemonOutput); readinessErr != nil {
			return true
		}
		_, statErr := os.Stat(readyPath)
		return statErr == nil
	}) {
		cause := context.Cause(ctx)
		if cause == nil {
			cause = errors.New("readiness poll exhausted its lifecycle budget")
		}
		return fmt.Errorf("remote daemon did not become ready: %w\n%s", cause, daemonOutput.String())
	}
	if readinessErr != nil {
		return readinessErr
	}

	// This is private test bootstrap of the listener this fixture just started,
	// after its readiness/liveness barrier. Abort before authentication after
	// capturing its public key. This callback never reaches a product client;
	// every downloaded-style CLI call below uses normal strict verification.
	key, err := remoteTrustOwnedHostKey(ctx, address)
	if err != nil {
		return err
	}
	if err := remoteTrustDaemonAlive(daemon, &daemonOutput); err != nil {
		return err
	}
	trustPath := filepath.Join(clientDir, "trusted hosts", "known_hosts")
	defaultTrustPath := filepath.Join(clientDir, "home", ".ssh", "known_hosts")
	defaultBody := []byte("# No trusted default hosts: the selected file must be used.\n")
	if err := os.WriteFile(defaultTrustPath, defaultBody, 0o600); err != nil {
		return err
	}
	clientEnv := remoteTrustEnvironment(os.Environ(), map[string]string{"HOME": filepath.Join(clientDir, "home"), "XDG_CONFIG_HOME": filepath.Join(clientDir, "home"), envXDGRuntimeDir: filepath.Join(clientDir, "r"), "ZE_SSH_PASSWORD": password, "ZE_SSH_KNOWN_HOSTS": trustPath, "ZE_SSH_INSECURE": "false"})
	client := func(environment []string) fixture10ProcessResult {
		return remoteTrustRunCLI(ctx, binary, clientDir, address, environment)
	}
	checkFiles := func(body []byte) error {
		for path, expected := range map[string][]byte{trustPath: body, defaultTrustPath: defaultBody} {
			got, err := os.ReadFile(path) //nolint:gosec // the path is one of the two trust files this fixture wrote
			if err != nil {
				return fmt.Errorf("product CLI changed trust file %s: %w", path, err)
			}
			if !bytes.Equal(got, expected) {
				return fmt.Errorf("product CLI changed trust file %s", path)
			}
		}
		return filepath.WalkDir(clientDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Name() == "database" || entry.Name() == "database.zefs" {
				return fmt.Errorf("standalone CLI unexpectedly created a local store at %s", path)
			}
			return nil
		})
	}
	unknown := []byte("# No trusted keys.\n")
	if err := os.WriteFile(trustPath, unknown, 0o600); err != nil {
		return err
	}
	if err := remoteTrustRefusalVerdict(client(clientEnv), "The SSH host key is unknown for "+address); err != nil {
		return err
	}
	if err := checkFiles(unknown); err != nil {
		return err
	}

	wrongPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	wrongKey, err := ssh.NewPublicKey(wrongPublic)
	if err != nil {
		return err
	}
	changed := []byte(knownhosts.Line([]string{address}, wrongKey) + "\n")
	if err := os.WriteFile(trustPath, changed, 0o600); err != nil {
		return err
	}
	if err := remoteTrustRefusalVerdict(client(clientEnv), "The SSH host key changed for "+address); err != nil {
		return err
	}
	if err := checkFiles(changed); err != nil {
		return err
	}

	verified := []byte(knownhosts.Line([]string{address}, key) + "\n")
	if err := os.WriteFile(trustPath, verified, 0o600); err != nil {
		return err
	}
	if err := remoteTrustVersionVerdict(client(clientEnv)); err != nil {
		return err
	}
	if err := checkFiles(verified); err != nil {
		return err
	}
	badPasswordEnv := childEnvironment(clientEnv, map[string]string{"ZE_SSH_PASSWORD": password + "-wrong"})
	if err := remoteTrustRefusalVerdict(client(badPasswordEnv), "unable to authenticate"); err != nil {
		return err
	}
	if err := checkFiles(verified); err != nil {
		return err
	}
	// The denial must leave the daemon usable for a valid operator session.
	if err := remoteTrustVersionVerdict(client(clientEnv)); err != nil {
		return err
	}
	if err := checkFiles(verified); err != nil {
		return err
	}
	if err := remoteTrustDaemonAlive(daemon, &daemonOutput); err != nil {
		return err
	}
	return nil
}

func remoteTrustEnvironment(base []string, overrides map[string]string) []string {
	clean := make([]string, 0, len(base))
	for _, assignment := range base {
		name, _, _ := strings.Cut(assignment, "=")
		if strings.HasPrefix(env.Normalize(name), "ze_") {
			continue
		}
		clean = append(clean, assignment)
	}
	return childEnvironment(clean, overrides)
}

func remoteTrustCopyBinary(ctx context.Context, source, target string, expected os.FileInfo) (copyErr error) {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	input, err := os.Open(source) //nolint:gosec // Resolved runner-owned executable.
	if err != nil {
		return err
	}
	defer func() { copyErr = errors.Join(copyErr, input.Close()) }()
	opened, err := input.Stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(expected, opened) || opened.Size() != expected.Size() || opened.Size() <= 0 {
		return errors.New("runner DUT changed before private copy")
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // Exclusively created inside the private download directory.
	if err != nil {
		return err
	}
	want := sha256.New()
	n, writeErr := io.CopyN(io.MultiWriter(output, want), remoteTrustContextReader{ctx: ctx, reader: input}, expected.Size())
	closeErr := output.Close()
	if writeErr != nil || closeErr != nil || n != expected.Size() {
		return fmt.Errorf("copy expected %d executable bytes, wrote %d: %w", expected.Size(), n, errors.Join(writeErr, closeErr))
	}
	after, err := input.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(expected, after) || after.Size() != expected.Size() || !after.ModTime().Equal(expected.ModTime()) {
		return errors.New("runner DUT changed during private copy")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != expected.Size() || os.SameFile(expected, info) {
		return errors.New("copied DUT is not a distinct regular executable of the expected size")
	}
	verify, err := os.Open(target) //nolint:gosec // This fixture just created and closed the private file.
	if err != nil {
		return err
	}
	defer func() { copyErr = errors.Join(copyErr, verify.Close()) }()
	got := sha256.New()
	if _, err := io.CopyN(got, remoteTrustContextReader{ctx: ctx, reader: verify}, expected.Size()); err != nil {
		return err
	}
	if !bytes.Equal(want.Sum(nil), got.Sum(nil)) {
		return errors.New("private DUT copy differs from the runner executable")
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return os.Chmod(target, expected.Mode().Perm())
}

// Check the lifecycle context between regular-file chunks without an extra
// copy worker. CopyN bounds both passes by the source's observed size.
type remoteTrustContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader remoteTrustContextReader) Read(buffer []byte) (int, error) {
	if err := context.Cause(reader.ctx); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func remoteTrustStartDaemon(ctx context.Context, binary, cwd, config string, environment []string, output *lockedBuffer) (*runner.ServerProcess, error) {
	command := exec.CommandContext(ctx, binary, "start", config) //nolint:gosec // Fixture-owned DUT and config.
	command.Dir, command.Env = cwd, environment
	command.Stdout, command.Stderr = output, output
	return runner.StartServerProcess(command)
}

func remoteTrustDaemonAlive(process *runner.ServerProcess, output *lockedBuffer) error {
	select {
	case <-process.Done():
		return fmt.Errorf("private remote daemon exited before proof completed: %w\n%s", exitCause(process.Wait()), output.String())
	default:
		return nil
	}
}

func remoteTrustOwnedHostKey(ctx context.Context, address string) (ssh.PublicKey, error) {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("reach proof-owned listener: %w", err)
	}
	defer connection.Close() //nolint:errcheck // The bootstrap handshake is deliberately aborted.
	deadline := time.Now().Add(deploymentBudget(20 * time.Second))
	if bound, ok := ctx.Deadline(); ok && bound.Before(deadline) {
		deadline = bound
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return nil, err
	}
	var key ssh.PublicKey
	abort := errors.New("proof-owned host key captured before authentication")
	config := &ssh.ClientConfig{User: "remote-proof-bootstrap", HostKeyCallback: func(_ string, _ net.Addr, offered ssh.PublicKey) error { key = offered; return abort }}
	client, _, _, handshakeErr := ssh.NewClientConn(connection, address, config)
	if client != nil {
		_ = client.Close()
	}
	if key == nil || !errors.Is(handshakeErr, abort) {
		return nil, fmt.Errorf("capture proof-owned listener key: %w", handshakeErr)
	}
	return key, nil
}

func remoteTrustRunCLI(ctx context.Context, binary, cwd, address string, environment []string) fixture10ProcessResult {
	commandCtx, cancel := context.WithTimeout(ctx, deploymentBudget(20*time.Second))
	defer cancel()
	command := exec.CommandContext(commandCtx, binary, "cli", "--remote", address, "--user", "remote-proof", "-c", "show version | json") //nolint:gosec // Fixture-owned DUT and selected endpoint.
	command.Dir, command.Env = cwd, environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if err != nil {
		code = 127
		if status, ok := errors.AsType[*exec.ExitError](err); ok {
			code = status.ExitCode()
		}
	}
	if commandCtx.Err() != nil {
		code = 124
	}
	return fixture10ProcessResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func remoteTrustVersionVerdict(result fixture10ProcessResult) error {
	var answer struct {
		Version string `json:"version"`
	}
	parseErr := json.Unmarshal([]byte(result.stdout), &answer)
	if parseErr != nil {
		return fmt.Errorf("verified standalone CLI did not return daemon version JSON: exit=%d stdout=%q stderr=%q parse=%w", result.code, result.stdout, result.stderr, parseErr)
	}
	if result.code != 0 {
		return fmt.Errorf("verified standalone CLI did not return daemon version JSON: exit=%d stdout=%q stderr=%q", result.code, result.stdout, result.stderr)
	}
	if answer.Version == "" {
		return fmt.Errorf("verified standalone CLI returned JSON with no daemon version: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	return nil
}

func remoteTrustRefusalVerdict(result fixture10ProcessResult, reason string) error {
	if result.code != 1 || result.stdout != "" || !strings.Contains(result.stderr, reason) {
		return fmt.Errorf("standalone CLI did not refuse with %q: exit=%d stdout=%q stderr=%q", reason, result.code, result.stdout, result.stderr)
	}
	return nil
}
