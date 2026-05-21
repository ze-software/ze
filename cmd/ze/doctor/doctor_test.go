// Design: docs/features/ai-first.md — doctor command tests

package doctor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/resolve"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	zeplugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

func TestMain(m *testing.M) {
	diagnostic.RegisterBuiltinCodes()
	env.MustRegister(env.EnvEntry{Key: "ze.storage.blob", Type: "bool", Default: "false", Description: "blob storage"})
	env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "config dir"})
	os.Exit(m.Run())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

const minimalConfig = `# empty config
`

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "ze.conf")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	return f
}

func TestDoctorHelp(t *testing.T) {
	code := Run([]string{"--help"})
	assert.Equal(t, 0, code)
}

func TestDoctorMissingConfig(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"--json", "/nonexistent/ze.conf"})
		assert.Equal(t, 1, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.False(t, result.Ready)
	assert.Equal(t, diagnostic.SchemaVersion, result.SchemaVersion)

	found := false
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-config-missing" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected doctor-config-missing diagnostic")
}

func TestDoctorValidConfigJSON(t *testing.T) {
	cfgPath := writeTestConfig(t, minimalConfig)
	out := captureStdout(t, func() {
		code := Run([]string{"--json", cfgPath})
		assert.Equal(t, 0, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.True(t, result.Ready)
	assert.Equal(t, diagnostic.SchemaVersion, result.SchemaVersion)

	for i := range result.Diagnostics {
		assert.NotEqual(t, diagnostic.SeverityError, result.Diagnostics[i].Severity,
			"unexpected error: %s", result.Diagnostics[i].Message)
	}
}

func TestDoctorValidConfigText(t *testing.T) {
	cfgPath := writeTestConfig(t, minimalConfig)
	out := captureStdout(t, func() {
		code := Run([]string{cfgPath})
		assert.Equal(t, 0, code)
	})
	assert.Contains(t, out, "all checks passed")
}

func TestDoctorInvalidConfig(t *testing.T) {
	cfgPath := writeTestConfig(t, "this is not valid config {{{")
	out := captureStdout(t, func() {
		code := Run([]string{"--json", cfgPath})
		assert.Equal(t, 1, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.False(t, result.Ready)

	found := false
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-config-parse" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected doctor-config-parse diagnostic")
}

func TestDoctorExtraArg(t *testing.T) {
	code := Run([]string{"file1.conf", "file2.conf"})
	assert.Equal(t, 1, code)
}

func TestCheckCertExpiry_Valid(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))
	diags := checkCertExpiry("test", "/test/cert.pem", certPEM)
	assert.Empty(t, diags)
}

func TestCheckCertExpiry_Expired(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))
	diags := checkCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-expired", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
}

func TestCheckCertExpiry_NotYetValid(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(24*time.Hour), time.Now().Add(365*24*time.Hour))
	diags := checkCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-expired", diags[0].Code)
	assert.Contains(t, diags[0].Message, "not yet valid")
}

func TestCheckCertExpiry_ExpiringSoon(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(-time.Hour), time.Now().Add(15*24*time.Hour))
	diags := checkCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-expired", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "expires in")
}

func TestCheckCertExpiry_InvalidPEM(t *testing.T) {
	diags := checkCertExpiry("test", "/test/cert.pem", []byte("not-pem"))
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-invalid", diags[0].Code)
	assert.Contains(t, diags[0].Message, "not valid PEM")
}

func TestCheckPlugins_InternalSkipped(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "rib", Internal: true, Run: "ze.rib"},
	}
	diags := checkPlugins(plugins)
	assert.Empty(t, diags)
}

func TestCheckPlugins_MissingBinary(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "custom", Internal: false, Run: "/nonexistent/binary"},
	}
	diags := checkPlugins(plugins)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-plugin-missing", diags[0].Code)
}

func sshEnabledTree() *config.Tree {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	return tree
}

func TestCheckSSHHostKey_Missing(t *testing.T) {
	dir := t.TempDir()
	diags := checkSSHHostKey(sshEnabledTree(), dir)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ssh-hostkey-missing", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
}

func TestCheckSSHHostKey_Present(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh_host_ed25519_key"), []byte("key"), 0o600))
	diags := checkSSHHostKey(sshEnabledTree(), dir)
	assert.Empty(t, diags)
}

func TestCheckSSHHostKey_NotEnabled(t *testing.T) {
	dir := t.TempDir()
	tree := config.NewTree()
	diags := checkSSHHostKey(tree, dir)
	assert.Empty(t, diags, "SSH not enabled should skip host key check")
}

func TestCheckSSHHostKey_EmptyDir(t *testing.T) {
	diags := checkSSHHostKey(sshEnabledTree(), "")
	assert.Empty(t, diags)
}

func TestCheckCertPair_KeyMissing(t *testing.T) {
	dir := t.TempDir()
	diags := checkCertPair("test", "", "/nonexistent/key.pem", dir)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-missing", diags[0].Code)
	assert.Contains(t, diags[0].Message, "key not found")
}

