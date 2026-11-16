// Design: docs/architecture/irr-owner.md -- a refused IRR commit must leave its table out of the kernel.
// Related: plugin_fixture_07_irr.go -- firewallIRRRefusedCommit07, the observer under test.
// Related: netfilter_fixture_domain_group_inventory.go -- an answered kernel inventory proves absence.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// firewallIRRRefusalProbe07 supplies independent command and kernel answers.
// A failed daemon command says nothing about the kernel inventory.
type firewallIRRRefusalProbe07 struct {
	table           string
	expectedRefusal string
	inventory       string
	stderr          string
	exitCode        int
	response        *rpc.DispatchCommandOutput
	commandErr      error
	errorsCommand   func(context.Context) (*rpc.DispatchCommandOutput, error)
}

// runFirewallIRRRefusalProbe07 drives the real observer with its reload marker
// already present. The SDK bridge supplies the daemon command answer; a fake nft
// supplies the independent kernel inventory and records the query it received.
// No daemon, kernel privileges, fixed port or timing assertion is involved.
func runFirewallIRRRefusalProbe07(t *testing.T, probe firewallIRRRefusalProbe07) (string, error) {
	t.Helper()
	run, query := prepareFirewallIRRRefusalProbe07(t, probe)
	err := run(t.Context())
	return query(), err
}

