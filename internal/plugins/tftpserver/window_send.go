// Design: docs/architecture/provisioning/tftp-server.md -- the windowed send (RFC 7440) and its retransmission
// Related: handler.go -- handleRRQ, which grants the window and the timeout a transfer runs with

package tftpserver

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// errNoACK is a window the client did not acknowledge before the timeout.
var errNoACK = errors.New("no ACK of the window before the timeout")

// serveFile sends the file of t to its client as the RFC 7440 data sender
// (DSND): t.windowsize DATA blocks, then a wait for an ACK. A window of one
// block is RFC 1350 lockstep, so a transfer that negotiated no window is sent
// exactly as RFC 1350 says.
//
// Each block is read from the file at its own offset, so a window sent again
// is read again and the transfer holds one block buffer whatever its window.
//
// RFC 1350 Section 4: each transfer uses its own connection (TID pair).
func serveFile(conn *net.UDPConn, t *transfer, log *slog.Logger) {
	f, err := os.Open(t.path) //nolint:gosec // path validated by resolvePath
	if err != nil {
		if _, wErr := conn.Write(tftp.BuildError(tftp.CodeFileNotFound, "file not found")); wErr != nil {
			log.Debug("tftpserver: error response failed", "error", wErr)
		}
		return
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			log.Debug("tftpserver: file close failed", "error", cErr)
		}
	}()

	sender := &windowSender{
		conn:       conn,
		file:       f,
		blksize:    t.blksize,
		windowsize: t.windowsize,
		timeout:    t.timeout,
		packet:     make([]byte, tftp.HeaderOctets+t.blksize),
		ackBuf:     make([]byte, tftp.HeaderOctets),
	}

	// first is the first block of the window to send: the block after the
	// last one the client acknowledged. It counts from 1 and runs past 65535;
	// the wire carries it modulo 2^16.
	first := int64(1)
	// timeouts counts the windows sent since the last acknowledged block.
	timeouts := 0
	for {
		sent, final, ok := sender.sendWindow(first, log)
		if !ok {
			return
		}
		acked, err := sender.awaitACK(first, sent)
		if errors.Is(err, errNoACK) {
			timeouts++
			// RFC 7440 Section 6: "Implementations SHOULD always set a
			// maximum number of retries for datagram retransmissions,
			// imposing an appropriate threshold on error recovery attempts,
			// after which a transfer SHOULD always be aborted to prevent
			// pathological retransmission conditions."
			if timeouts > maxRetransmit {
				log.Debug("tftpserver: transfer timeout", "file", t.path, "block", first)
				return
			}
			// RFC 7440 Section 4: "In the case of an expected ACK not
			// timely reaching the DSND (timeout), the last received ACK
			// SHALL set the beginning of the next windowsize data block
			// window to be sent." first is still the block after it.
			continue
		}
		if err != nil {
			return
		}
		// RFC 7440 Section 3: "The reception of a data window with a number
		// of blocks less than the negotiated windowsize is the final window."
		// The transfer ends when the final block itself is acknowledged.
		if final && acked == first+int64(sent)-1 {
			return
		}
		// RFC 7440 Section 4: "The notified DSND SHOULD send a new data
		// block window whose beginning MUST be set based on the ACK received
		// out of sequence." An ACK of the window's last block starts the next
		// window the same way.
		first = acked + 1
		timeouts = 0
	}
}

// windowSender is the sending side of one transfer. It is used by the
// transfer's goroutine only: not safe for concurrent use.
type windowSender struct {
	conn       *net.UDPConn
	file       *os.File
	blksize    int
	windowsize int
	timeout    time.Duration
	// packet holds one DATA packet: the header, then up to blksize octets.
	packet []byte
	// ackBuf holds one ACK; a longer datagram is read truncated.
	ackBuf []byte
}

