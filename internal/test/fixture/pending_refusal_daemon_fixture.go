// Design: docs/architecture/behavior/peer-lifecycle.md -- failed pending refusals remain visible.
// Related: rfc4271_next_hop_daemon_fixture.go -- real OPEN and UPDATE wire producers.
// Related: daemon_await_answer_fixture.go -- native CLI reads through daemon SSH.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

const pendingRefusalCode = "doctor-bgp-pending-refusal-undelivered"
const pendingRefusalPeer = "127.0.0.2"
const bgpStateEstablished = "ESTABLISHED"

func init() {
	Register("plugin/pending-refusal-write-failure", pendingRefusalDaemon, StopsDaemon)
}

// pendingRefusalDaemon is outside the filtered process tree. Its two runs
// differ only in whether the kernel refuses the daemon's 21-byte writes.
// Each run owns and reaps its daemon before the next one can bind its ports.
func pendingRefusalDaemon(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("pending-refusal-write-failure requires the runner's BGP listener address")
	}
	for _, filtered := range []bool{true, false} {
		if err := pendingRefusalRun(ctx, args[0], filtered); err != nil {
			return fmt.Errorf("pending refusal filtered=%t: %w", filtered, err)
		}
	}
	return nil
}

func pendingRefusalRun(ctx context.Context, address string, filtered bool) (result error) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	argv := []string{"ze", actionStart, "running.conf"}
	if filtered {
		argv = append([]string{"le", "test", "fail-syscall", "syscall", "write", "errno", "EIO", "length", "21", "--"}, argv...)
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // Fixed test-owned programs and arguments.
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	// After a successful Start, the fixture MUST reap this owned daemon
	// with Wait on every path, including a failed assertion.
	if err := command.Start(); err != nil {
		return err
	}
	// Cleanup MUST join even when a wire or operator assertion fails. A
	// successful run asks for an orderly stop; cancellation handles failure.
	defer func() {
		if result != nil {
			cancel()
		}
		if err := command.Wait(); err != nil {
			result = errors.Join(result, fmt.Errorf("owned daemon exit: %w", err))
		}
	}()
	if err := pendingRefusalPoll(ctx, "show bgp peer "+pendingRefusalPeer+" detail", func(data []byte) bool {
		_, err := pendingRefusalPeerRow(data)
		// refuse-by-default: guards pendingRefusalPoll -- an unreadable peer row cannot satisfy the positive readiness barrier; timeout retains the last answer
		return err == nil
	}); err != nil {
		return err
	}
	// RFC 4271 Sections 4.2 and 4.4: complete the OPEN/KEEPALIVE exchange first.
	original, err := pendingRefusalConnect(ctx, address)
	if err != nil {
		return err
	}
	defer original.Close() //nolint:errcheck // The daemon result and assertions remain authoritative.
	if err := pendingRefusalEstablished(ctx, 0); err != nil {
		return err
	}
	// RFC 4271 Section 6.8 and RFC 4486 Section 4: only the new connection loses.
	endpoint, err := pendingRefusalCollide(ctx, address, filtered)
	if err != nil {
		return err
	}
	if err := pendingRefusalReports(ctx, endpoint, filtered); err != nil {
		return err
	}
	// A valid UPDATE, after the loser closed and its reports were read, proves
	// the original socket still carries useful BGP input. No reconnect occurs.
	// A received iBGP route may retain an unresolved next hop. Keep it outside
	// this host's loopback addresses: a host-owned NEXT_HOP is ignored before
	// publication by the real receive path (session_next_hop.go).
	// RFC 4271 Section 4.3: send an UPDATE on the original Established connection.
	if err := rfc4271NextHopUpdate(original, "198.51.100.0/24", "10.250.1.1", 100); err != nil {
		return err
	}
	if err := pendingRefusalEstablished(ctx, 1); err != nil {
		return err
	}
	if err := pendingRefusalPoll(ctx, "show bgp rib received", pendingRefusalReceived); err != nil {
		return err
	}
	if _, err := pendingRefusalCommand(ctx, "request quiesce"); err != nil {
		return err
	}
	if _, err := pendingRefusalCommand(ctx, "request shutdown"); err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stderr, "OK: pending refusal filtered=%t endpoint=%s closed; operator reports checked; original Established socket accepted subsequent UPDATE\n", filtered, endpoint)
	return err
}

