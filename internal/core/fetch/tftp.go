// Design: docs/architecture/appliance/on-device-installer.md -- the tftp scheme: a read-only TFTP client
// Related: scheme.go -- the scheme table that names tftpToFile
// Related: fetch.go -- fileFetch, the retry, bound and digest check every file fetch shares

package fetch

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/tftp"
)

const (
	// tftpPortDefault is the TFTP server port. RFC 1350 Section 4: "A
	// requesting host chooses its source TID as described above, and sends
	// its initial request to the known TID 69 decimal (105 octal) on the
	// serving host."
	tftpPortDefault = 69

	// tftpRetransmitMax is how many times one packet is sent again after a
	// timeout before the attempt fails. It matches the server's bound.
	tftpRetransmitMax = 3

	// tftpWindowsizeRequest is the windowsize the client asks for, decided by
	// Thomas on 2026-10-30. RFC 7440 Section 5: "Operators SHOULD test various
	// values and SHOULD be conservative when selecting a windowsize value".
	tftpWindowsizeRequest = 4

	// tftpModeParameter introduces the transfer mode in a tftp URI (RFC 3617
	// Section 2: mode = ";" "mode=" ( "netascii" / "octet" )).
	tftpModeParameter = ";mode="
)

// tftpTimeout is how long the client waits for the server before it sends its
// last packet again. A var so tests can shorten it, as retryDelay is.
var tftpTimeout = 5 * time.Second

// tftpDally is how long the client waits after its final ACK for the final
// block to come again: one timeout, the time the server waits before it
// sends the block again. A var so tests can shorten it.
var tftpDally = tftpTimeout

// tftpRequest is the RRQ every fetch sends: the Ethernet blksize, the transfer
// size, and the windowsize Thomas chose.
var tftpRequest = tftp.Request{
	Blksize:    tftp.BlksizeEthernet,
	Tsize:      true,
	Windowsize: tftpWindowsizeRequest,
}

// tftpTarget is a parsed tftp URL.
type tftpTarget struct {
	host string
	port uint16
	file string
}

// tftpToFile downloads a tftp://host[:port]/file[;mode=octet] URL to dest
// through the shared retry, bound and digest check. A URL that does not parse
// is refused before any packet is sent and is not retried.
func tftpToFile(source, dest, expectedSHA string) error {
	target, err := parseTFTPURL(source)
	if err != nil {
		// The URL is redacted as every fetch error is: a refused URL may still
		// carry a password the operator typed before the "@".
		return fmt.Errorf("%s: %w", redact.URL(source), err)
	}
	return fileFetch(func(string, string) error { return tftpGet(target, dest) }, source, dest, expectedSHA)
}

// parseTFTPURL reads a tftp URI. RFC 3617 Section 2: "tftpURI = "tftp://"
// host "/" file [ mode ]", with octet when no mode is given. Ze also accepts a
// port after the host, as curl does (owner decision 2026-10-30).
func parseTFTPURL(source string) (tftpTarget, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return tftpTarget{}, fmt.Errorf("not a URL: %w", redact.URLError(err))
	}
	if !strings.EqualFold(parsed.Scheme, "tftp") {
		return tftpTarget{}, errors.New("not a tftp URL")
	}
	if parsed.User != nil {
		return tftpTarget{}, errors.New("a tftp URL carries no user name or password: TFTP has no login (RFC 1350 Section 1); remove the part before @")
	}
	if parsed.RawQuery != "" {
		return tftpTarget{}, errors.New("a tftp URL has no query: remove the part from ?")
	}
	if parsed.Fragment != "" {
		return tftpTarget{}, errors.New("a tftp URL has no fragment: remove the part from #")
	}
	host := parsed.Hostname()
	if host == "" {
		return tftpTarget{}, errors.New("the URL names no server: write tftp://<server>/<file>")
	}
	port, err := tftpPort(parsed.Port())
	if err != nil {
		return tftpTarget{}, err
	}
	file, err := tftpFile(parsed.Path)
	if err != nil {
		return tftpTarget{}, err
	}
	return tftpTarget{host: host, port: port, file: file}, nil
}

