package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// redistConfig wraps a `redistribute` block in the smallest config that reaches
// the reactor builder.
func redistConfig(block string) string {
	return `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}
` + block
}

// TestUnknownRedistributeSourceRefusesLoad is AC-5: a source no component
// registered stops the load, and the error names what the operator typed.
//
// VALIDATES: AC-5, and user story 4.
// PREVENTS: the 2026-09-04 behavior, where initRedistribute warned once and
// returned, leaving the global evaluator nil. That disabled EVERY rule in the
// file, so one mistyped word silently stopped redistribution the operator had
// configured correctly elsewhere.
func TestUnknownRedistributeSourceRefusesLoad(t *testing.T) {
	_, err := loadReactor(redistConfig(`
redistribute {
    destination bgp {
        import rip
    }
}
`))
	require.Error(t, err, "a source nothing registers must stop the load")
	assert.Contains(t, err.Error(), `"rip"`, "the error names the token the operator typed")
	assert.Contains(t, err.Error(), `"bgp"`, "and the destination it sits under")
}

// TestUnknownRedistributeFamilyRefusesLoad is A-5: an import naming filters
// with no final is the other way ExtractRedistributeRules errors, and the
// `redistribute-source` validator does not cover it. (The family leaf it once
// tested is gone: a family is now a match filter's condition.)
//
// VALIDATES: A-5 -- making the error fatal has to be judged on both error
// paths, not on the one the YANG validator already guards.
func TestUnknownRedistributeFamilyRefusesLoad(t *testing.T) {
	_, err := loadReactor(redistConfig(`
redistribute {
    destination bgp {
        import connected {
            filters [ V4 ]
        }
    }
}
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no final")
}

// TestValidRedistributeLoads is the positive that keeps the refusal honest: a
// config naming a registered source still starts.
//
// VALIDATES: R-4 -- the fatal error refuses a config that is wrong, not a
// config that works. Installing the rules is the redistribute orchestrator's,
// which owns the `redistribute` root (TestConfigureInstallsTheRules,
// internal/plugins/redistribute/config_test.go): the reactor builder does not
// run on a router with no bgp block, so it cannot be where the rules live.
func TestValidRedistributeLoads(t *testing.T) {
	_, err := loadReactor(redistConfig(`
ospf {
    router-id 10.0.0.1
}

redistribute {
    destination ospf {
        import bgp
    }
}
`))
	require.NoError(t, err)
}

// TestEmptyDestinationRefusesLoad holds a destination that imports nothing to
// the same rule as an unknown source: it is named rather than dropped.
//
// VALIDATES: the "destination imports nothing" refusal.
// PREVENTS: a block an operator wrote and a block that rejects every route
// producing the same silence (ai/rules/principles.md).
func TestEmptyDestinationRefusesLoad(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(redistConfig(`
redistribute {
    destination ospf {
    }
}
`), nil)
	require.NoError(t, err, "the shape is valid YANG; the refusal is the loader's")

	_, err = config.ExtractRedistributeRules(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"ospf"`)
	assert.Contains(t, err.Error(), "imports nothing")
}
