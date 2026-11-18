// Design: docs/architecture/config/yang-config-design.md -- uses grouping namespace binding
// Related: rfc7950_uses_deviation_test.go -- production loader fixture

package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoaderUsesDefersNestedAugmentNamespaceUntilInstantiation proves that
// an augment inside a grouping binds its added leaf where that grouping is
// used. Method: resolve library-local and imported wrapper copies, retaining
// an outside-grouping augment and an unaugmented copy as independent controls.
// RFC 7950 Section 7.13: "The identifiers defined in the grouping are not bound
// to a namespace until the contents of the grouping are added to the schema
// tree via a "uses" statement that does not appear inside a "grouping"
// statement, at which point they are bound to the namespace of the current module."
func TestLoaderUsesDefersNestedAugmentNamespaceUntilInstantiation(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t,
		`module uses-nested-library {
    yang-version 1.1; namespace "urn:ze:test:uses-nested-library"; prefix l;
    grouping base { container box { leaf original { type string; } } }
    grouping wrapper {
        uses base { augment box { leaf nested { type string; } } }
    }
    container library-copy { uses wrapper; }
}`,
		`module uses-nested-import {
    yang-version 1.1; namespace "urn:ze:test:uses-nested-import"; prefix u;
    import uses-nested-library { prefix lib; }
    container first {
        uses lib:wrapper { augment box { leaf local { type string; } } }
    }
    container second { uses lib:wrapper; }
    container plain { uses lib:base; }
}`)
	require.NoError(t, err)
	for _, instance := range []struct {
		module    string
		container string
		namespace string
		leaves    []string
	}{
		{"uses-nested-library", "library-copy", "urn:ze:test:uses-nested-library", []string{"original", "nested"}},
		{"uses-nested-import", "first", "urn:ze:test:uses-nested-import", []string{"original", "nested", "local"}},
		{"uses-nested-import", "second", "urn:ze:test:uses-nested-import", []string{"original", "nested"}},
		{"uses-nested-import", "plain", "urn:ze:test:uses-nested-import", []string{"original"}},
	} {
		t.Run(instance.container, func(t *testing.T) {
			module := resolved.GetEntry(instance.module)
			require.NotNil(t, module)
			container := module.Dir[instance.container]
			require.NotNil(t, container)
			box := container.Dir["box"]
			require.NotNil(t, box)
			require.Len(t, box.Dir, len(instance.leaves))
			for _, name := range instance.leaves {
				leaf := box.Dir[name]
				require.NotNil(t, leaf, name)
				namespace := leaf.Namespace()
				require.NotNil(t, namespace, name)
				assert.Equal(t, instance.namespace, namespace.Name, name)
			}
		})
	}
	consumer := resolved.GetEntry("uses-nested-import")
	require.NotNil(t, consumer)
	first := consumer.Dir["first"]
	second := consumer.Dir["second"]
	require.NotNil(t, first)
	require.NotNil(t, second)
	boxFirst := first.Dir["box"]
	boxSecond := second.Dir["box"]
	require.NotNil(t, boxFirst)
	require.NotNil(t, boxSecond)
	require.NotSame(t, boxFirst.Dir["nested"], boxSecond.Dir["nested"],
		"grouping copies must retain distinct entries")
}

// TestLoaderUsesPreservesExternalAugmentNamespace proves that deferring an
// augment inside a grouping does not erase a separate module's augmentation.
// Method: resolve an imported grouping and a third module's top-level augment,
// then compare the original and added leaves' distinct effective namespaces.
// RFC 7950 Section 7.17.2: "All data nodes defined in the "augment" statement
// are defined as XML elements in the XML namespace of the module where the
// "augment" is specified."
func TestLoaderUsesPreservesExternalAugmentNamespace(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t,
		`module uses-external-library {
    yang-version 1.1; namespace "urn:ze:test:uses-external-library"; prefix l;
    grouping base { container box { leaf original { type string; } } }
}`,
		`module uses-external-import {
    yang-version 1.1; namespace "urn:ze:test:uses-external-import"; prefix u;
    import uses-external-library { prefix lib; }
    container copy { uses lib:base; }
}`,
		`module uses-external-augment {
    yang-version 1.1; namespace "urn:ze:test:uses-external-augment"; prefix a;
    import uses-external-import { prefix user; }
    augment "/user:copy/user:box" { leaf external { type string; } }
}`)
	require.NoError(t, err)
	consumer := resolved.GetEntry("uses-external-import")
	require.NotNil(t, consumer)
	container := consumer.Dir["copy"]
	require.NotNil(t, container)
	box := container.Dir["box"]
	require.NotNil(t, box)
	require.Len(t, box.Dir, 2)
	for name, namespace := range map[string]string{
		"original": "urn:ze:test:uses-external-import",
		"external": "urn:ze:test:uses-external-augment",
	} {
		leaf := box.Dir[name]
		require.NotNil(t, leaf, name)
		actual := leaf.Namespace()
		require.NotNil(t, actual, name)
		assert.Equal(t, namespace, actual.Name, name)
	}
}
