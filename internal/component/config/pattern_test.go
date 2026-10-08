package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYANGPatternRestrictionsValidateParse verifies YANG pattern statements
// reject invalid values during config parsing, including list keys and
// bracket-syntax leaf-lists. A pattern the XSD translation cannot compile never
// reaches the parser: TestYANGPatternUncompilableRefusedAtSchemaBuild covers it.
//
// VALIDATES: YANG pattern constraints are enforced by the parser.
// PREVENTS: `ze config validate` accepting values outside schema patterns.
func TestYANGPatternRestrictionsValidateParse(t *testing.T) {
	schema, err := YANGSchemaWithPlugins(map[string]string{
		"ze-pattern-test-conf.yang": `
module ze-pattern-test-conf {
  namespace "urn:ze:pattern-test";
  prefix zpt;
  import ze-extensions { prefix ze; }

  container pattern-test {
	    leaf slug {
	      type string {
	        pattern '[a-z][a-z0-9-]*';
	      }
	    }

	    leaf alt {
	      type string {
	        pattern 'good|ok';
	      }
	    }

    list peer {
      key "name";
      leaf name {
        type string {
          pattern '[a-zA-Z_][a-zA-Z0-9_.\-]*';
        }
      }
      leaf description { type string; }
    }

    list inline {
      key "name";
      leaf name { type string; }
      leaf code {
        type string {
          pattern '[a-z][a-z0-9-]*';
        }
      }
      leaf text { type string; }
    }

    leaf-list tag {
      type string {
        pattern '[a-z][a-z0-9-]*';
      }
      ze:syntax "bracket";
    }
  }
}`,
	})
	require.NoError(t, err)

	_, err = NewParser(schema).Parse(`pattern-test { slug Bad; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewParser(schema).Parse(`pattern-test { alt goodx; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alt")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewParser(schema).Parse(`pattern-test { peer 1bad { description test; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewParser(schema).Parse(`pattern-test { tag [ good Bad ]; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewParser(schema).Parse(`pattern-test { inline item Bad text; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inline.code")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewParser(schema).Parse(`pattern-test { slug good-name; alt good; peer good_1 { description test; } inline item good text; tag [ good other-1 ]; }`)
	require.NoError(t, err)
}

// TestYANGPatternUncompilableRefusedAtSchemaBuild verifies a module whose
// pattern uses XSD character-class subtraction, which the XSD-to-RE2
// translation does not support, is refused when the schema is built. The
// refusal names the module, the leaf, the reason and the pattern, so the
// operator can find the statement. Method: one module with a single such leaf,
// built through YANGSchemaWithPlugins, the path the config loader takes.
//
// VALIDATES: an uncompilable pattern stops the schema build.
// PREVENTS: `ze config validate` accepting values outside schema patterns,
// through a pattern that constrains nothing.
func TestYANGPatternUncompilableRefusedAtSchemaBuild(t *testing.T) {
	_, err := YANGSchemaWithPlugins(map[string]string{
		"ze-pattern-subtraction-conf.yang": `
module ze-pattern-subtraction-conf {
  namespace "urn:ze:pattern-subtraction-test";
  prefix zpsub;

  container pattern-subtraction-test {
    leaf unsupported {
      type string {
        pattern '[a-z-[aeiou]]';
      }
    }
  }
}`,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module ze-pattern-subtraction-conf")
	assert.Contains(t, err.Error(), "leaf unsupported")
	assert.Contains(t, err.Error(), "unsupported XSD regex character-class subtraction")
	assert.Contains(t, err.Error(), "[a-z-[aeiou]]")
}

// TestYANGPatternRestrictionsValidateSetParse verifies set-format parsing uses
// the same YANG pattern restrictions as hierarchical parsing.
//
// VALIDATES: SetParser enforces YANG pattern constraints.
// PREVENTS: API/editor set-format commits bypassing parser pattern checks.
func TestYANGPatternRestrictionsValidateSetParse(t *testing.T) {
	schema, err := YANGSchemaWithPlugins(map[string]string{
		"ze-pattern-set-test-conf.yang": `
module ze-pattern-set-test-conf {
  namespace "urn:ze:pattern-set-test";
  prefix zpst;
  import ze-extensions { prefix ze; }

  container pattern-set-test {
    leaf slug {
      type string {
        pattern '[a-z][a-z0-9-]*';
      }
    }
    leaf-list tag {
      type string {
        pattern '[a-z][a-z0-9-]*';
      }
      ze:syntax "bracket";
    }
    leaf-list label {
      type string {
        pattern '[a-z][a-z0-9-]*';
      }
    }
    list peer {
      key "name";
      leaf name {
        type string {
          pattern '[a-zA-Z_][a-zA-Z0-9_.\-]*';
        }
      }
      leaf description { type string; }
    }
  }
}`,
	})
	require.NoError(t, err)

	_, err = NewSetParser(schema).Parse(`set pattern-set-test slug Bad`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewSetParser(schema).Parse(`set pattern-set-test peer 1bad description test`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewSetParser(schema).Parse(`set pattern-set-test tag [ good Bad ]`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewSetParser(schema).Parse(`set pattern-set-test label [ good Bad ]`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "label")
	assert.Contains(t, err.Error(), "does not match pattern")

	_, err = NewSetParser(schema).Parse(`set pattern-set-test slug good-name
set pattern-set-test tag [ good other-1 ]
set pattern-set-test label [ good other-1 ]
set pattern-set-test peer good_1 description test`)
	require.NoError(t, err)
}