func pendingRefusalConnect(ctx context.Context, address string) (net.Conn, error) {
	conn, err := pendingRefusalDial(ctx, address)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close() //nolint:errcheck // Failed handshake owns this socket.
		}
	}()
	// RFC 4271 Section 4.2: OPEN is the first message on this connection.
	if err := pendingRefusalOpen(conn); err != nil {
		return nil, err
	}
	// RFC 4271 Section 4.2: receive the daemon's OPEN before its confirmation.
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, err
	}
	// RFC 4271 Sections 4.2 and 4.4: require the KEEPALIVE confirming our OPEN.
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, err
	}
	// RFC 4271 Section 4.2: "If the OPEN message is acceptable, a
	// KEEPALIVE message confirming the OPEN is sent back."
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}

func pendingRefusalDial(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: &net.TCPAddr{IP: net.ParseIP(pendingRefusalPeer)}}
	conn, err := dialer.DialContext(ctx, "tcp4", address)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return conn, nil
}

// pendingRefusalOpen starts one connection without optional parameters.
// RFC 4271 Section 4.2: "After a TCP connection is established, the first message sent by each
// side is an OPEN message."
// RFC 4271 Sections 4.1 and 4.2, complete OPEN offsets in octets:
//
//	+---------+-------+------+---------+-------+-------+----------+---------+
//	| 0..15   | 16..17| 18   | 19      | 20..21| 22..23| 24..27   | 28      |
//	| Marker  | Len=29| OPEN | Version | My AS | Hold  | BGP ID   | Opt Len |
//	| all ff  |       | 1    | 4       | 65001 | 0     | 2.2.2.2  | 0       |
//	+---------+-------+------+---------+-------+-------+----------+---------+
func pendingRefusalOpen(conn net.Conn) error {
	// RFC 4271 Section 4.4: "If the negotiated Hold Time interval is zero, then periodic KEEPALIVE
	// messages MUST NOT be sent."
	// Hold zero removes timed keepalive workers from this socket's lifecycle.
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 0, BGPIdentifier: 0x02020202}
	_, err := conn.Write(message.PackTo(open, nil))
	return err
}

// pendingRefusalCollide requires closure of only the new OPEN-bearing connection.
// RFC 4271 Section 6.8: "Unless allowed via configuration, a connection collision with an
// existing BGP connection that is in the Established state causes closing of the newly created connection."
func pendingRefusalCollide(ctx context.Context, address string, filtered bool) (string, error) {
	conn, err := pendingRefusalDial(ctx, address)
	if err != nil {
		return "", err
	}
	defer conn.Close() //nolint:errcheck // EOF is asserted explicitly below.
	endpoint := conn.LocalAddr().String()
	// RFC 4271 Sections 4.2 and 6.8: supply the OPEN that triggers collision resolution.
	if err := pendingRefusalOpen(conn); err != nil {
		return "", err
	}
	// LimitReader bounds even a daemon that sends an unbounded wrong response.
	got, err := io.ReadAll(io.LimitReader(conn, 22))
	if err != nil {
		return "", fmt.Errorf("collision loser did not close with EOF: %w", err)
	}
	if filtered {
		if len(got) != 0 {
			return "", fmt.Errorf("filtered refusal delivered %x, want EOF without NOTIFICATION", got)
		}
		return endpoint, nil
	}
	// Literal bytes keep the control independent of the daemon's encoder:
	// 16 marker octets, Length 21, NOTIFICATION 3, Cease 6, Collision 7.
	// RFC 4486 Section 4: "If a BGP speaker decides to send a NOTIFICATION message with the
	// Error Code Cease as a result of the collision resolution procedure (as described in [BGP-4]),
	// then the subcode SHOULD be set to "Connection Collision Resolution"."
	want := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 21, 3, 6, 7}
	if !bytes.Equal(got, want) {
		return "", fmt.Errorf("unfiltered refusal delivered %x, want exact 21-byte Cease 6/7 %x then EOF", got, want)
	}
	return endpoint, nil
}

func pendingRefusalCommand(ctx context.Context, command string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", command+" | json")
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s exited %d: %s %s", command, code, stdout, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		return nil, fmt.Errorf("%s returned invalid JSON: %q", command, stdout)
	}
	return []byte(stdout), nil
}

func pendingRefusalPoll(ctx context.Context, command string, predicate func([]byte) bool) error {
	var last []byte
	var lastErr error
	if Poll(ctx, 40, 250*time.Millisecond, func() bool {
		last, lastErr = pendingRefusalCommand(ctx, command)
		if lastErr != nil {
			return false
		}
		return predicate(last)
	}) {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("%s did not reach asserted state: %w", command, lastErr)
	}
	return fmt.Errorf("%s did not reach asserted state: last answer %s", command, last)
}

