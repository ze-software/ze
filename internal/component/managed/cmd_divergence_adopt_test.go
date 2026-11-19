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
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/fleet"
)

// adoptServed and adoptHeldConfig are the configuration the hub serves the
// divergence client and the one the client holds and sent.
var (
	adoptServed     = divergenceClientConfig("10.0.0.1", divergenceServedSecret)
	adoptHeldConfig = divergenceClientConfig("10.0.0.2", divergenceServedSecret)
)

// adoptHub answers a hub store that serves the divergence client adoptServed
// and keeps its held copy of adoptHeldConfig, as the config-held arm writes it.
func adoptHub(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), []byte(adoptServed), 0o600))
	require.NoError(t, store.WriteKey(divergenceHeldKey(divergenceClient), []byte(divergenceHeldRecord(t, adoptHeldConfig))))
	return store
}

// adoptArgsOf answers the validated arguments of an adopt of client at version.
func adoptArgsOf(t *testing.T, client, version string) command.ValidatedArgs {
	t.Helper()
	name, err := command.NewStringArg(leafDivergenceName, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	hash, err := command.NewStringArg(leafDivergenceVersion, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	validated, err := command.ValidateArgs([]string{leafDivergenceName, client, leafDivergenceVersion, version},
		[]command.ArgDef{name, hash}, nil)
	require.NoError(t, err)
	return validated
}

// servesDivergenceClient is the predicate of a hub whose server block declares
// the divergence client alone.
func servesDivergenceClient(name string) bool { return name == divergenceClient }

// requireAdoptUnchanged requires store to serve adoptServed and to keep the
// held copy: what every refused adopt leaves.
func requireAdoptUnchanged(t *testing.T, store storage.Storage) {
	t.Helper()
	served, err := store.ReadFile(pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptServed, string(served), "a refused adopt changed the configuration the hub serves")
	record, held, err := pluginserver.ReadHeldRecord(store, divergenceClient)
	require.NoError(t, err)
	require.True(t, held, "a refused adopt removed the held copy")
	assert.Equal(t, fleet.VersionHash([]byte(adoptHeldConfig)), record.Version())
}

// TestAdoptServesHeldBytes proves that adopt, on the daemon route, makes the
// held bytes the configuration the hub serves the client: the plain file the
// managed server's ReadConfig reads, and the active version behind it.
//
// Method: a hub store with a held copy, the target installed over it, and the
// daemon handler given the copy's own hash.
// VALIDATES: spec-fleet-7 AC-11 (client-<name>.conf holds the record's bytes
// as a new active version), A-2.
// PREVENTS: an adopt that answers done and leaves the hub serving its old
// configuration, so the client holds for ever.
// MUTATION: drop the storage.CommitConfigWithGuard call in adoptHeldLocked:
// both reads still answer adoptServed.
func TestAdoptServesHeldBytes(t *testing.T) {
	store := adoptHub(t)
	InstallDivergenceTarget(&DivergenceTarget{Store: store, ServesClient: servesDivergenceClient, NotifyConfigChanged: func(string) {}})
	t.Cleanup(func() { InstallDivergenceTarget(nil) })
	device := fleet.VersionHash([]byte(adoptHeldConfig))

	resp, err := handleDivergenceAdopt(nil, adoptArgsOf(t, divergenceClient, device))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, "refused: %s", resp.Error)
	adopted, isAdopted := resp.Data.(divergenceAdopted)
	require.True(t, isAdopted, "data is %T, want an adopt answer", resp.Data)
	assert.Equal(t, divergenceClient, adopted.Client)
	assert.Equal(t, device, adopted.DeviceVersion)
	assert.NotEmpty(t, adopted.Stamp)

	served, err := store.ReadFile(pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptHeldConfig, string(served), "the plain file the hub serves")
	active, err := storage.ReadActiveConfig(store, pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptHeldConfig, string(active), "the active version")
}

// TestAdoptRemovesHeldCopy proves that a successful adopt removes the held
// copy, so the client is no longer listed and no doctor check reports it.
//
// Method: adoptHeld over a hub store with a held copy, then a read of the copy.
// VALIDATES: spec-fleet-7 AC-11 (the record is removed), E-11 (cleared by
// adopt).
// PREVENTS: a copy that outlives its adopt and keeps the client listed.
// MUTATION: drop the guard.RemoveKey call in adoptHeldLocked.
func TestAdoptRemovesHeldCopy(t *testing.T) {
	store := adoptHub(t)

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.Nil(t, refusal, "refused: %v", refusal)

	_, held, err := pluginserver.ReadHeldRecord(store, divergenceClient)
	require.NoError(t, err)
	assert.False(t, held, "the held copy is still in the store after its adopt")
}

// countingLockStore is a store that counts the writer locks taken on one
// configuration name.
type countingLockStore struct {
	storage.Storage
	name  string
	locks int
}

// AcquireLock counts a lock of the watched name and takes it.
func (c *countingLockStore) AcquireLock(name string) (storage.WriteGuard, error) {
	if name == c.name {
		c.locks++
	}
	return c.Storage.AcquireLock(name)
}

// TestAdoptTakesOneGuard proves that the hash check, the publication and the
// removal run under one writer lock of client-<name>.conf, the lock the
// config-held arm takes to write a newer copy.
//
// Method: adoptHeld over a store that counts the locks of that name.
// VALIDATES: spec-fleet-7 Data Flow step 7 and E-11 (removed under the same
// guard as the write).
// PREVENTS: an adopt built from storage.RestoreConfig and a separate removal,
// between which a newer copy from the client lands and is then removed
// unseen.
// MUTATION: publish with storage.RestoreConfig, which takes its own lock: two
// locks are counted.
func TestAdoptTakesOneGuard(t *testing.T) {
	store := &countingLockStore{Storage: adoptHub(t), name: pluginserver.ClientConfigKey(divergenceClient)}

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.Nil(t, refusal, "refused: %v", refusal)
	assert.Equal(t, 1, store.locks, "writer locks of %s taken by one adopt", store.name)
}

// TestAdoptNotifiesClient proves that the adopt's write reaches the store's
// write observer under the client's configuration name, which the running
// hub turns into config-changed for that client (cmd/ze/hub/managed_server.go).
//
// Method: a write observer on the hub store, then adoptHeld.
// VALIDATES: spec-fleet-7 AC-11 (the device is notified), A-3 at the store.
// PREVENTS: an adopt that writes only keys no observer maps to the client, so
// the client learns of it only when it reconnects.
// MUTATION: write the bytes with guard.WriteKey under another key instead of
// CommitConfigWithGuard: no observed key names the client.
func TestAdoptNotifiesClient(t *testing.T) {
	store := adoptHub(t)
	var notified []string
	store.SetWriteObserver(func(key string) {
		if name, isClient := pluginserver.ClientNameFromConfigKey(key); isClient {
			notified = append(notified, name)
		}
	})

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.Nil(t, refusal, "refused: %v", refusal)
	assert.Contains(t, notified, divergenceClient)
}

// TestAdoptRefusesStaleHash proves that an adopt naming a hash other than the
// held copy's writes nothing and names the copy's hash, with the show last.
//
// Method: adoptHeld with the hash of the configuration the hub serves.
// VALIDATES: spec-fleet-7 AC-12 (another hash refused, the record's current
// hash named, the show command last, nothing written), E-6.
// PREVENTS: an adopt of a copy the operator never saw, because the client
// sent a newer one after the show.
// MUTATION: drop the record.Version() != version guard: the hub then serves
// the held bytes.
func TestAdoptRefusesStaleHash(t *testing.T) {
	store := adoptHub(t)
	seen := fleet.VersionHash([]byte(adoptServed))

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, seen)
	require.NotNil(t, refusal, "an adopt of version %s was accepted", seen)
	assert.Contains(t, refusal.Why(), fleet.VersionHash([]byte(adoptHeldConfig)), "the refusal names the copy's hash")
	assert.Equal(t, "show fleet divergence name "+divergenceClient, refusal.Fix())
	requireAdoptUnchanged(t, store)
}

