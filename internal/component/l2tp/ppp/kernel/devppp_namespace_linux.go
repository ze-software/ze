// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original channel/unit namespace
// Related: session_namespace_linux.go -- retained producer namespace proof
// Related: internal/component/l2tp/pppoe/discovery_namespace.go -- startup worker precedent

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// A namespace failure cannot classify or erase an operation's own outcome.
// This transient result holds no native authority or persistent ledger.
type originalPPPNamespaceResult struct {
	operationErr error
	namespaceErr error
}

func (result originalPPPNamespaceResult) err() error {
	return errors.Join(result.operationErr, result.namespaceErr)
}

// Private complete operations keep restoration/partial-result tests separate
// from public native authority. Only the production constructor selects these.
type pppSetupNamespaceOps struct {
	lock    func()
	unlock  func()
	current func() (*capturedSessionNamespace, error)
	enter   func(*os.File) error
}

// One synchronous, joined lifecycle startup worker owns any namespace switch.
// No source socket or channel file lease spans the provider call. A failed
// restore retires this locked OS thread when the worker exits. The caller's
// thread is never changed. The existing backend checks its subscribed socket's
// actual namespace before Send; a foreign backend cannot mint a matching unit.
func createPPPSetupUnit(creator *UnitCreator, channel *CreatedChannel, name string) (*CreatedUnit, error) {
	if channel == nil {
		return nil, errors.New("ppp: missing original channel namespace")
	}
	if channel.namespace == nil {
		return nil, errors.New("ppp: original channel namespace is unknown")
	}
	return createPPPSetupUnitWith(creator, channel.namespace, name, runOriginalPPPNamespace)
}

// Shared by unit creation and pair admission before either begins file leases.
// The target MUST be its original producer-retained namespace object. The
// operation MUST finish before returning, and MUST NOT retain this worker.
// Concurrent target Close may refuse before Enter. Successful Setns itself
// pins that exact native namespace until restore or locked worker termination.
// No mutex or file lease may span the call; the caller MUST join this result.
func runOriginalPPPNamespace(target *capturedSessionNamespace, operation func() error) originalPPPNamespaceResult {
	if target == nil {
		return originalPPPNamespaceResult{namespaceErr: errors.New("ppp: original namespace is unknown")}
	}
	if operation == nil {
		return originalPPPNamespaceResult{operationErr: errors.New("ppp: missing original namespace operation")}
	}
	result := make(chan originalPPPNamespaceResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result <- runOriginalPPPNamespaceThread(target, operation, pppSetupNamespaceOps{
			lock: runtime.LockOSThread, unlock: runtime.UnlockOSThread,
			current: captureSessionNamespace, enter: enterPPPSetupNamespace,
		})
	}()
	created := <-result
	<-done
	return created
}

// Actual setup and private lifecycle runners share this one creation/finish
// producer. No caller may keep a separate namespace-failure classification.
func createPPPSetupUnitWith(creator *UnitCreator, target *capturedSessionNamespace, name string, runNamespace func(*capturedSessionNamespace, func() error) originalPPPNamespaceResult) (unit *CreatedUnit, err error) {
	result := runNamespace(target, func() (err error) {
		unit, err = creator.Create(name)
		return err
	})
	if result.namespaceErr != nil {
		refusePPPSetupNamespaceUnit(unit)
	}
	return unit, result.err()
}

// Private runner adapter injects only namespace operations. Creation, result
// ownership and revocation are the exact actual setup producer above.
func createPPPSetupUnitThreadWith(creator *UnitCreator, target *capturedSessionNamespace, name string, ops pppSetupNamespaceOps) (*CreatedUnit, error) {
	return createPPPSetupUnitWith(creator, target, name,
		func(target *capturedSessionNamespace, operation func() error) originalPPPNamespaceResult {
			return runOriginalPPPNamespaceThread(target, operation, ops)
		})
}

func runOriginalPPPNamespaceThread(target *capturedSessionNamespace, operation func() error, ops pppSetupNamespaceOps) (result originalPPPNamespaceResult) {
	defer func() {
		if failure := recover(); failure != nil {
			// A panic may come from Enter/restore/cleanup rather than the
			// provider. Never preserve definite-rejection authority for it.
			result.namespaceErr = errors.Join(result.namespaceErr, sessionPanicFailure(failure))
		}
	}()
	ops.lock()
	restored := true
	defer func() {
		if restored {
			ops.unlock()
		}
	}()
	original, err := ops.current()
	if err != nil {
		result.namespaceErr = err
		return result
	}
	defer func() {
		if closeErr := original.Close(); closeErr != nil {
			result.namespaceErr = errors.Join(result.namespaceErr, closeErr)
		}
	}()
	// A failed identity read is not permission to enter an unknown namespace.
	if target.file == nil {
		result.namespaceErr = errors.New("ppp: original channel has no namespace file")
		return result
	}
	originalIdentity, err := original.file.Stat()
	if err != nil {
		result.namespaceErr = err
		return result
	}
	targetIdentity, err := target.file.Stat()
	if err != nil {
		result.namespaceErr = err
		return result
	}
	if os.SameFile(originalIdentity, targetIdentity) {
		result.operationErr = operation()
		return result
	}
	if err := ops.enter(target.file); err != nil {
		result.namespaceErr = fmt.Errorf("ppp: enter original transport namespace: %w", err)
		return result
	}
	restored = false
	defer func() {
		restoreErr := ops.enter(original.file)
		restored = restoreErr == nil
		if restoreErr != nil {
			result.namespaceErr = errors.Join(result.namespaceErr, fmt.Errorf("ppp: restore setup worker namespace: %w", restoreErr))
		}
	}()
	result.operationErr = operation()
	return result
}

// A definite creation rejection does not classify a namespace cleanup failure.
// Such a result MUST NOT authorize C1's collision retry or later admission.
func refusePPPSetupNamespaceUnit(unit *CreatedUnit) {
	if unit == nil {
		return
	}
	unit.ready = false
	unit.creationRejection = nil
}

func enterPPPSetupNamespace(file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	leaseErr := connection.Control(func(fd uintptr) { operationErr = unix.Setns(int(fd), unix.CLONE_NEWNET) })
	return errors.Join(operationErr, leaseErr)
}
