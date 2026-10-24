// Design: docs/architecture/plugin/plugin-system.md -- registration fields, config needs
// Related: registry.go -- Registration.ConfigNeeds and the Register validation that calls validateConfigNeeds

package registry

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidConfigNeed is returned when a registration declares a config need
// that could never be honored: an empty path or path element, no trigger
// value, no provider, or a provider outside the owner's OptionalDependencies.
var ErrInvalidConfigNeed = errors.New("registry: invalid config need")

// AnyEntry is the Path element that crosses a YANG list: it stands for every
// entry of the list named by the element before it, whatever the entry's key.
// A need through a list holds when any entry carries Value at the rest of the
// path: {"bgp-ls-export", "domain", AnyEntry, "source"} holds when one
// `domain` entry names the source. No YANG keyword is "*", so the marker
// cannot be read as a container name.
const AnyEntry = "*"

// ConfigNeed is one config value of the owner's that needs a provider plugin.
//
// A config holds the need when the leaf at Path carries Value. The build meets
// the need when any plugin Providers names is registered in it. The config
// component refuses a config that holds a need its build does not meet
// (internal/component/config/optional_provider.go).
//
// The registry stays a leaf: it holds the declaration only. The config
// component reads it through All() and evaluates it over the parsed tree.
type ConfigNeed struct {
	// Path is the leaf as the operator writes it, one keyword per element:
	// {"flow-export", "enrichment", "bgp"}. It MUST NOT be empty, and no
	// element may be empty. A list on the path is followed by AnyEntry, so
	// AnyEntry MUST NOT be the first or the last element.
	Path []string
	// Value is the leaf's config text that needs a provider: "true" for a
	// boolean, "vpp" for a backend name. It MUST NOT be empty.
	Value string
	// Providers are plugin names, any one of which meets the need. There MUST
	// be at least one, and each MUST also be in the owner's
	// OptionalDependencies, so the edge that orders the provider and the need
	// that refuses its absence name the same plugin.
	Providers []string
}

// validateConfigNeeds refuses the first need of reg that could never be
// honored. Register calls it with the registry lock held; it reads only reg.
func validateConfigNeeds(reg *Registration) error {
	for _, need := range reg.ConfigNeeds {
		if len(need.Path) == 0 {
			return fmt.Errorf("%w: plugin %q declares a need with an empty path", ErrInvalidConfigNeed, reg.Name)
		}
		leaf := strings.Join(need.Path, " ")
		if slices.Contains(need.Path, "") {
			return fmt.Errorf("%w: plugin %q need %q has an empty path element", ErrInvalidConfigNeed, reg.Name, leaf)
		}
		if need.Path[0] == AnyEntry {
			return fmt.Errorf("%w: plugin %q need %q starts with %q, which must follow the list it crosses",
				ErrInvalidConfigNeed, reg.Name, leaf, AnyEntry)
		}
		if need.Path[len(need.Path)-1] == AnyEntry {
			return fmt.Errorf("%w: plugin %q need %q ends with %q, so it names a list entry and no leaf",
				ErrInvalidConfigNeed, reg.Name, leaf, AnyEntry)
		}
		if need.Value == "" {
			return fmt.Errorf("%w: plugin %q need %q has no trigger value", ErrInvalidConfigNeed, reg.Name, leaf)
		}
		if len(need.Providers) == 0 {
			return fmt.Errorf("%w: plugin %q need %q names no provider", ErrInvalidConfigNeed, reg.Name, leaf)
		}
		for _, provider := range need.Providers {
			if !slices.Contains(reg.OptionalDependencies, provider) {
				return fmt.Errorf("%w: plugin %q need %q names provider %q, which is not in its OptionalDependencies",
					ErrInvalidConfigNeed, reg.Name, leaf, provider)
			}
		}
	}
	return nil
}

// ErrConfigNeedsUnanswered is returned by ResolveDependencies when the caller
// passes no NeedHolds. Resolution decides whether an optional provider some
// need names is pulled, and that decision depends on the config being
// resolved, so a caller with no config cannot get an answer: guessing "no
// need holds" would start a daemon without the provider the operator turned
// on, with nothing in the log to say why.
var ErrConfigNeedsUnanswered = errors.New("registry: dependency resolution was given no config to judge config needs against")

// NeedHolds answers whether the config being resolved holds need, declared
// by the plugin named owner. The config component builds it from the tree it
// resolves for (config.ConfigNeedHolds), so the registry receives the
// evaluated answer and stays a leaf with no config import.
type NeedHolds func(owner string, need ConfigNeed) bool

// PullsProvider reports whether this need, when it holds, pulls its provider.
// Only a need naming exactly one provider does. A need naming several names
// alternatives, any one of which meets it, and pulls none: the operator's
// chosen source loads from its own config root. Boot resolution
// (optionalProviderPulled) and reload (plugin/server appendHeldNeedProviders)
// both ask this, so the two cannot disagree.
func (n ConfigNeed) PullsProvider() bool {
	return len(n.Providers) == 1
}

// optionalProviderPulled reports whether resolution pulls provider, an
// OptionalDependencies entry of reg. A provider no ConfigNeed of reg names is
// always pulled, which is how route server, graceful restart and static load
// their optional providers. A provider a single-provider need names is pulled
// only when that need holds for the config: flow export with BGP enrichment
// off does not start the BGP RIB.
//
// A need naming more than one provider is a set of alternatives (anomaly and
// DDoS detection read flow export or traffic usage), so it pulls none of them,
// even when it holds. The operator's chosen source loads from its own config
// root, and pulling every alternative would start sources nobody configured,
// each with its own hard dependencies: flow export pulls interface, which
// refuses boot on a host with no interface backend. Such a need is enforced
// elsewhere: refused when the build carries none of its providers, and a
// doctor warning when none is configured.
func optionalProviderPulled(reg *Registration, provider string, needHolds NeedHolds) bool {
	named := false
	for _, need := range reg.ConfigNeeds {
		if !slices.Contains(need.Providers, provider) {
			continue
		}
		named = true
		if !need.PullsProvider() {
			continue
		}
		if needHolds(reg.Name, need) {
			return true
		}
	}
	return !named
}
