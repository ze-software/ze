// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// RFC: rfc/short/rfc7999.md -- the honoring rules a reload reselects under
// Related: rib_multipath_config.go -- maximum-paths and relax-as-path
// Related: rib_blackhole_config.go -- the RFC 7999 honoring rules
// Related: rib_asn_notation.go -- the AS notation, recorded only by an external rib
// Related: rib_aigp.go -- reselectAIGPRoutes, the walk a selection-rule change runs

package rib

import (
	"net/netip"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ribConfig holds the BGP settings and the system distance table the RIB reads.
type ribConfig struct {
	maximumPaths      uint32
	relaxASPath       bool
	blackhole         map[configjson.PeerConfigKey]blackholeConfig
	notation          asn.Notation
	redirect          flowSpecRedirectConfig
	distances         map[string]uint8
	selectionDeferral time.Duration
	expectedPeers     []netip.Addr
	aggregates        aggregateConfig
}

// unconfiguredRIB is what the rib runs before any config: one best path, no
// relaxed AS_PATH, no blackhole honoring, asplain.
func unconfiguredRIB() ribConfig {
	return ribConfig{maximumPaths: defaultMultipathPaths, notation: asn.NotationPlain, redirect: flowSpecRedirectConfig{base: true}, distances: originDefaultDistances()}
}

// newConfigTx returns the rib's config transaction: Stage 2 installs at once,
// a reload installs at apply and restores at rollback, and an edit that
// leaves every leaf the rib reads as it was installs nothing.
//
// recordsNotation MUST be true only when the rib runs in a process of its
// own. In the daemon the reactor records the notation when a reload commits
// (SetConfigTree), and a second writer could leave a rolled-back candidate's
// notation in force.
func (r *RIBManager) newConfigTx(recordsNotation bool) *sdk.ConfigTx[ribConfig] {
	initial := unconfiguredRIB()
	var running atomic.Pointer[ribConfig]
	parse := func(sections []sdk.ConfigSection) (ribConfig, bool, error) {
		if current := running.Load(); current != nil {
			return parseRIBConfigSections(*current, sections)
		}
		return parseRIBConfig(sections)
	}
	install := func(cfg ribConfig) error {
		if err := r.installConfig(cfg, recordsNotation); err != nil {
			return err
		}
		// Publish one immutable base for the next changed-root delivery.
		running.Store(&cfg)
		return nil
	}
	return sdk.NewConfigTx(initial, parse, install)
}

// parseRIBConfig reads the first delivery over the default BGP and RIB settings.
// It changes no state, so a refused verify and a CLI validate leave the
// running rib as it was.
//
// A section the rib cannot parse is refused rather than leaving the previous
// rules in place: the blackhole rules decide whether a peer can make Ze
// discard traffic, and a rejected edit that silently keeps running with the
// old answer is the worst of the three outcomes.
func parseRIBConfig(sections []sdk.ConfigSection) (ribConfig, bool, error) {
	return parseRIBConfigSections(unconfiguredRIB(), sections)
}

// parseRIBConfigSections applies each delivered root to the installed base.
// Omitted roots keep their settings. An empty delivered root resets its settings.
func parseRIBConfigSections(cfg ribConfig, sections []sdk.ConfigSection) (ribConfig, bool, error) {
	distances, readDistances, distanceError := parseOriginPreferences(sections)
	if distanceError != nil {
		return ribConfig{}, false, distanceError
	}
	if readDistances {
		cfg.distances = distances
	}
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return ribConfig{}, false, errRibInvalidBgpConfigJson
		}
		var err error
		cfg.maximumPaths, cfg.relaxASPath, err = extractMultipathConfig(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		cfg.blackhole, err = parseBlackholeConfig(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		cfg.notation, err = parseASNotation(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		cfg.redirect, err = parseFlowSpecRedirectConfig(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		cfg.selectionDeferral, cfg.expectedPeers, err = parseSelectionDeferralConfig(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		// RFC 4271 Section 9.2.2.2, Section 5.1.7; RFC 9774 Section 5.2
		cfg.aggregates, err = parseAggregateConfig(bgpCfg)
		if err != nil {
			return ribConfig{}, false, err
		}
		return cfg, true, nil
	}
	if readDistances {
		return cfg, true, nil
	}
	return ribConfig{}, false, nil
}

// installConfig makes cfg the rib's running config. When a selection rule
// changed it reselects every stored prefix, so a prefix that receives no
// further UPDATE still answers under the new rule: its multipath set, and its
// blackhole route type, which the reselect publishes even when the winner
// stays (checkBestPathChange compares the route type).
func (r *RIBManager) installConfig(cfg ribConfig, recordsNotation bool) error {
	if err := r.installOriginPreferenceConfig(cfg.distances); err != nil {
		return err
	}
	rulesChanged := r.maximumPaths.Load() != cfg.maximumPaths ||
		r.relaxASPath.Load() != cfg.relaxASPath ||
		!r.sameBlackholeRules(cfg.blackhole)

	r.maximumPaths.Store(cfg.maximumPaths)
	r.relaxASPath.Store(cfg.relaxASPath)
	blackhole := cfg.blackhole
	r.blackholeCfg.Store(&blackhole)
	previousRedirect := r.flowSpecRedirectCfg.Load()
	redirect := cfg.redirect
	r.flowSpecRedirectCfg.Store(&redirect)
	previousAggregates := r.aggregateCfg.Load()
	var nextAggregates *aggregateConfig
	if len(cfg.aggregates.Rules) > 0 {
		aggCopy := cfg.aggregates
		nextAggregates = &aggCopy
	}
	r.aggregateCfg.Store(nextAggregates)
	if cfg.selectionDeferral > 0 {
		if r.selectionDefer.arm(r.deferClock(), cfg.selectionDeferral, cfg.expectedPeers, r.reselectDeferredFamily, r.emitSelectionComplete) {
			r.emitSelectionDeferred(cfg.selectionDeferral)
		}
	}
	if recordsNotation {
		asn.Record(cfg.notation)
	}
	logger().Debug("rib configured",
		"as-notation", cfg.notation.String(),
		"maximum-paths", cfg.maximumPaths,
		"relax-as-path", cfg.relaxASPath,
		"blackhole-honor-rules", len(cfg.blackhole),
		"reselect", rulesChanged,
	)
	if rulesChanged {
		r.reselectAIGPRoutes()
	}
	// A switched redirect-to-IP check changes retained FlowSpec verdicts
	// without any UPDATE, so they are recomputed here.
	if previousRedirect == nil || !reflect.DeepEqual(*previousRedirect, redirect) {
		r.reconcileFlowSpecs()
	}
	// RFC 4271 Section 9.2.2.2: recompute synthesized aggregates when rules change.
	if !reflect.DeepEqual(previousAggregates, nextAggregates) {
		r.recomputeAllAggregates()
	}
	return nil
}

// sameBlackholeRules reports whether the installed rules equal the candidate
// ones. No installed rules and an empty candidate are the same: neither
// honors anything.
func (r *RIBManager) sameBlackholeRules(candidate map[configjson.PeerConfigKey]blackholeConfig) bool {
	installed := r.blackholeCfg.Load()
	if installed == nil {
		return len(candidate) == 0
	}
	return reflect.DeepEqual(*installed, candidate)
}