// TestAdoptRefusesWithoutRecord proves that an adopt of a client the hub keeps
// no copy for writes nothing and says that none exists, with the list last.
//
// Method: adoptHeld over a store that serves the client and holds no copy.
// VALIDATES: spec-fleet-7 AC-12 (no record: refused, saying so).
// PREVENTS: an adopt that commits an empty or stale configuration.
// MUTATION: answer no refusal for !held: the hub serves an empty configuration.
func TestAdoptRefusesWithoutRecord(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(divergenceClient), []byte(adoptServed), 0o600))

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.NotNil(t, refusal, "an adopt with no held copy was accepted")
	assert.Contains(t, refusal.Why(), "keeps no copy")
	assert.Equal(t, pathDivergenceList, refusal.Fix())
	served, err := store.ReadFile(pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptServed, string(served))
}

// TestAdoptRefusesUndeclaredClient proves that an adopt for a client no server
// block declares writes nothing, with the refusal every hub writer of an
// unserved client configuration answers.
//
// Method: adoptHeld with a predicate that declares no client.
// VALIDATES: spec-fleet-7 AC-19 (adopt refuses an orphaned record), the
// Existing Mechanisms row "Declared-client predicate".
// PREVENTS: an adopt that commits a configuration no client would ever fetch
// and removes the only copy of an orphaned client's configuration.
// MUTATION: drop the storagecli.ServedClientRefusal call in adoptHeld.
func TestAdoptRefusesUndeclaredClient(t *testing.T) {
	store := adoptHub(t)

	_, refusal := adoptHeld(store, servesNoClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.NotNil(t, refusal, "an adopt for an undeclared client was accepted")
	assert.Contains(t, refusal.Why(), "no client entry named "+divergenceClient)
	assert.Equal(t, "ze config edit", refusal.Fix())
	requireAdoptUnchanged(t, store)
}

// TestAdoptRefusesStagedCandidate proves that an adopt refused because a
// change of the client's configuration is staged keeps the copy and names the
// editor of that configuration as its fix.
//
// Method: a candidate version of client-<name>.conf staged under its lock,
// then adoptHeld.
// VALIDATES: spec-fleet-7 R-11 (ErrCandidateExists answered as an operator
// error naming the candidate's fix; nothing is removed).
// PREVENTS: an adopt that answers a bare store error, or removes the copy
// although nothing was published.
// MUTATION: drop the errors.Is(err, storage.ErrCandidateExists) arm: the fix
// is ze doctor.
func TestAdoptRefusesStagedCandidate(t *testing.T) {
	store := adoptHub(t)
	name := pluginserver.ClientConfigKey(divergenceClient)
	guard, err := store.AcquireLock(name)
	require.NoError(t, err)
	_, err = storage.WriteCandidateVersionWithGuard(store, guard, name, []byte(adoptServed+"# staged\n"), time.Now())
	require.NoError(t, err)
	require.NoError(t, guard.Release())

	_, refusal := adoptHeld(store, servesDivergenceClient, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig)))
	require.NotNil(t, refusal, "an adopt over a staged change was accepted")
	assert.Contains(t, refusal.Why(), "already staged")
	assert.Equal(t, "ze config edit "+name, refusal.Fix())
	requireAdoptUnchanged(t, store)
}

