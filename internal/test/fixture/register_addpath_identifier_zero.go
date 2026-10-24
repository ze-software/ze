// Design: docs/architecture/wire/nlri.md -- ADD-PATH presence travels with the NLRI, never in its value
// Related: plugin_fixture_02.go -- the observer, command and poll helpers this driver uses
// RFC: rfc/short/rfc7911.md

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/addpath-identifier-zero", observer02("addpath-identifier-zero-test", addPathIdentifierZero02))
}

// addPathZeroKey is the Adj-RIB-In key of 10.0.0.0/24 learned under Path
// Identifier 0, and addPathZeroBareKey the key of the same prefix carrying none.
const (
	addPathZeroKey     = "ipv4/unicast:10.0.0.0/24:0"
	addPathZeroBareKey = "ipv4/unicast:10.0.0.0/24"
)

// addPathIdentifierZero02 proves that a route a peer sent under ADD-PATH with
// Path Identifier 0 is held as a route that carries identifier 0.
//
// Method: the source announces 10.0.0.0/24 with identifier 0 on a session that
// negotiated ADD-PATH. The driver polls `show bgp adj-rib-in` until the
// source's route is there, then requires its key to name identifier 0. A
// presence test on the identifier's value reads 0 as "no identifier" and keys
// the route as the bare prefix, which is a different route from the one the
// peer sent (RFC 7911 Section 3 reserves no value). The receiver's wire
// assertion in the .ci proves the four identifier octets reach the next peer.
func addPathIdentifierZero02(ctx context.Context, plugin *sdk.Plugin) error {
	// Both sessions are up once ze sent each its End-of-RIB, so the receiver is
	// a forwarding target before the route is looked for.
	if !eorSent02(ctx, plugin, "*", 2, 60) {
		return errors.New("ze never sent its End-of-RIB to both peers")
	}
	var keys []string
	var answer string
	if !Poll(ctx, 60, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp adj-rib-in")
		if err != nil {
			answer = err.Error()
			return false
		}
		answer = text02(raw)
		keys, err = adjRIBInKeys02(raw)
		if err != nil {
			answer = err.Error()
			return false
		}
		return len(keys) != 0
	}) {
		return fmt.Errorf("adj-rib-in never held the source's route for 10.0.0.0/24: %s", answer)
	}
	for _, key := range keys {
		if key == addPathZeroBareKey {
			return fmt.Errorf("adj-rib-in keys the route as %q, which carries no identifier; want %q", key, addPathZeroKey)
		}
	}
	for _, key := range keys {
		if key != addPathZeroKey {
			continue
		}
		// The forward pool drains before the daemon is asked to stop, so the
		// receiver's wire assertion is not raced by the shutdown.
		if err := quiesce02(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK adj-rib-in holds", key)
		return nil
	}
	return fmt.Errorf("adj-rib-in keys %v, want %q", keys, addPathZeroKey)
}

// adjRIBInKeys02 returns every route key of 10.0.0.0/24 in a
// `show bgp adj-rib-in` answer, whatever peer holds it. The answer is
// {"adj-rib-in": {<peer>: [<route>...]}}. An answer that does not decode is an
// error, never "no keys yet".
func adjRIBInKeys02(raw []byte) ([]string, error) {
	data, err := map02(raw)
	if err != nil {
		return nil, fmt.Errorf("adj-rib-in answer does not decode: %w", err)
	}
	peers, _ := data["adj-rib-in"].(map[string]any)
	var keys []string
	for _, value := range peers {
		routes, _ := value.([]any)
		for _, route := range routes {
			row, _ := route.(map[string]any)
			key, _ := row["key"].(string)
			if key == addPathZeroKey || key == addPathZeroBareKey {
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}
