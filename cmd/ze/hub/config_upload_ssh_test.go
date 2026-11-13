//go:build ze_ssh

// Design: docs/architecture/appliance/remote-operations.md -- authenticated configuration upload.
// Related: service_ssh.go -- the standalone SSH builder exercised here.
// Related: main_reload.go -- the real reload, publication, and rollback boundary.

package hub

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const sshUploadBefore = "set environment log level info\nset system dns resolv-conf-path \"\"\n"
const sshUploadAfter = "set environment log level error\nset system dns resolv-conf-path \"\"\n"

// sshUploadPolicy checks the authenticated connection's identity and the shared
// edit authorization contract. A successful SSH login alone grants no write.
type sshUploadPolicy struct{}

func (sshUploadPolicy) Authorize(username, remote, command string, readOnly bool) bool {
	if username != "operator" {
		return false
	}
	if remote == "" {
		return false
	}
	if readOnly {
		return false
	}
	return command == api.ConfigAuthCommit
}

type sshUploadFixture struct {
	store    storage.Storage
	path     string
	provider *zeconfig.Provider
	address  string
	signer   gossh.Signer
	hostKey  gossh.PublicKey
	stop     func()
}

// newSSHUploadFixture uses the production standalone builder with web absent.
// Its reload callback runs the real candidate parser, daemon transaction,
// ConfigProvider refresh, and source promotion; it never writes an active
// configuration itself. The optional refusal is inside that reload's loader.
func newSSHUploadFixture(t *testing.T, refuseReload bool) *sshUploadFixture {
	t.Helper()
	resetAAABundleForTest(t)
	isolateBaseLogLevel(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "router.conf")
	if err := os.WriteFile(path, []byte(sshUploadBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestFileStore(t, path)
	if err := initializeConfigSource(store, path, []byte(sshUploadBefore)); err != nil {
		t.Fatal(err)
	}
	before, err := zeconfig.ReadConfig(sshUploadBefore, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	provider := zeconfig.NewProvider()
	migrator := &listenerMigrator{}
	initial := lowerForPlugins(before.Tree, provider)
	daemon, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, plugin.NewCoordinator(initial))
	if err != nil {
		t.Fatal(err)
	}
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(daemon.Stop)
	load := func() (map[string]any, *zeconfig.Tree, error) {
		if refuseReload {
			return nil, nil, errors.New("injected daemon reload refusal")
		}
		content, _, present, err := storage.ReadCandidateConfig(store, path)
		if err != nil {
			return nil, nil, err
		}
		if !present {
			return nil, nil, errors.New("upload did not stage a daemon candidate")
		}
		parsed, err := zeconfig.ReadConfig(string(content), path, nil)
		if err != nil {
			return nil, nil, err
		}
		return parsed.Tree.ToRunningMap(), parsed.Tree, nil
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	keyBlock, err := gossh.MarshalPrivateKey(private, "test appliance upload host")
	if err != nil {
		t.Fatal(err)
	}
	hostPath := filepath.Join(dir, "ssh-host-key")
	if err := store.WriteFile(hostPath, pem.EncodeToMemory(keyBlock), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKey := aaa.SSHPublicKey{
		Name: "upload-test", Type: signer.PublicKey().Type(),
		Key: base64.StdEncoding.EncodeToString(signer.PublicKey().Marshal()),
	}
	addressFile := filepath.Join(dir, "ssh.addr")
	mutations := &configMutationOwner{}
	if err := mutations.bind(store, path, nil, func(ctx context.Context) error {
		return doReloadContext(ctx, reloadSourceCommit, daemon, nil, provider, store, path, load, migrator)
	}); err != nil {
		t.Fatal(err)
	}
	stop := sshBuildStandaloneImpl(&sshStandaloneInputs{
		Config: infra.SSHExtractedConfig{Listen: "127.0.0.1:0", HostKeyPath: hostPath, HasConfig: true},
		Users: []authz.UserConfig{
			{Name: "operator", PublicKeys: []aaa.SSHPublicKey{publicKey}},
			{Name: "observer", PublicKeys: []aaa.SSHPublicKey{publicKey}},
		},
		Authorizer: sshUploadPolicy{}, Storage: store, ConfigDir: dir,
		ConfigPath: path, EphemeralFile: addressFile,
		Migrator:    migrator,
		ConfigOwner: mutations,
		Log:         slogutil.Logger("hub.upload-test"),
	})
	if stop == nil {
		t.Fatal("production standalone SSH builder did not start")
	}
	stopServer := stop
	stop = func() {
		mutations.close()
		stopServer()
	}
	t.Cleanup(stop)
	address, err := os.ReadFile(addressFile)
	if err != nil {
		t.Fatal(err)
	}
	return &sshUploadFixture{store: store, path: path, provider: provider, address: string(address), signer: signer, hostKey: signer.PublicKey(), stop: stop}
}

func (f *sshUploadFixture) dial(t *testing.T, username string) *gossh.Client {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", f.address)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	transport, channels, requests, err := gossh.NewClientConn(conn, f.address, &gossh.ClientConfig{
		User: username, Auth: []gossh.AuthMethod{gossh.PublicKeys(f.signer)},
		HostKeyCallback: gossh.FixedHostKey(f.hostKey),
	})
	if err != nil {
		_ = conn.Close() //nolint:errcheck // Handshake failure cleanup.
		t.Fatal(err)
	}
	client := gossh.NewClient(transport, channels, requests)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func sshUploadChannel(t *testing.T, client *gossh.Client) (gossh.Channel, <-chan *gossh.Request) {
	t.Helper()
	channel, requests, err := client.OpenChannel("session", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = channel.Close() })
	accepted, err := channel.SendRequest("subsystem", true, gossh.Marshal(struct{ Name string }{"ze-config-upload"}))
	if err != nil {
		t.Fatal(err)
	}
	if !accepted {
		t.Fatal("authenticated appliance SSH receiver refused its configuration-upload subsystem")
	}
	return channel, requests
}

// upload drives the actual SSH subsystem bytes and exit-status request. It
// joins the request reader even when the remote end rejects the body.
func (f *sshUploadFixture) upload(t *testing.T, username, content string) (string, string, uint32) {
	t.Helper()
	return f.uploadDeclared(t, username, content, uint32(len(content)))
}

func (f *sshUploadFixture) uploadDeclared(t *testing.T, username, content string, declared uint32) (string, string, uint32) {
	t.Helper()
	channel, requests := sshUploadChannel(t, f.dial(t, username))
	exit := make(chan uint32, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		for request := range requests {
			if request.Type == "exit-status" {
				var payload struct{ Status uint32 }
				if err := gossh.Unmarshal(request.Payload, &payload); err == nil {
					exit <- payload.Status
				}
			}
		}
	}()
	t.Cleanup(func() { _ = channel.Close(); <-joined })
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], declared)
	_, headerErr := channel.Write(header[:])
	_, writeErr := io.WriteString(channel, content)
	_ = channel.CloseWrite() //nolint:errcheck // A refusing receiver may already have closed its input.
	output, err := io.ReadAll(io.LimitReader(channel, 8193))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := io.ReadAll(io.LimitReader(channel.Stderr(), 8193))
	if err != nil {
		t.Fatal(err)
	}
	<-joined
	select {
	case status := <-exit:
		if status == 0 {
			if headerErr != nil {
				t.Fatalf("receiver reported success after an incomplete upload header: %v", headerErr)
			}
			if writeErr != nil {
				t.Fatalf("receiver reported success after an incomplete upload: %v", writeErr)
			}
		}
		return string(output), string(diagnostic), status
	default:
		t.Fatal("configuration upload ended without an explicit exit status")
		return "", "", 255
	}
}

