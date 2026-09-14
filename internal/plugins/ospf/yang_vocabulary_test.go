// Design: docs/architecture/ospf/ospf-4-config.md -- OSPF config resolution
// Related: types/vocabulary.go -- the network types and the area types every OSPF package reads
// Related: config.go -- the OSPFv3 IPsec words this resolver reads
// Related: packet/auth_verify.go -- the OSPFv2 authentication algorithm words
// Related: opaque_registry.go -- OpaqueScope, the flooding-scope words
//
// Five vocabularies cross the configuration boundary into this plugin: the network type
// of an interface, the type of an area, the authentication algorithm of a key-chain key,
// the OSPFv3 IPsec transforms of RFC 4552, and the RFC 5250 flooding scope of an opaque
// LSA. ze-ospf-conf.yang and ze-ospf-cmd.yang decide which words an operator can type,
// and the Go side decides what each word selects. Neither side derives
// from the other, so the two are gated against each other here: a word added to the module
// alone reaches the resolver as an unknown value, and a word added to Go alone is one no
// operator can ask for (ai/rules/principles.md).

package ospf

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"

	// The blank import registers ze-ospf-conf with the loader. It imports only the
	// modules the loader embeds, so the trees below resolve in a binary that links
	// nothing else.
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

const (
	ospfModule    = "ze-ospf-conf"
	ospfCmdModule = "ze-ospf-cmd"
)

// afPath is one address family of the model. Every family uses one grouping, so a
// vocabulary read here is the one every family carries.
var afPath = []string{"ospf", "address-family", "ipv6"}

// TestNetworkTypeVocabularyMatchesModel compares the words types/vocabulary.go declares
// with the network-type enumeration of the base interface list.
//
// NetworkVirtual is left out on purpose: a virtual link is configured as one, and the
// engine gives its synthetic interface that network type, so no leaf offers the word.
func TestNetworkTypeVocabularyMatchesModel(t *testing.T) {
	loader := loadOSPFModel(t)

	configurable := sorted([]string{
		types.NetworkBroadcast,
		types.NetworkPointToPoint,
		types.NetworkLoopback,
		types.NetworkNBMA,
		types.NetworkPointToMultipoint,
	})

	model := modelEnum(t, loader, []string{"ospf", "interfaces", "interface", "network-type"})
	if !slices.Equal(model, configurable) {
		t.Errorf("the network types disagree: the model holds %v and types/vocabulary.go declares %v (NetworkVirtual excluded, which no leaf offers). "+
			"A word only the model carries reaches the ISM as a type it has no arm for",
			model, configurable)
	}

	// The address families declare the same leaf in their own grouping, and that copy
	// is allowed to offer FEWER words than the base one. It MUST NOT offer a word the
	// engine cannot act on.
	af := modelEnum(t, loader, append(slices.Clone(afPath), "interfaces", "interface", "network-type"))
	for _, word := range af {
		if slices.Contains(configurable, word) {
			continue
		}
		t.Errorf("the address-family network-type leaf offers %q and no OSPF package carries that word, "+
			"so an operator can configure a family the engine cannot run", word)
	}
}

// TestAreaTypeVocabularyMatchesModel compares the area-type words with the model, at the
// base list and at the address-family copy of it.
func TestAreaTypeVocabularyMatchesModel(t *testing.T) {
	loader := loadOSPFModel(t)

	declared := sorted([]string{types.AreaTypeNormal, types.AreaTypeStub, types.AreaTypeNSSA})

	paths := [][]string{
		{"ospf", "areas", "area", "area-type"},
		append(slices.Clone(afPath), "areas", "area", "area-type"),
	}
	for _, path := range paths {
		model := modelEnum(t, loader, path)
		if slices.Equal(model, declared) {
			continue
		}
		t.Errorf("the area types disagree: the model at %v holds %v and types/vocabulary.go declares %v. "+
			"An area type only the model carries is flooded as a normal area, which leaks the LSAs the operator asked to keep out",
			path, model, declared)
	}
}

// TestAuthAlgorithmVocabularyMatchesModel compares the OSPFv2 authentication algorithm
// words with the key-chains enumeration.
func TestAuthAlgorithmVocabularyMatchesModel(t *testing.T) {
	loader := loadOSPFModel(t)

	declared := sorted([]string{
		packet.AuthSimple,
		packet.AuthMD5,
		packet.AuthHMACSHA1,
		packet.AuthHMACSHA256,
		packet.AuthHMACSHA384,
		packet.AuthHMACSHA512,
	})

	model := modelEnum(t, loader, []string{"ospf", "key-chains", "key", "algorithm"})
	if !slices.Equal(model, declared) {
		t.Errorf("the authentication algorithms disagree: the model holds %v and the packet package declares %v. "+
			"An algorithm only the model carries signs nothing, and the adjacency it was configured for never forms",
			model, declared)
	}
}

