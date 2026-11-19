// Design: docs/guide/configuration.md -- a registry mirror's path is placed before /v2/ of every request

package yang_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	containeryang "github.com/ze-software/ze/internal/plugins/container/yang"
)

// TestMirrorPathPatternRefusesAnotherHost proves the schema's own pattern on a
// registry mirror's path, the first check a configured value meets. Goal: a
// path a URL reader takes as naming another host ("//host", "/\host") is
// refused at the leaf, beside the query, fragment and space the leaf already
// refused, while the root and ordinary prefixes stay accepted. Method: load the
// module into a schema, look up the leaf, and judge each value with the
// validator the config parser applies.
func TestMirrorPathPatternRefusesAnotherHost(t *testing.T) {
	schema, err := config.YANGSchemaWithPlugins(map[string]string{
		"ze-container-conf.yang": containeryang.ZeContainerConfYANG,
	})
	if err != nil {
		t.Fatalf("load the container module: %v", err)
	}
	path := []string{"container", "registry", "mirror", "path"}
	node, err := schema.Lookup("container/registry/mirror/path")
	if err != nil {
		t.Fatalf("look up the mirror path leaf: %v", err)
	}
	leaf, isLeaf := node.(*config.LeafNode)
	if !isLeaf {
		t.Fatalf("the mirror path is a %T, want a leaf", node)
	}
	for _, value := range []string{"/", "/cache", "/cache/v1/"} {
		if err := config.ValidateLeafValue(leaf, path, value); err != nil {
			t.Errorf("the mirror path %q is refused: %v", value, err)
		}
	}
	refused := []string{"//evil.example", `/\evil.example`, `/cache\x`, "/cache?x=1", "/cache#top", "/ca che", "cache"}
	for _, value := range refused {
		if err := config.ValidateLeafValue(leaf, path, value); err == nil {
			t.Errorf("the mirror path %q is accepted", value)
		}
	}
}
