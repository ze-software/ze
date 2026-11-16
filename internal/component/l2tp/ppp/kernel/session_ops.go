// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP construction dependencies

package kernel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
)

type readInterrupter interface{ InterruptRead() error }

type sessionOps struct {
	wrap           func(int, string) (io.ReadWriteCloser, error)
	closeFD        func(int) error
	interfaceIndex func(string) (uint32, error)
	connect        func(int, int) error
	disconnect     func(int) error
	setMRU         func(int, uint16) error
	setIPv4Mode    func(int, NPMode) error
	setIPv6Mode    func(int, NPMode) error
}

func productionSessionOps() sessionOps {
	return sessionOps{wrap: newInterruptibleFDFile, closeFD: closeSessionDescriptor, interfaceIndex: sessionInterfaceIndex, connect: realConnect, disconnect: realDisconnect, setMRU: realSetMRU, setIPv4Mode: SetIPv4Mode, setIPv6Mode: SetIPv6Mode}
}

func sessionInterfaceIndex(name string) (uint32, error) {
	device, err := net.InterfaceByName(name)
	if err != nil {
		return 0, err
	}
	if device.Index <= 0 {
		return 0, fmt.Errorf("ppp: interface %s has invalid index %d", name, device.Index)
	}
	return uint32(device.Index), nil
}

func closeSessionDescriptor(fd int) error {
	file := os.NewFile(uintptr(fd), "ppp.refused")
	if file == nil {
		return fmt.Errorf("ppp: cannot own refused descriptor %d", fd)
	}
	return file.Close()
}

func validateSessionConfig(cfg SessionConfig) error {
	if cfg.ChanFD < 0 {
		return errors.New("ppp: no kernel channel descriptor")
	}
	if cfg.UnitFD < 0 {
		return errors.New("ppp: no kernel unit descriptor")
	}
	if cfg.ChanFD == cfg.UnitFD {
		return errors.New("ppp: kernel channel and unit descriptors must be distinct")
	}
	if cfg.UnitNum < 0 {
		return errors.New("ppp: invalid kernel unit number")
	}
	if nilSessionDependency(cfg.Backend) {
		return errors.New("ppp: missing kernel interface operations")
	}
	if cfg.ReadCounters == nil {
		return errors.New("ppp: missing raw session counter reader")
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
