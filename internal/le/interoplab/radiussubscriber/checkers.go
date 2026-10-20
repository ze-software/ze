// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite
// Related: radiussubscriber.go -- the topology these checkers observe
// RFC: rfc/short/rfc5176.md -- RFC5176-2.3-2, the duplicate a retransmission is
// RFC: rfc/short/rfc2866.md -- RFC2866-5.5-1, the Acct-Session-Id a restart keeps unique
//
// Every scenario reads both sides: FreeRADIUS's own record (the ze_detail file
// and radclient's output) and ze's (its log, and the CoA datagrams ze's
// container received, captured with tcpdump).

package radiussubscriber

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// detailPath is where the lab's ze_detail module writes every
	// Accounting-Request the server accepted (mods-ze-detail).
	detailPath = "/var/log/freeradius/ze-detail"
	// coaAddress and coaPort are ze's CoA listener, zeHost on networkPrefix.
	// coaTarget names it for radclient, coaDevice for bash's UDP redirection.
	coaAddress = "172.26.0.2"
	coaPort    = "3799"
	coaTarget  = coaAddress + ":" + coaPort
	coaDevice  = "/dev/udp/" + coaAddress + "/" + coaPort
	// sharedSecret is the fixture secret in clients.conf and both ze.conf files.
	sharedSecret = "ze-subscriber-fixture-secret" //nolint:gosec // G101: a fixture secret published in clients.conf, protecting nothing.
	// requestFile is the Disconnect-Request file each half's radclient sends.
	requestFile = "/tmp/ze-disconnect.req"
	// capturePath is the tcpdump capture of datagrams to ze's CoA port.
	capturePath = "/tmp/ze-coa.pcap"
	// disconnectedLine is the line coa.go handleDisconnect logs once for each
	// session it tears down; a request answered from the cache logs nothing.
	disconnectedLine = "coa: disconnected session"
	// replyWaitSeconds bounds how long the replaying process waits for ze's
	// answer to its one datagram.
	replyWaitSeconds = "5"
	// RFC 5176 Section 3's Codes: 40 Disconnect-Request, 42 Disconnect-NAK.
	radiusCodeDisconnectRequest = 40
	radiusCodeDisconnectNAK     = 42
)

// acctSessionIDShape is the id genSessionID writes: tunnel, session, the
// 24-hex-digit boot tag, and the per-boot counter.
var acctSessionIDShape = regexp.MustCompile(`^\d+-\d+-([0-9a-f]{24})-\d+$`)

type scenarioLab struct {
	lab     interoplab.CheckerLab
	timeout time.Duration
}

// accountingRecord is one Accounting-Request the server wrote to its detail file.
type accountingRecord struct {
	status    string
	sessionID string
}

// coaDatagram is one CoA or Disconnect request ze's container received:
// its header fields, and octets, the whole RADIUS packet as it arrived.
type coaDatagram struct {
	sourcePort    uint16
	code          byte
	identifier    byte
	authenticator [16]byte
	octets        []byte
}

