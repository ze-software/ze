// Design: docs/architecture/testing/interop.md -- subscriber D-8 interop observations.
// Related: checkers.go -- the real FreeRADIUS radclient and accounting helpers.
// RFC 5176 Section 2.3 -- see rfc/short/rfc5176.md.
package radiussubscriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkCoADisconnect sends supported and unsupported changes through radclient
// to one real PPP session. Kernel rates, peer traffic and accounting distinguish
// a refusal from a partial apply or a disconnected subscriber.
// MUTATION: readCoAChange ignores the second unsupported Filter-Id; the mixed
// request then ACKs instead of NAKing, and this scenario refuses it.
func checkCoADisconnect(ctx context.Context, lab *scenarioLab) error {
	start, err := lab.waitRecord(ctx, "Start", "")
	if err != nil {
		return err
	}
	if err := lab.waitRate(ctx, 10_000_000); err != nil {
		return err
	}
	if err := lab.waitUploadRate(ctx, 2_000_000); err != nil {
		return err
	}
	if err := lab.subscriberPing(ctx); err != nil {
		return err
	}
	// RFC 5176 Section 2.3: supported changes must be made, not merely ACKed.
	if err := lab.dynamicRequest(ctx, start.sessionID, "coa", "Filter-Id = \"rate:20mbit/5mbit\"\n", "CoA-ACK", ""); err != nil {
		return err
	}
	if err := lab.waitRate(ctx, 20_000_000); err != nil {
		return err
	}
	if err := lab.waitUploadRate(ctx, 5_000_000); err != nil {
		return err
	}
	// RFC 5176 Section 2.3: "If the CoA-Request is unsuccessful for any matching
	// sessions, the NAS MUST send a CoA-NAK in reply, and the requested
	// authorization changes MUST NOT be made for any of the matching sessions."
	if err := lab.dynamicRequest(ctx, start.sessionID, "coa",
		"Filter-Id = \"rate:30mbit/7mbit\"\nFilter-Id += \"not-a-supported-filter\"\n",
		"CoA-NAK", "Invalid-Attribute-Value"); err != nil {
		return err
	}
	if err := lab.unchangedRate(ctx, 20_000_000); err != nil {
		return err
	}
	upload, err := lab.uploadRate(ctx)
	if err != nil {
		return err
	}
	if upload != 5_000_000 {
		return fmt.Errorf("refused CoA partially applied: kernel upload rate %d, want unchanged 5000000", upload)
	}
	// A Disconnect must contain only identification attributes (RFC 5176
	// Section 3). A Filter-Id must not tear down the matching live subscriber.
	if err := lab.dynamicRequest(ctx, start.sessionID, "disconnect", "Filter-Id = \"rate:30mbit\"\n",
		"Disconnect-NAK", "Unsupported-Attribute"); err != nil {
		return err
	}
	if err := lab.subscriberPing(ctx); err != nil {
		return err
	}
	if err := lab.dynamicRequest(ctx, start.sessionID, "disconnect", "", "Disconnect-ACK", ""); err != nil {
		return err
	}
	if _, err := lab.waitStop(ctx, start.sessionID); err != nil {
		return err
	}
	if err := lab.waitDisconnects(ctx, 1); err != nil {
		return err
	}
	// The second request is new, not a cached replay, and cannot claim success
	// for the session the preceding ACK removed (RFC 5176 Section 3).
	return lab.dynamicRequest(ctx, start.sessionID, "disconnect", "", "Disconnect-NAK", "Session-Context-Not-Found")
}

func (s *scenarioLab) dynamicRequest(ctx context.Context, sessionID, kind, attributes, reply, cause string) error {
	var request textbuf.Buffer
	request.Str("Acct-Session-Id = \"").Str(sessionID).Str("\"\n").
		Str("Event-Timestamp = ").Int(time.Now().Unix()).Byte('\n').
		Str("Message-Authenticator = 0x00\n").Str(attributes)
	// The script reads its data as a positional argument, never as shell syntax.
	result, err := s.lab.Exec(ctx, peerServer, []string{"sh", "-c", "printf '%s' \"$1\" > " + requestFile, "request", request.String()}, nil)
	if err != nil {
		return fmt.Errorf("write dynamic authorization request: %w: %s", err, result.Stderr)
	}
	result, err = s.lab.Exec(ctx, peerServer, []string{"radclient", "-x", "-r", "1", "-t", "5",
		"-f", requestFile, coaTarget, kind, sharedSecret}, nil)
	output := result.Stdout + result.Stderr
	if err != nil && !strings.Contains(output, "Received ") {
		return fmt.Errorf("radclient %s: %w: %s", kind, err, output)
	}
	return dynamicReply(output, reply, cause)
}

