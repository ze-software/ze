package fixture

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	managedClientName   = "edge-01"
	managedToken        = "edge-secret-that-is-at-least-32chars"
	managedServerSecret = "local-server-secret-that-is-at-least-32chars"
)

type managedConfigAck struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

type managedConfigFetchResponse struct {
	Version string `json:"version,omitempty"`
	Config  string `json:"config,omitempty"`
	Status  string `json:"status,omitempty"`
}

func init() {
	Register("managed/config-push-transactional", managedConfigPushDriver)
}

func managedConfigPushDriver(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("managed/config-push-transactional", flag.ContinueOnError)
	scenario := flags.String("scenario", "both", "scenario")
	hubPort := flags.Int("hub-port", 0, "fake hub listen port")
	pluginPort := flags.Int("plugin-port", 0, "plugin hub listen port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *hubPort == 0 || *pluginPort == 0 {
		return errors.New("hub-port and plugin-port are required")
	}
	scenarios := []string{*scenario}
	if *scenario == modeBoth {
		scenarios = []string{outcomeSuccess, actionReject}
	}
	for _, name := range scenarios {
		if err := runManagedScenario(ctx, name, *hubPort, *pluginPort); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: managed config push committed and rejected transactionally")
	return nil
}

func runManagedScenario(ctx context.Context, name string, hubPort, pluginPort int) error {
	reject := name == actionReject
	if name != outcomeSuccess && name != actionReject {
		return fmt.Errorf("unknown scenario %q", name)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgDir, err := os.MkdirTemp(cwd, "managed-"+name+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfgDir) //nolint:errcheck // fixture cleanup
	dbPath := filepath.Join(cfgDir, "database.zefs")
	env := miscEnvironment(map[string]string{
		envConfigDir:              cfgDir,
		"ZE_MANAGED_TLS_INSECURE": valueTrue,
		envNoColor:                "1",
		envTestBGPPort:            strconv.Itoa(pluginPort + 1000),
	})
	if _, err := managedRunCommand(ctx, env, cfgDir, "admin\nsecret123\n127.0.0.1\n2222\n"+managedClientName+"\n", "init", "--managed"); err != nil {
		return err
	}
	active := managedConfig("1.1.1.1", hubPort, pluginPort, reject)
	pushed := managedConfig("2.2.2.2", hubPort, pluginPort, reject)
	activePath := filepath.Join(cfgDir, managedClientName+".conf")
	if err := os.WriteFile(activePath, []byte(active), 0o600); err != nil {
		return err
	}
	if _, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "import", activePath); err != nil {
		return err
	}
	ackCh, hubErrCh, closeHub, err := startManagedFakeHub(hubPort, managedToken, []byte(pushed))
	if err != nil {
		return err
	}
	defer closeHub()
	var zeOutput bytes.Buffer
	zeCommand := exec.CommandContext(ctx, "ze", "start")
	zeCommand.Dir = cfgDir
	zeCommand.Env = env
	zeCommand.Stdout = &zeOutput
	zeCommand.Stderr = &zeOutput
	zeCommand.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := zeCommand.Start(); err != nil {
		return fmt.Errorf("start ze: %w", err)
	}
	zeDone := make(chan error, 1)
	go func() { zeDone <- zeCommand.Wait() }()
	var ack managedConfigAck
	select {
	case ack = <-ackCh:
	case err := <-hubErrCh:
		stopManagedProcess(zeCommand, zeDone)
		return fmt.Errorf("fake hub: %w\nze output:\n%s", err, zeOutput.String())
	case err := <-zeDone:
		if err != nil {
			return fmt.Errorf("ze exited before config ack: %w\n%s", err, zeOutput.String())
		}
		return fmt.Errorf("ze exited before config ack: <nil>\n%s", zeOutput.String())
	case <-time.After(35 * time.Second):
		stopManagedProcess(zeCommand, zeDone)
		return fmt.Errorf("timed out waiting for config ack\nze output:\n%s", zeOutput.String())
	case <-ctx.Done():
		stopManagedProcess(zeCommand, zeDone)
		return ctx.Err()
	}
	stopManagedProcess(zeCommand, zeDone)
	expectedVersion := managedVersionHash([]byte(pushed))
	if ack.Version != expectedVersion {
		return fmt.Errorf("ack version %q, want %q", ack.Version, expectedVersion)
	}
	if reject {
		if ack.OK {
			return errors.New("reject scenario ACK was OK")
		}
		if !strings.Contains(ack.Error, "reject router-id 2.2.2.2") {
			return fmt.Errorf("reject scenario ACK error %q", ack.Error)
		}
	} else if !ack.OK {
		return fmt.Errorf("success scenario ACK failed: %s", ack.Error)
	}
	activeData, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "cat", "file/active/"+managedClientName+".conf")
	if err != nil {
		return err
	}
	activeText := string(activeData)
	if reject {
		if !strings.Contains(activeText, "router-id 1.1.1.1") || strings.Contains(activeText, "router-id 2.2.2.2") {
			return fmt.Errorf("rejected push changed active config:\n%s", activeText)
		}
	} else if !strings.Contains(activeText, "router-id 2.2.2.2") {
		return fmt.Errorf("accepted push did not update active config:\n%s", activeText)
	}
	if data, err := managedRunCommand(ctx, env, cfgDir, "", "data", "--path", dbPath, "cat", "meta/config/candidate"); err == nil {
		return fmt.Errorf("candidate pointer remains after %s scenario: %s", name, data)
	}
	return nil
}