// checkCoADuplicate proves RFC 5176 Section 2.3 duplicate detection against
// FreeRADIUS radclient, in two halves on two live sessions.
//
// A retransmission: ze is paused while radclient sends one Disconnect-Request
// and retransmits it on its own socket, so ze reads several copies with the
// same source port, Identifier and Authenticator. It answers the first and
// the rest from the cache: radclient gets a Disconnect-ACK and ze tears the
// session down once.
//
// Two clients: radclient sends one Disconnect-Request, and a second process
// in the server's container sends the very octets ze received from it, read
// out of the capture, from another source port. The pinned radclient cannot
// choose its Identifier, so a second radclient could not send the same
// packet. The second is not a duplicate: ze processes it afresh and, the
// session being gone, answers Disconnect-NAK. Under a key that ignores the
// port it would get the first one's cached Disconnect-ACK.
func checkCoADuplicate(ctx context.Context, lab *scenarioLab) error {
	first, err := lab.waitRecord(ctx, "Start", "")
	if err != nil {
		return err
	}
	if err := lab.startCapture(ctx); err != nil {
		return err
	}

	if err := lab.retransmittedDisconnect(ctx, first.sessionID); err != nil {
		return err
	}
	if err := lab.waitDisconnects(ctx, 1); err != nil {
		return err
	}

	if err := lab.redial(ctx); err != nil {
		return err
	}
	second, err := lab.waitRecord(ctx, "Start", first.sessionID)
	if err != nil {
		return err
	}
	if err := lab.writeRequest(ctx, second.sessionID); err != nil {
		return err
	}
	// The first client's datagram is the first one captured after the
	// retransmission half's, found by its index rather than as the last one:
	// a read can race the capture, and the last datagram is then the previous
	// half's request.
	held, err := lab.readCapture(ctx)
	if err != nil {
		return err
	}
	answer, err := lab.radclient(ctx, "1", "3")
	if err != nil {
		return err
	}
	if !strings.Contains(answer, "Received Disconnect-ACK") {
		return fmt.Errorf("first client: radclient did not receive a Disconnect-ACK:\n%s", answer)
	}
	sent, err := lab.capturedRequest(ctx, len(held))
	if err != nil {
		return err
	}
	code, err := lab.replay(ctx, sent.octets)
	if err != nil {
		return err
	}
	if code != radiusCodeDisconnectNAK {
		return fmt.Errorf("second client, radclient's request replayed from another port: ze answered Code %d, "+
			"want a fresh Disconnect-NAK (%d), not the first client's cached answer", code, radiusCodeDisconnectNAK)
	}
	if err := lab.waitDisconnects(ctx, 2); err != nil {
		return err
	}
	// The replayed datagram MUST be in the file before tcpdump stops, or the
	// capture judged below ends one datagram short.
	if _, err := lab.capturedRequest(ctx, len(held)+1); err != nil {
		return err
	}

	datagrams, err := lab.stopCapture(ctx)
	if err != nil {
		return err
	}
	return judgeCapture(datagrams)
}

// judgeCapture confirms the preconditions both halves rest on, from the
// datagrams ze received: the retransmission kept its port, Identifier and
// Authenticator, and the two clients' Disconnect-Requests are the same octets
// from two ports.
func judgeCapture(datagrams []coaDatagram) error {
	if len(datagrams) < 4 {
		return fmt.Errorf("ze received %d Disconnect-Requests, want the retransmitted ones and two clients' (at least 4)", len(datagrams))
	}
	clients := datagrams[len(datagrams)-2:]
	retransmitted := datagrams[:len(datagrams)-2]
	for _, copyN := range retransmitted[1:] {
		if copyN.sourcePort != retransmitted[0].sourcePort || copyN.identifier != retransmitted[0].identifier ||
			copyN.authenticator != retransmitted[0].authenticator {
			return fmt.Errorf("radclient's retransmission is not a duplicate: %+v after %+v", copyN, retransmitted[0])
		}
	}
	if clients[0].sourcePort == clients[1].sourcePort {
		return fmt.Errorf("the two clients sent from one source port %d", clients[0].sourcePort)
	}
	if clients[0].code != radiusCodeDisconnectRequest {
		return fmt.Errorf("the first client's datagram is not a Disconnect-Request: %+v", clients[0])
	}
	if !bytes.Equal(clients[0].octets, clients[1].octets) {
		return fmt.Errorf("the two clients' requests differ beyond the port: %+v and %+v", clients[0], clients[1])
	}
	return nil
}

// replayScript is the bash program that sends octets to ze's CoA port from a
// socket of its own, so from a new source port, and prints the Code of the
// one answer it reads as a decimal number, or nothing when none arrives
// within replyWaitSeconds. Every octet is a \xHH escape of bash's printf, so
// the program carries no quote and no format directive.
func replayScript(octets []byte) string {
	var tb textbuf.Buffer
	tb.Str("exec 3<>").Str(coaDevice).Str(" || exit 2\nprintf '")
	for index := range octets {
		tb.Str(`\x`).Hex(octets[index : index+1])
	}
	tb.Str("' >&3\ntimeout ").Str(replyWaitSeconds).Str(" dd bs=4096 count=1 status=none <&3 | od -An -tu1 -N1\n")
	return tb.String()
}

