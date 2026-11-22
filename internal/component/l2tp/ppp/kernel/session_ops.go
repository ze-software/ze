// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP construction dependencies

package kernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type readInterrupter interface{ InterruptRead() error }

type sessionOps struct {
	wrap           func(int, string) (io.ReadWriteCloser, error)
	closeFD        func(int) error
	duplicateFD    func(int) (int, error)
	stampBirth     func(ppp.SessionIdentity, string) error
	wrapChannel    func(sessionChannelLease) (io.ReadWriteCloser, error)
	closeChannel   func(sessionChannelLease, io.ReadWriteCloser) error
	wrapUnit       func(sessionUnitLease) (io.ReadWriteCloser, error)
	closeUnit      func(sessionUnitLease, io.ReadWriteCloser) error
	connect        func(int, int) error
	preparePair    func(*claimedSessionPair) error
	connectPair    func(*claimedSessionPair) error
	disconnectPair func(*claimedSessionPair) error
	disconnect     func(int) error
	setMRU         func(int, uint16) error
	setIPv4Mode    func(int, NPMode) error
	setIPv6Mode    func(int, NPMode) error
	// openIPv6Carrier opens the carrier from the original unit. Only the
	// production set binds it; a fixture without it refuses the open.
	openIPv6Carrier func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error)
}

func productionSessionOps() sessionOps {
	return sessionOps{wrap: newInterruptibleFDFile, closeFD: closeSessionDescriptor,
		wrapChannel: wrapClaimedSessionChannel, closeChannel: closeClaimedSessionChannel,
		wrapUnit: wrapClaimedSessionUnit, closeUnit: closeClaimedSessionUnit,
		duplicateFD: duplicateSessionDescriptor, stampBirth: stampSessionBirth,
		preparePair: prepareOriginalSessionPair, connectPair: connectOriginalSessionPair, disconnectPair: disconnectOriginalSessionPair, setMRU: realSetMRU,
		setIPv4Mode: SetIPv4Mode, setIPv6Mode: SetIPv6Mode, openIPv6Carrier: openIPv6Carrier}
}

func closeSessionDescriptor(fd int) error {
	file := os.NewFile(uintptr(fd), "ppp.refused")
	if file == nil {
		return fmt.Errorf("ppp: cannot own refused descriptor %d", fd)
	}
	return file.Close()
}

func validateSessionConfig(cfg SessionConfig, unit sessionUnitDescription) error {
	if cfg.ChanFD < 0 {
		return errors.New("ppp: no kernel channel descriptor")
	}
	if unit.fd < 0 {
		return errors.New("ppp: original unit has no descriptor")
	}
	if cfg.ChanFD == unit.fd {
		return errors.New("ppp: kernel channel and original unit descriptors must be distinct")
	}
	if unit.number < 0 {
		return errors.New("ppp: original created unit has no valid native identity")
	}
	if unit.identity.NativeIndex == 0 {
		return errors.New("ppp: original created unit has no valid native identity")
	}
	if unit.identity.InterfaceName == "" {
		return errors.New("ppp: original created unit has no valid native identity")
	}
	if cfg.UnitFD != unit.fd {
		return errors.New("ppp: supplied unit labels disagree with original creation")
	}
	if cfg.UnitNum != unit.number {
		return errors.New("ppp: supplied unit labels disagree with original creation")
	}
	if cfg.InterfaceName != "" {
		if cfg.InterfaceName != unit.identity.InterfaceName {
			return errors.New("ppp: supplied interface name disagrees with original creation")
		}
	}
	if nilSessionDependency(cfg.Backend) {
		return errors.New("ppp: missing kernel interface operations")
	}
	if nilSessionDependency(unit.backend) {
		return errors.New("ppp: session backend differs from original unit creator")
	}
	if !reflect.TypeOf(cfg.Backend).Comparable() {
		return errors.New("ppp: session backend differs from original unit creator")
	}
	if !reflect.TypeOf(unit.backend).Comparable() {
		return errors.New("ppp: session backend differs from original unit creator")
	}
	if cfg.Backend != unit.backend {
		return errors.New("ppp: session backend differs from original unit creator")
	}
	return nil
}

// The PPPoE client's admission boundary uses this same cold-path reflection to
// reject typed nil implementations before any I/O. It never runs per packet.
func nilSessionDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	// Only nil-capable kinds have an IsNil operation.
	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}

// Connect links a previously created channel to its unit. The caller MUST
// serialize this ioctl with descriptor closure, as the client does today.
func Connect(channel, unit int) error { return realConnect(channel, unit) }

// SetMRU changes the unit's receive capacity. The caller MUST retain and
// serialize the unit descriptor against closure for the duration of the call.
func SetMRU(unit int, mru uint16) error { return realSetMRU(unit, mru) }
