// Design: docs/research/vpp-deployment-reference.md -- physical peer process leases
// Related: process_peer_linux.go -- kernel capture; process_exit.go -- value proofs
package vpp

import (
	"errors"
	"io"
	"sync"
	"time"
)

const processWitnessMax = 1024
const processReferencesMax = 131072

// processIdentity is a kernel pidfs identity, never a PID, socket pathname or
// server-supplied nonce. Its zero value is invalid. Only capture and test seams
// construct it; both retain the corresponding descriptor during comparison.
type processIdentity struct{ device, inode uint64 }

// processDescriptor owns one exact kernel process reference. Exited must be a
// nonblocking observation of whole-process exit, not connection loss. Identity
// is immutable until Close. The processWitness serializes all descriptor use.
type processDescriptor interface {
	Identity() processIdentity
	Exited() (bool, error)
	Close() error
}

// processWitness owns one descriptor and at most processReferencesMax leases.
// Binding starts one bounded lifetime worker which owns final descriptor close.
// The final lease stops it; ordinary Close joins, while exact-exit Retire only
// releases the reference. Stats witnesses are unbound and start no worker. No history survives
// the final lease. At most processWitnessMax API witnesses exist globally.
// Safe for concurrent use. Nil and zero witnesses are invalid.
type processWitness struct {
	mu                    sync.Mutex
	descriptor            processDescriptor
	identity              processIdentity
	references            int
	connector, generation uint64
	slots                 chan struct{}
	stop                  chan struct{}
	done                  chan struct{}
	failure               error // written by the descriptor owner before done closes
}

func newProcessWitness(descriptor processDescriptor) (*processWitness, error) {
	if descriptor == nil {
		return nil, errors.New("vpp: process witness has no descriptor")
	}
	identity := descriptor.Identity()
	if identity.inode == 0 {
		return nil, errors.New("vpp: process witness has no pidfs identity")
	}
	return &processWitness{descriptor: descriptor, identity: identity, references: 1}, nil
}

func (w *processWitness) bind(connector, generation uint64, slots chan struct{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.references == 0 {
		return io.ErrClosedPipe
	}
	if w.generation != 0 {
		return errors.New("vpp: process witness was already bound")
	}
	if connector == 0 {
		return errors.New("vpp: process witness has no connector identity")
	}
	if generation == 0 {
		return errors.New("vpp: process witness has no connection generation")
	}
	if slots == nil {
		return errors.New("vpp: process witness has no bounded owner")
	}
	select {
	case slots <- struct{}{}:
		w.connector, w.generation, w.slots = connector, generation, slots
		w.stop, w.done = make(chan struct{}), make(chan struct{})
		go w.observeExit()
		return nil
	default:
		return errors.New("vpp: retained process limit reached; run show warnings and reconcile retained VPP owners before reconnecting")
	}
}

// One worker covers one physical API lifetime, including time after its socket
// disconnects. Observer hooks MUST only update bounded bookkeeping. They may
// Retire an exactly covered lease; they MUST NOT Close or perform I/O. After one
// notification this worker waits for final release and owns descriptor cleanup.
func (w *processWitness) observeExit() {
	defer func() { w.finalize(); close(w.done) }()
	ticks := time.NewTicker(connectionProbeInterval)
	defer ticks.Stop()
	reported := false
	// The final lease or actual process exit ends this lifecycle worker.
	for {
		select {
		case <-w.stop:
			return
		case <-ticks.C:
			proof, err := w.exit()
			if err != nil {
				if !reported {
					reportProcessFailure(w, "observation", err)
					reported = true
				}
				continue
			}
			if reported {
				clearProcessObservationFailure(w)
				reported = false
			}
			if proof != nil {
				notifyProcessExit(proof)
				// No new observation or notification is needed after death.
				// References may still be owned by active cleanup consumers.
				<-w.stop
				return
			}
		}
	}
}

func (w *processWitness) exit() (ProcessExit, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.references == 0 {
		return nil, io.ErrClosedPipe
	}
	exited, err := w.descriptor.Exited()
	if err != nil {
		return nil, err
	}
	if !exited {
		return nil, nil //nolint:nilnil // A live original process has no exit proof and no observation failure.
	}
	return exitedProcess{connector: w.connector, generation: w.generation, physical: w.identity}, nil
}

func (w *processWitness) retain() (*ProcessLease, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.references == 0 {
		return nil, io.ErrClosedPipe
	}
	if w.generation == 0 {
		return nil, errors.New("vpp: process witness has no published generation")
	}
	if w.references >= processReferencesMax {
		return nil, errors.New("vpp: process lease limit reached")
	}
	w.references++
	return &ProcessLease{witness: w}, nil
}

func (w *processWitness) alive() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.references == 0 {
		return io.ErrClosedPipe
	}
	exited, err := w.descriptor.Exited()
	if err != nil {
		return err
	}
	if exited {
		return errors.New("vpp: captured Unix peer process exited")
	}
	return nil
}

