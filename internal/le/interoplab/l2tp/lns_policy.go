// Design: docs/labs/l2tp-interop.md -- LNS MRU and CHAP-only interoperability.
// Related: l2tp.go -- declarative scenario discovery and peer preparation.
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const lnsPolicyTrace = "/run/lns-policy-pppd.log"

type lnsPolicySession struct {
	ID       uint16  `json:"local-sid"`
	Username string  `json:"username"`
	Address  string  `json:"assigned-addr"`
	MRU      *uint16 `json:"mru"`
	Auth     *string `json:"auth-method"`
}

type lnsPolicyConfig struct {
	MRU   uint16   `json:"mru"`
	Auth  string   `json:"auth-method"`
	Allow []string `json:"authentication-allow"`
}

// checkLNSMRU requires pppd's own decoding of both LCP directions, CHAP success,
// a measured client MTU and traffic, then queries the daemon's actual session.
// The client requests 1493 and must converge to 1492 after Ze's correlated
// Nak. The session reports the resulting peer receive MRU, not a config copy.
func checkLNSMRU(ctx context.Context, lab labOperations, timeout time.Duration) error {
	if err := waitPeerLog(ctx, lab, peerZe, logSessionIPLine, timeout); err != nil {
		return err
	}
	trace, err := readLNSPolicyTrace(ctx, lab)
	if err != nil {
		return err
	}
	if err := lnsMRUEvidence(trace); err != nil {
		return interoplab.WithSearchedOutput(err, trace)
	}
	link, err := waitPPPInterface(ctx, lab, peerLAC, timeout)
	if err != nil {
		return err
	}
	output, err := execOutput(ctx, lab, peerLAC, []string{"ip", "-j", "link", "show", "dev", link})
	if err != nil {
		return err
	}
	if err := lnsClientMTU(output); err != nil {
		return interoplab.WithSearchedOutput(err, output)
	}
	if err := pingThroughSession(ctx, lab); err != nil {
		return err
	}
	if _, err := waitRadiusLine(ctx, lab, "Access-Request", "User-Name=alice", timeout); err != nil {
		return err
	}
	if err := checkLNSPolicyOutput(ctx, lab); err != nil {
		return err
	}
	if err := retainLNSPolicyEvidence(ctx, lab, trace, output); err != nil {
		return err
	}
	if err := stopLACDaemon(ctx, lab); err != nil {
		return err
	}
	return waitL2TPClean(ctx, lab, timeout)
}

// checkLNSCHAPOnly observes one complete pppd attempt, including Ze's refusal
// before authentication. The pinned pppd's fsm_rtermreq acknowledges TermReq in ReqSent
// but keeps negotiating, so process exit after the lower-layer teardown is the barrier,
// not reception of TermReq alone. The independent RADIUS backend must see no
// credential request; a missing PPP link alone proves none of these claims.
func checkLNSCHAPOnly(ctx context.Context, lab labOperations, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: "Ze TermReq reaching PAP-only pppd",
	}, func(probeCtx context.Context) (string, error) {
		return readLNSPolicyTrace(probeCtx, lab)
	}, func(trace string) bool { return strings.Contains(trace, "rcvd [LCP TermReq ") })
	if err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: "pppd exit after Ze lower-layer teardown",
	}, func(probeCtx context.Context) (string, error) {
		return execOutput(probeCtx, lab, peerLAC, []string{"sh", "-c",
			"if pidof pppd >/dev/null; then echo running; else echo exited; fi"})
	}, func(state string) bool { return strings.TrimSpace(state) == "exited" })
	if err != nil {
		return err
	}
	trace, err := readLNSPolicyTrace(ctx, lab)
	if err != nil {
		return err
	}
	if err := lnsCHAPOnlyEvidence(trace); err != nil {
		return interoplab.WithSearchedOutput(err, trace)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: time.Second, Description: "refused LNS session removed from daemon",
	}, func(probeCtx context.Context) ([]lnsPolicySession, error) {
		return lnsPolicySessions(probeCtx, lab)
	}, func(sessions []lnsPolicySession) bool { return len(sessions) == 0 })
	if err != nil {
		return err
	}
	logs, err := lab.Logs(ctx, peerZe, 1000)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("Ze refusal logs unavailable")
	}
	if strings.Contains(logs.Text, "PPP session up") {
		return interoplab.WithSearchedOutput(errors.New("CHAP-only refusal reached PPP session up"), logs.Text)
	}
	for _, detail := range []string{"auth-method-refused: peer suggested", "(pap)", "allowed [chap-md5]"} {
		if !strings.Contains(logs.Text, detail) {
			return interoplab.WithSearchedOutput(fmt.Errorf("Ze refusal log lacks %q", detail), logs.Text)
		}
	}
	radius, err := lab.Logs(ctx, peerRadius, 1000)
	if err != nil {
		return err
	}
	if err := lnsNoCredentialRequest(radius); err != nil {
		return err
	}
	// Only after proving Ze removed the refused session do we stop the LAC to
	// remove its idle control tunnel. This cannot manufacture the refusal.
	if err := retainLNSPolicyEvidence(ctx, lab, trace, logs.Text, radius.Text); err != nil {
		return err
	}
	if err := stopLACDaemon(ctx, lab); err != nil {
		return err
	}
	return waitL2TPClean(ctx, lab, timeout)
}

