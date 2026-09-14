// Design: docs/architecture/ike/ipsec-3-data-model.md -- the typed model between YANG and the engine
// Related: types.go -- the tables this test reads
// Related: identity.go, spd_policy.go -- the two vocabularies declared beside them
//
// The goal is one vocabulary. Each table in this package binds an operator word to a
// transform identifier, an identity type or a side of the IPsec boundary, and
// ze-ipsec-conf.yang decides which words an operator can type. Neither side can be
// derived from the other: the model cannot name the Go constant a word selects, and the
// module carries the description and the RFC citation beside each value. So the two are
// gated against each other here, and a value added to one side alone turns this test red
// (ai/rules/principles.md).

package ipsec

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-ipsec-conf with the loader. ze-extensions and
	// ze-types are embedded in the loader itself, and this module imports no other,
	// so the tree below resolves in a binary that links nothing else.
	_ "github.com/ze-software/ze/internal/component/ike/ipsec/yang"
)

const ipsecModule = "ze-ipsec-conf"

// TestVocabularyMatchesModel reads each enumeration out of the loaded model and
// compares it with the table this package keys on.
func TestVocabularyMatchesModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}

	cases := []struct {
		what  string
		path  []string
		table []string
	}{
		{
			what:  "the ESP encryption transforms",
			path:  []string{"vpn", "ipsec", "esp-group", "proposal", "encryption"},
			table: sorted(mapValues(encryptionNames)),
		},
		{
			what:  "the integrity and PRF algorithms",
			path:  []string{"vpn", "ipsec", "esp-group", "proposal", "hash"},
			table: sorted(mapValues(hashNames)),
		},
		{
			what:  "the peer authentication modes",
			path:  []string{"vpn", "ipsec", "site-to-site", "peer", "authentication", "mode"},
			table: sorted(mapValues(authModeNames)),
		},
		{
			what:  "the remote identity types",
			path:  []string{"vpn", "ipsec", "site-to-site", "peer", "authentication", "remote-id-type"},
			table: sorted(mapKeys(remoteIDTypeNames)),
		},
		{
			what:  "the IKE SA close actions",
			path:  []string{"vpn", "ipsec", "ike-group", "close-action"},
			table: sorted(mapValues(closeActionNames)),
		},
		{
			what:  "the dead-peer-detection actions",
			path:  []string{"vpn", "ipsec", "ike-group", "dead-peer-detection", "action"},
			table: sorted(mapValues(dpdActionNames)),
		},
		{
			what:  "the SPD sides",
			path:  []string{"vpn", "ipsec", "policy", "direction"},
			table: sorted(directionWords()),
		},
	}

	for _, tc := range cases {
		model := modelEnum(t, loader, tc.path)
		if slices.Equal(model, tc.table) {
			continue
		}
		t.Errorf("%s disagree: the model at %s holds %v and this package holds %v. "+
			"One side gained a value the other never learned, so an operator can name a word the daemon cannot read, "+
			"or the daemon carries a transform no operator can ask for",
			tc.what, slices.Concat([]string{ipsecModule}, tc.path), model, tc.table)
	}
}

// modelEnum answers the values of the enumeration at one leaf of the loaded model.
//
// It FAILS on a path that names no enumeration rather than answering an empty set.
// DefaultLoader discards its own LoadRegistered and Resolve errors, so a model that
// loaded half way comes back looking whole, and an empty set here would let every
// comparison above pass over nothing (ai/rules/evidence.md).
func modelEnum(t *testing.T, loader *configyang.Loader, path []string) []string {
	t.Helper()

	entry := loader.GetEntry(ipsecModule)
	if entry == nil {
		t.Fatalf("the loaded model holds no module %s: this binary registered nothing to compare against", ipsecModule)
	}
	walked := ipsecModule
	for _, name := range path {
		child := childEntry(entry, name)
		if child == nil {
			t.Fatalf("%s holds no child %q: the model this binary loaded is not the one this test reads", walked, name)
		}
		entry = child
		walked += "/" + name
	}
	if entry.Type == nil || entry.Type.Enum == nil {
		t.Fatalf("%s is not an enumeration, so it declares no vocabulary to compare", walked)
	}
	return sorted(entry.Type.Enum.Names())
}

func childEntry(entry *gyang.Entry, name string) *gyang.Entry {
	if entry.Dir == nil {
		return nil
	}
	return entry.Dir[name]
}

// directionWords is the SPD direction vocabulary, read from the one place this
// package spells it.
func directionWords() []string {
	words := make([]string, 0, len(spdDirections))
	for _, direction := range spdDirections {
		words = append(words, direction.String())
	}
	return words
}

func mapValues[K comparable](table map[K]string) []string {
	values := make([]string, 0, len(table))
	for _, value := range table {
		values = append(values, value)
	}
	return values
}

func mapKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	return keys
}

func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}
