package managed

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
)

// revertDecidedAt is when the tests' revert decisions are taken.
var revertDecidedAt = time.Date(2026, time.November, 19, 11, 0, 0, 0, time.UTC)

// revertArgsOf answers the validated arguments of a revert of client at
// version (D) and hubVersion (V): adopt's two leaves and the hub hash.
func revertArgsOf(t *testing.T, client, version, hubVersion string) command.ValidatedArgs {
	t.Helper()
	name, err := command.NewStringArg(leafDivergenceName, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	device, err := command.NewStringArg(leafDivergenceVersion, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	hub, err := command.NewStringArg(leafDivergenceHubVersion, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	validated, err := command.ValidateArgs(
		[]string{leafDivergenceName, client, leafDivergenceVersion, version, leafDivergenceHubVersion, hubVersion},
		[]command.ArgDef{name, device, hub}, nil)
	require.NoError(t, err)
	return validated
}

// revertDevice is D, the device hash of the copy adoptHub keeps.
func revertDevice() string { return fleet.VersionHash([]byte(adoptHeldConfig)) }

// revertHub is V, the hash of the configuration adoptHub serves the client:
// the hub hash its show answers.
func revertHub() string { return fleet.VersionHash([]byte(adoptServed)) }

// requireNoDecision requires the copy adoptHub keeps to be there, unchanged
// and carrying no revert decision: what every refused revert leaves.
func requireNoDecision(t *testing.T, store storage.Storage) {
	t.Helper()
	record, held, err := pluginserver.ReadHeldRecord(store, divergenceClient)
	require.NoError(t, err)
	require.True(t, held, "a refused revert removed the held copy")
	assert.Equal(t, revertDevice(), record.Version())
	_, decided := record.Revert()
	assert.False(t, decided, "a refused revert stored a decision")
}

// TestRevertStoresDecisionAndNotifies proves that revert, on the daemon
// route, stores the decision with the copy, bound to the copy's hash D and to
// the hash V of the configuration the hub serves the client, with the user
// who took it, and then tells the client.
//
// Method: a hub store with a held copy, the target installed over it with a
// recording NotifyConfigChanged, and the daemon handler given the copy's hash.
// VALIDATES: spec-fleet-7 AC-13 (the record stores the decision (D, V); the
// device is notified), E-4.
// PREVENTS: a revert that answers done and stores nothing the fetch answer
// could offer, or that never tells a connected client to fetch.
// MUTATION: drop the WriteHeldRecordWithGuard call in revertHeldLocked (no
// decision), or the target.NotifyConfigChanged call in handleDivergenceRevert
// (no notification).
func TestRevertStoresDecisionAndNotifies(t *testing.T) {
	store := adoptHub(t)
	var notified []string
	InstallDivergenceTarget(&DivergenceTarget{Store: store, ServesClient: servesDivergenceClient,
		NotifyConfigChanged: func(name string) { notified = append(notified, name) }})
	t.Cleanup(func() { InstallDivergenceTarget(nil) })

	resp, err := handleDivergenceRevert(&pluginserver.CommandContext{Username: "operator"}, revertArgsOf(t, divergenceClient, revertDevice(), revertHub()))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, "refused: %s", resp.Error)
	reverted, isReverted := resp.Data.(divergenceReverted)
	require.True(t, isReverted, "data is %T, want a revert answer", resp.Data)
	assert.Equal(t, divergenceReverted{Client: divergenceClient, DeviceVersion: revertDevice(),
		HubVersion: fleet.VersionHash([]byte(adoptServed))}, reverted)

	record, held, err := pluginserver.ReadHeldRecord(store, divergenceClient)
	require.NoError(t, err)
	require.True(t, held)
	decision, decided := record.Revert()
	require.True(t, decided, "no decision was stored")
	assert.Equal(t, revertDevice(), decision.DeviceVersion())
	assert.Equal(t, fleet.VersionHash([]byte(adoptServed)), decision.HubVersion())
	assert.Equal(t, "operator", decision.DecidedBy())
	assert.Equal(t, []string{divergenceClient}, notified)
}

// TestRevertKeepsServedConfig proves that revert changes nothing the hub
// serves: the client takes the hub's configuration as it is.
//
// Method: revertHeld over a hub store, then a read of what it serves.
// VALIDATES: E-4 (revert writes no client-<name>.conf).
// PREVENTS: a revert that publishes something, which would end the hold as an
// adopt does and voids its own decision.
// MUTATION: publish the held bytes in revertHeldLocked as adopt does.
func TestRevertKeepsServedConfig(t *testing.T) {
	store := adoptHub(t)

	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.Nil(t, refusal)

	served, err := store.ReadFile(pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptServed, string(served))
}

// TestRevertRefusesStaleHash proves that a revert naming a hash that is not
// the copy's is refused, naming the copy's hash and, last, the show.
//
// Method: revertHeld with the hash of the hub's own configuration.
// VALIDATES: spec-fleet-7 AC-12 for revert, R-2.
// PREVENTS: a decision taken on a copy the operator did not see.
// MUTATION: drop the record.Version() != version check of revertHeldLocked.
func TestRevertRefusesStaleHash(t *testing.T) {
	store := adoptHub(t)

	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptServed)), revertHub(), "operator", revertDecidedAt)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), revertDevice())
	assert.Equal(t, divergenceShowCommand(divergenceClient), refusal.Fix())
	requireNoDecision(t, store)
}

