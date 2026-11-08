package iface

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/ndp"
)

// TestDelegationsStatusOf checks what `show interface pppoe-client` reports
// under delegated-prefixes. Method: record delegations at a fixed receive
// time and read them 100 seconds later. Proves the lifetimes are what remains
// rather than what the Reply carried, the preferred one is bounded by the
// valid one, infinite stays infinite, an expired prefix is not listed, the
// list is sorted by prefix, and a source with nothing answers an empty list.
func TestDelegationsStatusOf(t *testing.T) {
	withCleanDelegations(t)
	received := time.Unix(1_800_000_000, 0)
	later := received.Add(100 * time.Second)
	heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:bb00::/56"), received, 3600, 1800)
	heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:aa00::/56"), received, lifetimeInfiniteSeconds, lifetimeInfiniteSeconds)
	heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:cc00::/60"), received, 60, 30)
	heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:dd00::/56"), received, 600, 7200)

	got := heldDelegations.statusOf("pppoe0", later)
	want := []DelegatedPrefixStatus{
		{Prefix: netip.MustParsePrefix("2001:db8:aa00::/56"), ValidLifetime: ndp.LifetimeInfinity, PreferredLifetime: ndp.LifetimeInfinity},
		{Prefix: netip.MustParsePrefix("2001:db8:bb00::/56"), ValidLifetime: 3500, PreferredLifetime: 1700},
		{Prefix: netip.MustParsePrefix("2001:db8:dd00::/56"), ValidLifetime: 500, PreferredLifetime: 500},
	}
	assert.Equal(t, want, got, "remaining lifetimes, sorted, the expired /60 left out")

	none := heldDelegations.statusOf("pppoe1", later)
	require.NotNil(t, none, "a source with no delegation answers a list, never null")
	b, err := json.Marshal(none)
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(b))
}

// TestPPPoEClientStatusDelegatedPrefixes checks the key the operator reads on
// the WAN side. Method: marshal a client's status before and after its lease
// delegates a prefix. Proves delegated-prefixes is an empty list, not absent
// and not null, while nothing is delegated, and that a delegation appears
// under the client's own name with its prefix and remaining lifetimes.
func TestPPPoEClientStatusDelegatedPrefixes(t *testing.T) {
	withCleanDelegations(t)
	client := NewPPPoEClient(PPPoEClientConfig{Name: "pppoe0"}, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var before map[string]json.RawMessage
	b, err := json.Marshal(client.status())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &before))
	require.Contains(t, before, "delegated-prefixes")
	assert.JSONEq(t, `[]`, string(before["delegated-prefixes"]))

	heldDelegations.record("pppoe0", netip.MustParsePrefix("2001:db8:aa00::/56"), time.Now(), 3600, 1800)
	heldDelegations.record("pppoe1", netip.MustParsePrefix("2001:db8:ff00::/56"), time.Now(), 3600, 1800)
	status := client.status()
	require.Len(t, status.DelegatedPrefixes, 1, "only pppoe0's own lease is reported")
	got := status.DelegatedPrefixes[0]
	assert.Equal(t, netip.MustParsePrefix("2001:db8:aa00::/56"), got.Prefix)
	assert.LessOrEqual(t, got.ValidLifetime, uint32(3600))
	assert.Greater(t, got.ValidLifetime, uint32(3590))
	assert.LessOrEqual(t, got.PreferredLifetime, uint32(1800))
	assert.Greater(t, got.PreferredLifetime, uint32(1790))

	b, err = json.Marshal(status)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"delegated-prefixes":[{"prefix":"2001:db8:aa00::/56","valid-lifetime":`)
	assert.Contains(t, string(b), `"preferred-lifetime":`)
}

// TestDelegatedUnitRows checks every state `show interface delegated-prefix`
// reports for a LAN unit. Method: two units number subnet ids 1 and 2 from
// pppoe0; the worker state is set by hand to each outcome the worker can
// leave, and the rows are read and answered. Proves the derived /64, its
// address, its source and its remaining lifetimes on an installed unit, and
// that a unit with no address says why instead of answering zeros.
func TestDelegatedUnitRows(t *testing.T) {
	withCleanDelegations(t)
	cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "1"}}`, `{"pppoe0": {"subnet-id": "2"}}`))
	keyLan0 := delegatedUnitKey{ifaceName: "lan0", unit: "0", source: "pppoe0"}
	keyLan1 := delegatedUnitKey{ifaceName: "lan1", unit: "0", source: "pppoe0"}
	received := time.Unix(1_800_000_000, 0)
	now := received.Add(600 * time.Second)

	t.Run("no delegation held: both wait, with a reason and no lifetimes", func(t *testing.T) {
		rows := delegatedUnitRows(cfg, newDelegatedState(), now)
		answer, err := delegatedPrefixAnswer(&rows, now)
		require.NoError(t, err)
		require.Len(t, answer, 2)
		assert.Equal(t, "lan0", answer[0].Interface)
		assert.Equal(t, "lan1", answer[1].Interface)
		for _, row := range answer {
			assert.Equal(t, "waiting", row.State)
			assert.Equal(t, "pppoe0", row.Source)
			assert.Contains(t, row.Reason, "pppoe-client pppoe0 holds no delegated prefix")
			b, err := json.Marshal(row)
			require.NoError(t, err)
			for _, absent := range []string{`"address"`, `"prefix"`, `"delegated"`, `"valid-lifetime"`, `"preferred-lifetime"`} {
				assert.NotContains(t, string(b), absent, "a waiting unit has no value to report for %s", absent)
			}
		}
	})

	delegated := netip.MustParsePrefix("2001:db8:aa00::/56")
	heldDelegations.record("pppoe0", delegated, received, 3600, 1800)
	state := newDelegatedState()
	state.installed[keyLan0] = delegatedInstall{device: "lan0", advert: DelegatedAdvert{
		Source:    "pppoe0",
		Address:   netip.MustParsePrefix("2001:db8:aa00:1::1/64"),
		Delegated: delegated,
		Valid:     newLifetimeDeadline(received, 3600),
		Preferred: newLifetimeDeadline(received, 1800),
	}}
	state.refused[keyLan1] = delegated

	t.Run("installed unit reports its /64, address, source and remaining lifetimes", func(t *testing.T) {
		rows := delegatedUnitRows(cfg, state, now)
		answer, err := delegatedPrefixAnswer(&rows, now)
		require.NoError(t, err)
		require.Len(t, answer, 2)
		b, err := json.Marshal(answer[0])
		require.NoError(t, err)
		assert.JSONEq(t, `{"interface":"lan0","unit":"0","source":"pppoe0","subnet-id":1,"state":"installed",
			"device":"lan0","address":"2001:db8:aa00:1::1/64","prefix":"2001:db8:aa00:1::/64",
			"delegated":"2001:db8:aa00::/56","valid-lifetime":3000,"preferred-lifetime":1200}`, string(b))
	})

	t.Run("refused unit is too small, names the delegation and the fix", func(t *testing.T) {
		rows := delegatedUnitRows(cfg, state, now)
		row := delegatedUnitStatusOf(&rows[1], now)
		assert.Equal(t, "too-small", row.State)
		assert.Equal(t, uint16(2), row.SubnetID)
		assert.Equal(t, delegated, row.Delegated)
		assert.Nil(t, row.ValidLifetime)
		assert.Contains(t, row.Reason, "set interface lan1 unit 0 ipv6 delegated-prefix pppoe0 subnet-id")
	})

	t.Run("a refusal for another delegation does not stand: the install failed", func(t *testing.T) {
		state.refused[keyLan1] = netip.MustParsePrefix("2001:db8:ee00::/60")
		rows := delegatedUnitRows(cfg, state, now)
		row := delegatedUnitStatusOf(&rows[1], now)
		assert.Equal(t, "not-installed", row.State)
		assert.Equal(t, delegated, row.Delegated)
		assert.Contains(t, row.Reason, "install")
	})
}

