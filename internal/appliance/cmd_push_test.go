// Design: docs/architecture/appliance/ota-push.md -- push tests with updater protocol

package appliance

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// updaterMux builds an HTTP handler that speaks the gokrazy updater protocol.
// It handles: GET /update/features, PUT /update/root, POST /update/switch,
// POST /update/testboot, POST /reboot.
func updaterMux(t *testing.T, opts *mockOpts) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("updatehash")) //nolint:errcheck // test
	})

	mux.HandleFunc("PUT /update/root", func(w http.ResponseWriter, r *http.Request) {
		if opts != nil && opts.authCheck != nil {
			if !opts.authCheck(r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		body, _ := io.ReadAll(r.Body)
		if opts != nil && opts.onBody != nil {
			opts.onBody(body)
		}

		h := crc32.NewIEEE()
		h.Write(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(hex.EncodeToString(h.Sum(nil)))) //nolint:errcheck // test
	})

	mux.HandleFunc("POST /update/switch", func(w http.ResponseWriter, _ *http.Request) {
		if opts != nil && opts.onSwitch != nil {
			opts.onSwitch()
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /update/testboot", func(w http.ResponseWriter, _ *http.Request) {
		if opts != nil && opts.onTestboot != nil {
			opts.onTestboot()
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /reboot", func(w http.ResponseWriter, _ *http.Request) {
		if opts != nil && opts.onReboot != nil {
			opts.onReboot()
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

type mockOpts struct {
	authCheck  func(*http.Request) bool
	onBody     func([]byte)
	onSwitch   func()
	onTestboot func()
	onReboot   func()
}

func setupPushTestAppliance(t *testing.T, name, serverAddr string, serverPort int, certPEM []byte) string { //nolint:unparam // test helper
	t.Helper()
	dir := t.TempDir()
	baseDir = dir

	appDir := filepath.Join(dir, name)
	secretsDir := filepath.Join(appDir, "secrets")
	tlsDir := filepath.Join(secretsDir, "tls")
	os.MkdirAll(tlsDir, 0o700) //nolint:errcheck // test

	cfg := DefaultConfig(name)
	cfg.Device.Address = serverAddr
	cfg.Device.UpdatePort = serverPort

	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	token := base64.StdEncoding.EncodeToString([]byte("test-update-token"))
	os.WriteFile(filepath.Join(secretsDir, "update.token"), []byte(token), 0o600) //nolint:errcheck // test

	os.WriteFile(filepath.Join(tlsDir, "cert.pem"), certPEM, 0o600) //nolint:errcheck // test

	imgData := []byte("fake-disk-image-content")
	os.WriteFile(filepath.Join(appDir, "ze-20260510-120000.img"), imgData, 0o644) //nolint:errcheck,gosec // test

	return dir
}

func extractTestServerCert(srv *httptest.Server) []byte {
	cert := srv.TLS.Certificates[0].Certificate[0]
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
}

func testServerHostPort(srv *httptest.Server) (string, int) {
	addr := srv.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		port, _ := strconv.Atoi(parts[1])
		return parts[0], port
	}
	return addr, 443
}

func TestPushSendsImage(t *testing.T) {
	var receivedBody []byte
	var receivedAuth string

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		authCheck: func(r *http.Request) bool {
			receivedAuth = r.Header.Get("Authorization")
			return true
		},
		onBody: func(body []byte) {
			receivedBody = body
		},
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	dir := setupPushTestAppliance(t, "lab", host, port, certPEM)
	baseDir = dir

	env.ResetCache()
	code := runPush([]string{"lab"})
	if code != exitOK {
		t.Fatalf("push returned %d, want 0", code)
	}

	if string(receivedBody) != "fake-disk-image-content" {
		t.Errorf("received body = %q, want fake-disk-image-content", string(receivedBody))
	}

	expectedToken := base64.StdEncoding.EncodeToString([]byte("test-update-token"))
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+expectedToken))
	if receivedAuth != expectedAuth {
		t.Errorf("auth = %q, want %q", receivedAuth, expectedAuth)
	}
}

func TestPushUnreachableDevice(t *testing.T) {
	dir := t.TempDir()
	baseDir = dir

	appDir := filepath.Join(dir, "unreachable")
	secretsDir := filepath.Join(appDir, "secrets")
	tlsDir := filepath.Join(secretsDir, "tls")
	os.MkdirAll(tlsDir, 0o700) //nolint:errcheck // test

	cfg := DefaultConfig("unreachable")
	cfg.Device.Address = "192.0.2.1"
	cfg.Device.UpdatePort = 59999

	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	token := base64.StdEncoding.EncodeToString([]byte("tok"))
	os.WriteFile(filepath.Join(secretsDir, "update.token"), []byte(token), 0o600) //nolint:errcheck // test

	os.WriteFile(filepath.Join(tlsDir, "cert.pem"), []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"), 0o600) //nolint:errcheck // test
	os.WriteFile(filepath.Join(appDir, "ze-20260510-120000.img"), []byte("img"), 0o644)                                              //nolint:errcheck,gosec // test

	env.ResetCache()
	code := runPush([]string{"unreachable"})
	if code != exitError {
		t.Errorf("push should fail for unreachable device, got %d", code)
	}
}

func TestPushWrongToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("updatehash")) //nolint:errcheck // test
	})
	mux.HandleFunc("PUT /update/root", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	dir := setupPushTestAppliance(t, "lab", host, port, certPEM)
	baseDir = dir

	env.ResetCache()
	code := runPush([]string{"lab"})
	if code != exitError {
		t.Errorf("push should fail with 401, got %d", code)
	}
}

func TestPushSpecificImage(t *testing.T) {
	var receivedBody []byte

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onBody: func(body []byte) {
			receivedBody = body
		},
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	dir := setupPushTestAppliance(t, "lab", host, port, certPEM)
	baseDir = dir

	appDir := filepath.Join(dir, "lab")
	os.WriteFile(filepath.Join(appDir, "ze-20260101-000000.img"), []byte("older-image"), 0o644) //nolint:errcheck,gosec // test

	env.ResetCache()
	code := runPush([]string{"--image", "ze-20260101-000000.img", "lab"})
	if code != exitOK {
		t.Fatalf("push returned %d, want 0", code)
	}

	if string(receivedBody) != "older-image" {
		t.Errorf("received body = %q, want older-image", string(receivedBody))
	}
}

func TestResolveImagePathRelativeBaseDir(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	appDir := filepath.Join("appliances", "lab")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir appliance dir: %v", err)
	}
	imagePath := filepath.Join(appDir, "ze-20260101-000000.img")
	if err := os.WriteFile(imagePath, []byte("image"), 0o644); err != nil { //nolint:gosec // test fixture
		t.Fatalf("write image: %v", err)
	}
	resolved, err := resolveImagePath("appliances", "lab", "")
	if err != nil {
		t.Fatalf("resolveImagePath() error = %v", err)
	}
	absImagePath, err := filepath.Abs(imagePath)
	if err != nil {
		t.Fatalf("abs image path: %v", err)
	}
	want, err := filepath.EvalSymlinks(absImagePath)
	if err != nil {
		t.Fatalf("eval image path: %v", err)
	}
	if resolved != want {
		t.Fatalf("resolved image path = %q, want %q", resolved, want)
	}
}
func TestPushAllParallel(t *testing.T) {
	received := make(map[string]bool)
	var mu sync.Mutex

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onBody: func(body []byte) {
			mu.Lock()
			received[string(body)] = true
			mu.Unlock()
		},
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for i := range 8 {
		name := fleetDeviceName(i)
		setupFleetDevice(t, dir, name, host, port, certPEM)
	}

	env.ResetCache()
	code := runPush([]string{"--all", "--parallel", "4"})
	if code != exitOK {
		t.Fatalf("push --all --parallel 4 returned %d, want 0", code)
	}

	if len(received) != 8 {
		t.Errorf("expected 8 devices pushed, got %d", len(received))
	}
}

func TestPushAllParallelPartialFailure(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("GET /update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("updatehash")) //nolint:errcheck // test
	})
	mux.HandleFunc("PUT /update/root", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		callCount++
		c := callCount
		mu.Unlock()
		if c == 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h := crc32.NewIEEE()
		h.Write(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(hex.EncodeToString(h.Sum(nil)))) //nolint:errcheck // test
	})
	mux.HandleFunc("POST /update/switch", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /reboot", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for i := range 4 {
		name := fleetDeviceName(i)
		setupFleetDevice(t, dir, name, host, port, certPEM)
	}

	env.ResetCache()
	code := runPush([]string{"--all", "--parallel", "4"})
	if code != exitError {
		t.Errorf("push --all with partial failure should return error, got %d", code)
	}
}