func prepareFirewallIRRRefusalProbe07(t *testing.T, probe firewallIRRRefusalProbe07) (func(context.Context) error, func() string) {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("reload.done", nil, 0o600))
	bin := t.TempDir()
	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	}
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" > nft.query
if [ "$#" -ne 2 ]; then exit 97; fi
if [ "$1" != list ]; then exit 97; fi
if [ "$2" != tables ]; then exit 97; fi
printf '%%s' %s
printf '%%s' %s >&2
exit %d
`, quote(probe.inventory), quote(probe.stderr), probe.exitCode)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "nft"), []byte(script), 0o700))
	t.Setenv("PATH", bin)

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	plugin := sdk.NewWithConn("irr-refusal-probe", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() {
		require.NoError(t, plugin.Close())
		require.NoError(t, engineEnd.Close())
	})
	expectedRefusal := probe.expectedRefusal
	if expectedRefusal == "" {
		expectedRefusal = "nothing is stored for AS99999"
		if probe.table == "irr_iface" {
			expectedRefusal = "nothing is stored for as-set AS-MISSING"
		}
	}
	refused := firewallIRRRefusalErrors07(t, firewallIRRRefusalEvent07(expectedRefusal))
	bridge.SetDispatchCommand(func(ctx context.Context, command string) (*rpc.DispatchCommandOutput, error) {
		if command == "show errors" {
			if probe.errorsCommand != nil {
				return probe.errorsCommand(ctx)
			}
			return refused, nil
		}
		if command != "show firewall ruleset "+probe.table {
			return nil, fmt.Errorf("unexpected observer command %q", command)
		}
		return probe.response, probe.commandErr
	})
	bridge.SetReady()

	query := func() string {
		query, readErr := os.ReadFile("nft.query")
		if readErr != nil {
			require.ErrorIs(t, readErr, os.ErrNotExist)
			return ""
		}
		return string(query)
	}
	scenario := firewallIRRRefusedCommit07(probe.table, expectedRefusal)
	run := func(ctx context.Context) error { return scenario(ctx, plugin) }
	return run, query
}

// This is the server's post-reload failure record, emitted after the failed
// transaction returned, rather than the earlier in-verify commit-aborted event.
func firewallIRRRefusalEvent07(reason string) map[string]string {
	return map[string]string{
		"source":   "plugin-server",
		"code":     "doctor-config-reload-refused",
		"subject":  "config-reload",
		"severity": "error",
		"message":  "configuration reload refused: firewall irr: " + reason + "\nthe rules naming it would filter nothing\nfix: update irr all",
	}
}

func firewallIRRRefusalErrors07(t *testing.T, entry map[string]string) *rpc.DispatchCommandOutput {
	t.Helper()
	data, err := json.Marshal(map[string]any{"errors": []any{entry}})
	require.NoError(t, err)
	return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, Data: data}
}

// startFirewallIRRRefusalObserver07 joins before the probe's bridge and working
// directory cleanups. The deadline bounds a broken observer; it never proves
// ordering or success, which the dispatch and completion channels establish.
func startFirewallIRRRefusalObserver07(t *testing.T, run func(context.Context) error) (context.Context, <-chan error, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), WaitBudget(50, 5*time.Second))
	result := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		result <- run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(WaitBudget(50, 5*time.Second)):
			t.Error("IRR refusal observer did not join after cancellation")
		}
	})
	return ctx, result, cancel
}

// TestFirewallIRRRefusedCommitWaitsForReloadRefusal holds the real command
// callback before it can return the exact post-reload refusal. The already
// present reload.done marker records only SIGHUP delivery, not verification.
func TestFirewallIRRRefusedCommitWaitsForReloadRefusal(t *testing.T) {
	for _, tc := range []struct {
		name            string
		table           string
		expectedRefusal string
	}{
		{name: "wan ASN", table: "wan", expectedRefusal: "nothing is stored for AS99999"},
		{name: "wan AS-SET", table: "wan", expectedRefusal: "nothing is stored for as-set AS-MISSING"},
		{name: "interface AS-SET", table: "irr_iface", expectedRefusal: "nothing is stored for as-set AS-MISSING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			refused := firewallIRRRefusalErrors07(t, firewallIRRRefusalEvent07(tc.expectedRefusal))
			run, query := prepareFirewallIRRRefusalProbe07(t, firewallIRRRefusalProbe07{
				table:           tc.table,
				expectedRefusal: tc.expectedRefusal,
				errorsCommand: func(ctx context.Context) (*rpc.DispatchCommandOutput, error) {
					select {
					case <-entered:
					default:
						close(entered)
					}
					select {
					case <-release:
						return refused, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			})
			ctx, result, _ := startFirewallIRRRefusalObserver07(t, run)
			select {
			case <-entered:
			case err := <-result:
				t.Fatalf("observer completed before reading the reload refusal: error=%v nft=%q", err, query())
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.Empty(t, query(), "the kernel must not be queried while the refusal answer is still held")
			close(release)
			select {
			case err := <-result:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.Equal(t, "list tables\n", query(), "the matching refusal must precede the answered kernel absence proof")
		})
	}
}

// TestFirewallIRRRefusedCommitRejectsUnprovenReloadRefusal supplies one bad
// answer, then holds the next command. Reaching that retry without an nft query
// proves the bad answer did not open the barrier. Cancellation then bounds and
// joins the refusal path without relying on elapsed time as success evidence.
func TestFirewallIRRRefusedCommitRejectsUnprovenReloadRefusal(t *testing.T) {
	for _, tc := range []struct {
		name            string
		field           string
		value           string
		data            string
		status          string
		err             error
		expectedRefusal string
	}{
		{name: "missing event", data: `{"errors":[]}`},
		{name: "wrong source", field: "source", value: "config"},
		{name: "wrong code", field: "code", value: "commit-aborted"},
		{name: "wrong subject", field: "subject", value: "other-reload"},
		{name: "wrong severity", field: "severity", value: "warning"},
		{name: "different IRR reference", field: "message", value: "nothing is stored for as-set AS-MISSING"},
		{name: "longer ASN shares prefix", field: "message", value: "firewall irr: nothing is stored for AS999999\nthe rules naming it would filter nothing\nfix: update irr all"},
		{name: "longer AS-SET shares prefix", expectedRefusal: "nothing is stored for as-set AS-MISSING", field: "message", value: "firewall irr: nothing is stored for as-set AS-MISSING-OTHER\nthe rules naming it would filter nothing\nfix: update irr all"},
		{name: "non-object answer", data: `[]`},
		{name: "malformed answer", data: `{"errors":`},
		{name: "failed command", status: rpc.StatusError},
		{name: "unreadable answer", err: errors.New("synthetic show-errors transport failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectedRefusal := tc.expectedRefusal
			if expectedRefusal == "" {
				expectedRefusal = "nothing is stored for AS99999"
			}
			entry := firewallIRRRefusalEvent07(expectedRefusal)
			if tc.field != "" {
				entry[tc.field] = tc.value
			}
			answer := firewallIRRRefusalErrors07(t, entry)
			if tc.data != "" {
				answer.Data = json.RawMessage(tc.data)
			}
			if tc.status != "" {
				answer.Status = tc.status
				answer.Error = "synthetic show-errors command failure"
			}
			retried := make(chan struct{})
			calls := 0
			run, query := prepareFirewallIRRRefusalProbe07(t, firewallIRRRefusalProbe07{
				table:           "wan",
				expectedRefusal: expectedRefusal,
				errorsCommand: func(ctx context.Context) (*rpc.DispatchCommandOutput, error) {
					calls++
					if calls == 1 {
						return answer, tc.err
					}
					if calls == 2 {
						close(retried)
					}
					<-ctx.Done()
					return nil, ctx.Err()
				},
			})
			ctx, result, cancel := startFirewallIRRRefusalObserver07(t, run)
			select {
			case <-retried:
			case err := <-result:
				require.Error(t, err, "an unproven refusal must fail before querying the kernel")
				require.Empty(t, query(), "an unproven refusal cannot authorize the kernel query")
				return
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.Empty(t, query(), "the invalid first answer must not open the kernel-query barrier")
			cancel()
			select {
			case err := <-result:
				require.Error(t, err)
			case <-time.After(WaitBudget(50, 5*time.Second)):
				t.Fatal("IRR refusal observer did not return its cancellation error")
			}
			require.Empty(t, query(), "an exhausted refusal barrier must leave the kernel unqueried")
		})
	}
}

// TestFirewallIRRRefusedCommitRejectsUnknownKernelAbsence gives the actual
// observer a failed daemon readback beside either a present kernel table or an
// unreadable inventory. None proves the absence the observer claims to assert.
// PREVENTS: any non-done command status being accepted as a missing table.
// MUTATION: restoring the old status-only guard returns nil in every case.
func TestFirewallIRRRefusedCommitRejectsUnknownKernelAbsence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe firewallIRRRefusalProbe07
	}{
		{
			name: "transport failure while wan table exists",
			probe: firewallIRRRefusalProbe07{
				table:      "wan",
				inventory:  "table inet ze_wan\n",
				commandErr: errors.New("synthetic dispatch transport failure"),
			},
		},
		{
			name: "missing backend while wan table exists",
			probe: firewallIRRRefusalProbe07{
				table:     "wan",
				inventory: "table inet ze_wan\n",
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  "firewall: no backend configured; firewall section absent from config",
				},
			},
		},
		{
			name: "counter failure while interface table exists",
			probe: firewallIRRRefusalProbe07{
				table:     "irr_iface",
				inventory: "table inet ze_irr_iface\n",
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  "synthetic nft counter read failure",
				},
			},
		},
		{
			name: "failed inventory with empty stdout",
			probe: firewallIRRRefusalProbe07{
				table:    "wan",
				stderr:   "synthetic inventory permission denied",
				exitCode: 1,
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  `firewall: table "wan" not found; no firewall tables have been applied`,
				},
			},
		},
		{
			name: "malformed successful inventory",
			probe: firewallIRRRefusalProbe07{
				table:     "wan",
				inventory: "table inet\n",
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  `firewall: table "wan" not found; no firewall tables have been applied`,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runFirewallIRRRefusalProbe07(t, tc.probe)
			require.Error(t, err, "an unanswered command or inventory cannot prove kernel table absence")
		})
	}
}

// TestFirewallIRRRefusedCommitAcceptsAnsweredKernelAbsence preserves the useful
// outcome: a refused commit whose positively queried inventory lacks the exact
// inet table succeeds, including the separate interface table identity.
// PREVENTS: rejecting every refusal instead of proving the requested absence.
func TestFirewallIRRRefusedCommitAcceptsAnsweredKernelAbsence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		table     string
		inventory string
	}{
		{name: "no tables", table: "wan"},
		{
			name:      "other families and similar names are not the interface table",
			table:     "irr_iface",
			inventory: "table inet ze_irr_iface_old\ntable ip ze_irr_iface\ntable inet ze_other\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, err := runFirewallIRRRefusalProbe07(t, firewallIRRRefusalProbe07{
				table:     tc.table,
				inventory: tc.inventory,
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  fmt.Sprintf("firewall: table %q not found; no firewall tables have been applied", tc.table),
				},
			})
			require.NoError(t, err)
			require.Equal(t, "list tables\n", query, "absence requires an answered kernel table inventory")
		})
	}
}

// TestFirewallIRRRefusedCommitRejectsPresentKernelTableAfterNotFound requires
// the actual observer to inspect both prefixed kernel table identities even
// when the daemon answers its normal table-not-found error. Neither a bare-name
// query nor refusing the daemon error before querying the kernel proves this.
func TestFirewallIRRRefusedCommitRejectsPresentKernelTableAfterNotFound(t *testing.T) {
	for _, table := range []string{"wan", "irr_iface"} {
		t.Run(table, func(t *testing.T) {
			query, err := runFirewallIRRRefusalProbe07(t, firewallIRRRefusalProbe07{
				table:     table,
				inventory: "table inet ze_" + table + "\n",
				response: &rpc.DispatchCommandOutput{
					Status: rpc.StatusError,
					Error:  fmt.Sprintf("firewall: table %q not found; no firewall tables have been applied", table),
				},
			})
			require.Error(t, err, "a table still present in the kernel disproves the observer's absence claim")
			require.Equal(t, "list tables\n", query, "the observer must inspect the kernel despite the daemon's not-found reply")
		})
	}
}
