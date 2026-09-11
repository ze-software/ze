// VALIDATES: every field of api.Key has a decided place in the first-packet
// index -- indexed and always set, indexed and relaxable, or deliberately not
// indexed -- and the code agrees with that decision.
// PREVENTS: the failure a missing relaxation cannot cause. A field added to
// api.Key and NOT added to firstPacketKey makes two DISTINCT sessions produce
// the same index, and engine.go's `l.byKey[firstPacketIndex(key)] = entry`
// overwrites, so one session silently replaces the other in the index and stops
// receiving first packets. No behavioral test reds on that, because both
// sessions still exist and both still answer by discriminator.

package engine

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// keyPlacement is the decision one api.Key field carries into the index.
type keyPlacement struct {
	// indexField is the firstPacketKey field this one becomes, empty when the
	// field is deliberately not indexed.
	indexField string
	// relaxable says a session may leave this field unset, so a packet
	// carrying any value in it still matches (engine.go, keyRelaxation).
	relaxable bool
	// why records the reason, so the next person adding a field reads a
	// decision rather than a table.
	why string
}

// keyPlacements is that decision for every field of api.Key. A field added to
// api.Key and not named here fails TestFirstPacketKeyMirrorsEveryKeyField,
// which is the point: the index is a second declaration of a session's
// identity, and the two cannot be allowed to drift in silence.
var keyPlacements = map[string]keyPlacement{
	"Peer":      {indexField: "peer", why: "the remote address; a session without one cannot exist"},
	"Local":     {indexField: "local", relaxable: true, why: "Canonical leaves it unset where it cannot derive a source"},
	"VRF":       {indexField: "vrf", why: "canonicalized to DefaultVRF, so it is never unset, and a loop serves one VRF"},
	"Interface": {indexField: "iface", relaxable: true, why: "Canonical leaves it unset for a link it cannot name, and multi-hop clears it"},
	"Mode":      {indexField: "mode", why: "single-hop and multi-hop are separate loops and separate sockets"},
}

func TestFirstPacketKeyMirrorsEveryKeyField(t *testing.T) {
	keyType := reflect.TypeFor[api.Key]()
	for field := range keyType.Fields() {
		name := field.Name
		if _, decided := keyPlacements[name]; !decided {
			t.Errorf("api.Key.%s has no entry in keyPlacements.\n"+
				"Decide what it means for the first-packet index (RFC 5880 Section 6.8.6) and say so there: "+
				"a field that is part of a session's identity but absent from firstPacketKey makes two distinct "+
				"sessions share one index slot, and the later one silently replaces the earlier.", name)
		}
	}

	indexType := reflect.TypeFor[firstPacketKey]()
	for name, placement := range keyPlacements {
		if _, ok := keyType.FieldByName(name); !ok {
			t.Errorf("keyPlacements names %q, which api.Key no longer has", name)
			continue
		}
		if placement.indexField == "" {
			continue
		}
		if _, ok := indexType.FieldByName(placement.indexField); !ok {
			t.Errorf("api.Key.%s is placed at firstPacketKey.%s, which does not exist", name, placement.indexField)
		}
	}
	// A SET comparison, not a count: two api.Key fields mapped to one index
	// field plus one index field nothing maps to balances the arithmetic and
	// leaves a field matched against nothing.
	mapped := make(map[string]string, indexType.NumField())
	for name, placement := range indexedPlacements() {
		if previous, taken := mapped[placement.indexField]; taken {
			t.Errorf("api.Key.%s and api.Key.%s both map to firstPacketKey.%s; two identities would share one index slot", previous, name, placement.indexField)
			continue
		}
		mapped[placement.indexField] = name
	}
	for field := range indexType.Fields() {
		if _, ok := mapped[field.Name]; !ok {
			t.Errorf("firstPacketKey.%s is matched against nothing: no api.Key field maps to it", field.Name)
		}
	}
}

// TestRelaxationsCoverEveryRelaxableField derives the relaxation set from the
// CODE rather than from a second list: it zeroes a fully-populated key with
// every optional flag and reads back which fields moved.
func TestRelaxationsCoverEveryRelaxableField(t *testing.T) {
	full := firstPacketKey{
		peer:  netip.MustParseAddr("203.0.113.9"),
		local: netip.MustParseAddr("203.0.113.1"),
		vrf:   "red",
		iface: "eth0",
		mode:  api.MultiHop,
	}
	relaxed := full.without(optionalKeyFields)

	fullValue, relaxedValue := reflect.ValueOf(full), reflect.ValueOf(relaxed)
	indexType := reflect.TypeFor[firstPacketKey]()
	moved := make(map[string]bool, indexType.NumField())
	for i := range indexType.NumField() {
		moved[indexType.Field(i).Name] = !fullValue.Field(i).Equal(relaxedValue.Field(i))
	}

	for name, placement := range keyPlacements {
		if placement.indexField == "" {
			continue
		}
		if moved[placement.indexField] != placement.relaxable {
			t.Errorf("api.Key.%s is %s in keyPlacements but the relaxation walk %s.\n"+
				"A relaxable field MUST have a bit in optionalKeyFields and be cleared by without(); "+
				"a field that is never unset MUST NOT be cleared, or two sessions that differ only in it "+
				"become interchangeable for a first packet.",
				name, relaxableWord(placement.relaxable), walkWord(moved[placement.indexField]))
		}
	}
}

