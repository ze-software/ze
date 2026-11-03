package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

func availablePort09() (int, error) {
	// The listener closes on the next line, so there is nothing to cancel.
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("a tcp4 listener answered %T, want *net.TCPAddr", listener.Addr())
	}
	port := address.Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

func startProcess09(ctx context.Context, args []string, stdin io.Reader, environment []string, logPath string) (*exec.Cmd, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Stdin = stdin
	command.Stdout = os.Stdout
	command.Stderr = logFile
	command.Env = environment
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	if err := logFile.Close(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, err
	}
	return command, nil
}

func stopProcess09(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		<-done
	}
}

func waitForLog09(ctx context.Context, path, needle string) (string, error) {
	var content []byte
	ready := Poll(ctx, 100, 200*time.Millisecond, func() bool {
		content, _ = os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
		return bytes.Contains(content, []byte(needle))
	})
	if !ready {
		content, _ = os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
		return string(content), fmt.Errorf("%q never appeared in %s", needle, path)
	}
	return string(content), nil
}

func lgTLSStatusBody09(ctx context.Context, port int, fingerprint [sha256.Size]byte) (string, error) {
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // fixture proves the generated self-signed certificate is served
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://127.0.0.1:"+strconv.Itoa(port)+"/api/looking-glass/status", http.NoBody)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("TLS request to the looking glass failed: %w", err)
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return string(body), fmt.Errorf("TLS request returned HTTP %d", response.StatusCode)
	}
	if response.TLS == nil {
		return "", errors.New("looking glass response has no TLS connection state")
	}
	if len(response.TLS.PeerCertificates) == 0 {
		return "", errors.New("looking glass response has no peer certificate")
	}
	served := sha256.Sum256(response.TLS.PeerCertificates[0].Raw)
	if served != fingerprint {
		return "", fmt.Errorf("looking glass served certificate SHA-256 %x, want seeded %x", served, fingerprint)
	}
	return string(body), nil
}

// lgTLSSeed09 writes a fresh, known pair through the live store API. Its owner
// lock MUST be released before the daemon starts.
func lgTLSSeed09(dir string, cert, key []byte) (err error) {
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	if err := store.WriteFile(zefs.KeyWebCert.Pattern, cert, 0o600); err != nil {
		return err
	}
	return store.WriteFile(zefs.KeyWebKey.Pattern, key, 0o600)
}

// lgTLSPersisted09 opens the store only after shutdown and compares both PEM
// values exactly. Certificate existence alone cannot prove reuse.
func lgTLSPersisted09(dir string, cert, key []byte) (err error) {
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	gotCert, err := store.ReadFile(zefs.KeyWebCert.Pattern)
	if err != nil {
		return err
	}
	if !bytes.Equal(gotCert, cert) {
		return errors.New("looking glass persisted certificate differs from seed")
	}
	gotKey, err := store.ReadFile(zefs.KeyWebKey.Pattern)
	if err != nil {
		return err
	}
	if !bytes.Equal(gotKey, key) {
		return errors.New("looking glass persisted private key differs from seed")
	}
	return nil
}

