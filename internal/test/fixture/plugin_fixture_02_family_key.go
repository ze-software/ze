// Design: docs/architecture/api/json-format.md -- one key per family in the update event
// Related: plugin_fixture_02_events.go -- the event observer this scenario runs under

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// familyKeyFamilies02 names the families test/plugin/family-key-merge-event.ci
// sends an UPDATE for, each carrying an announce and a withdrawal of that one
// family.
var familyKeyFamilies02 = []string{"ipv6/unicast", "ipv4/unicast"}

// familyKeyMergeEvent02 waits for one received UPDATE event per family in
// familyKeyFamilies02 and requires the family's array to hold both an add and a
// del. The event is decoded into a map, as every map-based consumer reads it,
// so a family stated as two keys keeps only its last copy and the check fails.
func familyKeyMergeEvent02(ctx context.Context, plugin *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	pending := make(map[string]bool, len(familyKeyFamilies02))
	for _, fam := range familyKeyFamilies02 {
		pending[fam] = true
	}
	for len(pending) > 0 {
		event, ok := nextEvent02(ctx, events, shutdown, 20*time.Second)
		if !ok {
			return fmt.Errorf("FAMILY-KEY-EVENT: no received UPDATE event arrived for %d of %d families", len(pending), len(familyKeyFamilies02))
		}
		_, kind, direction := eventFacts02(event)
		if kind != eventUpdate {
			continue
		}
		if direction != directionReceived {
			continue
		}
		for _, fam := range familyKeyFamilies02 {
			actions, present := familyKeyActions02(eventBody02(event), fam)
			if !present {
				continue
			}
			if !strings.Contains(actions, " add") {
				return fmt.Errorf("FAMILY-KEY-EVENT: the %s key holds no add operation (it holds:%s), so the announce in the same UPDATE was lost", fam, actions)
			}
			if !strings.Contains(actions, " del") {
				return fmt.Errorf("FAMILY-KEY-EVENT: the %s key holds no del operation (it holds:%s), so the withdrawal in the same UPDATE was lost", fam, actions)
			}
			delete(pending, fam)
		}
	}
	if err := quiesce02(ctx, plugin); err != nil {
		return err
	}
	requestShutdownAsync02(ctx, plugin)
	return waitForShutdown02(ctx, shutdown, 15*time.Second)
}

// familyKeyActions02 prints one FAMILY-KEY-EVENT line per operation under fam
// in the update in body, and returns the actions in order, each after a space.
// present is false when the update states no fam key.
func familyKeyActions02(body map[string]any, fam string) (actions string, present bool) {
	update, _ := body["update"].(map[string]any)
	nlri, _ := update["nlri"].(map[string]any)
	entries, present := nlri[fam].([]any)
	if !present {
		return "", false
	}
	var seen strings.Builder
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		action, _ := entry["action"].(string)
		seen.WriteByte(' ')
		seen.WriteString(action)
		var line strings.Builder
		line.WriteString("FAMILY-KEY-EVENT: ")
		line.WriteString(fam)
		line.WriteByte(' ')
		line.WriteString(action)
		elements, _ := entry["nlri"].([]any)
		for _, element := range elements {
			prefix, _ := element.(string)
			line.WriteByte(' ')
			line.WriteString(prefix)
		}
		fmt.Fprintln(os.Stderr, line.String())
	}
	return seen.String(), true
}