func TestPushAllParallelDefault(t *testing.T) {
	var received []string
	var mu sync.Mutex

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onBody: func(body []byte) {
			mu.Lock()
			received = append(received, string(body))
			mu.Unlock()
		},
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for _, name := range []string{"dev1", "dev2"} {
		setupFleetDevice(t, dir, name, host, port, certPEM)
	}

	env.ResetCache()
	code := runPush([]string{"--all", "--parallel", "1"})
	if code != exitOK {
		t.Fatalf("push --all --parallel 1 returned %d, want 0", code)
	}

	if len(received) != 2 {
		t.Errorf("expected 2 pushes, got %d", len(received))
	}
}

func TestPushAllIteratesAppliances(t *testing.T) {
	received := make(map[string]bool)

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onBody: func(body []byte) {
			received[string(body)] = true
		},
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for _, name := range []string{"dev1", "dev2", "dev3"} {
		setupFleetDevice(t, dir, name, host, port, certPEM)
	}

	env.ResetCache()
	code := runPush([]string{"--all"})
	if code != exitOK {
		t.Fatalf("push --all returned %d, want 0", code)
	}

	for _, name := range []string{"dev1", "dev2", "dev3"} {
		if !received["img-"+name] {
			t.Errorf("device %s was not pushed to", name)
		}
	}
}

