// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// Related: xfrm_linux.go -- the three kernel-name tables this test reads
// Related: vpp_vocabulary_test.go -- the same proof for the VPP backend
//
// The xfrm tables bind each algorithm word to a kernel transform name and an ICV
// length, and two YANG modules decide which words can reach them: ze-ipsec-conf.yang
// through a negotiated Child SA (ipsec.Encryption.String, ipsec.Hash.String) and
// ze-ospf-conf.yang through an RFC 4552 manual SA. Neither side derives from the
// other, so the tables are gated against both modules here. A word the model admits
// and no table holds is refused at install time, so the tunnel carries nothing; a
// word only a table holds is one no operator can reach (ai/rules/principles.md).

//go:build linux

package dataplane

import (
	"errors"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank imports register the two modules with the loader. Each imports
	// only the modules the loader embeds, so the leaves below resolve in a binary
	// that links nothing else.
	_ "github.com/ze-software/ze/internal/component/ike/ipsec/yang"
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// The leaves whose words reach the tables. The OSPF family grouping declares the
// ipsec container once for every address family, so one family stands for all.
const (
	espEncryptionLeaf = "vpn/ipsec/esp-group/proposal/encryption"
	espHashLeaf       = "vpn/ipsec/esp-group/proposal/hash"
	ospfAuthLeaf      = "ospf/address-family/ipv6/interfaces/interface/ipsec/algorithm"
	ospfEncLeaf       = "ospf/address-family/ipv6/interfaces/interface/ipsec/encryption-algorithm"
)

// espParseRefused are the ESP encryption words the model offers and config parse
// refuses for an ESP proposal (ipsec.EncryptionImplementedESP, ike/ipsec/
// algorithm_support.go): RFC 5282 carries AES-CCM into the IKE SA only, and lifting
// that is RFC 4309 work in the dataplane. The tables MUST NOT hold them, because a
// word that never reaches the backend has no kernel transform proven for it, and
// the backend MUST refuse them, so a parse guard lifted without this table learning
// the cipher fails the install rather than encrypting with AES-GCM.
var espParseRefused = map[string]bool{
	"aes128ccm8":  true,
	"aes256ccm8":  true,
	"aes128ccm12": true,
	"aes256ccm12": true,
	"aes128ccm16": true,
	"aes256ccm16": true,
}

// TestXfrmCipherVocabularyMatchesModel checks both directions of the cipher binding:
// every word the two modules admit names exactly one kernel transform or is one
// config parse refuses, and every word the two tables hold is one a module admits.
func TestXfrmCipherVocabularyMatchesModel(t *testing.T) {
	model := modelWords(t, espEncryptionLeaf, ospfEncLeaf)

	reaching := make([]string, 0, len(model))
	for _, word := range model {
		_, plain := xfrmEncNames[word]
		_, aead := xfrmAEADNames[word]
		switch {
		case espParseRefused[word] && (plain || aead):
			t.Errorf("config parse refuses %q for an ESP proposal and an xfrm table holds it, so the table names a transform nothing proved", word)
		case espParseRefused[word]:
			if _, err := xfrmAEADName(word); !errors.Is(err, ErrNotSupported) {
				t.Errorf("xfrmAEADName(%q) err = %v, want ErrNotSupported", word, err)
			}
		case plain && aead:
			t.Errorf("%q is in both xfrmEncNames and xfrmAEADNames, so the state carries two transforms for one cipher", word)
		case !plain && !aead:
			t.Errorf("the model admits cipher %q and neither xfrm table holds it, so a Child SA that negotiates it installs nothing", word)
		default:
			reaching = append(reaching, word)
		}
	}

	held := slices.Concat(sortedKeys(xfrmEncNames), sortedKeys(xfrmAEADNames))
	slices.Sort(held)
	if !slices.Equal(held, reaching) {
		t.Errorf("the ciphers disagree: the modules admit %v and the xfrm tables hold %v", reaching, held)
	}

	for word := range espParseRefused {
		if !slices.Contains(model, word) {
			t.Errorf("espParseRefused names %q and the model no longer offers it, so the entry excuses nothing", word)
		}
	}
}

// TestXfrmAuthVocabularyMatchesModel compares the integrity table with the words the
// two modules admit.
func TestXfrmAuthVocabularyMatchesModel(t *testing.T) {
	model := modelWords(t, espHashLeaf, ospfAuthLeaf)
	held := sortedKeys(xfrmAuthNames)
	if !slices.Equal(held, model) {
		t.Errorf("the integrity algorithms disagree: the modules admit %v and xfrmAuthNames holds %v. "+
			"A word only the model carries is refused at install time, so the SA authenticates nothing",
			model, held)
	}
}

// modelWords answers the union of the enumerations at the named leaves, sorted and
// deduplicated. EnumValues FAILS on a leaf that declares no enumeration rather than
// answering an empty set, so a comparison here never passes over nothing
// (ai/rules/evidence.md).
func modelWords(t *testing.T, leaves ...string) []string {
	t.Helper()

	seen := map[string]bool{}
	for _, leaf := range leaves {
		values, err := configyang.EnumValues(leaf)
		if err != nil {
			t.Fatalf("read the enumeration at %s: %v", leaf, err)
		}
		for _, value := range values {
			seen[value] = true
		}
	}
	return sortedKeys(seen)
}

func sortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
