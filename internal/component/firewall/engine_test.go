// Design: docs/architecture/core-design.md -- Firewall plugin engine (SDK 5-stage)
// Related: engine.go -- validateBackendGate under test

package firewall

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// swapBackendGateSchema replaces the cached gate schema with the supplied
// Schema. The sync.Once guarding the lazy YANG load is reset and then marked
// Done so validateBackendGate bypasses the real load and uses the override.
// t.Cleanup clears the cache so subsequent tests re-load the real YANG.
//
// NOT safe for t.Parallel(): mutates package-level state (backendGateSchema,
// backendGateSchemaErr, backendGateSchemaOnce).
func swapBackendGateSchema(t *testing.T, s *config.Schema) {
	t.Helper()
	backendGateSchema = s
	backendGateSchemaErr = nil
	backendGateSchemaOnce = sync.Once{}
	backendGateSchemaOnce.Do(func() {})
	t.Cleanup(func() {
		backendGateSchema = nil
		backendGateSchemaErr = nil
		backendGateSchemaOnce = sync.Once{}
	})
}

// VALIDATES: validateBackendGate rejects a firewall section when the active
// backend is "" (the non-Linux default). The walker's empty-backend guard
// fires regardless of YANG annotations and the rejection names the backend
// leaf path so operators know where to set it.
//
// PREVENTS: A non-Linux deployment silently accepting a firewall config that
// could never be programmed, with the Apply-time failure surfacing only after
// commit.
func TestFirewallBackendGateRejects_EmptyBackend(t *testing.T) {
	data := `{"firewall":{"table":{"t1":{"name":"t1","family":"ip"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "")
	require.Error(t, err, "empty backend must be rejected")
	assert.Contains(t, err.Error(), backendLeafPath,
		"rejection must name /firewall/backend so operators know where to set it")
}

// VALIDATES: validateBackendGate aggregates diagnostics for every YANG node
// whose ze:backend annotation excludes the active backend. Synthetic schema
// annotates the `table` list `ze:backend "nft"` and the active backend is
// "vpp", so the walker emits a diagnostic naming the path and backends.
//
// PREVENTS: Wiring regression that disables the feature gate. If the walker
// were bypassed or the annotation stopped flowing onto the schema node, the
// annotated node would no longer produce a diagnostic and this test would
// catch it.
func TestFirewallBackendGateRejects_Synthetic(t *testing.T) {
	list := config.List(config.TypeString,
		config.Field("name", config.Leaf(config.TypeString)),
	)
	list.KeyName = "name"
	list.Backend = []string{"nft"}

	synthetic := config.NewSchema()
	synthetic.Define(configRootFirewall, config.Container(
		config.Field("backend", config.Leaf(config.TypeString)),
		config.Field("table", list),
	))

	swapBackendGateSchema(t, synthetic)

	data := `{"firewall":{"backend":"vpp","table":{"t1":{"name":"t1"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "vpp")
	require.Error(t, err, "synthetic schema must reject vpp backend on nft-annotated list")
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "/firewall/table"),
		"rejection must name the /firewall/table YANG path, got: %s", msg)
	assert.Contains(t, msg, `"vpp"`, "rejection must name the active backend")
	assert.Contains(t, msg, "nft", "rejection must list supporting backends")
}

// VALIDATES: validateBackendGate accepts a firewall section when the active
// backend matches every ze:backend annotation on the real schema. Today all
// seven annotations are `ze:backend "nft"`, so backend="nft" with a minimal
// firewall payload must return nil.
//
// PREVENTS: False positives if the walker ever learns to reject a config on
// an annotation that names the active backend (the "narrowest wins" and
// "absent annotation = unrestricted" rules must keep holding).
func TestFirewallBackendGateAccepts_NftBackend(t *testing.T) {
	data := `{"firewall":{"table":{"t1":{"name":"t1","family":"ip"}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	err := validateBackendGate(sections, "nft")
	require.NoError(t, err, "nft backend must satisfy every ze:backend \"nft\" annotation")
}

// VALIDATES: extractFlushOnShutdown parses the `flush-on-shutdown` leaf. Absent
// leaf, empty data, and no firewall section all yield the fail-safe default
// (true = flush on clean shutdown); an explicit false disables it; both JSON
// bool and the CLI/YANG string encoding are accepted; a non-bool value is
// rejected so a typo cannot silently flip the shutdown behavior.
// PREVENTS: a mis-encoded flush-on-shutdown silently defaulting a firewall to
// leave-on-shutdown (or the reverse) without the operator noticing.
func TestExtractFlushOnShutdown(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    bool
		wantErr bool
	}{
		{name: "empty data defaults true", data: "", want: true},
		{name: "no firewall section defaults true", data: `{"other":{}}`, want: true},
		{name: "absent leaf defaults true", data: `{"firewall":{"backend":"nft"}}`, want: true},
		{name: "json bool true", data: `{"firewall":{"flush-on-shutdown":true}}`, want: true},
		{name: "json bool false", data: `{"firewall":{"flush-on-shutdown":false}}`, want: false},
		{name: "string true", data: `{"firewall":{"flush-on-shutdown":"true"}}`, want: true},
		{name: "string false", data: `{"firewall":{"flush-on-shutdown":"false"}}`, want: false},
		{name: "null leaf defaults true", data: `{"firewall":{"flush-on-shutdown":null}}`, want: true},
		{name: "bogus string rejected", data: `{"firewall":{"flush-on-shutdown":"maybe"}}`, wantErr: true},
		{name: "numeric rejected", data: `{"firewall":{"flush-on-shutdown":1}}`, wantErr: true},
		{name: "malformed json rejected", data: `{"firewall":`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFlushOnShutdown(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: AC-1 -- the guide's IRR table term survives the verify path AND
// the registry merge the configure path performs. parseAndVerifyFirewallSections
// is what OnConfigVerify calls; RegisterTables plus ApplyAll is what
// OnConfigure calls next. The set the term names arrives from the firewall-irr
// owner, so the backend receives one table carrying both the term and the set.
// PREVENTS: validation accepting a reference the merge cannot resolve, which
// would move a commit-time rejection to an apply-time failure of every owner's
// ruleset.
func TestConfigureAcceptsIRRTableTerm(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	data := `{"firewall":{"backend":"nft","table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop","term":{"from-cloudflare":{"from":{"source-asn":"13335"},"then":{"accept":""}}}}}}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	cfg, err := parseAndVerifyFirewallSections(sections)
	require.NoError(t, err, "an IRR table term must pass the verify path")
	require.Len(t, cfg.Tables, 1)

	var applied []Table
	require.NoError(t, RegisterBackend("irr-merge-nft", func() (Backend, error) {
		return &countingBackend{onApply: func(d []Table) { applied = d }}, nil
	}))
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "irr-merge-nft"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	_ = RegisterTables("firewall", cfg.Tables)
	// What the firewall-irr owner registers once the prefixes are cached:
	// the same table name and family, carrying only the sets.
	_ = RegisterTables("firewall-irr", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Sets: []Set{
			{Name: "irr_v4_AS13335", Type: SetTypeIPv4, Flags: SetFlagInterval},
			{Name: "irr_v6_AS13335", Type: SetTypeIPv6, Flags: SetFlagInterval},
		},
	}})
	require.NoError(t, ApplyAll(), "the merged ruleset must apply")

	require.Len(t, applied, 1, "both owners must merge into one table")
	merged := applied[0]
	declared := make(map[string]SetType, len(merged.Sets))
	for _, s := range merged.Sets {
		declared[s.Name] = s.Type
	}
	require.Len(t, merged.Chains, 1)
	for _, term := range merged.Chains[0].Terms {
		for _, m := range term.Matches {
			in, ok := m.(MatchInSet)
			if !ok {
				continue
			}
			got, ok := declared[in.SetName]
			assert.True(t, ok, "term %q references set %q, which the merged table does not declare", term.Name, in.SetName)
			assert.Equal(t, in.ProvidedType, got, "set %q type disagrees with what the match expects", in.SetName)
		}
	}
	assert.Len(t, merged.Chains[0].Terms, 2, "the v4 term and its v6 twin must both reach the backend")
}