func (f *sshUploadFixture) requireState(t *testing.T, content, level string) {
	t.Helper()
	active, err := storage.ReadActiveConfig(f.store, f.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != content {
		t.Fatalf("persistent active configuration = %q, want %q", active, content)
	}
	file, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(file) != content {
		t.Fatalf("selected file source = %q, want %q", file, content)
	}
	root, err := f.provider.Get("environment")
	if err != nil {
		t.Fatal(err)
	}
	log, ok := root["log"].(map[string]any)
	if !ok {
		t.Fatalf("running log configuration is missing: %#v", root)
	}
	if log["level"] != level {
		t.Fatalf("running log level = %v, want %s", log["level"], level)
	}
	_, _, present, err := storage.ReadCandidateConfig(f.store, f.path)
	if err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("completed upload left an uncommitted candidate")
	}
}

// TestSSHConfigUploadPublishesPersistentAndRunningState proves useful output
// through real key authentication and the production web-independent receiver.
func TestSSHConfigUploadPublishesPersistentAndRunningState(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	output, diagnostic, status := f.upload(t, "operator", sshUploadAfter)
	if status != 0 {
		t.Fatalf("upload refused: status=%d stdout=%q stderr=%q", status, output, diagnostic)
	}
	if output != "configuration applied\n" {
		t.Fatalf("upload receipt = %q", output)
	}
	f.requireState(t, sshUploadAfter, "error")
}

