// Design: docs/architecture/testing/ci-format.md -- editor command barriers.

package testing

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
)

func TestHeadlessNestedCommandsSettle(t *testing.T) {
	hm := &headlessModel{model: cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal)}
	t.Cleanup(func() { require.NoError(t, hm.model.Close()) })
	var applied []int
	var command func(int) tea.Cmd
	command = func(depth int) tea.Cmd {
		return func() tea.Msg {
			applied = append(applied, depth)
			if depth == 12 {
				return nil
			}
			return tea.BatchMsg{command(depth + 1)}
		}
	}
	hm.processCmd(command(0))
	hm.settleWait()
	require.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, applied)
}

func TestHeadlessSequenceAppliesNestedBatchBeforeNextCommand(t *testing.T) {
	hm := &headlessModel{model: cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal)}
	t.Cleanup(func() { require.NoError(t, hm.model.Close()) })
	var applied []int
	step := func(n int) tea.Cmd {
		return func() tea.Msg { applied = append(applied, n); return nil }
	}
	hm.processCmd(tea.Sequence(
		tea.Batch(step(1), step(2)),
		func() tea.Msg {
			return tea.BatchMsg{tea.Sequence(step(3), step(4))}
		},
		step(5),
	))
	hm.settleWait()
	require.Equal(t, []int{1, 2, 3, 4, 5}, applied)
}

func TestHeadlessDurationWaitCancellation(t *testing.T) {
	hm := &headlessModel{
		model:   cli.NewCommandModel(cli.FilesystemAuthorityOperatorLocal),
		stopped: make(chan struct{}),
		ready:   make(chan struct{}),
	}
	t.Cleanup(func() { require.NoError(t, hm.close()) })
	result := make(chan error, 1)
	go func() { result <- hm.wait(WaitAction{Values: map[string]string{"ms": "3600000"}}) }()
	// The unbuffered notification proves the wait reached its blocking select.
	select {
	case hm.ready <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("duration wait did not start")
	}
	hm.stopOnce.Do(func() { close(hm.stopped) })
	select {
	case err := <-result:
		require.ErrorContains(t, err, "canceled")
	case <-time.After(5 * time.Second):
		t.Fatal("duration wait ignored cancellation")
	}
}
