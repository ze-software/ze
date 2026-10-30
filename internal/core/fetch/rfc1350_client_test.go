// VALIDATES: RFC 1350 (TFTP) on the client side: a packet from another TID
// is answered with ERROR 5 and does not disturb the transfer, the client
// dallies after its final ACK, block numbers wrap past 65535, a silent server
// ends the attempt after bounded retransmissions, and a server ERROR ends the
// transfer with no ACK.
// PREVENTS: a client that writes a stranger's DATA into the file, loses the
// final ACK for good, stops at block 65535, waits forever, or acknowledges an
// ERROR.
package fetch

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RFC requirement: RFC1350-4-4 positive -- DATA from a port other than the server's TID,
// sent in the middle of the transfer, draws ERROR 5 back to that port, and the transfer
// with the server's TID completes.
// RFC requirement: RFC1350-4-4 negative -- the stranger's DATA, numbered as the block the
// client expects next, is not written: the file holds only the server's blocks.
func TestRFC1350ClientRefusesStrangerTID(t *testing.T) {
	shortenTFTP(t)

	first, last := block(1, 512), block(2, 20)
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, first)
		if !p.expectACK(1) {
			return
		}
		stranger, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			p.t.Errorf("stranger socket: %v", err)
			return
		}
		defer stranger.Close() //nolint:errcheck // test cleanup
		pkt := binary.BigEndian.AppendUint16(nil, tftp.OpDATA)
		pkt = binary.BigEndian.AppendUint16(pkt, 2)
		if _, err := stranger.WriteToUDP(append(pkt, "intruder"...), p.client); err != nil {
			p.t.Errorf("stranger send: %v", err)
			return
		}
		buf := make([]byte, 100)
		if err := stranger.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			p.t.Errorf("deadline: %v", err)
			return
		}
		n, _, err := stranger.ReadFromUDP(buf)
		if err != nil {
			p.t.Errorf("the stranger got no answer: %v", err)
			return
		}
		if n < 4 || binary.BigEndian.Uint16(buf[:2]) != tftp.OpERROR || binary.BigEndian.Uint16(buf[2:4]) != tftp.CodeUnknownTID {
			p.t.Errorf("the stranger got % x, want ERROR 5", buf[:n])
		}
		p.data(2, last)
		p.expectACK(2)
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(got, append(append([]byte(nil), first...), last...)) {
		t.Fatalf("fetched %q..., want only the server's blocks", got[min(len(got), 512):])
	}
}

// RFC requirement: RFC1350-6-2 positive -- after the final ACK the client dallies: when the
// server sends the final DATA again, as it does when the final ACK was lost, the client
// acknowledges it again.
// RFC requirement: RFC1350-6-2 negative -- while dallying the client acknowledges only the
// final block: an older block sent again draws no ACK.
func TestRFC1350ClientDallies(t *testing.T) {
	shortenTFTP(t)
	tftpDally = time.Second // the dally must outlast the script below

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, block(1, 512))
		if !p.expectACK(1) {
			return
		}
		p.data(2, block(2, 5))
		if !p.expectACK(2) {
			return
		}
		p.data(1, block(1, 512))
		p.expectSilence(200*time.Millisecond, "an older block draws no ACK while dallying")
		p.data(2, block(2, 5))
		p.expectACK(2)
	})
	if _, err := fetchTFTP(t, target); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	wait()
}

// TestTFTPClientBlockNumberWraps drives the receive state across the 16-bit
// block number: with block 65535 expected, blocks 65535, 0 and 1 are taken
// in order and each is acknowledged by its own number. Driving the state
// directly keeps the test short; a real transfer needs 65537 blocks.
func TestTFTPClientBlockNumberWraps(t *testing.T) {
	shortenTFTP(t)

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close() //nolint:errcheck // test cleanup
	peerConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close() //nolint:errcheck // test cleanup
	peerAddr, ok := peerConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("a UDP socket without a UDP address")
	}

	var out bytes.Buffer
	transfer := newTFTPTransfer(clientConn, peerAddr, &out)
	transfer.peer = peerAddr
	transfer.granted = tftp.Negotiated{Blksize: 8, Windowsize: 1}
	transfer.next = 65535

	ack := make([]byte, 16)
	for i, number := range []uint16{65535, 0, 1} {
		payload := block(i, 8)
		if number == 1 {
			payload = block(i, 3)
		}
		pkt := binary.BigEndian.AppendUint16(nil, tftp.OpDATA)
		pkt = binary.BigEndian.AppendUint16(pkt, number)
		done, err := transfer.data(append(pkt, payload...))
		if err != nil {
			t.Fatalf("block %d: %v", number, err)
		}
		if done != (number == 1) {
			t.Fatalf("block %d: final = %v", number, done)
		}
		if err := peerConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		n, _, err := peerConn.ReadFromUDP(ack)
		if err != nil {
			t.Fatalf("no ACK for block %d: %v", number, err)
		}
		if n != 4 || binary.BigEndian.Uint16(ack[:2]) != tftp.OpACK || binary.BigEndian.Uint16(ack[2:4]) != number {
			t.Fatalf("block %d drew % x, want its ACK", number, ack[:n])
		}
	}
	want := append(append(block(0, 8), block(1, 8)...), block(2, 3)...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("wrote % x, want % x", out.Bytes(), want)
	}
}

