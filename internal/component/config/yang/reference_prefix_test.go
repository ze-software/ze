// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: reference_path.go — checkPathPrefix
// Related: loader_reference.go — representsConfig

package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLeafrefPathPrefixChecked loads leafref paths whose node names carry a
// prefix: one this module does not declare, and one naming an imported module
// that does not hold the node, are refused at a step, at a key predicate and
// at a current() expression; the module's own prefix loads.
//
// VALIDATES: checkPathPrefix, through Loader.Resolve (RFC 7950 Sections 6.4
// and 6.4.1: every prefix is in the XPath context, and names the namespace of
// the node it qualifies).
// PREVENTS: a path naming "bogus:l" resolving to l by its local name alone.
func TestLeafrefPathPrefixChecked(t *testing.T) {
	const other = `module pf-other { namespace "urn:ze:test:pfo"; prefix po; leaf z { type string; } }`
	const head = `module pf-conf { namespace "urn:ze:test:pf"; prefix pf; import pf-other { prefix po; }
        list l { key k; leaf k { type string; } leaf v { type string; } } leaf a { type string; } `
	for name, body := range map[string]string{
		"undeclared step":      `leaf b { type leafref { path "../bogus:l/k"; } }`,
		"other module step":    `leaf b { type leafref { path "../po:l/k"; } }`,
		"other module key":     `leaf b { type leafref { path "../l[po:k = current()/../a]/v"; } }`,
		"undeclared current()": `leaf b { type leafref { path "../l[k = current()/../bogus:a]/v"; } }`,
		"undeclared key":       `leaf b { type leafref { path "../l[bogus:k = current()/../a]/v"; } }`,
		"other module current": `leaf b { type leafref { path "../l[k = current()/../po:a]/v"; } }`,
	} {
		err := loadModuleTexts(t, other, head+body+`}`)
		require.Error(t, err, name)
		assert.ErrorIs(t, err, ErrLeafrefPath, name)
		assert.Contains(t, err.Error(), "prefix", name)
	}
	require.NoError(t, loadModuleTexts(t, other, head+`leaf b { type leafref { path "../pf:l[pf:k = current()/../pf:a]/pf:v"; } } leaf c { type leafref { path "/pf:l/pf:k"; } }}`))
}

// TestLeafrefNonConfigNodeReferringToState loads leafrefs to a config false
// leaf from an rpc input and from a notification, which are no configuration
// data, and refuses the same leafref on a config leaf.
//
// VALIDATES: representsConfig in checkLeafrefSchema (RFC 7950 Section 9.9:
// the config-to-config rule binds a referring node that "represents
// configuration data").
// PREVENTS: an rpc input naming an operational entry being refused at load.
func TestLeafrefNonConfigNodeReferringToState(t *testing.T) {
	const head = `module rs-conf { namespace "urn:ze:test:rs"; prefix rs; leaf s { config false; type string; } `
	require.NoError(t, loadModuleTexts(t, head+`rpc r { input { leaf b { type leafref { path "/rs:s"; } } } }
        notification n { leaf c { type leafref { path "/rs:s"; } } } }`))
	err := loadModuleTexts(t, head+`leaf b { type leafref { path "/rs:s"; } } }`)
	require.ErrorIs(t, err, ErrLeafrefPath)
	assert.Contains(t, err.Error(), "is state")
}