type pendingRefusalPeerDetail struct {
	FSM         string `json:"fsm-state"`
	Established uint64 `json:"connections-established"`
	Dropped     uint64 `json:"connections-dropped"`
	Updates     uint64 `json:"updates-received"`
}

func pendingRefusalPeerRow(data []byte) (pendingRefusalPeerDetail, error) {
	var answer struct {
		Peers map[string]pendingRefusalPeerDetail `json:"peers"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return pendingRefusalPeerDetail{}, err
	}
	row, found := answer.Peers[pendingRefusalPeer]
	if !found {
		return row, errors.New("configured peer missing from daemon detail")
	}
	return row, nil
}

func pendingRefusalEstablished(ctx context.Context, updates uint64) error {
	return pendingRefusalPoll(ctx, "show bgp peer "+pendingRefusalPeer+" detail", func(data []byte) bool {
		row, err := pendingRefusalPeerRow(data)
		if err != nil {
			return false
		}
		if row.FSM != bgpStateEstablished {
			return false
		}
		if row.Established != 1 {
			return false
		}
		if row.Dropped != 0 {
			return false
		}
		return row.Updates == updates
	})
}

func pendingRefusalReceived(data []byte) bool {
	// The RIB producer streams records under "routes"; the native CLI's
	// explicit JSON pipe unwraps that single-key array (pipe.go applyJSON).
	var rows []struct {
		Peer      string `json:"peer"`
		Prefix    string `json:"prefix"`
		Direction string `json:"direction"`
	}
	// refuse-by-default: guards pendingRefusalPoll -- malformed JSON cannot satisfy the positive received-route barrier; timeout retains the last answer
	if err := json.Unmarshal(data, &rows); err != nil {
		return false
	}
	if len(rows) != 1 {
		return false
	}
	if rows[0].Peer != pendingRefusalPeer {
		return false
	}
	if rows[0].Prefix != "198.51.100.0/24" {
		return false
	}
	return rows[0].Direction == directionReceived
}

func pendingRefusalReports(ctx context.Context, endpoint string, filtered bool) error {
	for _, command := range []string{"show errors", "show doctor"} {
		data, err := pendingRefusalCommand(ctx, command)
		if err != nil {
			return err
		}
		var answer struct {
			Errors      []pendingRefusalReport `json:"errors"`
			Diagnostics []pendingRefusalReport `json:"diagnostics"`
		}
		if err := json.Unmarshal(data, &answer); err != nil {
			return err
		}
		rows := answer.Errors
		if command == "show doctor" {
			rows = answer.Diagnostics
		}
		count := 0
		for _, row := range rows {
			if row.Code != pendingRefusalCode {
				continue
			}
			count++
			if err := row.check(command, endpoint); err != nil {
				return err
			}
		}
		want := 0
		if filtered {
			want = 1
		}
		if count != want {
			return fmt.Errorf("%s reports %d pending refusal failures, want %d: %s", command, count, want, data)
		}
	}
	return nil
}

type pendingRefusalReport struct {
	Code    string `json:"code"`
	Source  string `json:"source"`
	Subject string `json:"subject"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
	Detail  struct {
		Operation string `json:"operation"`
	} `json:"detail"`
}

func (row pendingRefusalReport) check(command, endpoint string) error {
	if row.Fix != "ze show bgp peer list" {
		return fmt.Errorf("%s refusal has wrong fix: %+v", command, row)
	}
	for _, text := range []string{"write NOTIFICATION", "input/output error"} {
		if !strings.Contains(row.Message, text) {
			return fmt.Errorf("%s refusal lacks %q: %+v", command, text, row)
		}
	}
	if command == "show errors" {
		if row.Source != namespaceBGP {
			return fmt.Errorf("show errors refusal has wrong source: %+v", row)
		}
		if row.Subject != endpoint {
			return fmt.Errorf("show errors refusal subject %q, want second TCP endpoint %q", row.Subject, endpoint)
		}
		if row.Detail.Operation != "write NOTIFICATION" {
			return fmt.Errorf("show errors refusal has wrong operation: %+v", row)
		}
		return nil
	}
	if !strings.HasPrefix(row.Message, "bgp error event on "+endpoint+":") {
		return fmt.Errorf("show doctor refusal lacks source and second TCP endpoint %q: %+v", endpoint, row)
	}
	return nil
}
