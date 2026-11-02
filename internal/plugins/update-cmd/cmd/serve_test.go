package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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

// updateServeFrame is the frame of the one goroutine an update server under
// the daemon runs.
const updateServeFrame = "(*updateServeListener).serve"

// updateServeGoroutines counts the goroutines inside updateServeFrame.
func updateServeGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), updateServeFrame)
		}
		buf = make([]byte, 2*len(buf))
	}
}

// TestUpdateServeLifecycle proves the daemon form of `update serve` (AC-16):
// start answers the address it listens on, the server answers /version.json
// there, pause toggles the pause that manifest reports, a second start and a
// pause or stop with no server are refused with the command that fixes it,
// and stop ends the listener and returns only after the server's goroutine
// ended. Method: drive the owner the daemon handlers call, on 127.0.0.1:0,
// with an HTTP client; read the goroutine's end from its done channel at the
// moment stop returns, and count its frame in a full stack dump.
func TestUpdateServeLifecycle(t *testing.T) {
	var owner updateServeOwner
	daemon := testDaemon(t)
	before := updateServeGoroutines()

	started, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
	require.NoError(t, err)
	require.NotEqual(t, "127.0.0.1:0", started.Listen, "start must answer the bound address")
	assert.False(t, started.Paused)

	fetch := func() serveManifest {
		resp := testGet(t, "http://"+started.Listen+"/version.json")
		defer resp.Body.Close() //nolint:errcheck // test cleanup
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var m serveManifest
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		return m
	}
	m := fetch()
	assert.NotEmpty(t, m.Version)
	assert.Len(t, m.SHA256, 64)
	assert.False(t, m.Paused)

	_, err = owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
	require.Error(t, err, "a second start must be refused")
	assert.Contains(t, err.Error(), started.Listen)
	assert.Contains(t, err.Error(), "'update serve stop'")

	paused, err := owner.pause()
	require.NoError(t, err)
	assert.True(t, paused.Paused)
	assert.True(t, fetch().Paused, "pause must reach the manifest")
	resumed, err := owner.pause()
	require.NoError(t, err)
	assert.False(t, resumed.Paused)
	assert.False(t, fetch().Paused, "a second pause must resume")

	running := owner.running
	stopped, err := owner.stop()
	require.NoError(t, err)
	select {
	case <-running.done:
	default:
		t.Fatal("stop returned while the server's goroutine was still running")
	}
	assert.True(t, stopped.Stopped)
	assert.Equal(t, started.Listen, stopped.Listen)

	deadline := time.Now().Add(time.Second)
	for updateServeGoroutines() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	assert.Equal(t, before, updateServeGoroutines(), "no server goroutine may outlive stop")

	var dialer net.Dialer
	conn, dialErr := dialer.DialContext(t.Context(), "tcp", started.Listen)
	if dialErr == nil {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatal("the listener must be closed after stop")
	}

	_, err = owner.stop()
	require.ErrorIs(t, err, errNoUpdateServer)
	_, err = owner.pause()
	require.ErrorIs(t, err, errNoUpdateServer)
	assert.Contains(t, errNoUpdateServer.Error(), "'update serve start'")

	restarted, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
	require.NoError(t, err, "a stopped server must leave room for a new start")
	_, err = owner.stop()
	require.NoError(t, err)
	assert.NotEmpty(t, restarted.Listen)
}

// testDaemon answers a plugin server standing in for the daemon's, whose Stop
// is the call the daemon's ordered shutdown makes (cmd/ze/hub, apiServer.Stop).
func testDaemon(t *testing.T) *pluginserver.Server {
	t.Helper()
	daemon, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(daemon.Stop)
	return daemon
}

// assertUpdateServeGone fails unless nothing answers on addr and no server
// goroutine is left beyond before.
func assertUpdateServeGone(t *testing.T, addr string, before int) {
	t.Helper()
	assert.Equal(t, before, updateServeGoroutines(), "no server goroutine may outlive the daemon")
	var dialer net.Dialer
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err == nil {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatal("the listener must be closed once the daemon stopped")
	}
}

