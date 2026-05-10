package appliance

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

func setupPushTestAppliance(t *testing.T, name, serverAddr string, serverPort int, certPEM []byte) string {
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/update" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
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

	// Self-signed cert (won't verify, but command fails before TLS anyway)
	os.WriteFile(filepath.Join(tlsDir, "cert.pem"), []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"), 0o600) //nolint:errcheck // test
	os.WriteFile(filepath.Join(appDir, "ze-20260510-120000.img"), []byte("img"), 0o644)                                              //nolint:errcheck,gosec // test

	env.ResetCache()
	code := runPush([]string{"unreachable"})
	if code != exitError {
		t.Errorf("push should fail for unreachable device, got %d", code)
	}
}

func TestPushWrongToken(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/update" {
			body, _ := io.ReadAll(r.Body)
			receivedBody = body
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
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

func TestPushAllParallel(t *testing.T) {
	received := make(map[string]bool)
	var mu sync.Mutex

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/update" {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			received[string(body)] = true
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for i := range 8 {
		name := fmt.Sprintf("dev%d", i)
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/update" {
			mu.Lock()
			callCount++
			c := callCount
			mu.Unlock()
			if c == 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for i := range 4 {
		name := fmt.Sprintf("dev%d", i)
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

	env.ResetCache()
	code := runPush([]string{"--all", "--parallel", "4"})
	if code != exitError {
		t.Errorf("push --all with partial failure should return error, got %d", code)
	}
}

func TestPushAllParallelDefault(t *testing.T) {
	var received []string
	var mu sync.Mutex

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/update" {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			received = append(received, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for _, name := range []string{"dev1", "dev2"} {
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

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/update" {
			body, _ := io.ReadAll(r.Body)
			received[string(body)] = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	certPEM := extractTestServerCert(srv)
	host, port := testServerHostPort(srv)

	dir := t.TempDir()
	baseDir = dir

	for _, name := range []string{"dev1", "dev2", "dev3"} {
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