// TestSSHConfigUploadMovesItsOwnListener drives the actual listener migrator
// from inside the accepted upload. Its receipt must complete, the previous
// listener must close, and the new listener must accept the same pinned key.
func TestSSHConfigUploadMovesItsOwnListener(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	newAddress := reservation.Addr().String()
	_, port, err := net.SplitHostPort(newAddress)
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	content := sshUploadAfter + "set environment ssh enabled true\n" +
		"set environment ssh server upload ip 127.0.0.1\n" +
		"set environment ssh server upload port " + port + "\n"
	output, diagnostic, status := f.upload(t, "operator", content)
	if status != 0 {
		t.Fatalf("self-migration upload refused: status=%d stdout=%q stderr=%q", status, output, diagnostic)
	}
	if output != "configuration applied\n" {
		t.Fatalf("self-migration did not deliver its acceptance receipt: %q", output)
	}
	f.requireState(t, content, "error")
	if conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", f.address); err == nil {
		_ = conn.Close() //nolint:errcheck // Report the unexpected accepting old listener.
		t.Fatal("upload reported migration success while its former listener still accepted connections")
	}
	f.address = newAddress
	client := f.dial(t, "operator")
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestSSHConfigUploadInvalidPreservesState sends malformed configuration with
// a private marker and proves rejection changes neither authority nor leaks it.
func TestSSHConfigUploadInvalidPreservesState(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	const private = "upload-private-credential-7d4a"
	output, diagnostic, status := f.upload(t, "operator", "system { "+private)
	if status == 0 {
		t.Fatal("malformed upload reported success")
	}
	if strings.Contains(output+diagnostic, private) {
		t.Fatal("upload refusal disclosed configuration contents")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadReloadRefusalPreservesState reaches canonical candidate
// reload refusal and requires the same persistent and runtime values afterward.
func TestSSHConfigUploadReloadRefusalPreservesState(t *testing.T) {
	f := newSSHUploadFixture(t, true)
	_, _, status := f.upload(t, "operator", sshUploadAfter)
	if status == 0 {
		t.Fatal("daemon reload refusal reported success")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadRequiresEditAuthorization admits a real SSH key login
// whose identity lacks edit permission, then refuses the mutation.
func TestSSHConfigUploadRequiresEditAuthorization(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	_, _, status := f.upload(t, "observer", sshUploadAfter)
	if status == 0 {
		t.Fatal("authenticated observer changed configuration without edit permission")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadBodyLimit brackets the established four-MiB limit with
// valid config comments so it cannot pass by rejecting all nontrivial uploads.
func TestSSHConfigUploadBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "at-limit", size: 4 << 20},
		{name: "over-limit", size: (4 << 20) + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSSHUploadFixture(t, false)
			content := sshUploadAfter + "#" + strings.Repeat("x", tc.size-len(sshUploadAfter)-2) + "\n"
			_, _, status := f.upload(t, "operator", content)
			if tc.size == 4<<20 {
				if status != 0 {
					t.Fatal("configuration at the four-MiB limit was refused")
				}
				f.requireState(t, content, "error")
				return
			}
			if status == 0 {
				t.Fatal("configuration over the four-MiB limit was accepted")
			}
			f.requireState(t, sshUploadBefore, "info")
		})
	}
}

// TestSSHConfigUploadEmptyPreservesState distinguishes an empty complete body
// from a valid whole-configuration replacement.
func TestSSHConfigUploadEmptyPreservesState(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	_, _, status := f.upload(t, "operator", "")
	if status == 0 {
		t.Fatal("empty upload reported success")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadTruncatedValidPrefixPreservesState declares a larger body
// than it sends. The sent prefix is independently valid configuration, so an
// EOF-only receiver would incorrectly publish it.
func TestSSHConfigUploadTruncatedValidPrefixPreservesState(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	_, _, status := f.uploadDeclared(t, "operator", sshUploadAfter, uint32(len(sshUploadAfter)+64))
	if status == 0 {
		t.Fatal("truncated upload published a valid configuration prefix")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadTrailingBodyPreservesState requires the announced byte
// count to describe the entire upload, not merely an accepted prefix.
func TestSSHConfigUploadTrailingBodyPreservesState(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	_, _, status := f.uploadDeclared(t, "operator", sshUploadAfter+"# trailing bytes\n", uint32(len(sshUploadAfter)))
	if status == 0 {
		t.Fatal("upload accepted undeclared trailing configuration bytes")
	}
	f.requireState(t, sshUploadBefore, "info")
}

// TestSSHConfigUploadDisconnectWithValidPrefixDoesNotPublish declares 96 bytes
// but sends a shorter valid configuration prefix, disconnects, and joins the
// production SSH owner before checking that neither authority changed.
func TestSSHConfigUploadDisconnectWithValidPrefixDoesNotPublish(t *testing.T) {
	f := newSSHUploadFixture(t, false)
	client := f.dial(t, "operator")
	channel, requests := sshUploadChannel(t, client)
	joined := make(chan struct{})
	go func() { gossh.DiscardRequests(requests); close(joined) }()
	// Independent wire fixture: four-byte big-endian length, not an encoder
	// shared with the producer. sshUploadAfter is a valid shorter prefix.
	if _, err := channel.Write([]byte{0, 0, 0, 96}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(channel, sshUploadAfter); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	<-joined
	f.stop()
	f.requireState(t, sshUploadBefore, "info")
}
