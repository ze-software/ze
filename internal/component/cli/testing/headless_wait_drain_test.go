// Design: docs/architecture/testing/ci-format.md -- finite declared-timer barriers.

package testing

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
)

// TestETWaitTimerDrainAppliesRealPing drives the authored surface with the real
// first poll; no elapsed-time threshold substitutes for its delivery.
func TestETWaitTimerDrainAppliesRealPing(t *testing.T) {
	result := RunETContentIn(t.Context(), "option=mode:value=operational\noption=monitor:ping=fake\n"+
		"input=type:text=monitor ping 192.0.2.1 | log\ninput=enter\nwait=timer:drain\n"+
		"expect=viewport:contains=seq=0\nexpect=viewport:contains=seq=2\n", t.TempDir())
	require.True(t, result.Passed, "%s", result.Error)
}

func TestETWaitTimerDrainRejectsInactive(t *testing.T) {
	result := RunETContentIn(t.Context(), "option=mode:value=command\nwait=timer:drain\n", t.TempDir())
	require.False(t, result.Passed)
	require.Contains(t, result.Error, "pending timer at entry")
}

// TestHeadlessTimerDrainAppliesOnlyEntryTimers proves already-ready deliveries
// and their state chains apply, while a newly rearmed timer stays pending.
func TestHeadlessTimerDrainAppliesOnlyEntryTimers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	first := make(chan tea.Msg, 1)
	second := make(chan tea.Msg, 1)
	rearmed := make(chan tea.Msg, 1)
	hm := &headlessModel{
		model:    cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal),
		pending:  []pendingTick{{result: first}, {result: second}},
		canceled: ctx.Done(),
	}
	t.Cleanup(func() {
		rearmed <- nil
		require.NoError(t, hm.close())
	})
	var applied []int
	first <- tea.BatchMsg{func() tea.Msg {
		applied = append(applied, 1)
		hm.pending = append(hm.pending, pendingTick{result: rearmed})
		return tea.BatchMsg{func() tea.Msg { applied = append(applied, 2); return nil }}
	}}
	second <- tea.BatchMsg{func() tea.Msg { applied = append(applied, 3); return nil }}
	require.NoError(t, hm.wait(WaitAction{Values: map[string]string{etTimer: waitTimerDrain}}))
	require.Equal(t, []int{1, 2, 3}, applied)
	require.Len(t, hm.pending, 1)
	require.True(t, hm.pending[0].result == rearmed, "the rearmed timer must remain pending")
}

// TestHeadlessTimerDrainCancellationKeepsUnreadTimers synchronizes cancellation
// after the first delivery, then proves cleanup can still join the unread one.
func TestHeadlessTimerDrainCancellationKeepsUnreadTimers(t *testing.T) {
	for _, name := range []string{"caller", "model"} {
		t.Run(name, func(t *testing.T) {
			first := make(chan tea.Msg, 1)
			unread := make(chan tea.Msg, 1)
			started := make(chan struct{})
			canceled := make(chan struct{})
			hm := &headlessModel{
				model:   cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal),
				pending: []pendingTick{{result: first}, {result: unread}},
				stopped: make(chan struct{}), canceled: canceled,
			}
			t.Cleanup(func() {
				unread <- nil
				require.NoError(t, hm.close())
			})
			first <- tea.BatchMsg{func() tea.Msg { close(started); return nil }}
			result := make(chan error, 1)
			go func() { result <- hm.wait(WaitAction{Values: map[string]string{etTimer: waitTimerDrain}}) }()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("timer drain did not apply the first delivery")
			}
			if name == "model" {
				hm.stopOnce.Do(func() { close(hm.stopped) })
			} else {
				close(canceled)
			}
			select {
			case err := <-result:
				require.ErrorContains(t, err, "canceled")
			case <-time.After(10 * time.Second):
				t.Fatal("timer drain ignored cancellation")
			}
			require.Len(t, hm.pending, 1)
			require.True(t, hm.pending[0].result == unread, "the unread timer must remain available for cleanup")
		})
	}
}

func TestHeadlessTimerDrainRejectsCanceledDelivery(t *testing.T) {
	result := make(chan tea.Msg, 1)
	result <- nil
	hm := &headlessModel{
		model:   cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal),
		pending: []pendingTick{{result: result}},
	}
	t.Cleanup(func() { require.NoError(t, hm.close()) })
	require.ErrorContains(t, hm.wait(WaitAction{Values: map[string]string{etTimer: waitTimerDrain}}), "canceled")
	require.Empty(t, hm.pending)
}
