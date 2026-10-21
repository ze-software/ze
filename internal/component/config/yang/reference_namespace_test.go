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
// module and inside a grouping used in another module.
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
	}
	for name, texts := range valid {
		require.NoError(t, loadModuleTexts(t, texts...), name)
	}
}

// TestLeafrefUnprefixedRootInAugmentingModule loads leafrefs on a leaf that
// module pu augments into module pa, whose unprefixed first step names a
// top-level node of pu: from an absolute path, from a relative path that
// climbs to the root, from a current() expression that climbs to the root,
// and from a grouping pu uses inside its augment. Each loads, because the
// augmented leaf is in pu's namespace. An unprefixed first step naming pa's
// own top-level node is refused naming pa's prefix.
//
// VALIDATES: topLevelParents looking the unprefixed first step up in the
// module of the referring node's namespace, through Loader.Resolve (RFC 7950
// Section 6.4.1: "Names without a namespace prefix belong to the same
// namespace as the identifier of the current node."; Section 4.2.8: "If a
// module augments another module, the XML elements that are added to the
// encoding are in the namespace of the augmenting module.").
// PREVENTS: the first step looked up in the augment target's module, which
// refuses a valid path as naming no node and would accept "/c/x" into pa.
func TestLeafrefUnprefixedRootInAugmentingModule(t *testing.T) {
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a; container c { leaf x { type string; } } }`
	pu := func(leaf string) string {
		return `module pu { namespace "urn:ze:test:u"; prefix u; import pa { prefix a; }
        container top2 { leaf z { type string; } list l { key k; leaf k { type string; } } }
        augment "/a:c" { ` + leaf + ` } }`
	}
	for name, path := range map[string]string{
		"absolute":  "/top2/z",
		"climbing":  "../../top2/z",
		"current()": "/u:top2/u:l[u:k = current()/../../top2/z]/u:k",
	} {
		require.NoError(t, loadModuleTexts(t, pa, pu(`leaf r { type leafref { path "`+path+`"; } }`)), name)
	}
	const pg = `module pg { namespace "urn:ze:test:g"; prefix g; grouping gr { leaf r { type leafref { path "/top2/z"; } } } }`
	require.NoError(t, loadModuleTexts(t, pa, pg, `module pu { namespace "urn:ze:test:u"; prefix u;
        import pa { prefix a; } import pg { prefix g; }
        container top2 { leaf z { type string; } } augment "/a:c" { uses g:gr; } }`), "grouping in augment")

	err := loadModuleTexts(t, pa, pu(`leaf r { type leafref { path "/c/x"; } }`))
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `names no node "c"`)
	assert.Contains(t, err.Error(), `write "/a:c/a:x"`)
}

// TestLeafrefUnprefixedNoMergedSectionException loads leafrefs whose
// unprefixed names reach into a top-level section of another `-conf` module,
// one that Ze merges into the same config data node as the referring module's
// section of that name. Ze keeps no exception to the namespace rule for them:
// each is refused, naming the prefixed path to write and the import to add
// where the module lacks it, and the prefixed paths load.
//
// VALIDATES: leafrefTarget's refusal of a step no node answers, and
// rootPathFix, through Loader.Resolve (RFC 7950 Section 6.4.1: "Names without
// a namespace prefix belong to the same namespace as the identifier of the
// current node.").
// PREVENTS: an unprefixed name resolved in another module because Ze merges
// sections in its config text, a departure from the RFC with no deviation.
func TestLeafrefUnprefixedNoMergedSectionException(t *testing.T) {
	const zb = `module ze-b-conf { namespace "urn:ze:test:zb"; prefix zb;
        container sec { leaf y { type string; } } container bsec { leaf y { type string; } }
        list bl { key k; leaf k { type string; } } }`
	za := func(body string) string {
		return `module ze-a-conf { namespace "urn:ze:test:za"; prefix za; import ze-b-conf { prefix zb; }
        container sec { leaf own { type string; } } leaf s { type string; } ` + body + ` }`
	}
	refused := map[string]struct{ body, fix string }{
		"other section":     {`leaf r { type leafref { path "/bsec/y"; } }`, `write "/zb:bsec/zb:y"`},
		"shared section":    {`container q { leaf r { type leafref { path "/sec/y"; } } }`, `write "/zb:sec/zb:y"`},
		"climbing":          {`container q { leaf r { type leafref { path "../../bsec/y"; } } }`, `write "/zb:bsec/zb:y"`},
		"list with key":     {`leaf r { type leafref { path "/bl[k = current()/../s]/k"; } }`, `names no node "bl"`},
		"current() to root": {`leaf r { type leafref { path "/zb:bl[zb:k = current()/../bsec/y]/zb:k"; } }`, `current() expression names no node "bsec"`},
	}
	for name, tc := range refused {
		err := loadModuleTexts(t, za(tc.body), zb)
		require.ErrorIs(t, err, ErrLeafrefPath, name)
		assert.Contains(t, err.Error(), tc.fix, name)
		assert.NotContains(t, err.Error(), "merge", name)
	}
	for name, body := range map[string]string{
		"prefixed other section":  `leaf r { type leafref { path "/zb:bsec/zb:y"; } }`,
		"prefixed shared section": `leaf r { type leafref { path "/zb:sec/zb:y"; } }`,
		"own section":             `leaf r { type leafref { path "/sec/own"; } }`,
	} {
		require.NoError(t, loadModuleTexts(t, za(body), zb), name)
	}

	// A module that does not import the section's module is told the import.
	err := loadModuleTexts(t, `module pu { namespace "urn:ze:test:u"; prefix u; leaf r { type leafref { path "/bsec/y"; } } }`, zb)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `add "import ze-b-conf { prefix zb; }" to module pu and write "/zb:bsec/zb:y"`)
}

// TestLeafrefPrefixedIntoOtherConfSection runs the whole-tree reference walk
// over a leafref of one `-conf` module whose prefixed path names a section of
// another, which Ze's config data merges by local name. The configured value
// is found, and an absent one is refused.
//
// VALIDATES: childEntries answering every module's node of a section at the
// root, through ValidateReferences (RFC 7950 Section 9.9.3: "If
// "require-instance" is "true", it means that the instance being referred to
// MUST exist for the data to be valid.").
// PREVENTS: the walk reading the section through the referring module's node
// alone, which holds no y, and refusing every value.
func TestLeafrefPrefixedIntoOtherConfSection(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.AddModuleFromText("ze-b-conf.yang", `module ze-b-conf { namespace "urn:ze:test:zb"; prefix zb;
        container sec { leaf y { type string; } } }`))
	require.NoError(t, loader.AddModuleFromText("ze-a-conf.yang", `module ze-a-conf { namespace "urn:ze:test:za"; prefix za;
        import ze-b-conf { prefix zb; } container sec { leaf r { type leafref { path "/zb:sec/zb:y"; } } } }`))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	v := NewValidator(resolved)
	assert.Empty(t, v.ValidateReferences(map[string]any{"sec": map[string]any{"y": "v1", "r": "v1"}}))
	errs := v.ValidateReferences(map[string]any{"sec": map[string]any{"y": "v1", "r": "v2"}})
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, `no y "v2" exists`)
}

// TestLeafrefFromSubmoduleDeviation loads leafrefs that a "deviate replace"
// in a submodule gives a leaf of another module. The path's prefixes resolve
// in the submodule, whose own prefix is the one its belongs-to declares, so
// that prefix and an imported one load, an unprefixed relative path loads,
// and the including module's prefix, which the submodule does not declare, is
// refused naming the submodule's prefix.
//
// VALIDATES: deviationStatement's search of submodules, then checkPathPrefix,
// through Loader.Resolve (RFC 7950 Section 7.20.3.2: "The argument "replace"
// replaces properties of the target node."; Section 7.2.2: the belongs-to
// prefix "is used to refer to definitions in the module").
// PREVENTS: a deviation written in a submodule refused as a Ze defect, or its
// prefixes resolved against the including module.
func TestLeafrefFromSubmoduleDeviation(t *testing.T) {
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a;
        container c { leaf x { type string; } leaf w { type string; } } }`
	const pm = `module pm { yang-version 1.1; namespace "urn:ze:test:m"; prefix m; include ps; leaf y { type string; } }`
	ps := func(path string) string {
		return `submodule ps { yang-version 1.1; belongs-to pm { prefix mm; } import pa { prefix a; }
        deviation "/a:c/a:w" { deviate replace { type leafref { path "` + path + `"; } } } }`
	}
	for _, path := range []string{"/a:c/a:x", "/mm:y", "../x"} {
		require.NoError(t, loadModuleTexts(t, pa, pm, ps(path)), path)
	}
	err := loadModuleTexts(t, pa, pm, ps("/m:y"))
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `prefix "m" of "m:y" is neither the prefix of module pm`)
	assert.Contains(t, err.Error(), `write "mm:y"`)
	assert.NotContains(t, err.Error(), "Ze found no type statement")
}