func lgTLSDefaultOn09(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-lg-tls-default-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup
	adminDir := filepath.Join(root, "admin")
	if err := os.Mkdir(adminDir, 0o700); err != nil {
		return err
	}
	lgPort, err := availablePort09()
	if err != nil {
		return err
	}
	sshPort, err := availablePort09()
	if err != nil {
		return err
	}
	defaultConfig := fmt.Sprintf(`environment {
	looking-glass {
		enabled true
		server main {
			ip 127.0.0.1;
			port %d;
		}
	}
}
`, lgPort)
	plaintextConfig := fmt.Sprintf(`environment {
	looking-glass {
		enabled true
		tls false
		server main {
			ip 127.0.0.1;
			port %d;
		}
	}
}
`, lgPort)
	defaultPath := filepath.Join(adminDir, "lg-default.conf")
	plaintextPath := filepath.Join(adminDir, "lg-plaintext.conf")
	if err := os.WriteFile(defaultPath, []byte(defaultConfig), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(plaintextPath, []byte(plaintextConfig), 0o600); err != nil {
		return err
	}
	environment := append(env.Without(os.Environ(), "ze.config.dir"), "ze.config.dir="+adminDir)
	initCommand := exec.CommandContext(ctx, "ze", "init")
	initCommand.Env = environment
	initCommand.Stdin = strings.NewReader(fmt.Sprintf("admin\ntestpass\n127.0.0.1\n%d\n", sshPort))
	initCommand.Stdout = io.Discard
	var initError bytes.Buffer
	initCommand.Stderr = &initError
	if err := initCommand.Run(); err != nil {
		return fmt.Errorf("ze init: %w: %s", err, initError.String())
	}
	cert, key, err := selfcert.GenerateWebCertWithAddr("127.0.0.1")
	if err != nil {
		return err
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return err
	}
	if err := lgTLSSeed09(adminDir, cert, key); err != nil {
		return fmt.Errorf("seed looking glass certificate: %w", err)
	}

	var daemon *exec.Cmd
	defer func() { stopProcess09(daemon) }()
	defaultLog := filepath.Join(root, "default.log")
	daemon, err = startProcess09(ctx, []string{actionStart, defaultPath}, nil, environment, defaultLog)
	if err != nil {
		return fmt.Errorf("start default TLS daemon: %w", err)
	}
	logText, err := waitForLog09(ctx, defaultLog, "looking glass listening on")
	if err != nil {
		return fmt.Errorf("%w: %s", err, logText)
	}
	httpsBanner := fmt.Sprintf("looking glass listening on https://127.0.0.1:%d/", lgPort)
	if !strings.Contains(logText, httpsBanner) {
		return fmt.Errorf("default looking glass did not announce https: %s", logText)
	}
	fmt.Fprintln(os.Stderr, "OK: looking glass defaults to TLS")
	body, err := lgTLSStatusBody09(ctx, lgPort, sha256.Sum256(pair.Certificate[0]))
	if err != nil {
		return err
	}
	if !strings.Contains(body, "router_id") {
		return fmt.Errorf("https status response did not carry router_id: %s", body)
	}
	fmt.Fprintln(os.Stderr, "OK: TLS handshake completed and the API answered")
	fmt.Fprintln(os.Stderr, "OK: looking glass served the seeded certificate")
	stopProcess09(daemon)
	daemon = nil
	if err := lgTLSPersisted09(adminDir, cert, key); err != nil {
		return fmt.Errorf("reopen looking glass store after shutdown: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: looking glass certificate and key persisted unchanged")

	plaintextLog := filepath.Join(root, "plaintext.log")
	daemon, err = startProcess09(ctx, []string{actionStart, plaintextPath}, nil, environment, plaintextLog)
	if err != nil {
		return fmt.Errorf("start plaintext daemon: %w", err)
	}
	logText, err = waitForLog09(ctx, plaintextLog, "looking glass listening on")
	if err != nil {
		return fmt.Errorf("%w: %s", err, logText)
	}
	httpBanner := fmt.Sprintf("looking glass listening on http://127.0.0.1:%d/", lgPort)
	if !strings.Contains(logText, httpBanner) {
		return fmt.Errorf("tls false did not serve plaintext: %s", logText)
	}
	fmt.Fprintln(os.Stderr, "OK: the tls false opt-out is honored")
	stopProcess09(daemon)
	daemon = nil
	return nil
}

func lgHTTPStatus09(ctx context.Context, port int, path, authorization string) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, http.NoBody)
	if err != nil {
		return 0, err
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}).Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	return response.StatusCode, nil
}

func lgTokenGate09(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-lg-token-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup
	port, err := availablePort09()
	if err != nil {
		return err
	}
	config := fmt.Sprintf(`environment {
	looking-glass {
		enabled true
		tls false
		token lg-s3cret-token
		server main {
			ip 127.0.0.1;
			port %d;
		}
	}
}
`, port)
	logPath := filepath.Join(root, "daemon.log")
	daemon, err := startProcess09(ctx, []string{"-"}, strings.NewReader(config), os.Environ(), logPath)
	if err != nil {
		return fmt.Errorf("start token-gated daemon: %w", err)
	}
	defer stopProcess09(daemon)
	logText, err := waitForLog09(ctx, logPath, "looking glass listening on")
	if err != nil {
		return fmt.Errorf("%w: %s", err, logText)
	}
	for _, path := range []string{"/api/looking-glass/status", "/lg/peers"} {
		status, err := lgHTTPStatus09(ctx, port, path, "")
		if err != nil {
			return fmt.Errorf("%s without token: %w", path, err)
		}
		if status != http.StatusUnauthorized {
			return fmt.Errorf("%s without a token returned %d, want 401", path, status)
		}
		status, err = lgHTTPStatus09(ctx, port, path, "Bearer wrong-token")
		if err != nil {
			return fmt.Errorf("%s with wrong token: %w", path, err)
		}
		if status != http.StatusUnauthorized {
			return fmt.Errorf("%s with a wrong token returned %d, want 401", path, status)
		}
		status, err = lgHTTPStatus09(ctx, port, path, "Bearer lg-s3cret-token")
		if err != nil {
			return fmt.Errorf("%s with configured token: %w", path, err)
		}
		if status == http.StatusUnauthorized {
			return fmt.Errorf("%s rejected the configured token", path)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: the configured token gates every looking glass route")
	return nil
}
