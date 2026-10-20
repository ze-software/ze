// Design: docs/architecture/storage-backends.md -- the boot record
// Related: source.go -- the bound handle that carries the record's gate.
// Related: pointer.go -- the readers the pending record answers.

package storage

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// bootRecordWaitMax bounds how long a config write waits for the boot record.
// The record takes milliseconds on an idle disk; on a disk another writer
// saturates each of its syncs can take seconds, and a writer then gets a
// refusal it can retry rather than a session that hangs.
var bootRecordWaitMax = 30 * time.Second

// errBootRecordNotRun is what a waiter learns when the daemon stopped before
// it started the record, so no waiter outlives the handle.
var errBootRecordNotRun = errors.New("the daemon stopped before it recorded its boot configuration")

// BootRecord makes the boot configuration durable in the store after the
// daemon serves, instead of before. Until the record lands, the bytes the daemon
// booted from are the accepted configuration: a read of the bound name answers
// them from memory, and a write to it (anything that takes the config lock)
// waits for the record, at most bootRecordWaitMax.
//
// Lifecycle: DeferBootRecord arms it on the bound handle; Start runs the record
// once in its own goroutine; Wait MUST be called before the handle closes.
// Safe for concurrent use.
type BootRecord struct {
	bound   *configStore // the handle the record gates
	content []byte       // the boot bytes, owned by the record
	done    chan struct{}
	once    sync.Once // closes done
	started sync.Once // runs the record
	err     error     // written once, before done closes
}

// DeferBootRecord arms the boot record on the handle BindConfigSource returned.
// The caller MUST call Start once it serves, and Wait before the handle closes.
func DeferBootRecord(store Storage, content []byte) (*BootRecord, error) {
	bound, ok := store.(*configStore)
	if !ok {
		return nil, errors.New("boot record needs the handle BindConfigSource returned")
	}
	record := &BootRecord{
		bound:   bound,
		content: slices.Clone(content),
		done:    make(chan struct{}),
	}
	bound.boot.Store(record)
	return record, nil
}

// Start runs record once, in its own goroutine, against a view of the bound
// handle the gate does not hold, and opens the gate when it returns. A failure
// is kept: every later write to the bound name returns it.
func (r *BootRecord) Start(record func(Storage) error) {
	view := &configStore{Storage: r.bound.Storage, path: r.bound.path, mode: r.bound.mode}
	r.started.Do(func() { go r.run(view, record) })
}

// run is the record's one lifecycle step.
func (r *BootRecord) run(view Storage, record func(Storage) error) {
	r.finish(record(view))
}

// Wait blocks until the record has returned. A record never started is
// finished with errBootRecordNotRun, so no writer waits for it. MUST be called
// before the store handle closes.
func (r *BootRecord) Wait() {
	r.started.Do(func() { r.finish(errBootRecordNotRun) })
	<-r.done
}

// Done is closed once the record has returned, landed or failed, so a stop can
// tell whether Wait would block. Safe for concurrent use.
func (r *BootRecord) Done() <-chan struct{} {
	return r.done
}

func (r *BootRecord) finish(err error) {
	r.once.Do(func() {
		r.err = err
		close(r.done)
	})
}

// covers reports whether a lock or read of path concerns the bound name. It
// compares the store key each path resolves to (resolveKey), not the base name
// alone, so a namespaced key that merely ends in the same name (meta/.../ze.conf)
// is not mistaken for the configuration, and file/active/<name> is.
func (r *BootRecord) covers(path string) bool {
	if path == "" {
		return false
	}
	return resolveKey(path) == resolveKey(r.bound.path)
}

// await is the write gate: nil once the record landed, its failure if it
// failed, and a refusal naming the cause when the bound expires first.
func (r *BootRecord) await() error {
	timer := time.NewTimer(bootRecordWaitMax)
	defer timer.Stop()
	select {
	case <-r.done:
	case <-timer.C:
		return fmt.Errorf("configuration change refused: the store is still recording the boot configuration of %s after %s, because the disk is slow; retry the change in a moment", r.bound.path, bootRecordWaitMax)
	}
	if errors.Is(r.err, errBootRecordNotRun) {
		return fmt.Errorf("configuration change refused: ze is stopping: %w of %s; retry the change once ze has started again", r.err, r.bound.path)
	}
	if r.err != nil {
		return fmt.Errorf("configuration change refused: the boot configuration of %s was not recorded in the store: %w; run `ze data check`, then restart ze", r.bound.path, r.err)
	}
	return nil
}

// pending answers the boot bytes while the record has not landed. After a
// failed record they stay the answer: they are what the daemon runs.
func (r *BootRecord) pending() ([]byte, bool) {
	select {
	case <-r.done:
		if r.err == nil {
			return nil, false
		}
	default:
	}
	return slices.Clone(r.content), true
}

// pendingBootConfig is the read gate for pointer.go: the boot bytes of the
// bound name while its record has not landed.
func pendingBootConfig(store Storage, path string) ([]byte, bool) {
	bound, ok := store.(*configStore)
	if !ok {
		return nil, false
	}
	record := bound.boot.Load()
	if record == nil {
		return nil, false
	}
	if !record.covers(path) {
		return nil, false
	}
	return record.pending()
}
