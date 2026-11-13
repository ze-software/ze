// Design: docs/guide/config-editor.md -- fixes work from any editor position.
// Related: validator.go -- sectionErrors, validateWithYANG

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestEditorSectionRefusalFramesTheFix proves section validation holds the
// same typed command as parser validation and frames it at the real position.
func TestEditorSectionRefusalFramesTheFix(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)
	tree, err := config.NewParser(v.schema).Parse("ospf { interfaces { interface eth0 { cost 10; } } }")
	require.NoError(t, err)
	tree.GetContainer("ospf").GetContainer("interfaces").GetList("interface")["eth0"].Set("cost", "0")
	issues := v.sectionErrors(tree)
	var matched bool
	for _, issue := range issues {
		if !strings.Contains(issue.Message, "is outside the accepted range") {
			continue
		}
		matched = true
		require.NotNil(t, issue.Fix)
		assert.Equal(t, "fix: set ospf interfaces interface eth0 cost <1..65535>", lastLine(issue.Text(nil)))
		assert.Equal(t, "fix: top set ospf interfaces interface eth0 cost <1..65535>", lastLine(issue.Text([]string{"bgp"})))
	}
	assert.True(t, matched, "the section walk produced a range refusal: %v", issues)
}

const requiredHintProbe = `module ze-required-hint-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:required-hint-probe";
  prefix probe;
  import ze-extensions { prefix ze; }
  container required-probe {
    list target {
      key "name";
      ze:required "address";
      leaf name { type string; }
      leaf address { type string; }
    }
  }
}`

// TestRequiredHintFramesTheFix proves the generic ze:required warning carries
// its command as words, so a commit in another context proposes top set.
func TestRequiredHintFramesTheFix(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-required-hint-probe-conf.yang", requiredHintProbe))
	v, err := NewConfigValidator()
	require.NoError(t, err)
	content := "required-probe { target one {} }"
	tree, err := config.NewParser(v.schema).Parse(content)
	require.NoError(t, err)
	_, warnings := v.validateWithYANG(tree, content)
	require.Len(t, warnings, 1)
	issue := warnings[0]
	require.NotNil(t, issue.Fix)
	assert.Contains(t, issue.Text(nil), "(set required-probe target one address <value>)")
	assert.Contains(t, issue.Text([]string{"bgp"}), "(top set required-probe target one address <value>)")
	tree.GetContainer("required-probe").GetList("target")["one"].Set("address", "ready")
	_, warnings = v.validateWithYANG(tree, content)
	assert.Empty(t, warnings, "setting the missing field resolves the warning")
}
