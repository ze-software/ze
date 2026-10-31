// Design: docs/architecture/provisioning/tftp-server.md -- TFTP packet handling (RFC 1350, RFC 2347 option negotiation)
// Related: window_send.go -- serveFile, the RFC 7440 data sender a transfer runs

package tftpserver

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/tftp"
)

// The opcodes, error codes, block sizes and the ERROR encoder are the wire
// facts the server shares with Ze's TFTP client, declared once in
// internal/core/tftp.

const (
	// ackTimeout is the retransmission interval of a transfer whose client
	// requested no RFC 2349 timeout, when no config sets one: the YANG
	// default of the timeout leaf.
	ackTimeout = defaultTimeoutSeconds * time.Second
	// maxRetransmit bounds the resends of one window that the client does not
	// acknowledge: it is sent at most maxRetransmit+1 times.
	maxRetransmit = 3
)

// transferLimits is what the config sets for each transfer: the
// retransmission interval when the client requests no timeout, and the
// largest window granted. A request takes the values running when it
// arrives, so a reload never changes a transfer in flight.
type transferLimits struct {
	timeout       time.Duration
	windowsizeMax int
}

// defaultLimits answers a limits source holding the YANG defaults, for a
// server that runs without config (ServeConn and the tests).
func defaultLimits() *atomic.Pointer[transferLimits] {
	var limits atomic.Pointer[transferLimits]
	limits.Store(&transferLimits{timeout: ackTimeout, windowsizeMax: defaultMaxWindowsize})
	return &limits
}

// rrqOptions is what a read request asked for. A numeric option is zero when
// the request did not carry it or carried a value outside its RFC range, which
// RFC 2347 treats alike: the option is not acknowledged. Zero is never a valid
// value of any of them, so it names the absence.
type rrqOptions struct {
	blksize int
	// timeout is the requested retransmission interval, in seconds.
	timeout int
	tsize   bool
	// windowsize records that the request named the windowsize option with a
	// NUL-terminated value, whatever the value.
	windowsize bool
	// windowsizeBlocks is the requested window, in blocks.
	windowsizeBlocks int
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
		case tftp.OptionBlksize:
			// RFC 2348 Blocksize Option Specification: "Valid values range
			// between "8" and "65464" octets, inclusive."
			opts.blksize = optionInRange(tftp.OptionBlksize, optVal, tftp.BlksizeMin, tftp.BlksizeMax)
		case tftp.OptionTimeout:
			// RFC 2349 Timeout Interval Option Specification: "Valid values
			// range between "1" and "255" seconds, inclusive."
			opts.timeout = optionInRange(tftp.OptionTimeout, optVal, tftp.TimeoutMin, tftp.TimeoutMax)
		case tftp.OptionTsize:
			opts.tsize = true
		case tftp.OptionWindowsize:
			opts.windowsize = true
			// RFC 7440 Section 3: "The valid values range MUST be between 1
			// and 65535 blocks, inclusive."
			opts.windowsizeBlocks = optionInRange(tftp.OptionWindowsize, optVal, tftp.WindowsizeMin, tftp.WindowsizeMax)
		}
	}

	return filename, mode, opts, nil
}

// optionInRange answers the value of option name when it is a plain decimal
// number from low to high, and zero, which no option of an RRQ takes, when it
// is not: RFC 2347 Negotiation Protocol, "An option not acknowledged by the
// server must be ignored by the client and server as if it were never
// requested."
func optionInRange(name, value string, low, high int) int {
	number, err := tftp.ParseDecimal(name, value)
	if err != nil {
		return 0
	}
	if number < int64(low) {
		return 0
	}
	if number > int64(high) {
		return 0
	}
	return int(number)
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

// serve answers the requests that reach conn until conn is closed, under the
// YANG default limits (serveLimited).
func serve(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, log *slog.Logger) {
	serveLimited(generation, conn, rootDir, sem, defaultLimits(), log)
}

// serveLimited answers the requests that reach conn until conn is closed.
// Every transfer it starts joins generation, takes the limits stored in limits
// when its request arrives, and runs until it ends or generation ends.
func serveLimited(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, limits *atomic.Pointer[transferLimits], log *slog.Logger) {
	// RFC 2347: RRQ with options can be up to 512 octets.
	buf := make([]byte, 516)
	for serveRequests(generation, conn, rootDir, sem, limits, buf, log) {
		// The pass ended on a contained panic; the next one reads on.
	}
}

// serveRequests is the request task: it answers requests until conn closes,
// answering false, or until a panic ends the pass, answering true. The client
// whose request was being handled is then sent an ERROR, RFC 1350 Section 2.
func serveRequests(generation *transferGeneration, conn *net.UDPConn, rootDir string, sem chan struct{}, limits *atomic.Pointer[transferLimits], buf []byte, log *slog.Logger) (again bool) {
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
			handleRRQ(generation, conn, pkt, clientAddr, rootDir, sem, limits.Load(), log)
		case tftp.OpWRQ:
			resp := tftp.BuildError(tftp.CodeIllegalOperation, "write not supported")
			if _, wErr := conn.WriteToUDP(resp, clientAddr); wErr != nil {
				log.Debug("tftpserver: write error response failed", "error", wErr)
			}
		}
	}
}