// TestLeafrefDeviateReplaceLeafList loads a leafref that a "deviate replace"
// gives a leaf-list of another module. A path to an existing leaf loads, one
// naming no node is refused, and one with a prefix naming the wrong module
// is refused naming the right prefix.
//
// VALIDATES: pathStatement reading a leaf-list's type and the deviate's,
// then leafrefTarget, through Loader.Resolve (RFC 7950 Section 7.20.3.2:
// "The argument "replace" replaces properties of the target node.").
// PREVENTS: a leaf-list's deviated type escaping the path checks.
func TestLeafrefDeviateReplaceLeafList(t *testing.T) {
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a;
        container c { leaf x { type string; } leaf-list ww { type string; } } }`
	pd := func(path string) string {
		return `module pd { namespace "urn:ze:test:d"; prefix d; import pa { prefix a; }
        deviation "/a:c/a:ww" { deviate replace { type leafref { path "` + path + `"; } } } }`
	}
	require.NoError(t, loadModuleTexts(t, pa, pd("/a:c/a:x")))

	err := loadModuleTexts(t, pa, pd("/a:c/a:nope"))
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `/pa/c/ww: leafref path "/a:c/a:nope" names no node "a:nope"`)

	err = loadModuleTexts(t, pa, pd("/d:c/a:x"))
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), `prefix "d" names module pd, but "c" belongs to module pa: write "a:c"`)
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
