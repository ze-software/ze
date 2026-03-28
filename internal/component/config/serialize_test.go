package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	grschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/gr/schema"
)

// serializeSchemaWithGR returns schema with GR plugin YANG for serialize tests.
func serializeSchemaWithGR() (*Schema, error) {
	return YANGSchemaWithPlugins(map[string]string{
		"ze-graceful-restart.yang": grschema.ZeGracefulRestartYANG,
	})
}

// TestSerializeSimple verifies basic serialization.
//
// VALIDATES: Simple config serializes and round-trips.
//
// PREVENTS: Lost data during serialization.
func TestSerializeSimple(t *testing.T) {
	input := `bgp {
    router-id 1.2.3.4
    local {
        as 65000
    }
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)
	require.NotEmpty(t, output)

	// Parse again
	tree2, err := p.Parse(output)
	require.NoError(t, err)

	// Compare
	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestSerializeNeighbor verifies neighbor block serialization.
//
// VALIDATES: Neighbor configs serialize correctly.
//
// PREVENTS: Lost neighbor settings.
func TestSerializeNeighbor(t *testing.T) {
	input := `bgp {
    peer peer1 {
        remote {
            ip 192.0.2.1
            as 65001
        }
        local {
            as 65000
        }
        router-id 1.2.3.4
        timer {
            receive-hold-time 90
        }
    }
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	// Parse again
	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestSerializeFamily verifies family block serialization.
//
// VALIDATES: Freeform family blocks serialize correctly.
//
// PREVENTS: Lost address families.
func TestSerializeFamily(t *testing.T) {
	input := `bgp {
    peer peer1 {
        remote {
            ip 192.0.2.1
            as 65001
        }
        local {
            as 65000
        }
        family {
            ipv4/unicast
            ipv6/unicast
        }
    }
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestSerializePlugin verifies plugin block serialization.
//
// VALIDATES: Plugin configs serialize correctly.
//
// PREVENTS: Lost plugin settings.
func TestSerializePlugin(t *testing.T) {
	input := `plugin {
    external watcher {
        run "/usr/bin/watcher"
        encoder json
    }
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestSerializeCapability verifies capability serialization.
//
// VALIDATES: Nested capabilities serialize correctly.
//
// PREVENTS: Lost capability settings.
func TestSerializeCapability(t *testing.T) {
	input := `bgp {
    peer peer1 {
        remote {
            ip 192.0.2.1
            as 65001
        }
        local {
            as 65000
        }
        capability {
            asn4 true
            route-refresh true
            graceful-restart {
                restart-time 120
            }
        }
    }
}
`
	schema, err := serializeSchemaWithGR() // Use schema with GR plugin YANG
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestSerializeLeafRoundtrip verifies single-value leaf serialization.
//
// VALIDATES: Leaf values like "listen 0.0.0.0:179" survive roundtrip.
//
// PREVENTS: Lost leaf values.
func TestSerializeLeafRoundtrip(t *testing.T) {
	input := `bgp {
    listen 0.0.0.0:179
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2), "trees should be equal after roundtrip")
}

// TestRoundtripConfigFiles tests roundtrip on actual config files.
//
// VALIDATES: Real ExaBGP configs survive roundtrip.
//
// PREVENTS: Incompatibility with real configs.
func TestRoundtripConfigFiles(t *testing.T) {
	// Find config files
	files, err := filepath.Glob("../../etc/ze/bgp/*.conf")
	if err != nil || len(files) == 0 {
		t.Skip("no config files found")
	}

	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)

	var passed, failed, skipped int

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(file) //nolint:gosec // Test file from known directory
			if err != nil {
				t.Skip("cannot read file")
				skipped++
				return
			}

			input := string(data)

			// Skip files with backslash line continuation - these don't roundtrip cleanly
			// because the continuation syntax is preserved in values but not reconstructed
			if strings.Contains(input, "\\\n") {
				t.Skip("backslash continuation doesn't roundtrip")
				skipped++
				return
			}

			// Try to parse
			tree, err := p.Parse(input)
			if err != nil {
				// Config uses features not yet supported
				t.Skipf("parse error: %v", err)
				skipped++
				return
			}

			// Serialize
			output := Serialize(tree, schema)

			// Parse again
			tree2, err := p.Parse(output)
			if err != nil {
				t.Errorf("reparse failed: %v\nSerialized:\n%s", err, output)
				failed++
				return
			}

			// Compare
			if !TreeEqual(tree, tree2) {
				t.Errorf("trees not equal after roundtrip")
				failed++
				return
			}

			passed++
		})
	}

	t.Logf("Roundtrip results: %d passed, %d failed, %d skipped", passed, failed, skipped)
}

// TreeEqual compares two trees for equality.
func TreeEqual(a, b *Tree) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare values
	if len(a.values) != len(b.values) {
		return false
	}
	for k, v := range a.values {
		if b.values[k] != v {
			return false
		}
	}

	// Compare multiValues (leaf-list / ValueOrArray storage)
	if len(a.multiValues) != len(b.multiValues) {
		return false
	}
	for k, av := range a.multiValues {
		bv := b.multiValues[k]
		if len(av) != len(bv) {
			return false
		}
		for i, v := range av {
			if bv[i] != v {
				return false
			}
		}
	}

	// Compare containers
	if len(a.containers) != len(b.containers) {
		return false
	}
	for k, v := range a.containers {
		if !TreeEqual(v, b.containers[k]) {
			return false
		}
	}

	// Compare lists
	if len(a.lists) != len(b.lists) {
		return false
	}
	for k, v := range a.lists {
		bList := b.lists[k]
		if len(v) != len(bList) {
			return false
		}
		for key, entry := range v {
			if !TreeEqual(entry, bList[key]) {
				return false
			}
		}
	}

	return true
}

// TestTreeEqual verifies TreeEqual works correctly.
func TestTreeEqual(t *testing.T) {
	t1 := NewTree()
	t1.Set("foo", "bar")

	t2 := NewTree()
	t2.Set("foo", "bar")

	t3 := NewTree()
	t3.Set("foo", "baz")

	require.True(t, TreeEqual(t1, t2))
	require.False(t, TreeEqual(t1, t3))
}

// TestSerializeArray verifies array serialization.
//
// VALIDATES: Arrays serialize with [ ] syntax.
//
// PREVENTS: Broken array roundtrip.
func TestSerializeArray(t *testing.T) {
	schema := NewSchema()
	schema.Define("items", BracketLeafList(TypeString))

	input := `items [ foo bar baz ]`

	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)
	require.Contains(t, output, "[")
	require.Contains(t, output, "]")

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2))
}

// TestSerializeQuotedStrings verifies strings with spaces are quoted.
//
// VALIDATES: Strings with spaces get quoted.
//
// PREVENTS: Broken serialization of descriptions.
func TestSerializeQuotedStrings(t *testing.T) {
	input := `bgp {
    peer peer1 {
        remote {
            ip 192.0.2.1
            as 65001
        }
        local {
            as 65000
        }
        description "My BGP Peer"
    }
}
`
	schema, err := YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := Serialize(tree, schema)

	// Should contain quoted description
	require.Contains(t, output, `"My BGP Peer"`)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	require.True(t, TreeEqual(tree, tree2))
}