// TestTFTPClientSilentServer: a server that never answers ends the attempt
// after the RRQ was sent four times, naming the server.
func TestTFTPClientSilentServer(t *testing.T) {
	shortenTFTP(t)
	tftpTimeout = 150 * time.Millisecond

	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // test cleanup
	addr, ok := listener.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("a UDP listener without a UDP address")
	}
	port := addr.Port
	_, err = fetchTFTP(t, tftpTarget{host: "127.0.0.1", port: uint16(port), file: "f.bin"})
	if err == nil || !strings.Contains(err.Error(), "no answer from 127.0.0.1") {
		t.Fatalf("fetch from a silent server: %v, want no answer", err)
	}
	requests := 0
	buf := make([]byte, 600)
	for {
		if err := listener.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := listener.ReadFromUDP(buf); err != nil {
			break
		}
		requests++
	}
	if requests != tftpRetransmitMax+1 {
		t.Fatalf("the client sent %d read requests, want %d", requests, tftpRetransmitMax+1)
	}
}

// TestTFTPClientStallAfterData: a server that stops after block 1 gets the ACK
// of block 1 again on each timeout, and the attempt fails after the bound.
func TestTFTPClientStallAfterData(t *testing.T) {
	shortenTFTP(t)

	tftpTimeout = 150 * time.Millisecond
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, block(1, 512))
		for sent := range tftpRetransmitMax + 1 {
			pkt, ok := p.read(2 * time.Second)
			if !ok {
				p.t.Errorf("ACK 1 number %d never came", sent+1)
				return
			}
			if !bytes.Equal(pkt, []byte{0, byte(tftp.OpACK), 0, 1}) {
				p.t.Errorf("client sent % x, want ACK 1", pkt)
				return
			}
		}
	})
	_, err := fetchTFTP(t, target)
	wait()
	if err == nil || !strings.Contains(err.Error(), "stopped sending") {
		t.Fatalf("fetch from a stalled server: %v", err)
	}
}

// RFC requirement: RFC1350-2-3 negative -- an ERROR, a packet used for termination, is not
// acknowledged: the client sends nothing in reply, and the fetch fails naming the error's
// code and message.
func TestRFC1350ClientServerError(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.send(tftp.BuildError(tftp.CodeFileNotFound, "file not found"))
		p.expectSilence(300*time.Millisecond, "no ACK answers an ERROR")
	})
	_, err := fetchTFTP(t, target)
	wait()
	if err == nil || !strings.Contains(err.Error(), "TFTP error 1") || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("fetch answered by ERROR 1: %v", err)
	}
}

// TestTFTPClientOversizedBlock: a DATA larger than the negotiated blksize is
// refused with ERROR 4.
func TestTFTPClientOversizedBlock(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.oack(tftp.OptionBlksize, "8")
		if !p.expectACK(0) {
			return
		}
		p.data(1, block(1, 9))
		p.expectError(tftp.CodeIllegalOperation)
	})
	_, err := fetchTFTP(t, target)
	wait()
	if err == nil {
		t.Fatal("the fetch accepted a 9-octet block with blksize 8")
	}
}

// TestTFTPClientResendsACKZero: when the server sends its OACK again, the
// lost ACK 0 is sent again and the transfer goes on.
func TestTFTPClientResendsACKZero(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.oack(tftp.OptionBlksize, "8")
		if !p.expectACK(0) {
			return
		}
		p.oack(tftp.OptionBlksize, "8")
		if !p.expectACK(0) {
			return
		}
		p.data(1, []byte("abc"))
		p.expectACK(1)
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("fetched %q, want abc", got)
	}
}

