package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/redistribute"
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
	_, err := LoadReactor(redistConfig(`
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

// TestUnknownRedistributeFamilyRefusesLoad is A-5: the family name is the other
// way ExtractRedistributeRules errors, and the `redistribute-source` validator
// does not cover it.
//
// VALIDATES: A-5 -- making the error fatal has to be judged on both error
// paths, not on the one the YANG validator already guards.
func TestUnknownRedistributeFamilyRefusesLoad(t *testing.T) {
	_, err := LoadReactor(redistConfig(`
redistribute {
    destination bgp {
        import connected {
            family [ ipv9/unicast ]
        }
    }
}
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ipv9/unicast")
}

// TestValidRedistributeLoads is the positive that keeps the refusal honest: a
// config naming a registered source still starts, and installs its rules.
//
// VALIDATES: R-4 -- the fatal error refuses a config that is wrong, not a
// config that works.
func TestValidRedistributeLoads(t *testing.T) {
	_, err := LoadReactor(redistConfig(`
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

	ev := redistribute.Global()
	require.NotNil(t, ev, "a config with rules installs an evaluator")
	assert.True(t, ev.HasDestination("ospf"))
}

// TestNoRedistributeInstallsAnEmptyEvaluator is the second half of the finding
// this spec carried. initRedistribute used to set NOTHING for an empty rule
// list. A reload that removed the last `redistribute` block therefore left the
// previous rules live until the daemon restarted.
//
// VALIDATES: an evaluator is always installed, and it accepts nothing when the
// config asks for nothing.
// PREVENTS: a removed `redistribute` block that keeps redistributing.
func TestNoRedistributeInstallsAnEmptyEvaluator(t *testing.T) {
	redistribute.SetGlobal(redistribute.NewEvaluator([]redistribute.ImportRule{
		{Source: "connected", Destination: "ospf"},
	}))
	require.True(t, redistribute.Global().HasDestination("ospf"), "the stale evaluator is installed")

	_, err := LoadReactor(redistConfig(""))
	require.NoError(t, err)

	ev := redistribute.Global()
	require.NotNil(t, ev)
	assert.False(t, ev.HasDestination("ospf"), "a config with no redistribute block redistributes nothing")
	assert.Empty(t, ev.Rules())
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
