// Design: docs/functional-tests.md -- explicit final policy on both real ingress and egress.
// Related: test/plugin/policy-final-disposition.ci -- socket scripts and configuration.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The order is shared with the numbered wire epochs in the .ci. Every refusal
// has a later permitted fence on the SAME destination socket, after quiesce.
var policyFinalCases = []struct {
	name    string
	filters string
	final   string
	accept  bool
	raw     bool
}{
	{name: "empty-accept", final: actionAccept, accept: true},
	{name: "empty-reject", final: actionReject},
	{name: "passing-reject", filters: "bgp-filter-path-asn:PASS", final: actionReject},
	{name: "early-reject", filters: "bgp-filter-path-asn:DENY", final: actionAccept},
	{name: "delta-accept", filters: policyFinalFilterMED, final: actionAccept, accept: true},
	{name: "delta-reject", filters: policyFinalFilterMED, final: actionReject},
	{name: "raw-accept", filters: "bgp-filter-family:NoV6", final: actionAccept, accept: true, raw: true},
	{name: "raw-reject", filters: "bgp-filter-family:NoV6", final: actionReject, raw: true},
}

// policyFinalFilterMED is the filter chain that rewrites MED, so an accepted
// route carries 77 rather than the 9 it was announced with.
const policyFinalFilterMED = "bgp-filter-modify:MED"

func policyFinalDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/policy-final-disposition takes no arguments")
	}
	return Observe(ctx, "policy-final", &sdk.Registration{}, policyFinalScenario)
}

func policyFinalScenario(ctx context.Context, plugin *sdk.Plugin) error {
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
	if !waitPeersEOR03(ctx, plugin, 2) {
		return errors.New("both policy peers must finish initial synchronization")
	}
	for index := range policyFinalCases {
		if err := policyFinalCase(ctx, plugin, tree, schema, index); err != nil {
			return fmt.Errorf("%s: %w", policyFinalCases[index].name, err)
		}
	}
	return policyFinalSessions(ctx, plugin, 2)
}

func policyFinalCase(ctx context.Context, plugin *sdk.Plugin, base *config.Tree, schema *config.Schema, index int) error {
	row := &policyFinalCases[index]
	configured := base.Clone()
	bgp := configured.GetContainer("bgp")
	if bgp == nil {
		return errors.New("policy fixture has no bgp configuration")
	}
	for peer, direction := range map[string]string{"peer-1": "import", "peer-2": "export"} {
		settings := bgp.GetList("peer")[peer]
		if settings == nil {
			return fmt.Errorf("policy fixture has no %s", peer)
		}
		policy := settings.GetContainerPath("filter/" + direction)
		if policy == nil {
			return fmt.Errorf("policy fixture has no %s %s policy", peer, direction)
		}
		policy.SetSlice("filters", strings.Fields(row.filters))
		policy.Set("final", row.final)
	}
	if err := policyFinalReload(ctx, plugin, config.Serialize(configured, schema), "applied"); err != nil {
		return err
	}
	before, err := peerCounter03(ctx, plugin, "127.0.0.1", "updates-received")
	if err != nil {
		return err
	}
	epoch := strconv.Itoa(index + 1)
	if err := liveStagesRoute(ctx, plugin, "127.0.0.1 update text nhop 192.0.2.1 nlri add 192.168."+epoch+".0/24"); err != nil {
		return err
	}
	if !waitPeerCounter03(ctx, plugin, "updates-received", before+1, 80) {
		return errors.New("source did not send this epoch's UPDATE")
	}
	if err := quiesce03(ctx, plugin); err != nil {
		return err
	}
	// The raw wire-mode command retains MP_REACH in its attributes and a
	// separate legacy IPv4 NLRI. No test filter substitutes for family removal.
	attributes := "4001010040020602010000FDE9400304C000020180040400000009C00804FDE90063"
	if row.raw {
		attributes += "800E1C0002011020010DB8000000000000000000000001003020010DB8000" + epoch
	}
	// The wire command takes its next-hop policy separately from attr set;
	// embedding NEXT_HOP alone leaves the announce batch's next hop unset.
	command := "update hex attr set " + attributes + " nhop set C0000201 nlri ipv4/unicast add 180A470" + epoch
	announced, _, err := plugin.UpdateRoute(ctx, "127.0.0.2", command)
	if err != nil {
		return err
	}
	if row.accept && announced != 1 {
		return fmt.Errorf("accepted export probe announced %d NLRIs, want 1", announced)
	}
	if err := quiesce03(ctx, plugin); err != nil {
		return err
	}
	// API-originated export probes have no retained RS source. Restoring the
	// policy cannot replay a forbidden probe and the next fence remains honest.
	if err := policyFinalReload(ctx, plugin, config.Serialize(base, schema), "applied"); err != nil {
		return err
	}
	if err := liveStagesRoute(ctx, plugin, "127.0.0.2 update text nhop 192.0.2.1 nlri add 192.169."+epoch+".0/24"); err != nil {
		return err
	}
	if !waitForFile(ctx, "final-fence-"+epoch, 100, liveStagesPollDelay) {
		return errors.New("destination did not receive the permitted wire fence")
	}
	prefix := "10.71." + epoch + ".0/24"
	if err := policyFinalRIB(ctx, plugin, prefix, row.accept); err != nil {
		return err
	}
	if row.accept {
		med := byte(9)
		if row.filters == policyFinalFilterMED {
			med = 77
		}
		if err := policyFinalImportedAttributes(ctx, plugin, prefix, med); err != nil {
			return err
		}
	}
	if row.raw {
		if err := policyFinalRIB(ctx, plugin, "2001:db8:"+epoch+"::/48", false); err != nil {
			return fmt.Errorf("removed family: %w", err)
		}
	}
	// Leave no API advertisements for the next final-reject reload to withdraw.
	// The sink acknowledges these withdrawals before the next epoch starts.
	withdraw := "127.0.0.2 update text nlri del 192.169." + epoch + ".0/24"
	if row.accept {
		withdraw += " " + prefix
	}
	if err := liveStagesRoute(ctx, plugin, withdraw); err != nil {
		return err
	}
	if !waitForFile(ctx, "final-clean-"+epoch, 100, liveStagesPollDelay) {
		return errors.New("destination did not receive the epoch cleanup withdrawals")
	}
	return nil
}

