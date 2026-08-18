package process

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// slowReleaseDelay is how long the test plugin spends releasing after its read loop
// ends. It is longer than the 500ms this wait used to run for, and shorter than the
// grace it runs for now, so the test discriminates the change rather than the clock.
const slowReleaseDelay = 700 * time.Millisecond

// registerRunEngine registers one internal plugin whose engine is fn. The name must be
// unique for the life of the process, because the registry refuses a duplicate and has
// no removal.
func registerRunEngine(t *testing.T, name string, fn func(net.Conn) int) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "test plugin for ProcessManager.Stop cleanup waiting",
		RunEngine:   fn,
		CLIHandler:  func(_ []string) int { return 0 },
	})
	require.NoError(t, err)
}

// drainUntilClosed blocks on conn until the far end closes it, which is the shutdown
// signal an internal plugin gets (startInternal closes the engine side in Stop).
func drainUntilClosed(conn net.Conn) {
	buf := make([]byte, 64)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-5 -- ProcessManager.Stop waits for
// a plugin's release to finish, so a resource the plugin took is given back before the
// daemon exits.
// PREVENTS: the release running into a closed door. Closing the connection unblocks a
// plugin's read loop, but an engine releases what it installed AFTER that loop ends: the
// IKE engine deletes eight node-wide XFRM bypass policies there. This wait ran for 500ms
// and discarded its own timeout, so a release slower than that was simply lost, with
// nothing logged. MEASURED 2026-08-17: with the release delayed 700ms, all eight policies
// stayed in the kernel after both daemons had exited
// (test/ipsec/ipsec-teardown-leaves-nothing.ci reported RESIDUE: policies=8), and the same
// test failed the same way in the QEMU VM at the real speed of that release.
func TestStopWaitsForAPluginReleaseThatOutlastsItsReadLoop(t *testing.T) {
	var released atomic.Bool
	registerRunEngine(t, "test-stop-wait-slow-release", func(conn net.Conn) int {
		drainUntilClosed(conn)
		// The release an engine runs on its way out. Deliberately slower than the
		// 500ms this wait used to allow.
		time.Sleep(slowReleaseDelay)
		released.Store(true)
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-wait-slow-release", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())

	pm.Stop()

	assert.True(t, released.Load(),
		"Stop returned before the plugin finished releasing, so whatever it installed is left behind")
}

// VALIDATES: a plugin that does not finish inside the grace is NAMED, not discarded.
// PREVENTS: the silent half of the same defect. The wait's error was thrown away, so a
// lost cleanup left no trace anywhere -- the operator saw a clean "Ze stopped." and a
// kernel still holding what the plugin installed (ai/rules/evidence.md: a guard that goes
// quiet on the failure it exists to catch fails open).
func TestStopNamesThePluginThatMissesItsCleanupGrace(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	registerRunEngine(t, "test-stop-wait-stuck", func(conn net.Conn) int {
		drainUntilClosed(conn)
		<-stuck
		return 0
	})

	var mu sync.Mutex
	var lines []string
	origLogger := logger
	logger = func() *slog.Logger {
		return slog.New(recordHandler{mu: &mu, lines: &lines})
	}
	origGrace := pluginStopGrace
	pluginStopGrace = 50 * time.Millisecond
	t.Cleanup(func() {
		logger = origLogger
		pluginStopGrace = origGrace
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-wait-stuck", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())

	pm.Stop()

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()
	assert.Contains(t, got, "test-stop-wait-stuck",
		"the plugin that outlasted the grace was not named, so a lost cleanup leaves no trace")
	assert.Contains(t, got, "may be left behind")
	assert.Contains(t, got, "engine", "the message must say it is the ENGINE that was waited for")
}

// recordHandler collects the message plus the rendered attributes of every record, which
// is all these assertions need: the plugin name arrives as an attribute.
type recordHandler struct {
	mu    *sync.Mutex
	lines *[]string
}

func (h recordHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature; a pointer receiver for the record is not allowed.
func (h recordHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" ")
		b.WriteString(a.String())
		return true
	})
	h.mu.Lock()
	*h.lines = append(*h.lines, b.String())
	h.mu.Unlock()
	return nil
}

func (h recordHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h recordHandler) WithGroup(_ string) slog.Handler      { return h }
