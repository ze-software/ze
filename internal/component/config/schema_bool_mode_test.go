package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// capabilityModeConfig wraps one capability line in a peer that parses.
func capabilityModeConfig(line string) string {
	return `
bgp {
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            capability {
                ` + line + `
            }
        }
    }
}
`
}

// TestBooleanLeafRefusesCapabilityMode proves a boolean leaf takes no
// capability mode. Until 2026-09-14 ValidateValue accepted require and refuse
// on EVERY boolean leaf so that asn4, then declared boolean, could carry them;
// asn4 is now a capability-mode enumeration, so the two words are refused
// wherever a boolean is expected.
//
// VALIDATES: `connect require` and `accept refuse` fail at parse with
// "invalid bool", through the same YANG-derived schema the daemon loads.
// PREVENTS: an operator writing `enabled require` on any boolean leaf and
// the validator answering yes.
func TestBooleanLeafRefusesCapabilityMode(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)

	for _, value := range []string{"require", "refuse"} {
		input := `
bgp {
    peer peer1 {
        connection {
            remote {
                ip 192.0.2.1
                connect ` + value + `
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
        }
    }
}
`
		_, err := p.Parse(input)
		require.Error(t, err, "connect %s must be refused", value)
		require.Contains(t, err.Error(), "invalid bool")
		require.Contains(t, err.Error(), "expected true/false/enable/disable")
	}
}

// TestASN4IsACapabilityMode proves asn4 takes the four capability modes and
// nothing else, and that the tree stores the mode word unchanged so
// parseCapMode in the reactor reads what the operator wrote.
//
// VALIDATES: enable, disable, require and refuse each parse and are stored
// as written; true and false are refused as an invalid enum.
// PREVENTS: asn4 silently degrading to a boolean again.
func TestASN4IsACapabilityMode(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)

	for _, mode := range []string{"enable", "disable", "require", "refuse"} {
		tree, err := p.Parse(capabilityModeConfig("asn4 " + mode))
		require.NoError(t, err, "asn4 %s must parse", mode)
		peer := tree.GetContainer("bgp").GetList("peer")["peer1"]
		val, ok := peer.GetContainer("session").GetContainer("capability").Get("asn4")
		require.True(t, ok)
		require.Equal(t, mode, val)
	}

	for _, value := range []string{"true", "false"} {
		_, err := p.Parse(capabilityModeConfig("asn4 " + value))
		require.Error(t, err, "asn4 %s must be refused", value)
		require.Contains(t, err.Error(), "invalid enum")
	}
}

// TestCapabilityModeLeavesShareOneTypedef proves every mode leaf under
// capability resolves the capability-mode typedef, including the one that
// reaches it through a module import (ze-softver.yang, bgp:capability-mode).
//
// VALIDATES: require parses on each leaf, and a word outside the set is
// refused as an invalid enum on each leaf.
// PREVENTS: a sibling leaf keeping a private copy of the set that drifts.
func TestCapabilityModeLeavesShareOneTypedef(t *testing.T) {
	schema, schemaErr := YANGSchema()
	require.NoError(t, schemaErr)
	p := NewParser(schema)

	leaves := []struct {
		name string
		line func(mode string) string
	}{
		{"software-version mode", func(m string) string { return "software-version { mode " + m + "; }" }},
		{"add-path family mode", func(m string) string {
			return "add-path { family { ipv4/unicast { mode " + m + "; } } }"
		}},
		{"nexthop mode", func(m string) string { return "nexthop { ipv4/unicast ipv6 " + m + "; }" }},
	}
	for _, leaf := range leaves {
		_, err := p.Parse(capabilityModeConfig(leaf.line("require")))
		require.NoError(t, err, "%s require must parse", leaf.name)

		_, err = p.Parse(capabilityModeConfig(leaf.line("bogus")))
		require.Error(t, err, "%s bogus must be refused", leaf.name)
		require.Contains(t, err.Error(), "invalid enum", leaf.name)
	}
}