// replyCode reads the Code replayScript printed. No output is no answer.
func replyCode(output string) (byte, error) {
	field := strings.TrimSpace(output)
	if field == "" {
		return 0, errors.New("ze sent no answer to the replayed request within " + replyWaitSeconds + " seconds")
	}
	code, err := strconv.ParseUint(field, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("the replaying process printed %q, not a RADIUS Code: %w", field, err)
	}
	return byte(code), nil
}

// checkAcctRestart proves RFC 2866 Section 5.5 across a real daemon restart:
// the Accounting-Start before ze restarts and the one after carry ids of the
// form tunnel-session-tag-counter whose boot tags differ, and each Stop
// carries its Start's id, in FreeRADIUS's own detail file.
func checkAcctRestart(ctx context.Context, lab *scenarioLab) error {
	before, err := lab.closedSession(ctx, "")
	if err != nil {
		return err
	}
	if err := lab.lab.Stop(ctx, peerZe, 10); err != nil {
		return fmt.Errorf("stop ze: %w", err)
	}
	if err := lab.lab.Start(ctx, peerZe); err != nil {
		return fmt.Errorf("start ze again: %w", err)
	}
	if err := lab.waitZeListening(ctx); err != nil {
		return err
	}
	if err := lab.redial(ctx); err != nil {
		return err
	}
	after, err := lab.closedSession(ctx, before)
	if err != nil {
		return err
	}
	beforeTag, afterTag := acctSessionIDShape.FindStringSubmatch(before), acctSessionIDShape.FindStringSubmatch(after)
	if beforeTag == nil {
		return fmt.Errorf("Acct-Session-Id %q before the restart carries no boot tag", before)
	}
	if afterTag == nil {
		return fmt.Errorf("Acct-Session-Id %q after the restart carries no boot tag", after)
	}
	if beforeTag[1] == afterTag[1] {
		return fmt.Errorf("the boot tag %s repeated across a restart (%q, %q)", beforeTag[1], before, after)
	}
	return nil
}

// closedSession waits for an Accounting-Start whose id is not previous, clears
// the session through ze's REST surface, and waits for the Stop carrying the
// same id. It answers that id.
func (s *scenarioLab) closedSession(ctx context.Context, previous string) (string, error) {
	start, err := s.waitRecord(ctx, "Start", previous)
	if err != nil {
		return "", err
	}
	if err := s.clearSessions(ctx); err != nil {
		return "", err
	}
	if _, err := s.waitStop(ctx, start.sessionID); err != nil {
		return "", err
	}
	return start.sessionID, nil
}

// retransmittedDisconnect pauses ze, lets radclient send and retransmit one
// Disconnect-Request for sessionID on its socket, and resumes ze, which then
// reads every copy. radclient must end with a Disconnect-ACK.
func (s *scenarioLab) retransmittedDisconnect(ctx context.Context, sessionID string) error {
	if err := s.writeRequest(ctx, sessionID); err != nil {
		return err
	}
	if err := s.lab.Pause(ctx, peerZe); err != nil {
		return fmt.Errorf("pause ze: %w", err)
	}
	script := "radclient -x -r 5 -t 2 -f " + requestFile + " " + coaTarget +
		" disconnect " + sharedSecret + " > /tmp/ze-retransmit.out 2>&1"
	if err := s.lab.ExecDetached(ctx, peerServer, []string{"sh", "-c", script}, nil); err != nil {
		return errors.Join(fmt.Errorf("start the retransmitting radclient: %w", err), s.lab.Unpause(ctx, peerZe))
	}
	// Three sends at 2-second intervals land before ze resumes.
	if err := sleepContext(ctx, 5*time.Second); err != nil {
		return errors.Join(err, s.lab.Unpause(ctx, peerZe))
	}
	if err := s.lab.Unpause(ctx, peerZe); err != nil {
		return fmt.Errorf("resume ze: %w", err)
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "radclient's answer to its retransmitted request",
	}, func(probeCtx context.Context) (string, error) {
		return s.lab.Query(probeCtx, peerServer, []string{"cat", "/tmp/ze-retransmit.out"}, nil)
	}, func(output string) bool { return strings.Contains(output, "Received Disconnect-ACK") })
	if err != nil {
		return fmt.Errorf("retransmission: radclient received no Disconnect-ACK: %w", err)
	}
	return nil
}

