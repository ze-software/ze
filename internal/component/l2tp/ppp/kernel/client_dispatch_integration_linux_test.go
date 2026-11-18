//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- one ordered read attempt.
// Related: client_linux.go -- actual descriptor algorithm; CPE frameReader owns dispatch.

package kernel

import (
	"bytes"
	"testing"

	"golang.org/x/sys/unix"
)

// The former CPE descriptor question follows its actual producer into kernel.
// The CPE companion retains the frameReader's received() dispatch boundary.
// Real CHAP/IPCP socket packets prove channel priority, no unit read-ahead and
// exact useful second delivery, without manufacturing a public PPP owner.
func TestOrderedDescriptorDispatch(t *testing.T) {
	control := clientReadSocket(t, unix.SOCK_DGRAM)
	unit := clientReadSocket(t, unix.SOCK_DGRAM)
	controlPacket := []byte{0xc2, 0x23, 3, 63, 0, 4}
	unitPacket := []byte{0x80, 0x21, 1, 71, 0, 10, 3, 6, 192, 0, 2, 1}
	clientReadSend(t, unit[1], unitPacket)
	clientReadSend(t, control[1], controlPacket)
	var buffer [128]byte
	n, err := rawOrderedClientAttempt(control[0], unit[0], true, buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:n], controlPacket) {
		t.Fatalf("channel did not precede unit: %x", buffer[:n])
	}
	var peek [64]byte
	n, _, err = unix.Recvfrom(unit[0], peek[:], unix.MSG_PEEK|unix.MSG_DONTWAIT)
	if err != nil {
		t.Fatalf("unit read ahead before dispatch: %d %v", n, err)
	}
	if !bytes.Equal(peek[:n], unitPacket) {
		t.Fatalf("unit read ahead before dispatch: %x", peek[:n])
	}
	n, err = rawOrderedClientAttempt(control[0], unit[0], true, buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer[:n], unitPacket) {
		t.Fatalf("unit packet lost: %x", buffer[:n])
	}
}
