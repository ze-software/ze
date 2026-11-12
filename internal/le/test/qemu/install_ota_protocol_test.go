// Design: docs/architecture/testing/qemu-integration.md -- actual OTA protocol.

package testqemu

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// VALIDATES: the runner commits through the public authenticated upload API.
// PREVENTS: a proof that stops at nonexistent SSH stage/apply commands.
func TestApplianceOTAUsesPublicConfigurationUpload(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.run(t)
}

// VALIDATES: testboot recovery reboots the retained original boot selection.
// PREVENTS: /update/switch promoting the tested slot instead of recovering.
func TestApplianceOTATestbootRecoveryDoesNotSwitch(t *testing.T) {
	// Accept the old invented configuration commands only in this fixture to
	// isolate recovery: the public-upload test rejects them independently.
	fixture := newOTAProtocolFixture(t, true)
	fixture.run(t)
}

type otaProtocolFixture struct {
	mu              sync.Mutex
	installer       *Installer
	legacyConfig    bool
	runtime         string
	host            string
	switched        bool
	uploads         int
	sshLogins       int
	hostnameQueries map[string]int
	forward         string
	webPort         int
	sshPort         int
	stallExec       chan struct{}
	sshStderr       string
	healthOutput    map[string]string
	healthError     map[string]bool
	healthBoots     []string
	closeSSH        func()
}