func readLNSPolicyTrace(ctx context.Context, lab labOperations) (string, error) {
	return execOutput(ctx, lab, peerLAC, []string{"cat", lnsPolicyTrace})
}

// lnsTracePacket returns pppd's decoded packet identifier and line index. It
// consumes only bracketed packet records, not configuration or diagnostic text.
func lnsTracePacket(lines []string, start int, prefix string, options ...string) (byte, int, error) {
	for index := start; index < len(lines); index++ {
		_, packet, found := strings.Cut(lines[index], prefix)
		if !found {
			continue
		}
		packet, _, found = strings.Cut(packet, "]")
		if !found {
			return 0, 0, errors.New("pppd packet record lacks closing bracket")
		}
		fields := strings.Fields(packet)
		if len(fields) == 0 {
			return 0, 0, errors.New("pppd packet record lacks identifier")
		}
		if !strings.HasPrefix(fields[0], "id=0x") {
			return 0, 0, fmt.Errorf("pppd packet identifier malformed: %s", fields[0])
		}
		identifier, err := strconv.ParseUint(strings.TrimPrefix(fields[0], "id=0x"), 16, 8)
		if err != nil {
			return 0, 0, fmt.Errorf("parse pppd packet identifier: %w", err)
		}
		matches := true
		for _, option := range options {
			if !strings.Contains(packet, option) {
				matches = false
			}
		}
		if matches {
			return byte(identifier), index, nil
		}
	}
	return 0, 0, fmt.Errorf("pppd trace lacks %s with %v", prefix, options)
}

func lnsSingleAttempt(trace string) error {
	if strings.Count(trace, "Connect:") != 1 {
		return errors.New("expected exactly one pppd Connect: record (redial/persist must be disabled)")
	}
	if strings.Contains(trace, "[PAP ") {
		return errors.New("PAP protocol traffic appeared")
	}
	return nil
}

func lnsMRUEvidence(trace string) error {
	lines, localAck, err := lnsLocalMRUAgreement(trace)
	if err != nil {
		return err
	}
	requestID, index, err := lnsTracePacket(lines, 0, "sent [LCP ConfReq ", "<mru 1493>")
	if err != nil {
		return err
	}
	nakID, index, err := lnsTracePacket(lines, index+1, "rcvd [LCP ConfNak ", "<mru 1492>")
	if err != nil {
		return err
	}
	if nakID != requestID {
		return errors.New("Ze's MRU1492 Nak identifier does not match pppd's MRU1493 request")
	}
	retryID, index, err := lnsTracePacket(lines, index+1, "sent [LCP ConfReq ", "<mru 1492>")
	if err != nil {
		return err
	}
	ackID, index, err := lnsTracePacket(lines, index+1, "rcvd [LCP ConfAck ", "<mru 1492>")
	if err != nil {
		return err
	}
	if ackID != retryID {
		return errors.New("Ze's MRU1492 Ack identifier does not match pppd's converged request")
	}
	return lnsCHAPSuccess(lines, max(localAck, index)+1)
}

// lnsLocalMRUAgreement preserves the independent proof that Ze advertises its
// configured receive policy and pppd acknowledges those exact MRU/auth options.
func lnsLocalMRUAgreement(trace string) ([]string, int, error) {
	if err := lnsSingleAttempt(trace); err != nil {
		return nil, 0, err
	}
	lines := strings.Split(trace, "\n")
	id, index, err := lnsTracePacket(lines, 0, "rcvd [LCP ConfReq ", "<mru 1492>", "<auth chap MD5>")
	if err != nil {
		return nil, 0, err
	}
	ackID, index, err := lnsTracePacket(lines, index+1, "sent [LCP ConfAck ", "<mru 1492>", "<auth chap MD5>")
	if err != nil {
		return nil, 0, err
	}
	if ackID != id {
		return nil, 0, errors.New("pppd did not acknowledge Ze's MRU1492/CHAP request identifier")
	}
	return lines, index, nil
}

