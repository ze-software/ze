// Design: docs/architecture/provisioning/tftp-server.md -- TFTP packet handling (RFC 1350, RFC 2347 option negotiation)

package tftpserver

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/tftp"
)

// The opcodes, error codes, block sizes and the ERROR encoder are the wire
// facts the server shares with Ze's TFTP client, declared once in
// internal/core/tftp.

const (
	ackTimeout    = 5 * time.Second
	maxRetransmit = 3
)

type rrqOptions struct {
	blksize    int
	tsize      bool
	windowsize bool
}

// RFC 2347: RRQ/WRQ with options appends NUL-terminated key/value pairs after mode.
func parseRRQ(pkt []byte) (filename, mode string, opts rrqOptions, err error) {
	if len(pkt) < 6 {
		return "", "", opts, errors.New("packet too short")
	}

	payload := pkt[2:]
	nulIdx := -1
	for i, b := range payload {
		if b == 0 {
			nulIdx = i
			break
		}
	}
	if nulIdx < 1 {
		return "", "", opts, errors.New("missing filename")
	}
	filename = string(payload[:nulIdx])

	rest := payload[nulIdx+1:]
	nulIdx2 := -1
	for i, b := range rest {
		if b == 0 {
			nulIdx2 = i
			break
		}
	}
	if nulIdx2 < 1 {
		return "", "", opts, errors.New("missing mode")
	}
	mode = string(rest[:nulIdx2])

	// RFC 2347: parse option/value pairs after mode NUL.
	remaining := rest[nulIdx2+1:]
	for len(remaining) > 0 {
		optEnd := -1
		for i, b := range remaining {
			if b == 0 {
				optEnd = i
				break
			}
		}
		if optEnd < 1 {
			break
		}
		optName := string(remaining[:optEnd])
		remaining = remaining[optEnd+1:]

		valEnd := -1
		for i, b := range remaining {
			if b == 0 {
				valEnd = i
				break
			}
		}
		if valEnd < 0 {
			break
		}
		optVal := string(remaining[:valEnd])
		remaining = remaining[valEnd+1:]

		switch strings.ToLower(optName) {
		case "blksize":
			n, parseErr := strconv.Atoi(optVal)
			if parseErr == nil && n >= tftp.BlksizeMin && n <= tftp.BlksizeMax {
				opts.blksize = n
			}
		case "tsize":
			opts.tsize = true
		case "windowsize":
			opts.windowsize = true
		}
	}

	return filename, mode, opts, nil
}

// RFC 2347: OACK packet is opcode (2) + option/value NUL-terminated pairs.
func buildOACK(opts []string) []byte {
	size := 2
	for _, o := range opts {
		size += len(o) + 1
	}
	pkt := make([]byte, 2, size)
	binary.BigEndian.PutUint16(pkt[0:2], tftp.OpOACK)
	for _, o := range opts {
		pkt = append(pkt, o...)
		pkt = append(pkt, 0)
	}
	return pkt
}

// RFC 1350 Section 2: DATA packet is opcode (2) + block# (2) + data (0-blksize).
func buildData(block uint16, data []byte) []byte {
	pkt := make([]byte, 4+len(data))
	binary.BigEndian.PutUint16(pkt[0:2], tftp.OpDATA)
	binary.BigEndian.PutUint16(pkt[2:4], block)
	copy(pkt[4:], data)
	return pkt
}

func resolvePath(rootDir, filename string) (string, error) {
	cleaned := filepath.Clean(filename)
	if filepath.IsAbs(cleaned) {
		cleaned = cleaned[1:]
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal")
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	absRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(absRoot, cleaned)
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}

	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(absResolved, absRoot+string(filepath.Separator)) && absResolved != absRoot {
		return "", errors.New("path traversal via symlink")
	}

	return resolved, nil
}

// serve answers the requests that reach conn until conn is closed. Every
// transfer it starts joins generation, and runs until it ends or generation
// ends.
func serve(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, log *slog.Logger) {
	// RFC 2347: RRQ with options can be up to 512 octets.
	buf := make([]byte, 516)
	for serveRequests(generation, conn, rootDir, sem, buf, log) {
		// The pass ended on a contained panic; the next one reads on.
	}
}

