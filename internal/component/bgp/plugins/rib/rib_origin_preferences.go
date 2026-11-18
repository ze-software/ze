// Design: docs/architecture/rib/unified-locrib.md -- configured source distances.
// Related: config_tx.go -- one existing config apply/rollback transaction.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"

	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// originDefaultDistances derives bootstrap defaults from the declaration seam.
// The table is an owned immutable config value, including legitimate zero.
func originDefaultDistances() map[string]uint8 { return ribdistance.Bootstrap() }

func parseOriginPreferences(sections []sdk.ConfigSection) (map[string]uint8, bool, error) {
	table := originDefaultDistances()
	for _, section := range sections {
		if section.Root != "rib" {
			continue
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal([]byte(section.Data), &data); err != nil {
			return nil, false, fmt.Errorf("read origin distance declaration: %w", err)
		}
		subtree := data["rib"]
		if len(subtree) == 0 {
			subtree = []byte(section.Data)
		}
		var root struct {
			Distance map[string]json.RawMessage `json:"distance"`
		}
		if err := json.Unmarshal(subtree, &root); err != nil {
			return nil, false, fmt.Errorf("read origin distance leaves: %w", err)
		}
		for name, raw := range root.Distance {
			if _, known := table[name]; !known {
				return nil, false, fmt.Errorf("origin distance protocol %q is not declared; use a rib/distance leaf", name)
			}
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, false, err
			}
			var number uint64
			var err error
			switch value := value.(type) {
			case string:
				number, err = strconv.ParseUint(value, 10, 8)
			case float64:
				number, err = strconv.ParseUint(strconv.FormatFloat(value, 'f', -1, 64), 10, 8)
			default:
				return nil, false, fmt.Errorf("origin distance %s is not a numeric leaf; use a distance from zero through 255", name)
			}
			if err != nil {
				return nil, false, fmt.Errorf("origin distance %s: %w", name, err)
			}
			table[name] = uint8(number)
		}
		return table, true, nil
	}
	return table, false, nil
}

// installOriginPreferences publishes one table/revision generation and joins
// affected current origin scopes, including an equal winner under a fresh table. On failure it restores the prior
// declaration with fresh revisions and joins that restoration before returning.
func (d *selectedDispatch) installOriginPreferences(ctx context.Context, table map[string]uint8) error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	previous := d.distances
	if maps.Equal(previous, table) {
		d.mu.Unlock()
		return nil
	}
	changed, err := d.publishOriginPreferencesLocked(maps.Clone(table))
	d.signalLocked()
	d.mu.Unlock()
	if err == nil {
		err = d.joinOriginPreferences(ctx, changed)
	}
	if err == nil {
		return nil
	}
	d.mu.Lock()
	restored, restoreError := d.publishOriginPreferencesLocked(previous)
	d.signalLocked()
	d.mu.Unlock()
	if restoreError == nil {
		restoreError = d.joinOriginPreferences(ctx, restored)
	}
	if restoreError != nil {
		return sdk.RunningConfigLost(errors.Join(err, restoreError), "BGP origin preferences could not be restored", "Restore the selecting process connection")
	}
	return err
}

type originPreferenceChange struct {
	scope originScope
	key   advertiseKey
}

func (d *selectedDispatch) publishOriginPreferencesLocked(table map[string]uint8) ([]originPreferenceChange, error) {
	d.distances = table
	var changed []originPreferenceChange
	for key := range d.origins {
		for scope := range d.originScopesLocked(key) {
			before := d.cut
			if err := d.reelectOriginLocked(scope, key, true); err != nil {
				return changed, err
			}
			if before != d.cut {
				changed = append(changed, originPreferenceChange{scope: scope, key: key})
			}
		}
	}
	return changed, nil
}

func (d *selectedDispatch) joinOriginPreferences(ctx context.Context, changed []originPreferenceChange) error {
	for _, change := range changed {
		if err := d.originBarrier(ctx, change.scope, change.key); err != nil {
			return err
		}
	}
	return nil
}

func (r *RIBManager) installOriginPreferenceConfig(table map[string]uint8) error {
	if table == nil {
		table = originDefaultDistances()
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout)
	defer cancel()
	return r.selectedDispatch.installOriginPreferences(ctx, table)
}