// TestRevertRefusesHubWrittenSinceShow proves that a revert is refused when
// the hub's configuration for the client was written between the show the
// operator read and the revert, and that it stores no decision: the decision
// binds the hub hash the operator saw, never the configuration as it stands
// at decision time.
//
// Method: a hub store with a held copy; the client's configuration written
// again on the hub; revertHeld with the hub hash the show answered before.
// VALIDATES: spec-fleet-7 E-13 (the revert names the hub hash and is refused
// when the hub's configuration no longer hashes to it), AC-12.
// PREVENTS: a client reverted to a hub configuration the operator never saw.
// MUTATION: bind the decision to fleet.VersionHash(served) in revertHeldLocked
// without comparing it with hubVersion, and this fails.
func TestRevertRefusesHubWrittenSinceShow(t *testing.T) {
	store := adoptHub(t)
	written := divergenceClientConfig("10.0.0.3", divergenceServedSecret)
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), []byte(written), 0o600))

	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), fleet.VersionHash([]byte(written)), "the refusal names what the hub serves now")
	assert.Contains(t, refusal.Error(), revertHub(), "the refusal names the hub hash the operator gave")
	assert.Equal(t, divergenceShowCommand(divergenceClient), refusal.Fix())
	requireNoDecision(t, store)
}

// TestRevertThroughDaemonSendsBothHashes proves that the local route sends
// the daemon a line carrying the device hash and the hub hash the operator
// gave, so the daemon checks the same two.
//
// Method: revertThroughDaemon with an ask that records the line and answers
// the revert's shape.
// VALIDATES: spec-fleet-7 E-13 (daemon and local routes).
// PREVENTS: a local revert whose hub hash the daemon never sees, which then
// refuses every line as missing a mandatory leaf, or binds another hash.
// MUTATION: drop the hub-version write of revertThroughDaemon, and this fails.
func TestRevertThroughDaemonSendsBothHashes(t *testing.T) {
	var sent string
	ask := func(_, line string) ([]byte, error) {
		sent = line
		return []byte(`{"client":"` + divergenceClient + `","device-version":"` + revertDevice() +
			`","hub-version":"` + revertHub() + `"}`), nil
	}

	got := revertThroughDaemon(t.TempDir(), storage.ErrBusy, ask, divergenceClient, revertDevice(), revertHub())
	assert.Equal(t, 0, got.Code, got.Diagnostic)
	assert.Equal(t, "request fleet divergence name "+divergenceClient+" revert version "+revertDevice()+" hub-version "+revertHub(), sent)
}

// TestRevertRefusesWithoutRecord proves that a revert of a client the hub
// keeps no copy for is refused, with the list as its fix.
//
// Method: revertHeld over a hub store that serves the client and keeps no copy.
// VALIDATES: spec-fleet-7 AC-12 (no record).
// PREVENTS: a decision written as a record with no copy, which the strict
// decode would refuse at every fetch.
// MUTATION: drop the !held check of revertHeldLocked.
func TestRevertRefusesWithoutRecord(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), []byte(adoptServed), 0o600))

	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), "keeps no copy")
	assert.Equal(t, pathDivergenceList, refusal.Fix())
	_, held, err := pluginserver.ReadHeldRecord(store, divergenceClient)
	require.NoError(t, err)
	assert.False(t, held, "a refused revert wrote a record")
}

// TestRevertRefusesUndeclaredClient proves that a revert of a client no
// server block declares is refused and stores nothing.
//
// Method: revertHeld with a predicate that declares no client.
// VALIDATES: spec-fleet-7 AC-19 (adopt and revert refuse an orphaned record).
// PREVENTS: a decision for a client the hub never serves.
// MUTATION: drop the storagecli.ServedClientRefusal call of revertHeld.
func TestRevertRefusesUndeclaredClient(t *testing.T) {
	store := adoptHub(t)

	_, refusal := revertHeld(store, servesNoClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.NotNil(t, refusal)
	assert.Contains(t, refusal.Error(), divergenceClient)
	requireNoDecision(t, store)
}

// TestRevertDaemonWithoutTarget proves that a daemon with no managed server
// refuses a revert with an operator error whose fix is last.
//
// Method: the daemon handler with no target installed.
// VALIDATES: the DivergenceTarget contract (none installed, none acted on).
// PREVENTS: a revert answered done by a daemon that serves no client.
// MUTATION: answer done when divergenceTarget.Load() is nil.
func TestRevertDaemonWithoutTarget(t *testing.T) {
	InstallDivergenceTarget(nil)

	resp, err := handleDivergenceRevert(nil, revertArgsOf(t, divergenceClient, revertDevice(), revertHub()))
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "runs no managed server")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(resp.Error), "ze doctor"), "the fix is last: %s", resp.Error)
}