// writeRequest writes the one Disconnect-Request file a half sends. The
// Event-Timestamp is written once, so every copy carries the same octets.
func (s *scenarioLab) writeRequest(ctx context.Context, sessionID string) error {
	var tb textbuf.Buffer
	tb.Str("Acct-Session-Id = \"").Str(sessionID).Str("\"\n").
		Str("Event-Timestamp = ").Int(time.Now().Unix()).Byte('\n').
		Str("Message-Authenticator = 0x00\n")
	result, err := s.lab.Exec(ctx, peerServer, []string{"sh", "-c", "printf '%s' '" + tb.String() + "' > " + requestFile}, nil)
	if err != nil {
		return fmt.Errorf("write %s: %w: %s", requestFile, err, result.Stderr)
	}
	return nil
}

// radclient sends the request file once as a new process, so from a new
// source port, and answers its output.
func (s *scenarioLab) radclient(ctx context.Context, retries, timeoutSeconds string) (string, error) {
	result, err := s.lab.Exec(ctx, peerServer, []string{"radclient", "-x",
		"-r", retries, "-t", timeoutSeconds, "-f", requestFile, coaTarget, "disconnect", sharedSecret}, nil)
	output := result.Stdout + result.Stderr
	// radclient exits non-zero on a NAK, which is an answer here, not a failure.
	if err != nil && !strings.Contains(output, "Received ") {
		return "", fmt.Errorf("radclient: %w: %s", err, output)
	}
	return output, nil
}

// capturedRequest waits until the running capture holds a datagram at index
// and answers it, which MUST be a Disconnect-Request. The caller passes the
// count the capture held before its client sent, so the answer is that
// client's datagram. A packet reaches ze before it reaches the file, so a
// read right after the client's answer can miss it: the wait closes that gap.
func (s *scenarioLab) capturedRequest(ctx context.Context, index int) (coaDatagram, error) {
	datagrams, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the client's datagram in the capture",
	}, s.readCapture, func(held []coaDatagram) bool { return len(held) > index })
	if err != nil {
		return coaDatagram{}, fmt.Errorf("the capture holds %d datagrams, want the client's at index %d: %w", len(datagrams), index, err)
	}
	request := datagrams[index]
	if request.code != radiusCodeDisconnectRequest {
		return coaDatagram{}, fmt.Errorf("the client's captured datagram is not a Disconnect-Request: %+v", request)
	}
	return request, nil
}

// replay sends octets to ze once from a new process in the server's
// container, so from the server's address and a new source port, and answers
// the Code of ze's reply.
func (s *scenarioLab) replay(ctx context.Context, octets []byte) (byte, error) {
	result, err := s.lab.Exec(ctx, peerServer, []string{"bash", "-c", replayScript(octets)}, nil)
	if err != nil {
		return 0, fmt.Errorf("replay radclient's request from another port: %w: %s", err, result.Stdout+result.Stderr)
	}
	return replyCode(result.Stdout)
}

// waitRecord waits for an accounting record of status whose id is not
// previous, and answers the first such record.
func (s *scenarioLab) waitRecord(ctx context.Context, status, previous string) (accountingRecord, error) {
	var found accountingRecord
	description := "an Accounting-" + status + " in the server's detail file"
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: s.timeout, Interval: time.Second, Description: description},
		func(probeCtx context.Context) ([]accountingRecord, error) { return s.records(probeCtx) },
		func(records []accountingRecord) bool {
			for _, record := range records {
				if record.status == status && record.sessionID != previous {
					found = record
					return true
				}
			}
			return false
		})
	if err != nil {
		return accountingRecord{}, fmt.Errorf("server detail file: no %s: %w", description, err)
	}
	return found, nil
}

// waitStop waits for the Accounting-Stop carrying sessionID.
func (s *scenarioLab) waitStop(ctx context.Context, sessionID string) (accountingRecord, error) {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: s.timeout, Interval: time.Second, Description: "the Accounting-Stop of " + sessionID},
		func(probeCtx context.Context) ([]accountingRecord, error) { return s.records(probeCtx) },
		func(records []accountingRecord) bool {
			for _, record := range records {
				if record.status == "Stop" && record.sessionID == sessionID {
					return true
				}
			}
			return false
		})
	if err != nil {
		return accountingRecord{}, fmt.Errorf("server detail file: no Accounting-Stop carrying its Start's id %q: %w", sessionID, err)
	}
	return accountingRecord{status: "Stop", sessionID: sessionID}, nil
}

