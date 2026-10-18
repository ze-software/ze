package yang

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// referenceModule declares one leaf of each reference type, shaped on the
// RFC 7950 Section 9.9.6 and 9.10.5 examples.
const referenceModule = `module ref-conf {
    namespace "urn:ze:test:ref";
    prefix rf;
    identity crypto;
    identity aes { base crypto; }
    identity des { base crypto; }
    identity des3 { base des; }
    container sys {
        list interface {
            key name;
            leaf name { type string; }
            leaf mtu { type uint16; default 1500; }
            list address { key ip; leaf ip { type string; } }
        }
        leaf mgmt { type leafref { path "../interface/name"; } }
        leaf loose { type leafref { path "../interface/name"; require-instance false; } }
        container default-address {
            leaf ifname { type leafref { path "../../interface/name"; } }
            leaf address { type leafref { path "../../interface[name = current()/../ifname]/address/ip"; } }
        }
        leaf mtu-ref { type leafref { path "/rf:sys/rf:interface/rf:mtu"; } }
        leaf crypto { type identityref { base crypto; } }
        leaf target { type instance-identifier; }
        leaf loose-target { type instance-identifier { require-instance false; } }
        leaf either { type union { type uint8; type leafref { path "../interface/name"; } } }
    }
}`

// referenceValidator loads referenceModule and answers a validator over it.
func referenceValidator(t *testing.T) *Validator {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("ref-conf.yang", referenceModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	return NewValidator(resolved)
}

// referenceErrors runs the per-section walk and the whole-tree reference walk
// over a config holding the sys section with the given extra leaves, the
// two walks config.ValidateCustomSections runs.
func referenceErrors(t *testing.T, v *Validator, leaves map[string]any) []ValidationError {
	t.Helper()
	sys := map[string]any{
		"interface": map[string]any{
			"eth0": map[string]any{"address": map[string]any{"192.0.2.1": map[string]any{}, "192.0.2.2": map[string]any{}}},
			"lo":   map[string]any{"address": map[string]any{"127.0.0.1": map[string]any{}}},
		},
	}
	maps.Copy(sys, leaves)
	errs := v.ValidateTreeAllModules("sys", sys)
	return append(errs, v.ValidateReferences(map[string]any{"sys": sys})...)
}

// requireRefused asserts the config is refused at the leaf named.
func requireRefused(t *testing.T, errs []ValidationError, leaf string) {
	t.Helper()
	require.NotEmpty(t, errs, "%s must be refused", leaf)
	assert.Contains(t, errs[0].Path, leaf, "%v", errs)
}

// TestRFC7950ReferenceTypesLoad loads a module declaring every reference
// type, which the loader refused before this feature.
//
// RFC requirement: RFC7950-9.1-1 positive -- Loader.Resolve accepts a module declaring leafref (relative, absolute, with a current() predicate, with require-instance false, as a union member), identityref and instance-identifier leaves.
func TestRFC7950ReferenceTypesLoad(t *testing.T) {
	referenceValidator(t)
}

// TestRFC7950LeafrefValueAccepted drives values that equal a configured
// instance of the referred leaf, or its default in use.
//
// RFC requirement: RFC7950-9.9-1 positive -- mgmt "eth0" (a configured interface name), mtu-ref "1500" (the default of mtu, in use on both interfaces) and default-address/address "192.0.2.2" with ifname "eth0" (an address of eth0, selected through the current() predicate) are each accepted by the per-section and whole-tree walks.
func TestRFC7950LeafrefValueAccepted(t *testing.T) {
	v := referenceValidator(t)
	for name, leaves := range map[string]map[string]any{
		"configured":    {"mgmt": "eth0"},
		"default":       {"mtu-ref": "1500"},
		"predicate":     {"default-address": map[string]any{"ifname": "eth0", "address": "192.0.2.2"}},
		"union-leafref": {"either": "lo"},
	} {
		assert.Empty(t, referenceErrors(t, v, leaves), name)
	}
}

// TestRFC7950LeafrefInstanceRefused drives values no instance carries.
//
// RFC requirement: RFC7950-9.9-1 negative -- mgmt "eth9" (no interface has that name), mtu-ref "9000" (no mtu holds it, configured or default) and default-address/address "127.0.0.1" with ifname "eth0" (an address of lo, not of the interface the predicate selects) are each refused by ValidateReferences.
func TestRFC7950LeafrefInstanceRefused(t *testing.T) {
	v := referenceValidator(t)
	requireRefused(t, referenceErrors(t, v, map[string]any{"mgmt": "eth9"}), "mgmt")
	requireRefused(t, referenceErrors(t, v, map[string]any{"mtu-ref": "9000"}), "mtu-ref")
	requireRefused(t, referenceErrors(t, v, map[string]any{"default-address": map[string]any{"ifname": "eth0", "address": "127.0.0.1"}}), "address")
	requireRefused(t, referenceErrors(t, v, map[string]any{"either": "eth9"}), "either")
}

// TestRFC7950LeafrefRequireInstance pins require-instance: true (the
// default) needs the instance, false does not.
//
// RFC requirement: RFC7950-9.9.3-1 positive -- loose, a leafref with require-instance false, accepts "eth9", which no interface carries, while mgmt, the same path with require-instance absent (true), refuses it.
func TestRFC7950LeafrefRequireInstance(t *testing.T) {
	v := referenceValidator(t)
	assert.Empty(t, referenceErrors(t, v, map[string]any{"loose": "eth9"}))
	requireRefused(t, referenceErrors(t, v, map[string]any{"mgmt": "eth9"}), "mgmt")
}

// TestRFC7950LeafrefRequireInstanceMissing pins that require-instance true
// refuses data whose referred instance is absent, across sections too.
//
// RFC requirement: RFC7950-9.9.3-1 negative -- mgmt, whose require-instance is absent (true), refuses "eth9" when no interface carries it, and refuses "eth0" when the interface list is absent altogether, through ValidateReferences.
func TestRFC7950LeafrefRequireInstanceMissing(t *testing.T) {
	v := referenceValidator(t)
	requireRefused(t, referenceErrors(t, v, map[string]any{"mgmt": "eth9"}), "mgmt")
	errs := v.ValidateReferences(map[string]any{"sys": map[string]any{"mgmt": "eth0"}})
	requireRefused(t, errs, "mgmt")
	assert.Contains(t, errs[0].Message, "none is configured")
}

// TestRFC7950IdentityrefPrefixRefused pins that a prefix names the module
// defining the identity, and nothing else.
//
// RFC requirement: RFC7950-9.10.3-1 negative -- crypto refuses "xx:aes" (a prefix no loaded module declares) and "ref:aes" (neither the prefix "rf" nor the module name "ref-conf" of the module defining aes).
func TestRFC7950IdentityrefPrefixRefused(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["crypto"]
	for _, value := range []string{"xx:aes", "ref:aes"} {
		requireValidationErrorType(t, v.validateLeafValue("sys/crypto", leaf, leaf.Type, value), ErrTypeType, value)
	}
}

// TestRFC7950LeafrefLexical checks a leafref value against the referred
// leaf's type.
//
// RFC requirement: RFC7950-9.1-1 negative -- mtu-ref, a leafref to a uint16 leaf, refuses "abc" and "70000" as values outside the uint16 lexical space and range, through validateLeafValue.
func TestRFC7950LeafrefLexical(t *testing.T) {
	v := referenceValidator(t)
	sys := v.schema.GetEntry("ref-conf").Dir["sys"]
	leaf := sys.Dir["mtu-ref"]
	require.NotNil(t, leaf)
	assert.NoError(t, v.validateLeafValue("sys/mtu-ref", leaf, leaf.Type, "1500"))
	for _, value := range []string{"abc", "70000"} {
		assert.Error(t, v.validateLeafValue("sys/mtu-ref", leaf, leaf.Type, value), value)
	}
}

// TestRFC7950LeafrefPathRefused loads modules whose leafref path cannot be
// honored.
//
// RFC requirement: RFC7950-9.9.2-1 negative -- Loader.Resolve refuses a leafref whose path names no node ("../nope"), ends at a container ("../box"), or puts a predicate on a non-key leaf ("../l[v = current()/../a]/k"), each with ErrLeafrefPath or a goyang error naming the path.
func TestRFC7950LeafrefPathRefused(t *testing.T) {
	const head = `module lr-conf { namespace "urn:ze:test:lr"; prefix lr; container box { leaf x { type string; } } list l { key k; leaf k { type string; } leaf v { type string; } } leaf a { type string; } `
	for name, body := range map[string]string{
		"no node":       `leaf b { type leafref { path "../nope"; } }`,
		"container":     `leaf b { type leafref { path "../box"; } }`,
		"non-key":       `leaf b { type leafref { path "../l[v = current()/../a]/k"; } }`,
		"current-wrong": `leaf b { type leafref { path "../l[k = current()/../nope]/v"; } }`,
	} {
		err := loadModuleTexts(t, head+body+`}`)
		require.Error(t, err, name)
		assert.ErrorIs(t, err, ErrLeafrefPath, name)
	}
	require.NoError(t, loadModuleTexts(t, head+`leaf b { type leafref { path "../l[k = current()/../a]/v"; } }}`))
}

// TestRFC7950LeafrefPathAccepted loads a leafref to a leaf-list.
//
// RFC requirement: RFC7950-9.9.2-1 positive -- Loader.Resolve accepts a leafref whose path ends at a leaf-list ("../tags") and one whose predicate tests the list key through current().
func TestRFC7950LeafrefPathAccepted(t *testing.T) {
	const module = `module lp-conf { namespace "urn:ze:test:lp"; prefix lp; leaf-list tags { type string; } list l { key k; leaf k { type string; } leaf v { type string; } } leaf a { type string; }
        leaf b { type leafref { path "../tags"; } }
        leaf c { type leafref { path "../l[k = current()/../a]/v"; } } }`
	require.NoError(t, loadModuleTexts(t, module))
}

// TestRFC7950LeafrefConfigAndLoop loads modules a config leafref can not
// honor: a state target, and a circular chain.
//
// RFC requirement: RFC7950-9.9-1 negative -- Loader.Resolve refuses a config leafref requiring an instance of a config false leaf, and two leafrefs that refer to each other, with ErrLeafrefPath; the state target with require-instance false loads.
func TestRFC7950LeafrefConfigAndLoop(t *testing.T) {
	const head = `module lc-conf { namespace "urn:ze:test:lc"; prefix lc; leaf s { config false; type string; } `
	err := loadModuleTexts(t, head+`leaf b { type leafref { path "../s"; } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), "is state")
	err = loadModuleTexts(t, head+`leaf p { type leafref { path "../q"; } } leaf q { type leafref { path "../p"; } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), "comes back")
	require.NoError(t, loadModuleTexts(t, head+`leaf b { type leafref { path "../s"; require-instance false; } } }`))
}

// TestRFC7950IdentityrefAccepted drives identity names derived from the
// base, unprefixed and qualified.
//
// RFC requirement: RFC7950-9.10.2-1 positive -- crypto, an identityref of base crypto, accepts "aes", "rf:aes", "ref-conf:des" and "des3" (derived from crypto through des), through validateLeafValue.
func TestRFC7950IdentityrefAccepted(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["crypto"]
	for _, value := range []string{"aes", "rf:aes", "ref-conf:des", "des3"} {
		assert.NoError(t, v.validateLeafValue("sys/crypto", leaf, leaf.Type, value), value)
	}
}

// TestRFC7950IdentityrefRefused drives names that are no derived identity.
//
// RFC requirement: RFC7950-9.10.2-1 negative -- crypto refuses "crypto" (the base itself, which is not derived from itself), "md5" (no identity), "xx:aes" (a prefix of no module defining aes) and "rf:" (no name).
func TestRFC7950IdentityrefRefused(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["crypto"]
	for _, value := range []string{"crypto", "md5", "xx:aes", "rf:"} {
		requireValidationErrorType(t, v.validateLeafValue("sys/crypto", leaf, leaf.Type, value), ErrTypeType, value)
	}
}

// TestRFC7950IdentityrefOtherModule resolves a base and a value defined in an
// imported module, and an unprefixed name two modules define.
//
// RFC requirement: RFC7950-9.10.3-1 positive -- an identityref with base "rf:crypto" in a module importing ref-conf accepts "rf:aes" and "xo:aes" (its own aes) and refuses the unprefixed "aes", which both modules define, naming the qualified choices.
func TestRFC7950IdentityrefOtherModule(t *testing.T) {
	const other = `module xo-conf { namespace "urn:ze:test:xo"; prefix xo; import ref-conf { prefix rf; }
        identity aes { base rf:crypto; }
        leaf alg { type identityref { base rf:crypto; } } }`
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("ref-conf.yang", referenceModule))
	require.NoError(t, loader.AddModuleFromText("xo-conf.yang", other))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	v := NewValidator(resolved)
	leaf := resolved.GetEntry("xo-conf").Dir["alg"]
	assert.NoError(t, v.validateLeafValue("alg", leaf, leaf.Type, "rf:aes"))
	assert.NoError(t, v.validateLeafValue("alg", leaf, leaf.Type, "xo:aes"))
	err = v.validateLeafValue("alg", leaf, leaf.Type, "aes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module prefix")
}

