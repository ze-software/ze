// Design: ai/rules/plugins.md -- ze_ssh absent (compile-out) validation
//
//go:build !ze_ssh

package hub

// VALIDATES: without the ze_ssh build tag (e.g. ze-stripped), the ssh seam
// stays nil -- the compile-out proof at the seam layer (the go tool nm symbol
// check is the binary-level proof).
// PREVENTS: a regression where ssh leaks into a hardened build via an always-on
// import or an ungated seam installation.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

func TestBuildTag_SSH_Absent(t *testing.T) {
	if sshBuild != nil || sshWirePostStart != nil || sshBuildStandalone != nil {
		t.Fatal("non-ze_ssh build: ssh seam unexpectedly installed (ssh not compiled out)")
	}
}

func TestBuildTag_SSH_AbsentAcceptsSharedUserConfig(t *testing.T) {
	tree, err := zeconfig.ParseTreeWithYANG(sshSharedIdentityConfig, nil)
	if err != nil {
		t.Fatalf("non-ze_ssh build rejected shared authentication config: %v", err)
	}

	users := infra.ExtractAuthUsers(tree.GetContainer("system").ToMap())
	if len(users) != 1 {
		t.Fatalf("ExtractAuthUsers returned %d users, want 1", len(users))
	}
	if users[0].Name != "operator" || users[0].Hash == "" {
		t.Fatalf("shared user extraction = %#v, want operator with password", users[0])
	}
	if len(users[0].Profiles) != 1 || users[0].Profiles[0] != "readonly" {
		t.Fatalf("shared user profiles = %v, want [readonly]", users[0].Profiles)
	}
	if len(users[0].PublicKeys) != 0 {
		t.Fatalf("non-ze_ssh shared user unexpectedly has SSH public keys: %#v", users[0].PublicKeys)
	}
}

func TestBuildTag_SSH_AbsentRejectsSSHConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(sshOnlyTransportConfig, nil)
	if err == nil {
		t.Fatal("non-ze_ssh build unexpectedly accepted environment.ssh config")
	}
	if !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("SSH config rejection = %v, want unknown ssh field", err)
	}
}

func TestBuildTag_SSH_AbsentRejectsSSHPublicKeys(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(sshPublicKeyIdentityConfig, nil)
	if err == nil {
		t.Fatal("non-ze_ssh build unexpectedly accepted user public-keys")
	}
	if !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), "public-keys") {
		t.Fatalf("SSH public-key rejection = %v, want unknown public-keys field", err)
	}
}

func buildNoSSHBinary(t *testing.T, tags string) string {
	t.Helper()

	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-no-ssh")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags %q failed: %v\n%s", tags, err, out)
	}
	return bin
}

func assertNoSSHImplementationSymbols(t *testing.T, bin, tags string) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm for -tags %q failed: %v\n%s", tags, err, out)
	}
	needles := []string{
		"internal/component/ssh.",
		"internal/component/ssh/",
		"sshBuildImpl",
		"sshWireImpl",
		"sshBuildStandaloneImpl",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("-tags %q binary retained SSH implementation symbol %q matching %q", tags, line, needle)
			}
		}
	}
}

func TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols(t *testing.T) {
	const tags = "ze_core"
	bin := buildNoSSHBinary(t, tags)
	assertNoSSHImplementationSymbols(t, bin, tags)
}

type noSSHRESTResult struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func noSSHRESTExecute(t *testing.T, client *http.Client, url, credential, command string) (int, noSSHRESTResult) {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"command":%q}`, command))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("build REST request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("REST execute %q: %v", command, err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body is read before return
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read REST execute response: %v", err)
	}
	var result noSSHRESTResult
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("decode REST execute response %q: %v", raw, err)
		}
	}
	return resp.StatusCode, result
}

func TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser(t *testing.T) {
	const tags = "ze_core,ze_rest"
	bin := buildNoSSHBinary(t, tags)
	assertNoSSHImplementationSymbols(t, bin, tags)
	dir := t.TempDir()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve REST port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release REST port: %v", err)
	}

	const password = "operator-secret"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash config-user password: %v", err)
	}
	configPath := filepath.Join(dir, "ze.conf")
	configText := fmt.Sprintf(`
environment {
	api-server {
		rest {
			enabled true
			server main {
				ip 127.0.0.1
				port %d
			}
		}
	}
}
system {
	authentication {
		user operator {
			password %q
			profile [ readonly ]
		}
	}
	authorization {
		profile readonly {
			run {
				default-action deny
				entry 10 {
					action allow
					match "show version"
				}
			}
			edit { default-action deny }
		}
	}
}
`, port, string(hash))
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatalf("write no-SSH REST config: %v", err)
	}

	logPath := filepath.Join(dir, "ze.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create daemon log: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, bin, "start", configPath)
	cmd.Env = append(os.Environ(),
		"ZE_CONFIG_DIR="+dir,
		"ZE_API_SERVER_REST_ENABLED=",
		"ZE_API_SERVER_REST_LISTEN=",
		"ZE_API_SERVER_TOKEN=",
		"ZE_SSH_EPHEMERAL=",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		cancel()
		_ = logFile.Close()
		t.Fatalf("start no-SSH REST binary: %v", err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case <-wait:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-wait
		}
		_ = logFile.Close()
	}
	t.Cleanup(stop)

	client := &http.Client{Timeout: 300 * time.Millisecond}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1", port)
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/commands", nil)
		if reqErr != nil {
			t.Fatalf("build REST readiness request: %v", reqErr)
		}
		req.Header.Set("Authorization", "Bearer operator:"+password)
		resp, requestErr := client.Do(req)
		if requestErr == nil {
			_ = resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		stop()
		out, _ := os.ReadFile(logPath)
		t.Fatalf("no-SSH REST listener did not start:\n%s", out)
	}

	status, result := noSSHRESTExecute(t, client, baseURL+"/execute", "operator:"+password, "show version")
	if status != http.StatusOK || result.Status != "done" {
		t.Fatalf("allowed config-user command = HTTP %d, %#v; want 200/done", status, result)
	}

	status, _ = noSSHRESTExecute(t, client, baseURL+"/execute", "operator:wrong", "show version")
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong config-user password = HTTP %d, want 401", status)
	}

	status, result = noSSHRESTExecute(t, client, baseURL+"/execute", "operator:"+password, "request reload")
	if status != http.StatusForbidden {
		t.Fatalf("command outside profile = HTTP %d, %#v; want 403", status, result)
	}
}
