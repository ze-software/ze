//go:build !ze_test && !ze_chaos && !ze_perf && !ze_analyze

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestComputeBinaryHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testbin")
	content := []byte("test binary content for hashing")
	require.NoError(t, os.WriteFile(path, content, 0o755))

	got, err := computeBinaryHash(path)
	require.NoError(t, err)

	expected := sha256.Sum256(content)
	assert.Equal(t, hex.EncodeToString(expected[:]), got)
}

func TestComputeBinaryHashMissing(t *testing.T) {
	_, err := computeBinaryHash("/nonexistent/path/binary")
	assert.Error(t, err)
}

func TestUpdateServeEnhancedManifest(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "ze-test")
	content := []byte("fake ze binary for manifest test")
	require.NoError(t, os.WriteFile(binPath, content, 0o755))

	expectedHash, err := computeBinaryHash(binPath)
	require.NoError(t, err)

	info, err := os.Stat(binPath)
	require.NoError(t, err)

	selfArch := runtime.GOOS + "/" + runtime.GOARCH

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqArch := r.Header.Get("X-Ze-Arch")
		if reqArch != "" && reqArch != selfArch {
			http.Error(w, "arch mismatch", http.StatusNotFound)
			return
		}
		m := serveManifest{
			Version: "26.05.20",
			SHA256:  expectedHash,
			Size:    info.Size(),
			Paused:  false,
		}
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(m)
		data = append(data, '\n')
		w.Write(data) //nolint:errcheck // test handler
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp := testGet(t, ts.URL)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var m serveManifest
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	assert.Equal(t, "26.05.20", m.Version)
	assert.Equal(t, expectedHash, m.SHA256)
	assert.Equal(t, info.Size(), m.Size)
	assert.False(t, m.Paused)
}

func TestUpdateServePauseFile(t *testing.T) {
	dir := t.TempDir()

	var (
		pauseMu      sync.RWMutex
		signalPaused bool
	)

	isPaused := func() bool {
		_, statErr := os.Stat(filepath.Join(dir, "update-paused"))
		filePaused := statErr == nil
		pauseMu.RLock()
		sigPaused := signalPaused
		pauseMu.RUnlock()
		return filePaused || sigPaused
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m := serveManifest{
			Version: "26.05.20",
			SHA256:  "abc123",
			Size:    100,
			Paused:  isPaused(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m) //nolint:errcheck // test handler
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	fetchPaused := func() bool {
		resp := testGet(t, ts.URL)
		defer resp.Body.Close() //nolint:errcheck // test cleanup
		var m serveManifest
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		return m.Paused
	}

	assert.False(t, fetchPaused(), "should not be paused initially")

	pauseFile := filepath.Join(dir, "update-paused")
	require.NoError(t, os.WriteFile(pauseFile, nil, 0o644))
	assert.True(t, fetchPaused(), "should be paused with pause file")

	require.NoError(t, os.Remove(pauseFile))
	assert.False(t, fetchPaused(), "should resume after pause file removed")

	pauseMu.Lock()
	signalPaused = true
	pauseMu.Unlock()
	assert.True(t, fetchPaused(), "should be paused via signal flag")
}

func TestUpdateServeChecksumEndpoint(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "ze-test")
	content := []byte("binary content for checksum endpoint")
	require.NoError(t, os.WriteFile(binPath, content, 0o755))

	expectedHash, err := computeBinaryHash(binPath)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(expectedHash + "\n")) //nolint:errcheck // test handler
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp := testGet(t, ts.URL)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), expectedHash)
}

func TestUpdateServeBackwardCompat(t *testing.T) {
	m := serveManifest{
		Version: "26.05.20",
		SHA256:  "abcdef1234567890",
		Size:    42000,
		Paused:  false,
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	ver, ok := raw["version"]
	assert.True(t, ok, "manifest must contain 'version' field for backward compatibility")
	assert.Equal(t, "26.05.20", ver)

	_, hasSHA := raw["sha256"]
	assert.True(t, hasSHA)
	_, hasSize := raw["size"]
	assert.True(t, hasSize)
}

func TestServeManifestPausedOmittedWhenFalse(t *testing.T) {
	m := serveManifest{
		Version: "26.05.20",
		SHA256:  "abc",
		Size:    100,
		Paused:  false,
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasPaused := raw["paused"]
	assert.False(t, hasPaused, "paused should be omitted when false (omitempty)")
}