// TestRFC7950InstanceIdentifierLexical drives instance-identifier values.
//
// RFC requirement: RFC7950-9.1-2 positive -- target, an instance-identifier, accepts "/rf:sys/rf:interface[rf:name='eth0']/rf:mtu" and "/rf:sys/rf:mgmt", and refuses "/sys/mgmt" (no prefix, Section 9.13.2), "rf:sys" (no leading slash), "/rf:sys/rf:nope" and "/rf:sys/rf:interface[rf:name='eth0'" (unclosed predicate).
func TestRFC7950InstanceIdentifierLexical(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["target"]
	for _, value := range []string{"/rf:sys/rf:interface[rf:name='eth0']/rf:mtu", "/rf:sys/rf:mgmt", `/rf:sys/rf:interface[ rf:name = "lo" ]`} {
		assert.NoError(t, v.validateLeafValue("sys/target", leaf, leaf.Type, value), value)
	}
	for _, value := range []string{"/sys/mgmt", "rf:sys", "/rf:sys/rf:nope", "/rf:sys/rf:interface[rf:name='eth0'", ""} {
		requireValidationErrorType(t, v.validateLeafValue("sys/target", leaf, leaf.Type, value), ErrTypeType, value)
	}
}

// TestRFC7950InstanceIdentifierKeys requires one predicate per list key.
//
// RFC requirement: RFC7950-9.13-1 negative -- target refuses "/rf:sys/rf:interface/rf:mtu" (no key predicate), "/rf:sys/rf:interface[rf:mtu='1']" (a predicate on a non-key leaf), "/rf:sys/rf:interface[1]" (a position on a keyed list) and the instance "/rf:sys/rf:interface[rf:name='eth9']", which the tree does not hold.
func TestRFC7950InstanceIdentifierKeys(t *testing.T) {
	v := referenceValidator(t)
	for _, value := range []string{"/rf:sys/rf:interface/rf:mtu", "/rf:sys/rf:interface[rf:mtu='1']", "/rf:sys/rf:interface[1]"} {
		requireRefused(t, referenceErrors(t, v, map[string]any{"target": value}), "target")
	}
	requireRefused(t, referenceErrors(t, v, map[string]any{"target": "/rf:sys/rf:interface[rf:name='eth9']"}), "target")
}

