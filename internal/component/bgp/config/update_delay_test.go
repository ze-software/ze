// VALIDATES: the `bgp update-delay` container reaches reactor.Config as a typed
//            UpdateDelay, and a configuration whose establish-wait is more than
//            its max-delay is refused at BOTH doors: the daemon's startup load
//            and the walk `ze config validate` and `ze doctor` reach.
// PREVENTS:  an operator's verify accepting a hold the daemon refuses, and an
//            establish-wait that can never fire being accepted in silence
//            because max-delay always ends the hold first.

package bgpconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// updateDelayConfig wraps an `update-delay` block in the smallest BGP config
// that reaches the reactor builder.
func updateDelayConfig(block string) string {
	return `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
` + block + `
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
`
}

func updateDelayTree(t *testing.T, block string) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(updateDelayConfig(block), nil)
	require.NoError(t, err, "the config must parse before update-delay can be judged")
	return tree
}

// TestUpdateDelayParsedFromTree is the config half of AC-1: the two leaves
// arrive at the reactor as durations.
func TestUpdateDelayParsedFromTree(t *testing.T) {
	got, err := ParseUpdateDelay(updateDelayTree(t, `
    update-delay {
        max-delay 30;
        establish-wait 10;
    }`))
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, got.MaxDelay, "max-delay must arrive in seconds")
	require.Equal(t, 10*time.Second, got.EstablishWait, "establish-wait must arrive in seconds")
	require.True(t, got.Enabled())
}

// TestUpdateDelayAbsentContainerDisablesTheHold is AC-5: an absent container
// leaves the reactor exactly as it was before the feature existed.
func TestUpdateDelayAbsentContainerDisablesTheHold(t *testing.T) {
	got, err := ParseUpdateDelay(updateDelayTree(t, ``))
	require.NoError(t, err)
	require.Equal(t, reactorUpdateDelayZero(), got, "an absent container must produce the zero value")
	require.False(t, got.Enabled(), "an absent container must not arm the hold")
}

// TestUpdateDelayExplicitZeroDisablesTheHold is the other half of AC-5: naming
// the YANG default is the same as naming nothing.
func TestUpdateDelayExplicitZeroDisablesTheHold(t *testing.T) {
	got, err := ParseUpdateDelay(updateDelayTree(t, `
    update-delay {
        max-delay 0;
    }`))
	require.NoError(t, err)
	require.False(t, got.Enabled(), "max-delay 0 must not arm the hold")
}

// TestUpdateDelayEstablishWaitOverMaxDelayRefused is AC-4.
func TestUpdateDelayEstablishWaitOverMaxDelayRefused(t *testing.T) {
	tree := updateDelayTree(t, `
    update-delay {
        max-delay 30;
        establish-wait 40;
    }`)

	_, err := ParseUpdateDelay(tree)
	require.Error(t, err, "establish-wait 40 with max-delay 30 must be refused")
	require.Contains(t, err.Error(), "establish-wait 40")
	require.Contains(t, err.Error(), "max-delay 30")

	// The same config must be refused at the door `ze config validate` and `ze
	// doctor` reach. Refusing at startup alone lets an operator's verify pass a
	// file the daemon will not load.
	_, verifyErr := PeersFromConfigTree(tree)
	require.Error(t, verifyErr, "the config-validate walk must refuse it too")
	require.Contains(t, verifyErr.Error(), "establish-wait")
}

// TestUpdateDelayEstablishWaitOverMaxDelayRefusedAtLoad closes the loop on the
// startup door: the whole reactor build fails rather than silently dropping the
// leaf.
func TestUpdateDelayEstablishWaitOverMaxDelayRefusedAtLoad(t *testing.T) {
	_, err := LoadReactor(updateDelayConfig(`
    update-delay {
        max-delay 30;
        establish-wait 40;
    }`))
	require.Error(t, err, "the daemon must refuse to start on an establish-wait it can never honor")
	require.Contains(t, err.Error(), "establish-wait")
}

// TestUpdateDelayBoundaries walks the numeric edges of both leaves. The last
// valid value, the value one past it, and the equal-to-max-delay case that AC-4
// makes the boundary of establish-wait.
func TestUpdateDelayBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		block string
		valid bool
	}{
		{"max-delay at its floor", "max-delay 0;", true},
		{"max-delay one above its floor", "max-delay 1;", true},
		{"max-delay at its ceiling", "max-delay 3600;", true},
		{"max-delay one above its ceiling", "max-delay 3601;", false},
		{"establish-wait at its floor", "max-delay 30; establish-wait 1;", true},
		{"establish-wait below its floor", "max-delay 30; establish-wait 0;", false},
		{"establish-wait equal to max-delay", "max-delay 30; establish-wait 30;", true},
		{"establish-wait one above max-delay", "max-delay 30; establish-wait 31;", false},
		{"establish-wait at its ceiling with max-delay there too", "max-delay 3600; establish-wait 3600;", true},
		{"establish-wait one above its ceiling", "max-delay 3600; establish-wait 3601;", false},
		{"establish-wait without max-delay", "establish-wait 1;", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := updateDelayConfig("    update-delay { " + tc.block + " }")
			// The YANG range refuses some of these before the parser sees them,
			// and the cross-leaf rule refuses the rest. Both are the daemon
			// saying no, and LoadReactor is where an operator meets either.
			_, err := LoadReactor(text)
			if tc.valid {
				require.NoError(t, err, "the config must be accepted")
				return
			}
			require.Error(t, err, "the config must be refused")
		})
	}
}

// reactorUpdateDelayZero names the disabled value so the assertion above reads
// as "the hold is off" rather than as a struct literal the reader must decode.
func reactorUpdateDelayZero() reactor.UpdateDelay {
	return reactor.UpdateDelay{}
}