func (fixture *otaProtocolFixture) run(t *testing.T) {
	t.Helper()
	report, err := fixture.installer.executeApplianceOTA(context.Background())
	if err != nil {
		t.Fatalf("real runner protocol: %v", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if report.Verdict != InstallVerdictPass || fixture.runtime != "original" || fixture.host != otaPersistentHost {
		t.Fatalf("report=%+v runtime=%q host=%q", report, fixture.runtime, fixture.host)
	}
	if fixture.switched || fixture.uploads != 1 || fixture.sshLogins < 6 {
		t.Fatalf("switch=%v uploads=%d authenticated SSH logins=%d", fixture.switched, fixture.uploads, fixture.sshLogins)
	}
	// Health logins cannot replace the original hostname queries: two on the
	// updated runtime and four across the original and recovered runtime.
	for boot, want := range map[string]int{"original": 4, "updated": 2} {
		if fixture.hostnameQueries[boot] < want {
			t.Fatalf("hostname queries on %s runtime=%d, want at least %d", boot, fixture.hostnameQueries[boot], want)
		}
	}
	want := "hostfwd=tcp:" + net.JoinHostPort(forwardHost, strconv.Itoa(fixture.webPort)) + "-:8080"
	if !strings.Contains(fixture.forward, want) {
		t.Fatalf("missing loopback web forward %q: %s", want, fixture.forward)
	}
}

func newOTAProtocolFixture(t *testing.T, legacy bool) *otaProtocolFixture {
	t.Helper()
	registerOTAHealthSchema(t)
	fixture := &otaProtocolFixture{legacyConfig: legacy, runtime: "original", hostnameQueries: make(map[string]int)}
	installer := installTestInstaller(t, InstallKindISO)
	fixture.installer = installer
	installer.Tree = t.TempDir()
	installer.Options.Arch = runtime.GOARCH
	installer.Options.Image = ""
	installer.Options.BootTimeout = 2 * time.Second
	seedDir := filepath.Join(installer.Tree, "gokrazy", "ze")
	if err := os.MkdirAll(seedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "ze.conf"), []byte("set environment ssh enabled true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	update := httptest.NewTLSServer(http.HandlerFunc(fixture.update))
	t.Cleanup(update.Close)
	web := httptest.NewUnstartedServer(http.HandlerFunc(fixture.upload))
	web.TLS = update.TLS.Clone()
	web.StartTLS()
	t.Cleanup(web.Close)
	fixture.webPort = web.Listener.Addr().(*net.TCPAddr).Port
	sshPort := fixture.serveSSH(t)
	fixture.sshPort = sshPort
	ports := []int{sshPort, update.Listener.Addr().(*net.TCPAddr).Port, fixture.webPort}
	installer.ops.Port = func() (int, error) {
		if len(ports) == 0 {
			return 0, fmt.Errorf("unexpected extra forward")
		}
		port := ports[0]
		ports = ports[1:]
		return port, nil
	}
	installer.ops.Look = func(string) (string, error) { return "fixture-qemu", nil }
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installer.ops.Start = func(command *exec.Cmd) error {
		fixture.forward = strings.Join(command.Args, " ")
		command.Path, command.Args, command.Err = executable, []string{executable, "-test.run=^TestISOCaptureGuestProcess$"}, nil
		command.Env = append(os.Environ(), "ZE_ISO_CAPTURE_TEST=hang")
		return command.Start()
	}
	builds := 0
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		if spec.Name == "go" {
			return commandResult{}, nil
		}
		appDir := ""
		for _, value := range spec.Env {
			if dir, ok := strings.CutPrefix(value, "ZE_APPLIANCE_DIR="); ok {
				appDir = filepath.Join(dir, InstallApplianceName)
			}
		}
		if appDir == "" {
			return commandResult{}, fmt.Errorf("missing appliance directory")
		}
		switch spec.Args[1] {
		case "init":
			if err := os.MkdirAll(filepath.Join(appDir, "secrets", "tls"), 0o700); err != nil {
				return commandResult{}, err
			}
			for name, data := range map[string][]byte{
				"appliance.json":       []byte(`{"credentials":{"username":"admin","ssh-authorized-keys":["fixture-key"]}}`),
				"secrets/update.token": []byte("fixture-update-token"),
				"secrets/tls/cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: update.Certificate().Raw}),
			} {
				if err := os.WriteFile(filepath.Join(appDir, name), data, 0o600); err != nil {
					return commandResult{}, err
				}
			}
		case "build":
			builds++
			if err := os.WriteFile(filepath.Join(appDir, fmt.Sprintf("ze-%d.img", builds)), []byte("fixture-disk"), 0o600); err != nil {
				return commandResult{}, err
			}
		case "push":
			fixture.mu.Lock()
			fixture.runtime = "updated"
			fixture.mu.Unlock()
		default:
			return commandResult{}, fmt.Errorf("unexpected host command: %v", spec.Args)
		}
		return commandResult{}, nil
	}
	return fixture
}

func (fixture *otaProtocolFixture) update(w http.ResponseWriter, r *http.Request) {
	user, password, _ := r.BasicAuth()
	if user != "gokrazy" || password != "fixture-update-token" {
		http.Error(w, "wrong updater identity", http.StatusUnauthorized)
		return
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	switch r.Method + " " + r.URL.Path {
	case "GET /":
		if r.Header.Get("Accept") != "application/json" {
			http.Error(w, "missing JSON negotiation", http.StatusNotAcceptable)
			return
		}
		if err := json.NewEncoder(w).Encode(otaStatus{BuildTimestamp: fixture.runtime, PermUUID: "persistent-original"}); err != nil {
			return
		}
	case "POST /update/switch":
		fixture.switched = true
	case "POST /reboot":
		if !fixture.switched {
			fixture.runtime = "original"
		}
	default:
		http.Error(w, "unknown updater endpoint", http.StatusNotFound)
	}
}

func (fixture *otaProtocolFixture) upload(w http.ResponseWriter, r *http.Request) {
	user, password, _ := r.BasicAuth()
	if user != fixture.installer.Options.SSHUser || password != fixture.installer.Options.SSHPassword {
		http.Error(w, "wrong Ze identity", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != webUploadPath || r.Header.Get("Content-Type") != "text/plain" || r.Header.Get("Origin") != "https://"+r.Host {
		http.Error(w, "wrong upload protocol", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || !strings.Contains(string(body), "set environment ssh enabled true") || !strings.Contains(string(body), otaPersistentHost) {
		http.Error(w, "missing complete persisted configuration", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	fixture.host, fixture.uploads = otaPersistentHost, fixture.uploads+1
	fixture.mu.Unlock()
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusSeeOther)
}

func (fixture *otaProtocolFixture) serveSSH(t *testing.T) int {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(connection ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if connection.User() != fixture.installer.Options.SSHUser || string(password) != fixture.installer.Options.SSHPassword {
			return nil, fmt.Errorf("wrong Ze login")
		}
		fixture.mu.Lock()
		fixture.sshLogins++
		fixture.mu.Unlock()
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", net.JoinHostPort(forwardHost, "0"))
	if err != nil {
		t.Fatal(err)
	}
	var connectionsMu sync.Mutex
	connections := make(map[net.Conn]struct{})
	var connectionsDone sync.WaitGroup
	acceptDone := make(chan struct{})
	var closeOnce sync.Once
	fixture.closeSSH = func() {
		closeOnce.Do(func() {
			_ = listener.Close()
			<-acceptDone
			connectionsMu.Lock()
			for connection := range connections {
				_ = connection.Close()
			}
			connectionsMu.Unlock()
			connectionsDone.Wait()
		})
	}
	t.Cleanup(fixture.closeSSH)
	go func() {
		defer close(acceptDone)
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			connectionsMu.Lock()
			connections[connection] = struct{}{}
			connectionsMu.Unlock()
			connectionsDone.Add(1)
			go func() {
				defer connectionsDone.Done()
				defer func() { connectionsMu.Lock(); delete(connections, connection); connectionsMu.Unlock() }()
				fixture.sshConnection(connection, config)
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func (fixture *otaProtocolFixture) sshConnection(connection net.Conn, config *ssh.ServerConfig) {
	defer connection.Close() //nolint:errcheck // Fixture transport.
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	transport, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	requestsDone := make(chan struct{})
	defer func() { _ = transport.Close(); <-requestsDone }()
	go func() { defer close(requestsDone); ssh.DiscardRequests(requests) }()
	for request := range channels {
		channel, requests, err := request.Accept()
		if err != nil {
			return
		}
		for request := range requests {
			if request.Type != "exec" {
				_ = request.Reply(false, nil)
				continue
			}
			var command struct{ Command string }
			if ssh.Unmarshal(request.Payload, &command) != nil {
				_ = channel.Close()
				break
			}
			_ = request.Reply(true, nil)
			// A successful exec MUST consume stdin before sending its exit
			// status and closing the channel. Otherwise the client's input
			// copier races our close and Session.Wait returns EOF despite a
			// successful remote status, hiding the recovery assertion.
			if _, err := io.Copy(io.Discard, io.LimitReader(channel, 1<<20)); err != nil {
				_ = channel.Close()
				break
			}
			fixture.mu.Lock()
			stallExec := fixture.stallExec
			fixture.mu.Unlock()
			if stallExec != nil {
				close(stallExec)
				for range requests {
				}
				_ = channel.Close()
				return
			}
			fixture.mu.Lock()
			code := uint32(0)
			if command.Command == HostnameQuery {
				fixture.hostnameQueries[fixture.runtime]++
				_, _ = fmt.Fprintf(channel, `{"host":{"hostname":%q}}`, fixture.host)
			} else if command.Command == "show health | json" {
				fixture.healthBoots = append(fixture.healthBoots, fixture.runtime)
				output, ok := fixture.healthOutput[fixture.runtime]
				if !ok {
					output = otaHealthyResponse
				}
				_, _ = io.WriteString(channel, output)
				if fixture.healthError[fixture.runtime] {
					code = 1
					_, _ = io.WriteString(channel.Stderr(), "health command refused")
				}
			} else if fixture.legacyConfig && strings.HasPrefix(command.Command, "config ") {
				fixture.host = otaPersistentHost
			} else {
				code = 1
				_, _ = io.WriteString(channel.Stderr(), "unknown command: "+command.Command)
			}
			if fixture.sshStderr != "" {
				_, _ = io.WriteString(channel.Stderr(), fixture.sshStderr)
			}
			fixture.mu.Unlock()
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
			_ = channel.Close()
			break
		}
	}
}
