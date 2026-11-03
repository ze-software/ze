//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- packet sender admission.
// Related: peer_linux.go -- shared session envelope encoder.

package pppoeac

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// TestPacketAdmission checks that invalid handles and envelope lengths are
// refused before socket I/O. Exact errors distinguish admission from EBADF.
func TestPacketAdmission(t *testing.T) {
	for _, input := range []struct {
		name    string
		peer    *Peer
		remote  []byte
		payload []byte
		want    string
	}{
		{name: "nil", want: "wire peer is nil"},
		{name: "zero", peer: &Peer{}, want: "wire peer is uninitialized"},
		{name: "destination", peer: &Peer{initialized: true, fd: -1}, remote: make([]byte, 5), want: "six-octet destination"},
		{name: "oversize", peer: &Peer{initialized: true, fd: -1}, remote: make([]byte, 6), payload: make([]byte, pppoe.EthMaxLen-21), want: "exceeds the Ethernet buffer"},
	} {
		t.Run(input.name, func(t *testing.T) {
			err := input.peer.sendPacket(input.remote, 0x21, input.payload)
			if err == nil {
				t.Fatal("invalid packet admitted")
			}
			if !strings.Contains(err.Error(), input.want) {
				t.Fatalf("error = %v, want %q", err, input.want)
			}
		})
	}
	var session *Session
	if err := session.SendPacket(0x21, nil); err == nil {
		t.Fatal("nil session sent packet")
	}
	if _, _, err := session.ReadPacket(t.Context()); err == nil {
		t.Fatal("nil session read packet")
	}
}
