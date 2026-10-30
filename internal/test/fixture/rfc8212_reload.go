// Design: plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md -- D1 and D2 real-entry drafts.
// Related: policy_final.go -- the existing SIGHUP completion and original-session helpers.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func rfc8212ReloadDriver(scenario func(context.Context, *sdk.Plugin, *rfc8212ReloadWire, *config.Tree, *config.Schema) error) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("RFC8212 reload observer takes no arguments")
		}
		wire := newRFC8212ReloadWire()
		wire.snapshot = rfc8212DownstreamFile
		return observeConfigured(ctx, "rfc8212-reload", &sdk.Registration{}, func(p *sdk.Plugin) error {
			p.OnEvent(wire.receive)
			p.SetStartupSubscriptions([]string{"update", "state"}, []string{"*"}, "full")
			return nil
		}, func(ctx context.Context, p *sdk.Plugin) error {
			data, err := os.ReadFile(fileBGPConf)
			if err != nil {
				return err
			}
			schema, err := config.YANGSchema()
			if err != nil {
				return err
			}
			tree, err := config.NewParser(schema).Parse(string(data))
			if err != nil {
				return err
			}
			if !waitPeersEOR03(ctx, p, 3) {
				return errors.New("three original sessions never synchronized")
			}
			if err := scenario(ctx, p, wire, tree, schema); err != nil {
				return err
			}
			wire.mu.Lock()
			defer wire.mu.Unlock()
			if wire.downs != 0 {
				return fmt.Errorf("policy reload synthesized %d DOWN events", wire.downs)
			}
			return wire.failure
		})
	}
}

func rfc8212ReloadPolicy(tree *config.Tree, final string) error {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return errors.New("fixture omitted bgp")
	}
	peer := bgp.GetList("peer")["peer-1"]
	if peer == nil {
		return errors.New("fixture omitted peer-1")
	}
	filter := peer.GetContainer("filter")
	if filter == nil {
		return errors.New("fixture omitted source export policy")
	}
	if final == "absent" {
		filter.RemoveContainer("import")
		return nil
	}
	policy := config.NewTree()
	policy.Set("final", final)
	filter.SetContainer("import", policy)
	return nil
}

func rfc8212ReloadTrigger(ctx context.Context, p *sdk.Plugin, peer, epoch string) error {
	return liveStagesRoute(ctx, p, peer+" update text nhop 192.0.2.1 nlri add 192.168."+epoch+".0/24")
}

func rfc8212ReloadSeed(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, accepted bool) error {
	if err := rfc8212ReloadTrigger(ctx, p, "127.0.0.3", "1"); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	if err := wire.path(ctx, "10.72.2.0/24", true); err != nil {
		return err
	}
	if err := wire.path(ctx, "10.72.3.0/24", true); err != nil {
		return err
	}
	before, err := rfc8212ReloadInput(ctx, p)
	if err != nil {
		return err
	}
	if err := rfc8212ReloadTrigger(ctx, p, addrLoopback, "1"); err != nil {
		return err
	}
	if err := rfc8212ReloadWaitInput(ctx, p, before, 2); err != nil {
		return fmt.Errorf("source initial UPDATEs: %w", err)
	}
	if accepted {
		if err := wire.wait(ctx, prefix72First, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
			return err
		}
		if err := wire.path(ctx, "10.72.3.0/24", false); err != nil {
			return err
		}
		return rfc8212ReloadBest(ctx, p, "10.72.3.0/24", addrLoopback)
	}
	// A later UPDATE on the SAME source socket is counted before the RIB and
	// wire checks. The positive alternate baseline prevents a dead sink passing.
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	return rfc8212ReloadBest(ctx, p, prefix72First, "")
}

func rfc8212ReloadRecover(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	if err := rfc8212ReloadSeed(ctx, p, wire, false); err != nil {
		return err
	}
	before, err := peerRows03(ctx, p, "*")
	if err != nil {
		return err
	}
	if err := rfc8212ReloadPolicy(tree, "accept"); err != nil {
		return err
	}
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
		return err
	}
	// The source script reannounces ONLY after a Route Refresh on conn=1.
	if err := wire.wait(ctx, prefix72First, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	if err := wire.path(ctx, prefix72First, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, p, prefix72First, addrLoopback); err != nil {
		return err
	}
	if err := rfc8212ReloadSessions(ctx, p, before, 1); err != nil {
		return err
	}
	if err := rfc8212ReloadInputDelta(ctx, p, before, 2); err != nil {
		return err
	}
	return rfc8212ReloadRunning(ctx, p, "accept")
}