// TestUpdateServeEndsWithDaemonStop proves the update server's lifetime is
// bounded by the daemon's: the daemon's Stop ends a running server, closes its
// listener and returns only after the server's goroutine ended; an operator's
// stop first leaves the daemon's Stop nothing to do; and a daemon already
// stopping refuses a new start, so no server outlives it. Method: drive the
// owner with a real plugin server's OnStop, call that server's Stop (the
// daemon's shutdown call), then read the done channel at the moment Stop
// returns, count the goroutine's frame, and dial the address.
func TestUpdateServeEndsWithDaemonStop(t *testing.T) {
	before := updateServeGoroutines()

	t.Run("daemon stop ends a running server", func(t *testing.T) {
		var owner updateServeOwner
		daemon := testDaemon(t)
		started, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
		require.NoError(t, err)
		resp := testGet(t, "http://"+started.Listen+"/version.json")
		resp.Body.Close() //nolint:errcheck // test cleanup
		require.Equal(t, http.StatusOK, resp.StatusCode)

		running := owner.running
		daemon.Stop()
		select {
		case <-running.done:
		default:
			t.Fatal("the daemon's Stop returned while the server's goroutine was still running")
		}
		assertUpdateServeGone(t, started.Listen, before)
		_, err = owner.stop()
		require.ErrorIs(t, err, errNoUpdateServer, "the daemon's stop must clear the running server")
	})

	t.Run("operator stop first leaves the daemon nothing to end", func(t *testing.T) {
		var owner updateServeOwner
		daemon := testDaemon(t)
		started, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
		require.NoError(t, err)
		_, err = owner.stop()
		require.NoError(t, err)
		restarted, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
		require.NoError(t, err)

		daemon.Stop()
		assertUpdateServeGone(t, started.Listen, before)
		assertUpdateServeGone(t, restarted.Listen, before)
	})

	t.Run("a stopping daemon refuses a start", func(t *testing.T) {
		var owner updateServeOwner
		daemon := testDaemon(t)
		daemon.Stop()
		_, err := owner.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the daemon is stopping")
		assert.Nil(t, owner.running, "a refused start must leave no server")
		assert.Equal(t, before, updateServeGoroutines())
	})
}

// TestUpdateServeStartNeedsADaemon proves the daemon handler refuses a start
// no daemon server dispatched, naming both routes that work, rather than
// starting a server nothing would stop.
func TestUpdateServeStartNeedsADaemon(t *testing.T) {
	resp, err := handleUpdateServeStart(&pluginserver.CommandContext{}, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "update serve start")
	assert.Contains(t, resp.Error, "ze update serve --listen")
	assert.Nil(t, updateServer.running)
}

// TestUpdateServeStatus proves `update serve` through the daemon answers the
// status of the daemon's update server as data: not running before a start,
// running with the bound address after it, paused after a pause, and not
// running once stopped. Method: call the daemon handler for the exact path
// around the owner's own start, pause and stop on the daemon's one server,
// and read the Data each answer carries.
func TestUpdateServeStatus(t *testing.T) {
	status := func() updateServeStatus {
		t.Helper()
		resp, err := handleUpdateServe(nil, commandtest.Args())
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		st, ok := resp.Data.(updateServeStatus)
		require.True(t, ok, "update serve must answer its status as data, got %T", resp.Data)
		return st
	}

	require.Nil(t, updateServer.running, "the test needs the daemon's server stopped")
	before := status()
	assert.False(t, before.Running, "before a start no server runs")
	assert.Empty(t, before.Listen)

	daemon := testDaemon(t)
	started, err := updateServer.start(t.Context(), "127.0.0.1:0", daemon.OnStop)
	require.NoError(t, err)
	t.Cleanup(func() {
		updateServer.stop() //nolint:errcheck // the test stops it first; this only covers a failed assertion
	})

	running := status()
	assert.True(t, running.Running, "after start the server runs")
	assert.Equal(t, started.Listen, running.Listen, "status names the bound address")
	assert.False(t, running.Paused)

	_, err = updateServer.pause()
	require.NoError(t, err)
	paused := status()
	assert.True(t, paused.Running)
	assert.True(t, paused.Paused, "status reports the pause")

	_, err = updateServer.stop()
	require.NoError(t, err)
	stopped := status()
	assert.False(t, stopped.Running, "after stop no server runs")
	assert.Empty(t, stopped.Listen)
}
