// Design: docs/architecture/irr-owner.md -- the one-time conversion of the firewall's stored lists
// Related: datasets.go -- writeDataset and writeBinding, which the conversion reuses
//
// Before the owner, firewall-irr stored each list it enforced as JSON under
// meta/irr/{name}, keyed by AS number or AS-SET. Those lists are what the
// firewall enforced at the upgrade boot, so dropping them would change the
// firewall then; the owner converts each one once, under the configured
// server, and removes it only after its dataset, pointer and binding are
// stored. A list the owner cannot convert stays where it is, so nothing the
// firewall enforced is lost to a failed conversion.

package irr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// legacyPrefix lists the old per-name keys (zefs.KeyIRRPrefixCache).
const legacyPrefix = "meta/irr/"

// convertLegacy converts every meta/irr/{name} the store holds. A set that
// already has a pointer keeps it: the owner's copy is the newer one, and the
// old key is removed.
func (o *owner) convertLegacy(ctx context.Context) {
	keys, err := o.state.StateList(ctx, legacyPrefix)
	if err != nil {
		o.unreadable(legacyPrefix, err)
		return
	}
	for _, key := range keys {
		if ctx.Err() != nil {
			return
		}
		if err := o.convertLegacyKey(ctx, key); err != nil {
			o.unreadable(key, err)
		}
	}
}

// convertLegacyKey converts one old key and removes it once converted.
func (o *owner) convertLegacyKey(ctx context.Context, key string) error {
	name := strings.TrimPrefix(key, legacyPrefix)
	value, found, err := o.state.StateGet(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		return errNotFound(found)
	}
	var entry store.CachedEntry
	if err := json.Unmarshal(value, &entry); err != nil {
		return err
	}
	if entry.Name != name {
		return errors.New("the stored entry names " + entry.Name + ", not the key's " + name)
	}
	queried := entry.ASSet
	if queried == "" {
		queried = name
	}
	asSet, err := store.ASSetName(queried)
	if err != nil {
		return err
	}
	if err := o.convertLegacyEntry(ctx, asSet, entry); err != nil {
		return err
	}
	if asn, ok := parseLegacyASN(name); ok {
		o.writeBinding(ctx, asn, asSet)
	}
	return o.state.StateRemove(ctx, key)
}

// convertLegacyEntry stores entry as the dataset of asSet unless the set
// already has a pointer. checked-at is the entry's refreshed-at: the oldest
// time the server is known to have answered, so the converted set is due no
// later than the old one was.
func (o *owner) convertLegacyEntry(ctx context.Context, asSet string, entry store.CachedEntry) error {
	_, found, err := o.state.StateGet(ctx, store.PointerKey(asSet))
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	if len(entry.IPv4) == 0 && len(entry.IPv6) == 0 {
		return errors.New("the stored entry holds no prefix")
	}
	entry.Name, entry.ASSet = asSet, asSet
	return o.writeDataset(ctx, asSet, entry, entry.RefreshedAt)
}

// parseLegacyASN reads the AS number of an old key named AS<asn>. An AS-SET
// name (AS-FOO) is not one.
func parseLegacyASN(name string) (uint32, bool) {
	digits, ok := strings.CutPrefix(strings.ToUpper(name), "AS")
	if !ok {
		return 0, false
	}
	return parseASNSegment(digits)
}

// parseASNSegment reads the AS number a binding key carries, through asn.Parse,
// the one AS number reader (a key holds the decimal form).
func parseASNSegment(text string) (uint32, bool) {
	if text == "" || text[0] == '+' || text[0] == '-' {
		return 0, false
	}
	number, err := asn.Parse(text)
	if err != nil {
		return 0, false
	}
	return number, true
}
