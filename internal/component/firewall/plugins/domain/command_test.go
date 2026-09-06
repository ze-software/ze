package domain

import (
	"encoding/json"
	"testing"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeCommand runs a command and decodes its JSON payload.
func decodeCommand(t *testing.T, plug *domainPlugin, command string, args ...string) map[string]any {
	t.Helper()
	status, data, err := plug.handleCommand(command, args)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	raw, ok := data.(json.RawMessage)
	require.True(t, ok, "a command payload must be structured data, so the pipe operators can render it")

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// TestShowDomainGroupReportsWhatEachNameResolvedTo proves the command an
// operator runs before a commit answers the question they are asking: does this
// group have data, and how old is it.
func TestShowDomainGroupReportsWhatEachNameResolvedTo(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	out := decodeCommand(t, plug, cmdShowDomainGroup)
	groups, ok := out["groups"].([]any)
	require.True(t, ok)
	require.Len(t, groups, 1)

	g, ok := groups[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cdn", g["name"])
	assert.InDelta(t, 1, g["ipv4-count"], 0)
	assert.InDelta(t, 0, g["ipv6-count"], 0)
	assert.Equal(t, "domain_v4_cdn", g["ipv4-set"])
	assert.Equal(t, "domain_v6_cdn", g["ipv6-set"])
	assert.InDelta(t, 60, g["ttl-floor-seconds"], 0)

	names, ok := g["names"].([]any)
	require.True(t, ok)
	require.Len(t, names, 1)
	name, ok := names[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "a.invalid", name["name"])

	families, ok := name["families"].([]any)
	require.True(t, ok)
	require.Len(t, families, 2, "both families are reported, each with its own state")

	v4, ok := families[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ipv4", v4["family"])
	assert.Equal(t, "NOERROR", v4["status"])
	assert.Equal(t, []any{"192.0.2.1"}, v4["addresses"])
}

// TestShowDomainGroupSeparatesMissingFromEmpty proves a family that was never
// asked for reads differently from one that answered with nothing.
//
// The two are different facts: the first says nothing was asked, the second
// says the name holds no address of that family. An operator debugging a rule
// that filters nothing needs to know which.
func TestShowDomainGroupSeparatesMissingFromEmpty(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})

	out := decodeCommand(t, plug, cmdShowDomainGroup)
	families := firstFamilies(t, out)
	v4, ok := families[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "missing", v4["status"], "nothing has been asked for yet")

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	_, err = plug.resolveAndRecord(nameKey{group: "cdn", name: "a.invalid", family: familyV6})
	require.NoError(t, err)

	out = decodeCommand(t, plug, cmdShowDomainGroup)
	families = firstFamilies(t, out)
	v6, ok := families[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NOERROR", v6["status"], "the name answered, and it holds no IPv6 address")
	assert.Empty(t, v6["addresses"])
}

// TestShowDomainGroupFiltersByName proves the optional name argument selects
// one group.
func TestShowDomainGroupFiltersByName(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{
			{Name: "cdn", Names: []string{"a.invalid"}, TTLFloor: 60},
			{Name: "mail", Names: []string{"b.invalid"}, TTLFloor: 60},
		},
	}
	plug, _ := newTestPlugin(t, cfg, nil)

	out := decodeCommand(t, plug, cmdShowDomainGroup, "mail")
	groups, ok := out["groups"].([]any)
	require.True(t, ok)
	require.Len(t, groups, 1)
	g, ok := groups[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mail", g["name"])
}

// TestCommandsNameTheConfiguredGroupsOnAnUnknownName proves an operator who
// mistyped a group sees the answer without a second command.
func TestCommandsNameTheConfiguredGroupsOnAnUnknownName(t *testing.T) {
	cfg := &domainConfig{groups: []group{{Name: "cdn"}, {Name: "mail"}}}
	plug, _ := newTestPlugin(t, cfg, nil)

	for _, command := range []string{cmdShowDomainGroup, cmdUpdateDomainGroup, cmdClearDomainGroup} {
		_, _, err := plug.handleCommand(command, []string{"typo"})
		require.Error(t, err, command)
		assert.Contains(t, err.Error(), "typo", command)
		assert.Contains(t, err.Error(), "cdn, mail", command+" must list the groups that do exist")
	}
}

// TestUpdateAndClearRequireAName proves the two commands that take a mandatory
// value say so rather than acting on every group.
func TestUpdateAndClearRequireAName(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)

	for _, command := range []string{cmdUpdateDomainGroup, cmdClearDomainGroup} {
		status, _, err := plug.handleCommand(command, nil)
		require.Error(t, err, command)
		assert.Equal(t, statusError, status)
		assert.Contains(t, err.Error(), "usage: ")
	}
}

// TestClearDomainGroupRemovesTheAddresses proves the deliberate exit from
// last-known-good works.
//
// VALIDATES: user story 6 -- an operator clears a group and its set leaves the
// kernel.
// PREVENTS: a group that was deregistered upstream enforcing its old addresses
// forever, which is the cost of keeping the last good answer on every failure.
func TestClearDomainGroupRemovesTheAddresses(t *testing.T) {
	useRecordingBackend(t)
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	require.NotEmpty(t, plug.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV4))

	out := decodeCommand(t, plug, cmdClearDomainGroup, "cdn")
	assert.Equal(t, "cdn", out["cleared"])
	assert.InDelta(t, 1, out["entries"], 0)
	assert.Empty(t, plug.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV4))
}

