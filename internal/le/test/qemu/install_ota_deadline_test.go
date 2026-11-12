// Design: docs/architecture/testing/qemu-integration.md -- bounded OTA phases.

package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// VALIDATES: accepted SSH connections remain owned by their phase context.
// PREVENTS: canceled handshake/exec probes delaying cleanup by thirty seconds.
func TestApplianceOTASSHCancellation(t *testing.T) {
	for _, phase := range []string{"handshake", "exec"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			installer := installTestInstaller(t, InstallKindISO)
			guest := &otaGuest{installer: installer}
			var started <-chan struct{}
			var stop func()
			if phase == "exec" {
				fixture := newOTAProtocolFixture(t, false)
				fixture.mu.Lock()
				fixture.stallExec = make(chan struct{})
				fixture.mu.Unlock()
				guest.installer, guest.sshPort = fixture.installer, fixture.sshPort
				started, stop = fixture.stallExec, fixture.closeSSH
			} else {
				listener, err := net.Listen("tcp", net.JoinHostPort(forwardHost, "0"))
				if err != nil {
					t.Fatal(err)
				}
				guest.sshPort = listener.Addr().(*net.TCPAddr).Port
				var accepted net.Conn
				var acceptedMu sync.Mutex
				stopped := false
				ready, done := make(chan struct{}), make(chan struct{})
				go func() {
					defer close(done)
					connection, err := listener.Accept()
					if err != nil {
						return
					}
					acceptedMu.Lock()
					if stopped {
						_ = connection.Close()
						acceptedMu.Unlock()
						return
					}
					accepted = connection
					acceptedMu.Unlock()
					close(ready)
					_, _ = io.Copy(io.Discard, connection)
				}()
				started = ready
				var stopOnce sync.Once
				stop = func() {
					stopOnce.Do(func() {
						_ = listener.Close()
						acceptedMu.Lock()
						stopped = true
						if accepted != nil {
							_ = accepted.Close()
						}
						acceptedMu.Unlock()
						<-done
					})
				}
			}
			defer stop()
			result := make(chan error, 1)
			go func() { _, err := guest.ssh(ctx, HostnameQuery, ""); result <- err }()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				cancel()
				stop()
				<-result
				t.Fatal("SSH fixture did not reach the selected phase")
			}
			cancel()
			select {
			case err := <-result:
				stop()
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled %s lost context cause: %v", phase, err)
				}
			case <-time.After(time.Second):
				stop()
				<-result
				t.Fatalf("canceled %s continued waiting on its transport", phase)
			}
		})
	}
}

// VALIDATES: synchronous mutation phases can outlast a short status probe,
// while cancellation still ends the owned mutation and preserves its cause.
func TestApplianceOTAMutationDeadlines(t *testing.T) {
	for _, operation := range []string{"upload", "reboot"} {
		t.Run(operation, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			client := server.Client()
			defer client.CloseIdleConnections()
			client.Timeout = 100 * time.Millisecond
			guest := &otaGuest{installer: installTestInstaller(t, InstallKindISO), client: client, endpoint: server.URL, webEndpoint: server.URL}
			result := make(chan error, 1)
			go func() { result <- otaTestMutation(context.Background(), guest, operation) }()
			select {
			case <-started:
			case err := <-result:
				t.Fatalf("mutation ended before its controlled response: %v", err)
			case <-time.After(2 * time.Second):
				t.Fatal("mutation did not reach response fixture")
			}
			// The mutation is held across the independent status-probe budget.
			// The assertion is its successful publication/response, not elapsed time.
			<-time.After(3 * client.Timeout)
			close(release)
			if err := <-result; err != nil {
				t.Fatalf("valid %s inherited status-probe timeout: %v", operation, err)
			}
			if client.Timeout != 100*time.Millisecond {
				t.Fatal("mutation changed the shared status client")
			}
		})
		t.Run(operation+"-cancel", func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			guest := &otaGuest{installer: installTestInstaller(t, InstallKindISO), client: server.Client(), endpoint: server.URL, webEndpoint: server.URL}
			defer guest.client.CloseIdleConnections()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- otaTestMutation(ctx, guest, operation) }()
			<-started
			cancel()
			if err := <-result; !errors.Is(err, context.Canceled) {
				t.Fatalf("%s cancellation cause: %v", operation, err)
			}
		})
	}
}

func otaTestMutation(ctx context.Context, guest *otaGuest, operation string) error {
	switch operation {
	case "upload":
		return guest.uploadConfig(ctx, "set system host preserved\n")
	case "reboot":
		return guest.post(ctx, "/reboot")
	default:
		return fmt.Errorf("unknown mutation %q", operation)
	}
}

// VALIDATES: the named builder receives the requested Ze account before
// assembleZeFS publishes that identity into the persistent credential store.
func TestApplianceOTASeedsRequestedSSHUser(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.installer.Options.SSHUser = "proof-operator"
	originalRun := fixture.installer.ops.Run
	fixture.installer.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if len(spec.Args) > 1 && spec.Args[1] == "build" {
			var appDir string
			for _, value := range spec.Env {
				if dir, ok := strings.CutPrefix(value, "ZE_APPLIANCE_DIR="); ok {
					appDir = dir
				}
			}
			data, err := os.ReadFile(filepath.Join(appDir, InstallApplianceName, "appliance.json"))
			if err != nil {
				return commandResult{}, err
			}
			var config struct {
				Credentials struct {
					Username string   `json:"username"`
					Keys     []string `json:"ssh-authorized-keys"`
				} `json:"credentials"`
			}
			if err := json.Unmarshal(data, &config); err != nil {
				return commandResult{}, err
			}
			if config.Credentials.Username != "proof-operator" {
				return commandResult{}, fmt.Errorf("builder account %q does not match requested proof-operator", config.Credentials.Username)
			}
			if len(config.Credentials.Keys) != 1 || config.Credentials.Keys[0] != "fixture-key" {
				return commandResult{}, fmt.Errorf("existing credential configuration lost: %v", config.Credentials.Keys)
			}
		}
		return originalRun(ctx, spec)
	}
	if _, err := fixture.installer.prepareISOImage(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
