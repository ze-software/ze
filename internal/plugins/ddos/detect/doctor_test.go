package detect

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func enabledDetectTree() *config.Tree {
	tree := config.NewTree()
	dd := tree.GetOrCreateContainer("ddos").GetOrCreateContainer("detect")
	dd.Set("enabled", "true")
	return tree
}

func TestCheckFlowSourceWarnsWhenNoSource(t *testing.T) {
	// VALIDATES: D-6 -- detector enabled + characterization on + no flow source
	// -> doctor-ddos-detect-no-flow-source.
	diags := checkFlowSource(registry.DoctorCheckContext{Tree: enabledDetectTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-ddos-detect-no-flow-source", diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
}

func TestCheckFlowSourceSilentWithSource(t *testing.T) {
	// traffic-usage present -> no warning.
	tu := enabledDetectTree()
	tu.GetOrCreateContainer("traffic").GetOrCreateContainer("usage")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: tu}))

	// flow-export present -> no warning.
	fe := enabledDetectTree()
	fe.GetOrCreateContainer("flow-export")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: fe}))
}

func TestCheckFlowSourceSilentWhenDisabled(t *testing.T) {
	// characterize-enable=false -> no warning even with no source.
	off := enabledDetectTree()
	off.GetContainerPath(configRoot).Set("characterize-enable", "false")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: off}))

	// detector not enabled -> no warning.
	notEnabled := config.NewTree()
	notEnabled.GetOrCreateContainer("ddos").GetOrCreateContainer("detect").Set("enabled", "false")
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: notEnabled}))

	// ddos-detect absent, and nil tree -> no warning.
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: config.NewTree()}))
	assert.Empty(t, checkFlowSource(registry.DoctorCheckContext{Tree: nil}))
}

func TestDetectDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: ddos-detect registers the flow-source doctor check so `ze doctor`
	// runs it and removing ddos-detect removes the check.
	found := false
	for _, c := range registry.PluginDoctorChecks() {
		if c.Name == "ddos-detect-flow-source" {
			found = true
			break
		}
	}
	assert.True(t, found, "doctor check ddos-detect-flow-source not registered via Registration.DoctorChecks")
}

// blindTestClock is a settable clock for the rate feed watch.
type blindTestClock struct{ t time.Time }

func (c *blindTestClock) now() time.Time { return c.t }

// newBlindTestDetector builds an enabled detector whose window is
// baselineWindow*checkInterval feed samples, armed in-process on the
// trafficstat feed under a test clock.
func newBlindTestDetector(t *testing.T, baselineWindow, checkInterval int) (*detector, *blindTestClock) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.BaselineWindow = baselineWindow
	cfg.CheckInterval = checkInterval
	cfg.StartupGrace = 0
	d := newDetector(cfg, newDTestBus(), nil)
	clock := &blindTestClock{t: time.Unix(1_000_000, 0)}
	d.feed.now = clock.now
	d.arm(feedTrafficstat, true)

	return d, clock
}

// feedSamples delivers n one-second samples of the given entries.
func feedSamples(d *detector, clock *blindTestClock, n int, entries []trafficstat.InterfaceEntry) {
	for range n {
		clock.t = clock.t.Add(time.Second)
		d.emitPending(d.tickRates(entries))
	}
}

var idleEth0 = []trafficstat.InterfaceEntry{{Name: "eth0"}}

// captureDetectLog swaps the package logger for one writing into a buffer,
// restoring the previous one when the test ends.
func captureDetectLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := loggerPtr.Load()
	var buf bytes.Buffer
	loggerPtr.Store(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { loggerPtr.Store(prev) })
	return &buf
}

func TestRateFeedZeroTrafficReportedAtWindow(t *testing.T) {
	// VALIDATES: an armed detector whose feed carries an interface with zero
	// traffic for a whole window is reported, and one sample short is not.
	// Method: window 10x1; feed 9 zero samples (silent), then a 10th (warned).
	d, clock := newBlindTestDetector(t, 10, 1)
	feedSamples(d, clock, 9, idleEth0)
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags, "one sample short of the window must not warn")

	feedSamples(d, clock, 1, idleEth0)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorDDoSDetectBlind, diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
	assert.Contains(t, diags[0].Message, "zero received packets and bytes")
	assert.Contains(t, diags[0].Message, feedTrafficstat)
	assert.Contains(t, diags[0].Message, "10s")
	assert.Contains(t, diags[0].Message, "ze show interface rate")
}

func TestRateFeedWindowFollowsCheckInterval(t *testing.T) {
	// VALIDATES: the window is baseline-window evaluations of check-interval
	// samples each, and the zero run that fills it is what is reported.
	// Method: window 5x3 is 15 samples; 14 silent, 15 warned as zero traffic.
	// The zero-traffic text matters: with no sample recorded at all, the
	// no-sample finding fires at the same second, so "15s" alone would pass.
	d, clock := newBlindTestDetector(t, 5, 3)
	feedSamples(d, clock, 14, idleEth0)
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags)

	feedSamples(d, clock, 1, idleEth0)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "15s")
	assert.Contains(t, diags[0].Message, "zero received packets and bytes")
}