// indexedPlacements is the subset of keyPlacements that reaches the index.
func indexedPlacements() map[string]keyPlacement {
	out := make(map[string]keyPlacement, len(keyPlacements))
	for name, placement := range keyPlacements {
		if placement.indexField != "" {
			out[name] = placement
		}
	}
	return out
}

func relaxableWord(relaxable bool) string {
	if relaxable {
		return "relaxable"
	}
	return "always matched"
}

// walkWord says what the walk DID, so the message reads as a sentence.
func walkWord(cleared bool) string {
	if cleared {
		return "clears it"
	}
	return "keeps it"
}

// distinctKeys returns two api.Key values whose every field differs from the
// other's AND from that field's zero value. Both properties are asserted rather
// than assumed: api.HopMode's SingleHop IS the zero value, so a pair chosen
// carelessly would let a builder that drops a field still look populated.
func distinctKeys(t *testing.T) (api.Key, api.Key) {
	t.Helper()
	first := api.Key{
		Peer:      netip.MustParseAddr("203.0.113.9"),
		Local:     netip.MustParseAddr("203.0.113.1"),
		VRF:       "red",
		Interface: "eth0",
		Mode:      api.MultiHop,
	}
	second := api.Key{
		Peer:      netip.MustParseAddr("203.0.113.8"),
		Local:     netip.MustParseAddr("203.0.113.2"),
		VRF:       "blue",
		Interface: "eth1",
		Mode:      api.SingleHop,
	}
	zero := reflect.ValueOf(api.Key{})
	firstValue, secondValue := reflect.ValueOf(first), reflect.ValueOf(second)
	keyType := reflect.TypeFor[api.Key]()
	for i := range keyType.NumField() {
		name := keyType.Field(i).Name
		if firstValue.Field(i).Equal(secondValue.Field(i)) {
			t.Fatalf("distinctKeys: api.Key.%s is the same in both keys, so a builder that drops it cannot be caught", name)
		}
		if firstValue.Field(i).Equal(zero.Field(i)) {
			t.Fatalf("distinctKeys: api.Key.%s is its zero value in the first key, so a builder that drops it looks populated", name)
		}
	}
	return first, second
}

// TestFirstPacketIndexCarriesEveryFieldValue is the half the name mirror cannot
// reach, and it is this spec's own defect one layer down.
//
// TestFirstPacketKeyMirrorsEveryKeyField forces a new api.Key field into
// firstPacketKey and into keyPlacements. It cannot force firstPacketIndex to
// COPY it: a field declared on both structs and simply left out of the builder
// makes every session index with that field's zero value, and
// `l.byKey[firstPacketIndex(key)] = entry` then collapses two distinct sessions
// onto one slot, silently replacing the first. That is the same shape as the
// five repairs this package carries -- a key field that exists but does not
// reach the lookup -- and no behavioral test covers a field that does not exist
// yet.
func TestFirstPacketIndexCarriesEveryFieldValue(t *testing.T) {
	first, second := distinctKeys(t)
	indexed := firstPacketIndex(first)
	indexValue := reflect.ValueOf(indexed)
	zeroIndex := reflect.ValueOf(firstPacketKey{})
	indexType := reflect.TypeFor[firstPacketKey]()

	for name, placement := range indexedPlacements() {
		field, ok := indexType.FieldByName(placement.indexField)
		if !ok {
			continue // the mirror test reports this
		}
		if indexValue.Field(field.Index[0]).Equal(zeroIndex.Field(field.Index[0])) {
			t.Errorf("firstPacketIndex left firstPacketKey.%s at its zero value while api.Key.%s was set.\n"+
				"Every session then indexes alike in that field, and the map write in EnsureSession replaces "+
				"one session's index entry with another's.", placement.indexField, name)
		}
	}

	// The same statement from the other side: changing ONE api.Key field has to
	// change the index. A builder that drops the field makes the two equal.
	keyType := reflect.TypeFor[api.Key]()
	secondValue := reflect.ValueOf(second)
	for i := range keyType.NumField() {
		name := keyType.Field(i).Name
		placement, decided := keyPlacements[name]
		if !decided || placement.indexField == "" {
			continue
		}
		varied := reflect.New(keyType).Elem()
		varied.Set(reflect.ValueOf(first))
		varied.Field(i).Set(secondValue.Field(i))
		key, ok := reflect.TypeAssert[api.Key](varied)
		if !ok {
			t.Fatalf("varying %s produced a %s, not an api.Key", name, varied.Type())
		}
		if firstPacketIndex(key) == indexed {
			t.Errorf("two api.Key values differing only in %s produce the SAME firstPacketKey, so the two sessions share one index slot", name)
		}
	}
}
