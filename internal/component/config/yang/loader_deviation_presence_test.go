// Design: docs/architecture/config/yang-config-design.md -- schema loading
// Related: rfc7950_uses_deviation_test.go -- explicit singleton presence

package yang

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoaderDeviateAddCannotRepeatAnAddedSingleton proves that a property
// added by an earlier deviation is present for the next add. Method: add each
// singleton twice to a target that originally omitted it, including empty
// units and default-valued bounds whose scalar value does not prove presence.
// RFC 7950 Section 7.20.3.2: "If a property can only appear once, the property
// MUST NOT exist in the target node."
func TestLoaderDeviateAddCannotRepeatAnAddedSingleton(t *testing.T) {
	for _, c := range deviationSingletonCases() {
		t.Run(c.name, func(t *testing.T) {
			text := fmt.Sprintf(`module singleton-repeat {
    yang-version 1.1; namespace "urn:ze:test:singleton-repeat"; prefix d;
    %s target { type string; }
    deviation "/d:target" {
        deviate add { %s %s; }
        deviate add { %s %s; }
    }
}`, c.node, c.property, c.original, c.property, c.added)
			resolved, err := resolveUsesDeviationTexts(t, text)
			require.Error(t, err)
			assert.Nil(t, resolved)
			assert.Contains(t, err.Error(), c.property)
		})
	}
}