// serveRequests is the request task: it answers requests until conn closes,
// answering false, or until a panic ends the pass, answering true. The client
// whose request was being handled is then sent an ERROR, RFC 1350 Section 2.
func serveRequests(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, buf []byte, log *slog.Logger) (again bool) {
	var clientAddr *net.UDPAddr // the sender of the request being handled
	describe := func() taskguard.Context {
		return taskguard.Context{Unit: "request from " + clientAddr.String()}
	}
	// RFC 1350 Section 2
	defer requestTask.Contain(describe, func(*taskguard.Panic) {
		again = true
		if clientAddr != nil {
			signalPanic(conn, clientAddr)
		}
	})

	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return false
		}
		clientAddr = from
		if n < 4 {
			continue
		}

		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		opcode := binary.BigEndian.Uint16(pkt[:2])
		switch opcode {
		case tftp.OpRRQ:
			handleRRQ(generation, conn, pkt, clientAddr, rootDir, sem, log)
		case tftp.OpWRQ:
			resp := tftp.BuildError(tftp.CodeIllegalOperation, "write not supported")
			if _, wErr := conn.WriteToUDP(resp, clientAddr); wErr != nil {
				log.Debug("tftpserver: write error response failed", "error", wErr)
			}
		}
	}
}

// handleRRQ answers one read request. The transfer it starts joins
// generation and is cut when generation ends: its socket is closed, so its
// next read or write fails and the transfer ends.
func handleRRQ(generation *transferGeneration, mainConn *net.UDPConn, pkt []byte, clientAddr *net.UDPAddr, rootDir string, sem chan struct{}, log *slog.Logger) {
	sendErr := func(code uint16, msg string) {
		if _, wErr := mainConn.WriteToUDP(tftp.BuildError(code, msg), clientAddr); wErr != nil {
			log.Debug("tftpserver: error response failed", "error", wErr)
		}
	}

	filename, mode, opts, err := parseRRQ(pkt)
	if err != nil {
		sendErr(tftp.CodeNotDefined, "malformed request")
		return
	}

	if !strings.EqualFold(mode, "octet") {
		sendErr(tftp.CodeIllegalOperation, "only octet mode supported")
		return
	}

	// Surface every read request at info so a provisioning operator can watch
	// the bootloader (ipxe.efi/ipxe.pxe) being fetched over TFTP.
	log.Info("tftpserver: read request", "file", filename, "client", clientAddr.String())

	if len(filename) > 255 {
		sendErr(tftp.CodeFileNotFound, "filename too long")
		return
	}

	resolved, pathErr := resolvePath(rootDir, filename)
	if pathErr != nil {
		sendErr(tftp.CodeAccessViolation, "access denied")
		return
	}

	// RFC 2348 Blocksize Option Specification: "The specified value must be
	// less than or equal to the value specified by the client."
	negotiatedBlksize := tftp.DefaultBlockSize
	if opts.blksize > 0 {
		negotiatedBlksize = min(opts.blksize, tftp.BlksizeEthernet)
		negotiatedBlksize = max(negotiatedBlksize, tftp.BlksizeMin)
	}

	if opts.windowsize {
		log.Debug("tftpserver: windowsize option not supported, falling back to lockstep")
	}

	hasOptions := opts.blksize > 0 || opts.tsize

	select {
	case sem <- struct{}{}:
		t := &transfer{
			clientAddr: clientAddr,
			path:       resolved,
			opts:       opts,
			hasOptions: hasOptions,
			blksize:    negotiatedBlksize,
		}
		go runTransfer(generation, t, sem, log)
	default:
		sendErr(tftp.CodeNotDefined, "service unavailable")
	}
}

// transfer is one accepted read request: what runTransfer serves.
type transfer struct {
	clientAddr *net.UDPAddr
	path       string // resolved by resolvePath
	opts       rrqOptions
	hasOptions bool
	blksize    int // negotiated, within [tftp.BlksizeMin, tftp.BlksizeEthernet]
}

// runTransfer serves one transfer on its own socket and frees its sem slot
// when it ends. The transfer joins generation and is cut when generation
// ends. It is the transfer task: a panic ends this transfer only, after its
// client is sent an ERROR on the transfer's socket, RFC 1350 Section 2.
func runTransfer(generation *transferGeneration, t *transfer, sem chan struct{}, log *slog.Logger) {
	defer func() { <-sem }()

	transferConn, dialErr := net.DialUDP("udp4", nil, t.clientAddr)
	if dialErr != nil {
		log.Debug("tftpserver: dial failed", "client", t.clientAddr, "error", dialErr)
		return
	}
	// A config change ends generation, which closes the socket and cuts the
	// transfer before the reload returns. A transfer whose listener served a
	// config already replaced is refused here.
	if !generation.join(transferConn) {
		closeLogged(transferConn, log, "transfer socket")
		return
	}
	// When the transfer ends first, leave hands the close back to it.
	defer func() {
		if generation.leave(transferConn) {
			closeLogged(transferConn, log, "transfer socket")
		}
	}()

	// Deferred after leave, so it runs first: the socket is still open for
	// the ERROR. serveFile closes its file by defer, so the unwind leaks none.
	describe := func() taskguard.Context {
		return taskguard.Context{Unit: "transfer of " + t.path + " to " + t.clientAddr.String()}
	}
	// RFC 1350 Section 2
	defer transferTask.Contain(describe, func(*taskguard.Panic) { signalPanic(transferConn, nil) })

	if t.hasOptions {
		if !sendOACKAndWait(transferConn, t.path, t.opts, t.blksize, log) {
			return
		}
	}

	serveFile(transferConn, t.path, t.blksize, log)
}

