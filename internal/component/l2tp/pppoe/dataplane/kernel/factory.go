// Design: docs/architecture/l2tp/bng-5-pppoe.md -- registered kernel session factory
// Related: socket_linux.go -- native PPPoX allocation; transport.go -- transport owner

// Package kernel implements the netlink PPPoE session data plane. Its PPPoX
// transport owner is separate from the shared kernel PPP channel/unit owner.
package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

type sessionBackend interface {
	pppkernel.InterfaceOperations
	GetStats(string) (*iface.InterfaceStats, error)
}

// Name is the exact registry and interface-backend identity for this provider.
const Name = "netlink"

type factoryOps struct {
	backend       func() (sessionBackend, error)
	openTransport func(context.Context, dataplane.Request) (*pppoxTransport, error)
	setup         func(int, string) (int, int, int, error)
	newSession    func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error)
}

// Open creates one kernel session. Its caller MUST supply a nonnil context and
// close both returned owners. The registration wrapper passes it to the sealed
// backend registry, which validates context before calling. Safe for concurrent use.
func Open(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
	if err := requirePlatform(); err != nil {
		return nil, err
	}
	return openWithOps(ctx, request, factoryOps{
		backend: activeBackend, openTransport: openTransport,
		setup: pppkernel.DevPPPSetup, newSession: pppkernel.NewSession,
	})
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
	transport, err := ops.openTransport(ctx, request)
	if err != nil {
		return nil, err
	}
	// Until NewOpenedSession takes it, this factory owns the transport even
	// during a panic. The PPP constructor separately consumes channel/unit.
	defer func() {
		failure := recover()
		if transport == nil {
			if failure != nil {
				panic(failure)
			}
			return
		}
		closeErr := transport.Close()
		// The discovery guard owns panic containment. This allocation boundary
		// only cleans its unpublished owner and adds a failed close to the panic.
		if failure != nil {
			if closeErr != nil {
				cause, ok := failure.(error)
				if !ok {
					cause = fmt.Errorf("pppoe: kernel construction panicked: %v", failure)
				}
				panic(errors.Join(cause, closeErr))
			}
			panic(failure)
		}
		if err != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	channel, unit, number, err := ops.setup(transport.fd, "")
	if err != nil {
		return nil, err
	}
	frames, handle, err := ops.newSession(pppkernel.SessionConfig{
		ChanFD: channel, UnitFD: unit, UnitNum: number,
		CaptureProxyLCP: request.CaptureProxyLCP, Backend: backend,
		ReadCounters: rawCounters(backend),
	})
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

func validateRequest(request dataplane.Request) error {
	if request.AccessName == "" {
		return errors.New("pppoe: netlink session requires an access name")
	}
	if request.Device == "" {
		return errors.New("pppoe: netlink session requires a resolved kernel device")
	}
	const deviceNameMax = 15 // Linux IFNAMSIZ includes its terminating NUL.
	if len(request.Device) > deviceNameMax {
		return fmt.Errorf("pppoe: kernel device %q exceeds %d bytes", request.Device, deviceNameMax)
	}
	if strings.ContainsRune(request.Device, 0) {
		return errors.New("pppoe: kernel device name contains a NUL byte")
	}
	if request.Namespace != "" {
		return fmt.Errorf("pppoe: netlink session cannot open namespace %q; use an access interface in the daemon's namespace", request.Namespace)
	}
	if request.SessionID == 0 {
		return errors.New("pppoe: netlink session requires a nonzero session ID")
	}
	if request.SessionID == 0xffff {
		return errors.New("pppoe: netlink session ID 65535 is reserved")
	}
	if request.PeerMAC == [6]byte{} {
		return errors.New("pppoe: netlink session requires a peer MAC address")
	}
	if request.PeerMAC[0]&1 != 0 {
		return errors.New("pppoe: netlink session requires a unicast peer MAC address")
	}
	return nil
}

func activeBackend() (sessionBackend, error) {
	backend, err := iface.RequireActiveBackend(Name)
	if err != nil {
		return nil, fmt.Errorf("pppoe: netlink session requires the netlink interface backend; configure interface backend netlink: %w", err)
	}
	return backend, nil
}

func rawCounters(backend sessionBackend) func(string) (ppp.SessionCounters, error) {
	return func(name string) (ppp.SessionCounters, error) {
		stats, err := backend.GetStats(name)
		if err != nil {
			return ppp.SessionCounters{}, err
		}
		if stats == nil {
			return ppp.SessionCounters{}, fmt.Errorf("pppoe: raw counters for %s returned no snapshot", name)
		}
		return ppp.SessionCounters{
			RxBytes: stats.RxBytes, TxBytes: stats.TxBytes,
			RxPackets: stats.RxPackets, TxPackets: stats.TxPackets,
		}, nil
	}
}
