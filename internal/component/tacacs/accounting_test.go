package tacacs

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: AC-8 -- TacacsAccountant.CommandStart returns non-empty task ID.
// PREVENTS: empty task IDs that break START/STOP correlation.
func TestAccountantTaskIDMonotonic(t *testing.T) {
	// Use a nil-server client (calls will fail, but we test the task ID generation).
	client := NewTacacsClient(TacacsClientConfig{})
	acct := NewTacacsAccountant(client, nil)

	id1 := acct.CommandStart("admin", "10.0.0.1:12345", "show version")
	id2 := acct.CommandStart("admin", "10.0.0.1:12345", "show peers")

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "task IDs must be unique")
}

// VALIDATES: AC-8 -- TacacsAccountant implements the AccountingHook interface shape.
// PREVENTS: interface drift between tacacs and pluginserver packages.
func TestAccountantCommandStartStop(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{})
	acct := NewTacacsAccountant(client, nil)

	// CommandStart/Stop should not panic even with no servers configured.
	// The goroutines will fail silently (logged, never blocks).
	taskID := acct.CommandStart("user", "192.168.1.1:5000", "peer show")
	assert.NotEmpty(t, taskID)

	// CommandStop should not panic.
	acct.CommandStop(taskID, "user", "192.168.1.1:5000", "peer show")
}

// VALIDATES: enqueue after Stop returns false instead of panicking.
// PREVENTS: send-on-closed-channel panic in the dispatcher goroutine when
// a command arrives during reactor shutdown (review BLOCKER 1).
func TestAccountantEnqueueAfterStopNoPanic(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{})
	acct := NewTacacsAccountant(client, nil)
	acct.Start()
	acct.Stop()

	// Must not panic. Record is dropped silently.
	assert.NotPanics(t, func() {
		_ = acct.CommandStart("admin", "10.0.0.1:1", "show version")
		acct.CommandStop("task-1", "admin", "10.0.0.1:1", "show version")
	})
}

// VALIDATES: Stop is idempotent (safe to call multiple times).
// PREVENTS: double-close panic when Bundle.Close races with explicit Stop.
func TestAccountantStopIdempotent(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{})
	acct := NewTacacsAccountant(client, nil)
	acct.Start()

	assert.NotPanics(t, func() {
		acct.Stop()
		acct.Stop()
		acct.Stop()
	})
}

// VALIDATES: concurrent enqueue + Stop does not panic.
// PREVENTS: race between enqueue's channel send and Stop's close() where
// Load sees stopped=false, Stop sets stopped=true and closes queue, and
// enqueue then sends on the closed channel.
func TestAccountantEnqueueConcurrentStop(t *testing.T) {
	client := NewTacacsClient(TacacsClientConfig{})
	acct := NewTacacsAccountant(client, nil)
	acct.Start()

	var wg sync.WaitGroup
	// Fire many enqueues from a pool of goroutines.
	for range 50 {
		wg.Go(func() {
			for range 20 {
				_ = acct.CommandStart("user", "1.2.3.4", "cmd")
			}
		})
	}
	// Concurrently stop the accountant.
	wg.Go(func() {
		acct.Stop()
	})
	// No panic is the assertion.
	wg.Wait()
}

// VALIDATES: AcctRequest marshaling produces correct START/STOP flag encoding.
// PREVENTS: wrong flag bits sent to TACACS+ server.
func TestAcctRequestStartStopFlags(t *testing.T) {
	start := &AcctRequest{
		Flags:         AcctFlagStart,
		AuthenMethod:  0x06,
		PrivLvl:       1,
		AuthenType:    0x01,
		AuthenService: 0x01,
		User:          "admin",
		Port:          "ssh",
		RemAddr:       "10.0.0.1",
		Args:          []string{"task_id=1", "service=shell", "cmd=show version"},
	}

	data, err := start.MarshalBinary()
	assert.NoError(t, err)
	assert.Equal(t, uint8(AcctFlagStart), data[0], "first byte must be START flag")

	stop := &AcctRequest{
		Flags:         AcctFlagStop,
		AuthenMethod:  0x06,
		PrivLvl:       1,
		AuthenType:    0x01,
		AuthenService: 0x01,
		User:          "admin",
		Port:          "ssh",
		RemAddr:       "10.0.0.1",
		Args:          []string{"task_id=1", "service=shell", "cmd=show version"},
	}

	data, err = stop.MarshalBinary()
	assert.NoError(t, err)
	assert.Equal(t, uint8(AcctFlagStop), data[0], "first byte must be STOP flag")
}
