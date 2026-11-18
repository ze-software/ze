// Design: docs/research/l2tpv2-ze-integration.md -- original attached channel custody

package kernel

import (
	"errors"
	"io"
	"os"
	"sync"
)

// CreatedChannel owns the original /dev/ppp file attached by CreatePPPChannel.
// A nonnil result owns that file even on error and MUST be closed. It never
// acquires custody from a descriptor label at admission or replay. Native
// operations MUST borrow WithDescriptor; callers MUST NOT close the raw FD.
// Safe for concurrent use. NewSession consumes this owner once on every outcome.
type CreatedChannel struct {
	mu           sync.Mutex
	closeMu      sync.Mutex
	self         *CreatedChannel
	file         *os.File
	ready        bool
	closed       bool
	closeErr     error
	sessionClaim *claimedCreatedChannel
	namespace    *capturedSessionNamespace
	poll         func(*os.File) error
}

func (channel *CreatedChannel) WithDescriptor(operation func(int) error) error {
	if channel == nil {
		return errors.New("ppp: missing original channel operation")
	}
	if operation == nil {
		return errors.New("ppp: missing original channel operation")
	}
	channel.mu.Lock()
	if channel.closed {
		channel.mu.Unlock()
		return os.ErrClosed
	}
	if channel.file == nil {
		channel.mu.Unlock()
		return os.ErrClosed
	}
	file := channel.file
	channel.mu.Unlock()
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var operationErr error
	if err := connection.Control(func(fd uintptr) { operationErr = operation(int(fd)) }); err != nil {
		return err
	}
	return operationErr
}

func (channel *CreatedChannel) Close() error {
	if channel == nil {
		return nil
	}
	channel.closeMu.Lock()
	defer channel.closeMu.Unlock()
	channel.mu.Lock()
	if channel.closed {
		err := channel.closeErr
		channel.mu.Unlock()
		return err
	}
	channel.closed = true
	file := channel.file
	namespace := channel.namespace
	channel.mu.Unlock()
	var err error
	if file != nil {
		err = file.Close()
	}
	err = errors.Join(err, namespace.Close())
	channel.mu.Lock()
	channel.closeErr = err
	channel.mu.Unlock()
	return err
}

type claimedCreatedChannel struct {
	channel       *CreatedChannel
	file          *os.File
	fd            int
	descriptorErr error
}

func (channel *CreatedChannel) claimSessionChannel() (sessionChannelLease, error) {
	if channel == nil {
		return nil, errors.New("ppp: copied or unminted channel cannot transfer custody")
	}
	if channel.self != channel {
		return nil, errors.New("ppp: copied or unminted channel cannot transfer custody")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.sessionClaim != nil {
		return nil, errors.New("ppp: original channel already consumed")
	}
	fd, err := originalFileDescriptor(channel.file)
	lease := &claimedCreatedChannel{channel: channel, file: channel.file, fd: fd, descriptorErr: err}
	channel.sessionClaim = lease
	return lease, nil
}

func (lease *claimedCreatedChannel) descriptor() int { return lease.fd }
func (lease *claimedCreatedChannel) describeChannel() (int, error) {
	channel := lease.channel
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.closed {
		return -1, os.ErrClosed
	}
	if !channel.ready {
		return -1, errors.New("ppp: original channel attachment is unconfirmed")
	}
	if lease.file == nil {
		return -1, errors.New("ppp: original channel attachment is unconfirmed")
	}
	if channel.file != lease.file {
		return -1, errors.New("ppp: original channel attachment is unconfirmed")
	}
	if channel.poll == nil {
		return -1, errors.New("ppp: original channel attachment is unconfirmed")
	}
	if lease.descriptorErr != nil {
		return -1, lease.descriptorErr
	}
	fd, err := originalFileDescriptor(lease.file)
	if err != nil {
		return -1, err
	}
	if fd != lease.fd {
		return -1, errors.New("ppp: original channel file identity changed")
	}
	if err := channel.poll(lease.file); err != nil {
		return -1, err
	}
	return fd, nil
}
func (lease *claimedCreatedChannel) namespaceProof() sessionNamespaceProof {
	return lease.channel.namespace
}
func (lease *claimedCreatedChannel) wrapChannel() (io.ReadWriteCloser, error) {
	return newInterruptibleSessionFile(lease.file, "ppp.chan", lease.channel.Close)
}
func (lease *claimedCreatedChannel) closeStream(stream io.ReadWriteCloser) error {
	return errors.Join(closeSessionStream(stream, -1, nil), lease.channel.Close())
}

type ownedSessionChannelStream struct {
	io.ReadWriteCloser
	lease  sessionChannelLease
	closer func(sessionChannelLease, io.ReadWriteCloser) error
}

func (stream *ownedSessionChannelStream) Close() error {
	return closeOwnedSessionChannel(stream.lease, stream.ReadWriteCloser, stream.closer)
}
func (stream *ownedSessionChannelStream) InterruptRead() error {
	interrupter, ok := stream.ReadWriteCloser.(readInterrupter)
	if !ok {
		return errors.New("ppp: original channel stream cannot be interrupted")
	}
	return interrupter.InterruptRead()
}
func wrapClaimedSessionChannel(lease sessionChannelLease) (io.ReadWriteCloser, error) {
	return lease.wrapChannel()
}
func closeClaimedSessionChannel(lease sessionChannelLease, stream io.ReadWriteCloser) error {
	return lease.closeStream(stream)
}
func closeOwnedSessionChannel(lease sessionChannelLease, stream io.ReadWriteCloser, closer func(sessionChannelLease, io.ReadWriteCloser) error) (err error) {
	guarded := guardSessionStream(stream)
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(sessionPanicFailure(failure), closeOriginalSessionFile(func() error { return lease.closeStream(guarded) }))
		}
	}()
	if closer == nil {
		return lease.closeStream(guarded)
	}
	return closer(lease, guarded)
}
