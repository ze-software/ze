// Design: docs/architecture/testing/interop.md -- the pure readings of the session-protection probes.
// Related: check_session_protection.go -- the checker body that runs the probes and reads ze's table.
package bgp

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// sessionProbeOutcome is what one TCP connection attempt to ze's listener saw.
// Its zero is no outcome: a probe line the parser did not recognize.
type sessionProbeOutcome uint8

const (
	sessionProbeUnspecified sessionProbeOutcome = iota
	// sessionProbeConnected: the handshake completed, so ze's kernel answered
	// the SYN with a SYN-ACK.
	sessionProbeConnected
	// sessionProbeRefused: ze's kernel answered the SYN with a reset.
	sessionProbeRefused
	// sessionProbeTimedOut: no answer reached the probe before its deadline,
	// which is what a dropped SYN looks like from the sender.
	sessionProbeTimedOut
)

// String names the outcome for an assertion message.
func (outcome sessionProbeOutcome) String() string {
	switch outcome {
	case sessionProbeUnspecified:
		return "no outcome"
	case sessionProbeConnected:
		return "a completed handshake"
	case sessionProbeRefused:
		return "a reset"
	case sessionProbeTimedOut:
		return "no answer"
	}
	return "outcome " + strconv.Itoa(int(outcome))
}

// The words sessionProbeScript prints, one per outcome.
const (
	sessionProbeWordConnected = "connected"
	sessionProbeWordRefused   = "refused"
	sessionProbeWordTimedOut  = "timeout"
)

// parseSessionProbe reads the one line sessionProbeScript prints. Any other
// line, including the script's own "error <reason>", is an error naming it,
// never an outcome: a probe that could not bind or set its TTL observed
// nothing about ze.
func parseSessionProbe(output string) (sessionProbeOutcome, error) {
	line := strings.TrimSpace(output)
	switch line {
	case sessionProbeWordConnected:
		return sessionProbeConnected, nil
	case sessionProbeWordRefused:
		return sessionProbeRefused, nil
	case sessionProbeWordTimedOut:
		return sessionProbeTimedOut, nil
	}
	return sessionProbeUnspecified, fmt.Errorf("the probe printed %q, not one of %s, %s or %s",
		line, sessionProbeWordConnected, sessionProbeWordRefused, sessionProbeWordTimedOut)
}

// requireSessionProbe refuses any outcome other than want, naming both.
func requireSessionProbe(output, probe string, want sessionProbeOutcome) error {
	got, err := parseSessionProbe(output)
	if err != nil {
		return fmt.Errorf("%s: %w", probe, err)
	}
	if got != want {
		return fmt.Errorf("%s got %s from ze, want %s", probe, got, want)
	}
	return nil
}

// coppUnknownSourceDrops reads the packet counter of the unknown-source-drop
// term from `nft list table inet ze_copp`. That term is the one rule of the
// input chain that both counts and drops (internal/plugins/copp/translate.go,
// translatePolicy), so the line is found by those two words inside the input
// chain's block. A table with no input chain, or an input chain with no counted
// drop, is an error: it is the table a CoPP that lost its final term renders.
func coppUnknownSourceDrops(table string) (uint64, error) {
	inInput := false
	sawInput := false
	for line := range strings.Lines(table) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "chain" {
			inInput = len(fields) > 1 && fields[1] == coppInputChain
			sawInput = sawInput || inInput
			continue
		}
		if !inInput {
			continue
		}
		if !slices.Contains(fields, "drop") {
			continue
		}
		for index := range len(fields) - 2 {
			if fields[index] == "counter" && fields[index+1] == "packets" {
				return strconv.ParseUint(fields[index+2], 10, 64)
			}
		}
	}
	if !sawInput {
		return 0, errors.New("ze_copp holds no input chain")
	}
	return 0, errors.New("the ze_copp input chain holds no counted drop, so no term drops an unknown source")
}

// requireCoppDropsRose refuses a reading of the unknown-source-drop counter
// that did not rise over the one taken before the probe.
func requireCoppDropsRose(before, after uint64) error {
	if after > before {
		return nil
	}
	return fmt.Errorf("unknown-source-drop counted %d packets before the probe and %d after, want more", before, after)
}
