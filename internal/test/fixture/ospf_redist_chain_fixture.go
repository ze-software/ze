// Design: docs/architecture/testing/ci-format.md -- compiled Go fixtures
// Related: routing_fixture.go -- registers this scenario and holds the row helpers
// Related: plugin_fixture_12_redistribute_chain.go -- the same chain through fakedest, and the reload signal

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The prefixes of the static-into-OSPF chain scenario, as the Link State IDs of
// their AS-external-LSAs (RFC 2328 Section 12.1.4: the Link State ID of a type-5
// LSA is the destination's IP network number).
const (
	ospfChainKept    = "10.81.1.0" // passed by WIDE and by NARROW
	ospfChainDropped = "10.81.2.0" // passed by WIDE, rejected by NARROW
	ospfChainRefused = "10.81.3.0" // rejected by WIDE
)

// ospfChainNarrowConfig is the tmpfs file holding the narrowed config the
// scenario writes over the running one before it signals the reload.
const ospfChainNarrowConfig = "config-narrow-ospf.conf"

// ospfMaxAge is MaxAge (RFC 2328 Appendix B): an LSA at this age is being
// flushed and no longer advertises its destination.
const ospfMaxAge = 3600

// ospfRedistChainStaticScenario holds route-policy AC-14 with a real producer
// and a real consumer: static routes enter OSPF through
// `redistribute { destination ospf { import static { filters [ WIDE ]; ... } } }`.
// It waits until the LSDB holds a live AS-external-LSA for each prefix WIDE
// passes, requires none for the prefix it rejects, then reloads with the chain
// narrowed and waits until the dropped prefix's LSA is flushed while the kept
// one stays live.
func ospfRedistChainStaticScenario(ctx context.Context, plugin *sdk.Plugin) error {
	var live []string
	var dispatchErr error
	if !Poll(ctx, redistChainAttempts, redistChainDelay, func() bool {
		live, dispatchErr = ospfLiveExternals(ctx, plugin)
		if dispatchErr != nil {
			return true
		}
		return slices.Contains(live, ospfChainKept) && slices.Contains(live, ospfChainDropped)
	}) {
		return fmt.Errorf("show ospf database external: want live AS-external-LSAs for %s and %s, got %v",
			ospfChainKept, ospfChainDropped, live)
	}
	if dispatchErr != nil {
		return dispatchErr
	}
	// The three static routes reach the orchestrator in one config apply, so
	// once the two passed prefixes are originated the third has been judged.
	if slices.Contains(live, ospfChainRefused) {
		return fmt.Errorf("the chain rejects %s and OSPF originates it anyway: %v", ospfChainRefused, live)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF originates only the static prefixes the chain passes:", live)

	if err := ospfChainReloadNarrowed(ctx); err != nil {
		return err
	}
	flushed := Poll(ctx, redistChainAttempts, redistChainDelay, func() bool {
		live, dispatchErr = ospfLiveExternals(ctx, plugin)
		if dispatchErr != nil {
			return true
		}
		return !slices.Contains(live, ospfChainDropped)
	})
	if dispatchErr != nil {
		return dispatchErr
	}
	if !flushed {
		return fmt.Errorf("the reload narrowed the chain and OSPF still advertises %s: %v", ospfChainDropped, live)
	}
	if !slices.Contains(live, ospfChainKept) {
		return fmt.Errorf("the narrowed chain still passes %s and OSPF stopped advertising it: %v", ospfChainKept, live)
	}
	fmt.Fprintln(os.Stderr, "OK: the narrowing reload flushed", ospfChainDropped, "and kept", ospfChainKept)
	return nil
}

// ospfChainReloadNarrowed writes the narrowed config over the running one and
// sends the daemon SIGHUP.
func ospfChainReloadNarrowed(ctx context.Context) error {
	narrowed, err := os.ReadFile(ospfChainNarrowConfig)
	if err != nil {
		return fmt.Errorf("read %s: %w", ospfChainNarrowConfig, err)
	}
	if err := os.WriteFile(fileBGPConf, narrowed, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", fileBGPConf, err)
	}
	// The runner writes daemon.pid once the daemon reports ready, which can
	// come after every plugin is up and this scenario has started.
	if !Poll(ctx, redistChainAttempts, redistChainDelay, func() bool {
		_, err := os.Stat("daemon.pid")
		return err == nil
	}) {
		return errors.New("the runner never wrote daemon.pid, so no reload can be sent")
	}
	return plugin01SignalDaemonReload()
}

// ospfLiveExternals answers the Link State IDs of the AS-external-LSAs below
// MaxAge in `show ospf database external`.
func ospfLiveExternals(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	value, err := routingValue(ctx, plugin, "show ospf database external")
	if err != nil {
		return nil, err
	}
	database, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("show ospf database external: expected a JSON object, got %#v", value)
	}
	// An LSDB with no AS-external-LSA renders the list as null; an object
	// with no such key is a renamed field, which is refused, never read as empty.
	raw, present := database["as-external"]
	if !present {
		return nil, fmt.Errorf("show ospf database external: no as-external key: %#v", database)
	}
	var live []string
	if raw == nil {
		return live, nil
	}
	externals, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("show ospf database external: as-external is not a list: %#v", raw)
	}
	for _, entry := range externals {
		lsa, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("show ospf database external: an as-external entry is not an object: %#v", entry)
		}
		id, ok := lsa["link-state-id"].(string)
		if !ok {
			return nil, fmt.Errorf("show ospf database external: an LSA carries no link-state-id: %#v", lsa)
		}
		age, ok := lsa["age"].(float64)
		if !ok {
			return nil, fmt.Errorf("show ospf database external: LSA %s carries no age: %#v", id, lsa)
		}
		if age < ospfMaxAge {
			live = append(live, id)
		}
	}
	return live, nil
}
