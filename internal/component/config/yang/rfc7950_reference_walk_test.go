// VALIDATES: the whole-tree reference walk honors defaults in use, selects a
// keyless entry in data order, refuses an unprefixed key predicate, reports a
// missing reference with its own kind, and stays linear in the referred list.
// PREVENTS: a quadratic commit, and a valid reference refused for a default.

package yang

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// defaultsModule holds the defaults a reference can name while the referred
// leaf is absent from the data: one under a non-presence container, one in a
// choice's default case and one in its other case, and an integer default a
// module wrote in hexadecimal. slot is a keyless state list, selected by
// position.
const defaultsModule = `module dv-conf {
    namespace "urn:ze:test:dv";
    prefix dv;
    container box {
        container limits { leaf mtu { type uint16; default 0x5DC; } }
        choice mode {
            default auto;
            case auto { leaf speed { type uint32; default 1000; } }
            case manual { leaf rate { type uint32; default 10; } leaf burst { type uint32; } }
        }
        leaf mtu-ref { type leafref { path "../limits/mtu"; } }
        leaf speed-ref { type leafref { path "../speed"; } }
        leaf rate-ref { type leafref { path "../rate"; } }
        leaf target { type instance-identifier; }
        list slot { config false; leaf note { type string; } }
    }
}`

// defaultsValidator loads defaultsModule and answers a validator over it.
func defaultsValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("dv-conf.yang", defaultsModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	return NewValidator(resolved)
}

// boxErrors runs the whole-tree reference walk over a box section.
func boxErrors(v *Validator, box map[string]any) []ValidationError {
	return v.ValidateReferences(map[string]any{"box": box})
}

// TestRFC7950LeafrefDefaultInUse accepts a leafref value that only a default
// in use holds: under an absent non-presence container, written in hex in the
// module, and in a choice's default case while no case has data.
//
// RFC requirement: RFC7950-9.9-1 positive -- mtu-ref "1500" is accepted when limits, a non-presence container, is absent and when it is present but empty, the mtu default being written "0x5DC" in the module; speed-ref "1000" is accepted while no case of choice mode has data (auto, holding speed, is the default case); rate-ref "10" is accepted when burst puts the manual case in the data tree.
func TestRFC7950LeafrefDefaultInUse(t *testing.T) {
	v := defaultsValidator(t)
	assert.Empty(t, boxErrors(v, map[string]any{"mtu-ref": "1500"}))
	assert.Empty(t, boxErrors(v, map[string]any{"limits": map[string]any{}, "mtu-ref": "1500"}))
	assert.Empty(t, boxErrors(v, map[string]any{"speed-ref": "1000"}))
	assert.Empty(t, boxErrors(v, map[string]any{"burst": "5", "rate-ref": "10"}))
}

// TestRFC7950LeafrefDefaultNotInUse refuses a leafref value whose default is
// not in use: a case that is not the default case while no case has data, and
// a default case while another case has data.
//
// RFC requirement: RFC7950-9.9-1 negative -- rate-ref "10" is refused while no case of choice mode has data (manual is not the default case), and speed-ref "1000" is refused when burst puts the manual case in the data tree; mtu-ref "1501" is refused against the mtu default 0x5DC.
func TestRFC7950LeafrefDefaultNotInUse(t *testing.T) {
	v := defaultsValidator(t)
	requireRefused(t, boxErrors(v, map[string]any{"rate-ref": "10"}), "rate-ref")
	requireRefused(t, boxErrors(v, map[string]any{"burst": "5", "speed-ref": "1000"}), "speed-ref")
	requireRefused(t, boxErrors(v, map[string]any{"mtu-ref": "1501"}), "mtu-ref")
}

// TestRFC7950InstanceIdentifierDefaultInUse accepts an instance-identifier
// naming a leaf whose default is in use though the leaf is absent.
//
// RFC requirement: RFC7950-9.13-1 positive -- target accepts "/dv:box/dv:limits/dv:mtu" while limits, a non-presence container, is absent, and "/dv:box/dv:speed" while no case of choice mode has data; it refuses "/dv:box/dv:rate" then, manual not being the default case.
func TestRFC7950InstanceIdentifierDefaultInUse(t *testing.T) {
	v := defaultsValidator(t)
	assert.Empty(t, boxErrors(v, map[string]any{"target": "/dv:box/dv:limits/dv:mtu"}))
	assert.Empty(t, boxErrors(v, map[string]any{"target": "/dv:box/dv:speed"}))
	requireRefused(t, boxErrors(v, map[string]any{"target": "/dv:box/dv:rate"}), "target")
}

