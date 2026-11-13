// Design: docs/architecture/testing/qemu-integration.md -- native push orchestration boundaries.
package testqemu

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configupload"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestApplianceConfigPushNativeSequence proves the orchestration contract at
// its process, command and read-only disk boundaries. It is not a VM proof.
func TestApplianceConfigPushNativeSequence(t *testing.T) {
	fixture := newConfigPushProtocolFixture(t)
	runner, ok := any(fixture.installer).(applianceConfigPushRunner)
	if !ok {
		t.Fatal("missing native configuration push orchestration")
	}
	report, err := runner.executeApplianceConfigPush(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if report.Verdict != InstallVerdictPass {
		t.Fatalf("native sequence report=%+v", report)
	}
	if fixture.builds != 1 || fixture.boots != 3 {
		t.Fatalf("builds=%d boots=%d, want one current image and three boots of one disk", fixture.builds, fixture.boots)
	}
	if fixture.previews != 1 || fixture.pushes != 3 || fixture.accepted != 1 {
		t.Fatalf("previews=%d product pushes=%d accepted=%d", fixture.previews, fixture.pushes, fixture.accepted)
	}
	if fixture.malformed != 1 || fixture.denied != 1 || fixture.badPins != 1 {
		t.Fatalf("missing refusals: malformed=%d denied=%d bad pin=%d", fixture.malformed, fixture.denied, fixture.badPins)
	}
	if fixture.snapshots < 3 {
		t.Fatalf("selected-source stopped-disk reads=%d, want bootstrap/publication/refusals", fixture.snapshots)
	}
	if fixture.passwords < 2 || fixture.keys < 2 {
		t.Fatalf("retained authenticated logins: password=%d key=%d", fixture.passwords, fixture.keys)
	}
	fixture.assertStopped(t)
}

// TestApplianceConfigPushFailureJoinsOwnedResources proves that a real command
// failure after boot cannot leave the proof's agent socket or VM behind.
func TestApplianceConfigPushFailureJoinsOwnedResources(t *testing.T) {
	fixture := newConfigPushProtocolFixture(t)
	fixture.failPush = true
	runner, ok := any(fixture.installer).(applianceConfigPushRunner)
	if !ok {
		t.Fatal("missing native configuration push orchestration")
	}
	report, err := runner.executeApplianceConfigPush(t.Context())
	if err == nil || report.Verdict == InstallVerdictPass {
		t.Fatalf("failed publication accepted: report=%+v err=%v", report, err)
	}
	if fixture.pushes != 1 {
		t.Fatalf("failure did not reach actual product command boundary: %d", fixture.pushes)
	}
	fixture.assertStopped(t)
}

// TestApplianceConfigPushObserverLoginRequired proves that the edit-denial
// control cannot pass on a broken observer login. The fixture refuses the
// observer's key, so the host driver fails at authentication and never reaches
// edit authorization; the proof MUST fail rather than count that as a refusal.
func TestApplianceConfigPushObserverLoginRequired(t *testing.T) {
	fixture := newConfigPushProtocolFixture(t)
	fixture.refuseObserver = true
	runner, ok := any(fixture.installer).(applianceConfigPushRunner)
	if !ok {
		t.Fatal("missing native configuration push orchestration")
	}
	report, err := runner.executeApplianceConfigPush(t.Context())
	if err == nil {
		t.Fatalf("broken observer login passed as an edit refusal: report=%+v", report)
	}
	if report.Verdict == InstallVerdictPass {
		t.Fatalf("failed proof reported a pass: report=%+v", report)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.accepted != 1 || fixture.malformed != 1 {
		t.Fatalf("proof did not reach the observer control: accepted=%d malformed=%d", fixture.accepted, fixture.malformed)
	}
	if fixture.denied != 0 || fixture.badPins != 0 {
		t.Fatalf("proof continued past an unauthenticated observer: denied=%d bad pin=%d", fixture.denied, fixture.badPins)
	}
	fixture.assertStopped(t)
}

// TestApplianceConfigPushFixtureProcess is the bounded child used by the
// process seam; EOF models QEMU's successful -no-reboot exit.
func TestApplianceConfigPushFixtureProcess(t *testing.T) {
	if os.Getenv("ZE_CONFIG_PUSH_FIXTURE_PROCESS") != "1" {
		return
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

type configPushProtocolFixture struct {
	t                                                                                                 *testing.T
	installer                                                                                         *Installer
	mu                                                                                                sync.Mutex
	host                                                                                              ssh.Signer
	hostPEM                                                                                           []byte
	operator                                                                                          ssh.PublicKey
	listener                                                                                          net.Listener
	server                                                                                            *httptest.Server
	workers                                                                                           sync.WaitGroup
	connections                                                                                       map[net.Conn]struct{}
	processes                                                                                         []*exec.Cmd
	input                                                                                             *os.File
	ports                                                                                             []int
	seed, acceptedConfig, home, socket, disk                                                          string
	builds, boots, previews, pushes, accepted, malformed, denied, badPins, snapshots, passwords, keys int
	failPush, refuseObserver                                                                          bool
}

func newConfigPushProtocolFixture(t *testing.T) *configPushProtocolFixture {
	t.Helper()
	registerConfigPushSchema(t)
	installer := installTestInstaller(t, InstallKindHTTP)
	installer.Options.Arch, installer.Options.Image = runtime.GOARCH, ""
	installer.Options.Accelerator = "kvm"
	installer.Options.SSHUser = "proof-operator"
	installer.Options.BootTimeout = 3 * time.Second
	fixture := &configPushProtocolFixture{t: t, installer: installer, connections: make(map[net.Conn]struct{})}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture.host, err = ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "owned native fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixture.hostPEM = pem.EncodeToMemory(block)
	fixture.listener, err = net.Listen("tcp", net.JoinHostPort(forwardHost, "0"))
	if err != nil {
		t.Fatal(err)
	}
	fixture.workers.Go(fixture.serve)
	fixture.server = httptest.NewTLSServer(http.HandlerFunc(fixture.management))
	sshPort := fixture.listener.Addr().(*net.TCPAddr).Port
	updatePort := fixture.server.Listener.Addr().(*net.TCPAddr).Port
	fixture.ports = append(fixture.ports, sshPort)
	for range 3 {
		fixture.ports = append(fixture.ports, updatePort, updatePort)
	}
	installer.ops.Port = func() (int, error) {
		if len(fixture.ports) == 0 {
			return 0, errors.New("unexpected extra guest forward")
		}
		port := fixture.ports[0]
		fixture.ports = fixture.ports[1:]
		return port, nil
	}
	installer.ops.Start = fixture.start
	installer.ops.Run = fixture.run
	seedDir := filepath.Join(installer.Tree, "gokrazy", "ze")
	if err := os.MkdirAll(seedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "ze.conf"), []byte("set environment ssh enabled true\nset environment web enabled true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fixture.close)
	return fixture
}

func (fixture *configPushProtocolFixture) assertStopped(t *testing.T) {
	t.Helper()
	for _, process := range fixture.processes {
		if process.ProcessState == nil {
			t.Fatal("owned guest process was not joined")
		}
	}
	if fixture.socket == "" {
		t.Fatal("product command had no private agent socket")
	}
	connection, err := net.Dial("unix", fixture.socket)
	if err == nil {
		_ = connection.Close()
		t.Fatal("owned agent accepts connections after proof returned")
	}
}

func (fixture *configPushProtocolFixture) start(command *exec.Cmd) error {
	if !slices.Contains(command.Args, "-no-reboot") {
		return errors.New("native config proof boot can reboot behind its persistence check")
	}
	for _, argument := range command.Args {
		if strings.HasPrefix(argument, "file=") && strings.Contains(argument, "target.img") {
			disk := strings.Split(strings.TrimPrefix(argument, "file="), ",")[0]
			if fixture.disk != "" && fixture.disk != disk {
				return errors.New("native proof changed target disk")
			}
			fixture.disk = disk
		}
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return errors.Join(err, reader.Close(), writer.Close())
	}
	command.Path, command.Args, command.Err = executable, []string{executable, "-test.run=^TestApplianceConfigPushFixtureProcess$"}, nil
	command.Env = append(os.Environ(), "ZE_CONFIG_PUSH_FIXTURE_PROCESS=1")
	command.Stdin = reader
	if err := command.Start(); err != nil {
		return errors.Join(err, reader.Close(), writer.Close())
	}
	if err := reader.Close(); err != nil {
		return err
	}
	fixture.mu.Lock()
	fixture.input = writer
	fixture.mu.Unlock()
	fixture.processes = append(fixture.processes, command)
	fixture.boots++
	return nil
}

func (fixture *configPushProtocolFixture) management(w http.ResponseWriter, r *http.Request) {
	user, password, _ := r.BasicAuth()
	if user != "gokrazy" || password != "owned-update-token" {
		http.Error(w, "wrong updater credentials", 401)
		return
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /":
		_ = json.NewEncoder(w).Encode(otaStatus{BuildTimestamp: "same-current-image", PermUUID: "same-owned-perm"})
	case "POST /reboot":
		fixture.mu.Lock()
		input := fixture.input
		fixture.input = nil
		fixture.mu.Unlock()
		if input == nil {
			http.Error(w, "guest not running", 409)
			return
		}
		_ = input.Close()
	default:
		http.Error(w, "HTTP cannot mutate this native proof", 404)
	}
}

func (fixture *configPushProtocolFixture) run(ctx context.Context, spec commandSpec) (commandResult, error) {
	if spec.Name == "go" {
		return commandResult{}, nil
	}
	if len(spec.Args) > 1 && spec.Args[0] == "-R" {
		return commandResult{}, fixture.snapshot(spec.Args[1])
	}
	appDir := ""
	for _, value := range spec.Env {
		if dir, ok := strings.CutPrefix(value, "ZE_APPLIANCE_DIR="); ok {
			appDir = filepath.Join(dir, InstallApplianceName)
		}
		if home, ok := strings.CutPrefix(value, "HOME="); ok {
			fixture.home = home
		}
		if socket, ok := strings.CutPrefix(value, "SSH_AUTH_SOCK="); ok {
			fixture.socket = socket
		}
	}
	if len(spec.Args) < 2 {
		return commandResult{}, errors.New("missing actual product verb")
	}
	switch spec.Args[1] {
	case "init":
		if err := os.MkdirAll(filepath.Join(appDir, "secrets", "tls"), 0o700); err != nil {
			return commandResult{}, err
		}
		inputIndex := slices.Index(spec.Args, "--config")
		if inputIndex < 0 {
			return commandResult{}, errors.New("bootstrap named keys were not supplied to init's JSON producer")
		}
		namedInput, err := os.ReadFile(spec.Args[inputIndex+1])
		if err != nil {
			return commandResult{}, err
		}
		for name, data := range map[string][]byte{
			"appliance.json":       namedInput,
			"secrets/update.token": []byte("owned-update-token"),
			"secrets/tls/cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw}),
		} {
			if err := os.WriteFile(filepath.Join(appDir, name), data, 0o600); err != nil {
				return commandResult{}, err
			}
		}
		return commandResult{}, nil
	case "build":
		fixture.builds++
		data, err := os.ReadFile(filepath.Join(appDir, "appliance.json"))
		if err != nil {
			return commandResult{}, err
		}
		var named struct {
			Credentials struct {
				Username string   `json:"username"`
				Keys     []string `json:"ssh-authorized-keys"`
			} `json:"credentials"`
		}
		if err := json.Unmarshal(data, &named); err != nil {
			return commandResult{}, err
		}
		if named.Credentials.Username != "proof-operator" || len(named.Credentials.Keys) != 1 {
			return commandResult{}, errors.New("native image bypassed named bootstrap credentials")
		}
		fixture.operator, _, _, _, err = ssh.ParseAuthorizedKey([]byte(named.Credentials.Keys[0]))
		if err != nil {
			return commandResult{}, err
		}
		seed, err := os.ReadFile(filepath.Join(appDir, "ze.conf"))
		if err != nil {
			return commandResult{}, err
		}
		fixture.seed = string(seed)
		fixture.acceptedConfig = fixture.seed
		image := filepath.Join(appDir, "ze-current.img")
		writeGPTFixture(fixture.t, image, 0)
		file, err := os.OpenFile(image, os.O_WRONLY, 0)
		if err != nil {
			return commandResult{}, err
		}
		err = file.Truncate(4096 * 512)
		return commandResult{}, errors.Join(err, file.Close())
	case "config-push":
		if spec.Name != filepath.Join(filepath.Dir(filepath.Dir(appDir)), "ze-host") {
			return commandResult{}, errors.New("push did not use this run's current host driver")
		}
		seed, err := os.ReadFile(filepath.Join(appDir, "ze.conf"))
		if err != nil {
			return commandResult{}, err
		}
		if slices.Contains(spec.Args, "--dry-run") {
			fixture.previews++
			return commandResult{Stdout: string(seed)}, nil
		}
		fixture.pushes++
		if fixture.failPush {
			return commandResult{Code: 1}, nil
		}
		return fixture.productPush(ctx, appDir, string(seed))
	default:
		return commandResult{}, fmt.Errorf("unexpected product operation: %v", spec.Args)
	}
}

func (fixture *configPushProtocolFixture) productPush(ctx context.Context, appDir, content string) (commandResult, error) {
	data, err := os.ReadFile(filepath.Join(appDir, "appliance.json"))
	if err != nil {
		return commandResult{}, err
	}
	var named struct {
		Credentials struct {
			Username string `json:"username"`
		} `json:"credentials"`
		SSH struct {
			Port string `json:"port"`
		} `json:"ssh"`
		Device struct {
			Address string `json:"address"`
		} `json:"device"`
	}
	if err := json.Unmarshal(data, &named); err != nil {
		return commandResult{}, err
	}
	verify, err := knownhosts.New(filepath.Join(fixture.home, ".ssh", "known_hosts"))
	if err != nil {
		return commandResult{}, err
	}
	address := net.JoinHostPort(named.Device.Address, named.SSH.Port)
	if err := verify(address, fixture.listener.Addr(), fixture.host.PublicKey()); err != nil {
		fixture.badPins++
		return commandResult{Code: 1}, nil
	}
	connection, err := net.Dial("unix", fixture.socket)
	if err != nil {
		return commandResult{}, err
	}
	defer connection.Close()
	signers, err := agent.NewClient(connection).Signers()
	if err != nil {
		return commandResult{}, err
	}
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{User: named.Credentials.Username, Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)}, HostKeyCallback: verify, Timeout: time.Second})
	if err != nil {
		// The host driver exits 1 for every connection or login failure.
		return commandResult{Code: 1}, nil
	}
	defer client.Close()
	upload, err := configPushExchange(client, content)
	if err != nil {
		return commandResult{}, err
	}
	if upload.status != 0 {
		return commandResult{Code: 1}, nil
	}
	if string(upload.output) != configupload.Receipt {
		return commandResult{}, errors.New("receiver receipt absent")
	}
	return commandResult{Stdout: "config applied to " + InstallApplianceName + "\n"}, nil
}

func (fixture *configPushProtocolFixture) snapshot(command string) error {
	for _, process := range fixture.processes {
		if process.ProcessState == nil {
			return errors.New("disk inspected while owned VM still running")
		}
	}
	var target string
	if _, err := fmt.Sscanf(command, "rdump /ze %q", &target); err != nil {
		return err
	}
	fixture.snapshots++
	root := filepath.Join(target, "ze")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	store, err := storage.Create(root)
	if err != nil {
		return err
	}
	defer store.Close()
	return fixture.snapshotStore(store)
}

func (fixture *configPushProtocolFixture) snapshotStore(store storage.Storage) error {
	if err := store.WriteKey(zefs.KeyFileActive.Key("configpush_host_ed25519"), fixture.hostPEM); err != nil {
		return err
	}
	fixture.mu.Lock()
	accepted := fixture.acceptedConfig
	fixture.mu.Unlock()
	if _, _, err := storage.EnsureActiveVersion(store, "ze.conf", []byte(accepted), time.Now()); err != nil {
		return err
	}
	return store.WriteFile("ze.conf", []byte("set system host stale-mirror\n"), 0o600)
}

func (fixture *configPushProtocolFixture) serve() {
	for {
		connection, err := fixture.listener.Accept()
		if err != nil {
			return
		}
		fixture.mu.Lock()
		fixture.connections[connection] = struct{}{}
		fixture.mu.Unlock()
		fixture.workers.Go(func() { fixture.sshConnection(connection) })
	}
}

func (fixture *configPushProtocolFixture) sshConnection(connection net.Conn) {
	defer func() {
		_ = connection.Close()
		fixture.mu.Lock()
		delete(fixture.connections, connection)
		fixture.mu.Unlock()
	}()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	server := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != fixture.installer.Options.SSHUser || string(password) != fixture.installer.Options.SSHPassword {
			return nil, errors.New("password refused")
		}
		fixture.mu.Lock()
		fixture.passwords++
		fixture.mu.Unlock()
		return &ssh.Permissions{}, nil
	}, PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), fixture.operator.Marshal()) {
			return nil, errors.New("unrelated key refused")
		}
		if fixture.refuseObserver && metadata.User() == configPushObserver {
			return nil, errors.New("observer key refused")
		}
		fixture.mu.Lock()
		fixture.keys++
		fixture.mu.Unlock()
		return &ssh.Permissions{}, nil
	}}
	server.AddHostKey(fixture.host)
	transport, channels, requests, err := ssh.NewServerConn(connection, server)
	if err != nil {
		return
	}
	defer transport.Close()
	requestsDone := make(chan struct{})
	go func() { defer close(requestsDone); ssh.DiscardRequests(requests) }()
	defer func() { _ = transport.Close(); <-requestsDone }()
	for incoming := range channels {
		channel, requests, err := incoming.Accept()
		if err != nil {
			return
		}
		for request := range requests {
			var operation struct{ Value string }
			if err := ssh.Unmarshal(request.Payload, &operation); err != nil {
				_ = request.Reply(false, nil)
				continue
			}
			if request.Type != "exec" && request.Type != "subsystem" {
				_ = request.Reply(false, nil)
				continue
			}
			_ = request.Reply(true, nil)
			code := uint32(0)
			if request.Type == "subsystem" {
				content, err := configupload.Read(channel)
				fixture.mu.Lock()
				if err != nil {
					code = 1
				} else if transport.User() != "proof-operator" {
					fixture.denied++
					code = 1
				} else if _, err := config.ParseTreeForValidation(string(content)); err != nil {
					fixture.malformed++
					code = 1
				} else {
					fixture.acceptedConfig = string(content)
					fixture.accepted++
					_, _ = io.WriteString(channel, configupload.Receipt)
				}
				fixture.mu.Unlock()
			} else {
				_, _ = io.Copy(io.Discard, channel)
				fixture.mu.Lock()
				content := fixture.acceptedConfig
				fixture.mu.Unlock()
				switch operation.Value {
				case HostnameQuery:
					tree, err := config.ParseTreeForValidation(content)
					if err != nil {
						code = 1
					} else {
						system := tree.GetContainer("system")
						name := ""
						if system != nil {
							name, _ = system.Get("host")
						}
						_, _ = fmt.Fprintf(channel, `{"host":{"hostname":%q}}`, name)
					}
				case "show config":
					_, _ = io.WriteString(channel, content)
				default:
					code = 1
				}
			}
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{code}))
			_ = channel.Close()
			break
		}
	}
}

