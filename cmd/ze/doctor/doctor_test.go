// Design: docs/features/ai-first.md — doctor command tests

package doctor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/resolve"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	zeplugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	zeradius "codeberg.org/thomas-mangin/ze/internal/component/radius"
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
	assert.True(t, strings.Contains(out, "all checks passed") || strings.Contains(out, "ready (0 errors"), "unexpected doctor output: %s", out)
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

func TestCheckDHCPInterfaces(t *testing.T) {
	// VALIDATES: AC-4 DHCP server configured with a missing listen interface returns doctor-dhcp-iface.
	// PREVENTS: DHCP bind failures surfacing only when the daemon starts.
	tree := config.NewTree()
	service := tree.GetOrCreateContainer("service")
	dhcp := service.GetOrCreateContainer("dhcp-server")
	dhcp.Set("enabled", "true")
	dhcp.SetSlice("listen-interface", []string{"ze-doctor-missing0"})

	diags := checkDHCPInterfaces(tree)
	requireDiag(t, diags, "doctor-dhcp-iface", diagnostic.SeverityError)
}

func TestCheckListeners_BGP(t *testing.T) {
	// VALIDATES: AC-5 BGP configured with a local address reports doctor-bgp-listen when the port is unavailable.
	// PREVENTS: BGP TCP bind conflicts being hidden until reactor startup.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	conn := peer.GetOrCreateContainer("connection")
	local := conn.GetOrCreateContainer("local")
	local.Set("ip", "127.0.0.1")
	local.Set("port", port)
	remote := conn.GetOrCreateContainer("remote")
	remote.Set("ip", "192.0.2.1")
	session := peer.GetOrCreateContainer("session")
	asn := session.GetOrCreateContainer("asn")
	asn.Set("remote", "65001")
	bgp.AddListEntry("peer", "p1", peer)

	diags := checkListeners(tree)
	requireDiag(t, diags, "doctor-bgp-listen", diagnostic.SeverityWarning)
}

func TestCheckListeners_ServicePorts(t *testing.T) {
	// VALIDATES: AC-8/10/12/13/14 service listener failures use service-specific doctor codes.
	// PREVENTS: New UDP/TCP runtime dependencies falling back to an unhelpful generic code.
	oldProbe := listenerProbe
	listenerProbe = func(l serviceListener) error {
		if l.code == "doctor-bfd-port" || l.code == "doctor-ipsec-listen" || l.code == "doctor-tftp-listen" || l.code == "doctor-image-listen" || l.code == "doctor-ntp-listen" {
			return errors.New("bind failed")
		}
		return nil
	}
	t.Cleanup(func() { listenerProbe = oldProbe })

	tree := config.NewTree()
	tree.GetOrCreateContainer("bfd")
	vpn := tree.GetOrCreateContainer("vpn")
	vpn.GetOrCreateContainer("ipsec")
	service := tree.GetOrCreateContainer("service")
	tftp := service.GetOrCreateContainer("tftp-server")
	tftp.Set("enabled", "true")
	image := service.GetOrCreateContainer("image-server")
	image.Set("enabled", "true")
	env := tree.GetOrCreateContainer("environment")
	ntp := env.GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")

	diags := checkListeners(tree)
	requireDiag(t, diags, "doctor-bfd-port", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-ipsec-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-tftp-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-image-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-ntp-listen", diagnostic.SeverityWarning)
}

func TestCheckTACACSServers(t *testing.T) {
	// VALIDATES: AC-6 unreachable TACACS+ servers return doctor-tacacs-unreachable.
	// PREVENTS: AAA outages being discovered only after login attempts fail.
	oldProbe := tcpReachable
	tcpReachable = func(string, time.Duration) bool { return false }
	t.Cleanup(func() { tcpReachable = oldProbe })

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	auth := system.GetOrCreateContainer("authentication")
	tacacs := auth.GetOrCreateContainer("tacacs")
	server := config.NewTree()
	server.Set("address", "192.0.2.1")
	server.Set("port", "49")
	tacacs.AddListEntry("server", "192.0.2.1", server)

	diags := checkTACACSServers(tree)
	requireDiag(t, diags, "doctor-tacacs-unreachable", diagnostic.SeverityWarning)
}

func TestCheckRADIUSServers(t *testing.T) {
	// VALIDATES: AC-7 unreachable RADIUS servers return doctor-radius-unreachable.
	// PREVENTS: L2TP authentication failures surfacing only on subscriber login.
	oldProbe := udpReachable
	udpReachable = func(string, []byte, net.IP, string, time.Duration) bool { return false }
	t.Cleanup(func() { udpReachable = oldProbe })

	tree := config.NewTree()
	l2tp := tree.GetOrCreateContainer("l2tp")
	auth := l2tp.GetOrCreateContainer("auth")
	radius := auth.GetOrCreateContainer("radius")
	server := config.NewTree()
	server.Set("address", "radius.example.invalid")
	server.Set("port", "1812")
	server.Set("shared-key", "testing123")
	radius.AddListEntry("server", "primary", server)

	diags := checkRADIUSServers(tree)
	requireDiag(t, diags, "doctor-radius-unreachable", diagnostic.SeverityWarning)
}

func TestUDPServerReachableRequiresResponse(t *testing.T) {
	// VALIDATES: The RADIUS readiness probe requires an authenticated response instead of accepting Dial success.
	// PREVENTS: Unbound UDP ports or bad shared keys being reported as reachable.
	secret := []byte("testing123")
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := pc.LocalAddr().String()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64)
		n, addr, readErr := pc.ReadFrom(buf)
		if readErr != nil || n < 20 {
			return
		}
		var reqAuth [zeradius.AuthenticatorLen]byte
		copy(reqAuth[:], buf[4:4+zeradius.AuthenticatorLen])
		resp := &zeradius.Packet{Code: zeradius.CodeAccessReject, Identifier: buf[1], Authenticator: reqAuth}
		wire := make([]byte, zeradius.MaxPacketLen)
		respLen, encodeErr := resp.EncodeTo(wire, 0)
		if encodeErr != nil {
			return
		}
		respAuth := zeradius.ResponseAuthenticator(resp.Code, resp.Identifier, uint16(respLen), reqAuth, wire[zeradius.HeaderLen:respLen], secret)
		copy(wire[4:4+zeradius.AuthenticatorLen], respAuth[:])
		_, _ = pc.WriteTo(wire[:respLen], addr)
	}()

	assert.True(t, udpServerReachable(addr, secret, nil, "ze-doctor", time.Second))
	_ = pc.Close()
	<-done
	assert.False(t, udpServerReachable(addr, secret, nil, "ze-doctor", 10*time.Millisecond))
}

