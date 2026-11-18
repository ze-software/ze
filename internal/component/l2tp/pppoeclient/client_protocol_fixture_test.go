// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- logical fixture port.
// Related: client_owner.go -- production binds only a real ClientSession.

package pppoeclient

import (
	"io"
	"syscall"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// logicalClientProtocol preserves each old pure transport's reads, exact writes
// and close witnesses. Its default Connect is the old invalid-descriptor stop
// AFTER successful IPCP; it never manufactures a public original-file grant.
type logicalClientProtocol struct {
	transport    io.ReadWriteCloser
	read         func([]byte) (int, error)
	connect      func() error
	mru          func(uint16) error
	ipv4         func(pppkernel.NPMode) error
	ipv6         func(pppkernel.NPMode) error
	channelClose func() error
	unitClose    func() error
}

func testClientProtocol(transport io.ReadWriteCloser) *logicalClientProtocol {
	return &logicalClientProtocol{transport: transport}
}

func (*logicalClientProtocol) UnitNumber() int { return 0 }
func (client *logicalClientProtocol) ReadPacket(buffer []byte) (int, error) {
	if client.read != nil {
		return client.read(buffer)
	}
	return client.transport.Read(buffer)
}
func (client *logicalClientProtocol) WritePacket(buffer []byte) (int, error) {
	return client.transport.Write(buffer)
}
func (client *logicalClientProtocol) Connect() error {
	if client.connect != nil {
		return client.connect()
	}
	return syscall.EBADF
}
func (client *logicalClientProtocol) SetReceiveMRU(mru uint16) error {
	if client.mru != nil {
		return client.mru(mru)
	}
	return syscall.EBADF
}
func (client *logicalClientProtocol) SetIPv4Mode(mode pppkernel.NPMode) error {
	if client.ipv4 != nil {
		return client.ipv4(mode)
	}
	// Legal fixture modes remain logical. Invalid modes use the actual validator
	// before any ioctl. Descriptor -1 cannot grant native resource authority.
	switch mode {
	case pppkernel.NPModePass, pppkernel.NPModeDrop:
		return nil
	case pppkernel.NPModeUnspecified:
		return pppkernel.SetIPv4Mode(-1, mode)
	default: // NPMode accepts arbitrary integers at this fixture boundary.
		return pppkernel.SetIPv4Mode(-1, mode)
	}
}
func (client *logicalClientProtocol) SetIPv6Mode(mode pppkernel.NPMode) error {
	if client.ipv6 != nil {
		return client.ipv6(mode)
	}
	return client.SetIPv4Mode(mode)
}
func (client *logicalClientProtocol) CloseChannel() error {
	if client.channelClose != nil {
		return client.channelClose()
	}
	return client.transport.Close()
}
func (client *logicalClientProtocol) CloseUnit() error {
	if client.unitClose != nil {
		return client.unitClose()
	}
	return nil
}

// testClientTransportClose preserves the old pure side effect while supplying
// the new cause-bearing close contract. It never closes a real PPPoX by label.
func testClientTransportClose(closeTransport func()) func() error {
	return func() error { closeTransport(); return nil }
}