// TestIPsecVocabularyMatchesModel compares the RFC 4552 transform words this resolver
// reads with the address-family ipsec container.
func TestIPsecVocabularyMatchesModel(t *testing.T) {
	loader := loadOSPFModel(t)

	cases := []struct {
		what  string
		leaf  string
		words []string
	}{
		{
			what:  "the integrity algorithms",
			leaf:  "algorithm",
			words: sorted([]string{ipsecAuthSHA1, ipsecAuthSHA256, ipsecAuthSHA384, ipsecAuthSHA512}),
		},
		{
			what:  "the confidentiality algorithms",
			leaf:  "encryption-algorithm",
			words: sorted([]string{ipsecEncNull, ipsecEncAES128, ipsecEncAES256}),
		},
	}
	for _, tc := range cases {
		path := append(slices.Clone(afPath), "interfaces", "interface", "ipsec", tc.leaf)
		model := modelEnum(t, loader, path)
		if slices.Equal(model, tc.words) {
			continue
		}
		t.Errorf("%s disagree: the model at %v holds %v and config.go reads %v. "+
			"A transform only the model carries installs no Security Association, so the interface sends OSPFv3 in clear",
			tc.what, path, model, tc.words)
	}
}

// TestOpaqueScopeVocabularyMatchesModel checks both directions of the flooding-scope
// binding at the three leaves that declare it: every word a leaf admits selects a scope
// through parseOpaqueScope, and every scope the type carries is a word each leaf holds.
// The config leaf reaches parseRouterInformation and the two debug leaves reach
// parseOpaqueScopeKeyword, so a word one side gains alone is either a scope no operator
// can ask for or a word the daemon drops without a message.
func TestOpaqueScopeVocabularyMatchesModel(t *testing.T) {
	loader := loadOSPFModel(t)

	spelled := make([]string, 0, len(opaqueScopes))
	for _, scope := range opaqueScopes {
		spelled = append(spelled, scope.String())
	}
	spelled = sorted(spelled)

	leaves := []struct {
		module string
		path   []string
	}{
		{module: ospfModule, path: []string{"ospf", "router-information", "scope"}},
		{module: ospfCmdModule, path: []string{"debug", "ip", "ospf", "inject", "opaque", "scope", "scope"}},
		{module: ospfCmdModule, path: []string{"debug", "ipv6", "ospf", "inject", "lsa", "scope", "scope"}},
	}
	for _, leaf := range leaves {
		model := modelEnumIn(t, loader, leaf.module, leaf.path)
		if !slices.Equal(model, spelled) {
			t.Errorf("the flooding scopes disagree: %s at %v holds %v and OpaqueScope spells %v",
				leaf.module, leaf.path, model, spelled)
		}
		for _, word := range model {
			scope, ok := parseOpaqueScope(word)
			if !ok {
				t.Errorf("%s admits scope %q and parseOpaqueScope does not know it, so the LSA is flooded at no scope at all", leaf.module, word)
				continue
			}
			if scope.String() != word {
				t.Errorf("parseOpaqueScope(%q) selected %s, so the scope an operator named is not the one the LSA is flooded at", word, scope)
			}
		}
	}

	if _, ok := parseOpaqueScope("no-such-scope"); ok {
		t.Error("parseOpaqueScope accepted a word no leaf holds")
	}
}

func loadOSPFModel(t *testing.T) *configyang.Loader {
	t.Helper()

	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load the YANG model: %v", err)
	}
	return loader
}

// modelEnum answers the values of the enumeration at one leaf of the config module.
func modelEnum(t *testing.T, loader *configyang.Loader, path []string) []string {
	t.Helper()
	return modelEnumIn(t, loader, ospfModule, path)
}

// modelEnumIn answers the values of the enumeration at one leaf of the named module.
//
// It FAILS on a path that names no enumeration rather than answering an empty set.
// DefaultLoader discards its own LoadRegistered and Resolve errors, so a model that
// loaded half way comes back looking whole, and an empty set here would let every
// comparison above pass over nothing (ai/rules/evidence.md).
func modelEnumIn(t *testing.T, loader *configyang.Loader, module string, path []string) []string {
	t.Helper()

	entry := loader.GetEntry(module)
	if entry == nil {
		t.Fatalf("the loaded model holds no module %s: this binary registered nothing to compare against", module)
	}
	walked := module
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

func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}
