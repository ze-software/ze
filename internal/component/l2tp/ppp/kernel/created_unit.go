// Design: docs/research/l2tpv2-ze-integration.md -- original PPP creation custody

package kernel

import (
	"errors"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// UnitCreator borrows the backend that owns its original creation socket.
// The backend owner MUST retain it until all created units and reads stop.
// Construct it with NewUnitCreator before allocating a transport or channel.
type UnitCreator struct {
	create func(string) (*CreatedUnit, error)
}

// Create opens the original /dev/ppp file and creates through the backend's
// ordered socket. An empty name selects the native pppN name. On EVERY outcome,
// a nonnil result owns its file: the caller MUST close it, including on error.
// Failed Close retains that owner for reporting; a returned error never grants
// permission to delete a link found by name or index.
func (creator *UnitCreator) Create(name string) (*CreatedUnit, error) {
	if creator == nil || creator.create == nil {
		return nil, errors.New("ppp: missing original unit creator")
	}
	return creator.create(name)
}

// CreatedUnit retains the actual creation echo, the original unit file, and
// the original socket's generation-bound reader. Only UnitCreator constructs
// one. Its zero value grants no authority. Identity and UnitNumber are labels;
// neither can establish a new reader. The file MUST NOT be closed by raw FD.
// Safe for concurrent Counters and Close. This owner borrows its backend.
type CreatedUnit struct {
	mu                sync.Mutex
	closeMu           sync.Mutex
	closed            bool
	closeErr          error
	retirement        nativePPPUnitRetirement
	ready             bool
	attempted         bool
	file              *os.File
	self              *CreatedUnit
	backend           InterfaceOperations
	namespace         *capturedSessionNamespace
	sessionClaim      *claimedCreatedUnit
	creationRejection nativeLinkCreationRejection
	identity          ppp.SessionIdentity
	number            int
	readCounters      func() (ppp.SessionCounters, error)
	poll              func(*os.File) error
}

func (unit *CreatedUnit) Identity() ppp.SessionIdentity { return unit.identity }
func (unit *CreatedUnit) UnitNumber() int               { return unit.number }

// Attempted distinguishes an unsubmitted refusal from a possibly created unit.
// A partial echo remains available through Identity even on creation failure.
func (unit *CreatedUnit) Attempted() bool { return unit.attempted }

// WithDescriptor lends the original file for one short native operation.
// The operation MUST NOT close, retain, duplicate or publish the descriptor,
// or wait for network I/O. This lease prevents descriptor-number reuse while
// the operation runs. It is also usable to inspect an uncertain creation.
// WaitNativePPPUnitRetired MUST join the completion of all such short borrows.
func (unit *CreatedUnit) WithDescriptor(operation func(int) error) error {
	if unit == nil {
		return errors.New("ppp: missing original unit descriptor operation")
	}
	if operation == nil {
		return errors.New("ppp: missing original unit descriptor operation")
	}
	if unit.self != unit {
		return errors.New("ppp: copied or unminted unit cannot lend its original descriptor")
	}
	return unit.withOriginalUnitDescriptor(operation)
}

// Counters takes only short original-file leases for liveness checks. It MUST
// NOT hold mu or a unit descriptor lease across the native counter query: Close
// can end native custody while a reply is delayed. The final check discards
// that reply. The reader itself refuses lost history, movement and index reuse.
func (unit *CreatedUnit) Counters() (ppp.SessionCounters, error) {
	if unit == nil {
		return ppp.SessionCounters{}, errors.New("ppp: no original created unit")
	}
	unit.mu.Lock()
	if unit.closed {
		unit.mu.Unlock()
		return ppp.SessionCounters{}, os.ErrClosed
	}
	if !unit.ready || unit.file == nil || unit.readCounters == nil || unit.poll == nil {
		unit.mu.Unlock()
		return ppp.SessionCounters{}, errors.New("ppp: original unit creation was not confirmed")
	}
	file, poll, read := unit.file, unit.poll, unit.readCounters
	unit.mu.Unlock()
	if err := unit.pollNativeUnitFile(file, poll); err != nil {
		return ppp.SessionCounters{}, err
	}
	counters, readErr := read()
	liveErr := unit.pollNativeUnitFile(file, poll)
	unit.mu.Lock()
	closed := unit.closed
	unit.mu.Unlock()
	if closed {
		return ppp.SessionCounters{}, errors.Join(os.ErrClosed, readErr, liveErr)
	}
	if err := errors.Join(readErr, liveErr); err != nil {
		return ppp.SessionCounters{}, err
	}
	return counters, nil
}

// Close ends the original file's custody without waiting for a native counter
// query. It never deletes a device by its current name or native index.
// A caller settling native writes MUST join WaitNativePPPUnitRetired after
// arranging Close and release of the same native-release seal's duplicate.
func (unit *CreatedUnit) Close() error {
	if unit == nil {
		return nil
	}
	unit.closeMu.Lock()
	defer unit.closeMu.Unlock()
	unit.mu.Lock()
	if unit.closed {
		err := unit.closeErr
		unit.mu.Unlock()
		return err
	}
	unit.closed = true
	file := unit.file
	namespace := unit.namespace
	unit.mu.Unlock()
	var err error
	if file != nil {
		err = file.Close()
		unit.finishNativeUnitFileClose(err)
	}
	err = errors.Join(err, namespace.Close())
	unit.mu.Lock()
	unit.closeErr = err
	unit.mu.Unlock()
	return err
}
