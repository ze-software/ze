// Design: docs/guide/config-editor.md -- model timer lifetime.

package cli

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
)

func TestModelCloseCancelsCopiedValidationTick(t *testing.T) {
	m := newTestModel(t)
	copy := *m
	cmd := copy.scheduleValidation()
	require.NotNil(t, cmd)
	require.NoError(t, m.Close())
	require.Nil(t, cmd(), "a tick scheduled by a copy must not run after the session closes")
}

func TestModelTickSchedulesBeforeClockStarts(t *testing.T) {
	m := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	probe := &timerProbe{started: make(chan time.Duration, 1), expired: make(chan time.Time, 1), stopped: make(chan struct{})}
	m.dispatch.clock = probe
	cmd := m.scheduleValidation()
	returned := make(chan tea.Msg, 1)
	go func() { returned <- cmd() }()
	select {
	case msg := <-returned:
		require.NotNil(t, msg, "the command must declare timer work")
		_, run := m.Update(msg)
		require.NotNil(t, run, "the originating live model must start the timer")
		other := newTestModel(t)
		t.Cleanup(func() { require.NoError(t, other.Close()) })
		_, run = other.Update(msg)
		require.Nil(t, run, "a different lifetime must refuse the schedule")
		require.NoError(t, m.Close())
		_, run = m.Update(msg)
		require.Nil(t, run, "a closed lifetime must refuse the schedule")
	case <-probe.started:
		require.NoError(t, m.Close())
		<-returned
		t.Fatal("the scheduling command waited for the timer instead of declaring it")
	case <-time.After(5 * time.Second):
		t.Fatal("scheduling command did not return")
	}
}

// cancelAtDelivery places cancellation exactly at the post-receive Err check,
// after select chose a ready timer. It avoids racing two runnable goroutines.
type cancelAtDelivery struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *cancelAtDelivery) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestModelTimerCloseWinsAfterReadyReceive(t *testing.T) {
	m := newTestModel(t)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	probe := &timerProbe{started: make(chan time.Duration, 1), expired: make(chan time.Time, 1), stopped: make(chan struct{})}
	m.dispatch.clock = probe
	request, ok := m.scheduleValidation()().(TimerMsg)
	require.True(t, ok)
	m.dispatch.ctx = &cancelAtDelivery{Context: m.dispatch.ctx, cancel: m.dispatch.cancel}
	probe.expired <- time.Now()
	require.Nil(t, request.Wait(), "cancellation after receive must suppress delivery")
	select {
	case <-probe.stopped:
	default:
		t.Fatal("canceled ready timer was not stopped")
	}
}

// timerProbe controls only this model's timer; authored editor runs keep the
// real clock. The start/stop channels prove ownership without elapsed sleeps.
type timerProbe struct {
	clock.RealClock
	started chan time.Duration
	expired chan time.Time
	stopped chan struct{}
}

func (p *timerProbe) NewTimer(delay time.Duration) clock.Timer {
	p.started <- delay
	return &probeTimer{probe: p}
}

type probeTimer struct {
	probe *timerProbe
}

func (t *probeTimer) C() <-chan time.Time { return t.probe.expired }
func (t *probeTimer) Stop() bool {
	close(t.probe.stopped)
	return true
}
func (t *probeTimer) Reset(time.Duration) bool { return false }

