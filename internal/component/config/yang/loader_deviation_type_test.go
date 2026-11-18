// Design: docs/architecture/config/yang-config-design.md -- schema loading
// Related: rfc7950_uses_deviation_test.go -- singleton deviation controls

package yang

import (
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoaderDeviateAddCannotReplaceType proves that add cannot silently change
// a leaf's required type. Method: load the journal's add-type reproducer and
// compare it with the permitted replace form in a fresh production loader.
// RFC 7950 Section 7.20.3.2: "If a property can only appear once, the property
// MUST NOT exist in the target node."
func TestLoaderDeviateAddCannotReplaceType(t *testing.T) {
	const head = `module singleton-type {
    yang-version 1.1; namespace "urn:ze:test:singleton-type"; prefix d;
    leaf target { type string; }
`
	resolved, err := resolveUsesDeviationTexts(t, head+`
    deviation "/d:target" { deviate add { type uint8; } }
}`)
	require.Error(t, err)
	assert.Nil(t, resolved)
	assert.Contains(t, err.Error(), "type")
	resolved, err = resolveUsesDeviationTexts(t, head+`
    deviation "/d:target" { deviate replace { type uint8; } }
}`)
	require.NoError(t, err)
	assert.Equal(t, goyang.Yuint8, resolved.GetEntry("singleton-type").Dir["target"].Type.Kind)
}
