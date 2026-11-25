// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel access preparation
// Related: factory.go -- kernel session provider

package kernel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// Prepare reads the selected Linux access device through the shared logical
// name resolver and acquires an independent backend for its sessions. It changes
// no configured interface. The caller MUST Close the access after its sessions
// and discovery stop. Safe for concurrent use, subject to the backend contract.
func Prepare(ctx context.Context, name string) (dataplane.Access, error) {
	if err := requirePlatform(); err != nil {
		return nil, err
	}
	return prepareWithResolver(ctx, name, func() (iface.Backend, error) {
		if _, err := activeBackend(); err != nil {
			return nil, err
		}
		return iface.OpenBackend(Name)
	}, func(logical string) (iface.Binding, error) {
		return iface.ResolveForBackend(Name, logical)
	})
}

func prepareWithResolver(ctx context.Context, name string, acquireBackend func() (iface.Backend, error),
	resolve func(string) (iface.Binding, error)) (access dataplane.Access, err error) {
	return prepareWithCreator(ctx, name, acquireBackend, resolve, pppkernel.NewUnitCreator)
}

// prepareWithCreator is the private preflight fault port; production always
// supplies NewUnitCreator over the SAME acquired independent backend owner.
func prepareWithCreator(ctx context.Context, name string, acquireBackend func() (iface.Backend, error),
	resolve func(string) (iface.Binding, error),
	newCreator func(pppkernel.InterfaceOperations) (*pppkernel.UnitCreator, error)) (access dataplane.Access, err error) {
	if ctx == nil {
		return nil, fmt.Errorf("pppoe: kernel access requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("pppoe: kernel access requires a logical interface name")
	}
	backend, err := acquireBackend()
	if nilBackend(backend) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("pppoe: kernel access backend returned no owner")
	}
	owner := &kernelAccessOwner{name: name, backend: backend}
	defer func() {
		failure := recover()
		var cleanupErr error
		if owner != nil {
			cleanupErr = closeFactoryOwner(owner.Close)
		}
		if failure != nil {
			if cleanupErr != nil {
				cause, ok := failure.(error)
				if !ok {
					cause = fmt.Errorf("pppoe: kernel preparation panicked: %v", failure)
				}
				panic(errors.Join(cause, cleanupErr))
			}
			panic(failure)
		}
		err = errors.Join(err, cleanupErr)
	}()
	if err != nil {
		return nil, err
	}
	owner.creator, err = newCreator(backend)
	if err != nil {
		return nil, err
	}
	if owner.creator == nil {
		return nil, errors.New("pppoe: kernel preparation returned no original unit creator")
	}
	binding, err := resolve(name)
	if err != nil {
		return nil, err
	}
	mac, err := net.ParseMAC(binding.OperMAC)
	if err != nil {
		return nil, fmt.Errorf("pppoe: parse access %q MAC address: %w", name, err)
	}
	if len(mac) != 6 {
		return nil, fmt.Errorf("pppoe: access %q MAC address has %d bytes, want 6", name, len(mac))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info := dataplane.AccessInfo{Device: binding.OsName, ControlDevice: binding.OsName,
		DiscoveryIfIndex: binding.Ifindex, MTU: binding.MTU}
	copy(info.HWAddr[:], mac)
	owner.info = info
	// NewAccess consumes the owner even when snapshot validation refuses it.
	access, err = dataplane.NewAccess(info, owner)
	owner = nil
	return access, err
}

// kernelAccessOwner retains one independent backend for all its sessions. Its
// caller MUST stop admission and join the sessions before Close. Closing the
// access releases only this backend, never the configured Linux interface.
type kernelAccessOwner struct {
	name    string
	info    dataplane.AccessInfo
	backend iface.Backend
	creator *pppkernel.UnitCreator
}

func (o *kernelAccessOwner) Open(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
	if err := requirePlatform(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.AccessName != o.name {
		return nil, errors.New("pppoe: session logical access differs from its prepared kernel access")
	}
	if request.Device != o.info.Device {
		return nil, errors.New("pppoe: session device differs from its prepared kernel access")
	}
	if request.ControlDevice != o.info.ControlDevice {
		return nil, errors.New("pppoe: control device differs from its prepared kernel access")
	}
	if request.Namespace != o.info.Namespace {
		return nil, errors.New("pppoe: control namespace differs from its prepared kernel access")
	}
	return openWithOps(ctx, request, preparedFactoryOps(o.backend, o.creator))
}

func (o *kernelAccessOwner) Close() error { return o.backend.Close() }

func nilBackend(backend iface.Backend) bool {
	if backend == nil {
		return true
	}
	value := reflect.ValueOf(backend)
	// Only nil-capable representations can hide a missing backend owner.
	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
