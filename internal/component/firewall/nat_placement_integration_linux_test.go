//go:build linux && integration

// Design: docs/guide/firewall.md -- nft NAT placement at configuration verify
// Related: nat_placement_test.go -- the same configured input is judged offline
//
// The integration gate judges the matrix against the nftables kernel in Ze's
// guest, with every case starting from an empty ruleset in a private namespace.

package firewall_test

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft" // Registers the real backend.
	"github.com/ze-software/ze/internal/test/userns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestNftNATPlacementIntegration(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	conn, err := nftables.New()
	require.NoError(t, err)

	// Enter re-executes only this test. The observer lives in that child and
	// sees entry to the production backend apply, even if a commit fails.
	var applies int
	firewall.RegisterCommitObserver(func() func(error) {
		applies++
		return nil
	})
	for _, test := range nftNATPlacementCases() {
		t.Run(test.name, func(t *testing.T) {
			before := applies
			err := firewall.VerifyConfig([]sdk.ConfigSection{{Root: "firewall", Data: test.data}})
			require.Equal(t, before, applies, "configuration verify reached backend apply")
			require.Nil(t, firewall.GetBackend(), "configuration verify loaded a backend")
			tables, listErr := conn.ListTables()
			require.NoError(t, listErr)
			require.Empty(t, tables, "configuration verify changed the isolated nft ruleset")
			if test.want != nil {
				require.Error(t, err)
				for _, fragment := range test.want {
					require.Contains(t, strings.ToLower(err.Error()), fragment)
				}
				return
			}
			require.NoError(t, err)

			configured, err := firewall.ParseFirewallConfig(test.data)
			require.NoError(t, err)
			require.Len(t, configured, 1)
			require.Equal(t, "ze_placement", configured[0].Name)
			const owner = "nat-placement-integration"
			require.NoError(t, firewall.LoadBackend("nft"))
			t.Cleanup(func() {
				require.NoError(t, errors.Join(
					firewall.RegisterTables(owner, nil),
					firewall.ApplyAll(),
					firewall.CloseBackend(),
				))
			})
			require.NoError(t, firewall.RegisterTables(owner, configured))
			require.NoError(t, firewall.ApplyAll())
			require.Equal(t, before+1, applies)
			requireNftPlacementTable(t, conn, configured[0])
		})
	}
}

func requireNftPlacementTable(t *testing.T, conn *nftables.Conn, want firewall.Table) {
	t.Helper()
	tables, err := conn.ListTables()
	require.NoError(t, err)
	require.Len(t, tables, 1)
	table := tables[0]
	families := map[firewall.TableFamily]nftables.TableFamily{
		firewall.FamilyIP:   nftables.TableFamilyIPv4,
		firewall.FamilyIP6:  nftables.TableFamilyIPv6,
		firewall.FamilyInet: nftables.TableFamilyINet,
	}
	require.Contains(t, families, want.Family)
	require.Equal(t, families[want.Family], table.Family)
	require.Equal(t, want.Name, table.Name)

	chains, err := conn.ListChainsOfTableFamily(table.Family)
	require.NoError(t, err)
	require.NotEmpty(t, want.Chains)
	require.Len(t, chains, len(want.Chains))
	byName := make(map[string]*nftables.Chain, len(chains))
	for _, chain := range chains {
		require.NotNil(t, chain.Table)
		require.Equal(t, table.Name, chain.Table.Name)
		byName[chain.Name] = chain
	}
	hooks := map[firewall.ChainHook]*nftables.ChainHook{
		firewall.HookInput:       nftables.ChainHookInput,
		firewall.HookOutput:      nftables.ChainHookOutput,
		firewall.HookPrerouting:  nftables.ChainHookPrerouting,
		firewall.HookPostrouting: nftables.ChainHookPostrouting,
	}
	for _, expected := range want.Chains {
		chain := byName[expected.Name]
		require.NotNil(t, chain, "missing configured chain %s", expected.Name)
		require.Equal(t, expected.IsBase, chain.Hooknum != nil, expected.Name)
		if expected.IsBase {
			require.Contains(t, hooks, expected.Hook)
			require.Equal(t, hooks[expected.Hook], chain.Hooknum)
			require.Equal(t, nftables.ChainType(expected.Type.String()), chain.Type)
			require.NotNil(t, chain.Priority)
			require.Equal(t, expected.Priority, int32(*chain.Priority))
		}
		rules, err := conn.GetRules(table, chain)
		require.NoError(t, err)
		require.Len(t, rules, len(expected.Terms), expected.Name)
		for i, term := range expected.Terms {
			rule := rules[i]
			require.Equal(t, term.Name, string(rule.UserData))
			require.Len(t, term.Actions, 1)
			requireNftPlacementAction(t, rule.Exprs, term.Actions[0])
		}
	}
}

func requireNftPlacementAction(t *testing.T, expressions []expr.Any, want firewall.Action) {
	t.Helper()
	require.NotEmpty(t, expressions)
	last := expressions[len(expressions)-1]
	switch action := want.(type) {
	case firewall.SNAT:
		requireNftPlacementTarget(t, expressions, expr.NATTypeSourceNAT, action.Address)
	case firewall.DNAT:
		requireNftPlacementTarget(t, expressions, expr.NATTypeDestNAT, action.Address)
	case firewall.Masquerade:
		require.IsType(t, &expr.Masq{}, last)
	case firewall.Redirect:
		redirect, ok := last.(*expr.Redir)
		require.True(t, ok, "expected redirect, got %T", last)
		port := binary.BigEndian.AppendUint16(nil, action.Port)
		requireNftPlacementImmediate(t, expressions, redirect.RegisterProtoMin, port)
	case firewall.Accept:
		require.Equal(t, &expr.Verdict{Kind: expr.VerdictAccept}, last)
	case firewall.Jump:
		require.Equal(t, &expr.Verdict{Kind: expr.VerdictJump, Chain: action.Target}, last)
	case firewall.Goto:
		require.Equal(t, &expr.Verdict{Kind: expr.VerdictGoto, Chain: action.Target}, last)
	default:
		t.Fatalf("unsupported placement fixture action %T", want)
	}
}

func requireNftPlacementTarget(t *testing.T, expressions []expr.Any, kind expr.NATType, address netip.Addr) {
	t.Helper()
	nat, ok := expressions[len(expressions)-1].(*expr.NAT)
	require.True(t, ok, "expected NAT, got %T", expressions[len(expressions)-1])
	require.Equal(t, kind, nat.Type)
	family := uint32(nftables.TableFamilyIPv6)
	if address.Is4() {
		family = uint32(nftables.TableFamilyIPv4)
	}
	require.Equal(t, family, nat.Family)
	requireNftPlacementImmediate(t, expressions, nat.RegAddrMin, address.AsSlice())
}

func requireNftPlacementImmediate(t *testing.T, expressions []expr.Any, register uint32, data []byte) {
	t.Helper()
	found := slices.ContainsFunc(expressions, func(expression expr.Any) bool {
		immediate, ok := expression.(*expr.Immediate)
		return ok && immediate.Register == register && slices.Equal(immediate.Data, data)
	})
	require.True(t, found, "readback is missing immediate %x for register %d", data, register)
}