// TestDelegatedPrefixCommandRefusesWithoutWorker checks that a process with no
// delegation worker refuses rather than answering an empty list, which would
// read as "no unit numbers from a delegated prefix". Method: answer with no
// published rows, directly and through executeCommand.
func TestDelegatedPrefixCommandRefusesWithoutWorker(t *testing.T) {
	saved := globalDelegatedUnits.Load()
	t.Cleanup(func() { globalDelegatedUnits.Store(saved) })
	globalDelegatedUnits.Store(nil)

	_, err := delegatedPrefixAnswer(nil, time.Now())
	assert.True(t, errors.Is(err, errDelegationWorkerNotRunning))

	status, _, err := executeCommand(strings.ToUpper(DelegatedPrefixCommand), nil)
	assert.Equal(t, "error", status)
	assert.True(t, errors.Is(err, errDelegationWorkerNotRunning))
}

// TestDelegationWorkerPublishesRows checks the wiring: the worker that numbers
// units is the one that publishes the rows the command answers, after each
// pass, and withdraws them when it stops. Method: run the worker over a config
// whose unit numbers from pppoe0, which holds nothing, wake it, and read the
// published rows from the refreshRA callback that follows the pass.
func TestDelegationWorkerPublishesRows(t *testing.T) {
	withCleanDelegations(t)
	saved := globalDelegatedUnits.Load()
	t.Cleanup(func() { globalDelegatedUnits.Store(saved) })

	cfg := mustParseIfaceJSON(t, delegatedUnitJSON(`{"pppoe0": {"subnet-id": "1"}}`, `{"pppoe0": {"subnet-id": "2"}}`))
	var active atomic.Pointer[ifaceConfig]
	active.Store(cfg)
	q := newDelegationQueue()
	stop := make(chan struct{})
	done := make(chan struct{})
	passed := make(chan []DelegatedUnitStatus, 1)
	refresh := func() {
		rows, err := delegatedPrefixAnswer(globalDelegatedUnits.Load(), time.Now())
		if err != nil {
			t.Errorf("rows not published before refreshRA: %v", err)
		}
		passed <- rows
	}
	go runDelegationWorker(q, stop, done, &active, refresh, slog.New(slog.NewTextHandler(io.Discard, nil)))

	q.wake()
	select {
	case rows := <-passed:
		require.Len(t, rows, 2)
		assert.Equal(t, "waiting", rows[0].State)
	case <-time.After(5 * time.Second):
		t.Fatal("the worker made no pass after a wake")
	}

	close(stop)
	<-done
	assert.Nil(t, globalDelegatedUnits.Load(), "a stopped worker's rows are withdrawn")
}