func TestRateFeedClearsOnTraffic(t *testing.T) {
	// VALIDATES: the finding clears on the first sample with traffic, and the
	// zero run restarts from there. Method: blind, then one busy sample, then
	// a window-1 zero run.
	d, clock := newBlindTestDetector(t, 10, 1)
	feedSamples(d, clock, 10, idleEth0)
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)

	feedSamples(d, clock, 1, []trafficstat.InterfaceEntry{{Name: "eth0", RxPps: 5}})
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags, "traffic must clear the finding")

	feedSamples(d, clock, 9, idleEth0)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags, "the zero run restarts after traffic")

	// Bytes alone are traffic too.
	feedSamples(d, clock, 1, []trafficstat.InterfaceEntry{{Name: "eth0", RxBps: 64}})
	feedSamples(d, clock, 9, idleEth0)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRateFeedNoInterface(t *testing.T) {
	// VALIDATES: a feed that reports no interface for a whole window is reported
	// with the no-interface message and the `ze show interface` command.
	d, clock := newBlindTestDetector(t, 10, 1)
	feedSamples(d, clock, 10, nil)
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorDDoSDetectBlind, diags[0].Code)
	assert.Contains(t, diags[0].Message, "reported no interface")
	assert.Contains(t, diags[0].Message, "ze show interface")
}

func TestRateFeedNoInterfaceOnCounterPoll(t *testing.T) {
	// VALIDATES: the interface counter poll fallback counts only interfaces
	// with stats, so a poll of stat-less interfaces reads as no interface.
	d, clock := newBlindTestDetector(t, 10, 1)
	d.arm(feedIfaceCounters, true)
	for range 10 {
		clock.t = clock.t.Add(time.Second)
		d.emitPending(d.tickInfos([]iface.InterfaceInfo{{Name: "eth0"}}))
	}
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, feedIfaceCounters)
	assert.Contains(t, diags[0].Message, "reported no interface")
}

func TestRateFeedNoSample(t *testing.T) {
	// VALIDATES: a feed that delivers nothing is reported once a whole window
	// has passed since the subscription, and not one second before.
	d, clock := newBlindTestDetector(t, 10, 1)
	clock.t = clock.t.Add(9 * time.Second)
	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	assert.Empty(t, diags)

	clock.t = clock.t.Add(time.Second)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorDDoSDetectBlind, diags[0].Code)
	assert.Contains(t, diags[0].Message, "received no sample")

	// A feed that stops after delivering traffic is reported the same way.
	feedSamples(d, clock, 1, []trafficstat.InterfaceEntry{{Name: "eth0", RxPps: 5}})
	clock.t = clock.t.Add(10 * time.Second)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "received no sample")
}

func TestRateFeedExternalReportsCause(t *testing.T) {
	// VALIDATES: an external ddos-detect reports that it runs external, with
	// the config block to remove, from the moment it is armed. Its own
	// process's trafficstat service ticks over an empty interface list, and
	// those empty samples must not turn the finding into "no interface",
	// which would send the operator to `ze show interface` on a box whose
	// daemon lists its interfaces fine. Nor do they log a zero-run WARN:
	// warnIfExternal already logged the cause.
	buf := captureDetectLog(t)
	d, clock := newBlindTestDetector(t, 10, 1)
	d.arm(feedTrafficstat, false)

	diags, err := checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorDDoSDetectBlind, diags[0].Code)
	assert.Equal(t, "warning", diags[0].Severity)
	assert.Contains(t, diags[0].Message, "runs as an external plugin process")
	assert.Contains(t, diags[0].Message, "plugin { external ddos-detect { ... } }")

	feedSamples(d, clock, 25, nil)
	diags, err = checkRateFeed(rateFeedCheck, d)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "runs as an external plugin process")
	assert.NotContains(t, diags[0].Message, "reported no interface")
	assert.NotContains(t, buf.String(), "level=WARN", buf.String())
}

func TestRateFeedLogsEachEdgeOnce(t *testing.T) {
	// VALIDATES: the WARN log fires once at the onset of a zero run, not on
	// every later idle sample, and one INFO log marks the recovery.
	buf := captureDetectLog(t)
	d, clock := newBlindTestDetector(t, 10, 1)
	feedSamples(d, clock, 25, idleEth0)
	assert.Equal(t, 1, strings.Count(buf.String(), "level=WARN"), buf.String())
	assert.Contains(t, buf.String(), "zero received packets and bytes")

	feedSamples(d, clock, 3, []trafficstat.InterfaceEntry{{Name: "eth0", RxPps: 5}})
	assert.Equal(t, 1, strings.Count(buf.String(), "shows traffic again"), buf.String())
}

func TestRateFeedDisabledDetectorReportsNothing(t *testing.T) {
	// VALIDATES: a nil detector (ddos detect disabled) is no finding.
	diags, err := checkRateFeed(rateFeedCheck, nil)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRateFeedRefusesUnknownCheckAndUnarmedDetector(t *testing.T) {
	// VALIDATES: an unknown check name and a detector never subscribed are
	// errors, never an empty "healthy" answer.
	d, _ := newBlindTestDetector(t, 10, 1)
	_, err := checkRateFeed("ddos-detect-other", d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), rateFeedCheck)

	unarmed := newDetector(DefaultConfig(), newDTestBus(), nil)

	_, err = checkRateFeed(rateFeedCheck, unarmed)
	require.Error(t, err)
}

func TestRateFeedCheckDeclaration(t *testing.T) {
	// VALIDATES: the SDK declaration names the check the callback answers, runs
	// post-config, and lists a code `ze show diagnostic` explains.
	assert.Equal(t, rateFeedCheck, rateFeedCheckDecl.Name)
	assert.Equal(t, rpc.DoctorPhasePostConfig, rateFeedCheckDecl.Phase)
	assert.Equal(t, []string{diagnostic.CodeDoctorDDoSDetectBlind}, rateFeedCheckDecl.Codes)
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup(diagnostic.CodeDoctorDDoSDetectBlind)
	require.NotNil(t, meta)
	assert.Contains(t, meta.Description, "ze show interface rate")
}