func managedConfig(routerID string, hubPort, pluginPort int, reject bool) string {
	var builder textbuf.Buffer
	builder.Str("plugin {\n  hub {\n")
	fmt.Fprintf(&builder, "    client %s { host 127.0.0.1; port %d; secret %q; }\n", managedClientName, hubPort, managedToken)
	if reject {
		fmt.Fprintf(&builder, "    server local { ip 127.0.0.1; port %d; secret %q; }\n", pluginPort, managedServerSecret)
	}
	builder.Str("  }\n")
	if reject {
		builder.Str("  external managed-reject-plugin { run \"ze-test fixture managed/config-push-transactional-observer\"; encoder json; }\n")
	}
	builder.Str("}\n")
	fmt.Fprintf(&builder, "bgp {\n  router-id %s\n", routerID)
	if reject {
		builder.Str("  session { asn { local 1; } }\n")
		builder.Str("  peer peer1 {\n")
		builder.Str("    connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }\n")
		builder.Str("    session { asn { remote 1; } router-id 1.2.3.4; family { ipv4/unicast { prefix { maximum 10000; } } } }\n")
		builder.Str("    behavior { group-updates disable; }\n")
		builder.Str("    attach process managed-reject-plugin { }\n")
		builder.Str("  }\n")
	}
	builder.Str("}\n")
	return builder.String()
}

func startManagedFakeHub(port int, expectedToken string, configData []byte) (<-chan managedConfigAck, <-chan error, func(), error) {
	certificate, err := managedSelfSignedCert()
	if err != nil {
		return nil, nil, nil, err
	}
	listener, err := tls.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port), &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
	if err != nil {
		return nil, nil, nil, err
	}
	ackCh := make(chan managedConfigAck, 1)
	errCh := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer connection.Close() //nolint:errcheck // handler result is authoritative
		if err := handleManagedHubConnection(connection, expectedToken, configData, ackCh); err != nil {
			errCh <- err
		}
	}()
	return ackCh, errCh, func() { _ = listener.Close() }, nil
}

func handleManagedHubConnection(connection net.Conn, expectedToken string, configData []byte, ackCh chan<- managedConfigAck) error {
	reader := bufio.NewReader(connection)
	id, verb, payload, err := readManagedRPCLine(reader)
	if err != nil {
		return err
	}
	if verb != "auth" {
		return fmt.Errorf("expected auth, got %s", verb)
	}
	var auth struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(payload, &auth); err != nil {
		return err
	}
	if auth.Token != expectedToken || auth.Name != managedClientName {
		return fmt.Errorf("bad auth name=%q token-ok=%t", auth.Name, auth.Token == expectedToken)
	}
	if err := writeManagedOK(connection, id, nil); err != nil {
		return err
	}
	id, verb, _, err = readManagedRPCLine(reader)
	if err != nil {
		return err
	}
	if verb != "config-fetch" {
		return fmt.Errorf("expected config-fetch, got %s", verb)
	}
	response := managedConfigFetchResponse{Version: managedVersionHash(configData), Config: base64.StdEncoding.EncodeToString(configData)}
	if err := writeManagedOK(connection, id, response); err != nil {
		return err
	}
	id, verb, payload, err = readManagedRPCLine(reader)
	if err != nil {
		return err
	}
	if verb != "config-ack" {
		return fmt.Errorf("expected config-ack, got %s", verb)
	}
	var ack managedConfigAck
	if err := json.Unmarshal(payload, &ack); err != nil {
		return err
	}
	ackCh <- ack
	return writeManagedOK(connection, id, nil)
}

func readManagedRPCLine(reader *bufio.Reader) (uint64, string, []byte, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, "", nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "#") {
		return 0, "", nil, fmt.Errorf("missing RPC prefix: %q", line)
	}
	idPart, rest, ok := strings.Cut(line[1:], " ")
	if !ok {
		return 0, "", nil, fmt.Errorf("missing RPC verb: %q", line)
	}
	id, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil {
		return 0, "", nil, err
	}
	verb, payload, _ := strings.Cut(rest, " ")
	return id, verb, []byte(payload), nil
}

func writeManagedOK(writer io.Writer, id uint64, payload any) error {
	if payload == nil {
		_, err := fmt.Fprintf(writer, "#%d ok\n", id)
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "#%d ok %s\n", id, data)
	return err
}

func managedVersionHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func managedSelfSignedCert() (tls.Certificate, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "managed-test-hub"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return tls.X509KeyPair(certPEM, keyPEM)
}

func managedRunCommand(ctx context.Context, env []string, dir, stdin string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = env
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("ze %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func stopManagedProcess(command *exec.Cmd, done <-chan error) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-done
	}
}
