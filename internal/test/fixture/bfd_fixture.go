// Design: docs/architecture/bfd.md -- the BFD timers an operator reads back
// RFC: rfc/short/rfc5880.md
// Related: routing_fixture.go -- the same observer shape for the IGP suites
//
// bfd_fixture.go holds the scenarios of the test/bfd suite. A BFD timer is
// computed inside the session machine and reaches no wire until a peer answers,
// so the operator surface is where a functional test can read it: the observer
// asks the running daemon for one session and checks the two durations RFC 5880
// fixes for a session that has heard nothing yet.

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The session test/bfd/bfd-detection-interval.ci configures, and the durations
// RFC 5880 fixes for it before any Control packet arrives.
//
// detect-multiplier 5 and required-min-rx-us 120000 are deliberately not the
// YANG defaults (3 and 300000). A test that configures the default cannot tell
// the value it asked for from the fallback the code applies when it reads
// nothing.
const (
	bfdDetectPeer = addrTestNet3Nine
	// RFC 5880 Section 6.8.4: the detection time is the Detect Multiplier times
	// the larger of bfd.RequiredMinRxInterval and the peer's Desired Min TX
	// Interval. Nothing answers on this address, so the peer's value stays zero
	// and the local 120000us wins: 5 * 120000us.
	bfdDetectExpected = 600 * time.Millisecond
	// RFC 5880 Section 6.8.3: bfd.DesiredMinTxInterval is held at one second
	// until the session is Up, so the transmit interval is one second whatever
	// the profile asked for.
	bfdSlowStartTxExpected = time.Second
	// The multiplier the profile configured, read back unchanged.
	bfdDetectMultExpected = 5
)

// bfdDetectionIntervalScenario asks the daemon for the configured single-hop
// session and checks the timers it publishes.
//
// The session never leaves Down, because nothing answers on 203.0.113.9. That
// is the point: every number below follows from local configuration alone, so
// the test holds an oracle the code under test does not supply.
func bfdDetectionIntervalScenario(ctx context.Context, plugin *sdk.Plugin) error {
	session, err := bfdSessionDetail(ctx, plugin, bfdDetectPeer)
	if err != nil {
		return err
	}
	if err := bfdDuration(session, "detection-interval", bfdDetectExpected); err != nil {
		return err
	}
	if err := bfdDuration(session, "tx-interval", bfdSlowStartTxExpected); err != nil {
		return err
	}
	got, held := session["detect-multiplier"].(float64)
	if !held {
		return fmt.Errorf("show bfd session address %s: detect-multiplier is %#v, not a number",
			bfdDetectPeer, session["detect-multiplier"])
	}
	if int(got) != bfdDetectMultExpected {
		return fmt.Errorf("show bfd session address %s: detect-multiplier = %d, want %d",
			bfdDetectPeer, int(got), bfdDetectMultExpected)
	}
	fmt.Fprintln(os.Stderr, "OK: bfd session publishes detection-interval 600ms and tx-interval 1s")
	return nil
}

// bfdSessionDetail polls `show bfd session address <peer>` until the daemon
// answers done, and returns the session object.
//
// A poll rather than one call: the BFD plugin publishes its api.Service while
// the daemon is still starting, so the first dispatch can arrive before the
// configured session is pinned.
func bfdSessionDetail(ctx context.Context, plugin *sdk.Plugin, peer string) (map[string]any, error) {
	command := "show bfd session address " + peer
	var raw json.RawMessage
	var status string
	var dispatchErr error
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		status, dispatchErr = Dispatch(ctx, plugin, command, &raw)
		return dispatchErr == nil && status == statusDone
	}) {
		if dispatchErr != nil {
			return nil, fmt.Errorf("%s: %w", command, dispatchErr)
		}
		return nil, fmt.Errorf("%s: status=%s data=%s", command, status, raw)
	}

	var session map[string]any
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("%s: answer %s is not a session object: %w", command, raw, err)
	}
	if session["peer"] != peer {
		return nil, fmt.Errorf("%s: answered for peer %#v", command, session["peer"])
	}
	return session, nil
}

// bfdDuration checks one duration field of a session object. The field is a Go
// time.Duration, so JSON carries it as a number of nanoseconds.
func bfdDuration(session map[string]any, field string, want time.Duration) error {
	raw, held := session[field].(float64)
	if !held {
		return fmt.Errorf("show bfd session address %s: %s is %#v, not a number",
			bfdDetectPeer, field, session[field])
	}
	got := time.Duration(int64(raw))
	if got != want {
		return fmt.Errorf("show bfd session address %s: %s = %s, want %s",
			bfdDetectPeer, field, got, want)
	}
	return nil
}
