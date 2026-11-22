// Design: docs/research/l2tpv2-ze-integration.md -- consuming captured channel/unit owners

package kernel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// Private methods seal these capabilities to the actual native producers.
// Internal tests construct private logical owners and make no native claim.
type sessionUnitSource interface {
	claimSession() (sessionUnitLease, error)
}
type sessionChannelSource interface {
	claimSessionChannel() (sessionChannelLease, error)
}

// Implementations are comparable pointer owners, never value containers.
type sessionUnitLease interface {
	describe() (sessionUnitDescription, error)
	descriptor() int
	wrap() (io.ReadWriteCloser, error)
	Counters() (ppp.SessionCounters, error)
	closeStream(io.ReadWriteCloser) error
	aliasesChannel(int) bool
}

type sessionChannelLease interface {
	describeChannel() (int, error)
	descriptor() int
	wrapChannel() (io.ReadWriteCloser, error)
	closeStream(io.ReadWriteCloser) error
	namespaceProof() sessionNamespaceProof
}

type sessionUnitDescription struct {
	identity  ppp.SessionIdentity
	fd        int
	number    int
	backend   InterfaceOperations
	mutations NativeUnitOperations
	namespace sessionNamespaceProof
}

type claimedCreatedUnit struct {
	unit          *CreatedUnit
	file          *os.File
	fd            int
	descriptorErr error
}

// nativeLinkCreationRejection is the actual native producer's receipt type.
type nativeLinkCreationRejection interface {
	error
	NativeLinkCreationRejected() bool
}

// CreationRejected reports only the actual native producer's terminal negative
// ACK receipt. Generic errno, attempted submission, timeout or missing ECHO
// never establishes it. It remains available after exact-file Close.
func (unit *CreatedUnit) CreationRejected() bool {
	if unit == nil {
		return false
	}
	if unit.creationRejection == nil {
		return false
	}
	return unit.creationRejection.NativeLinkCreationRejected()
}

func originalFileDescriptor(file *os.File) (int, error) {
	if file == nil {
		return -1, errors.New("ppp: no original file")
	}
	connection, err := file.SyscallConn()
	if err != nil {
		return -1, err
	}
	fd := -1
	if err := connection.Control(func(value uintptr) { fd = int(value) }); err != nil {
		return -1, err
	}
	return fd, nil
}