func (fixture *configPushProtocolFixture) close() {
	fixture.server.Close()
	_ = fixture.listener.Close()
	fixture.mu.Lock()
	for connection := range fixture.connections {
		_ = connection.Close()
	}
	if fixture.input != nil {
		_ = fixture.input.Close()
	}
	fixture.mu.Unlock()
	fixture.workers.Wait()
}

// This leaf package does not link the product composition root. Keep the
// protocol fixture's schema limited to the canonical paths its seed needs.
func registerConfigPushSchema(t *testing.T) {
	t.Helper()
	t.Cleanup(yang.RegisterModuleForTest("ze-config-push-fixture-conf.yang", `module ze-config-push-fixture-conf {
 namespace "urn:ze:config-push-fixture";
 prefix proof;
 container environment {
  container ssh {
   leaf enabled { type boolean; }
   leaf host-key { type string; }
   list server { key "name"; leaf name { type string; } leaf ip { type string; } leaf port { type uint16; } }
  }
  container web { leaf enabled { type boolean; } }
 }
 container system {
  leaf host { type string; }
  container authentication {
   list user {
    key "name"; leaf name { type string; } leaf-list profile { type string; }
    list public-keys { key "name"; leaf name { type string; } leaf type { type string; } leaf key { type string; } }
   }
  }
  container authorization {
   list profile { key "name"; leaf name { type string; }
    container run { leaf default-action { type string; } }
    container edit { leaf default-action { type string; } }
   }
  }
 }
}`))
}