// RFC requirement: RFC1350-4-3 positive -- the client chooses its own TID and keeps it: every
// packet the server's transfer socket receives comes from the port the read request came
// from (tftpPeer.read checks each one), and the ACKs reach the server's new TID.
// RFC requirement: RFC1350-4-3 negative -- once the server answered from its own TID, the
// client sends nothing more to the port the request went to.
func TestRFC1350ClientKeepsItsTID(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.data(1, block(1, 512))
		if !p.expectACK(1) {
			return
		}
		p.data(2, block(2, 1))
		if !p.expectACK(2) {
			return
		}
		if err := p.listener.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
			p.t.Errorf("deadline: %v", err)
			return
		}
		buf := make([]byte, 600)
		if n, _, err := p.listener.ReadFromUDP(buf); err == nil {
			p.t.Errorf("the request port received % x after the server chose its TID", buf[:n])
		}
	})
	if _, err := fetchTFTP(t, target); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	wait()
}

// RFC requirement: RFC1350-5-3 positive -- with no option acknowledged the client reads a
// DATA of 0 to 511 octets as the end of the transfer: a 511-octet block ends the file, and
// a file that is an exact multiple of 512 ends on a zero-length block.
// RFC requirement: RFC1350-5-3 negative -- a 512-octet block is not the last block: the
// client acknowledges it and waits for the next one.
func TestRFC1350ClientDataLengthEndsTheTransfer(t *testing.T) {
	shortenTFTP(t)

	cases := map[string][][]byte{
		"short block":   {block(1, 512), block(2, 511)},
		"empty block":   {block(1, 512), {}},
		"only an empty": {{}},
	}
	for name, blocks := range cases {
		t.Run(name, func(t *testing.T) {
			target, wait := startTFTPScript(t, func(p *tftpPeer) {
				for i, payload := range blocks {
					p.data(uint16(i+1), payload)
					if !p.expectACK(uint16(i + 1)) {
						return
					}
				}
				p.data(uint16(len(blocks)+1), block(9, 5))
				p.expectSilence(300*time.Millisecond, "the transfer had ended")
			})
			got, err := fetchTFTP(t, target)
			wait()
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if want := bytes.Join(blocks, nil); !bytes.Equal(got, want) {
				t.Fatalf("fetched %d octets, want %d", len(got), len(want))
			}
		})
	}
}

// RFC requirement: RFC1350-7-1 positive -- a timeout detects a lost read request: when the
// server ignores the first RRQ, the client sends it again after one timeout, and the
// transfer the second RRQ starts completes.
// RFC requirement: RFC1350-7-1 negative -- a server that never answers is detected as an
// error after the bounded retransmissions: the fetch fails with "no answer" rather than
// waiting forever.
func TestRFC1350ClientTimeouts(t *testing.T) {
	shortenTFTP(t)
	tftpTimeout = 200 * time.Millisecond

	t.Run("lost request", func(t *testing.T) {
		listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close() //nolint:errcheck // test cleanup
		addr, ok := listener.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatal("a UDP listener without a UDP address")
		}
		done := make(chan error, 1)
		go func() {
			buf := make([]byte, 600)
			if err := listener.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				done <- err
				return
			}
			if _, _, err := listener.ReadFromUDP(buf); err != nil {
				done <- err
				return
			}
			_, client, err := listener.ReadFromUDP(buf) // the request sent again after the timeout
			if err != nil {
				done <- err
				return
			}
			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				done <- err
				return
			}
			defer conn.Close() //nolint:errcheck // test cleanup
			_, err = conn.WriteToUDP([]byte{0, byte(tftp.OpDATA), 0, 1, 'o', 'k'}, client)
			done <- err
		}()
		got, err := fetchTFTP(t, tftpTarget{host: "127.0.0.1", port: uint16(addr.Port), file: "f.bin"})
		if serverErr := <-done; serverErr != nil {
			t.Fatalf("scripted server: %v", serverErr)
		}
		if err != nil {
			t.Fatalf("fetch after a lost request: %v", err)
		}
		if string(got) != "ok" {
			t.Fatalf("fetched %q, want ok", got)
		}
	})

	t.Run("silent server", func(t *testing.T) {
		listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close() //nolint:errcheck // test cleanup
		addr, ok := listener.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatal("a UDP listener without a UDP address")
		}
		_, err = fetchTFTP(t, tftpTarget{host: "127.0.0.1", port: uint16(addr.Port), file: "f.bin"})
		if err == nil || !strings.Contains(err.Error(), "no answer") {
			t.Fatalf("fetch from a silent server: %v, want no answer", err)
		}
	})
}
