package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// rfc7950CaseDefaultsModule holds one choice inside a container and the same
// choice inside a list entry. Case a and case c each carry a defaulted leaf
// beside an undefaulted one; case b is the choice's default case and carries
// a defaulted leaf and a non-presence container holding a defaulted leaf.
const rfc7950CaseDefaultsModule = `module m {
  namespace "urn:m"; prefix m;
  grouping kinds {
    choice kind {
      default b;
      case a { leaf a1 { type string; } leaf a2 { type uint8; default 1; } }
      case b { leaf b1 { type uint8; default 2; } container bc { leaf bd { type uint8; default 3; } } }
      case c { leaf c1 { type string; } leaf c2 { type uint8; default 4; } }
    }
  }
  container top { uses kinds; }
  container plain {
    choice pick {
      case x { leaf x1 { type string; } leaf x2 { type uint8; default 5; } }
      case y { leaf y1 { type uint8; default 6; } }
    }
  }
  list item { key name; leaf name { type string; } uses kinds; }
}`

// rfc7950CaseDefaultsNode loads rfc7950CaseDefaultsModule and converts the
// top-level node name through yangToNode, the converter the config schema uses.
func rfc7950CaseDefaultsNode(t *testing.T, name string) Node {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang", rfc7950CaseDefaultsModule))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	module := schema.GetEntry("m")
	require.NotNil(t, module)
	entry := module.Dir[name]
	require.NotNil(t, entry, "module must define %s", name)
	node := yangToNode(entry, name)
	require.NotNil(t, node, "%s must convert to a schema node", name)
	return node
}

// TestRFC7950CaseDefaultsFollowTheCaseInEffect runs ApplyDefaults over a
// container whose choice has a default case, once with no case holding data,
// once with case a holding data, and once with case c holding data, and over a
// list entry carrying the same choice. Each run reads which case's defaults
// were filled.
//
// VALIDATES: the case in effect, or the default case, receives its defaults.
// PREVENTS: a choice's active case losing its defaults to the case filter.
//
// RFC requirement: RFC7950-7.6.1-2 positive — the case holding data receives its own leaf defaults (a2 1 when a1 is set, c2 4 when c1 is set, also inside a list entry), and with no case holding data the default case b receives b1 2 and its non-presence container bc with bd 3.
func TestRFC7950CaseDefaultsFollowTheCaseInEffect(t *testing.T) {
	t.Run("no case holds data, default case used", func(t *testing.T) {
		m := map[string]any{}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.Equal(t, "2", m["b1"], "the default case's leaf default is in use")
		bc, ok := m["bc"].(map[string]any)
		require.True(t, ok, "the default case's non-presence container is created for its default")
		assert.Equal(t, "3", bc["bd"])
	})

	t.Run("case a holds data", func(t *testing.T) {
		m := map[string]any{"a1": "x"}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.Equal(t, "1", m["a2"], "the case holding data receives its default")
	})

	t.Run("case c holds data", func(t *testing.T) {
		m := map[string]any{"c1": "y"}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.Equal(t, "4", m["c2"], "the case holding data receives its default")
	})

	t.Run("list entry, case a holds data", func(t *testing.T) {
		m := map[string]any{"item": map[string]any{"k": map[string]any{"a1": "x"}}}
		ApplyDefaults(m, Container(Field("item", rfc7950CaseDefaultsNode(t, "item"))))
		entry, ok := m["item"].(map[string]any)["k"].(map[string]any)
		require.True(t, ok, "list entry must survive default application")
		assert.Equal(t, "1", entry["a2"], "the case holding data receives its default inside a list entry")
	})
}

// TestRFC7950CaseDefaultsIgnoredOutsideTheCaseInEffect runs ApplyDefaults
// over the same schema and reads that no default of a case that is not in
// effect was filled: the other cases when one case holds data, every case but
// the default one when none does, every case when two cases hold data, and
// every case of a choice with no default case when none holds data. A
// container merged from a second module keeps the same behavior.
//
// VALIDATES: a case not in effect receives no default.
// PREVENTS: every case of a choice filled at once, which selects them all.
//
// RFC requirement: RFC7950-7.6.1-2 negative — ApplyDefaults fills no default of a case that is not in effect: with a1 set neither b1, bc nor c2 appears; with no data a2 and c2 do not appear; with a1 and c1 both set no default appears; a choice with no default case and no data gets neither x2 nor y1; a list entry with a1 set gets no b1; the same holds after the container is merged into another.
func TestRFC7950CaseDefaultsIgnoredOutsideTheCaseInEffect(t *testing.T) {
	t.Run("case a holds data", func(t *testing.T) {
		m := map[string]any{"a1": "x"}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.Equal(t, map[string]any{"a1": "x", "a2": "1"}, m, "only case a's default is in use")
	})

	t.Run("no case holds data", func(t *testing.T) {
		m := map[string]any{}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.NotContains(t, m, "a2", "a case that is not the default case gets no default")
		assert.NotContains(t, m, "c2", "a case that is not the default case gets no default")
	})

	t.Run("two cases hold data", func(t *testing.T) {
		m := map[string]any{"a1": "x", "c1": "y"}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "top"))
		assert.Equal(t, map[string]any{"a1": "x", "c1": "y"}, m, "no case is in effect while another case holds data")
	})

	t.Run("no default case, no data", func(t *testing.T) {
		m := map[string]any{}
		ApplyDefaults(m, rfc7950CaseDefaultsNode(t, "plain"))
		assert.Empty(t, m, "a choice with no default case and no data uses no default")
	})

	t.Run("list entry, case a holds data", func(t *testing.T) {
		m := map[string]any{"item": map[string]any{"k": map[string]any{"a1": "x"}}}
		ApplyDefaults(m, Container(Field("item", rfc7950CaseDefaultsNode(t, "item"))))
		entry, ok := m["item"].(map[string]any)["k"].(map[string]any)
		require.True(t, ok, "list entry must survive default application")
		assert.NotContains(t, entry, "b1", "a list entry's other case gets no default")
		assert.NotContains(t, entry, "c2", "a list entry's other case gets no default")
	})

	t.Run("merged container", func(t *testing.T) {
		top, ok := rfc7950CaseDefaultsNode(t, "top").(*ContainerNode)
		require.True(t, ok)
		merged := Container()
		mergeContainer(merged, top)
		m := map[string]any{"a1": "x"}
		ApplyDefaults(m, merged)
		assert.Equal(t, map[string]any{"a1": "x", "a2": "1"}, m, "a merged child keeps its case")
	})
}