func TestCheckListeners_FreePort(t *testing.T) {
	tree := config.NewTree()
	diags := checkListeners(tree)
	assert.Empty(t, diags, "empty tree should produce no listener diagnostics")
}

func TestCheckListeners_PortInUse(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	web.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-listen-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "web")
}

func TestCheckListeners_SSH(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "0")
	ssh.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	assert.Empty(t, diags, "port 0 should bind successfully")
}

func TestCheckListeners_API(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	api := env.GetOrCreateContainer("api-server")
	rest := api.GetOrCreateContainer("rest")
	rest.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	rest.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-listen-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "api-rest")
}

func TestCheckCertExpiry_BadDER(t *testing.T) {
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-der")})
	diags := checkCertExpiry("test", "/test/cert.pem", pemData)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-invalid", diags[0].Code)
	assert.Contains(t, diags[0].Message, "cannot parse certificate")
}

type stubStorage struct {
	storage.Storage
	data map[string][]byte
}

func (s *stubStorage) ReadFile(name string) ([]byte, error) {
	if d, ok := s.data[name]; ok {
		return d, nil
	}
	return s.Storage.ReadFile(name)
}

func (s *stubStorage) Exists(name string) bool {
	if _, ok := s.data[name]; ok {
		return true
	}
	return s.Storage.Exists(name)
}

func TestResolveDefaultConfig_NoInstanceFile(t *testing.T) {
	store := storage.NewFilesystem()
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "ze.conf", name, "filesystem storage with no instance file should return ze.conf")
}

func TestResolveDefaultConfig_InvalidRegex(t *testing.T) {
	store := &stubStorage{
		Storage: storage.NewFilesystem(),
		data:    map[string][]byte{zefs.KeyInstanceName.Pattern: []byte("../etc")},
	}
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "ze.conf", name, "instance name failing regex should return ze.conf")
}

func TestResolveDefaultConfig_ValidName(t *testing.T) {
	store := &stubStorage{
		Storage: storage.NewFilesystem(),
		data:    map[string][]byte{zefs.KeyInstanceName.Pattern: []byte("myrouter")},
	}
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "myrouter.conf", name)
}

func TestExtractSSHListeners_DefaultFallback(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")

	listeners := extractSSHListeners(tree)
	require.Len(t, listeners, 1)
	assert.Equal(t, "ssh", listeners[0].service)
	assert.Equal(t, "127.0.0.1", listeners[0].host)
	assert.Equal(t, "2222", listeners[0].port)
}

func TestExtractSSHListeners_Disabled(t *testing.T) {
	tree := config.NewTree()
	listeners := extractSSHListeners(tree)
	assert.Nil(t, listeners)
}

func TestExtractSSHListeners_ServerList(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "10.0.0.1")
	srv.Set("port", "2223")
	ssh.AddListEntry("server", "s1", srv)

	listeners := extractSSHListeners(tree)
	require.Len(t, listeners, 1)
	assert.Equal(t, "ssh", listeners[0].service)
	assert.Equal(t, "10.0.0.1", listeners[0].host)
	assert.Equal(t, "2223", listeners[0].port)
}

func TestCheckWebTLS_NoCerts(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")

	store := storage.NewFilesystem()
	diags := checkWebTLS(tree, store)
	assert.Empty(t, diags, "no blob certs should produce no diagnostics")
}

func TestCheckWebTLS_ExpiredCert(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")

	store := &stubStorage{
		Storage: storage.NewFilesystem(),
		data: map[string][]byte{
			zefs.KeyWebCert.Pattern: certPEM,
			zefs.KeyWebKey.Pattern:  []byte("key-data"),
		},
	}
	diags := checkWebTLS(tree, store)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-expired", diags[0].Code)
}

func TestCheckWebTLS_CertWithoutKey(t *testing.T) {
	certPEM := generateTestCert(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")

	store := &stubStorage{
		Storage: storage.NewFilesystem(),
		data: map[string][]byte{
			zefs.KeyWebCert.Pattern: certPEM,
		},
	}
	diags := checkWebTLS(tree, store)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-missing", diags[0].Code)
	assert.Contains(t, diags[0].Message, "key missing")
}

func TestCheckWebTLS_KeyWithoutCert(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")

	store := &stubStorage{
		Storage: storage.NewFilesystem(),
		data: map[string][]byte{
			zefs.KeyWebKey.Pattern: []byte("key-data"),
		},
	}
	diags := checkWebTLS(tree, store)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-tls-missing", diags[0].Code)
	assert.Contains(t, diags[0].Message, "certificate missing")
}

func TestCheckWebTLS_Disabled(t *testing.T) {
	tree := config.NewTree()
	store := storage.NewFilesystem()
	diags := checkWebTLS(tree, store)
	assert.Empty(t, diags, "web not enabled should skip")
}

func TestResolveStorageWithDiag_Fallback(t *testing.T) {
	store, diags := resolveStorageWithDiag()
	assert.NotNil(t, store, "should always return a usable storage")
	for _, d := range diags {
		assert.NotEqual(t, diagnostic.SeverityError, d.Severity,
			"storage fallback should not produce errors")
	}
}

func generateTestCert(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}
