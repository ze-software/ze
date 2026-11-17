// Design: docs/functional-tests.md -- final-only reloads reconcile the existing wire state.
// Related: test/reload/bgp-policy-final-reload.ci -- retained routes and a client-specific alternate.
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

func policyFinalReloadDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("reload/bgp-policy-final-reload takes no arguments")
	}
	state := &policyFinalWireState{routes: make(map[string]bool)}
	return observeConfigured(ctx, "policy-final-reload", &sdk.Registration{}, func(plugin *sdk.Plugin) error {
		plugin.OnEvent(state.receive)
		plugin.SetStartupSubscriptions([]string{eventUpdate}, []string{addrLoopbackSecond}, "full")
		return nil
	}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return policyFinalReloadScenario(ctx, plugin, state)
	})
}

func policyFinalReloadScenario(ctx context.Context, plugin *sdk.Plugin, state *policyFinalWireState) error {
	base, err := os.ReadFile(fileBGPConf)
	if err != nil {
		return err
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return err
	}
	tree, err := config.NewParser(schema).Parse(string(base))
	if err != nil {
		return err
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return errors.New("reload fixture has no bgp configuration")
	}
	destination := bgp.GetList("peer")["peer-2"]
	if destination == nil {
		return errors.New("reload fixture has no destination peer-2")
	}
	filter := destination.GetContainer("filter")
	if filter == nil {
		return errors.New("reload fixture has no destination filter")
	}
	export := filter.GetContainer("export")
	if export == nil {
		return errors.New("reload fixture has no destination export policy")
	}
	if !waitPeersEOR03(ctx, plugin, 3) {
		return errors.New("both sources and destination initial synchronization did not finish")
	}
	// The best-path oracle needs the received next hops to resolve locally.
	if err := rfc8212ReloadTopology(ctx, plugin); err != nil {
		return err
	}
	if err := liveStagesRoute(ctx, plugin, "127.0.0.1 update text nhop 192.0.2.1 nlri add 192.168.1.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "original subject", prefix72First); err != nil {
		return err
	}
	invalid := config.NewTree()
	invalid.SetSlice("filters", export.GetSlice("filters"))
	filter.SetContainer("export", invalid)
	if err := policyFinalReload(ctx, plugin, config.Serialize(tree, schema), "failed"); err != nil {
		return fmt.Errorf("missing-final validation: %w", err)
	}
	// This route is received only after invalid reload completion. Its delivery
	// proves the live accept policy survived; an unchanged parser status alone
	// would not prove the running dataplane was left intact.
	if err := liveStagesRoute(ctx, plugin, "127.0.0.1 update text nhop 192.0.2.1 nlri add 192.168.2.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "invalid reload preserved acceptance", prefix72First, "10.72.2.0/24"); err != nil {
		return err
	}
	before, err := peerCounter03(ctx, plugin, "127.0.0.1", "updates-received")
	if err != nil {
		return err
	}
	destinationBefore, err := peerRows03(ctx, plugin, addrLoopbackSecond)
	if err != nil {
		return err
	}
	exportBefore, ok := destinationBefore[addrLoopbackSecond]["export-policy"].(map[string]any)
	if !ok {
		return fmt.Errorf("before final reject: destination omitted export-policy: %v", destinationBefore)
	}
	filter.SetContainer("export", export)
	export.Set("final", "reject")
	reject := config.Serialize(tree, schema)
	if err := policyFinalReload(ctx, plugin, reject, "applied"); err != nil {
		return err
	}
	destinationAfter, err := peerRows03(ctx, plugin, addrLoopbackSecond)
	if err != nil {
		return err
	}
	exportAfter, ok := destinationAfter[addrLoopbackSecond]["export-policy"].(map[string]any)
	if !ok {
		return fmt.Errorf("after applied final reject: destination omitted export-policy: %v", destinationAfter)
	}
	_, _ = fmt.Fprintf(os.Stderr, "final-only reload installed-policy diagnostic: before=%v after=%v\n", exportBefore, exportAfter)
	if exportAfter["final"] != "reject" {
		return fmt.Errorf("after applied final reject: installed export-policy=%v, want final reject", exportAfter)
	}
	if _, present := exportAfter["final-source"]; present {
		return fmt.Errorf("after applied final reject: configured export-policy unexpectedly reports final-source: %v", exportAfter)
	}
	if !reflect.DeepEqual(exportBefore["filters"], exportAfter["filters"]) {
		return fmt.Errorf("final-only reload changed installed export filters: before=%v after=%v", exportBefore, exportAfter)
	}
	if err := state.wait(ctx, "final reject withdrew both advertisements"); err != nil {
		return err
	}
	if err := policyFinalReload(ctx, plugin, string(base), "applied"); err != nil {
		return err
	}
	if err := state.wait(ctx, "final accept restored both retained routes", prefix72First, "10.72.2.0/24"); err != nil {
		return err
	}
	after, err := peerCounter03(ctx, plugin, "127.0.0.1", "updates-received")
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("source sent an UPDATE during final-only reconciliation: %d -> %d", before, after)
	}
	if err := policyFinalAlternate(ctx, plugin, state, string(base), reject); err != nil {
		return err
	}
	if err := policyFinalSessions(ctx, plugin, 3); err != nil {
		return err
	}
	return state.wait(ctx, "final received state", prefix72First, "10.72.2.0/24", "10.72.3.0/24", "10.72.253.0/24")
}
