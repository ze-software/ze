// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discovery namespace ownership
// Related: discovery_namespace_linux.go -- Linux namespace and socket operations

package pppoe

import (
	"errors"
	"fmt"
)

type discoveryNamespace interface {
	Enter() error
	Close() error
}

type discoveryNamespaceOperations interface {
	LockThread()
	UnlockThread()
	Current() (discoveryNamespace, error)
	Named(string) (discoveryNamespace, error)
	OpenSocket() (int, error)
	CloseSocket(int) error
}

type discoverySocketResult struct {
	fd  int
	err error
}

// openDiscoveryNamespace MUST transfer a successful descriptor to a caller
// which closes it after discovery stops. No descriptor escapes on failure.
// A dedicated startup worker owns the temporary thread: if namespace restore
// fails, it exits still locked, so Go discards that thread rather than allowing
// unrelated work to execute in the wrong namespace. The caller synchronously
// joins the worker; there is one result and no unbounded queue or retry.
func openDiscoveryNamespace(name string, ops discoveryNamespaceOperations) (int, error) {
	if name == "" {
		return ops.OpenSocket()
	}
	result := make(chan discoverySocketResult, 1)
	go func() {
		fd, err := openDiscoveryNamespaceThread(name, ops)
		result <- discoverySocketResult{fd: fd, err: err}
	}()
	opened := <-result
	return opened.fd, opened.err
}

func openDiscoveryNamespaceThread(name string, ops discoveryNamespaceOperations) (fd int, err error) {
	fd = -1
	ops.LockThread()
	restored := true
	defer func() {
		if restored {
			ops.UnlockThread()
		}
	}()
	// Registered before namespace handles: an error from restoration or either
	// handle close also refuses and releases a socket that was already opened.
	defer func() {
		if fd >= 0 {
			if err != nil {
				err = errors.Join(err, ops.CloseSocket(fd))
				fd = -1
			}
		}
	}()
	original, err := ops.Current()
	if err != nil {
		return -1, fmt.Errorf("read current discovery namespace: %w", err)
	}
	defer func() { err = errors.Join(err, original.Close()) }()
	target, err := ops.Named(name)
	if err != nil {
		return -1, fmt.Errorf("open discovery namespace %q: %w", name, err)
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	if err := target.Enter(); err != nil {
		return -1, fmt.Errorf("enter discovery namespace %q: %w", name, err)
	}
	restored = false
	defer func() {
		restoreErr := original.Enter()
		restored = restoreErr == nil
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore discovery namespace: %w", restoreErr))
		}
	}()
	return ops.OpenSocket()
}