// tftpPort reads the optional port of a tftp URL.
func tftpPort(text string) (uint16, error) {
	if text == "" {
		return tftpPortDefault, nil
	}
	port, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return 0, errors.New("port " + strconv.Quote(text) + " is not a number from 1 to 65535")
	}
	if port == 0 {
		return 0, errors.New("port 0 is not a server port: use 1 to 65535, or leave it out for 69")
	}
	return uint16(port), nil
}

// tftpFile reads the file and the optional mode from the URL path.
func tftpFile(path string) (string, error) {
	file := strings.TrimPrefix(path, "/")
	if cut := strings.LastIndex(strings.ToLower(file), tftpModeParameter); cut >= 0 {
		mode := file[cut+len(tftpModeParameter):]
		file = file[:cut]
		if !strings.EqualFold(mode, tftp.ModeOctet) {
			return "", errors.New("mode " + strconv.Quote(mode) + " is not supported: Ze transfers in octet mode only; remove ;mode= or write ;mode=octet")
		}
	}
	if file == "" {
		return "", errors.New("the URL names no file: write tftp://<server>/<file>")
	}
	return file, nil
}

// tftpGet runs one read transfer of target into dest.
func tftpGet(target tftpTarget, dest string) error {
	server, err := net.ResolveUDPAddr("udp", net.JoinHostPort(target.host, strconv.Itoa(int(target.port))))
	if err != nil {
		return fmt.Errorf("resolve %s: %w", target.host, err)
	}
	network := "udp6"
	if server.IP.To4() != nil {
		network = "udp4"
	}
	// RFC 1350 Section 4: "each end of the connection chooses a TID for
	// itself, to be used for the duration of that connection". The kernel
	// picks a fresh ephemeral port for this socket.
	conn, err := net.ListenUDP(network, nil)
	if err != nil {
		return fmt.Errorf("open a UDP socket: %w", err)
	}
	defer conn.Close() //nolint:errcheck // the transfer's result is already decided

	// 0600: a fetched blob carries credentials and the SSH host key.
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // dest chosen by the caller
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	out := bufio.NewWriter(f)
	transfer := newTFTPTransfer(conn, server, out)
	runErr := transfer.run(target.file)
	flushErr := out.Flush()
	closeErr := f.Close()
	if runErr != nil {
		return runErr
	}
	if err := errors.Join(flushErr, closeErr); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}

// tftpTransfer is the state of one read transfer. It is used by one goroutine
// and is not safe for concurrent use.
type tftpTransfer struct {
	conn    *net.UDPConn
	server  *net.UDPAddr // where the RRQ went
	peer    *net.UDPAddr // the server's TID, set by its first reply
	out     io.Writer
	granted tftp.Negotiated
	buf     []byte
	ack     [tftp.HeaderOctets]byte

	next     uint16 // the block number expected next; wraps from 65535 to 0
	inWindow int    // blocks accepted since the last ACK
	gapAcked bool   // a sequence error was already answered in this window
	written  int64
	deadline time.Time // the client gives up waiting at this time
	retries  int       // retransmissions since the last progress
}

func newTFTPTransfer(conn *net.UDPConn, server *net.UDPAddr, out io.Writer) *tftpTransfer {
	return &tftpTransfer{
		conn:   conn,
		server: server,
		out:    out,
		// One more octet than the largest DATA the request allows, so an
		// oversized block is seen rather than cut to size.
		buf:  make([]byte, tftp.HeaderOctets+tftpRequest.Blksize+1),
		next: 1,
	}
}