// records reads the server's detail file. A record is a block of
// "\tName = value" lines ending at a blank line.
func (s *scenarioLab) records(ctx context.Context) ([]accountingRecord, error) {
	text, err := s.lab.Query(ctx, peerServer, []string{"cat", detailPath}, nil)
	if err != nil {
		return nil, err
	}
	return parseDetail(text), nil
}

func parseDetail(text string) []accountingRecord {
	var records []accountingRecord
	var current accountingRecord
	for line := range strings.SplitSeq(text+"\n\n", "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), " = ")
		if !found {
			if current.status != "" {
				records = append(records, current)
			}
			current = accountingRecord{}
			continue
		}
		switch name {
		case "Acct-Status-Type":
			current.status = value
		case "Acct-Session-Id":
			current.sessionID = strings.Trim(value, "\"")
		}
	}
	return records
}

// waitDisconnects waits until ze has logged exactly want teardowns by CoA.
// A count past want is a duplicate processed twice, and fails at once.
func (s *scenarioLab) waitDisconnects(ctx context.Context, want int) error {
	count := 0
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: s.timeout, Interval: time.Second, Description: "ze's CoA teardowns"},
		func(probeCtx context.Context) (int, error) {
			result, err := s.lab.Logs(probeCtx, peerZe, 5000)
			if err != nil {
				return 0, err
			}
			if !result.Available {
				return 0, errors.New("ze logs unavailable")
			}
			return strings.Count(result.Text, disconnectedLine), nil
		}, func(got int) bool { count = got; return got >= want })
	if err != nil {
		return fmt.Errorf("ze logged %d teardowns by CoA, want %d: %w", count, want, err)
	}
	if count != want {
		return fmt.Errorf("ze logged %d teardowns by CoA, want %d: a duplicate was processed again", count, want)
	}
	return nil
}

// redial restarts the LAC, so xl2tpd dials a new tunnel and session.
func (s *scenarioLab) redial(ctx context.Context) error {
	if err := s.lab.Stop(ctx, peerLAC, 5); err != nil {
		return fmt.Errorf("stop the LAC: %w", err)
	}
	if err := s.lab.Start(ctx, peerLAC); err != nil {
		return fmt.Errorf("start the LAC again: %w", err)
	}
	return nil
}

// waitZeListening waits for the restarted daemon's L2TP and CoA sockets.
func (s *scenarioLab) waitZeListening(ctx context.Context) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: s.timeout, Interval: time.Second, Description: "ze listening again"},
		func(probeCtx context.Context) (interoplab.CommandResult, error) {
			return s.lab.Exec(probeCtx, peerZe, []string{"sh", "-c", "ss -lun | grep -q ':1701 ' && ss -lun | grep -q ':3799 '"}, nil)
		}, func(result interoplab.CommandResult) bool { return result.ExitCode == 0 })
	if err != nil {
		return fmt.Errorf("ze did not listen again after its restart: %w", err)
	}
	return nil
}

// clearSessions asks ze, through its REST surface, to tear every L2TP session
// down, which sends each session's Accounting-Stop.
func (s *scenarioLab) clearSessions(ctx context.Context) error {
	result, err := s.lab.Exec(ctx, peerZe, []string{
		"wget", "-qO-",
		"--header=Authorization: Bearer secret",
		"--header=Content-Type: application/json",
		"--post-data={\"command\":\"clear l2tp session all\"}",
		"http://127.0.0.1:17012/api/v1/execute",
	}, nil)
	if err != nil {
		return fmt.Errorf("clear every L2TP session: %w: %s", err, result.Stdout+result.Stderr)
	}
	return nil
}

// startCapture records every datagram ze's container receives on the CoA port.
func (s *scenarioLab) startCapture(ctx context.Context) error {
	err := s.lab.ExecDetached(ctx, peerZe, []string{"tcpdump", "-n", "-U", "-i", "eth0", "-w", capturePath,
		"udp", "dst", "port", "3799"}, nil)
	if err != nil {
		return fmt.Errorf("start tcpdump in ze's container: %w", err)
	}
	return sleepContext(ctx, time.Second)
}