func lnsCHAPSuccess(lines []string, start int) error {
	for _, prefix := range []string{"rcvd [CHAP Challenge ", "sent [CHAP Response ", "rcvd [CHAP Success "} {
		_, next, packetErr := lnsTracePacket(lines, start, prefix)
		if packetErr != nil {
			return packetErr
		}
		start = next + 1
	}
	return nil
}

func lnsCHAPOnlyEvidence(trace string) error {
	if err := lnsSingleAttempt(trace); err != nil {
		return err
	}
	for _, forbidden := range []string{"[CHAP ", "[IPCP ", "local  IP address", "remote IP address"} {
		if strings.Contains(trace, forbidden) {
			return fmt.Errorf("refused negotiation progressed past LCP: %s", forbidden)
		}
	}
	lines := strings.Split(trace, "\n")
	id, index, err := lnsTracePacket(lines, 0, "rcvd [LCP ConfReq ", "<auth chap MD5>")
	if err != nil {
		return err
	}
	nakID, index, err := lnsTracePacket(lines, index+1, "sent [LCP ConfNak ", "<auth pap>")
	if err != nil {
		return err
	}
	if nakID != id {
		return errors.New("PAP Nak identifier does not match Ze's CHAP request")
	}
	termID, index, err := lnsTracePacket(lines, index+1, "rcvd [LCP TermReq ")
	if err != nil {
		return err
	}
	// pppd's sent packets are received by Ze. An Ack of Ze's request can
	// complete LCP before the later refusal; Ze's Ack of the peer's own
	// request alone can legitimately leave Ze in Ack-Sent instead.
	var requests [256]bool
	inAttempt := false
	for _, raw := range lines[:index] {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "Connect:") {
			inAttempt = true
			continue
		}
		if !inAttempt {
			continue
		}
		prefix := "rcvd [LCP ConfReq "
		request := strings.HasPrefix(line, prefix)
		if !request {
			prefix = "sent [LCP ConfAck "
			if !strings.HasPrefix(line, prefix) {
				continue
			}
		}
		packetID, _, err := lnsTracePacket([]string{line}, 0, prefix)
		if err != nil {
			return err
		}
		if request {
			requests[packetID] = true
		} else if requests[packetID] {
			return errors.New("PAP-only peer acknowledged Ze's LCP request before Terminate-Request: LCP could open")
		}
	}
	if !inAttempt {
		return errors.New("pppd trace lacks the checked connection attempt")
	}
	ackID, _, err := lnsTracePacket(lines, index+1, "sent [LCP TermAck ")
	if err != nil {
		return err
	}
	if termID != ackID {
		return errors.New("pppd TermAck identifier does not match Ze's TermReq")
	}
	for _, line := range lines {
		if strings.Contains(line, "rcvd [LCP ConfReq ") && strings.Contains(line, "<auth pap>") {
			return errors.New("Ze offered PAP after the disallowed Nak")
		}
	}
	return nil
}

func lnsClientMTU(output string) error {
	return lnsMeasuredLinkMTU(output, 1492)
}

