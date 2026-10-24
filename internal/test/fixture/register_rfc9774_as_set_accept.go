// Design: docs/architecture/wire/attributes.md -- AS_PATH, RFC 9774 receive and session as-set accept
// Related: plugin_fixture_13.go -- runPlugin13 and replayIdleTimeout13, the harness this driver uses
// RFC: rfc/short/rfc9774.md

package fixture

import (
	"context"
	"encoding/hex"
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

// attrCodeASPath is the AS_PATH path attribute type code (RFC 4271 Section 5.1.2).
const attrCodeASPath = 2

// asSetAcceptMigrated waits for the received UPDATE that announces
// asSetAcceptPrefix, writes the received AS_PATH attribute value in hex to
// stderr behind the ZE-AS-SET-KEPT line the .ci expects, and asks the daemon to
// shut down. The event is subscribed in the `full` format because the parsed
// AS_PATH lists ASNs only: the raw attribute bytes carry the segment type, so
// the .ci can tell an AS_SET (type 1) kept as a set from one turned into an
// AS_SEQUENCE. On a peer without `session as-set accept` the UPDATE is treated
// as withdrawn (RFC 9774 Section 3), no event announces the prefix, and the
// driver fails on its idle timer with the ZE-OBSERVER-FAIL sentinel the .ci
// rejects.
func asSetAcceptMigrated(ctx context.Context, _ []string) error {
	events := make(chan string, 128)
	setup := func(plugin *sdk.Plugin) {
		plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "full")
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
				asPath, ok, err := announcedASPath(event, asSetAcceptPrefix)
				if err != nil {
					ReportFailure(err)
					return err
				}
				if !ok {
					continue
				}
				out := textbuf.New()
				out.Str("ZE-AS-SET-KEPT prefix=").Str(asSetAcceptPrefix).Str(" as-path=").HexUpper(asPath).Byte('\n')
				return out.StdErr()
			}
		}
	}, true)
}

// announcedASPath answers the received AS_PATH attribute value of a `full`
// UPDATE event when one of its NLRI entries announces prefix ("action":
// "add"), and false for any other event, a withdrawal of prefix included. An
// event that is not JSON, carries no raw attributes, or whose attributes hold
// no AS_PATH is an error: the observer cannot judge it, so it must not read as
// "not this route".
func announcedASPath(event, prefix string) ([]byte, bool, error) {
	var root struct {
		BGP struct {
			Message struct {
				Direction string `json:"direction"`
			} `json:"message"`
			Update struct {
				NLRI map[string][]asSetNLRIEntry `json:"nlri"`
			} `json:"update"`
			Raw struct {
				Attributes string `json:"attributes"`
			} `json:"raw"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(event), &root); err != nil {
		return nil, false, fmt.Errorf("as-set accept observer: event is not JSON: %w", err)
	}
	if root.BGP.Message.Direction != directionReceived {
		return nil, false, nil
	}
	if !announces(root.BGP.Update.NLRI, prefix) {
		return nil, false, nil
	}
	if root.BGP.Raw.Attributes == "" {
		return nil, false, fmt.Errorf("as-set accept observer: the UPDATE announcing %s carries no raw attributes", prefix)
	}
	attrs, err := hex.DecodeString(root.BGP.Raw.Attributes)
	if err != nil {
		return nil, false, fmt.Errorf("as-set accept observer: raw attributes are not hex: %w", err)
	}
	asPath, err := pathAttributeValue(attrs, attrCodeASPath)
	if err != nil {
		return nil, false, fmt.Errorf("as-set accept observer: %w", err)
	}
	return asPath, true, nil
}

// asSetNLRIEntry is one family's NLRI operation in a parsed UPDATE event.
type asSetNLRIEntry struct {
	Action string   `json:"action"`
	NLRI   []string `json:"nlri"`
}

// announces answers whether one NLRI entry of a parsed UPDATE adds prefix.
func announces(nlri map[string][]asSetNLRIEntry, prefix string) bool {
	for _, entries := range nlri {
		for _, entry := range entries {
			if entry.Action != argAdd {
				continue
			}
			if slices.Contains(entry.NLRI, prefix) {
				return true
			}
		}
	}
	return false
}

// pathAttributeValue walks packed path attributes (RFC 4271 Section 4.3: flags,
// type code, then a one-octet length, or two octets when the Extended Length
// bit 0x10 is set) and answers the value of the attribute with code. A
// truncated attribute or an absent code is an error. The loop is bounded by
// len(attrs): each step consumes at least the three-octet header.
func pathAttributeValue(attrs []byte, code byte) ([]byte, error) {
	for off := 0; off < len(attrs); {
		if len(attrs)-off < 3 {
			return nil, fmt.Errorf("path attribute header truncated at offset %d", off)
		}
		flags, attrCode := attrs[off], attrs[off+1]
		headerOctets, valueOctets := 3, int(attrs[off+2])
		if flags&0x10 != 0 {
			if len(attrs)-off < 4 {
				return nil, fmt.Errorf("extended-length path attribute header truncated at offset %d", off)
			}
			headerOctets, valueOctets = 4, int(attrs[off+2])<<8|int(attrs[off+3])
		}
		start := off + headerOctets
		end := start + valueOctets
		if end > len(attrs) {
			return nil, fmt.Errorf("path attribute %d value truncated at offset %d", attrCode, off)
		}
		if attrCode == code {
			return attrs[start:end], nil
		}
		off = end
	}
	return nil, fmt.Errorf("no path attribute %d in the received UPDATE", code)
}