// TestLeafrefPathAcrossModules loads leafref paths that leave the module the
// referring leaf is in: into an imported module's top level, through an
// augment, past a top-level node of the same name in the leaf's own module,
// and from a typedef another module defines. Each loads. A prefix the
// defining module does not declare is refused even where the using module
// declares it, and every prefix mismatch names the fix that applies.
//
// VALIDATES: leafrefTarget, topLevelParents and checkPathPrefix, through
// Loader.Resolve (RFC 7950 Section 6.4.1: the root node has the top-level
// data nodes of all modules as children; prefixes resolve in the module
// where the XPath expression is specified; inside a grouping an unprefixed
// name takes the namespace of where the grouping is used).
// PREVENTS: a valid path into another module refused at load, and a prefix
// resolved in the module that uses a typedef rather than the one defining it.
func TestLeafrefPathAcrossModules(t *testing.T) {
	const pt = `module pt { namespace "urn:ze:test:t"; prefix t; list l { key k; leaf k { type string; } }
        typedef ref { type leafref { path "/t:l/t:k"; } } }`
	const pa = `module pa { namespace "urn:ze:test:a"; prefix a; container c { leaf x { type string; } } }`
	const pu = `module pu { namespace "urn:ze:test:u"; prefix u; container c { leaf z { type string; } } }`
	valid := map[string][]string{
		"imported top level": {pt, `module pu { namespace "urn:ze:test:u"; prefix u; import pt { prefix t; }
            leaf r { type leafref { path "/t:l/t:k"; } } }`},
		"typedef imported as another prefix": {pt, `module pu { namespace "urn:ze:test:u"; prefix u; import pt { prefix x; }
            leaf r { type x:ref; } }`},
		"typedef imported as its own prefix": {pt, `module pu { namespace "urn:ze:test:u"; prefix u; import pt { prefix t; }
            leaf r { type t:ref; } }`},
		"augmented node": {pa, `module pb { namespace "urn:ze:test:b"; prefix b; import pa { prefix a; }
            augment "/a:c" { leaf y { type string; } } leaf r { type leafref { path "/a:c/b:y"; } } }`},
		"same top-level name": {
			`module pa { namespace "urn:ze:test:a"; prefix a; container top { leaf x { type string; } } }`,
			`module pb { namespace "urn:ze:test:b"; prefix b; import pa { prefix a; } container top { leaf z { type string; } }
            leaf r { type leafref { path "/a:top/a:x"; } } }`},
		"typedef prefix resolved where defined": {pu,
			`module pt { namespace "urn:ze:test:t"; prefix t; import pu { prefix u; } augment "/u:c" { leaf k { type string; } }
            typedef ref { type leafref { path "../t:k"; } } }`,
			`module pv { namespace "urn:ze:test:v"; prefix v; import pu { prefix u; } import pt { prefix t2; }
            augment "/u:c" { leaf r { type t2:ref; } } }`},
	}
	for name, texts := range valid {
		assert.NoError(t, loadModuleTexts(t, texts...), name)
	}
	const pc = `module pc { namespace "urn:ze:test:c"; prefix c; import pa { prefix a; } augment "/a:c" { leaf w { type string; } } }`
	invalid := map[string]struct {
		texts []string
		want  string
	}{
		"prefix declared only where the typedef is used": {[]string{pu,
			`module pt { namespace "urn:ze:test:t"; prefix t; import pu { prefix u; } augment "/u:c" { leaf k { type string; } }
            typedef ref { type leafref { path "../x:k"; } } }`,
			`module pv { namespace "urn:ze:test:v"; prefix v; import pu { prefix u; } import pt { prefix x; }
            augment "/u:c" { leaf r { type x:ref; } } }`},
			`prefix "x" of "x:k" is neither the prefix of module pt, where the path is written, nor one it imports: write "t:k"`},
		"grouping prefix names the defining module": {[]string{
			`module pg { namespace "urn:ze:test:g"; prefix g; grouping gr { list l { key k; leaf k { type string; } }
            leaf r { type leafref { path "../g:l/g:k"; } } } }`,
			`module pu { namespace "urn:ze:test:u"; prefix u; import pg { prefix g; } container c { uses g:gr; } }`},
			`write "l" with no prefix`},
		"own module node under an imported prefix": {[]string{pa,
			`module pb { namespace "urn:ze:test:b"; prefix b; import pa { prefix a; }
            augment "/a:c" { leaf y { type string; } } leaf r { type leafref { path "/a:c/a:y"; } } }`},
			`write "b:y"`},
		"imported module node under another prefix": {[]string{pa, pc,
			`module pb { namespace "urn:ze:test:b"; prefix b; import pa { prefix a; } import pc { prefix cc; }
            leaf r { type leafref { path "/a:c/a:w"; } } }`},
			`write "cc:w"`},
		"node of a module not imported": {[]string{pa, pc,
			`module pb { namespace "urn:ze:test:b"; prefix b; import pa { prefix a; }
            leaf r { type leafref { path "/a:c/a:w"; } } }`},
			`add "import pc { prefix c; }" to module pb and write "c:w"`},
	}
	for name, tc := range invalid {
		err := loadModuleTexts(t, tc.texts...)
		if !assert.Error(t, err, name) {
			continue
		}
		assert.ErrorIs(t, err, ErrLeafrefPath, name)
		assert.Contains(t, err.Error(), tc.want, name)
	}
}
