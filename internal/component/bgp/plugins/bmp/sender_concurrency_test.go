package bmp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestBMPSenderConcurrentProducers drives the two independent goroutines that
// really do reach a single senderSession in the daemon:
//
//   - the bmp plugin's own delivery loop (bmp.go RunBMPPlugin ->
//     handleStructuredEvent -> handleSenderState -> writePeerUp/writePeerDown), and
//   - whichever goroutine publishes a RIB best-change. Engine EventBus
//     subscribers fire synchronously on the publisher's goroutine
//     (internal/component/plugin/server/engine_event.go SubscribeEngineEvent),
//     so the RFC 9069 Loc-RIB path (bmp_locrib.go handleBestChange ->
//     ensureLocRIBPeerUp / writeRouteMonitoring) runs on the RIB plugin's
//     delivery goroutine, not on bmp's.
//
// VALIDATES: every BMP message a senderSession emits arrives whole and correctly
// framed no matter how many goroutines call write* concurrently -- the decoded
// Peer Up still carries its own OPEN bytes and the decoded Route Monitoring
// still carries its own UPDATE body.
// PREVENTS: regressing the shared-scratch corruption behind the flaky
// test/plugin/bmp-locrib.ci. Both producers encoded into the one senderSession
// scratch array and then flushed buf[:n], so the loser's bytes were overwritten
// before its write and a message went out whose Common Header length did not
// match its content; the collector's framing then desynchronised
// ("BMP-COLLECTOR: invalid-pdu: BMP version=0 length=...").
func TestBMPSenderConcurrentProducers(t *testing.T) {
	// A real TCP pair, not net.Pipe: net.Pipe is unbuffered and forces the
	// writers into lockstep with the reader, which hides the overlap this test
	// exists to create.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer closeLog(ln, "test-listener")

	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr != nil {
			accepted <- nil
			return
		}
		accepted <- c
	}()

	var d net.Dialer
	client, err := d.DialContext(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer closeLog(client, "test-client")

	server := <-accepted
	if server == nil {
		t.Fatal("accept failed")
	}
	defer closeLog(server, "test-server")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}

	const rounds = 200
	sentOpen := makeBGPOpen(65001, 0x01020304)
	recvOpen := makeBGPOpen(65002, 0x05060708)
	// Long enough that a Peer Up encoded over it (or the reverse) leaves a
	// length or content mismatch rather than an accidentally identical buffer.
	body := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 24)
	peer := testPeerHeader()

	type readOut struct {
		peerUps int
		monitor int
		err     error
	}
	// Reader: decode until the producers close the connection, stopping early at
	// the first framing or content error. Runs concurrently so the producers
	// never block on the socket. Draining to EOF rather than to a fixed count
	// means a producer that dies early cannot leave the reader blocked: the
	// failure is then reported by the producer plus the message-count assertion,
	// not by a timeout that would blame framing for something else.
	done := make(chan readOut, 1)
	go func() {
		var out readOut
		for i := 0; ; i++ {
			msg, rerr := readBMPFromPipe(server)
			if rerr != nil {
				if errors.Is(rerr, io.EOF) || errors.Is(rerr, io.ErrUnexpectedEOF) {
					break // producers finished and closed the connection
				}
				out.err = fmt.Errorf("message %d: %w", i, rerr)
				break
			}
			switch m := msg.(type) {
			case *PeerUp:
				if !bytes.Equal(m.SentOpenMsg, sentOpen) || !bytes.Equal(m.ReceivedOpenMsg, recvOpen) {
					out.err = fmt.Errorf("message %d: Peer Up OPEN bytes corrupted", i)
				}
				out.peerUps++
			case *RouteMonitoring:
				want := message.HeaderLen + len(body)
				switch {
				case len(m.BGPUpdate) != want:
					out.err = fmt.Errorf("message %d: Route Monitoring UPDATE is %d bytes, want %d", i, len(m.BGPUpdate), want)
				case !bytes.Equal(m.BGPUpdate[message.HeaderLen:], body):
					out.err = fmt.Errorf("message %d: Route Monitoring body corrupted", i)
				}
				out.monitor++
			default:
				out.err = fmt.Errorf("message %d: unexpected type %T", i, msg)
			}
			if out.err != nil {
				break
			}
		}
		done <- out
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	// Producer 1: the bmp delivery loop's Peer Up path.
	go func() {
		defer wg.Done()
		for range rounds {
			if werr := ss.writePeerUp(peer, [16]byte{}, 179, 54321, sentOpen, recvOpen); werr != nil {
				t.Errorf("writePeerUp: %v", werr)
				return
			}
		}
	}()
	// Producer 2: the Loc-RIB best-change path, on the RIB plugin's goroutine.
	go func() {
		defer wg.Done()
		for range rounds {
			if werr := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); werr != nil {
				t.Errorf("writeRouteMonitoring: %v", werr)
				return
			}
		}
	}()
	wg.Wait()
	// Closing the write side is what ends the reader's drain loop.
	closeLog(client, "test-producers-done")

	select {
	case out := <-done:
		if out.err != nil {
			t.Fatalf("collector saw a corrupted BMP stream: %v", out.err)
		}
		if out.peerUps != rounds || out.monitor != rounds {
			t.Fatalf("decoded %d Peer Up + %d Route Monitoring, want %d each", out.peerUps, out.monitor, rounds)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("reader did not finish after the connection was closed")
	}
}