// run sends the RRQ, settles the options, and receives the file.
func (t *tftpTransfer) run(file string) error {
	rrq, err := tftp.BuildRRQ(file, tftpRequest)
	if err != nil {
		return err
	}
	first, err := t.firstReply(rrq)
	if err != nil {
		return err
	}
	switch binary.BigEndian.Uint16(first[:2]) {
	case tftp.OpOACK:
		if err := t.settle(first); err != nil {
			return err
		}
	case tftp.OpDATA:
		// RFC 2347 Negotiation Protocol: "An option not acknowledged by the
		// server must be ignored by the client and server as if it were never
		// requested." A DATA answer acknowledged none of them.
		t.granted = tftp.Unnegotiated()
		done, err := t.data(first)
		if err != nil {
			return err
		}
		if done {
			return t.dally()
		}
	default:
		return serverError(t.server, first)
	}
	return t.receive()
}

// firstReply sends rrq until the server answers with an OACK, DATA block 1 or
// an ERROR, and returns that answer. The port it comes from is the server's
// TID for the rest of the transfer.
func (t *tftpTransfer) firstReply(rrq []byte) ([]byte, error) {
	for attempt := 0; attempt <= tftpRetransmitMax; attempt++ {
		if _, err := t.conn.WriteToUDP(rrq, t.server); err != nil {
			return nil, fmt.Errorf("send the read request to %s: %w", t.server, err)
		}
		t.deadline = time.Now().Add(tftpTimeout)
		for {
			pkt, from, err := t.read()
			if isTimeout(err) {
				break
			}
			if err != nil {
				return nil, err
			}
			if !from.IP.Equal(t.server.IP) {
				continue // not the server the request went to
			}
			if !isFirstReply(pkt) {
				continue
			}
			t.peer = from
			return pkt, nil
		}
	}
	return nil, fmt.Errorf("no answer from %s after %d read requests %v apart: check the server address, its port and that a TFTP server runs there", t.server, tftpRetransmitMax+1, tftpTimeout)
}

// isFirstReply reports whether pkt can answer an RRQ: an OACK, DATA block 1,
// or an ERROR.
func isFirstReply(pkt []byte) bool {
	switch binary.BigEndian.Uint16(pkt[:2]) {
	case tftp.OpOACK, tftp.OpERROR:
		return true
	case tftp.OpDATA:
		return len(pkt) >= tftp.HeaderOctets && binary.BigEndian.Uint16(pkt[2:4]) == 1
	}
	return false
}

// settle checks the server's OACK and confirms it with ACK 0.
func (t *tftpTransfer) settle(oack []byte) error {
	granted, err := tftp.ParseOACK(oack, tftpRequest)
	if err != nil {
		if refused, ok := errors.AsType[*tftp.OptionError](err); ok {
			// RFC 2347 Negotiation Protocol: "If the client rejects the OACK,
			// then it sends an ERROR packet, with error code 8, to the server
			// and the transfer is terminated."
			t.sendError(tftp.CodeOptionNegotiation, refused.Error())
		}
		return fmt.Errorf("%s: %w", t.peer, err)
	}
	// RFC 2349 Transfer Size Option Specification: "If the file is too large
	// for the client to handle, it may abort the transfer with an Error
	// packet (error code 3)."
	if granted.HasTsize && granted.Tsize > fileSizeMax {
		t.sendError(tftp.CodeDiskFull, "file too large")
		return fmt.Errorf("%s announces %d bytes, larger than the %d bytes a fetch accepts", t.peer, granted.Tsize, fileSizeMax)
	}
	t.granted = granted
	// RFC 2347 Negotiation Protocol: "If the transfer was initiated with a
	// Read Request, then an ACK (with the data block number set to 0) is sent
	// by the client to confirm the values in the server's OACK packet."
	return t.sendACK(0)
}

// receive takes DATA until the final block, then dallies.
func (t *tftpTransfer) receive() error {
	t.progress()
	for {
		pkt, from, err := t.read()
		if isTimeout(err) {
			if retryErr := t.retransmit(); retryErr != nil {
				return retryErr
			}
			continue
		}
		if err != nil {
			return err
		}
		if !t.fromPeer(from) {
			t.refuseStranger(from)
			continue
		}
		switch binary.BigEndian.Uint16(pkt[:2]) {
		case tftp.OpDATA:
			done, err := t.data(pkt)
			if err != nil {
				return err
			}
			if done {
				return t.dally()
			}
		case tftp.OpERROR:
			return serverError(t.peer, pkt)
		case tftp.OpOACK:
			// The server did not see ACK 0 and sent its OACK again.
			if t.next == 1 {
				if err := t.sendACK(0); err != nil {
					return err
				}
			}
		}
	}
}

