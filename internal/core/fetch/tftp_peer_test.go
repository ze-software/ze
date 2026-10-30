// Design: docs/architecture/appliance/on-device-installer.md -- the tftp scheme's test server
// Related: tftp.go -- the client these scripts drive

package fetch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// tftpPeer is the server side of one scripted transfer: it holds the read
// request the client sent and a socket on a fresh port, the server's TID.
// Its methods report a mismatch with t.Errorf, so a script runs to its end
// and the test sees every difference.
type tftpPeer struct {
	t        *testing.T
	conn     *net.UDPConn
	listener *net.UDPConn // the port the read request went to
	client   *net.UDPAddr
	rrq      []byte
}

// startTFTPScript listens on loopback for one read request and plays script
// as the server. It returns the target to fetch and a wait function that
// returns once the script ended.
func startTFTPScript(t *testing.T, script func(p *tftpPeer)) (tftpTarget, func()) {
	t.Helper()
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 600)
		if err := listener.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Errorf("listener deadline: %v", err)
			return
		}
		n, client, err := listener.ReadFromUDP(buf)
		if err != nil {
			t.Errorf("no read request reached the scripted server: %v", err)
			return
		}
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Errorf("transfer socket: %v", err)
			return
		}
		defer conn.Close() //nolint:errcheck // test cleanup
		script(&tftpPeer{t: t, conn: conn, listener: listener, client: client, rrq: append([]byte(nil), buf[:n]...)})
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Logf("close the scripted listener: %v", err)
		}
		<-done
	})
	addr, ok := listener.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("a UDP listener without a UDP address")
	}
	port := addr.Port
	return tftpTarget{host: "127.0.0.1", port: uint16(port), file: "f.bin"}, func() { <-done }
}

// shortenTFTP sets the client's timers for a test: a one-second timeout, a
// short dally and no retry delay. expectACK waits half a timeout, so an ACK
// the client only sends again after a timeout never satisfies it: what the
// script sees is what the client sent in answer to the packet before.
func shortenTFTP(t *testing.T) {
	t.Helper()
	timeout, dally, delay := tftpTimeout, tftpDally, retryDelay
	tftpTimeout, tftpDally, retryDelay = time.Second, 150*time.Millisecond, time.Millisecond
	t.Cleanup(func() { tftpTimeout, tftpDally, retryDelay = timeout, dally, delay })
}

// fetchTFTP runs one transfer attempt of target into a temporary file and
// returns the file's bytes and the attempt's error.
func fetchTFTP(t *testing.T, target tftpTarget) ([]byte, error) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "out")
	getErr := tftpGet(target, dest)
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read the fetched file: %v", err)
	}
	return got, getErr
}

func (p *tftpPeer) send(pkt []byte) {
	if _, err := p.conn.WriteToUDP(pkt, p.client); err != nil {
		p.t.Errorf("send to the client: %v", err)
	}
}

func (p *tftpPeer) data(block uint16, payload []byte) {
	pkt := binary.BigEndian.AppendUint16(nil, tftp.OpDATA)
	pkt = binary.BigEndian.AppendUint16(pkt, block)
	p.send(append(pkt, payload...))
}

// oack sends an OACK carrying the name and value pairs in order.
func (p *tftpPeer) oack(pairs ...string) {
	pkt := binary.BigEndian.AppendUint16(nil, tftp.OpOACK)
	for _, field := range pairs {
		pkt = append(pkt, field...)
		pkt = append(pkt, 0)
	}
	p.send(pkt)
}

// read returns the next packet from the client, or ok false after wait.
func (p *tftpPeer) read(wait time.Duration) (pkt []byte, ok bool) {
	buf := make([]byte, 600)
	if err := p.conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		p.t.Errorf("deadline: %v", err)
		return nil, false
	}
	n, from, err := p.conn.ReadFromUDP(buf)
	if err != nil {
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			p.t.Errorf("read from the client: %v", err)
		}
		return nil, false
	}
	// RFC 1350 Section 4: the client keeps the TID it sent the request from
	// for the whole transfer.
	if !from.IP.Equal(p.client.IP) || from.Port != p.client.Port {
		p.t.Errorf("client packet from %s, want the TID of its request %s", from, p.client)
	}
	return buf[:n], true
}

// expectACK requires the client's next packet to be the ACK of block.
func (p *tftpPeer) expectACK(block uint16) bool {
	pkt, ok := p.read(tftpTimeout / 2)
	if !ok {
		p.t.Errorf("no ACK %d from the client", block)
		return false
	}
	want := binary.BigEndian.AppendUint16(nil, tftp.OpACK)
	want = binary.BigEndian.AppendUint16(want, block)
	if !bytes.Equal(pkt, want) {
		p.t.Errorf("client sent % x, want ACK %d (% x)", pkt, block, want)
		return false
	}
	return true
}

// expectError requires the client's next packet to be an ERROR with code.
func (p *tftpPeer) expectError(code uint16) {
	pkt, ok := p.read(2 * time.Second)
	if !ok {
		p.t.Errorf("no ERROR %d from the client", code)
		return
	}
	if len(pkt) < tftp.HeaderOctets || binary.BigEndian.Uint16(pkt[:2]) != tftp.OpERROR {
		p.t.Errorf("client sent % x, want ERROR %d", pkt, code)
		return
	}
	if got := binary.BigEndian.Uint16(pkt[2:4]); got != code {
		p.t.Errorf("client sent ERROR %d (%q), want ERROR %d", got, pkt[4:], code)
	}
}

// expectSilence requires the client to send nothing for wait.
func (p *tftpPeer) expectSilence(wait time.Duration, why string) {
	if pkt, ok := p.read(wait); ok {
		p.t.Errorf("client sent % x, want nothing: %s", pkt, why)
	}
}

// block returns n bytes of a pattern that differs per block, so a block
// written twice or out of order changes the file.
func block(seq, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(seq*7 + i)
	}
	return out
}
