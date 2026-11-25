// Design: docs/architecture/l2tp/bng-5-pppoe.md -- registered kernel session factory
// Related: socket_linux.go -- native PPPoX allocation; transport.go -- transport owner

// Package kernel implements the kernel PPPoE session data plane. Its PPPoX
// transport owner is separate from the shared kernel PPP channel/unit owner.
package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

type sessionBackend interface {
	pppkernel.InterfaceOperations
}

// Name is the exact registry and interface-backend identity for this provider.
const Name = "kernel"

type factoryOps struct {
	backend       func() (sessionBackend, error)
	openTransport func(context.Context, dataplane.Request) (*pppoxTransport, error)
	creator       func(sessionBackend) (*pppkernel.UnitCreator, error)
	setup         func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error)
	config        func(*pppkernel.CreatedUnit, *pppkernel.CreatedChannel) (pppkernel.SessionConfig, error)
	closeUnit     func(*pppkernel.CreatedUnit) error
	closeChannel  func(*pppkernel.CreatedChannel) error
	newSession    func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error)
}

// preparedFactoryOps borrows only the original independent prepared backend.
// Its private fault ports never grant public ownership from descriptor labels.
func preparedFactoryOps(backend sessionBackend, creator *pppkernel.UnitCreator) factoryOps {
	return factoryOps{
		backend:       func() (sessionBackend, error) { return backend, nil },
		creator:       func(sessionBackend) (*pppkernel.UnitCreator, error) { return creator, nil },
		openTransport: openTransport, setup: func(creator *pppkernel.UnitCreator, transport *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
			return setupOriginalNativePPP(backend, creator, transport, name)
		},
		config:    (*pppkernel.CreatedUnit).SessionConfig,
		closeUnit: closeOriginalNativePPP, closeChannel: (*pppkernel.CreatedChannel).Close,
		newSession: pppkernel.NewSession,
	}
}

func openWithOps(ctx context.Context, request dataplane.Request, ops factoryOps) (
	opened dataplane.OpenedSession, err error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	backend, err := ops.backend()
	if err != nil {
		return nil, err
	}
	creator, err := ops.creator(backend)
	if err != nil {
		return nil, err
	}
	if creator == nil {
		return nil, errors.New("pppoe: kernel factory has no original unit creator")
	}
	transport, err := ops.openTransport(ctx, request)
	var channel *pppkernel.CreatedChannel
	var unit *pppkernel.CreatedUnit
	// Until NewOpenedSession takes it, this factory owns the transport even
	// during a panic. The PPP constructor separately consumes channel/unit.
	defer func() {
		failure := recover()
		var cleanupErr error
		if unit != nil {
			cleanupErr = errors.Join(cleanupErr, closeFactoryOwner(func() error { return ops.closeUnit(unit) }))
		}
		if channel != nil {
			cleanupErr = errors.Join(cleanupErr, closeFactoryOwner(func() error { return ops.closeChannel(channel) }))
		}
		if transport != nil {
			cleanupErr = errors.Join(cleanupErr, closeFactoryOwner(transport.Close))
		}
		// The discovery guard owns panic containment. This allocation boundary
		// only cleans its unpublished owner and adds a failed close to the panic.
		if failure != nil {
			if cleanupErr != nil {
				cause, ok := failure.(error)
				if !ok {
					cause = fmt.Errorf("pppoe: kernel construction panicked: %v", failure)
				}
				panic(errors.Join(cause, cleanupErr))
			}
			panic(failure)
		}
		if err != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if err != nil {
		return nil, err
	}
	if transport == nil {
		return nil, errors.New("pppoe: kernel factory returned no original transport")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	channel, unit, err = ops.setup(creator, transport.file, "")
	if err != nil {
		return nil, err
	}
	cfg, err := ops.config(unit, channel)
	if err != nil {
		return nil, err
	}
	cfg.CaptureProxyLCP = request.CaptureProxyLCP
	// SessionConfig is non-consuming. NewSession alone consumes BOTH original
	// owners on every outcome, including panic, so relinquish them before call.
	channel, unit = nil, nil
	frames, handle, err := ops.newSession(cfg)
	if err != nil {
		return nil, err
	}
	// NewOpenedSession consumes both owners on every outcome. Relinquish our
	// deferred cleanup before transferring them, including an invalid result.
	transportOwner := transport
	transport = nil
	opened, err = dataplane.NewOpenedSession(frames, handle, transportOwner)
	if err != nil {
		return nil, err
	}
	// Construction consumes channel/unit even if cancellation arrived during
	// setup. The admitted result now exposes their one permitted close path.
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, opened.Dataplane().Close(), opened.CloseTransport())
	}
	return opened, nil
}

// closeFactoryOwner retains each independent cleanup outcome even if one
// private dependency panics; its sibling and original transport still close.
func closeFactoryOwner(closeOwner func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			cause, ok := failure.(error)
			if ok {
				err = cause
				return
			}
			err = fmt.Errorf("pppoe: kernel cleanup panicked: %v", failure)
		}
	}()
	return closeOwner()
}

func validateRequest(request dataplane.Request) error {
	if request.AccessName == "" {
		return errors.New("pppoe: kernel session requires an access name")
	}
	if request.Device == "" {
		return errors.New("pppoe: kernel session requires a resolved kernel device")
	}
	const deviceNameMax = 15 // Linux IFNAMSIZ includes its terminating NUL.
	if len(request.Device) > deviceNameMax {
		return fmt.Errorf("pppoe: kernel device %q exceeds %d bytes", request.Device, deviceNameMax)
	}
	if strings.ContainsRune(request.Device, 0) {
		return errors.New("pppoe: kernel device name contains a NUL byte")
	}
	if request.Namespace != "" {
		return fmt.Errorf("pppoe: kernel session cannot open namespace %q; use an access interface in the daemon's namespace", request.Namespace)
	}
	if request.SessionID == 0 {
		return errors.New("pppoe: kernel session requires a nonzero session ID")
	}
	if request.SessionID == 0xffff {
		return errors.New("pppoe: kernel session ID 65535 is reserved")
	}
	if request.PeerMAC == [6]byte{} {
		return errors.New("pppoe: kernel session requires a peer MAC address")
	}
	if request.PeerMAC[0]&1 != 0 {
		return errors.New("pppoe: kernel session requires a unicast peer MAC address")
	}
	return nil
}

func activeBackend() (sessionBackend, error) {
	backend, err := iface.RequireActiveBackend(Name)
	if err != nil {
		return nil, fmt.Errorf("pppoe: kernel session requires the kernel interface backend; configure interface backend kernel: %w", err)
	}
	return backend, nil
}
