// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original CPE native writes
// Related: client_owner.go -- private protocol port; dialer.go -- worker joins

package pppoeclient

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// clientNativeOwner belongs to the dialer's existing cleanup closure. It is
// retained on a partial Dial result too. No registry of retired units is used.
// setup and close run serially; busy refuses concurrent cleanup without holding
// a mutex over a native reply, a worker join or a caller callback.
type clientNativeOwner struct {
	backend          iface.Backend
	creator          *pppkernel.UnitCreator
	channel          *pppkernel.CreatedChannel
	unit             *pppkernel.CreatedUnit
	client           clientProtocol
	writer           iface.NativePPPWriter
	busy             atomic.Bool
	unitCloseStarted bool
	closed           bool
	history          error
	finalCounters    clientNativeFinalCounters // nil means no final sample attempted
}

const clientNativeWait = 5 * time.Second

func prepareClientDial() (*clientDialPreparation, error) {
	return prepareClientDialFrom(func() (iface.Backend, error) { return iface.OpenBackend("netlink") })
}

// prepareClientDialFrom captures one independent original backend before
// discovery. The factory MUST return a distinct owner for every attempt.
func prepareClientDialFrom(open func() (iface.Backend, error)) (prepared *clientDialPreparation, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, clientPanicFailure(failure))
		}
	}()
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		return nil, err
	}
	backend, err := open()
	if err != nil {
		return nil, err
	}
	owner := &clientNativeOwner{backend: backend}
	prepared = &clientDialPreparation{
		setup:        owner.setup,
		close:        owner.close,
		nativeWriter: func() iface.NativePPPWriter { return owner.writer },
	}
	// Refuse before discovery: a name-based backend is not a substitute for
	// the capability bound to the SAME creator backend and original unit.
	if _, ok := backend.(iface.NativePPPBackend); !ok {
		return prepared, errors.New("pppoe-client: original backend cannot bind native PPP writes")
	}
	owner.creator, err = pppkernel.NewUnitCreator(backend)
	return prepared, err
}

func (owner *clientNativeOwner) setup(transport *os.File, name string) (client clientProtocol, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, clientPanicFailure(failure))
		}
	}()
	if err := owner.createOriginalPair(transport, name); err != nil {
		return nil, err
	}
	return owner.bindOriginalPair()
}

// createOriginalPair MUST precede bindOriginalPair. It retains every actual
// partial owner returned by the genuine original-file setup producer.
func (owner *clientNativeOwner) createOriginalPair(transport *os.File, name string) error {
	var err error
	owner.channel, owner.unit, err = pppkernel.DevPPPSetup(owner.creator, transport, name)
	return err
}

// bindOriginalPair MUST follow successful createOriginalPair. It retains the
// actual returned native facet even when binding or namespace restoration fails.
func (owner *clientNativeOwner) bindOriginalPair() (clientProtocol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clientNativeWait)
	defer cancel()
	operations, err := owner.unit.BindNativeOperations(ctx, owner.backend,
		func(unit *pppkernel.CreatedUnit, index int, number int32) (pppkernel.NativeUnitOperations, error) {
			native, ok := owner.backend.(iface.NativePPPBackend)
			if !ok {
				return nil, errors.New("pppoe-client: original backend cannot bind native PPP writes")
			}
			return native.BindNativePPPUnit(unit, index, number)
		})
	// BindNativeOperations retains this exact partial facet independently of
	// the consuming session constructor, including namespace-restore failure.
	owner.writer = operations
	if err != nil {
		return nil, err
	}
	config, err := owner.unit.SessionConfig(owner.channel)
	if err != nil {
		return nil, err
	}
	owner.client, err = pppkernel.NewClientSession(config)
	return owner.client, err
}

// close MUST follow protocol stop and all phase/discovery/frame-reader joins.
// CloseChannel already disconnected the exact pair before its physical close.
// After joins, capture the final same-unit counters before closing its file.
// Pair closure precedes settlement, avoiding writer ACK -> unit Close cycles.
// A genuine passive retirement failure supplies no release proof, so the SAME
// owner/backend remains reachable for a later bounded retry.
func (owner *clientNativeOwner) close() (result error) {
	if !owner.busy.CompareAndSwap(false, true) {
		return errors.Join(iface.ErrNativePPPPending, errors.New("pppoe-client: original cleanup is already running"))
	}
	defer owner.busy.Store(false)
	defer func() {
		if failure := recover(); failure != nil {
			owner.history = errors.Join(owner.history, clientPanicFailure(failure))
			result = errors.Join(owner.history, iface.ErrNativePPPPending)
		}
	}()
	if owner.closed {
		return owner.history
	}
	if owner.unit != nil {
		if !owner.unitCloseStarted {
			owner.unitCloseStarted = true
			owner.history = errors.Join(owner.history, owner.captureFinalCounters())
			if owner.client != nil {
				owner.history = errors.Join(owner.history, closeClientOwner(owner.client.CloseUnit))
			} else {
				if owner.channel != nil {
					owner.history = errors.Join(owner.history, closeClientOwner(owner.channel.Close))
				}
				owner.history = errors.Join(owner.history, closeClientOwner(owner.unit.Close))
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), clientNativeWait)
		defer cancel()
		if err := owner.waitNativeOriginals(ctx); err != nil {
			return errors.Join(owner.history, err, iface.ErrNativePPPPending)
		}
		nativeErr := owner.unit.CloseNativeOperations(ctx)
		if owner.unit.NativeOperationsPending() {
			return errors.Join(owner.history, nativeErr, iface.ErrNativePPPPending)
		}
		owner.history = errors.Join(owner.history, nativeErr)
	} else if owner.channel != nil {
		owner.history = errors.Join(owner.history, closeClientOwner(owner.channel.Close))
		ctx, cancel := context.WithTimeout(context.Background(), clientNativeWait)
		defer cancel()
		if err := owner.waitNativeOriginals(ctx); err != nil {
			return errors.Join(owner.history, err, iface.ErrNativePPPPending)
		}
	}
	closeErr := closeClientOwner(owner.backend.Close)
	if errors.Is(closeErr, iface.ErrNativePPPPending) {
		return errors.Join(owner.history, closeErr)
	}
	owner.history = errors.Join(owner.history, closeErr)
	owner.closed = true
	return owner.history
}
