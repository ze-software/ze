// Design: docs/research/vpp-deployment-reference.md -- confirmed process retirement
// Related: process_peer.go -- retained kernel physical exit creates this proof
package vpp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/naming"
)

var connectorIdentities atomic.Uint64

func nextConnectorIdentity() uint64 { return connectorIdentities.Add(1) }

// ProcessExit is a sealed value proving that an exact VPP process exited
// through its retained kernel pidfd. Reconnects, claimed PIDs and completed
// waits alone cannot create it. Covers names the original logical epoch;
// retained physical leases MUST additionally validate their exact witness.
type ProcessExit interface {
	Covers(connectorIdentity, generation uint64) bool
	processExit()
}
type exitedProcess struct {
	connector, generation uint64
	physical              processIdentity
}

func (exitedProcess) processExit() {}
func (p exitedProcess) Covers(connectorIdentity, generation uint64) bool {
	if p.connector == 0 {
		return false
	}
	if generation == 0 {
		return false
	}
	return p.connector == connectorIdentity && generation == p.generation
}

// Identity returns an immutable process-local owner identity, never an OS PID.
func (c *Connector) Identity() uint64 { return c.identity }

// managedConnection identifies a transport physically matched to the descriptor
// obtained atomically when its child was spawned. Its zero value is invalid.
type managedConnection struct {
	connector, generation uint64
	physical              processIdentity
}

// bindManagedProcess MUST precede readiness. The caller retains child until its
// original process has been joined. Reported PIDs carry no physical authority.
func (c *Connector) bindManagedProcess(ctx context.Context, child *processWitness) (managedConnection, error) {
	if ctx == nil {
		return managedConnection{}, errors.New("vpp: managed peer check requires a context")
	}
	if err := ctx.Err(); err != nil {
		return managedConnection{}, err
	}
	if child == nil {
		return managedConnection{}, errors.New("vpp: managed child has no atomic process descriptor")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	peer, err := c.processForGenerationLocked(c.generation)
	if err != nil {
		return managedConnection{}, err
	}
	if peer.identity != child.identity {
		return managedConnection{}, errors.New("vpp: API socket belongs to a different physical process; configure vpp api-socket for the supervised VPP child")
	}
	if err := child.alive(); err != nil {
		return managedConnection{}, err
	}
	if err := peer.alive(); err != nil {
		return managedConnection{}, err
	}
	if err := ctx.Err(); err != nil {
		return managedConnection{}, err
	}
	return managedConnection{connector: c.identity, generation: c.generation, physical: child.identity}, nil
}

// retireManagedConnection follows the completed original child wait and closes
// only its matched transport. The independent pidfd observer alone notifies
// physical death; a logical wait never retires a live or foreign Unix peer.
func (c *Connector) retireManagedConnection(owned managedConnection) {
	c.mu.Lock()
	if owned.connector != c.identity {
		c.mu.Unlock()
		return
	}
	if owned.generation == 0 {
		c.mu.Unlock()
		return
	}
	if owned.physical.inode == 0 {
		c.mu.Unlock()
		return
	}
	if c.generation != owned.generation {
		c.mu.Unlock()
		return
	}
	conn, health := c.conn, c.health
	c.generation++
	c.conn, c.health, c.connected = nil, nil, false
	c.mu.Unlock()
	stopConnectorHealth(conn, health)
}

type processExitObservers struct {
	sync.Mutex
	byName    map[string]func(ProcessExit)
	peerSlots chan struct{}
}

var processObservers = processExitObservers{byName: make(map[string]func(ProcessExit)), peerSlots: make(chan struct{}, processWitnessMax)}

// RegisterProcessExitObserver declares an in-process resource owner's retirement
// hook at initialization. Hooks MUST perform bounded bookkeeping only, with no
// I/O or workers; they MUST NOT treat connection loss as process death. Safe for
// concurrent registration and notification. The fixed bound is not a packet path.
func RegisterProcessExitObserver(name string, observer func(ProcessExit)) error {
	if err := naming.ValidateNodeName("VPP process-exit observer", name, 63); err != nil {
		return err
	}
	if observer == nil {
		return errors.New("vpp: process-exit observer is nil")
	}
	processObservers.Lock()
	defer processObservers.Unlock()
	if _, exists := processObservers.byName[name]; exists {
		return fmt.Errorf("vpp: duplicate process-exit observer %q", name)
	}
	const observerMax = 128
	if len(processObservers.byName) >= observerMax {
		return errors.New("vpp: process-exit observer limit reached")
	}
	processObservers.byName[name] = observer
	return nil
}
func notifyProcessExit(proof ProcessExit) {
	processObservers.Lock()
	observers := make([]func(ProcessExit), 0, len(processObservers.byName))
	for _, observer := range processObservers.byName {
		observers = append(observers, observer)
	}
	processObservers.Unlock()
	for _, observer := range observers {
		observer(proof)
	}
}
