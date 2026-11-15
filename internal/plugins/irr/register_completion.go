// Design: docs/architecture/irr-owner.md -- the AS-SET names the editor completes
// Detail: completion.go -- the store read behind the names
// Related: internal/component/cli/completer.go -- validateCompletions, the consumer
//
// The AS-SET leaves of the consumers (bgp-filter-irr's `irr as-set`,
// firewall-irr's `source-as-set` and `destination-as-set`) carry
// `ze:validate "irr-as-set"`, and this file declares that name as a
// SUGGESTION from the owner, which is the one plugin that knows every AS-SET
// in play. RegisterSuggestion
// (internal/component/config/yang/validator_registry.go) writes a validator
// with no ValidateFn, so the walk refuses nothing: an AS-SET nothing stored or
// referenced yet stays valid, because writing it is how an operator asks for
// it. Whether a stored list exists is the consumer's commit-time question,
// answered with the `update irr as-set` command that fetches it.
//
// The name is in a register file because the pretool-writeedit gate refuses
// a Register call inside init() outside a file whose name starts with
// "register", and register.go carries the plugin registration.

package irr

import (
	"context"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// asSetValidatorName is the ze:validate name every consumer AS-SET leaf
// carries. It names a completion, not a rule.
const asSetValidatorName = "irr-as-set"

func init() {
	configyang.RegisterSuggestion(asSetValidatorName, asSetValues, asSetHelp)
}

// asSetValues answers the AS-SET names the store holds, as an operator types
// them. The completer sorts and filters by prefix.
func asSetValues() []string {
	known := daemonKnownASSets()
	values := make([]string, 0, len(known))
	for _, set := range known {
		values = append(values, set.name)
	}
	return values
}

// asSetHelp says whether one offered AS-SET has a stored dataset. It reads
// the pointer key alone, so the dropdown costs one read per offered name. A
// value that is no AS-SET name, or whose pointer cannot be read, answers the
// empty string, and the completer prints its generic label rather than a
// claim it could not check.
func asSetHelp(value string) string {
	name, err := store.ASSetName(value)
	// refuse-by-default: guards configyang.RegisterSuggestion -- its DescribeFn permits no description; the completer then prints its generic label.
	if err != nil {
		return ""
	}
	_, found, err := daemonStore{}.StateGet(context.Background(), store.PointerKey(name))
	// refuse-by-default: guards configyang.RegisterSuggestion -- its DescribeFn permits no description; the completer then prints its generic label.
	if err != nil {
		return ""
	}
	return knownASSet{name: name, stored: found}.help()
}