func TestCheckPKICerts_MissingCA(t *testing.T) {
	// VALIDATES: AC-9 PKI CA entries without certificate material return doctor-pki-cert.
	// PREVENTS: Certificate store gaps being missed until IPsec or TLS uses the CA.
	tree := config.NewTree()
	pki := tree.GetOrCreateContainer("pki")
	pki.AddListEntry("ca", "root", config.NewTree())

	diags := checkPKICerts(tree)
	requireDiag(t, diags, "doctor-pki-cert", diagnostic.SeverityError)
}

func TestCheckPKICerts_ExpiredCA(t *testing.T) {
	// VALIDATES: AC-9 PKI CA certificate validity is checked, not just presence.
	// PREVENTS: Expired embedded CA certificates passing readiness checks.
	tree := config.NewTree()
	pki := tree.GetOrCreateContainer("pki")
	ca := config.NewTree()
	ca.Set("certificate", base64.StdEncoding.EncodeToString(generateTestCertDER(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))))
	pki.AddListEntry("ca", "root", ca)

	diags := checkPKICerts(tree)
	requireDiag(t, diags, "doctor-pki-cert", diagnostic.SeverityError)
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
	certDER := generateTestCertDER(t, notBefore, notAfter)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func generateTestCertDER(t *testing.T, notBefore, notAfter time.Time) []byte {
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
	return certDER
}

func requireDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			assert.Equal(t, severity, diags[i].Severity, "severity for %s", code)
			return
		}
	}
	require.Failf(t, "missing diagnostic", "expected %s in %+v", code, diags)
}

// --- Config reference tests ---

func TestCheckConfigReferences_NoBGP(t *testing.T) {
	tree := config.NewTree()
	diags := checkConfigReferences(tree)
	assert.Empty(t, diags)
}

func TestCheckConfigReferences_NoPolicyNoRefs(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("bgp")
	diags := checkConfigReferences(tree)
	assert.Empty(t, diags)
}

