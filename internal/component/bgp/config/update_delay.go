// Design: docs/architecture/config/syntax.md -- the bgp/update-delay container
// Related: loader_create.go -- CreateReactorFromTree, which puts the value in reactor.Config
// Related: peers.go -- peersAndDynamicGroups, the walk `ze config validate` reaches

package bgpconfig

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// updateDelaySecondsMax is the upper bound the YANG leaves declare
// (range "0..3600" and "1..3600"). It is repeated here because this parser runs
// on a tree the schema has not necessarily validated: PeersFromConfigTree walks
// a resolved map, and `ze config validate` reaches this function through it.
// A value the schema would have refused is refused here with the same number.
const updateDelaySecondsMax = 3600

// ParseUpdateDelay reads the `bgp update-delay` container and returns the
// startup convergence hold it configures.
//
// An absent container returns the zero UpdateDelay, which disables the hold. So
// does an explicit `max-delay 0`, which is the leaf's YANG default: the operator
// naming the default and the operator naming nothing produce the same reactor.
//
// It is the ONE producer of a reactor.UpdateDelay, and it is called twice for a
// reason. CreateReactorFromTree calls it for the VALUE it puts in the reactor
// config, and peersAndDynamicGroups calls it for the ERROR, because that walk is
// what `ze config validate` and `ze doctor` reach (register.go,
// validatePeersFromTree) and BGP is excluded from the generic custom-validator
// walk (internal/component/config/validate_sections.go, validatedSections). One
// function answers both, so a value the daemon accepts at startup and a value
// the operator's verify accepts can never diverge.
func ParseUpdateDelay(tree *config.Tree) (reactor.UpdateDelay, error) {
	if tree == nil {
		return reactor.UpdateDelay{}, nil
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return reactor.UpdateDelay{}, nil
	}
	container := bgp.GetContainer("update-delay")
	if container == nil {
		return reactor.UpdateDelay{}, nil
	}

	maxDelay, err := updateDelaySeconds(container, "max-delay", 0)
	if err != nil {
		return reactor.UpdateDelay{}, err
	}
	establishWait, err := updateDelaySeconds(container, "establish-wait", 1)
	if err != nil {
		return reactor.UpdateDelay{}, err
	}

	// establish-wait ends the hold EARLY, so a value at or under max-delay is
	// the only one that can ever fire: the outer bound releases first otherwise,
	// and the leaf would silently do nothing. Refused rather than clamped,
	// because a clamp answers a question the operator did not ask and hides the
	// typo that produced it.
	if establishWait > maxDelay {
		return reactor.UpdateDelay{}, fmt.Errorf(
			"bgp update-delay: establish-wait %d must not be more than max-delay %d; "+
				"establish-wait ends the hold early, so a larger value can never take effect",
			establishWait, maxDelay)
	}

	return reactor.UpdateDelay{
		MaxDelay:      time.Duration(maxDelay) * time.Second,
		EstablishWait: time.Duration(establishWait) * time.Second,
	}, nil
}

// updateDelaySeconds reads one seconds-valued leaf out of the update-delay
// container. An absent leaf returns 0, which every caller reads as "the operator
// said nothing about this one".
//
// secondsMin is the smallest value the leaf's own YANG range allows. It is a
// parameter rather than a constant because the two leaves differ: max-delay
// accepts 0 as its off switch, and establish-wait does not, since 0 seconds of
// waiting is what an absent leaf already means.
func updateDelaySeconds(container *config.Tree, leaf string, secondsMin int) (int, error) {
	raw, present := container.Get(leaf)
	if !present {
		return 0, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("bgp update-delay: %s must be a whole number of seconds, got %q", leaf, raw)
	}
	if seconds < secondsMin || seconds > updateDelaySecondsMax {
		return 0, fmt.Errorf("bgp update-delay: %s must be between %d and %d seconds, got %d",
			leaf, secondsMin, updateDelaySecondsMax, seconds)
	}
	return seconds, nil
}
