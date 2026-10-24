// Design: docs/architecture/wire/attributes.md -- AS_PATH, RFC 9774 receive and session as-set accept
// Related: plugin_fixture_13.go -- runPlugin13 and replayIdleTimeout13, the harness this driver uses
// RFC: rfc/short/rfc9774.md

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rfc9774-as-set-accept-migrated", asSetAcceptMigrated)
}

// asSetAcceptPrefix is the route test/plugin/rfc9774-as-set-accept-migrated.ci
// announces with an AS_SET in its AS_PATH.
const asSetAcceptPrefix = "10.1.0.0/24"

// asSetAcceptMigrated waits for the received UPDATE that announces
// asSetAcceptPrefix, writes its AS_PATH to stderr behind the ZE-AS-SET-KEPT
// line the .ci expects, and asks the daemon to shut down. On a peer without
// `session as-set accept` the UPDATE is treated as withdrawn (RFC 9774
// Section 3), no event announces the prefix, and the driver fails on its idle
// timer with the ZE-OBSERVER-FAIL sentinel the .ci rejects.
func asSetAcceptMigrated(ctx context.Context, _ []string) error {
	events := make(chan string, 128)
	setup := func(plugin *sdk.Plugin) {
		plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		plugin.OnEvent(func(event string) error {
			select {
			case events <- event:
			default:
				return errors.New("as-set accept observer event queue full")
			}
			return nil
		})
	}
	return runPlugin13(ctx, "rfc9774-as-set-accept-migrated", setup, func(ctx context.Context, _ *sdk.Plugin) error {
		idle := time.NewTimer(replayIdleTimeout13())
		defer idle.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-idle.C:
				err := fmt.Errorf("no received UPDATE announced %s: the AS_SET route was not kept", asSetAcceptPrefix)
				ReportFailure(err)
				return err
			case event := <-events:
				asPath, ok := announcedASPath(event, asSetAcceptPrefix)
				if !ok {
					continue
				}
				out := textbuf.New()
				out.Str("ZE-AS-SET-KEPT prefix=").Str(asSetAcceptPrefix).Str(" as-path=").Str(asPath).Byte('\n')
				return out.StdErr()
			}
		}
	}, true)
}

// announcedASPath answers the AS_PATH of a received parsed UPDATE event when
// one of its NLRI entries announces prefix ("action": "add"), and false for
// any other event, a withdrawal of prefix included.
func announcedASPath(event, prefix string) (string, bool) {
	var root struct {
		BGP struct {
			Message struct {
				Direction string `json:"direction"`
			} `json:"message"`
			Update struct {
				Attr map[string]json.RawMessage `json:"attr"`
				NLRI map[string][]struct {
					Action string   `json:"action"`
					NLRI   []string `json:"nlri"`
				} `json:"nlri"`
			} `json:"update"`
		} `json:"bgp"`
	}
	if json.Unmarshal([]byte(event), &root) != nil || root.BGP.Message.Direction != directionReceived {
		return "", false
	}
	for _, entries := range root.BGP.Update.NLRI {
		for _, entry := range entries {
			if entry.Action == "add" && slices.Contains(entry.NLRI, prefix) {
				return string(root.BGP.Update.Attr["as-path"]), true
			}
		}
	}
	return "", false
}