func TestPushTestboot(t *testing.T) {
	testbootCalled := false
	switchCalled := false

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onSwitch:   func() { switchCalled = true },
		onTestboot: func() { testbootCalled = true },
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	setupPushTestAppliance(t, "lab", host, port, certPEM)

	env.ResetCache()
	code := runPush([]string{"--testboot", "lab"})
	if code != exitOK {
		t.Fatalf("push --testboot returned %d, want 0", code)
	}

	if !testbootCalled {
		t.Error("testboot endpoint was not called")
	}
	if switchCalled {
		t.Error("switch should not be called when --testboot is used")
	}
}

func TestPushNoReboot(t *testing.T) {
	rebootCalled := false

	srv := httptest.NewTLSServer(updaterMux(t, &mockOpts{
		onReboot: func() { rebootCalled = true },
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	setupPushTestAppliance(t, "lab", host, port, certPEM)

	env.ResetCache()
	code := runPush([]string{"--no-reboot", "lab"})
	if code != exitOK {
		t.Fatalf("push --no-reboot returned %d, want 0", code)
	}

	if rebootCalled {
		t.Error("reboot should not be called when --no-reboot is used")
	}
}

func TestPushHashVerification(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /update/features", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("")) //nolint:errcheck // test (no updatehash -> SHA256)
	})
	mux.HandleFunc("PUT /update/root", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h := sha256.Sum256(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(hex.EncodeToString(h[:]))) //nolint:errcheck // test
	})
	mux.HandleFunc("POST /update/switch", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /reboot", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	setupPushTestAppliance(t, "lab", host, port, certPEM)

	env.ResetCache()
	code := runPush([]string{"lab"})
	if code != exitOK {
		t.Fatalf("push with SHA256 hash verification returned %d, want 0", code)
	}
}

func TestAuthTransport(t *testing.T) {
	var authHeaders []string
	var mu sync.Mutex

	mux := http.NewServeMux()
	captureAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			mu.Unlock()
			next(w, r)
		}
	}
	mux.HandleFunc("GET /update/features", captureAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("updatehash")) //nolint:errcheck // test
	}))
	mux.HandleFunc("PUT /update/root", captureAuth(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h := crc32.NewIEEE()
		h.Write(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(hex.EncodeToString(h.Sum(nil)))) //nolint:errcheck // test
	}))
	mux.HandleFunc("POST /update/switch", captureAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mux.HandleFunc("POST /reboot", captureAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)
	setupPushTestAppliance(t, "lab", host, port, certPEM)

	env.ResetCache()
	code := runPush([]string{"lab"})
	if code != exitOK {
		t.Fatalf("push returned %d, want 0", code)
	}

	// Features probe + StreamTo + Switch + Reboot = 4 requests, all with auth
	if len(authHeaders) < 4 {
		t.Errorf("expected at least 4 authenticated requests, got %d", len(authHeaders))
	}
	for i, auth := range authHeaders {
		if auth == "" {
			t.Errorf("request %d had no Authorization header", i)
		}
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthTransportDoesNotMutateRequest(t *testing.T) {
	at := &authTransport{
		base: rtFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") == "" {
				t.Error("cloned request is missing the Authorization header")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}),
		token: "secret",
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://device/update/features", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := at.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() //nolint:errcheck // test

	// RoundTripper contract: the original request must be unmodified.
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("RoundTrip mutated the original request Authorization header: %q", got)
	}
}

// setupFleetDevice creates a test appliance with a device-specific image.
func setupFleetDevice(t *testing.T, dir, name, host string, port int, certPEM []byte) {
	t.Helper()

	appDir := filepath.Join(dir, name)
	secretsDir := filepath.Join(appDir, "secrets")
	tlsDir := filepath.Join(secretsDir, "tls")
	os.MkdirAll(tlsDir, 0o700) //nolint:errcheck // test

	cfg := DefaultConfig(name)
	cfg.Device.Address = host
	cfg.Device.UpdatePort = port
	data, _ := json.MarshalIndent(&cfg, "", "  ")
	os.WriteFile(filepath.Join(appDir, "appliance.json"), append(data, '\n'), 0o644) //nolint:errcheck,gosec // test

	token := base64.StdEncoding.EncodeToString([]byte("tok"))
	os.WriteFile(filepath.Join(secretsDir, "update.token"), []byte(token), 0o600)             //nolint:errcheck // test
	os.WriteFile(filepath.Join(tlsDir, "cert.pem"), certPEM, 0o600)                           //nolint:errcheck // test
	os.WriteFile(filepath.Join(appDir, "ze-20260510-120000.img"), []byte("img-"+name), 0o644) //nolint:errcheck,gosec // test
}