// TestAdoptDaemonWithoutTarget proves that the daemon route of a daemon that
// runs no managed server refuses with an operator error, and never reads a
// store it was not handed.
//
// Method: the daemon handler with no target installed.
// VALIDATES: spec-fleet-7 E-6 (the commands reach the server through an
// installed target).
// PREVENTS: a nil target dereferenced in the daemon.
// MUTATION: drop the target == nil guard: the handler panics.
func TestAdoptDaemonWithoutTarget(t *testing.T) {
	InstallDivergenceTarget(nil)

	resp, err := handleDivergenceAdopt(nil, adoptArgsOf(t, divergenceClient, fleet.VersionHash([]byte(adoptHeldConfig))))
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "runs no managed server")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(resp.Error), "ze doctor"), "the fix is last: %s", resp.Error)
}

// TestAdoptLocalOnStoppedHubRefused proves that the local route with no
// daemon refuses the adopt and writes nothing, because only the running hub
// records the decision in its audit log.
//
// Method: a stopped hub's store (its own configuration declaring the client,
// the served configuration and a held copy), then the local handler with an
// ask that fails the test if it is called.
// VALIDATES: the Security Review row "each records an audit action with user
// and client" on the local route: an adopt is taken only where the audit
// action is recorded, as revert is (TestRevertLocalOnStoppedHubRefused).
// PREVENTS: a change to a client's configuration that no audit log names.
// MUTATION: adopt on the opened store in adoptLocal: the served configuration
// changes and the copy is removed.
func TestAdoptLocalOnStoppedHubRefused(t *testing.T) {
	fleetStatusStore(t,
		map[string]string{divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, adoptHeldConfig)},
		map[string]string{
			fleetStatusConfigName:                          divergenceHubConfig,
			pluginserver.ClientConfigKey(divergenceClient): adoptServed,
		})
	device := fleet.VersionHash([]byte(adoptHeldConfig))
	noAsk := func(string, string) ([]byte, error) {
		t.Error("the local adopt asked a daemon although the store was free")
		return nil, errNoDaemon
	}

	got := adoptLocal(adoptArgsOf(t, divergenceClient, device), noAsk)
	assert.Equal(t, 1, got.Code)
	assert.Contains(t, got.Diagnostic, "the hub is stopped")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(got.Diagnostic), "ze start"), "the fix is last: %s", got.Diagnostic)

	folder := resolve.StoreDir("")
	store, err := storage.OpenReadOnly(folder, folder)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // read-only handle
	served, err := store.ReadFile(pluginserver.ClientConfigKey(divergenceClient))
	require.NoError(t, err)
	assert.Equal(t, adoptServed, string(served), "a refused adopt changed the served configuration")
	_, err = store.ReadFile(divergenceHeldKey(divergenceClient))
	assert.NoError(t, err, "a refused adopt removed the copy")
}

// TestAdoptIsAnEditCommand proves that adopt is a write for authorization
// while the two shows are reads, so a profile that denies the edit section
// is refused adopt and still answered the shows.
//
// Method: the dispatcher's read-only predicate over the three paths.
// VALIDATES: spec-fleet-7 AC-24, the Security Review row "Adopt and revert
// authorization".
// PREVENTS: an adopt path whose verb a read-only operator reaches.
// MUTATION: register adopt under the show verb: the first assertion goes red.
func TestAdoptIsAnEditCommand(t *testing.T) {
	assert.False(t, pluginserver.IsReadOnlyPath(pathDivergenceAdopt), "adopt changes what the hub serves")
	assert.True(t, pluginserver.IsReadOnlyPath(pathDivergenceShow))
	assert.True(t, pluginserver.IsReadOnlyPath(pathDivergenceList))
}
