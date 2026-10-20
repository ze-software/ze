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
