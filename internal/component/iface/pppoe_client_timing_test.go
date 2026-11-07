// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- keepalive and reconnect timing tests
// Related: pppoe_client_timing.go -- parse, validation and the redial wait

package iface

import (
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPPPoETiming is the timing a pppoe-client entry gets when it writes no
// keepalive or reconnect leaf. TestPPPoEClientKeepaliveDefaults proves the
// parse yields exactly this value, so fixtures that use it run at the defaults.
var testPPPoETiming = PPPoEClientTiming{
	KeepaliveInterval: 10 * time.Second,
	KeepaliveFailures: 3,
	ReconnectDelayMin: time.Second,
	ReconnectDelayMax: 60 * time.Second,
}

// testPPPoEClientSchema resolves the timing containers from the real schema.
func testPPPoEClientSchema(t *testing.T) pppoeClientSchema {
	t.Helper()
	pppoeSchema, err := loadPPPoEClientSchema()
	require.NoError(t, err)
	return pppoeSchema
}

// pppoeTimingJSON parses one pppoe-client entry carrying the given keepalive
// and reconnect blocks, each a JSON object body or empty for an absent block.
func pppoeTimingJSON(t *testing.T, keepalive, reconnect string) (*ifaceConfig, error) {
	t.Helper()
	blocks := ""
	if keepalive != "" {
		blocks += `"keepalive": {` + keepalive + `},`
	}
	if reconnect != "" {
		blocks += `"reconnect": {` + reconnect + `},`
	}
	return parseIfaceConfig(`{"interface": {"pppoe-client": {"pppoe0": {
		"source-interface": "eth0",` + blocks + `
		"authentication": {"username": "user", "password": "pass"}}}}}`)
}

// TestPPPoEClientKeepaliveDefaults proves AC-1: with no keepalive or reconnect
// leaf, the parse yields the values Ze used before the leaves existed, applied
// from the YANG defaults: 10 s interval, 3 failures, 1 to 60 s redial wait.
func TestPPPoEClientKeepaliveDefaults(t *testing.T) {
	cfg, err := pppoeTimingJSON(t, "", "")
	require.NoError(t, err)
	require.Len(t, cfg.PPPoEClient, 1)
	assert.Equal(t, testPPPoETiming, cfg.PPPoEClient[0].Timing)

	// An empty block still takes every default of its container.
	cfg, err = pppoeTimingJSON(t, `"interval": "4"`, `"delay-max": "30"`)
	require.NoError(t, err)
	assert.Equal(t, PPPoEClientTiming{
		KeepaliveInterval: 4 * time.Second, KeepaliveFailures: 3,
		ReconnectDelayMin: time.Second, ReconnectDelayMax: 30 * time.Second,
	}, cfg.PPPoEClient[0].Timing)

	// The default backoff is unchanged: 1, 2, 4, 8, 16, 32, then 60 s.
	want := []time.Duration{1, 2, 4, 8, 16, 32, 60, 60, 60}
	for i, seconds := range want {
		delay, err := testPPPoETiming.reconnectDelay(i + 1)
		require.NoError(t, err)
		assert.Equal(t, seconds*time.Second, delay, "attempt %d", i+1)
	}
}

// TestPPPoEClientTimingBoundaries walks each numeric leaf: the last valid
// value at both ends is accepted, the first invalid value below and above is
// refused with the leaf named. The YANG range refuses the same values first
// at commit; this is the parse half of the pair.
func TestPPPoEClientTimingBoundaries(t *testing.T) {
	leaves := []struct {
		block, leaf string
		max         int
	}{
		{"keepalive", "interval", 3600},
		{"keepalive", "failures", 255},
		{"reconnect", "delay-min", 3600},
		{"reconnect", "delay-max", 3600},
	}
	for _, l := range leaves {
		t.Run(l.block+"-"+l.leaf, func(t *testing.T) {
			for _, value := range []int{1, l.max} {
				body := `"` + l.leaf + `": "` + strconv.Itoa(value) + `"`
				if l.leaf == "delay-min" {
					// delay-max must not sit below delay-min.
					body += `, "delay-max": "3600"`
				}
				keepalive, reconnect := body, ""
				if l.block == "reconnect" {
					keepalive, reconnect = "", body
				}
				_, err := pppoeTimingJSON(t, keepalive, reconnect)
				require.NoError(t, err, "%s %s %d must be accepted", l.block, l.leaf, value)
			}
			for _, value := range []int{0, l.max + 1} {
				body := `"` + l.leaf + `": "` + strconv.Itoa(value) + `"`
				keepalive, reconnect := body, ""
				if l.block == "reconnect" {
					keepalive, reconnect = "", body
				}
				_, err := pppoeTimingJSON(t, keepalive, reconnect)
				require.Error(t, err, "%s %s %d must be refused", l.block, l.leaf, value)
				assert.Contains(t, err.Error(), l.block+" "+l.leaf)
				assert.Contains(t, err.Error(), "1.."+strconv.Itoa(l.max))
			}
		})
	}
}

// TestPPPoEClientDelayMaxBelowMin proves AC-4: delay-max below delay-min is
// refused with both leaves named and the fix; equal values are accepted.
func TestPPPoEClientDelayMaxBelowMin(t *testing.T) {
	_, err := pppoeTimingJSON(t, "", `"delay-min": "10", "delay-max": "5"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reconnect delay-max 5 is below reconnect delay-min 10")
	assert.Contains(t, err.Error(), "set reconnect delay-max to 10 or more")

	cfg, err := pppoeTimingJSON(t, "", `"delay-min": "10", "delay-max": "10"`)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, cfg.PPPoEClient[0].Timing.ReconnectDelayMax)
}

// TestPPPoEClientTimingValidateRefusesZero proves the second check of the
// pair: a timing built without the config, the zero value included, gets no
// redial wait, so no zero delay reaches the redial loop.
func TestPPPoEClientTimingValidateRefusesZero(t *testing.T) {
	_, err := PPPoEClientTiming{}.reconnectDelay(1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keepalive interval")
	require.NoError(t, testPPPoETiming.Validate())
}

// timingDialer records the config of each Dial and the time it ran. A nil
// session makes every Dial fail, so the client keeps redialing.
type timingDialer struct {
	mu      sync.Mutex
	configs []PPPoEClientConfig
	times   []time.Time
	session *PPPoESession
}

func (d *timingDialer) Dial(cfg PPPoEClientConfig, _ <-chan struct{}, _ *slog.Logger) (PPPoESession, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.configs = append(d.configs, cfg)
	d.times = append(d.times, time.Now())
	if d.session == nil {
		return PPPoESession{}, errors.New("timingDialer: no access concentrator")
	}
	return *d.session, nil
}

func (d *timingDialer) snapshot() ([]PPPoEClientConfig, []time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]PPPoEClientConfig(nil), d.configs...), append([]time.Time(nil), d.times...)
}

// startTimingClients installs d as the dialer and reconciles cfg once.
func startTimingClients(t *testing.T, cfg *ifaceConfig, d PPPoEDialer) map[string]*PPPoEClient {
	t.Helper()
	previous := pppoeDialerVar
	SetPPPoEDialer(d)
	t.Cleanup(func() { pppoeDialerVar = previous })
	active := map[string]*PPPoEClient{}
	reconcilePPPoEClients(cfg, active, nil, slog.Default())
	t.Cleanup(func() {
		for _, c := range active {
			c.Stop()
		}
	})
	return active
}

// TestPPPoEClientKeepaliveFromConfig proves the hand-over of the keepalive
// leaves: `keepalive interval 2; failures 2` reaches the Dialer through parse,
// reconcilePPPoEClients and runSession. The dialer itself is faked here;
// TestPPPoEClientKeepaliveConfigEndToEnd proves the production dialer acts on
// the values.
func TestPPPoEClientKeepaliveFromConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg, err := pppoeTimingJSON(t, `"interval": "2", "failures": "2"`, "")
		require.NoError(t, err)
		d := &timingDialer{}
		startTimingClients(t, cfg, d)
		synctest.Wait()
		configs, _ := d.snapshot()
		require.NotEmpty(t, configs)
		assert.Equal(t, 2*time.Second, configs[0].Timing.KeepaliveInterval)
		assert.Equal(t, 2, configs[0].Timing.KeepaliveFailures)
	})
}

// TestPPPoEClientReconnectDelayFromConfig proves AC-3 through the run loop:
// with `reconnect delay-min 5; delay-max 20` and every dial failing, the
// client waits 5, 10, 20 and 20 s between dials. synctest's clock makes the
// gaps exact, so an off-by-one doubling or an ignored cap is red.
func TestPPPoEClientReconnectDelayFromConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg, err := pppoeTimingJSON(t, "", `"delay-min": "5", "delay-max": "20"`)
		require.NoError(t, err)
		d := &timingDialer{}
		startTimingClients(t, cfg, d)
		// sleep(test-clock): run the redial loop through four waits.
		time.Sleep(56 * time.Second)
		synctest.Wait()
		_, times := d.snapshot()
		require.Len(t, times, 5)
		want := []time.Duration{5, 10, 20, 20}
		for i, seconds := range want {
			assert.Equal(t, seconds*time.Second, times[i+1].Sub(times[i]), "wait %d", i+1)
		}
	})
}

// TestPPPoEClientReloadKeepsSessionOnTimerChange proves AC-6 and AC-7 at the
// reconciler: a reload that changes only keepalive or reconnect leaves keeps
// the running client and its session (one Dial) and stores the new timing for
// the next session; a reload that changes any other leaf restarts the client.
func TestPPPoEClientReloadKeepsSessionOnTimerChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		done := make(chan struct{})
		t.Cleanup(func() { close(done) })
		session := pppoeTestSession(done)
		d := &timingDialer{session: &session}
		first, err := pppoeTimingJSON(t, `"failures": "2"`, "")
		require.NoError(t, err)
		active := startTimingClients(t, first, d)
		synctest.Wait()
		running := active["pppoe0"]
		require.NotNil(t, running)

		timers, err := pppoeTimingJSON(t, `"failures": "3", "interval": "7"`, `"delay-max": "9"`)
		require.NoError(t, err)
		reconcilePPPoEClients(timers, active, nil, slog.Default())
		synctest.Wait()
		assert.Same(t, running, active["pppoe0"], "a timer-only reload must keep the client")
		configs, _ := d.snapshot()
		assert.Len(t, configs, 1, "a timer-only reload must not redial")
		running.mu.Lock()
		stored := running.cfg.Timing
		running.mu.Unlock()
		assert.Equal(t, timers.PPPoEClient[0].Timing, stored)

		other := *timers
		other.PPPoEClient = append([]pppoeClientEntry(nil), timers.PPPoEClient...)
		other.PPPoEClient[0].MTU = 1400
		reconcilePPPoEClients(&other, active, nil, slog.Default())
		synctest.Wait()
		assert.NotSame(t, running, active["pppoe0"], "any other change must restart the client")
		configs, _ = d.snapshot()
		require.Len(t, configs, 2)
		assert.Equal(t, timers.PPPoEClient[0].Timing, configs[1].Timing)
	})
}