func lnsMeasuredLinkMTU(output string, wantMTU uint16) error {
	var links []struct {
		MTU uint16 `json:"mtu"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return fmt.Errorf("decode measured link MTU: %w", err)
	}
	if len(links) != 1 {
		return fmt.Errorf("expected one measured PPP link, got %d", len(links))
	}
	if links[0].MTU != wantMTU {
		return fmt.Errorf("measured PPP MTU is %d, want %d", links[0].MTU, wantMTU)
	}
	return nil
}

func lnsNoCredentialRequest(logs interoplab.LogResult) error {
	if !logs.Available {
		return errors.New("independent RADIUS backend logs unavailable")
	}
	if !strings.Contains(logs.Text, "radius-mock listening") {
		return interoplab.WithSearchedOutput(errors.New("RADIUS backend never became ready"), logs.Text)
	}
	if strings.Contains(logs.Text, "RADIUS-RX ") {
		return interoplab.WithSearchedOutput(errors.New("refused LCP reached the subscriber backend"), logs.Text)
	}
	return nil
}

func checkLNSPolicyOutput(ctx context.Context, lab labOperations) error {
	return checkLNSPolicyOutputForMRU(ctx, lab, 1492)
}

func checkLNSPolicyOutputForMRU(ctx context.Context, lab labOperations, peerMRU uint16) error {
	data, err := lnsPolicyCommand(ctx, lab, "show l2tp config")
	if err != nil {
		return err
	}
	var config lnsPolicyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode LNS effective config: %w", err)
	}
	if config.MRU != 1492 {
		return fmt.Errorf("daemon effective MRU is %d, want 1492", config.MRU)
	}
	if config.Auth != "chap-md5" {
		return fmt.Errorf("daemon effective auth-method is %q, want chap-md5", config.Auth)
	}
	if !slices.Equal(config.Allow, []string{"chap-md5"}) {
		return fmt.Errorf("daemon effective authentication-allow is %v, want [chap-md5]", config.Allow)
	}
	sessions, err := lnsPolicySessions(ctx, lab)
	if err != nil {
		return err
	}
	if len(sessions) != 1 {
		return fmt.Errorf("daemon reports %d sessions, want one live CHAP session", len(sessions))
	}
	if sessions[0].ID == 0 {
		return errors.New("daemon session lacks a nonzero local-sid")
	}
	var command textbuf.Buffer
	data, err = lnsPolicyCommand(ctx, lab, command.Str("show l2tp session id ").Str(strconv.Itoa(int(sessions[0].ID))).String())
	if err != nil {
		return err
	}
	var session lnsPolicySession
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("decode LNS live session detail: %w", err)
	}
	if session.ID != sessions[0].ID {
		return errors.New("daemon session detail returned another session")
	}
	return lnsNegotiatedSessionMRU(session, peerMRU)
}

func lnsNegotiatedSession(session lnsPolicySession) error {
	return lnsNegotiatedSessionMRU(session, 1492)
}

func lnsNegotiatedSessionMRU(session lnsPolicySession, peerMRU uint16) error {
	if session.Username != "alice" {
		return fmt.Errorf("daemon session username is %q, want alice", session.Username)
	}
	if session.Address != peerPPPAddress {
		return fmt.Errorf("daemon session address is %q, want %s", session.Address, peerPPPAddress)
	}
	if session.MRU == nil {
		return errors.New("daemon session has no negotiated peer MRU")
	}
	if *session.MRU != peerMRU {
		return fmt.Errorf("daemon negotiated peer MRU is %d, want client's %d", *session.MRU, peerMRU)
	}
	if session.Auth == nil {
		return errors.New("daemon session has no negotiated authentication method")
	}
	if *session.Auth != "chap-md5" {
		return fmt.Errorf("daemon negotiated auth-method is %q, want chap-md5", *session.Auth)
	}
	return nil
}

func lnsPolicySessions(ctx context.Context, lab labOperations) ([]lnsPolicySession, error) {
	data, err := lnsPolicyCommand(ctx, lab, "show l2tp sessions")
	if err != nil {
		return nil, err
	}
	var sessions []lnsPolicySession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("decode daemon L2TP session list: %w", err)
	}
	return sessions, nil
}

func lnsPolicyCommand(ctx context.Context, lab labOperations, command string) (json.RawMessage, error) {
	var body textbuf.Buffer
	request := body.Str(`--post-data={"command":`).Quoted(command).Byte('}').String()
	answer, err := execOutput(ctx, lab, peerZe, []string{
		"wget", "-qO-", "--header=Authorization: Bearer secret",
		"--header=Content-Type: application/json", request,
		"http://127.0.0.1:17012/api/v1/execute",
	})
	if err != nil {
		return nil, fmt.Errorf("daemon %s: %w", command, err)
	}
	var response struct {
		Status string          `json:"status"`
		Error  string          `json:"error"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(answer), &response); err != nil {
		return nil, interoplab.WithSearchedOutput(fmt.Errorf("decode daemon %s: %w", command, err), answer)
	}
	if response.Status != "done" {
		return nil, interoplab.WithSearchedOutput(fmt.Errorf("daemon %s failed: %s", command, response.Error), answer)
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("daemon %s has no data", command)
	}
	if string(response.Data) == "null" {
		return nil, fmt.Errorf("daemon %s has null data", command)
	}
	if _, err := fmt.Fprintln(os.Stdout, "LNS daemon evidence:", command, string(response.Data)); err != nil {
		return nil, fmt.Errorf("retain daemon evidence: %w", err)
	}
	return response.Data, nil
}

func retainLNSPolicyEvidence(ctx context.Context, lab labOperations, evidence ...string) error {
	for _, text := range evidence {
		if _, err := fmt.Fprintln(os.Stdout, "LNS policy evidence:\n"+text); err != nil {
			return fmt.Errorf("retain LNS policy evidence: %w", err)
		}
	}
	// Read the final trace too, after all assertions and before container cleanup.
	trace, err := readLNSPolicyTrace(ctx, lab)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "LNS final pppd trace:\n"+trace); err != nil {
		return fmt.Errorf("retain final pppd trace: %w", err)
	}
	return nil
}