func policyFinalRIB(ctx context.Context, plugin *sdk.Plugin, prefix string, present bool) error {
	var stored struct {
		Peers map[string][]struct {
			Key string `json:"key"`
		} `json:"adj-rib-in"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp adj-rib-in", &stored)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("show bgp adj-rib-in: status=%s", status)
	}
	found := false
	for _, route := range stored.Peers["127.0.0.1"] {
		if strings.HasSuffix(route.Key, ":"+prefix) {
			found = true
		}
	}
	if found != present {
		return fmt.Errorf("received RIB prefix %s: want present=%t, got %v", prefix, present, stored)
	}
	return nil
}

func policyFinalReload(ctx context.Context, plugin *sdk.Plugin, config, outcome string) error {
	if err := os.WriteFile("policy-final-next.conf", []byte(config), 0o600); err != nil {
		return err
	}
	if err := liveStagesReload(ctx, plugin, "policy-final-next.conf"); err != nil {
		return err
	}
	value, err := done03(ctx, plugin, "show reload-status")
	if err != nil {
		return err
	}
	status, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("reload status is not an object: %v", value)
	}
	if status["last-outcome"] != outcome {
		return fmt.Errorf("want reload %s, got %v", outcome, value)
	}
	return nil
}

func policyFinalSessions(ctx context.Context, plugin *sdk.Plugin, count int) error {
	rows, err := peerRows03(ctx, plugin, "*")
	if err != nil {
		return err
	}
	if len(rows) != count {
		return fmt.Errorf("want %d original sessions, got %v", count, rows)
	}
	for peer, row := range rows {
		if _, exists := row["connections-dropped"]; !exists {
			return fmt.Errorf("%s omitted the connection-drop counter: %v", peer, row)
		}
		if number03(row["connections-established"]) != 1 {
			return fmt.Errorf("%s reconnected: %v", peer, row)
		}
		if number03(row["connections-dropped"]) != 0 {
			return fmt.Errorf("%s dropped its session: %v", peer, row)
		}
	}
	return nil
}