func fleetDeviceName(i int) string {
	var buf []byte
	buf = append(buf, "dev"...)
	buf = strconv.AppendInt(buf, int64(i), 10)
	return string(buf)
}

// servedPair returns the appliance's current certificate and key as a
// tls.Certificate, which is the material the DEVICE serves after its image was
// assembled from those files.
func servedPair(t *testing.T, dir, name string) tls.Certificate {
	t.Helper()

	certPEM, err := os.ReadFile(filepath.Join(dir, name, "secrets", "tls", "cert.pem")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read cert.pem: %v", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, name, "secrets", "tls", "key.pem")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read key.pem: %v", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("the appliance material is not a usable pair: %v", err)
	}
	return pair
}

// startDeviceServer runs the updater protocol behind the material a device
// serves, and points the named appliance's config at it.
func startDeviceServer(t *testing.T, dir, name string, served tls.Certificate) {
	t.Helper()

	srv := httptest.NewUnstartedServer(updaterMux(t, &mockOpts{}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{served}, MinVersion: tls.VersionTLS13}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	host, port := testServerHostPort(srv)
	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Device.Address = host
	cfg.Device.UpdatePort = port
	if saveErr := saveConfig(ConfigPath(dir, name), cfg); saveErr != nil {
		t.Fatalf("save config: %v", saveErr)
	}

	imgPath := filepath.Join(dir, name, "ze-20260510-120000.img")
	if writeErr := os.WriteFile(imgPath, []byte("image"), 0o600); writeErr != nil {
		t.Fatalf("write image: %v", writeErr)
	}
}

// TestPushTrustsAReissuedLeaf covers AC-9 and A-2. The device's certificate was
// reissued after the build host wrote its trust file, so the leaf the device
// now serves appears nowhere in that file. The push still succeeds, because the
// file's anchor is the ISSUER: the certificate authority that signed both
// leaves. A trust file holding a copy of one leaf could not do this.
func TestPushTrustsAReissuedLeaf(t *testing.T) {
	dir := initTestAppliance(t, "reissued", nil)
	baseDir = dir

	certPath := filepath.Join(dir, "reissued", "secrets", "tls", "cert.pem")
	trustFile, err := os.ReadFile(certPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read the trust file: %v", err)
	}

	if code := runReplaceCert([]string{"reissued"}); code != exitOK {
		t.Fatalf("replace-cert returned %d", code)
	}
	served := servedPair(t, dir, "reissued")

	reissued, err := os.ReadFile(certPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read the reissued certificate: %v", err)
	}
	if bytes.Equal(trustFile, reissued) {
		t.Fatal("replace-cert left the certificate unchanged, so this test would prove nothing")
	}

	// The operator still holds the file written at initialization; only the
	// device moved on.
	if writeErr := os.WriteFile(certPath, trustFile, 0o600); writeErr != nil {
		t.Fatalf("restore the trust file: %v", writeErr)
	}

	startDeviceServer(t, dir, "reissued", served)

	env.ResetCache()
	if code := runPush([]string{"reissued"}); code != exitOK {
		t.Fatalf("push returned %d, want 0: the pool holds the issuer, so a reissued leaf must verify", code)
	}
}

// TestPushRefusesALeafFromAnotherAuthority is the other half of AC-9. Trusting
// an issuer must not become trusting anybody: a device presenting a leaf that
// this appliance's root did not sign is refused.
func TestPushRefusesALeafFromAnotherAuthority(t *testing.T) {
	dir := initTestAppliance(t, "anchored", nil)
	baseDir = dir

	// A second appliance has its own certificate authority, so its leaf is
	// signed by a root the first appliance never saw.
	otherCfg := DefaultConfig("stranger")
	otherCfgPath := filepath.Join(dir, "stranger-input.json")
	data, err := json.MarshalIndent(&otherCfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(otherCfgPath, data, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if code := runInit([]string{"--config", otherCfgPath, "stranger"}); code != exitOK {
		t.Fatalf("init of the second appliance returned %d", code)
	}

	startDeviceServer(t, dir, "anchored", servedPair(t, dir, "stranger"))

	env.ResetCache()
	if code := runPush([]string{"anchored"}); code == exitOK {
		t.Fatal("push accepted a device whose leaf this appliance's root did not sign")
	}
}