// TestClearDomainGroupSurvivesARestart proves the purge reached the disk. A
// clear that removed the addresses from memory alone would put them back on
// the next boot, which is the opposite of what the operator asked for.
func TestClearDomainGroupSurvivesARestart(t *testing.T) {
	useRecordingBackend(t)
	cfg := oneGroupConfig()
	plug, _ := newTestPlugin(t, cfg, map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	_ = decodeCommand(t, plug, cmdClearDomainGroup, "cdn")

	reopened := newStore(plug.cache.path)
	require.NoError(t, reopened.open(cfg.groups))
	t.Cleanup(reopened.close)
	assert.Empty(t, reopened.groupAddresses("cdn", []string{"a.invalid"}, familyV4),
		"the purge must reach the disk, not only memory")
}

// TestClearDomainGroupWithNothingCachedIsAnError proves clearing a group that
// holds nothing says so rather than reporting a successful no-op. An operator
// who cleared the wrong name would otherwise believe they had cleared the
// right one.
func TestClearDomainGroupWithNothingCachedIsAnError(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)
	status, _, err := plug.handleCommand(cmdClearDomainGroup, []string{"cdn"})
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "no cached addresses")
}

// TestUpdateDomainGroupResolvesAndReports proves the command that recovers a
// cold cache reports what it learned.
//
// VALIDATES: user story 1 -- an operator fetches a name, then commits a rule
// permitting it.
func TestUpdateDomainGroupResolvesAndReports(t *testing.T) {
	useRecordingBackend(t)
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA):    {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		stubKey("a.invalid", mdns.TypeAAAA): {records: []string{"2001:db8::1"}, ttl: 300, status: "NOERROR"},
	})

	out := decodeCommand(t, plug, cmdUpdateDomainGroup, "cdn")
	assert.Equal(t, "cdn", out["group"])
	assert.InDelta(t, 2, out["changed"], 0, "both families changed from nothing to something")
	assert.InDelta(t, 1, out["ipv4-count"], 0)
	assert.InDelta(t, 1, out["ipv6-count"], 0)

	// And the commit that was refused a moment ago is now accepted.
	assert.NoError(t, plug.verify(oneGroupConfig()))
}

// TestUpdateDomainGroupRefusesAGroupWithNoNames proves a group holding no
// domain-names says so rather than reporting a successful refresh of nothing.
func TestUpdateDomainGroupRefusesAGroupWithNoNames(t *testing.T) {
	cfg := &domainConfig{groups: []group{{Name: "cdn", TTLFloor: 60}}}
	plug, _ := newTestPlugin(t, cfg, nil)

	status, _, err := plug.handleCommand(cmdUpdateDomainGroup, []string{"cdn"})
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "nothing to resolve")
}

// TestUpdateDomainGroupReportsAPartialRefresh proves a failing name is
// reported together with what did work. Reporting only the failure would hide
// the addresses now in the kernel.
func TestUpdateDomainGroupReportsAPartialRefresh(t *testing.T) {
	useRecordingBackend(t)
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	plug, _ := newTestPlugin(t, cfg, map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {err: assert.AnError},
		stubKey("b.invalid", mdns.TypeA): {records: []string{"192.0.2.2"}, ttl: 300, status: "NOERROR"},
	})

	status, _, err := plug.handleCommand(cmdUpdateDomainGroup, []string{"cdn"})
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "partially refreshed")
	assert.Contains(t, err.Error(), "cdn")
	assert.NotEmpty(t, plug.cache.groupAddresses("cdn", cfg.groups[0].Names, familyV4),
		"the name that answered was still recorded")
}

// TestUnknownCommandIsRefused proves the dispatch is fail-closed. A default
// that answered "done" would make a mistyped command look like it worked.
func TestUnknownCommandIsRefused(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)
	status, data, err := plug.handleCommand("show firewall something-else", nil)
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Nil(t, data)
}

// TestCommandNamesMatchTheYANGCommandTree pins the plugin-side command strings
// to the ones the server forwarders register. They are declared once in
// register_cmd.go and used by both sides, so this guards a future edit that
// splits them.
func TestCommandNamesMatchTheYANGCommandTree(t *testing.T) {
	assert.Equal(t, "show firewall domain-group", cmdShowDomainGroup)
	assert.Equal(t, "update firewall domain-group", cmdUpdateDomainGroup)
	assert.Equal(t, "clear firewall domain-group", cmdClearDomainGroup)
}

func firstFamilies(t *testing.T, out map[string]any) []any {
	t.Helper()
	groups, ok := out["groups"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, groups)
	g, ok := groups[0].(map[string]any)
	require.True(t, ok)
	names, ok := g["names"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, names)
	name, ok := names[0].(map[string]any)
	require.True(t, ok)
	families, ok := name["families"].([]any)
	require.True(t, ok)
	return families
}
