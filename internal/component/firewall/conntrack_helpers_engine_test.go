// Design: docs/guide/configuration.md -- conntrack helper assignment through the firewall transaction

package firewall

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// helperBackend loads a countingBackend under name and records the tables of
// every Apply it receives. The registry, the backends and the applied snapshot
// are reset when the test ends.
func helperBackend(t *testing.T, name string) (*countingBackend, *[]Table) {
	t.Helper()
	resetBackends()
	resetTables()
	StoreLastApplied(nil)
	t.Cleanup(func() {
		resetBackends()
		resetTables()
		StoreLastApplied(nil)
	})
	var applied []Table
	b := &countingBackend{onApply: func(d []Table) { applied = d }}
	if err := RegisterBackend(name, func() (Backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if err := LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	return b, &applied
}

func conntrackSection(data string) sdk.ConfigSection {
	return sdk.ConfigSection{Root: configRootConntrack, Data: data}
}

func tableNames(tables []Table) []string {
	names := make([]string, 0, len(tables))
	for i := range tables {
		names = append(names, tables[i].Name)
	}
	return names
}

// VALIDATES: spec-home-cpe-3 AC-5 -- a reload that deletes the whole
// `system conntrack` block, or keeps the block and drops every module, parses
// to no helper table, so the apply withdraws the running ones.
// PREVENTS: B-1, the hub's early return on an absent block that left
// ze_conntrack_helpers installed and the helper assigned.
func TestConntrackSectionRemovedWithdrawsHelpers(t *testing.T) {
	running, err := parseConfigSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["ftp"]}}}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(running.HelperTables) != 1 || running.HelperTables[0].Name != conntrackHelperTableInet {
		t.Fatalf("ftp must lower to %s, got %v", conntrackHelperTableInet, tableNames(running.HelperTables))
	}

	for name, data := range map[string]string{
		"block deleted":   `{}`,
		"module removed":  `{"system":{"conntrack":{"table-size":"262144"}}}`,
		"empty delivered": ``,
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := parseConfigSections([]sdk.ConfigSection{conntrackSection(data)}, running)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Helpers) != 0 || len(cfg.HelperTables) != 0 {
				t.Fatalf("helpers %v, tables %v: want none", cfg.Helpers, tableNames(cfg.HelperTables))
			}
		})
	}
}

// VALIDATES: a reload hands the engine only the roots that changed, so the
// root a delivery does not carry keeps its running value.
// PREVENTS: a firewall-only edit withdrawing the helpers, or a conntrack-only
// edit withdrawing every firewall table.
func TestConntrackSectionUndeliveredKeepsRunningValue(t *testing.T) {
	running := &firewallConfig{
		Backend:    "nft",
		Configured: true,
		Tables:     []Table{makeTable(FamilyInet)},
	}
	withHelpers, err := parseConfigSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["ftp","pptp"]}}}`),
	}, running)
	if err != nil {
		t.Fatal(err)
	}
	if len(withHelpers.Tables) != 1 || !withHelpers.Configured {
		t.Fatalf("a conntrack-only delivery dropped the running firewall tables: %+v", withHelpers)
	}
	if got := tableNames(withHelpers.HelperTables); !slices.Equal(got, []string{conntrackHelperTableInet, conntrackHelperTableIP}) {
		t.Fatalf("helper tables %v", got)
	}

	firewallOnly, err := parseConfigSections([]sdk.ConfigSection{
		{Root: configRootFirewall, Data: `{"firewall":{"backend":"nft"}}`},
	}, withHelpers)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(firewallOnly.Helpers, []string{"ftp", "pptp"}) || len(firewallOnly.HelperTables) != 2 {
		t.Fatalf("a firewall-only delivery dropped the running helpers: %v", firewallOnly.Helpers)
	}
	if len(firewallOnly.Tables) != 0 {
		t.Fatalf("the delivered firewall section lists no table, got %d", len(firewallOnly.Tables))
	}
}

// VALIDATES: spec-home-cpe-3 AC-8 at the component -- verify runs the
// selected backend's verifier over the helper tables, so a backend with no
// conntrack helper refuses at verify, before any apply.
// PREVENTS: I-1, a refusal that only fired after commit as a log line.
func TestVerifyRunsBackendVerifierOverHelperTables(t *testing.T) {
	const backendName = "no-ct-helper"
	helperBackend(t, backendName)
	refused := errors.New("conntrack helpers not supported by backend no-ct-helper")
	if err := RegisterVerifier(backendName, func(tables []Table) error {
		for i := range tables {
			if len(tables[i].CtHelpers) > 0 {
				return refused
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	running := &firewallConfig{Backend: backendName, Configured: true}

	_, err := parseAndVerifyFirewallSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["sip"]}}}`),
	}, running)
	if !errors.Is(err, refused) {
		t.Fatalf("verify answered %v, want the backend's refusal", err)
	}

	if _, err := parseAndVerifyFirewallSections([]sdk.ConfigSection{conntrackSection(`{}`)}, running); err != nil {
		t.Fatalf("withdrawing every helper must verify: %v", err)
	}
}

