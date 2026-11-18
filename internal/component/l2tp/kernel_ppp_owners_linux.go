// Design: docs/research/l2tpv2-ze-integration.md -- original PPP worker resources
// Related: kernel_linux.go -- the worker retains these owners through transfer

//go:build linux

package l2tp

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"reflect"
	"slices"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/opererr"
)

type kernelPPPSetupOperations struct {
	create   func(int, netip.AddrPort, uint16, uint16, uint16, uint16) (*os.File, int, error)
	setup    func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error)
	channel  func(*os.File) (*pppkernel.CreatedChannel, error)
	snapshot func(*kernelPPPOwners) (pppkernel.SessionConfig, error)
	bridge   func(kernelSetupEvent, *kernelPPPOwners) (bool, error)
	close    func(*kernelPPPOwners, *os.File, [3]int) [3]error
}

// openKernelPPPTransport supplies the original file and a diagnostic label.
// Only private fault fixtures supply a logical label without a native file.
func openKernelPPPTransport(socketFD int, peer netip.AddrPort, localTID, localSID, remoteTID, remoteSID uint16) (*os.File, int, error) {
	file, err := pppoxCreate(socketFD, peer, localTID, localSID, remoteTID, remoteSID)
	if file == nil {
		return nil, -1, err
	}
	return file, int(file.Fd()), err
}

// withKernelPPPTransport pins the original through one native operation.
// Its fd exists only inside RawConn.Control and cannot authorize later use.
func withKernelPPPTransport(file *os.File, operation func(int) error) error {
	if file == nil {
		return errors.New("l2tp: setup has no original transport file")
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	if err := raw.Control(func(fd uintptr) { operationErr = operation(int(fd)) }); err != nil {
		return err
	}
	return operationErr
}

// prepareKernelPPPSetup captures the original backend's creation capability
// before any subscriber allocation. A LAC bridge has no local PPP unit and
// therefore requires only its original attached channel, not a UnitCreator.
func prepareKernelPPPSetup(ev kernelSetupEvent, backend pppkernel.InterfaceOperations) (kernelPPPSetup, error) {
	if !ev.lnsMode {
		if ev.channel != nil {
			return func() (pppSessionFDs, error) { return pppSetupReal(ev, nil) }, nil
		}
	}
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		return nil, opererr.Wrap(err, "L2TP cannot create its original PPP interface",
			"The kernel backend cannot retain creation-bound counters for this subscriber.", "ze doctor")
	}
	return func() (pppSessionFDs, error) { return pppSetupReal(ev, creator) }, nil
}

func originalKernelPPPConfig(owners *kernelPPPOwners, backend pppkernel.InterfaceOperations) (pppkernel.SessionConfig, error) {
	if nilKernelBackend(backend) {
		return pppkernel.SessionConfig{}, errors.New("l2tp: PPP has no original kernel backend")
	}
	if _, err := pppkernel.NewUnitCreator(backend); err != nil {
		return pppkernel.SessionConfig{}, err
	}
	config, err := snapshotOriginalKernelPPP(owners)
	if err != nil {
		return pppkernel.SessionConfig{}, err
	}
	if !sameKernelBackend(config.Backend, backend) {
		return pppkernel.SessionConfig{}, errors.New("l2tp: PPP backend differs from its original unit creator")
	}
	return config, nil
}

func snapshotOriginalKernelPPP(owners *kernelPPPOwners) (pppkernel.SessionConfig, error) {
	if owners == nil {
		return pppkernel.SessionConfig{}, errors.New("l2tp: PPP has no retained original files")
	}
	if owners.unit == nil {
		return pppkernel.SessionConfig{}, errors.New("l2tp: local PPP has no original created unit")
	}
	return owners.unit.SessionConfig(owners.channel)
}

func sameKernelBackend(first, second pppkernel.InterfaceOperations) bool {
	if nilKernelBackend(first) {
		return false
	}
	if nilKernelBackend(second) {
		return false
	}
	if !reflect.TypeOf(first).Comparable() {
		return false
	}
	if !reflect.TypeOf(second).Comparable() {
		return false
	}
	return first == second
}

// closeOriginalKernelPPP closes each distinct native owner, not the unit or
// channel labels. The caller consumes the original transport pointer before
// this operation. All three closes run despite an error/panic.
// Returned positions retain unit, channel, transport diagnostic order.
func closeOriginalKernelPPP(owners *kernelPPPOwners, transport *os.File, closeTransport func(*os.File) error) (failures [3]error) {
	if owners != nil {
		if owners.bridge != nil {
			failures[1] = closeKernelPPPOperation(owners.bridge.Close)
			// Native uncertainty retains BOTH channels and the transport. A terminal
			// descriptor error still permits independent transport cleanup below.
			if owners.bridge.ClosePending() {
				return failures
			}
			owners.bridge = nil
		}
		if owners.relayFile != nil {
			failures[1] = errors.Join(failures[1], closeKernelPPPOperation(owners.relayFile.Close))
			owners.relayFile = nil
		}
	}
	var operations [3]kernelPPPCloseOperation
	if owners != nil {
		if owners.unit != nil {
			operations[0] = kernelPPPCloseOperation{identity: owners.unit, close: owners.unit.Close}
		}
		if owners.channel != nil {
			operations[1] = kernelPPPCloseOperation{identity: owners.channel, close: owners.channel.Close}
		}
	}
	if transport != nil {
		operations[2] = kernelPPPCloseOperation{identity: transport, close: func() error { return closeTransport(transport) }}
	}
	closed := closeKernelPPPResources(operations)
	for index := range failures {
		failures[index] = errors.Join(failures[index], closed[index])
	}
	return failures
}

// kernelPPPCloseOperation is a private cleanup port. A nil close owns nothing.
// Its identity MUST be comparable: an original typed pointer in production,
// or an int for an explicitly private logical test token.
// Identity only suppresses duplicate operations; it never creates custody.
type kernelPPPCloseOperation struct {
	identity any
	close    func() error
}

// closeKernelPPPResources is the sole cleanup algorithm for both actual
// original owners and logical fault ports. Slots MUST be unit, channel,
// transport. It tries each distinct operation despite any error or panic.
// Native unit/channel identity is their exact pointer, never a numeric label.
func closeKernelPPPResources(operations [3]kernelPPPCloseOperation) (failures [3]error) {
	for index, operation := range operations {
		if operation.close == nil {
			continue
		}
		if slices.ContainsFunc(operations[:index], func(previous kernelPPPCloseOperation) bool {
			return previous.identity == operation.identity
		}) {
			continue
		}
		failures[index] = closeKernelPPPOperation(operation.close)
	}
	return failures
}

func closeKernelPPPOperation(operation func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = kernelPPPFailure(failure)
		}
	}()
	return operation()
}

func kernelPPPFailure(failure any) error {
	if cause, ok := failure.(error); ok {
		return fmt.Errorf("l2tp: original PPP operation panicked: %w", cause)
	}
	return fmt.Errorf("l2tp: original PPP operation panicked: %v", failure)
}

func bridgeOriginalKernelPPP(ev kernelSetupEvent, owners *kernelPPPOwners) (bool, error) {
	return maybeBridgePPPoE(ev, owners)
}

func unbridgeOriginalKernelPPP(owners *kernelPPPOwners) error {
	if owners == nil {
		return errors.New("l2tp: bridge cleanup has no original resources")
	}
	if owners.bridge == nil {
		return errors.New("l2tp: bridge cleanup has no retained original bridge")
	}
	return owners.bridge.Unbridge()
}