// TestRevertLocalOnStoppedHubRefused proves that the local route with no
// daemon refuses the revert and stores nothing, because only the running hub
// records the decision in its audit log.
//
// Method: a stopped hub's store with a held copy, then the local handler with
// an ask that fails the test if it is called.
// VALIDATES: the Security Review row "each records an audit action with user
// and client" on the local route: a revert is decided only where the audit
// action is recorded.
// PREVENTS: a decision no audit log names.
// MUTATION: decide the revert on the opened store in revertLocal.
func TestRevertLocalOnStoppedHubRefused(t *testing.T) {
	fleetStatusStore(t,
		map[string]string{divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, adoptHeldConfig)},
		map[string]string{
			fleetStatusConfigName:                          divergenceHubConfig,
			pluginserver.ClientConfigKey(divergenceClient): adoptServed,
		})
	noAsk := func(string, string) ([]byte, error) {
		t.Error("the local revert asked a daemon although the store was free")
		return nil, errNoDaemon
	}

	got := revertLocal(revertArgsOf(t, divergenceClient, revertDevice(), revertHub()), noAsk)
	assert.Equal(t, 1, got.Code)
	assert.Contains(t, got.Diagnostic, "the hub is stopped")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(got.Diagnostic), "ze start"), "the fix is last: %s", got.Diagnostic)
}

// TestRevertIsAWriteCommand proves that revert is a write for authorization.
//
// Method: the dispatcher's read-only predicate over the revert path.
// VALIDATES: spec-fleet-7 AC-24 for revert.
// PREVENTS: a revert a read-only profile reaches.
// MUTATION: register revert under the show verb.
func TestRevertIsAWriteCommand(t *testing.T) {
	assert.False(t, pluginserver.IsReadOnlyPath(pathDivergenceRevert), "revert makes a holding client apply the hub's configuration")
}

// TestShowDivergenceReportsPendingDecision proves that a decision whose hub
// configuration is still the one it was taken on is reported pending.
//
// Method: revertHeld over a hub store, then the show of the client.
// VALIDATES: spec-fleet-7 AC-10 (the revert state), E-4.
// PREVENTS: a show that hides a decision the client is offered.
// MUTATION: leave the row's revert at none in divergenceRowOf.
func TestShowDivergenceReportsPendingDecision(t *testing.T) {
	store := adoptHub(t)
	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.Nil(t, refusal)

	detail, err := readDivergenceDetail(store, divergenceClient, servesDivergenceClient)
	require.NoError(t, err)
	assert.Equal(t, divergenceRevertPending, detail.Revert)
	assert.Empty(t, detail.RevertVoid)
}

// TestShowDivergenceReportsVoidDecision proves that a decision is reported
// void once the hub serves the client another configuration than the one the
// decision was taken on, with the revert to decide again as its fix.
//
// Method: revertHeld over a hub store, then a new configuration for the
// client written on the hub, then the show of the client.
// VALIDATES: spec-fleet-7 AC-16 (the show reports the decision void because
// the hub configuration changed, with the revert command as its fix), R-3.
// PREVENTS: a show that reports pending a decision the hub no longer offers,
// so the operator waits for a revert that never comes.
// MUTATION: report pending for every decision, or compare the decision's V
// with anything but the hash of what the hub serves now.
func TestShowDivergenceReportsVoidDecision(t *testing.T) {
	store := adoptHub(t)
	_, refusal := revertHeld(store, servesDivergenceClient, divergenceClient, revertDevice(), revertHub(), "operator", revertDecidedAt)
	require.Nil(t, refusal)
	written := divergenceClientConfig("10.0.0.3", divergenceServedSecret)
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), []byte(written), 0o600))

	detail, err := readDivergenceDetail(store, divergenceClient, servesDivergenceClient)
	require.NoError(t, err)
	assert.Equal(t, divergenceRevertVoid, detail.Revert)
	assert.Contains(t, detail.RevertVoid, fleet.VersionHash([]byte(adoptServed)), "the void reason names the V the decision was taken on")
	assert.Contains(t, detail.RevertVoid, fleet.VersionHash([]byte(written)), "the void reason names what the hub serves now")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(detail.RevertVoid),
		"request fleet divergence name "+divergenceClient+" revert version "+revertDevice()+" hub-version "+fleet.VersionHash([]byte(written))),
		"the fix is the revert over the hashes the show answers now, last: %s", detail.RevertVoid)
}