// TestInstanceIdentifierPositionInDataOrder selects a keyless list entry by
// position in the order the data delivers, never the order a Go map yields.
// The entries are ordered "b" then "a", against their sorted order, and only
// "a" holds a note.
func TestInstanceIdentifierPositionInDataOrder(t *testing.T) {
	v := defaultsValidator(t)
	box := map[string]any{
		"slot": map[string]any{
			"a": map[string]any{"note": "x"},
			"b": map[string]any{},
		},
		configorder.OrderKey("slot"): []string{"b", "a"},
		"target":                     "/dv:box/dv:slot[2]/dv:note",
	}
	for range 20 {
		require.Empty(t, boxErrors(v, box))
	}
	box["target"] = "/dv:box/dv:slot[1]/dv:note"
	requireRefused(t, boxErrors(v, box), "target")
	box["target"] = "/dv:box/dv:slot[3]"
	requireRefused(t, boxErrors(v, box), "target")
	delete(box, configorder.OrderKey("slot"))
	box["target"] = "/dv:box/dv:slot[2]/dv:note"
	errs := boxErrors(v, box)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "order")
}

// TestRFC7950InstanceIdentifierKeyPrefix refuses a key predicate whose node
// name carries no prefix, or the prefix of no module defining the key.
//
// RFC requirement: RFC7950-9.1-2 negative -- target refuses "/rf:sys/rf:interface[name='eth0']" (an unprefixed key) and "/rf:sys/rf:interface[xx:name='eth0']" (a prefix of no module), each with a message naming the key's prefix, while "/rf:sys/rf:interface[rf:name='eth0']" is accepted.
func TestRFC7950InstanceIdentifierKeyPrefix(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["target"]
	require.NoError(t, v.validateLeafValue("sys/target", leaf, leaf.Type, "/rf:sys/rf:interface[rf:name='eth0']"))
	err := v.validateLeafValue("sys/target", leaf, leaf.Type, "/rf:sys/rf:interface[name='eth0']")
	requireValidationErrorType(t, err, ErrTypeType, "unprefixed key")
	assert.Contains(t, err.Error(), `key "name" has no module prefix`)
	err = v.validateLeafValue("sys/target", leaf, leaf.Type, "/rf:sys/rf:interface[xx:name='eth0']")
	requireValidationErrorType(t, err, ErrTypeType, "foreign key prefix")
	assert.Contains(t, err.Error(), `prefix "xx" is not the module of "name"`)
}

// TestReferenceFailureMessage pins the kind and the wording of a missing
// reference: its own error kind, the value, and what to configure.
func TestReferenceFailureMessage(t *testing.T) {
	v := referenceValidator(t)
	errs := v.ValidateReferences(map[string]any{"sys": map[string]any{"mgmt": "eth0"}})
	require.Len(t, errs, 1)
	assert.Equal(t, ErrTypeReference, errs[0].Type)
	text := errs[0].Error()
	assert.True(t, strings.HasPrefix(text, "reference error at sys/mgmt: "), text)
	assert.Contains(t, text, `no name "eth0" exists at "../interface/name"`)
	assert.Contains(t, text, "add interface eth0 under sys")
}

// TestRFC7950UnionDefaultLeafrefMember reads a union default through its
// leafref member, which checks the value against the leaf it refers to, and
// names that member when no member accepts the default.
//
// RFC requirement: RFC7950-9.1-1 positive -- the default "eth0" of a union of uint8 and a leafref to a string leaf is read by CanonicalModuleDefault as the leafref member's value, and the module loads; the default "300" of a union of uint8 and a leafref to a uint8 leaf is refused, naming the leafref member.
func TestRFC7950UnionDefaultLeafrefMember(t *testing.T) {
	const module = `module ud-conf { namespace "urn:ze:test:ud"; prefix ud;
        leaf name { type string; }
        leaf small { type uint8; }
        leaf either { type union { type uint8; type leafref { path "../name"; } } default "eth0"; }
        leaf narrow { type union { type uint8; type leafref { path "../small"; } } }
    }`
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("ud-conf.yang", module))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	root := resolved.GetEntry("ud-conf")
	either := root.Dir["either"]
	canonical, err := CanonicalModuleDefault(either, either.Type, "eth0")
	require.NoError(t, err)
	assert.Equal(t, "eth0", canonical)
	narrow := root.Dir["narrow"]
	_, err = CanonicalModuleDefault(narrow, narrow.Type, "300")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `leafref "../small"`)
}