func rfc8212ReloadNoRefresh(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	if err := rfc8212ReloadSeed(ctx, p, wire, false); err != nil {
		return err
	}
	before, err := peerRows03(ctx, p, "*")
	if err != nil {
		return err
	}
	prior, err := done03(ctx, p, "show config running")
	if err != nil {
		return err
	}
	active, err := rfc8212ReloadActive()
	if err != nil {
		return err
	}
	if err := rfc8212ReloadPolicy(tree, "accept"); err != nil {
		return err
	}
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "failed"); err != nil {
		return err
	}
	if _, err := os.Stat("refusal.applied"); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("no-refresh refusal reached a config apply callback: refusal.applied exists")
		}
		return fmt.Errorf("no-refresh refusal reached a config apply callback: marker: %w", err)
	}
	current, err := done03(ctx, p, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(prior, current) {
		return errors.New("no-refresh refusal published a changed running configuration")
	}
	if err := rfc8212ReloadDurable(active); err != nil {
		return err
	}
	if err := rfc8212ReloadRunning(ctx, p, "reject"); err != nil {
		return err
	}
	if err := rfc8212ReloadTrigger(ctx, p, addrLoopback, "2"); err != nil {
		return err
	}
	if !waitForFile(ctx, "source.after", 80, liveStagesPollDelay) {
		return errors.New("source export fence never reached original TCP connection")
	}
	if err := rfc8212ReloadInputDelta(ctx, p, before, 1); err != nil {
		return fmt.Errorf("source mixed UPDATE on preserved session: %w", err)
	}
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	if err := rfc8212ReloadInputDelta(ctx, p, before, 1); err != nil {
		return err
	}
	return rfc8212ReloadSessions(ctx, p, before, 0)
}

func rfc8212ReloadInvalidate(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	return rfc8212ReloadInvalidateBy(ctx, p, wire, tree, schema, func(tree *config.Tree) error {
		return rfc8212ReloadPolicy(tree, "absent")
	})
}

// rfc8212ReloadInherited removes the import policy peer-1 inherits from its
// group, so the same invalidation must follow a change at the group layer.
func rfc8212ReloadInherited(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema) error {
	return rfc8212ReloadInvalidateBy(ctx, p, wire, tree, schema, rfc8212ReloadGroupImportAbsent)
}

// rfc8212ReloadGroupImportAbsent removes group sources' import policy. The
// group MUST carry one and peer-1 MUST NOT, or the edit changes nothing peer-1
// inherits and the scenario would test the wrong layer.
func rfc8212ReloadGroupImportAbsent(tree *config.Tree) error {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return errors.New("fixture omitted bgp")
	}
	group := bgp.GetList("group")["sources"]
	if group == nil {
		return errors.New("fixture omitted group sources")
	}
	filter := group.GetContainer("filter")
	if filter == nil {
		return errors.New("fixture omitted the group filter")
	}
	if filter.GetContainer("import") == nil {
		return errors.New("fixture omitted the group import policy")
	}
	peer := group.GetList("peer")["peer-1"]
	if peer == nil {
		return errors.New("fixture omitted peer-1 inside group sources")
	}
	if own := peer.GetContainer("filter"); own != nil {
		if own.GetContainer("import") != nil {
			return errors.New("peer-1 carries its own import policy, so the group edit is not inherited")
		}
	}
	filter.RemoveContainer("import")
	return nil
}

// rfc8212ReloadInvalidateBy seeds the three sessions, applies edit (which
// removes the source's last accepting import layer) through a live reload, and
// checks the downstream withdrawal, the re-election, the unchanged export to
// the source and the preserved session identity.
func rfc8212ReloadInvalidateBy(ctx context.Context, p *sdk.Plugin, wire *rfc8212ReloadWire, tree *config.Tree, schema *config.Schema, edit func(*config.Tree) error) error {
	if err := rfc8212ReloadSeed(ctx, p, wire, true); err != nil {
		return err
	}
	before, err := peerRows03(ctx, p, "*")
	if err != nil {
		return err
	}
	if err := edit(tree); err != nil {
		return err
	}
	if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, p, prefix72First, ""); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, p, "10.72.3.0/24", "127.0.0.3"); err != nil {
		return err
	}
	if err := wire.path(ctx, "10.72.3.0/24", true); err != nil {
		return err
	}
	if err := wire.path(ctx, "10.72.2.0/24", true); err != nil {
		return err
	}
	if err := rfc8212ReloadRunning(ctx, p, "reject"); err != nil {
		return err
	}
	// The script rejects withdrawal of the pre-existing export marker and
	// answers this new export with mixed input: old withdrawal + denied add.
	if err := rfc8212ReloadTrigger(ctx, p, addrLoopback, "2"); err != nil {
		return err
	}
	if !waitForFile(ctx, "source.after", 80, liveStagesPollDelay) {
		return errors.New("source export direction was reset by import invalidation")
	}
	if err := rfc8212ReloadInputDelta(ctx, p, before, 1); err != nil {
		return fmt.Errorf("mixed UPDATE after invalidation: %w", err)
	}
	if err := quiesce03(ctx, p); err != nil {
		return err
	}
	if err := wire.wait(ctx, "10.72.2.0/24", "10.72.3.0/24"); err != nil {
		return err
	}
	if err := rfc8212ReloadInputDelta(ctx, p, before, 1); err != nil {
		return err
	}
	return rfc8212ReloadSessions(ctx, p, before, 0)
}

func rfc8212ReloadRunning(ctx context.Context, p *sdk.Plugin, final string) error {
	rows, err := peerRows03(ctx, p, addrLoopback)
	if err != nil {
		return err
	}
	policy, ok := rows[addrLoopback]["import-policy"].(map[string]any)
	if !ok {
		return fmt.Errorf("source detail omitted effective import policy: %v", rows)
	}
	if policy["final"] != final {
		return fmt.Errorf("source import final=%v, want %s", policy, final)
	}
	return nil
}