// drop performs bookkeeping only. A nonnil result belongs only to the caller
// releasing the final bound reference; waiting on it is a separate operation.
func (w *processWitness) drop() (done <-chan struct{}, last bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.references == 0 {
		return nil, false
	}
	w.references--
	if w.references != 0 {
		return nil, false
	}
	if w.stop != nil {
		close(w.stop)
	}
	return w.done, true
}

func (w *processWitness) finalize() {
	w.mu.Lock()
	w.failure = w.descriptor.Close()
	w.mu.Unlock()
	if w.slots != nil {
		<-w.slots
	}
	clearProcessObservationFailure(w)
	if w.failure != nil {
		reportProcessFailure(w, "descriptor-close", w.failure)
	}
}

func (w *processWitness) result() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.failure
}

func (w *processWitness) release() error {
	done, last := w.drop()
	if !last {
		return nil
	}
	if done == nil {
		// An unbound statistics/candidate witness has no lifecycle worker.
		w.finalize()
		return w.result()
	}
	<-done
	return w.result()
}

func (w *processWitness) coveredBy(proof ProcessExit) bool {
	if proof == nil {
		return false
	}
	// A logical epoch alone never proves the physical peer exited. Only the
	// sealed pidfd observer can authorize release, including across epochs.
	physical, ok := proof.(exitedProcess)
	if !ok {
		return false
	}
	if physical.physical.inode == 0 {
		return false
	}
	return physical.physical == w.identity
}

// ProcessLease retains the actual API peer beyond transport disconnection.
// MUST Close each successful RetainProcessForGeneration result, or Retire it
// with a covering sealed exit proof. Both are idempotent and concurrent-safe.
// Close joins only when this lease released the final reference and never holds
// its mutex while joining. Retire is bookkeeping only. A lease is process-local
// and MUST NOT cross RPC. Only the self-contained ProcessExit result may cross
// a component boundary. Nil and zero leases are invalid.
type ProcessLease struct {
	mu      sync.Mutex
	witness *processWitness
	closed  bool
	failure error
	done    <-chan struct{} // nonnil only if this lease released the final reference
}

