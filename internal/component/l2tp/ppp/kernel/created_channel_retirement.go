// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual channel-file close
// Related: created_channel.go -- same original owner and first real Close

package kernel

import (
	"context"
	"errors"
	"os"
)

// nativePPPChannelRetirement belongs only to its same CreatedChannel. A nil
// outcome means the actual first file Close has not completed; an outcome with
// nil cause means it succeeded. Namespace/history errors never create or replace
// that physical result. The original's short borrows must also finish.
type nativePPPChannelRetirement struct {
	borrows   int
	fileClose *nativePPPChannelCloseResult
	retired   *nativePPPChannelCloseResult
	done      chan struct{}
}
type nativePPPChannelCloseResult struct{ cause error }

// WaitNativePPPChannelRetired never initiates Close. The caller MUST advance
// this original channel Close first and MUST NOT let this wait prevent a
// separately owned original unit Close. It MUST retain writer/backend while
// this same channel's actual first file-close outcome remains uncertain. Nil
// follows only a successful actual Close and every original short borrow.
// A first actual close error remains pending custody; it cannot be repaired by
// Stat, errno classification or retrying a numeric descriptor. Safe concurrently;
// nil, zero and copied values grant no physical release proof.
func (channel *CreatedChannel) WaitNativePPPChannelRetired(ctx context.Context) error {
	if channel == nil {
		return errors.New("ppp: original channel retirement owner is absent")
	}
	if channel.self != channel {
		return errors.New("ppp: copied or unminted channel cannot confirm retirement")
	}
	if ctx == nil {
		return errors.New("ppp: original channel retirement needs a context")
	}
	channel.mu.Lock()
	if channel.file == nil {
		channel.mu.Unlock()
		return errors.New("ppp: original channel file is absent")
	}
	if result := channel.retirement.retired; result != nil {
		channel.mu.Unlock()
		return result.cause
	}
	if channel.retirement.done == nil {
		channel.retirement.done = make(chan struct{})
	}
	done := channel.retirement.done
	channel.mu.Unlock()
	select {
	case <-done:
		channel.mu.Lock()
		cause := channel.retirement.retired.cause
		channel.mu.Unlock()
		return cause
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (channel *CreatedChannel) withOriginalNativeChannelFile(operation func(*os.File) error) error {
	channel.mu.Lock()
	if channel.self != channel {
		channel.mu.Unlock()
		return errors.New("ppp: original channel owner was copied")
	}
	if channel.closed {
		channel.mu.Unlock()
		return os.ErrClosed
	}
	if channel.file == nil {
		channel.mu.Unlock()
		return os.ErrClosed
	}
	file := channel.file
	channel.retirement.borrows++
	channel.mu.Unlock()
	defer func() {
		channel.mu.Lock()
		channel.retirement.borrows--
		channel.publishNativeChannelRetirementLocked()
		channel.mu.Unlock()
	}()
	return operation(file)
}

func (channel *CreatedChannel) finishNativeChannelFileClose(cause error) {
	channel.mu.Lock()
	defer channel.mu.Unlock()
	channel.retirement.fileClose = &nativePPPChannelCloseResult{cause: cause}
	channel.publishNativeChannelRetirementLocked()
}

func (channel *CreatedChannel) publishNativeChannelRetirementLocked() {
	if channel.retirement.retired != nil {
		return
	}
	if channel.retirement.fileClose == nil {
		return
	}
	if channel.retirement.borrows != 0 {
		return
	}
	channel.retirement.retired = channel.retirement.fileClose
	if channel.retirement.done == nil {
		channel.retirement.done = make(chan struct{})
	}
	close(channel.retirement.done)
}
