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
)

// RFC 1350 Section 2: TFTP opcodes.
const (
	opRRQ   uint16 = 1
	opWRQ   uint16 = 2
	opDATA  uint16 = 3
	opACK   uint16 = 4
	opERROR uint16 = 5
	opOACK  uint16 = 6 // RFC 2347 Section "Packet Formats"
)

// RFC 1350 Section 5: error codes.
const (
	errNotDefined      uint16 = 0
	errFileNotFound    uint16 = 1
	errAccessViolation uint16 = 2
	errIllegalOp       uint16 = 4
)

// RFC 1350 Section 2: data block size.
const defaultBlockSize = 512

// RFC 2348: blksize valid range.
const (
	blksizeMin = 8
	blksizeMax = 65464
)

// RFC 2348: practical upper bound for one untagged Ethernet frame
// (1500 MTU - 20 IP - 8 UDP - 4 TFTP header).
const blksizeEthernet = 1468

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
			if parseErr == nil && n >= blksizeMin && n <= blksizeMax {
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
	binary.BigEndian.PutUint16(pkt[0:2], opOACK)
	for _, o := range opts {
		pkt = append(pkt, o...)
		pkt = append(pkt, 0)
	}
	return pkt
}

// RFC 1350 Section 2: DATA packet is opcode (2) + block# (2) + data (0-blksize).
func buildData(block uint16, data []byte) []byte {
	pkt := make([]byte, 4+len(data))
	binary.BigEndian.PutUint16(pkt[0:2], opDATA)
	binary.BigEndian.PutUint16(pkt[2:4], block)
	copy(pkt[4:], data)
	return pkt
}

// RFC 1350 Section 5: ERROR packet is opcode (2) + errcode (2) + message + NUL.
func buildError(code uint16, msg string) []byte {
	pkt := make([]byte, 5+len(msg))
	binary.BigEndian.PutUint16(pkt[0:2], opERROR)
	binary.BigEndian.PutUint16(pkt[2:4], code)
	copy(pkt[4:], msg)
	pkt[4+len(msg)] = 0
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

func serve(conn *net.UDPConn, rootDir string, sem chan struct{}, log *slog.Logger) {
	// RFC 2347: RRQ with options can be up to 512 octets.
	buf := make([]byte, 516)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < 4 {
			continue
		}

		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		opcode := binary.BigEndian.Uint16(pkt[:2])
		switch opcode {
		case opRRQ:
			handleRRQ(conn, pkt, clientAddr, rootDir, sem, log)
		case opWRQ:
			resp := buildError(errIllegalOp, "write not supported")
			if _, wErr := conn.WriteToUDP(resp, clientAddr); wErr != nil {
				log.Debug("tftpserver: write error response failed", "error", wErr)
			}
		}
	}
}

func handleRRQ(mainConn *net.UDPConn, pkt []byte, clientAddr *net.UDPAddr, rootDir string, sem chan struct{}, log *slog.Logger) {
	sendErr := func(code uint16, msg string) {
		if _, wErr := mainConn.WriteToUDP(buildError(code, msg), clientAddr); wErr != nil {
			log.Debug("tftpserver: error response failed", "error", wErr)
		}
	}

	filename, mode, opts, err := parseRRQ(pkt)
	if err != nil {
		sendErr(errNotDefined, "malformed request")
		return
	}

	if !strings.EqualFold(mode, "octet") {
		sendErr(errIllegalOp, "only octet mode supported")
		return
	}

	// Surface every read request at info so a provisioning operator can watch
	// the bootloader (ipxe.efi/ipxe.pxe) being fetched over TFTP.
	log.Info("tftpserver: read request", "file", filename, "client", clientAddr.String())

	if len(filename) > 255 {
		sendErr(errFileNotFound, "filename too long")
		return
	}

	resolved, pathErr := resolvePath(rootDir, filename)
	if pathErr != nil {
		sendErr(errAccessViolation, "access denied")
		return
	}

	// RFC 2348 Blocksize Option Specification: "The specified value must be
	// less than or equal to the value specified by the client."
	negotiatedBlksize := defaultBlockSize
	if opts.blksize > 0 {
		negotiatedBlksize = min(opts.blksize, blksizeEthernet)
		negotiatedBlksize = max(negotiatedBlksize, blksizeMin)
	}

	if opts.windowsize {
		log.Debug("tftpserver: windowsize option not supported, falling back to lockstep")
	}

	hasOptions := opts.blksize > 0 || opts.tsize

	select {
	case sem <- struct{}{}:
		go func() {
			defer func() { <-sem }()

			transferConn, dialErr := net.DialUDP("udp4", nil, clientAddr)
			if dialErr != nil {
				log.Debug("tftpserver: dial failed", "client", clientAddr, "error", dialErr)
				return
			}
			defer func() {
				if cErr := transferConn.Close(); cErr != nil {
					log.Debug("tftpserver: close failed", "error", cErr)
				}
			}()

			if hasOptions {
				if !sendOACKAndWait(transferConn, resolved, opts, negotiatedBlksize, log) {
					return
				}
			}

			serveFile(transferConn, resolved, negotiatedBlksize, log)
		}()
	default:
		sendErr(errNotDefined, "service unavailable")
	}
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
			if _, wErr := conn.Write(buildError(errFileNotFound, "file not found")); wErr != nil {
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
		if _, wErr := conn.Write(buildError(errFileNotFound, "file not found")); wErr != nil {
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
			if _, wErr := conn.Write(buildError(errNotDefined, "read error")); wErr != nil {
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

func sendAndWaitACK(conn *net.UDPConn, data []byte, block uint16, ackBuf []byte) bool {
	for attempt := 0; attempt <= maxRetransmit; attempt++ {
		if _, err := conn.Write(data); err != nil {
			return false
		}

		if err := conn.SetReadDeadline(time.Now().Add(ackTimeout)); err != nil {
			return false
		}
		ackN, err := conn.Read(ackBuf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return false
		}

		if ackN >= 4 {
			ackOp := binary.BigEndian.Uint16(ackBuf[:2])
			ackBlock := binary.BigEndian.Uint16(ackBuf[2:4])
			if ackOp == opACK && ackBlock == block {
				return true
			}
		}
	}
	return false
}