// sendWindow sends up to windowsize blocks from block first. It answers how
// many it sent and whether the last of them is the final block, the one
// shorter than blksize, and false when the file cannot be read, after the
// client is sent an ERROR, or when the socket fails.
//
// RFC 7440 Section 4: "The DSND MUST cyclically send to the DRCV the agreed
// windowsize consecutive data blocks before normally stopping and waiting for
// the ACK of the transferred window."
func (s *windowSender) sendWindow(first int64, log *slog.Logger) (sent int, final, ok bool) {
	for sent < s.windowsize {
		block := first + int64(sent)
		n, err := s.readBlock(block)
		if err != nil {
			if _, wErr := s.conn.Write(tftp.BuildError(tftp.CodeNotDefined, "read error")); wErr != nil {
				log.Debug("tftpserver: error response failed", "error", wErr)
			}
			return sent, false, false
		}
		// RFC 1350 Section 5: "DATA packet (opcode 3)", then the block number
		// and the data.
		//
		//	 2 bytes     2 bytes      n bytes
		//	 ----------------------------------
		//	| Opcode |   Block #  |   Data     |
		//	 ----------------------------------
		binary.BigEndian.PutUint16(s.packet[0:2], tftp.OpDATA)
		binary.BigEndian.PutUint16(s.packet[2:4], uint16(block)) //nolint:gosec // G115: the wire block number is the block modulo 2^16
		if _, err := s.conn.Write(s.packet[:tftp.HeaderOctets+n]); err != nil {
			return sent, false, false
		}
		sent++
		// RFC 1350 Section 6: "A data packet of less than 512 bytes signals
		// termination of a transfer." RFC 2348 puts blksize in place of 512.
		if n < s.blksize {
			return sent, true, true
		}
	}
	return sent, false, true
}

// readBlock reads block, counted from 1, into the data field of the packet
// and answers its length. A block past the end of the file is empty, which an
// exact multiple of blksize ends on.
func (s *windowSender) readBlock(block int64) (int, error) {
	n, err := s.file.ReadAt(s.packet[tftp.HeaderOctets:tftp.HeaderOctets+s.blksize], (block-1)*int64(s.blksize))
	if errors.Is(err, io.EOF) {
		return n, nil
	}
	return n, err
}

// awaitACK waits up to the timeout for the client to acknowledge a block of
// the window of sent blocks from first, and answers the block acknowledged.
// An ACK of the block before the window, or of a block not sent yet, is
// neither: the wait goes on to the same deadline, so a duplicate or stale ACK
// draws no DATA. It answers errNoACK at the deadline, and the socket's error
// when the socket fails.
func (s *windowSender) awaitACK(first int64, sent int) (int64, error) {
	if err := s.conn.SetReadDeadline(time.Now().Add(s.timeout)); err != nil {
		return 0, err
	}
	// The read deadline bounds this loop: every Read returns by it.
	for {
		n, err := s.conn.Read(s.ackBuf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return 0, errNoACK
			}
			return 0, err
		}
		position, isACK := windowPosition(s.ackBuf[:n], first)
		if !isACK {
			continue
		}
		// RFC 1350 Section 5: "All packets other than duplicate ACK's and
		// those used for termination are acknowledged unless a timeout occurs
		// [4]." Position 0 is the ACK of the block before the window, which
		// the client already sent once: a duplicate.
		if position == 0 {
			continue
		}
		if int(position) > sent {
			continue
		}
		return first + int64(position) - 1, nil
	}
}

// windowPosition answers where the ACK pkt falls in the window that starts at
// block first: 1 for first, 2 for the block after it, 0 for the block before
// the window, and above the window's length for a block not sent. Block
// numbers wrap at 65535 on the wire, so the position is the 16-bit distance
// from the block before the window. It answers false when pkt is no ACK.
func windowPosition(pkt []byte, first int64) (uint16, bool) {
	if len(pkt) < tftp.HeaderOctets {
		return 0, false
	}
	if binary.BigEndian.Uint16(pkt[:2]) != tftp.OpACK {
		return 0, false
	}
	return binary.BigEndian.Uint16(pkt[2:4]) - uint16(first-1), true //nolint:gosec // G115: the wire block number is the block modulo 2^16
}