// stopCapture ends the capture and answers the CoA datagrams it holds.
func (s *scenarioLab) stopCapture(ctx context.Context) ([]coaDatagram, error) {
	if result, err := s.lab.Exec(ctx, peerZe, []string{"pkill", "-INT", "tcpdump"}, nil); err != nil {
		return nil, fmt.Errorf("stop tcpdump: %w: %s", err, result.Stderr)
	}
	if err := sleepContext(ctx, time.Second); err != nil {
		return nil, err
	}
	return s.readCapture(ctx)
}

// readCapture answers the CoA datagrams the capture file holds so far.
func (s *scenarioLab) readCapture(ctx context.Context) ([]coaDatagram, error) {
	encoded, err := s.lab.Query(ctx, peerZe, []string{"base64", capturePath}, nil)
	if err != nil {
		return nil, fmt.Errorf("read the capture: %w", err)
	}
	capture, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return nil, fmt.Errorf("decode the capture: %w", err)
	}
	return parseCapture(capture)
}

// parseCapture reads a classic pcap file of Ethernet frames and answers the
// RADIUS header of each IPv4 UDP datagram in it. Every length is checked
// against the bytes that remain before it is used.
//
// pcap global header 24 octets, then per frame a 16-octet record header whose
// octets 8..12 are the captured length; Ethernet 14 octets (type at 12..14);
// IPv4 header of IHL*4 octets (protocol at 9); UDP 8 octets (source port at
// 0..2, length of header and payload at 4..6); RADIUS Code at 0, Identifier
// at 1, Authenticator at 4..20.
func parseCapture(capture []byte) ([]coaDatagram, error) {
	if len(capture) < 24 {
		return nil, fmt.Errorf("capture of %d octets has no pcap header", len(capture))
	}
	var order binary.ByteOrder = binary.LittleEndian
	if bytes.Equal(capture[:4], []byte{0xa1, 0xb2, 0xc3, 0xd4}) {
		order = binary.BigEndian
	}
	var datagrams []coaDatagram
	for rest := capture[24:]; len(rest) >= 16; {
		captured := int(order.Uint32(rest[8:12]))
		if captured > len(rest)-16 {
			return nil, fmt.Errorf("capture record claims %d octets, %d remain", captured, len(rest)-16)
		}
		frame := rest[16 : 16+captured]
		rest = rest[16+captured:]
		if datagram, ok := radiusOfFrame(frame); ok {
			datagrams = append(datagrams, datagram)
		}
	}
	return datagrams, nil
}

func radiusOfFrame(frame []byte) (coaDatagram, bool) {
	if len(frame) < 14+20 || frame[12] != 0x08 || frame[13] != 0x00 {
		return coaDatagram{}, false
	}
	ip := frame[14:]
	headerOctets := int(ip[0]&0x0f) * 4
	if ip[9] != 17 || headerOctets < 20 || len(ip) < headerOctets+8+20 {
		return coaDatagram{}, false
	}
	udp := ip[headerOctets:]
	// The UDP length bounds the payload: a short frame may carry Ethernet
	// padding past it.
	udpOctets := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpOctets < 8+20 {
		return coaDatagram{}, false
	}
	if udpOctets > len(udp) {
		return coaDatagram{}, false
	}
	payload := udp[8:udpOctets]
	datagram := coaDatagram{sourcePort: binary.BigEndian.Uint16(udp[0:2]), code: payload[0], identifier: payload[1], octets: payload}
	copy(datagram.authenticator[:], payload[4:20])
	return datagram, true
}

// withDiagnostics appends each peer's recent log to a scenario failure.
func (s *scenarioLab) withDiagnostics(ctx context.Context, cause error) error {
	var diagnostics textbuf.Buffer
	for _, peer := range []string{peerZe, peerServer, peerLAC} {
		result, err := s.lab.Logs(ctx, peer, 80)
		if err != nil {
			diagnostics.Str("\n--- ").Str(peer).Str(" logs unavailable: ").Err(err).Str(" ---")
			continue
		}
		diagnostics.Str("\n--- ").Str(peer).Str(" logs ---\n").Str(result.Text)
	}
	return fmt.Errorf("%w%s", cause, diagnostics.String())
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
