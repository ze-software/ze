// Design: docs/architecture/exabgp-bridge.md -- completeness of single event and command declaration tables.
// Related: bridge_declarations.go -- DeclaredEventMessages, MapAPIKeyToBusMessage, IsFormAPIKey, DeclaredCommandPrefixes.

package bridge

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/events"
)

// TestDeclarationTablesCompleteness verifies that the single event and command
// declaration tables in bridge_declarations.go match every Q-E4 API key mapping
// and every CLI command the ExaBGP translator emits, with no duplicates or dead
// entries.
//
// VALIDATES: Q-E3 single declaration source and Q-E4 ExaBGP API key to bus
// message mapping.
// PREVENTS: drift between the commands/events bridge translates and the bus
// declarations registered by bridgerun or emitted by migration.
func TestDeclarationTablesCompleteness(t *testing.T) {
	t.Parallel()

	t.Run("event-messages-and-api-key-mapping", func(t *testing.T) {
		t.Parallel()

		declared := DeclaredEventMessages()
		require.Len(t, declared, 10)
		seenDeclared := make(map[string]bool, len(declared))
		for _, msg := range declared {
			assert.False(t, seenDeclared[msg], "duplicate declared event message %q", msg)
			seenDeclared[msg] = true
		}

		// Verify every Q-E4 mapping row explicitly.
		wantMappings := map[string][]BusMessageMapping{
			"neighbor-changes":     {{Message: "bgp:state"}, {Message: "bgp:connected"}},
			"negotiated":           {{Message: "bgp:negotiated"}},
			"fsm":                  {{Message: "bgp:fsm"}},
			"signal":               {{Message: "bgp:signal"}},
			"receive-open":         {{Message: "bgp:open", Direction: BusDirectionReceived}},
			"send-open":            {{Message: "bgp:open", Direction: BusDirectionSent}},
			"receive-update":       {{Message: "bgp:update", Direction: BusDirectionReceived}},
			"send-update":          {{Message: "bgp:update", Direction: BusDirectionSent}},
			"receive-notification": {{Message: "bgp:notification", Direction: BusDirectionReceived}},
			"send-notification":    {{Message: "bgp:notification", Direction: BusDirectionSent}},
			"receive-keepalive":    {{Message: "bgp:keepalive", Direction: BusDirectionReceived}},
			"send-keepalive":       {{Message: "bgp:keepalive", Direction: BusDirectionSent}},
			"receive-refresh":      {{Message: "bgp:refresh", Direction: BusDirectionReceived}},
			"send-refresh":         {{Message: "bgp:refresh", Direction: BusDirectionSent}},
		}

		reached := make(map[string]bool, len(declared))
		for key, want := range wantMappings {
			got, ok := MapAPIKeyToBusMessage(key)
			require.True(t, ok, "MapAPIKeyToBusMessage(%q) should match", key)
			assert.Equal(t, want, got, "MapAPIKeyToBusMessage(%q)", key)
			assert.False(t, IsFormAPIKey(key), "event key %q must not be a form key", key)
			for _, m := range got {
				assert.True(t, seenDeclared[m.Message], "mapped message %q missing from DeclaredEventMessages", m.Message)
				reached[m.Message] = true
			}
		}
		for _, msg := range declared {
			assert.True(t, reached[msg], "declared event message %q is not mapped from any ExaBGP api key", msg)
		}

		// Unsupported operational keys map to nothing and are not form keys.
		for _, unsupported := range []string{"receive-operational", "send-operational", "unknown-key"} {
			got, ok := MapAPIKeyToBusMessage(unsupported)
			assert.False(t, ok, "unsupported key %q must not map", unsupported)
			assert.Nil(t, got)
			assert.False(t, IsFormAPIKey(unsupported))
		}

		// All six form keys are recognized by IsFormAPIKey and do not map to bus messages.
		for _, formKey := range []string{
			"receive-packets", "receive-consolidate", "receive-parsed",
			"send-packets", "send-consolidate", "send-parsed",
		} {
			assert.True(t, IsFormAPIKey(formKey), "expected form key %q", formKey)
			got, ok := MapAPIKeyToBusMessage(formKey)
			assert.False(t, ok, "form key %q must not map to a bus message", formKey)
			assert.Nil(t, got)
		}

		// Returned slices must be defensive copies.
		declared[0] = "mutated"
		assert.Equal(t, "bgp:state", DeclaredEventMessages()[0])
		mapped, ok := MapAPIKeyToBusMessage("neighbor-changes")
		require.True(t, ok)
		mapped[0].Message = "mutated"
		fresh, _ := MapAPIKeyToBusMessage("neighbor-changes")
		assert.Equal(t, "bgp:state", fresh[0].Message)
	})

	t.Run("command-prefixes-cover-every-emitted-command", func(t *testing.T) {
		t.Parallel()

		prefixes := DeclaredCommandPrefixes()
		seenPrefix := make(map[string]bool, len(prefixes))
		for _, p := range prefixes {
			assert.True(t, strings.HasPrefix(p, events.NamespaceCLI+":"), "prefix %q must start with cli:", p)
			assert.False(t, seenPrefix[p], "duplicate declared command prefix %q", p)
			seenPrefix[p] = true
		}

		// Translate a representative command from every translator family.
		exabgpLines := []string{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4",
			"withdraw route 10.0.0.0/24 next-hop 1.2.3.4",
			"announce eor ipv4 unicast",
			"announce flow route { match { destination 10.0.0.0/24; } then { discard; } }",
			"announce vpls rd 65000:1 endpoint 1 base 100 offset 1 size 8 next-hop 1.2.3.4",
			"announce attributes next-hop 1.2.3.4 nlri 10.0.1.0/24 10.0.2.0/24",
			"announce watchdog dnsr",
			"withdraw watchdog dnsr",
			"clear adj-rib in",
			"clear adj-rib out",
			"flush adj-rib out",
			"neighbor 10.0.0.1 teardown 6 2 maintenance",
			"neighbor 10.0.0.1 disable maintenance",
			"neighbor 10.0.0.1 enable",
			"neighbor 10.0.0.1 announce route-refresh ipv4 unicast",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002",
			"delete neighbor 10.0.0.2",
			"shutdown",
			"help",
		}

		emitted := make([]string, 0, len(exabgpLines)+3)
		for _, line := range exabgpLines {
			tr, err := TranslateLine(line)
			require.NoError(t, err, "TranslateLine(%q)", line)
			require.NotEmpty(t, tr.Commands, "TranslateLine(%q) emitted no command", line)
			for _, cmd := range tr.Commands {
				emitted = append(emitted, cmd.Text)
			}
		}

		// Add commands emitted by helper builders and runner plumbing.
		wdRoute, err := TranslateLine("announce route 10.0.4.0/24 next-hop 1.1.1.1 watchdog dnsr")
		require.NoError(t, err)
		wdGive, err := watchdogGiveCommand("script1", "*", wdRoute.Commands[0])
		require.NoError(t, err)
		emitted = append(emitted, wdGive, StatusPeerCommand, "request peer * flush")

		usedPrefix := make(map[string]bool, len(prefixes))
		for _, cmd := range emitted {
			candidate := events.NamespaceCLI + ":" + cmd
			matched := false
			for _, prefix := range prefixes {
				if candidate == prefix || strings.HasPrefix(candidate, prefix+" ") {
					usedPrefix[prefix] = true
					matched = true
				}
			}
			assert.True(t, matched, "emitted command %q is not covered by DeclaredCommandPrefixes()", cmd)
		}
		for _, prefix := range prefixes {
			assert.True(t, usedPrefix[prefix], "declared command prefix %q was not exercised by any emitted command", prefix)
		}

		// Ensure returned slice is a defensive copy.
		origFirst := prefixes[0]
		prefixes[0] = "mutated"
		assert.Equal(t, origFirst, DeclaredCommandPrefixes()[0])
		assert.False(t, slices.Contains(DeclaredCommandPrefixes(), events.NamespaceCLI+":request bgp watchdog forget"),
			"watchdog forget is no longer emitted by the bridge (handled by bgp-watchdog on bus:participant-exit)")
	})
}