// TestBMPSenderInitiationPrecedesConcurrentProducer starts a producer that
// hammers writePeerUp from before the session dials, then asserts the collector
// still reads Initiation as message 1.
//
// VALIDATES: RFC 7854 Section 4.3 -- Initiation is the first message on a new
// BMP session even when another goroutine is already trying to send. run()
// publishes ss.conn only after sendInitiation returns, so a producer racing the
// connect either finds a nil conn or blocks on writeMu; it cannot get ahead.
// PREVENTS: reverting to publishing ss.conn before sendInitiation, which let a
// Peer Up or a Loc-RIB Route Monitoring reach the collector first. Verified to
// gate: with the publish moved back ahead of sendInitiation this fails 28/30
// runs. TestBMPSenderConnects covers the same guarantee uncontended.
func TestBMPSenderInitiationPrecedesConcurrentProducer(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer closeLog(ln, "test-listener")

	addr, _ := ln.Addr().(*net.TCPAddr)
	ss := newSenderSession("test", collectorConfig{
		Address: "127.0.0.1",
		Port:    strconv.Itoa(addr.Port),
	})

	connCh := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		connCh <- c
	}()

	// Producer racing the connect: starts before run() dials and keeps trying
	// until told to stop. errNotConnected is the expected outcome for most
	// iterations; what matters is that none of its bytes precede Initiation.
	stop := make(chan struct{})
	var producer sync.WaitGroup
	producer.Go(func() {
		peer := testPeerHeader()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = ss.writePeerUp(peer, [16]byte{}, 179, 54321, nil, nil)
		}
	})

	var wg sync.WaitGroup
	wg.Go(ss.run)

	var collectorConn net.Conn
	select {
	case collectorConn = <-connCh:
	case <-time.After(5 * time.Second):
		close(stop)
		producer.Wait()
		ss.stop()
		wg.Wait()
		t.Fatal("sender did not connect within timeout")
	}

	headerBuf := make([]byte, CommonHeaderSize)
	if _, rerr := io.ReadFull(collectorConn, headerBuf); rerr != nil {
		t.Errorf("read first header: %v", rerr)
	} else if ch, _, derr := DecodeCommonHeader(headerBuf, 0); derr != nil {
		t.Errorf("decode first header: %v", derr)
	} else if ch.Type != MsgInitiation {
		t.Errorf("first message type = %d, want %d (Initiation): a producer got ahead of it", ch.Type, MsgInitiation)
	}

	close(stop)
	producer.Wait()
	ss.stop()
	wg.Wait()
	closeLog(collectorConn, "test-collector")
}
