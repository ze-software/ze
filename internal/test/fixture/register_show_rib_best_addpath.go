// Design: docs/architecture/plugin/rib-storage-design.md -- one best path per prefix under ADD-PATH
// Related: plugin_fixture_02.go -- the observer, command and poll helpers this driver uses
// RFC: rfc/short/rfc7911.md
// RFC: rfc/short/rfc4271.md

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/show-rib-best-addpath", observer02("show-rib-best-addpath-test", showRIBBestAddPath02))
}

// addPathTestPrefix is the prefix both ADD-PATH paths of the test announce.
const addPathTestPrefix = "10.0.0.0/24"

// showRIBBestAddPath02 proves that two paths one ADD-PATH session announces for
// one prefix are elected together, so `show bgp rib best` answers ONE row for
// the prefix, carrying the better path.
//
// Method: the peer announces 10.0.0.0/24 under path id 1 with MED 50 and under
// path id 2 with MED 10. The driver waits until the Adj-RIB-In holds both paths,
// then reads the best-path answer. A RIB that elects per path id answers two
// rows, one per path, and each is its own winner; a RIB that drops a path
// answers the MED 50 row or nothing. Only the per-prefix election answers one
// row, path id 2, MED 10 (RFC 4271 Section 9.1.2.2 step c, RFC 7911 Section 2).
func showRIBBestAddPath02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return errors.New("ze never sent its End-of-RIB to peer1")
	}
	var received int
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rib received")
		if err != nil {
			return false
		}
		rows, err := addPathPrefixRows02(raw, "routes")
		if err != nil {
			return false
		}
		received = len(rows)
		return received == 2
	}) {
		return fmt.Errorf("adj-rib-in holds %d paths for %s, want 2", received, addPathTestPrefix)
	}
	raw, err := requireDone02(ctx, plugin, "show bgp rib best")
	if err != nil {
		return err
	}
	rows, err := addPathPrefixRows02(raw, "best-path")
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("best-path answers %d rows for %s, want exactly 1: %s", len(rows), addPathTestPrefix, text02(raw))
	}
	best := rows[0]
	prefix, _ := best["prefix"].(string)
	if !strings.Contains(prefix, "[pathID=2]") {
		return fmt.Errorf("best path is %q, want path id 2: %s", prefix, text02(raw))
	}
	attributes, _ := best["attributes"].(map[string]any)
	med, _ := attributes["med"].(map[string]any)
	if int02(med["value"]) != 10 {
		return fmt.Errorf("best path MED is %v, want 10: %s", med["value"], text02(raw))
	}
	fmt.Fprintln(os.Stderr, "OK best: one row for", addPathTestPrefix, "path", prefix, "med", int02(med["value"]))
	return nil
}

// addPathPrefixRows02 returns the rows of the answer list under key whose
// prefix is addPathTestPrefix, with or without a path id suffix.
func addPathPrefixRows02(raw json.RawMessage, key string) ([]map[string]any, error) {
	data, err := map02(raw)
	if err != nil {
		return nil, err
	}
	list, ok := data[key].([]any)
	if !ok {
		return nil, fmt.Errorf("answer has no %q list: %s", key, text02(raw))
	}
	var rows []map[string]any
	for _, value := range list {
		row, ok := value.(map[string]any)
		if !ok {
			continue
		}
		prefix, _ := row["prefix"].(string)
		if strings.HasPrefix(prefix, addPathTestPrefix) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