func dynamicReply(output, reply, cause string) error {
	_, received, found := strings.Cut(output, "Received ")
	if !found {
		return fmt.Errorf("radclient received no answer: %s", output)
	}
	line, attributes, _ := strings.Cut(received, "\n")
	if !strings.HasPrefix(line, reply+" ") {
		return fmt.Errorf("radclient received %q, want %s: %s", line, reply, output)
	}
	if cause != "" {
		for attribute := range strings.SplitSeq(attributes, "\n") {
			if strings.TrimSpace(attribute) == "Error-Cause = "+cause {
				return nil
			}
		}
		return fmt.Errorf("radclient %s missing Error-Cause %s: %s", reply, cause, output)
	}
	return nil
}

// tbfRate reads iproute2's numeric byte/second rate and converts it to bit/second.
// Exactly one PPP root TBF must exist; an absent qdisc is not a zero rate.
func tbfRate(output string) (uint64, error) {
	var qdiscs []struct {
		Kind    string `json:"kind"`
		Device  string `json:"dev"`
		Root    bool   `json:"root"`
		Options struct {
			Rate uint64 `json:"rate"`
		} `json:"options"`
	}
	if err := json.Unmarshal([]byte(output), &qdiscs); err != nil {
		return 0, fmt.Errorf("decode kernel qdiscs: %w", err)
	}
	var rate uint64
	count := 0
	for _, qdisc := range qdiscs {
		if qdisc.Kind != "tbf" {
			continue
		}
		if !qdisc.Root {
			continue
		}
		if !strings.HasPrefix(qdisc.Device, "ppp") {
			continue
		}
		if qdisc.Options.Rate == 0 {
			return 0, errors.New("kernel TBF has no nonzero rate")
		}
		if qdisc.Options.Rate > ^uint64(0)/8 {
			return 0, errors.New("kernel TBF rate overflows bit/second conversion")
		}
		rate = qdisc.Options.Rate * 8
		count++
	}
	if count != 1 {
		return 0, fmt.Errorf("kernel has %d PPP root TBF qdiscs, want one: %s", count, output)
	}
	return rate, nil
}

func (s *scenarioLab) kernelRate(ctx context.Context) (uint64, error) {
	output, err := s.lab.Query(ctx, peerZe, []string{"tc", "-j", "qdisc", "show"}, nil)
	if err != nil {
		return 0, err
	}
	return tbfRate(output)
}

func (s *scenarioLab) waitRate(ctx context.Context, want uint64) error {
	rate, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "the acknowledged kernel shaping rate",
	}, s.kernelRate, func(rate uint64) bool { return rate == want })
	if err != nil {
		return fmt.Errorf("kernel download rate %d, want %d: %w", rate, want, err)
	}
	return nil
}

func (s *scenarioLab) unchangedRate(ctx context.Context, want uint64) error {
	if err := s.subscriberPing(ctx); err != nil {
		return err
	}
	rate, err := s.kernelRate(ctx)
	if err != nil {
		return err
	}
	if rate != want {
		return fmt.Errorf("refused CoA partially applied: kernel rate %d, want unchanged %d", rate, want)
	}
	return nil
}

func (s *scenarioLab) subscriberPing(ctx context.Context) error {
	result, err := s.lab.Exec(ctx, peerLAC, []string{"ping", "-I", "ppp0", "-c", "3", "-W", "2", "10.101.0.1"}, nil)
	if err != nil {
		return fmt.Errorf("subscriber PPP traffic failed: %w: %s", err, result.Stdout+result.Stderr)
	}
	return nil
}
