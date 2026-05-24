// Design: plan/spec-install-2-tftpserver.md -- TFTP packet handling (RFC 1350)

package tftpserver

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
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
)

// RFC 1350 Section 5: error codes.
const (
	errNotDefined      uint16 = 0
	errFileNotFound    uint16 = 1
	errAccessViolation uint16 = 2
	errIllegalOp       uint16 = 4
)

// RFC 1350 Section 2: data block size.
const blockSize = 512

const (
	ackTimeout    = 5 * time.Second
	maxRetransmit = 3
)

// RFC 1350 Section 2: RRQ/WRQ packet format is opcode (2) + filename (NUL) + mode (NUL).
func parseRRQ(pkt []byte) (filename, mode string, err error) {
	if len(pkt) < 6 {
		return "", "", errors.New("packet too short")
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
		return "", "", errors.New("missing filename")
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
		return "", "", errors.New("missing mode")
	}
	mode = string(rest[:nulIdx2])

	return filename, mode, nil
}

// RFC 1350 Section 2: DATA packet is opcode (2) + block# (2) + data (0-512).
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

	filename, mode, err := parseRRQ(pkt)
	if err != nil {
		sendErr(errNotDefined, "malformed request")
		return
	}

	if !strings.EqualFold(mode, "octet") {
		sendErr(errIllegalOp, "only octet mode supported")
		return
	}

	if len(filename) > 255 {
		sendErr(errFileNotFound, "filename too long")
		return
	}

	resolved, pathErr := resolvePath(rootDir, filename)
	if pathErr != nil {
		sendErr(errAccessViolation, "access denied")
		return
	}

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

			serveFile(transferConn, resolved, log)
		}()
	default:
		sendErr(errNotDefined, "service unavailable")
	}
}

// RFC 1350 Section 4: each transfer uses its own connection (TID pair).
func serveFile(conn *net.UDPConn, filePath string, log *slog.Logger) {
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

	readBuf := make([]byte, blockSize)
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

		if n < blockSize {
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
