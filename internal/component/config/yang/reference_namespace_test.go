// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: reference_path.go — pathStatement, deviationStatement, checkStepNamespace
// Related: reference_prefix_test.go — the prefix checks these extend

package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLeafrefFromDeviateReplace loads a leafref that a "deviate replace"
// gives a leaf of another module. Its prefixes resolve in the deviating
// module, so a prefix that module imports loads and one it does not declare
// is refused naming the fix, and no refusal calls the module a Ze defect.
//
// VALIDATES: pathStatement through deviationStatement, then checkPathPrefix,
// through Loader.Resolve (RFC 7950 Section 7.20.3.2: "The argument "replace"
// replaces properties of the target node."; Section 6.4.1: prefixes come from
// the module where the XPath expression is specified).
// PREVENTS: a valid deviation refused as "a Ze defect" because only the
// leaf's own type statement was searched for the path.
func TestLeafrefFromDeviateReplace(t *testing.T) {
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a;
        container c { leaf x { type string; } leaf w { type string; } } }`
	const head = `module pd { namespace "urn:ze:test:d"; prefix d; import pa { prefix p; }
        deviation "/p:c/p:w" { deviate replace { type `
	require.NoError(t, loadModuleTexts(t, pa, head+`leafref { path "/p:c/p:x"; } } } }`))
	require.NoError(t, loadModuleTexts(t, pa, head+`leafref { path "../p:x"; } } } }`))

	// The prefix of pa, "a", is not declared in pd, which imports pa as "p".
	err := loadModuleTexts(t, pa, head+`leafref { path "/a:c/a:x"; } } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `prefix "a" of "a:c" is neither the prefix of module pd`)
	assert.Contains(t, err.Error(), `write "p:c"`)
	assert.NotContains(t, err.Error(), "Ze defect")

	// A leafref replaced in as a union member resolves in the deviating module too.
	require.NoError(t, loadModuleTexts(t, pa, head+`union { type int8; type leafref { path "/p:c/p:x"; } } } } }`))
}

// TestLeafrefUnprefixedStepNamespace loads leafref paths whose unprefixed
// names stand for a node of another module: at a step, at a key predicate and
// at a current() expression, each written in a module that augments the
// node's module. Each is refused naming the prefix to write. The same paths
// with the prefixes load, as do unprefixed names in the referring leaf's own
// module, inside a grouping used in another module, and into a top-level
// section a `-conf` module contributes to Ze's merged config.
//
// VALIDATES: checkStepNamespace in leafrefTarget, checkPathKeys and
// keyExprTarget, through Loader.Resolve (RFC 7950 Section 6.4.1: "Names
// without a namespace prefix belong to the same namespace as the identifier
// of the current node.").
// PREVENTS: "../y" resolving to another module's y by its local name alone.
func TestLeafrefUnprefixedStepNamespace(t *testing.T) {
	const pc = `module pc { namespace "urn:ze:test:c"; prefix c;
        container top { leaf x { type string; } leaf s { type string; }
        list l { key k; leaf k { type string; } leaf v { type string; } } } }`
	const pd = `module pd { namespace "urn:ze:test:d"; prefix d; import pc { prefix c; }
        augment "/c:top" { leaf y { type string; } } }`
	pe := func(path string) string {
		return `module pe { namespace "urn:ze:test:e"; prefix e; import pc { prefix c; } import pd { prefix dd; }
        augment "/c:top" { leaf r { type leafref { path "` + path + `"; } } } }`
	}
	refused := map[string]struct{ path, fix string }{
		"step":      {"../y", `write "dd:y"`},
		"own step":  {"../x", `write "c:x"`},
		"key":       {"../c:l[k = current()/../c:s]/c:v", `write "c:k"`},
		"current()": {"../c:l[c:k = current()/../s]/c:v", `write "c:s"`},
		"last step": {"../c:l/v", `write "c:v"`},
	}
	for name, tc := range refused {
		err := loadModuleTexts(t, pc, pd, pe(tc.path))
		require.ErrorIs(t, err, ErrLeafrefPath, name)
		assert.Contains(t, err.Error(), "has no prefix, so it names module pe", name)
		assert.Contains(t, err.Error(), tc.fix, name)
	}
	for name, path := range map[string]string{
		"prefixed step": "../dd:y",
		"prefixed key":  "../c:l[c:k = current()/../c:s]/c:v",
	} {
		require.NoError(t, loadModuleTexts(t, pc, pd, pe(path)), name)
	}

	valid := map[string][]string{
		"own module": {`module po { namespace "urn:ze:test:o"; prefix o;
            container q { list l { key k; leaf k { type string; } } leaf r { type leafref { path "../l/k"; } }
            leaf s { type leafref { path "/q/l[k = current()/../r]/k"; } } } }`},
		"grouping used elsewhere": {`module pg { namespace "urn:ze:test:g"; prefix g;
            grouping gr { list l { key k; leaf k { type string; } } leaf r { type leafref { path "../l/k"; } } } }`,
			`module pu { namespace "urn:ze:test:u"; prefix u; import pg { prefix g; } container c { uses g:gr; } }`},
		"merged conf section": {`module ze-x-conf { namespace "urn:ze:test:x"; prefix x; container c { leaf x { type string; } } }`,
			`module pu { namespace "urn:ze:test:u"; prefix u; leaf r { type leafref { path "/c/x"; } } }`},
	}
	for name, texts := range valid {
		require.NoError(t, loadModuleTexts(t, texts...), name)
	}
}

// TestLeafrefPrefixRefusedAtTopLevel loads absolute leafref paths whose first
// step carries a prefix that does not lead to the node: one the module the
// path is written in does not declare, and one naming a module whose top
// level does not hold it. Each is refused for its prefix, naming the prefix to
// write, and not as a path naming no node.
//
// VALIDATES: topLevelParents' lookup among every loaded module, then
// checkPathPrefix, through Loader.Resolve (RFC 7950 Section 6.4: "all
// prefixes used MUST be present in the XPath context").
// PREVENTS: a wrong prefix reported as "names no node", which hides the fix.
func TestLeafrefPrefixRefusedAtTopLevel(t *testing.T) {
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a; leaf x { type string; } }`
	const pb = `module pb { namespace "urn:ze:test:b"; prefix b; leaf y { type string; } }`
	err := loadModuleTexts(t, pa, pb, `module pu { namespace "urn:ze:test:u"; prefix u;
        import pa { prefix p; } import pb { prefix q; } leaf r { type leafref { path "/p:y"; } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `prefix "p" names module pa, but "y" belongs to module pb: write "q:y"`)

	// A typedef of pt uses a prefix pt does not declare, reached from pu through pm.
	err = loadModuleTexts(t,
		`module pt { namespace "urn:ze:test:t"; prefix t; leaf a { type string; } typedef ref { type leafref { path "/z:a"; } } }`,
		`module pm { namespace "urn:ze:test:m"; prefix m; import pt { prefix p; } typedef ref2 { type p:ref; } }`,
		`module pu { namespace "urn:ze:test:u"; prefix u; import pm { prefix z; } leaf r { type z:ref2; } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `prefix "z" of "z:a" is neither the prefix of module pt`)
	assert.Contains(t, err.Error(), `write "t:a"`)
}
