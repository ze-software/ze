// Design: docs/architecture/testing/ci-format.md — the reject=bgp: directive
// Overview: checker.go — the positive half, expect=bgp:
// Related: peer.go — the message loop that calls rejected on every frame

package peer

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ParseRejectRule reads one `reject=bgp:conn=N:pattern=<hex>` line.
//
// isReject is false for any other line, and then conn and pattern are unset.
//
// This is the ONE parser for the directive. It is exported because the test
// runner's peer-block guard must reach the same verdict ze-peer will
// (`validatePeerBlockRejects`, internal/test/runner/peer_contract.go).
//
// A second hand-written reader is the defect this file exists to remove. The
// guard once split the line on ':' itself. `reject=bgp:pattern=AABB:conn=1`
// then satisfied the guard and failed inside ze-peer, and the runner reported
// that as a peer which never bound rather than as the typo it is.
func ParseRejectRule(rule string) (conn int, pattern string, isReject bool, err error) {
	after, ok := strings.CutPrefix(strings.TrimSpace(rule), "reject=bgp:")
	if !ok {
		return 0, "", false, nil
	}
	kv := parseKV(after)
	connStr := kv["conn"]
	if connStr == "" {
		return 0, "", true, fmt.Errorf("reject=bgp missing conn: %q", rule)
	}
	conn, cerr := strconv.Atoi(connStr)
	if cerr != nil || conn < 1 {
		return 0, "", true, fmt.Errorf("reject=bgp invalid conn=%q (must be >= 1): %q", connStr, rule)
	}
	pattern = strings.ToUpper(kv["pattern"])
	if pattern == "" {
		return 0, "", true, fmt.Errorf("reject=bgp missing pattern: %q", rule)
	}
	// Both checks refuse a needle that can never match a wire byte. An
	// odd-length or non-hex pattern is never found, so the rejection passes
	// for the one reason a rejection must never pass.
	if len(pattern)%2 != 0 {
		return 0, "", true, fmt.Errorf("reject=bgp pattern %q has an odd hex length, so it can never align to a wire byte: %q", pattern, rule)
	}
	if strings.Trim(pattern, "0123456789ABCDEF") != "" {
		return 0, "", true, fmt.Errorf("reject=bgp pattern %q is not hexadecimal: %q", pattern, rule)
	}
	return conn, pattern, true, nil
}

// splitRejectRules separates `reject=bgp:` rules from the expectation rules.
//
// A rejection is not an expectation: it names bytes that must never arrive, so
// it carries no seq= and never advances a sequence. Keeping the two apart here
// means groupMessages still sees only rules parseExpectRule understands.
func splitRejectRules(rules []string) (expect []string, rejects map[int][]string, err error) {
	for _, rule := range rules {
		conn, pattern, isReject, perr := ParseRejectRule(rule)
		if perr != nil {
			return nil, nil, perr
		}
		if !isReject {
			expect = append(expect, rule)
			continue
		}
		if rejects == nil {
			rejects = make(map[int][]string)
		}
		rejects[conn] = append(rejects[conn], pattern)
	}
	return expect, rejects, nil
}

// hasRejections reports whether any `reject=bgp:` rule was loaded.
func (c *Checker) hasRejections() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.rejects) > 0
}

// rejection reports the first `reject=bgp:` needle the message carries on the
// connection the checker is matching, and whether one was found.
//
// A rejection is never consumed. Every frame the message loop reads is checked
// against it, for as long as that connection lives. That includes the frames a
// lingering peer reads after its expectations are complete. A negative
// assertion that stopped at the last positive one would pass whenever the
// forbidden route arrived one frame late.
//
// The OPEN handshake and the connmap post-batch drain sit outside the message
// loop, so they are outside this check. Neither carries an UPDATE.
//
// conn comes from currentConnection, which is the connection the expectation
// sequence is on. That is the connection the frame arrived on only while the
// peer reads one connection at a time, which is check mode. New refuses a
// rejection in any other mode for exactly that reason.
func (c *Checker) rejection(msg *Message) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.rejects) == 0 {
		return "", false
	}
	// currentConnection is 0 only before Init has loaded the first sequence,
	// which is also the only time the frames read belong to connection 1.
	conn := c.currentConnection
	if conn == 0 {
		conn = 1
	}
	needles := c.rejects[conn]
	if len(needles) == 0 {
		return "", false
	}
	stream := msg.Stream()
	for _, needle := range needles {
		if indexByteAligned(stream, needle, 0) >= 0 {
			return needle, true
		}
	}
	return "", false
}

// rejected fails the peer when a frame carries bytes a `reject=bgp:` forbids.
// The frame is printed first: the pattern alone says which rule fired, and the
// reader still has to see the message that broke it.
func (p *Peer) rejected(msg *Message) (Result, bool) {
	needle, found := p.checker.rejection(msg)
	if !found {
		return Result{}, false
	}
	p.printPayload("msg  recv", msg.Header, msg.Body)
	return Result{Success: false, Error: fmt.Errorf(
		"received bytes that reject=bgp:pattern=%s forbids, in %s", needle, msg.Stream())}, true
}

// completed is returned at every check-mode completion site. Without linger it
// reports success, and the connection closes when the caller returns.
//
// With linger it announces success on the peer's output NOW, because teardown
// is a kill and the post-Run "successful" print can be lost. It then holds the
// session open until the test ends or the remote closes.
//
// The linger loop reads to keep the session alive, and it re-checks every frame
// it reads against the rejections. That is what makes `option=linger` the way to
// hold a negative assertion open for the rest of the test.
func (p *Peer) completed(ctx context.Context, conn net.Conn) Result {
	if !p.config.Linger {
		return Result{Success: true}
	}
	p.printf("\nsuccessful\n")
	p.printf("lingering: holding the session open until teardown (option=linger)\n")
	for {
		select {
		case <-ctx.Done():
			return Result{Success: true}
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		header, body, err := ReadMessage(conn)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			// EOF/reset after completion: the exchange already validated.
			return Result{Success: true}
		}
		if res, rejected := p.rejected(&Message{Header: header, Body: body}); rejected {
			return res
		}
		if _, err := conn.Write(KeepaliveMsg()); err != nil {
			// Remote closed after completion: the exchange already validated.
			return Result{Success: true}
		}
	}
}
