// Design: docs/architecture/firewall/firewall-domain-group.md -- a failed kernel query is not proof that a table disappeared.
// Related: netfilter_fixture_domain_group_restart.go -- the stop barrier before a second daemon start.
// Related: netfilter_fixture_domain_group.go -- the clear observer's kernel absence assertion.

package fixture

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDomainGroupTableInventoryAbsence reads an explicit `nft list tables`
// inventory. Only a missing exact inet table is absent: another family and a
// name that merely shares its prefix must not stand for the table the
// observer names.
func TestDomainGroupTableInventoryAbsence(t *testing.T) {
	const table = "ze_group_inventory"
	for _, tc := range []struct {
		name   string
		body   string
		absent bool
	}{
		{
			name:   "empty inventory",
			body:   "",
			absent: true,
		},
		{
			name:   "blank lines only",
			body:   "\n\n",
			absent: true,
		},
		{
			name: "exact table present",
			body: "table inet ze_group_inventory\n",
		},
		{
			name: "exact table present without final newline",
			body: "table inet ze_group_inventory",
		},
		{
			name:   "other family",
			body:   "table ip ze_group_inventory\n",
			absent: true,
		},
		{
			name:   "similar name",
			body:   "table inet ze_group_inventory_old\n",
			absent: true,
		},
		{
			name: "table after unrelated tables",
			body: "table inet le_test_conntrack\ntable inet ze_other\ntable inet ze_group_inventory\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			absent, err := domainGroupTableInventoryAbsent(tc.body, table)
			require.NoError(t, err)
			require.Equal(t, tc.absent, absent)
		})
	}
}

// TestDomainGroupTableInventoryRefusesUnknown ensures a malformed inventory
// cannot satisfy the shutdown or clear barrier. Every refusal must retain an
// error and must not report the table absent.
func TestDomainGroupTableInventoryRefusesUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"cut short after family", "table inet\n"},
		{"cut short after keyword", "table\n"},
		{"extra field", "table inet ze_group_inventory extra\n"},
		{"not a table", "chain inet ze_group_inventory\n"},
		{"JSON instead of text", `{"nftables":[]}`},
		{"error text on stdout", "Error: JSON support not compiled-in\n"},
		{"malformed line after a valid one", "table inet ze_other\ntable inet\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			absent, err := domainGroupTableInventoryAbsent(tc.body, "ze_group_inventory")
			require.Error(t, err)
			require.False(t, absent, "an unreadable kernel inventory is not absence")
		})
	}
}

// TestDomainGroupTableAbsenceRefusesCanceledQuery drives the command wrapper
// with a canceled context. The failed query must remain an error rather than
// becoming the successful table removal both observers wait for.
func TestDomainGroupTableAbsenceRefusesCanceledQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	absent, err := domainGroupTableAbsent(ctx, "ze_group_inventory")
	require.Error(t, err)
	require.False(t, absent)
}