// Exit returns nil, nil while the captured process lives. Kernel observation
// failure returns an error and never a proof. Retained-resource reconciliation
// MUST poll this before treating a new server incarnation as a replacement.
// No PID namespace translation, integer PID lookup or socket redial occurs.
func (l *ProcessLease) Exit() (ProcessExit, error) {
	if l == nil {
		return nil, errors.New("vpp: process lease is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil, io.ErrClosedPipe
	}
	if l.witness == nil {
		return nil, errors.New("vpp: process lease is uninitialized")
	}
	return l.witness.exit()
}

func (l *ProcessLease) Close() error {
	if l == nil {
		return errors.New("vpp: process lease is nil")
	}
	l.mu.Lock()
	if l.witness == nil {
		l.mu.Unlock()
		return errors.New("vpp: process lease is uninitialized")
	}
	if !l.closed {
		l.closed = true
		l.done, _ = l.witness.drop()
	}
	done, witness, failure := l.done, l.witness, l.failure
	l.mu.Unlock()
	if done == nil {
		return failure
	}
	// The notifier may synchronously call Retire on this same lease. Joining
	// under mu would deadlock the worker against the caller that waits for it.
	<-done
	failure = witness.result()
	l.mu.Lock()
	l.failure = failure
	l.mu.Unlock()
	return failure
}

// Retire releases a lease from a bounded process-exit observer without I/O or
// joining. The existing witness worker owns the final descriptor close and
// slot release. A wrong proof changes nothing. A pidfd proof can cover another
// transport epoch of that same retained physical process. A managed wait alone
// has no retirement authority. Closed leases are idempotent.
func (l *ProcessLease) Retire(proof ProcessExit) error {
	if l == nil {
		return errors.New("vpp: process lease is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.witness == nil {
		return errors.New("vpp: process lease is uninitialized")
	}
	if l.closed {
		return nil
	}
	if !l.witness.coveredBy(proof) {
		return errors.New("vpp: exit proof does not cover this retained process lease")
	}
	l.closed = true
	l.done, _ = l.witness.drop()
	return nil
}

// RetainProcessForGeneration MUST precede the first native mutation whose
// cleanup can survive this transport. The owner MUST retain the returned lease
// until all its native obligations are absent or its exact process exited.
// Epoch checking and reference acquisition are atomic with transport selection.
func (c *Connector) RetainProcessForGeneration(generation uint64) (*ProcessLease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	witness, err := c.processForGenerationLocked(generation)
	if err != nil {
		return nil, err
	}
	if err := witness.alive(); err != nil {
		return nil, err
	}
	return witness.retain()
}

type witnessedConnection interface{ processPeer() *processWitness }

func (c *Connector) processForGenerationLocked(generation uint64) (*processWitness, error) {
	if !c.connected {
		return nil, errVppNotConnected
	}
	if c.generation != generation {
		return nil, errVppGenerationChanged
	}
	connection, ok := c.conn.(witnessedConnection)
	if !ok {
		return nil, errors.New("vpp: API connection has no physical process witness")
	}
	if connection.processPeer() == nil {
		return nil, errors.New("vpp: API process witness is unavailable")
	}
	return connection.processPeer(), nil
}

func (c *Connector) checkAPIProcess(generation uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	witness, err := c.processForGenerationLocked(generation)
	if err != nil {
		return err
	}
	return witness.alive()
}

var errVppStatsPeerMismatch = errors.New("vpp: API and statistics sockets belong to different processes; configure both sockets for the same VPP instance")

func (c *Connector) checkStatsProcess(generation uint64, client runtimeStatsClient) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkStatsProcessLocked(generation, client)
}

// The caller MUST hold c.mu through admission and provider installation. This
// uses only bounded nonblocking pidfd observations, never API or socket I/O.
func (c *Connector) checkStatsProcessLocked(generation uint64, client runtimeStatsClient) error {
	stats, ok := client.(witnessedConnection)
	if !ok {
		return errors.New("vpp: statistics connection has no physical process witness")
	}
	return c.checkStatsPeerLocked(generation, stats.processPeer())
}

func (c *Connector) checkStatsPeer(generation uint64, statsPeer *processWitness) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkStatsPeerLocked(generation, statsPeer)
}

// The caller MUST hold c.mu. The socket hook uses this same comparison before
// mapping, and atomic publication uses it without recursively locking c.mu.
func (c *Connector) checkStatsPeerLocked(generation uint64, statsPeer *processWitness) error {
	apiPeer, err := c.processForGenerationLocked(generation)
	if err != nil {
		return err
	}
	if statsPeer == nil {
		return errors.New("vpp: statistics process witness is unavailable")
	}
	if apiPeer.identity != statsPeer.identity {
		return errVppStatsPeerMismatch
	}
	if err := apiPeer.alive(); err != nil {
		return err
	}
	return statsPeer.alive()
}