// handleRRQ answers one read request under limits, the values running when
// it arrived. The transfer it starts joins generation and is cut when
// generation ends: its socket is closed, so its next read or write fails and
// the transfer ends.
func handleRRQ(generation *transferGeneration, mainConn *net.UDPConn, pkt []byte, clientAddr *net.UDPAddr, rootDir string, sem chan struct{}, limits *transferLimits, log *slog.Logger) {
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

	// RFC 2349 Timeout Interval Option Specification: "The specified timeout
	// value must match the value specified by the client." A timeout outside
	// 1..255 was left unrequested by parseRRQ, so it is not acknowledged and
	// the configured interval applies.
	timeout := limits.timeout
	if opts.timeout > 0 {
		timeout = time.Duration(opts.timeout) * time.Second
	}
	// RFC 7440 Section 3
	windowsize := grantWindowsize(opts.windowsizeBlocks, limits.windowsizeMax)

	hasOptions := opts.blksize > 0 || opts.tsize || opts.timeout > 0 || windowsize > tftp.WindowsizeMin
	log.Debug("tftpserver: options granted", "file", filename, "client", clientAddr.String(),
		"blksize", negotiatedBlksize, "windowsize", windowsize, "timeout", timeout)

	select {
	case sem <- struct{}{}:
		t := &transfer{
			clientAddr: clientAddr,
			path:       resolved,
			opts:       opts,
			hasOptions: hasOptions,
			blksize:    negotiatedBlksize,
			timeout:    timeout,
			windowsize: windowsize,
		}
		go runTransfer(generation, t, sem, log)
	default:
		sendErr(tftp.CodeNotDefined, "service unavailable")
	}
}

// grantWindowsize answers the window a request for requested blocks is sent
// with under the server maximum: the smaller of the two, or one block when
// that is all the grant would be or no window was requested. A window of one
// block is RFC 1350 lockstep and is not acknowledged in the OACK, so a
// request for windowsize 1 alone is answered as one with no option.
//
// RFC 7440 Section 3: "The specified value MUST be less than or equal to the
// value specified by the client."
func grantWindowsize(requested, maximum int) int {
	granted := min(requested, maximum)
	if granted <= tftp.WindowsizeMin {
		return tftp.WindowsizeMin
	}
	return granted
}

// transfer is one accepted read request: what runTransfer serves.
type transfer struct {
	clientAddr *net.UDPAddr
	path       string // resolved by resolvePath
	opts       rrqOptions
	hasOptions bool
	blksize    int // negotiated, within [tftp.BlksizeMin, tftp.BlksizeEthernet]
	// timeout is the retransmission interval: the acknowledged RFC 2349
	// timeout, else the configured one.
	timeout time.Duration
	// windowsize is the blocks sent before an ACK is awaited, from 1 to the
	// configured maximum; above 1 it was acknowledged in the OACK.
	windowsize int
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
		if !sendOACKAndWait(transferConn, t, log) {
			return
		}
	}

	serveFile(transferConn, t, log)
}

// RFC 2347: send OACK, wait for ACK block 0.
func sendOACKAndWait(conn *net.UDPConn, t *transfer, log *slog.Logger) bool {
	var oackOpts []string

	if t.opts.blksize > 0 {
		oackOpts = append(oackOpts, tftp.OptionBlksize, strconv.Itoa(t.blksize))
	}

	// RFC 2349 Timeout Interval Option Specification: "If the server is
	// willing to accept the timeout option, it sends an Option Acknowledgment
	// (OACK) to the client.  The specified timeout value must match the value
	// specified by the client."
	if t.opts.timeout > 0 {
		oackOpts = append(oackOpts, tftp.OptionTimeout, strconv.Itoa(t.opts.timeout))
	}

	// RFC 2349: tsize in RRQ with value "0" is a query; respond with file size.
	if t.opts.tsize {
		info, err := os.Stat(t.path)
		if err != nil {
			if _, wErr := conn.Write(tftp.BuildError(tftp.CodeFileNotFound, "file not found")); wErr != nil {
				log.Debug("tftpserver: error response failed", "error", wErr)
			}
			return false
		}
		var buf [20]byte
		oackOpts = append(oackOpts, tftp.OptionTsize, string(strconv.AppendInt(buf[:0], info.Size(), 10)))
	}

	// RFC 7440 Section 3: "If the server is willing to accept the windowsize
	// option, it sends an Option Acknowledgment (OACK) to the client.  The
	// specified value MUST be less than or equal to the value specified by
	// the client." grantWindowsize made it so.
	if t.windowsize > tftp.WindowsizeMin {
		oackOpts = append(oackOpts, tftp.OptionWindowsize, strconv.Itoa(t.windowsize))
	}

	if len(oackOpts) == 0 {
		return true
	}

	oack := buildOACK(oackOpts)
	ackBuf := make([]byte, 4)

	// RFC 2347 Negotiation Protocol: "an ACK (with the data block number
	// set to 0) is sent by the client to confirm the values in the server's
	// OACK packet."
	return sendAndWaitACK(conn, oack, 0, ackBuf, t.timeout)
}

// sendAndWaitACK sends data and waits for the ACK of block: the OACK and its
// ACK 0, RFC 2347 Negotiation Protocol. It retransmits only when timeout
// expires, at most maxRetransmit times, and returns false when the peer never
// acknowledges or the socket fails.
func sendAndWaitACK(conn *net.UDPConn, data []byte, block uint16, ackBuf []byte, timeout time.Duration) bool {
	for attempt := 0; attempt <= maxRetransmit; attempt++ {
		if _, err := conn.Write(data); err != nil {
			return false
		}

		// refuse-by-default: guards runTransfer -- false ends the transfer, the answer a failed socket gets
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
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