// data handles one DATA packet from the server's TID and reports whether it
// was the final block.
func (t *tftpTransfer) data(pkt []byte) (bool, error) {
	if len(pkt) < tftp.HeaderOctets {
		return false, nil
	}
	block := binary.BigEndian.Uint16(pkt[2:4])
	payload := pkt[tftp.HeaderOctets:]
	switch {
	case block == t.next:
		return t.accept(block, payload)
	case block == t.next-1:
		// The server sent the last block again: the ACK for it was lost.
		t.inWindow = 0
		return false, t.sendACK(block)
	case block-t.next < uint16(t.granted.Windowsize):
		// RFC 7440 Section 4: "In the case of a data block sequence error,
		// the DRCV SHOULD notify the DSND by sending an ACK corresponding to
		// the last data block correctly received."
		if t.gapAcked {
			return false, nil
		}
		t.gapAcked = true
		t.inWindow = 0
		return false, t.sendACK(t.next - 1)
	}
	return false, nil // an older duplicate, already acknowledged
}

// accept writes the expected block and acknowledges it at the end of a window
// or of the file.
func (t *tftpTransfer) accept(block uint16, payload []byte) (bool, error) {
	if len(payload) > t.granted.Blksize {
		t.sendError(tftp.CodeIllegalOperation, "DATA larger than the negotiated blksize")
		return false, fmt.Errorf("%s sent %d octets in block %d, more than the %d negotiated", t.peer, len(payload), block, t.granted.Blksize)
	}
	t.written += int64(len(payload))
	if t.written > fileSizeMax {
		t.sendError(tftp.CodeDiskFull, "file too large")
		return false, fmt.Errorf("%s sent more than the %d bytes a fetch accepts", t.peer, fileSizeMax)
	}
	if _, err := t.out.Write(payload); err != nil {
		t.sendError(tftp.CodeDiskFull, "write failed")
		return false, fmt.Errorf("write: %w", err)
	}
	t.next++
	t.inWindow++
	t.gapAcked = false
	t.progress()
	// RFC 2348 Blocksize Option Specification: "The reception of a data
	// packet with a data length less than the negotiated blocksize is the
	// final packet."
	final := len(payload) < t.granted.Blksize
	if final {
		return true, t.sendACK(block)
	}
	// RFC 7440 Section 4: "The DRCV MUST send to the DSND the ACK of the last
	// data block of the window in order to confirm a successful data block
	// window reception." With a window of one block this is RFC 1350
	// lockstep, one ACK for each DATA (RFC 7440 Section 4: "Traffic with
	// windowsize = 1 MUST be equivalent to traffic specified by [RFC1350].").
	if t.inWindow < t.granted.Windowsize {
		return false, nil
	}
	t.inWindow = 0
	return false, t.sendACK(block)
}

// retransmit answers a timeout: it sends the ACK of the last block received
// in order again, which also restarts a window, until the bound is reached.
func (t *tftpTransfer) retransmit() error {
	t.retries++
	if t.retries > tftpRetransmitMax {
		return fmt.Errorf("%s stopped sending: no DATA after block %d for %d timeouts of %v", t.peer, t.next-1, tftpRetransmitMax, tftpTimeout)
	}
	t.inWindow = 0
	t.gapAcked = false
	t.deadline = time.Now().Add(tftpTimeout)
	return t.sendACK(t.next - 1)
}