// VALIDATES: registerConfigTables installs and withdraws the helper owner,
// and show system conntrack reads what the backend last applied, so a closed
// backend lists no helper.
// PREVENTS: N-4, a helper list that outlived the tables it described.
func TestHelperTablesInstallWithdrawAndShow(t *testing.T) {
	_, applied := helperBackend(t, "helper-apply")
	cfg, err := parseConfigSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["ftp"]}}}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := registerConfigTables(cfg); err != nil {
		t.Fatal(err)
	}
	if got := tableNames(*applied); !slices.Equal(got, []string{conntrackHelperTableInet}) {
		t.Fatalf("applied %v", got)
	}
	rows := assignedConntrackHelpers()
	if len(rows) != 1 || rows[0]["module"] != "ftp" || rows[0]["kernel-helper"] != "ftp" {
		t.Fatalf("show rows %v", rows)
	}
	ports, ok := rows[0]["ports"].([]int)
	if !ok || !slices.Equal(ports, []int{21}) {
		t.Fatalf("show ports %v, want [21]", rows[0]["ports"])
	}

	if err := registerConfigTables(&firewallConfig{}); err != nil {
		t.Fatal(err)
	}
	if len(*applied) != 0 || len(assignedConntrackHelpers()) != 0 {
		t.Fatalf("withdraw left %v, rows %v", tableNames(*applied), assignedConntrackHelpers())
	}

	if err := registerConfigTables(cfg); err != nil {
		t.Fatal(err)
	}
	if err := CloseBackend(); err != nil {
		t.Fatal(err)
	}
	if rows := assignedConntrackHelpers(); len(rows) != 0 {
		t.Fatalf("a closed backend still lists helpers: %v", rows)
	}
}

// VALIDATES: I-2 -- a reconcile that fails while helpers are listed names the
// helpers and the two fix commands, after the backend's own error.
func TestHelperApplyFailureNamesTheFix(t *testing.T) {
	b, _ := helperBackend(t, "helper-refuses")
	b.applyErr = errors.New("netlink: no such file or directory")
	cfg, err := parseConfigSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["netbios-ns"]}}}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = registerConfigTables(cfg)
	if !errors.Is(err, b.applyErr) {
		t.Fatalf("error %v does not wrap the backend's", err)
	}
	for _, want := range []string{
		"conntrack helpers listed: netbios-ns",
		"modprobe nf_conntrack_netbios_ns",
		"delete system conntrack module netbios-ns",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

// VALIDATES: I-3 -- a clean stop without a `firewall {}` block withdraws the
// engine's own owners and leaves another owner's tables, while a stop with
// one flushes every ze-owned table as before.
// PREVENTS: removing the `system conntrack` block stopping the engine and
// flushing copp's tables with it.
func TestFlushOnStopScopedToTheEngineWithoutFirewallBlock(t *testing.T) {
	_, applied := helperBackend(t, "helper-stop")
	other := makeTable(FamilyInet)
	other.Name = tableNamePrefix + "copp"
	if err := RegisterTables("copp", []Table{other}); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfigSections([]sdk.ConfigSection{
		conntrackSection(`{"system":{"conntrack":{"module":["tftp"]}}}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := registerConfigTables(cfg); err != nil {
		t.Fatal(err)
	}

	if err := flushOnStop(cfg); err != nil {
		t.Fatal(err)
	}
	if got := tableNames(*applied); !slices.Equal(got, []string{other.Name}) {
		t.Fatalf("stop without a firewall block left %v, want only %s", got, other.Name)
	}

	if err := flushOnStop(&firewallConfig{Configured: true}); err != nil {
		t.Fatal(err)
	}
	if len(*applied) != 0 {
		t.Fatalf("stop with a firewall block left %v", tableNames(*applied))
	}
}