// VALIDATES: AC-1, AC-2 -- an ASN or AS-SET announcing ONE family still lands
// the operator's table in the kernel. It drives the merge from the same entry
// point as TestConfigureAcceptsIRRTableTerm. The registered tables are what the
// firewall-irr owner builds for an IPv4-only entry: both set names, the IPv6
// one empty (buildTermSets,
// internal/component/firewall/plugins/irr/sets.go).
// PREVENTS: the common case losing the whole table. expandIRRTermV6
// (config.go) emits the IPv6 twin for every IRR term, because the parser
// cannot see the prefix data. A v4-only entry therefore left the twin naming
// an undeclared set. dropTablesMissingAProvidedSet (registry.go) then removed
// the operator's ENTIRE table while the commit reported success.
func TestConfigureAcceptsIRRTableTermWithOneFamily(t *testing.T) {
	resetBackends()
	resetTables()
	t.Cleanup(func() {
		resetBackends()
		resetTables()
	})

	data := `{"firewall":{"backend":"nft","table":{"wan":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"drop","term":{"from-cloudflare":{"from":{"source-asn":"13335"},"then":{"accept":""}}}}}}}}}`
	sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: data}}

	cfg, err := parseAndVerifyFirewallSections(sections)
	require.NoError(t, err, "an IRR table term must pass the verify path")

	var applied []Table
	require.NoError(t, RegisterBackend("irr-one-family-nft", func() (Backend, error) {
		return &countingBackend{onApply: func(d []Table) { applied = d }}, nil
	}))
	prev := defaultBackendForAutoload
	defaultBackendForAutoload = "irr-one-family-nft"
	t.Cleanup(func() { defaultBackendForAutoload = prev })

	_ = RegisterTables("firewall", cfg.Tables)
	// What buildTermSets produces for an entry holding IPv4 prefixes only: the
	// IPv6 set is declared and carries no element, so its term matches nothing.
	_ = RegisterTables("firewall-irr", []Table{{
		Name:   "ze_wan",
		Family: FamilyInet,
		Sets: []Set{
			{Name: "irr_v4_AS13335", Type: SetTypeIPv4, Flags: SetFlagInterval, Elements: []SetElement{{Value: "1.1.1.0"}, {Value: "1.1.2.0", IntervalEnd: true}}},
			{Name: "irr_v6_AS13335", Type: SetTypeIPv6, Flags: SetFlagInterval},
		},
	}})
	require.NoError(t, ApplyAll(), "the merged ruleset must apply")

	require.Len(t, applied, 1, "an IPv4-only entry must not cost the operator the whole table")
	merged := applied[0]
	declared := make(map[string]SetType, len(merged.Sets))
	for _, s := range merged.Sets {
		declared[s.Name] = s.Type
	}
	require.Len(t, merged.Chains, 1)
	assert.Len(t, merged.Chains[0].Terms, 2, "the v4 term and its v6 twin must both reach the backend")
	for _, term := range merged.Chains[0].Terms {
		for _, m := range term.Matches {
			in, ok := m.(MatchInSet)
			if !ok {
				continue
			}
			got, ok := declared[in.SetName]
			assert.True(t, ok, "term %q references set %q, which the merged table does not declare", term.Name, in.SetName)
			assert.Equal(t, in.ProvidedType, got, "set %q type disagrees with what the match expects", in.SetName)
		}
	}
}