func TestCheckConfigReferences_DanglingGlobalRef(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	filter := bgp.GetOrCreateContainer("filter")
	filter.SetSlice("import", []string{"nonexistent"})

	diags := checkConfigReferences(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-config-reference", diags[0].Code)
	assert.Contains(t, diags[0].Message, "nonexistent")
}

func TestCheckConfigReferences_DefinedPolicyPasses(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")

	policy := bgp.GetOrCreateContainer("policy")
	policy.AddListEntry("prefix-list", "customers", config.NewTree())

	filter := bgp.GetOrCreateContainer("filter")
	filter.SetSlice("import", []string{"customers"})

	diags := checkConfigReferences(tree)
	assert.Empty(t, diags)
}

func TestCheckConfigReferences_NamespacedRefPasses(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")

	policy := bgp.GetOrCreateContainer("policy")
	policy.AddListEntry("prefix-list", "customers", config.NewTree())

	filter := bgp.GetOrCreateContainer("filter")
	filter.SetSlice("import", []string{"bgp-filter-prefix:customers"})

	diags := checkConfigReferences(tree)
	assert.Empty(t, diags)
}

func TestCheckConfigReferences_PeerLevelRef(t *testing.T) {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")

	policy := bgp.GetOrCreateContainer("policy")
	policy.AddListEntry("prefix-list", "allowed", config.NewTree())

	peerTree := config.NewTree()
	peerFilter := peerTree.GetOrCreateContainer("filter")
	peerFilter.SetSlice("export", []string{"missing"})
	bgp.AddListEntry("peer", "192.0.2.1", peerTree)

	diags := checkConfigReferences(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "missing")
	assert.Contains(t, diags[0].Message, "bgp/peer/192.0.2.1/filter/export")
}

// --- Disk space tests ---

func TestCheckDiskSpace_ReturnsNilOnWorkingFilesystem(t *testing.T) {
	diags := checkDiskSpace()
	assert.Empty(t, diags)
}

// --- DNS resolver tests ---

func TestCheckDNSResolvers_NoSystemBlock(t *testing.T) {
	tree := config.NewTree()
	diags := checkDNSResolvers(tree)
	assert.Empty(t, diags)
}

func TestCheckDNSResolvers_NoNameServers(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")
	diags := checkDNSResolvers(tree)
	assert.Empty(t, diags)
}

func TestCheckDNSResolvers_UnreachableServer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network timeout")
	}
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	sys.SetSlice("name-server", []string{"192.0.2.254"})
	diags := checkDNSResolvers(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-dns-resolver", diags[0].Code)
}

func TestDNSServerResponds_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires network timeout")
	}
	assert.False(t, dnsServerResponds("192.0.2.254"))
}

// --- Filter instance name tests ---

func TestFilterInstanceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"customers", "customers"},
		{"prefix-list:customers", "customers"},
		{"bgp-filter-prefix:customers", "customers"},
	}
	for _, tt := range tests {
		got := filterInstanceName(tt.input)
		assert.Equal(t, tt.want, got, "filterInstanceName(%q)", tt.input)
	}
}

// --- Store integrity code registration test ---

func TestDoctorStoreIntegrityCodeRegistered(t *testing.T) {
	meta := diagnostic.Lookup("doctor-store-integrity")
	require.NotNil(t, meta, "doctor-store-integrity code must be registered")
	assert.NotEmpty(t, meta.Title)
}

func TestDoctorCoverageCodesRegistered(t *testing.T) {
	// VALIDATES: AC-17 every new doctor coverage diagnostic code is registered for ze explain.
	// PREVENTS: ze doctor emitting codes that ze explain cannot describe.
	for _, code := range []string{
		"doctor-l2tp-module",
		"doctor-pppoe-module",
		"doctor-firewall-nftables",
		"doctor-dhcp-iface",
		"doctor-bgp-listen",
		"doctor-tacacs-unreachable",
		"doctor-radius-unreachable",
		"doctor-bfd-port",
		"doctor-pki-cert",
		"doctor-ipsec-listen",
		"doctor-telemetry-procfs",
		"doctor-tftp-listen",
		"doctor-image-listen",
		"doctor-ntp-listen",
		"doctor-sysctl-procfs",
		"doctor-conntrack-procfs",
		"doctor-policyroute-netlink",
	} {
		meta := diagnostic.Lookup(code)
		require.NotNil(t, meta, "%s code must be registered", code)
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	}
}