// SessionConfig snapshots labels from TWO actual producer-owned files.
// It neither claims nor reconstructs either file. On error both owners remain
// with the caller. NewSession consumes every newly supplied owner on every
// outcome; replayed/copy owners contain no new transferable resource.
func (unit *CreatedUnit) SessionConfig(channel *CreatedChannel) (SessionConfig, error) {
	if unit == nil {
		return SessionConfig{}, errors.New("ppp: session configuration requires original channel and unit owners")
	}
	if unit.self != unit {
		return SessionConfig{}, errors.New("ppp: session configuration requires original channel and unit owners")
	}
	if channel == nil {
		return SessionConfig{}, errors.New("ppp: session configuration requires original channel and unit owners")
	}
	if channel.self != channel {
		return SessionConfig{}, errors.New("ppp: session configuration requires original channel and unit owners")
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if unit.closed {
		return SessionConfig{}, errors.New("ppp: original unit is unconfirmed, closed or consumed")
	}
	if unit.sessionClaim != nil {
		return SessionConfig{}, errors.New("ppp: original unit is unconfirmed, closed or consumed")
	}
	if !unit.ready {
		return SessionConfig{}, errors.New("ppp: original unit is unconfirmed, closed or consumed")
	}
	if unit.file == nil {
		return SessionConfig{}, errors.New("ppp: original unit is unconfirmed, closed or consumed")
	}
	if nilSessionDependency(unit.backend) {
		return SessionConfig{}, errors.New("ppp: original unit is unconfirmed, closed or consumed")
	}
	if channel.closed {
		return SessionConfig{}, errors.New("ppp: original channel is unconfirmed, closed or consumed")
	}
	if channel.sessionClaim != nil {
		return SessionConfig{}, errors.New("ppp: original channel is unconfirmed, closed or consumed")
	}
	if !channel.ready {
		return SessionConfig{}, errors.New("ppp: original channel is unconfirmed, closed or consumed")
	}
	if channel.file == nil {
		return SessionConfig{}, errors.New("ppp: original channel is unconfirmed, closed or consumed")
	}
	if err := validateSessionNamespace(channel.namespace, unit.namespace); err != nil {
		return SessionConfig{}, err
	}
	channelFD, err := originalFileDescriptor(channel.file)
	if err != nil {
		return SessionConfig{}, err
	}
	unitFD, err := originalFileDescriptor(unit.file)
	if err != nil {
		return SessionConfig{}, err
	}
	return SessionConfig{ChanFD: channelFD, UnitFD: unitFD, UnitNum: unit.number,
		InterfaceName: unit.identity.InterfaceName, Channel: channel, Unit: unit, Backend: unit.backend}, nil
}

func (unit *CreatedUnit) claimSession() (sessionUnitLease, error) {
	if unit == nil {
		return nil, errors.New("ppp: copied or unminted unit cannot transfer custody")
	}
	if unit.self != unit {
		return nil, errors.New("ppp: copied or unminted unit cannot transfer custody")
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.sessionClaim != nil {
		return nil, errors.New("ppp: original unit already consumed")
	}
	fd, err := originalFileDescriptor(unit.file)
	lease := &claimedCreatedUnit{unit: unit, file: unit.file, fd: fd, descriptorErr: err}
	unit.sessionClaim = lease
	return lease, nil
}

func (lease *claimedCreatedUnit) describe() (sessionUnitDescription, error) {
	unit := lease.unit
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.closed {
		return sessionUnitDescription{}, os.ErrClosed
	}
	if !unit.ready {
		return sessionUnitDescription{}, errors.New("ppp: original unit creation is not confirmed")
	}
	if lease.file == nil {
		return sessionUnitDescription{}, errors.New("ppp: original unit creation is not confirmed")
	}
	if unit.file != lease.file {
		return sessionUnitDescription{}, errors.New("ppp: original unit creation is not confirmed")
	}
	if unit.readCounters == nil {
		return sessionUnitDescription{}, errors.New("ppp: original unit creation is not confirmed")
	}
	if unit.poll == nil {
		return sessionUnitDescription{}, errors.New("ppp: original unit creation is not confirmed")
	}
	if lease.descriptorErr != nil {
		return sessionUnitDescription{}, lease.descriptorErr
	}
	fd, err := originalFileDescriptor(lease.file)
	if err != nil {
		return sessionUnitDescription{}, err
	}
	if fd != lease.fd {
		return sessionUnitDescription{}, errors.New("ppp: original unit file identity changed")
	}
	if err := unit.poll(lease.file); err != nil {
		return sessionUnitDescription{}, err
	}
	var mutations NativeUnitOperations
	if unit.nativeOperations != nil {
		mutations, err = unit.nativeOperationsLocked()
		if err != nil {
			return sessionUnitDescription{}, err
		}
	}
	// Original descriptor admission alone never grants native mutation.
	return sessionUnitDescription{identity: unit.identity, fd: fd, number: unit.number, backend: unit.backend, mutations: mutations, namespace: unit.namespace}, nil
}

func (lease *claimedCreatedUnit) descriptor() int            { return lease.fd }
func (lease *claimedCreatedUnit) aliasesChannel(fd int) bool { return fd >= 0 && fd == lease.fd }
func (lease *claimedCreatedUnit) wrap() (io.ReadWriteCloser, error) {
	return newInterruptibleCreatedUnit(lease.unit, lease.file)
}
func (lease *claimedCreatedUnit) Counters() (ppp.SessionCounters, error) {
	return lease.unit.Counters()
}
func (lease *claimedCreatedUnit) closeStream(stream io.ReadWriteCloser) error {
	return errors.Join(closeSessionStream(stream, -1, nil), lease.unit.Close())
}

type ownedSessionUnitStream struct {
	io.ReadWriteCloser
	lease     sessionUnitLease
	closeUnit func(sessionUnitLease, io.ReadWriteCloser) error
}

func (stream *ownedSessionUnitStream) Close() error {
	return closeOwnedSessionUnit(stream.lease, stream.ReadWriteCloser, stream.closeUnit)
}
func wrapClaimedSessionUnit(lease sessionUnitLease) (io.ReadWriteCloser, error) { return lease.wrap() }
func closeClaimedSessionUnit(lease sessionUnitLease, stream io.ReadWriteCloser) error {
	return lease.closeStream(stream)
}

func guardSessionStream(stream io.ReadWriteCloser) io.ReadWriteCloser {
	if nilSessionDependency(stream) {
		return nil
	}
	return &singleCloseSessionStream{ReadWriteCloser: stream}
}

func closeOriginalSessionFile(closeFile func() error) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = sessionPanicFailure(failure)
		}
	}()
	return closeFile()
}