func TestModelRunningTimerOwnership(t *testing.T) {
	for _, action := range []string{"cancel", "expire", "expire-then-close", "stale-generation"} {
		t.Run(action, func(t *testing.T) {
			m := newTestModel(t)
			t.Cleanup(func() { require.NoError(t, m.Close()) })
			probe := &timerProbe{
				started: make(chan time.Duration, 1),
				expired: make(chan time.Time, 1),
				stopped: make(chan struct{}),
			}
			m.dispatch.clock = probe
			copy := *m
			cmd := copy.scheduleValidation()
			m.validationID = copy.validationID
			m.editor.setWorkingContent("bgp { router-id invalid; }")
			result := make(chan tea.Msg, 1)
			request, ok := cmd().(TimerMsg)
			require.True(t, ok)
			go func() { result <- request.Wait() }()
			select {
			case delay := <-probe.started:
				require.Equal(t, validationDebounce, delay)
			case <-time.After(5 * time.Second):
				t.Fatal("timer did not start")
			}
			if action == "cancel" {
				require.NoError(t, m.Close())
			} else {
				probe.expired <- time.Now()
			}
			var msg tea.Msg
			select {
			case msg = <-result:
			case <-time.After(5 * time.Second):
				t.Fatal("timer command did not return")
			}
			select {
			case <-probe.stopped:
			default:
				t.Fatal("timer was not stopped before command return")
			}
			if action == "cancel" {
				require.Nil(t, msg)
				return
			}
			require.NotNil(t, msg)
			if action == "expire-then-close" {
				require.NoError(t, m.Close())
			}
			if action == "stale-generation" {
				m.scheduleValidation()
			}
			next, _ := m.Update(msg)
			after, ok := next.(Model)
			require.True(t, ok, "Update answered %T, want Model", next)
			if action == "expire" {
				require.NotEmpty(t, after.ValidationErrors(), "current live expiry must validate")
			} else {
				require.Empty(t, after.ValidationErrors(), "closed or superseded expiry must not validate")
			}
		})
	}
}

func TestModelTickCannotValidateAnotherLifetime(t *testing.T) {
	old := newTestModel(t)
	cmd := old.scheduleValidation()
	request, ok := cmd().(TimerMsg)
	require.True(t, ok)
	msg := request.Wait()
	require.NotNil(t, msg, "real expiry must still produce a message")
	require.NoError(t, old.Close())

	restarted := newTestModel(t)
	defer restarted.Close() //nolint:errcheck // test cleanup
	// Both sessions start at the same validation generation. The lifetime,
	// rather than the generation number alone, must reject the old message.
	restarted.scheduleValidation()
	restarted.editor.setWorkingContent("bgp { router-id invalid; }")
	before := restarted.ValidationErrors()
	next, _ := restarted.Update(msg)
	after, ok := next.(Model)
	require.True(t, ok)
	require.Equal(t, before, after.ValidationErrors(), "a previous session's expiry must not validate this session")
}

func TestModelCloseCancelsPollerContexts(t *testing.T) {
	for _, view := range []string{"ping", "ping-piped", "traceroute", "traceroute-piped", "monitor", "provider"} {
		t.Run(view, func(t *testing.T) {
			m := NewCommandModel(FilesystemAuthorityOperatorLocal)
			t.Cleanup(func() { require.NoError(t, m.Close()) })
			var received context.Context
			capture := func(ctx context.Context) context.CancelFunc {
				child, cancel := context.WithCancel(ctx)
				received = child
				t.Cleanup(cancel)
				return cancel
			}
			m.SetPingFactory(func(ctx context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
				return make(chan map[string]any), capture(ctx), nil
			})
			m.setTracerouteFactory(func(ctx context.Context, _ string, _ int) (<-chan map[string]any, context.CancelFunc, error) {
				return make(chan map[string]any), capture(ctx), nil
			})
			m.SetMonitorFactory(func(ctx context.Context, _ []string) (*MonitorSession, error) {
				return &MonitorSession{Cancel: capture(ctx)}, nil
			})
			var cmd tea.Cmd
			switch view {
			case "ping":
				cmd = m.startPingMonitor("monitor ping 192.0.2.1")
			case "ping-piped":
				cmd = m.startPingMonitorPiped("monitor ping 192.0.2.1 | log")
			case "traceroute":
				cmd = m.startTraceroute("monitor traceroute 192.0.2.1")
			case "traceroute-piped":
				cmd = m.startTraceroutePiped("monitor traceroute 192.0.2.1 | log")
			case "monitor":
				cmd = m.startMonitorSessionFromInput(nil, "lifetime-test")
			case "provider":
				cmd = m.startProviderMonitor(&pluginserver.MonitorProvider{
					CreateFn: func(ctx context.Context, _ []string) (<-chan string, func(int, int) string, func(), error) {
						return nil, nil, capture(ctx), nil
					},
				}, nil)
			}
			require.NotNil(t, cmd)
			require.NotNil(t, received)
			require.NoError(t, received.Err())
			copy := m
			require.NoError(t, copy.Close())
			require.ErrorIs(t, received.Err(), context.Canceled)
			require.Nil(t, cmd(), "closed poller must not schedule its next tick")
		})
	}
}
