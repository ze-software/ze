package cli

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPastedBlockDispatchesWithoutRacingTheEditor covers the shared-editor race.
//
// VALIDATES: two config commands entered before the first answers do not race
// on *Editor, and the earlier command still lands first.
// PREVENTS: torn commit state and colliding backup stamps when an operator
// pastes a block of commands over an SSH session.
//
// Bubble Tea runs every tea.Cmd on its own goroutine (Program.handleCommands).
// Model has a value receiver, so every copy shares one *Editor. Update runs
// serially. The commands it returns do not. Bubble Tea enables no bracketed
// paste, so each pasted newline arrives as its own KeyPressMsg. This test
// builds that sequence.
func TestPastedBlockDispatchesWithoutRacingTheEditor(t *testing.T) {
	pairs := [][2]string{
		{"set bgp router-id 9.9.9.9", "set bgp session asn local 65100"},
		{"set bgp router-id 9.9.9.9", "commit"},
		{"commit", "set bgp router-id 9.9.9.9"},
		{"commit", "commit"},
		{"commit", "discard"},
		{"discard", "commit"},
		{"set bgp router-id 9.9.9.9", "discard"},
		{"rollback 1", "commit"},
	}
	for _, pair := range pairs {
		t.Run(pair[0]+"|"+pair[1], func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.conf")
			require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfigSimplePeer), 0o600))

			ed, err := NewEditor(configPath)
			require.NoError(t, err)
			defer ed.Close() //nolint:errcheck,gosec // test cleanup

			model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
			require.NoError(t, err)
			model.width = 80
			model.height = 24

			// The operator already has a pending edit when the block is pasted,
			// so a "commit" in the block has work to do.
			require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "8.8.8.8"))

			// Enter #1: Update hands the command to bubbletea, which runs it on
			// its own goroutine.
			model.textInput.SetValue(pair[0])
			n1, cmd1 := model.handleEnter()
			m1, ok := n1.(Model)
			require.True(t, ok)
			require.NotNil(t, cmd1)

			// Enter #2 arrives before commandResultMsg #1 was delivered.
			m1.textInput.SetValue(pair[1])
			_, cmd2 := m1.handleEnter()
			require.NotNil(t, cmd2)

			start := make(chan struct{})
			msgs := make([]tea.Msg, 2)
			var wg sync.WaitGroup
			for i, c := range []tea.Cmd{cmd1, cmd2} {
				wg.Add(1)
				go func(i int, c tea.Cmd) {
					defer wg.Done()
					<-start
					msgs[i] = c()
				}(i, c)
			}
			close(start)
			wg.Wait()

			for i, msg := range msgs {
				require.IsType(t, commandResultMsg{}, msg,
					"command %d produced %T", i+1, msg)
			}

			// "set" then "commit" is the block an operator actually pastes: the
			// committed file must carry the edit that was entered before it.
			if pair[0] == "set bgp router-id 9.9.9.9" && pair[1] == "commit" {
				onDisk, readErr := os.ReadFile(configPath)
				require.NoError(t, readErr)
				assert.Contains(t, string(onDisk), "9.9.9.9",
					"commit ran before the set that preceded it")
			}
		})
	}
}

// TestDispatchQueueRunsInReservationOrder covers the ordering guarantee.
//
// VALIDATES: turns run in the order they were reserved, whatever order their
// goroutines start in.
// PREVENTS: a pasted "set ... / commit" block committing a config that is
// missing the set -- a silent loss, with nothing on screen to report it.
func TestDispatchQueueRunsInReservationOrder(t *testing.T) {
	const turns = 4
	q := newDispatchQueue()

	waits := make([]<-chan struct{}, turns)
	dones := make([]func(), turns)
	for i := range turns {
		waits[i], dones[i] = q.reserve()
	}

	var mu sync.Mutex
	var ran []int

	// Start the goroutines in reverse, so only the queue can produce the order.
	var wg sync.WaitGroup
	for i := turns - 1; i >= 0; i-- {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-waits[i]
			mu.Lock()
			ran = append(ran, i)
			mu.Unlock()
			dones[i]()
		}(i)
	}
	wg.Wait()

	assert.Equal(t, []int{0, 1, 2, 3}, ran, "turns must run in reservation order")
}