// TestRFC7950LeafrefUnionLoop refuses two leaves whose union members refer to
// each other, a circular chain the loop check reaches only through a union.
//
// RFC requirement: RFC7950-9.9-1 negative -- Loader.Resolve refuses leaves p and q, each a union of uint8 and a leafref to the other, with ErrLeafrefPath naming the loop.
func TestRFC7950LeafrefUnionLoop(t *testing.T) {
	err := loadModuleTexts(t, `module ul-conf { namespace "urn:ze:test:ul"; prefix ul;
        leaf p { type union { type uint8; type leafref { path "../q"; } } }
        leaf q { type union { type uint8; type leafref { path "../p"; } } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), "comes back")
}

// scaleModule refers from one list to another, both of entries entries.
const scaleModule = `module sc-conf {
    namespace "urn:ze:test:sc";
    prefix sc;
    container sys {
        list interface { key name; leaf name { type string; } }
        list binding {
            key id;
            leaf id { type string; }
            leaf ifname { type leafref { path "/sc:sys/sc:interface/sc:name"; } }
            leaf local { type leafref { path "../../interface/name"; } }
            leaf target { type instance-identifier; }
        }
    }
    container plain { list item { key k; leaf k { type string; } leaf v { type uint32; } } }
}`

// scaleData builds entries interfaces and as many bindings, each binding
// referring to one interface by leafref and by instance-identifier, and as
// many plain items, which hold no reference.
func scaleData(entries int) map[string]any {
	interfaces := make(map[string]any, entries)
	bindings := make(map[string]any, entries)
	items := make(map[string]any, entries)
	var tb textbuf.Buffer
	for i := range entries {
		name := tb.Reset().Str("eth").Int(int64(i)).String()
		interfaces[name] = map[string]any{}
		bindings[name] = map[string]any{
			"ifname": name,
			"local":  name,
			"target": tb.Reset().Str("/sc:sys/sc:interface[sc:name='").Str(name).Str("']").String(),
		}
		items[name] = map[string]any{"v": "1"}
	}
	return map[string]any{
		"sys":   map[string]any{"interface": interfaces, "binding": bindings},
		"plain": map[string]any{"item": items},
	}
}

// scaleValidator loads scaleModule.
func scaleValidator(tb testing.TB) *Validator {
	tb.Helper()
	loader := NewLoader()
	require.NoError(tb, loader.AddModuleFromText("sc-conf.yang", scaleModule))
	resolved, err := loader.Resolve()
	require.NoError(tb, err)
	return NewValidator(resolved)
}

// TestReferenceWalkScales bounds the whole-tree walk at 20000 referring
// entries. A walk that evaluates each value's path over the whole referred
// list is quadratic and took 88 seconds at 10000 entries; one that indexes
// the referred instances once is linear and finishes well inside the bound.
func TestReferenceWalkScales(t *testing.T) {
	v := scaleValidator(t)
	data := scaleData(20000)
	start := time.Now()
	errs := v.ValidateReferences(data)
	elapsed := time.Since(start)
	assert.Empty(t, errs)
	assert.Less(t, elapsed, 10*time.Second, "the reference walk over 20000 entries took %s", elapsed)
}

// TestReferenceWalkSkipsSubtreesWithoutReference pins that the walk enters no
// subtree whose schema holds no reference type.
func TestReferenceWalkSkipsSubtreesWithoutReference(t *testing.T) {
	v := scaleValidator(t)
	walk := newReferenceWalk(v)
	root := v.schema.GetEntry("sc-conf")
	assert.False(t, walk.holdsReference(root.Dir["plain"]))
	assert.True(t, walk.holdsReference(root.Dir["sys"]))
	assert.False(t, walk.holdsReference(root.Dir["sys"].Dir["interface"]))
}

// BenchmarkReferenceWalk measures the whole-tree walk at 10000 entries.
func BenchmarkReferenceWalk(b *testing.B) {
	v := scaleValidator(b)
	data := scaleData(10000)
	b.ResetTimer()
	for b.Loop() {
		if errs := v.ValidateReferences(data); len(errs) > 0 {
			b.Fatal(errs[0].Error())
		}
	}
}
