//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- malformed peer frames.
// Related: script_linux.go -- ReadPacket envelope validation.

package pppoeac

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestPacketWireAdmission sends malformed envelopes through AF_PACKET, then
// sends a valid envelope to prove the receiver still reads the same session.
// The test owns both sockets synchronously; no reader goroutine is needed.
func TestPacketWireAdmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := OpenVeth(t, "zec0", "zea0", 44)
	link, err := netlink.LinkByName("zec0")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(NetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	})
	var remote [6]byte
	copy(remote[:], link.Attrs().HardwareAddr)
	sender, err := New(fd, link.Attrs().Index, remote, 44)
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{peer: peer, remote: remote}
	for _, input := range []struct {
		name   string
		offset int
		value  byte
		want   string
	}{
		{name: "session", offset: 17, value: 45, want: "session ID"},
		{name: "version", offset: 14, value: 0x21, want: "version/type"},
		{name: "code", offset: 15, value: 1, want: "session code"},
		{name: "length", offset: 18, value: 1, want: "truncated"},
	} {
		t.Run(input.name, func(t *testing.T) {
			// Deliberately raw: the valid shared encoder cannot emit these faults.
			var frame [60]byte
			copy(frame[:6], peer.mac[:])
			copy(frame[6:12], remote[:])
			binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_PPP_SES)
			frame[14], frame[17], frame[19] = 0x11, 44, 3
			frame[21], frame[22] = 0x21, 0x45
			frame[input.offset] = input.value
			if err := sender.SendFrame(frame[:]); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if _, _, err := session.ReadPacket(ctx); err == nil {
				t.Fatal("malformed envelope admitted")
			} else if !strings.Contains(err.Error(), input.want) {
				t.Fatalf("error = %v, want %q", err, input.want)
			}
			if err := sender.sendPacket(peer.mac[:], 0x21, []byte{0x45}); err != nil {
				t.Fatal(err)
			}
			proto, payload, err := session.ReadPacket(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if proto != 0x21 {
				t.Fatalf("positive protocol = %x", proto)
			}
			if !bytes.Equal(payload, []byte{0x45}) {
				t.Fatalf("positive payload = %x", payload)
			}
		})
	}
}
