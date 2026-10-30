package yang

import (
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// referenceInstancesRoot is the data tree TestReferenceInstances completes
// over: two interfaces, eth0 with two addresses and lo with one, and the
// default-address container naming ifname.
func referenceInstancesRoot(ifname string) map[string]any {
	return map[string]any{"sys": map[string]any{
		"interface": map[string]any{
			"eth0": map[string]any{"address": map[string]any{"192.0.2.1": map[string]any{}, "192.0.2.2": map[string]any{}}},
			"lo":   map[string]any{"address": map[string]any{"127.0.0.1": map[string]any{}}},
		},
		"default-address": map[string]any{"ifname": ifname},
	}}
}

// TestReferenceInstances checks the values a leafref may name at one data
// position. Method: referenceModule's leafrefs over a fixed tree, one case per
// path shape: relative, a current() predicate (whose answer moves with the
// value the predicate reads), absolute to a leaf whose default is in use, a
// union member, a leaf that is no reference, and paths the schema does not
// describe.
//
// VALIDATES: Validator.ReferenceInstances answers exactly, and sorted, what
// the commit walk accepts at that position.
// PREVENTS: completion offering every instance of a predicated path, or
// nothing for a union member.
func TestReferenceInstances(t *testing.T) {
	v := referenceValidator(t)
	cases := []struct {
		name   string
		ifname string
		path   []string
		want   []string
	}{
		{"relative path", "eth0", []string{"sys", "mgmt"}, []string{"eth0", "lo"}},
		{"require-instance false still names instances", "eth0", []string{"sys", "loose"}, []string{"eth0", "lo"}},
		{"predicate reads current() eth0", "eth0", []string{"sys", "default-address", "address"}, []string{"192.0.2.1", "192.0.2.2"}},
		{"predicate reads current() lo", "lo", []string{"sys", "default-address", "address"}, []string{"127.0.0.1"}},
		{"two levels up", "eth0", []string{"sys", "default-address", "ifname"}, []string{"eth0", "lo"}},
		{"absolute path, default in use", "eth0", []string{"sys", "mtu-ref"}, []string{"1500"}},
		{"union member", "eth0", []string{"sys", "either"}, []string{"eth0", "lo"}},
		{"not a reference", "eth0", []string{"sys", "crypto"}, nil},
		{"unknown leaf", "eth0", []string{"sys", "nothing"}, nil},
		{"unknown section", "eth0", []string{"nosuch", "mgmt"}, nil},
		{"container, not a leaf", "eth0", []string{"sys", "default-address"}, nil},
		{"list with no entry key", "eth0", []string{"sys", "interface", "name"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, v.ReferenceInstances(referenceInstancesRoot(tc.ifname), tc.path))
		})
	}
}

// probeSourceModule declares a ze:reference leaf inside a list entry and as
// a leaf-list, both naming the probe-paint source.
const probeSourceModule = `module ze-probe-conf {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    container probe {
        list paint { key name; leaf name { type string; } }
        list item {
            key name;
            leaf name { type string; }
            leaf color { type string; ze:reference "probe-paint"; }
            leaf-list colors { type string; ze:reference "probe-paint"; }
            leaf lost { type string; ze:reference "probe-unregistered"; }
        }
    }
}`

// probePaintAsked records every data path the probe-paint source was asked at.
var (
	probePaintMu    sync.Mutex
	probePaintAsked [][]string
)

func init() {
	RegisterReferenceSource("probe-paint", func(root map[string]any, at []string) ReferenceAnswer {
		probePaintMu.Lock()
		probePaintAsked = append(probePaintAsked, slices.Clone(at))
		probePaintMu.Unlock()
		probe, _ := root["probe"].(map[string]any)
		paint, _ := probe["paint"].(map[string]any)
		names := make([]string, 0, len(paint))
		for name := range paint {
			names = append(names, name)
		}
		return ReferenceAnswer{Names: names}
	})
}

// probeSourceValidator loads the embedded modules and probeSourceModule.
func probeSourceValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-probe-conf.yang", probeSourceModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	return NewValidator(resolved)
}

// probeRoot is a probe tree with paints red and blue and one item a holding
// the given leaves.
func probeRoot(item map[string]any) map[string]any {
	return map[string]any{"probe": map[string]any{
		"paint": map[string]any{"red": map[string]any{}, "blue": map[string]any{}},
		"item":  map[string]any{"a": item},
	}}
}

// TestReferenceSourceCompletes checks that a ze:reference leaf completes the
// names its registered source lists, and that the source is asked with the
// leaf's data path. Method: the probe-paint source over probeRoot.
//
// VALIDATES: ReferenceInstances asks the ze:reference source, and hands it
// the data path in the leafPath form.
// PREVENTS: a ze:reference leaf completing nothing, or a source reading the
// wrong position.
func TestReferenceSourceCompletes(t *testing.T) {
	v := probeSourceValidator(t)
	path := []string{"probe", "item", "a", "color"}
	assert.Equal(t, []string{"blue", "red"}, v.ReferenceInstances(probeRoot(map[string]any{}), path))
	assert.Equal(t, []string{"blue", "red"}, v.ReferenceInstances(probeRoot(map[string]any{}), []string{"probe", "item", "new", "colors"}),
		"an entry the tree does not hold yet still completes")
	assert.Nil(t, v.ReferenceInstances(probeRoot(map[string]any{}), []string{"probe", "item", "a", "lost"}),
		"an unregistered source names nothing")
	probePaintMu.Lock()
	defer probePaintMu.Unlock()
	assert.Contains(t, probePaintAsked, path)
}

// TestReferenceSourceRefusesAtCommit checks that commit validation refuses a
// ze:reference value its source does not list, through the leafref route,
// with a message naming the leaf, the value and the configured choices, and
// accepts a listed one. Method: ValidateReferences over probeRoot.
//
// VALIDATES: ValidateReferences checks ze:reference leaves, leaf and
// leaf-list, and names an unregistered source.
// PREVENTS: completion offering a set commit does not enforce.
func TestReferenceSourceRefusesAtCommit(t *testing.T) {
	v := probeSourceValidator(t)
	assert.Empty(t, v.ValidateReferences(probeRoot(map[string]any{"color": "red", "colors": []any{"red", "blue"}})))

	errs := v.ValidateReferences(probeRoot(map[string]any{"color": "green"}))
	require.Len(t, errs, 1, "%v", errs)
	assert.Equal(t, "probe/item[a]/color", errs[0].Path)
	assert.Equal(t, ErrTypeReference, errs[0].Type)
	assert.Equal(t, `no color "green" is configured: name one of "blue red"`, errs[0].Message)

	errs = v.ValidateReferences(probeRoot(map[string]any{"colors": []any{"red", "pink"}}))
	require.Len(t, errs, 1, "%v", errs)
	assert.Contains(t, errs[0].Message, `"pink"`)

	errs = v.ValidateReferences(probeRoot(map[string]any{"lost": "x"}))
	require.Len(t, errs, 1, "%v", errs)
	assert.Contains(t, errs[0].Message, `"probe-unregistered", which no component in this build registers`)

	err := CheckAllReferenceSourcesRegistered(v.schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe-unregistered")
	assert.NotContains(t, err.Error(), "probe-paint")
}
