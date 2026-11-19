// Design: docs/research/l2tpv2-ze-integration.md -- delayed kernel client pair.
// Related: client_linux.go -- original-file ordered I/O and atomic attachment.

package kernel

import (
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// ClientSession owns a channel and unit, with no readers or workers. Its zero,
// nil and copied values cannot operate. NewClientSession alone admits one.
// Safe for concurrent use. The sole frameReader MUST retry ReadPacket after
// EAGAIN/EINTR and release sessionLink.mu between attempts. The caller MUST
// serialize protocol operations with that lock and MUST close the channel,
// stop/join all consumers, then close the unit and its independent backend.
type ClientSession struct {
	self  *ClientSession
	pair  *claimedSessionPair
	ops   clientPairOps
	input clientInput
}

// Actual Connect creates only attachment; the old client's subsequent
// successful receive-MRU operation is also required before unit input opens.
type clientInput interface {
	readUnit() bool
	attached() bool
}
type clientHandshakeInput struct{}
type clientAttachedInput struct{}
type clientConnectedInput struct{}

func (clientHandshakeInput) readUnit() bool { return false }
func (clientAttachedInput) readUnit() bool  { return false }
func (clientConnectedInput) readUnit() bool { return true }
func (clientHandshakeInput) attached() bool { return false }
func (clientAttachedInput) attached() bool  { return true }
func (clientConnectedInput) attached() bool { return true }

type clientUnitRequest interface{ clientUnitRequest() }
type clientMRU struct{ mru uint16 }
type clientIPv4Mode struct{ mode NPMode }
type clientIPv6Mode struct{ mode NPMode }

func (clientMRU) clientUnitRequest()      {}
func (clientIPv4Mode) clientUnitRequest() {}
func (clientIPv6Mode) clientUnitRequest() {}

// Private logical test operations do not implement any public owner factory.
type clientPairOps struct {
	prepare    func(*claimedSessionPair) error
	connect    func(*claimedSessionPair) error
	disconnect func(*claimedSessionPair) error
	read       func(*claimedSessionPair, bool, []byte) (int, error)
	write      func(*claimedSessionPair, []byte) (int, error)
	unit       func(*claimedSessionPair, clientUnitRequest) error
}

// NewClientSession consumes EVERY newly supplied owner on EVERY outcome,
// including panic/refusal. The caller MUST NOT close transferred owners. It
// does not connect, change family modes, negotiate PPP or start a reader.
func NewClientSession(cfg SessionConfig) (*ClientSession, error) {
	return newClientSession(cfg, productionClientPairOps(), sessionPairClosers{})
}

func newClientSession(cfg SessionConfig, ops clientPairOps, closers sessionPairClosers) (client *ClientSession, err error) {
	pair, err := newClaimedSessionPair(cfg, nil, ops.prepare, closers)
	if err != nil {
		return nil, err
	}
	transferred := false
	defer pair.finishConstruction(&transferred, &err)
	if ops.prepare == nil {
		return nil, errors.New("ppp: client has no original-file preparation")
	}
	if ops.connect == nil {
		return nil, errors.New("ppp: client has no original-pair attachment")
	}
	if ops.disconnect == nil {
		return nil, errors.New("ppp: client has no original-pair disconnect")
	}
	if ops.read == nil {
		return nil, errors.New("ppp: client has no ordered original-file reader")
	}
	if ops.write == nil {
		return nil, errors.New("ppp: client has no original-file writer")
	}
	if ops.unit == nil {
		return nil, errors.New("ppp: client has no original-unit operations")
	}
	client = &ClientSession{pair: pair, ops: ops, input: clientHandshakeInput{}}
	client.self = client
	transferred = true
	return client, nil
}

func (client *ClientSession) valid() bool {
	if client == nil {
		return false
	}
	if client.self != client {
		return false
	}
	return client.pair != nil
}

// UnitNumber and Identity are immutable labels, never resource authority.
func (client *ClientSession) UnitNumber() int {
	if !client.valid() {
		return -1
	}
	return client.pair.info.number
}
func (client *ClientSession) Identity() ppp.SessionIdentity {
	if !client.valid() {
		return ppp.SessionIdentity{}
	}
	return client.pair.info.identity
}

// ReadPacket makes ONE channel-first bounded attempt. It returns EAGAIN when
// idle after at most100ms of polling; the caller owns retry and lock release.
func (client *ClientSession) ReadPacket(buffer []byte) (int, error) {
	if !client.valid() {
		return 0, io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return 0, io.ErrClosedPipe
	}
	if pair.unitClosed {
		return 0, io.ErrClosedPipe
	}
	if len(buffer) == 0 {
		return 0, io.ErrShortBuffer
	}
	return client.ops.read(pair, client.input.readUnit(), buffer)
}

// WritePacket writes one channel frame with the original file semantics.
func (client *ClientSession) WritePacket(buffer []byte) (int, error) {
	if !client.valid() {
		return 0, io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	if pair.channelClosed {
		pair.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	if pair.unitClosed {
		pair.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	pair.mu.Unlock()
	// The native writer rechecks admission, then uses os.File.Write's own
	// lifetime lease outside owner locks. Close must be able to wake a write
	// waiting for readiness. The caller still holds sessionLink.mu for PADT.
	return client.ops.write(pair, buffer)
}

// Write implements the existing CPE transport writer contract.
func (client *ClientSession) Write(buffer []byte) (int, error) { return client.WritePacket(buffer) }

// Connect MUST be called only after the existing client's successful IPCP.
// Failure retains both owners and leaves unit input disabled. Success retains
// attachment, but unit reads wait for successful SetReceiveMRU too. No numeric
// fallback is permitted, including when the running kernel returns ENOTTY.
func (client *ClientSession) Connect() error {
	if !client.valid() {
		return io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return io.ErrClosedPipe
	}
	if pair.unitClosed {
		return io.ErrClosedPipe
	}
	if client.input.attached() {
		return nil
	}
	if err := client.ops.connect(pair); err != nil {
		return err
	}
	client.input = clientAttachedInput{}
	return nil
}

func (client *ClientSession) SetReceiveMRU(mru uint16) error {
	return client.unitOperation(clientMRU{mru: mru})
}
func (client *ClientSession) SetIPv4Mode(mode NPMode) error {
	return client.unitOperation(clientIPv4Mode{mode: mode})
}
func (client *ClientSession) SetIPv6Mode(mode NPMode) error {
	return client.unitOperation(clientIPv6Mode{mode: mode})
}

func (client *ClientSession) unitOperation(request clientUnitRequest) error {
	if !client.valid() {
		return io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return io.ErrClosedPipe
	}
	if pair.unitClosed {
		return io.ErrClosedPipe
	}
	// Closed refusal precedes request validation in the native implementation.
	if err := client.ops.unit(pair, request); err != nil {
		return err
	}
	if _, mru := request.(clientMRU); mru {
		if client.input.attached() {
			client.input = clientConnectedInput{}
		}
	}
	return nil
}

// CloseChannel disconnects the same attached original pair while BOTH files
// remain live, then physically closes the channel to stop writes/read attempts.
// Every disconnect/close cause is retained. The caller MUST join every consumer
// and sample the original counters before CloseUnit, then release its backend.
func (client *ClientSession) CloseChannel() error {
	if !client.valid() {
		return io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if pair.channelClosed {
		return pair.channelErr
	}
	var disconnectErr error
	if client.input.attached() {
		disconnectErr = closeOriginalSessionFile(func() error { return client.ops.disconnect(pair) })
	}
	closeErr := pair.closeChannel()
	pair.channelErr = errors.Join(disconnectErr, closeErr)
	return pair.channelErr
}

// CloseUnit MUST follow CloseChannel and every consumer's join. Worker joins
// belong to the caller; the pair never starts or silently joins another reader.
func (client *ClientSession) CloseUnit() error {
	if !client.valid() {
		return io.ErrClosedPipe
	}
	pair := client.pair
	pair.mu.Lock()
	defer pair.mu.Unlock()
	if !pair.channelClosed {
		return errors.New("ppp: close the client channel and join its consumers before releasing its unit")
	}
	return pair.closeUnit()
}

// Close is for constructor cleanup or a caller that has already stopped and
// joined its workers. Active CPE shutdown MUST use the two split operations.
func (client *ClientSession) Close() error {
	if !client.valid() {
		return io.ErrClosedPipe
	}
	return client.pair.closeAll()
}
