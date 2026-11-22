//go:build linux

// Design: docs/architecture/appliance/kernel-profiles.md -- exact PPP bridge peers.
// Related: original_pair_abi_linux.go -- the sole native peer request ABI.

package kernel

import (
	"errors"
	"io"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// OriginalChannelBridge holds the same one-shot channel claims as sessions.
// Its nil, zero and copied values cannot operate. After successful transfer,
// the caller MUST retain this handle through Close, retrying an uncertain
// native cleanup. Close joins Unbridge before closing both claimed originals.
// It starts no workers. Safe for concurrent use.
type OriginalChannelBridge struct {
	mu             sync.Mutex
	self           *OriginalChannelBridge
	first          sessionChannelLease
	second         sessionChannelLease
	firstFeatures  uint32
	secondFeatures uint32
	closed         bool
	closeErr       error
}

// NewOriginalChannelBridge recognizes BOTH actual channel query replies and
// retains their files and capability receipts before any native mutation.
// EVERY newly supplied owner is consumed on EVERY outcome, including panic.
// Replayed originals return no new lease and MUST NOT be closed by rollback.
// Native files, never labels, choose peers. After successful transfer the
// caller MUST use bridge.Close, not close either consumed channel directly.
func NewOriginalChannelBridge(first, second *CreatedChannel) (*OriginalChannelBridge, error) {
	return newOriginalChannelBridge(first, second, prepareOriginalChannelBridge)
}

// This composes the existing shared claim/close helpers for two channels; it
// introduces no second claim flag, owner token, or replay/alias algorithm.
func newOriginalChannelBridge(first, second sessionChannelSource, prepare func(*OriginalChannelBridge) error) (bridge *OriginalChannelBridge, err error) {
	bridge = &OriginalChannelBridge{}
	owned := bridge
	transferred := false
	defer func() {
		if transferred {
			return
		}
		failure := recover()
		err = owned.finishFailedConstruction(failure, err)
	}()
	var firstErr, secondErr error
	var firstPanic, secondPanic any
	owned.first, firstPanic, firstErr = claimSessionChannel(first)
	owned.second, secondPanic, secondErr = claimSessionChannel(second)
	if failure := sessionClaimPanic(firstPanic, secondPanic); failure != nil {
		panic(failure)
	}
	if err := errors.Join(firstErr, secondErr); err != nil {
		return nil, err
	}
	if err := validateSessionNamespace(owned.first.namespaceProof(), owned.second.namespaceProof()); err != nil {
		return nil, err
	}
	if err := prepare(owned); err != nil {
		return nil, originalAttachmentRefusal(err)
	}
	owned.self = owned
	transferred = true
	return owned, nil
}

func (bridge *OriginalChannelBridge) finishFailedConstruction(failure any, err error) error {
	var firstErr, secondErr error
	if bridge.first != nil {
		firstErr = closeOriginalSessionFile(func() error { return closeOwnedSessionChannel(bridge.first, nil, nil) })
	}
	if bridge.second != nil {
		if bridge.second != bridge.first {
			secondErr = closeOriginalSessionFile(func() error { return closeOwnedSessionChannel(bridge.second, nil, nil) })
		}
	}
	cleanupErr := errors.Join(firstErr, secondErr)
	if failure != nil {
		if cleanupErr != nil {
			panic(RecordSetupCleanup(sessionPanicFailure(failure), cleanupErr))
		}
		panic(failure)
	}
	return RecordSetupCleanup(err, cleanupErr)
}

func prepareOriginalChannelBridge(bridge *OriginalChannelBridge) error {
	if err := checkOriginalBridgeSupport(bridge); err != nil {
		return err
	}
	return bridge.withOriginalFiles(func(firstFD, secondFD int) error {
		firstOwner := pppPairChannelOwner{abi: pppPairOwnerABI}
		if err := pppPairReadChannelOwner(firstFD, &firstOwner); err != nil {
			return err
		}
		if err := validateOriginalBridgeReply(firstOwner); err != nil {
			return err
		}
		secondOwner := pppPairChannelOwner{abi: pppPairOwnerABI}
		if err := pppPairReadChannelOwner(secondFD, &secondOwner); err != nil {
			return err
		}
		if err := validateOriginalBridgeReply(secondOwner); err != nil {
			return err
		}
		bridge.firstFeatures = firstOwner.features
		bridge.secondFeatures = secondOwner.features
		return nil
	})
}

func originalBridgeFile(lease sessionChannelLease) (*os.File, error) {
	original, ok := lease.(*claimedCreatedChannel)
	if !ok {
		return nil, errors.New("ppp: bridge requires the original channel claim")
	}
	channel := original.channel
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.self != channel {
		return nil, errors.New("ppp: bridge channel owner was copied or unminted")
	}
	if channel.sessionClaim != original {
		return nil, errors.New("ppp: bridge channel claim changed")
	}
	if channel.closed {
		return nil, os.ErrClosed
	}
	if !channel.ready {
		return nil, errors.New("ppp: bridge channel birth is not confirmed")
	}
	if channel.file == nil {
		return nil, os.ErrClosed
	}
	if channel.file != original.file {
		return nil, errors.New("ppp: bridge original file changed")
	}
	if original.descriptorErr != nil {
		return nil, original.descriptorErr
	}
	return original.file, nil
}

func validateOriginalBridgeReply(owner pppPairChannelOwner) error {
	if owner.abi != pppPairOwnerABI {
		return errors.New("ppp: bridge channel query returned another ABI")
	}
	if owner.reserved != 0 {
		return errors.New("ppp: bridge channel query returned reserved data")
	}
	if owner.channel < 0 {
		return errors.New("ppp: bridge channel query returned an invalid label")
	}
	if owner.features&pppPairBridgeFeature == 0 {
		return unix.EOPNOTSUPP
	}
	return nil
}

// No owner mutex stays held across another owner's mutex or file lease: the
// two strong os.File references independently exclude fd reuse during Control.
// Concurrent owner Close either precedes the borrow or waits for native use.
func (bridge *OriginalChannelBridge) withOriginalFiles(operation func(int, int) error) error {
	first, err := originalBridgeFile(bridge.first)
	if err != nil {
		return err
	}
	second, err := originalBridgeFile(bridge.second)
	if err != nil {
		return err
	}
	if first == second {
		return errors.New("ppp: bridge channels alias the same original file")
	}
	left, err := first.SyscallConn()
	if err != nil {
		return err
	}
	right, err := second.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	err = left.Control(func(firstFD uintptr) {
		if int(firstFD) != bridge.first.descriptor() {
			operationErr = errors.New("ppp: first bridge descriptor changed")
			return
		}
		secondErr := right.Control(func(secondFD uintptr) {
			if int(secondFD) != bridge.second.descriptor() {
				operationErr = errors.New("ppp: second bridge descriptor changed")
				return
			}
			operationErr = operation(int(firstFD), int(secondFD))
		})
		operationErr = errors.Join(operationErr, secondErr)
	})
	return errors.Join(err, operationErr)
}

func (bridge *OriginalChannelBridge) operation(command uintptr) error {
	if bridge == nil {
		return errors.New("ppp: missing original channel bridge")
	}
	if bridge.self != bridge {
		return errors.New("ppp: original channel bridge was copied or unminted")
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.closed {
		return io.ErrClosedPipe
	}
	return bridge.operationLocked(command)
}

func (bridge *OriginalChannelBridge) operationLocked(command uintptr) error {
	return bridge.withOriginalFiles(func(firstFD, secondFD int) error {
		if err := pppPairPeerOperation(firstFD, command, secondFD); err != nil {
			return originalAttachmentRefusal(err)
		}
		return nil
	})
}

func (bridge *OriginalChannelBridge) Bridge() error { return bridge.operation(pppPairBridge) }

// Unbridge deliberately uses the initially
// retained receipt: a retired native channel may settle its exact old pair.
func (bridge *OriginalChannelBridge) Unbridge() error { return bridge.operation(pppPairUnbridge) }

// Close MUST follow the caller's worker joins. An uncertain exact Unbridge
// preserves both original files and this handle for retry. After its joined
// success, both existing claim close helpers run despite any error or panic.
func (bridge *OriginalChannelBridge) Close() error {
	if bridge == nil {
		return nil
	}
	if bridge.self != bridge {
		return errors.New("ppp: original channel bridge was copied or unminted")
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.closed {
		return bridge.closeErr
	}
	if err := bridge.operationLocked(pppPairUnbridge); err != nil {
		return err
	}
	bridge.closed = true
	firstErr := closeOriginalSessionFile(func() error { return closeOwnedSessionChannel(bridge.first, nil, nil) })
	secondErr := closeOriginalSessionFile(func() error { return closeOwnedSessionChannel(bridge.second, nil, nil) })
	bridge.closeErr = errors.Join(firstErr, secondErr)
	return bridge.closeErr
}

// ClosePending reports whether this handle still owns native cleanup and its
// original files. A terminal file-close error is retained by Close but leaves
// no pending native relationship. Nil has nothing pending; an invalid/copied
// handle cannot establish completed cleanup and conservatively answers true.
// Safe for concurrent use; the caller MUST still retain any returned error.
func (bridge *OriginalChannelBridge) ClosePending() bool {
	if bridge == nil {
		return false
	}
	if bridge.self != bridge {
		return true
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return !bridge.closed
}