// dally waits one timeout after the final ACK and acknowledges the final
// block again if the server sends it again, because that means the final ACK
// was lost. RFC 1350 Section 6: "the host sending the final ACK will wait for
// a while before terminating in order to retransmit the final ACK if it has
// been lost."
func (t *tftpTransfer) dally() error {
	final := t.next - 1
	t.deadline = time.Now().Add(tftpDally)
	for range tftpRetransmitMax + 1 {
		pkt, from, err := t.read()
		if isTimeout(err) {
			return nil
		}
		if err != nil {
			return nil //nolint:nilerr // the file is complete; a socket error while dallying changes nothing
		}
		if !t.fromPeer(from) {
			t.refuseStranger(from)
			continue
		}
		if !isDataBlock(pkt, final) {
			continue
		}
		if err := t.sendACK(final); err != nil {
			return nil //nolint:nilerr // the file is complete; the server will time out
		}
	}
	return nil
}

func isDataBlock(pkt []byte, block uint16) bool {
	if len(pkt) < tftp.HeaderOctets {
		return false
	}
	if binary.BigEndian.Uint16(pkt[:2]) != tftp.OpDATA {
		return false
	}
	return binary.BigEndian.Uint16(pkt[2:4]) == block
}

// read waits for one packet until the transfer's deadline. A packet shorter
// than an opcode is dropped.
func (t *tftpTransfer) read() ([]byte, *net.UDPAddr, error) {
	if err := t.conn.SetReadDeadline(t.deadline); err != nil {
		return nil, nil, fmt.Errorf("set the read deadline: %w", err)
	}
	for {
		n, from, err := t.conn.ReadFromUDP(t.buf)
		if err != nil {
			return nil, nil, err
		}
		if n >= 2 {
			return t.buf[:n], from, nil
		}
	}
}

// progress moves the deadline after the transfer advanced.
func (t *tftpTransfer) progress() {
	t.retries = 0
	t.deadline = time.Now().Add(tftpTimeout)
}

func (t *tftpTransfer) fromPeer(from *net.UDPAddr) bool {
	if from.Port != t.peer.Port {
		return false
	}
	return from.IP.Equal(t.peer.IP)
}

// refuseStranger answers a packet from another TID without disturbing the
// transfer. RFC 1350 Section 4: "If a source TID does not match, the packet
// should be discarded as erroneously sent from somewhere else.  An error
// packet should be sent to the source of the incorrect packet, while not
// disturbing the transfer."
func (t *tftpTransfer) refuseStranger(from *net.UDPAddr) {
	//nolint:errcheck // the stranger is told once; the transfer goes on either way
	t.conn.WriteToUDP(tftp.BuildError(tftp.CodeUnknownTID, "unknown transfer ID"), from)
}

func (t *tftpTransfer) sendACK(block uint16) error {
	tftp.WriteACK(t.ack[:], block)
	if _, err := t.conn.WriteToUDP(t.ack[:], t.peer); err != nil {
		return fmt.Errorf("send ACK %d to %s: %w", block, t.peer, err)
	}
	return nil
}

func (t *tftpTransfer) sendError(code uint16, msg string) {
	//nolint:errcheck // the transfer already failed; the ERROR is a courtesy to the server
	t.conn.WriteToUDP(tftp.BuildError(code, msg), t.peer)
}

// serverError turns the server's ERROR packet into an error naming its code
// and message. No ACK answers an ERROR: it ends the transfer (RFC 1350
// Section 7).
func serverError(from *net.UDPAddr, pkt []byte) error {
	if binary.BigEndian.Uint16(pkt[:2]) != tftp.OpERROR {
		return fmt.Errorf("%s answered with opcode %d", from, binary.BigEndian.Uint16(pkt[:2]))
	}
	if len(pkt) < tftp.HeaderOctets {
		return fmt.Errorf("%s answered with a truncated ERROR", from)
	}
	code := binary.BigEndian.Uint16(pkt[2:4])
	msg := pkt[tftp.HeaderOctets:]
	if end := strings.IndexByte(string(msg), 0); end >= 0 {
		msg = msg[:end]
	}
	return fmt.Errorf("%s refused the transfer: TFTP error %d: %q", from, code, msg)
}

func isTimeout(err error) bool {
	var netErr net.Error
	if !errors.As(err, &netErr) {
		return false
	}
	return netErr.Timeout()
}