func closeOwnedSessionUnit(lease sessionUnitLease, stream io.ReadWriteCloser, closer func(sessionUnitLease, io.ReadWriteCloser) error) (err error) {
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

type singleCloseSessionStream struct {
	io.ReadWriteCloser
	once sync.Once
	err  error
}

func (stream *singleCloseSessionStream) Close() error {
	stream.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				stream.err = sessionPanicFailure(failure)
			}
		}()
		stream.err = stream.ReadWriteCloser.Close()
	})
	return stream.err
}

func validateSessionPreflight(cfg SessionConfig, ops sessionOps) error {
	if cfg.ChanFD < 0 {
		return errors.New("ppp: invalid channel or unit labels")
	}
	if cfg.UnitFD < 0 {
		return errors.New("ppp: invalid channel or unit labels")
	}
	if cfg.UnitNum < 0 {
		return errors.New("ppp: invalid channel or unit labels")
	}
	if cfg.ChanFD == cfg.UnitFD {
		return errors.New("ppp: invalid channel or unit labels")
	}
	if nilSessionDependency(cfg.Backend) {
		return errors.New("ppp: missing kernel interface operations")
	}
	if ops.setIPv4Mode == nil {
		return errors.New("ppp: missing kernel family mode operations")
	}
	if ops.setIPv6Mode == nil {
		return errors.New("ppp: missing kernel family mode operations")
	}
	if ops.wrapChannel == nil {
		return errors.New("ppp: missing original-file stream wrappers")
	}
	if ops.wrapUnit == nil {
		return errors.New("ppp: missing original-file stream wrappers")
	}
	return nil
}

// Both independent claims are attempted before reporting either failure.
// Even an internal claim panic cannot strand the other newly supplied owner.
func claimSessionUnit(source sessionUnitSource) (lease sessionUnitLease, failure any, err error) {
	defer func() { failure = recover() }()
	if nilSessionDependency(source) {
		return nil, nil, errors.New("ppp: no captured unit owner")
	}
	lease, err = source.claimSession()
	if nilSessionDependency(lease) {
		lease = nil
		if err == nil {
			err = errors.New("ppp: unit supplied no owned lease")
		}
	}
	return lease, nil, err
}
func claimSessionChannel(source sessionChannelSource) (lease sessionChannelLease, failure any, err error) {
	defer func() { failure = recover() }()
	if nilSessionDependency(source) {
		return nil, nil, errors.New("ppp: no captured channel owner")
	}
	lease, err = source.claimSessionChannel()
	if nilSessionDependency(lease) {
		lease = nil
		if err == nil {
			err = errors.New("ppp: channel supplied no owned lease")
		}
	}
	return lease, nil, err
}

func sessionClaimPanic(channel, unit any) any {
	if channel == nil {
		return unit
	}
	if unit == nil {
		return channel
	}
	return fmt.Errorf("ppp: both resource claims panicked: %w", errors.Join(sessionPanicFailure(channel), sessionPanicFailure(unit)))
}

// Append ONLY distinctSessionChannel to session_unit_owned.go.
func distinctSessionChannel(channel sessionChannelLease, unit sessionUnitLease) bool {
	if channel == nil {
		return false
	}
	if unit == nil {
		return true
	}
	return !unit.aliasesChannel(channel.descriptor())
}