// RFC 2347: send OACK, wait for ACK block 0.
func sendOACKAndWait(conn *net.UDPConn, filePath string, opts rrqOptions, blksize int, log *slog.Logger) bool {
	var oackOpts []string

	if opts.blksize > 0 {
		oackOpts = append(oackOpts, "blksize", strconv.Itoa(blksize))
	}

	// RFC 2349: tsize in RRQ with value "0" is a query; respond with file size.
	if opts.tsize {
		info, err := os.Stat(filePath)
		if err != nil {
			if _, wErr := conn.Write(tftp.BuildError(tftp.CodeFileNotFound, "file not found")); wErr != nil {
				log.Debug("tftpserver: error response failed", "error", wErr)
			}
			return false
		}
		var buf [20]byte
		oackOpts = append(oackOpts, "tsize", string(strconv.AppendInt(buf[:0], info.Size(), 10)))
	}

	if len(oackOpts) == 0 {
		return true
	}

	oack := buildOACK(oackOpts)
	ackBuf := make([]byte, 4)

	// RFC 2347 Negotiation Protocol: "an ACK (with the data block number
	// set to 0) is sent by the client to confirm the values in the server's
	// OACK packet."
	return sendAndWaitACK(conn, oack, 0, ackBuf)
}

// RFC 1350 Section 4: each transfer uses its own connection (TID pair).
func serveFile(conn *net.UDPConn, filePath string, blksize int, log *slog.Logger) {
	f, err := os.Open(filePath) //nolint:gosec // path validated by resolvePath
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

	readBuf := make([]byte, blksize)
	ackBuf := make([]byte, 4)
	block := uint16(1)

	for {
		n, readErr := f.Read(readBuf)
		if readErr != nil && readErr != io.EOF {
			if _, wErr := conn.Write(tftp.BuildError(tftp.CodeNotDefined, "read error")); wErr != nil {
				log.Debug("tftpserver: error response failed", "error", wErr)
			}
			return
		}

		data := buildData(block, readBuf[:n])
		if !sendAndWaitACK(conn, data, block, ackBuf) {
			log.Debug("tftpserver: transfer timeout", "file", filePath, "block", block)
			return
		}

		if n < blksize {
			return
		}
		block++
	}
}

// sendAndWaitACK sends data and waits for the ACK of block. It retransmits
// only when ackTimeout expires, at most maxRetransmit times, and returns false
// when the peer never acknowledges or the socket fails.
func sendAndWaitACK(conn *net.UDPConn, data []byte, block uint16, ackBuf []byte) bool {
	for attempt := 0; attempt <= maxRetransmit; attempt++ {
		if _, err := conn.Write(data); err != nil {
			return false
		}

		if err := conn.SetReadDeadline(time.Now().Add(ackTimeout)); err != nil {
			return false
		}

		// The read deadline bounds this loop: every Read returns by it.
		for {
			ackN, err := conn.Read(ackBuf)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					break
				}
				return false
			}

			if isACKFor(ackBuf[:ackN], block) {
				return true
			}
			// RFC 1350 Section 5: "All packets other than duplicate ACK's and
			// those used for termination are acknowledged unless a timeout
			// occurs [4]. Sending a DATA packet is an acknowledgment for the
			// first ACK packet of the previous DATA packet."
			// A duplicate or stale ACK is not acknowledged, so it triggers no
			// DATA: the wait continues until the same deadline.
		}
	}
	return false
}

// isACKFor reports whether pkt is an ACK of block.
func isACKFor(pkt []byte, block uint16) bool {
	if len(pkt) < 4 {
		return false
	}
	if binary.BigEndian.Uint16(pkt[:2]) != tftp.OpACK {
		return false
	}
	return binary.BigEndian.Uint16(pkt[2:4]) == block
}