// TestRFC7950InstanceIdentifierExists accepts instances the tree holds.
//
// RFC requirement: RFC7950-9.13-1 positive -- target accepts "/rf:sys/rf:interface[rf:name='eth0']" (a configured entry) and "/rf:sys/rf:interface[rf:name='lo']/rf:mtu" (a leaf whose default is in use), and loose-target, with require-instance false, accepts "/rf:sys/rf:interface[rf:name='eth9']".
func TestRFC7950InstanceIdentifierExists(t *testing.T) {
	v := referenceValidator(t)
	assert.Empty(t, referenceErrors(t, v, map[string]any{"target": "/rf:sys/rf:interface[rf:name='eth0']"}))
	assert.Empty(t, referenceErrors(t, v, map[string]any{"target": "/rf:sys/rf:interface[rf:name='lo']/rf:mtu"}))
	assert.Empty(t, referenceErrors(t, v, map[string]any{"loose-target": "/rf:sys/rf:interface[rf:name='eth9']"}))
}

// TestRFC7950IdentityrefFormat pins that an identityref value matches its
// lexical representation, the type having no canonical form.
//
// RFC requirement: RFC7950-9.1-2 negative -- crypto refuses "rf:aes:x" and "a es", which are no qualified name, while accepting "rf:aes".
func TestRFC7950IdentityrefFormat(t *testing.T) {
	v := referenceValidator(t)
	leaf := v.schema.GetEntry("ref-conf").Dir["sys"].Dir["crypto"]
	assert.NoError(t, v.validateLeafValue("sys/crypto", leaf, leaf.Type, "rf:aes"))
	for _, value := range []string{"rf:aes:x", "a es"} {
		requireValidationErrorType(t, v.validateLeafValue("sys/crypto", leaf, leaf.Type, value), ErrTypeType, value)
	}
}
