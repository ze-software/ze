// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: chain.go -- Compile, Run
// Related: sets.go -- Sets.ApplyTo

package policychain

import (
	"fmt"
	"math"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
)

// Bases is the value each numeric field's arithmetic starts from when the
// route carries no such attribute: bgp { defaults { attribute { } } }. The
// YANG defaults of both leaves make both always configured.
type Bases struct {
	LocalPreference, MED uint32
}

// Apply writes the sets the last accepting Run folded as ModAccumulator
// operations. It reads the route's LOCAL_PREF and MED from view only for a
// field the sets change, because an increment or decrement resolves against
// the current value or, absent one, against the chain's base. view MUST be the
// view that Run judged, not yet Cleared.
func (c *Chain) Apply(view *policyview.View, sets *RunSets, acc *filterapi.ModAccumulator) error {
	folded := &sets.folded
	current := Current{
		LocalPreferenceBase: c.bases.LocalPreference, HasLocalPreferenceBase: true,
		MEDBase: c.bases.MED, HasMEDBase: true,
	}
	if folded.written&setKindLocalPreference.bit() != 0 {
		value, present, err := numberOf(view, c.localPreference)
		if err != nil {
			return fmt.Errorf("local-preference: %w", err)
		}
		current.LocalPreference, current.HasLocalPreference = value, present
	}
	if folded.written&setKindMED.bit() != 0 {
		value, present, err := numberOf(view, c.med)
		if err != nil {
			return fmt.Errorf("med: %w", err)
		}
		current.MED, current.HasMED = value, present
	}
	return folded.ApplyTo(acc, current)
}

// numberOf answers a four-octet numeric attribute of the route, and false
// when the route carries none.
func numberOf(view *policyview.View, reader policyview.Reader) (uint32, bool, error) {
	value, err := view.Read(reader)
	if err != nil {
		return 0, false, err
	}
	if !value.Present() {
		return 0, false, nil
	}
	number, numbered := value.Number()
	if !numbered {
		return 0, false, fmt.Errorf("BUG: policychain: the %s reader answered no number", reader.Field())
	}
	if number > math.MaxUint32 {
		return 0, false, fmt.Errorf("value %d exceeds four octets", number)
	}
	return uint32(number), true, nil
}
