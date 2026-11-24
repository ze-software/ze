package managed

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/fleet"
)

// The client of the divergence tests, the secret the hub serves it and the
// secret its local edit rotated to.
const (
	divergenceClient       = "edge-01"
	divergenceServedSecret = "edge-01-served-secret-of-32-characters"
	divergenceHeldSecret   = "edge-01-rotated-secret-of-32-characters"
)

// divergenceClientConfig is a managed client's configuration with its hub at
// host, authenticated with secret.
func divergenceClientConfig(host, secret string) string {
	return "bus {\n    hub {\n        client " + divergenceClient + " {\n            host " + host +
		";\n            port 1791;\n            secret \"" + secret + "\";\n        }\n    }\n}\n"
}

// divergenceHubConfig is the hub's own configuration: one serving block that
// declares the divergence client.
const divergenceHubConfig = `bus {
    hub {
        server central {
            ip 127.0.0.1;
            port 1790;
            secret "central-plugin-secret-of-32-characters";
            client edge-01 {
                secret "edge-01-served-secret-of-32-characters";
            }
        }
    }
}
`

// divergenceEdited and divergenceReceived are the two times of every held
// copy the tests store.
var (
	divergenceEdited   = time.Date(2026, 11, 19, 9, 30, 0, 0, time.UTC)
	divergenceReceived = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
)

// divergenceHeldKey is the store key of the held copy of client name.
func divergenceHeldKey(name string) string {
	return pluginserver.HeldRecordKey(name)
}

// divergenceHeldRecord is the stored form of a held copy of content, as the
// managed server writes it (pluginserver.ReadHeldRecord decodes it).
func divergenceHeldRecord(t *testing.T, content string) string {
	t.Helper()
	encoded, err := json.Marshal(struct {
		Version    string    `json:"version"`
		Config     []byte    `json:"config"`
		EditedAt   time.Time `json:"edited-at"`
		ReceivedAt time.Time `json:"received-at"`
	}{fleet.VersionHash([]byte(content)), []byte(content), divergenceEdited, divergenceReceived})
	require.NoError(t, err)
	return string(encoded)
}

// divergenceArgs answers the validated arguments of a show that names
// divergenceClient.
func divergenceArgs(t *testing.T) command.ValidatedArgs {
	t.Helper()
	name, err := command.NewStringArg(leafDivergenceName, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	validated, err := command.ValidateArgs([]string{leafDivergenceName, divergenceClient}, []command.ArgDef{name}, nil)
	require.NoError(t, err)
	return validated
}

// installServing installs a divergence target whose server serves the named
// clients, and withdraws it when the test ends.
func installServing(t *testing.T, clients ...string) {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	InstallDivergenceTarget(&DivergenceTarget{
		Store:               store,
		ServesClient:        func(name string) bool { return slices.Contains(clients, name) },
		NotifyConfigChanged: func(string) {},
	})
	t.Cleanup(func() {
		InstallDivergenceTarget(nil)
		assert.NoError(t, store.Close())
	})
}

// noDivergenceDaemon is the ask of a host where no daemon answers.
func noDivergenceDaemon(string) (string, error) { return "", errNoDaemon }

// TestShowDivergenceMasksSecrets proves that the show of one client answers
// the diff of the hub's configuration to the client's with every secret masked
// and no configuration text, and that its count is its number of changes.
//
// Method: the daemon route over a store holding the configuration the hub
// serves the client and a held copy whose edit rotated the client's secret and
// moved its hub host; the answer is encoded as a surface would encode it.
// VALIDATES: spec-fleet-7 AC-10 (diff entries, every secret masked, count, D,
// V, edited-at, received-at) and the Security Review (no configuration text).
// PREVENTS: a show that prints either secret, answers the texts, or counts
// something other than its entries.
// MUTATION: drop the infra.DiffConfigs masking, or answer record.Config():
// the secret assertions go red.
func TestShowDivergenceMasksSecrets(t *testing.T) {
	served := divergenceClientConfig("10.0.0.1", divergenceServedSecret)
	held := divergenceClientConfig("10.0.0.2", divergenceHeldSecret)
	fleetStatusStore(t,
		map[string]string{divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, held)},
		map[string]string{pluginserver.ClientConfigKey(divergenceClient): served})
	installServing(t, divergenceClient)

	got := command.AnswerRemote(answerDivergenceShow, divergenceArgs(t))
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	detail, isDetail := got.Payload.(divergenceDetail)
	require.True(t, isDetail, "payload is %T, want a divergence detail", got.Payload)

	assert.Equal(t, divergenceClient, detail.Client)
	assert.True(t, detail.Declared)
	assert.Equal(t, fleet.VersionHash([]byte(held)), detail.DeviceVersion)
	assert.Equal(t, fleet.VersionHash([]byte(served)), detail.HubVersion)
	assert.Equal(t, "2026-11-19T09:30:00Z", detail.EditedAt)
	assert.Equal(t, "2026-11-19T10:00:00Z", detail.ReceivedAt)
	assert.Equal(t, divergenceRevertNone, detail.Revert)
	require.True(t, detail.DiffAvailable, "diff unavailable: %s", detail.DiffUnavailable)
	require.NotNil(t, detail.ChangeCount)
	assert.Equal(t, len(detail.Changes), *detail.ChangeCount)
	assert.Equal(t, []divergenceChange{
		{Path: "bus/hub/client/edge-01/host", Change: changeChanged, Old: "10.0.0.1", New: "10.0.0.2"},
		{Path: "bus/hub/client/edge-01/secret", Change: changeChanged, Old: config.SecretDataPlaceholder, New: config.SecretDataPlaceholder},
	}, detail.Changes)

	encoded, err := json.Marshal(got.Payload)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), divergenceServedSecret)
	assert.NotContains(t, string(encoded), divergenceHeldSecret)
	assert.NotContains(t, string(encoded), "host 10.0.0.2", "the answer carries configuration text")
}

// TestShowDivergenceListMarksOrphan proves that the list, with no daemon,
// answers every held copy with both hashes and the two times, and reads which
// clients are declared from the server blocks of the hub's stored
// configuration: a copy of a client no block declares is listed as such.
//
// VALIDATES: spec-fleet-7 AC-1 (the list names D and V), E-11 (an orphaned
// copy is reported), the local route of a stopped hub.
// PREVENTS: an orphaned copy hidden from the operator, or listed as served.
func TestShowDivergenceListMarksOrphan(t *testing.T) {
	served := divergenceClientConfig("10.0.0.1", divergenceServedSecret)
	held := divergenceClientConfig("10.0.0.2", divergenceHeldSecret)
	fleetStatusStore(t,
		map[string]string{
			divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, held),
			divergenceHeldKey("gone-01"):        divergenceHeldRecord(t, held),
		},
		map[string]string{
			fleetStatusConfigName:                          divergenceHubConfig,
			pluginserver.ClientConfigKey(divergenceClient): served,
		})

	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	got := command.AnswerLocal(func(args command.ValidatedArgs) command.LocalAnswer {
		return answerDivergenceListWith(args, noDivergenceDaemon)
	}, validated)
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	list, isList := got.Payload.(divergenceList)
	require.True(t, isList, "payload is %T, want a divergence list", got.Payload)

	assert.Equal(t, []divergenceRow{
		{
			Client: divergenceClient, Declared: true, DeviceVersion: fleet.VersionHash([]byte(held)),
			HubVersion: fleet.VersionHash([]byte(served)), EditedAt: "2026-11-19T09:30:00Z",
			ReceivedAt: "2026-11-19T10:00:00Z", Revert: divergenceRevertNone,
		},
		{
			Client: "gone-01", Declared: false, DeviceVersion: fleet.VersionHash([]byte(held)),
			HubVersion: "", EditedAt: "2026-11-19T09:30:00Z",
			ReceivedAt: "2026-11-19T10:00:00Z", Revert: divergenceRevertNone,
		},
	}, list.Clients)
}

// TestShowDivergenceListAnswersUnreadableRow proves that one held copy the
// store cannot decode is answered as an unreadable row with its reason, and
// that the list still answers every other copy.
//
// Method: a stopped hub's store with one good copy and one undecodable one;
// the list with no daemon.
// VALIDATES: spec-fleet-7 AC-1 (the list names every holding client), E-11.
// PREVENTS: one damaged key hiding every client that holds.
// MUTATION: return the read error from readDivergenceList, and this fails.
func TestShowDivergenceListAnswersUnreadableRow(t *testing.T) {
	served := divergenceClientConfig("10.0.0.1", divergenceServedSecret)
	held := divergenceClientConfig("10.0.0.2", divergenceHeldSecret)
	fleetStatusStore(t,
		map[string]string{
			divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, held),
			divergenceHeldKey("broken-01"):      "not a held record",
		},
		map[string]string{
			fleetStatusConfigName:                          divergenceHubConfig,
			pluginserver.ClientConfigKey(divergenceClient): served,
		})

	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	got := command.AnswerLocal(func(args command.ValidatedArgs) command.LocalAnswer {
		return answerDivergenceListWith(args, noDivergenceDaemon)
	}, validated)
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	list, isList := got.Payload.(divergenceList)
	require.True(t, isList, "payload is %T, want a divergence list", got.Payload)

	require.Len(t, list.Clients, 2)
	broken := list.Clients[0]
	assert.Equal(t, "broken-01", broken.Client)
	assert.False(t, broken.Declared)
	assert.Empty(t, broken.DeviceVersion)
	assert.Equal(t, "the held configuration of client broken-01 could not be read from the store", broken.Unreadable,
		"the cell is the problem line alone, so the table renders the row on one line")
	assert.Equal(t, divergenceRow{
		Client: divergenceClient, Declared: true, DeviceVersion: fleet.VersionHash([]byte(held)),
		HubVersion: fleet.VersionHash([]byte(served)), EditedAt: "2026-11-19T09:30:00Z",
		ReceivedAt: "2026-11-19T10:00:00Z", Revert: divergenceRevertNone,
	}, list.Clients[1])
}

// TestShowDivergenceListKeepsCopyOfUnservedRow proves that a row whose
// served configuration cannot be read keeps every field the held copy gives,
// and carries one line saying why.
//
// Method: a held copy read from a store, and the refusal readServedConfig
// answers; the row unservedDivergenceRow builds from them.
// VALIDATES: review round 2 finding 4 (spec-fleet-7 AC-1, the list names the
// held version of every holding client).
// PREVENTS: an unreadable row that drops the device version the hub read, and
// a multi-line cell that breaks the table row.
// MUTATION: answer unreadableDivergenceRow for an unreadable served
// configuration in readDivergenceList, and the device version is empty.
func TestShowDivergenceListKeepsCopyOfUnservedRow(t *testing.T) {
	held := divergenceClientConfig("10.0.0.2", divergenceHeldSecret)
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteKey(divergenceHeldKey(divergenceClient), []byte(divergenceHeldRecord(t, held))))
	record, stored, err := readHeld(store, divergenceClient)
	require.NoError(t, err)
	require.True(t, stored)
	const problem = "the configuration this hub serves client " + divergenceClient + " could not be read from the store"
	refusal := opererr.Wrap(errors.New("the key is damaged"), problem,
		"its hash and its diff with the client's configuration are what the divergence views compare", fixCheckStore)

	row := unservedDivergenceRow(divergenceClient, record, func(string) bool { return true }, refusal)

	assert.Equal(t, divergenceRow{
		Client: divergenceClient, Declared: true, DeviceVersion: fleet.VersionHash([]byte(held)),
		EditedAt: "2026-11-19T09:30:00Z", ReceivedAt: "2026-11-19T10:00:00Z", Unreadable: problem,
	}, row)
}

// TestShowDivergenceUncheckedPluginDegrades proves that a held copy holding a
// section of an external plugin the hub holds no schema of still answers its
// hashes and times, with the diff unavailable and the reason naming the
// plugin, rather than failing the command.
//
// VALIDATES: spec-fleet-7 AC-10 (unchecked plugin) and R-5.
// PREVENTS: a show that refuses the whole command, or answers a change count
// of zero, for a client whose diff the hub cannot compute.
func TestShowDivergenceUncheckedPluginDegrades(t *testing.T) {
	served := divergenceClientConfig("10.0.0.1", divergenceServedSecret)
	held := served + "bus {\n    endpoint acme {\n        external {\n            run \"acme-monitor\";\n        }\n    }\n}\nacme {\n    interval 7;\n}\n"
	fleetStatusStore(t,
		map[string]string{divergenceHeldKey(divergenceClient): divergenceHeldRecord(t, held)},
		map[string]string{pluginserver.ClientConfigKey(divergenceClient): served})
	installServing(t, divergenceClient)

	got := command.AnswerRemote(answerDivergenceShow, divergenceArgs(t))
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	detail, isDetail := got.Payload.(divergenceDetail)
	require.True(t, isDetail, "payload is %T, want a divergence detail", got.Payload)

	assert.Equal(t, fleet.VersionHash([]byte(held)), detail.DeviceVersion)
	assert.Equal(t, fleet.VersionHash([]byte(served)), detail.HubVersion)
	assert.False(t, detail.DiffAvailable)
	assert.Nil(t, detail.ChangeCount, "an unavailable diff answers no count")
	assert.Empty(t, detail.Changes)
	assert.Contains(t, detail.DiffUnavailable, "acme")
	assert.True(t, strings.HasSuffix(detail.DiffUnavailable, "fix: restart ze"), "the fix is not last: %s", detail.DiffUnavailable)
	assert.NotContains(t, detail.DiffUnavailable, "interval 7", "the reason quotes configuration text")
}

// TestShowDivergenceRefusesWithoutRecord proves the show of a client the hub
// keeps no copy for is refused with the list as its fix.
//
// VALIDATES: an operator error with the fix last (ai/rules/user-facing-errors.md).
// PREVENTS: an empty document read as a client that agrees with its hub.
func TestShowDivergenceRefusesWithoutRecord(t *testing.T) {
	fleetStatusStore(t, nil, nil)
	installServing(t, divergenceClient)

	got := command.AnswerRemote(answerDivergenceShow, divergenceArgs(t))
	assert.Equal(t, 1, got.Code)
	assert.Contains(t, got.Diagnostic, "keeps no held configuration for client "+divergenceClient)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(got.Diagnostic), pathDivergenceList), "diagnostic: %s", got.Diagnostic)
}

// TestDivergenceLocalRouteDrawsDaemonAnswer proves that the local route
// answers what a running daemon answered, sending it the command line, and
// that an answer of another shape is not drawn.
//
// VALIDATES: the local route reports the running server's declared clients.
// PREVENTS: a stopped-hub answer shown while a daemon runs, or another
// document read as a divergence answer.
func TestDivergenceLocalRouteDrawsDaemonAnswer(t *testing.T) {
	fleetStatusStore(t, nil, nil)
	daemonList := `{"clients":[{"client":"edge-01","declared":true,"device-version":"0123456789abcdef",` +
		`"hub-version":"fedcba9876543210","edited-at":"","received-at":"2026-11-19T10:00:00Z","revert":"none"}]}`
	var asked []string
	ask := func(line string) (string, error) { //nolint:unparam // it MUST have the daemon ask's shape, and this daemon always answers
		asked = append(asked, line)
		return daemonList, nil
	}
	validated, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	got := command.AnswerLocal(func(args command.ValidatedArgs) command.LocalAnswer {
		return answerDivergenceListWith(args, ask)
	}, validated)
	require.Equal(t, 0, got.Code, "diagnostic: %s", got.Diagnostic)
	list, isList := got.Payload.(divergenceList)
	require.True(t, isList)
	require.Len(t, list.Clients, 1)
	assert.True(t, list.Clients[0].Declared)
	assert.Equal(t, []string{pathDivergenceList}, asked)

	_, answered := daemonDivergenceList(func(string) (string, error) { return `{"managed":true}`, nil })
	assert.False(t, answered, "another document was read as the list")
	_, answered = daemonDivergenceDetail(func(line string) (string, error) {
		assert.Equal(t, "show fleet divergence name other-01", line)
		return `{"client":"edge-01"}`, nil
	}, "other-01")
	assert.False(t, answered, "the show of another client was drawn")
}

// TestCompleteDivergenceClients proves the client name of the show completes
// from the held copies in the daemon's store, after the name keyword only.
//
// VALIDATES: the Integration Checklist completion row of spec-fleet-7.
// PREVENTS: a completion offering clients with no copy, or offered at a
// position that takes no client name.
func TestCompleteDivergenceClients(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	held := divergenceClientConfig("10.0.0.2", divergenceHeldSecret)
	require.NoError(t, store.WriteKey(divergenceHeldKey("edge-02"), []byte(divergenceHeldRecord(t, held))))
	require.NoError(t, store.WriteKey(divergenceHeldKey(divergenceClient), []byte(divergenceHeldRecord(t, held))))
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey("edge-03"), []byte(held), 0))
	statestore.SetStore(store)
	t.Cleanup(func() { statestore.SetStore(nil) })

	var values []string
	for _, candidate := range completeDivergenceClients(nil, []string{leafDivergenceName}) {
		values = append(values, candidate.Value)
	}
	assert.Equal(t, []string{divergenceClient, "edge-02"}, values)
	assert.Empty(t, completeDivergenceClients(nil, nil))
	assert.Empty(t, completeDivergenceClients(nil, []string{leafDivergenceName, divergenceClient}))
}

// TestDivergenceRefusalNamesTypedCommand proves every problem line of the
// show, the adopt and the revert names the command as an operator types it,
// with the client after its name keyword, and the placeholder when the line
// named none: never the registered path, which no operator types.
//
// Method: the problem lines divergenceCommandOf writes, and the refusal of
// revertArgs and adoptArgs for a line that named no client.
// VALIDATES: review round 2 finding 2 (ai/rules/user-facing-errors.md: the
// message names what the operator typed).
// PREVENTS: a refusal that names `request fleet divergence name revert`.
// MUTATION: write pathDivergenceRevert in revertProblem, or drop the
// placeholder for an empty client, and this fails.
func TestDivergenceRefusalNamesTypedCommand(t *testing.T) {
	assert.Equal(t, "show fleet divergence name edge-01", divergenceShowCommand(divergenceClient))
	assert.Equal(t, "request fleet divergence name edge-01 revert: the held configuration of edge-01 was not reverted",
		revertProblem(divergenceClient))
	assert.Equal(t, "update fleet divergence name edge-01 adopt: the held configuration of edge-01 was not adopted",
		adoptProblem(divergenceClient))

	unnamed, err := command.ValidateArgs(nil, nil, nil)
	require.NoError(t, err)
	_, _, _, revertRefusal := revertArgs(unnamed)
	require.NotNil(t, revertRefusal)
	assert.Equal(t, "request fleet divergence name <client> revert: no client was named", revertRefusal.Problem())
	_, _, adoptRefusal := adoptArgs(unnamed)
	require.NotNil(t, adoptRefusal)
	assert.Equal(t, "update fleet divergence name <client> adopt: no client was named", adoptRefusal.Problem())
}

// TestDivergenceExtraWordRefusedOnBothRoutes proves a divergence command typed
// with a word it does not declare is refused the same way on both routes,
// naming the client and the word.
//
// Method: the show validated as each route validates it. The local route
// consumes the path with its name keyword, so the client and the extra word
// are two spare tokens; the daemon dispatcher binds the client as the name
// selector, so the extra word is the one spare token. Adopt and revert are
// judged on the local shape, where the client is a spare token beside the
// keyed hashes.
// VALIDATES: TestLocalCommandDaemonMatchesLocal/show_fleet_divergence_name
// (cmd/ze/hub), where the local route answered "no client was named" and the
// daemon route read the store for the client.
// PREVENTS: the extra word making the client unread on the local route, or
// being ignored on the daemon route.
// MUTATION: read the client with validated.Value, or drop the spare-word
// refusal of divergenceClientArg, and this fails.
func TestDivergenceExtraWordRefusedOnBothRoutes(t *testing.T) {
	const extra = "ze-match-no-such-word"
	name, err := command.NewStringArg(leafDivergenceName, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	defs := []command.ArgDef{name}
	noDaemon := func(string) (string, error) {
		t.Fatal("a refused line must not reach the daemon")
		return "", nil
	}

	local, err := command.ValidateArgs([]string{divergenceClient, extra}, defs, nil)
	require.NoError(t, err)
	daemon, err := command.ValidateArgs([]string{extra}, defs, map[string]string{leafDivergenceName: divergenceClient})
	require.NoError(t, err)

	want := `show fleet divergence name edge-01: unexpected word "ze-match-no-such-word"`
	localAnswer := answerDivergenceShowWith(local, noDaemon)
	daemonAnswer := answerDivergenceShowWith(daemon, noDaemon)
	assert.Equal(t, 1, localAnswer.Code)
	assert.Contains(t, localAnswer.Diagnostic, want)
	assert.Equal(t, localAnswer, daemonAnswer, "both routes must refuse the extra word in the same words")
	_, _, refusal := divergenceClientArg(pathDivergenceShow, local)
	require.NotNil(t, refusal)
	assert.Equal(t, want, refusal.Problem())
	assert.Equal(t, "show fleet divergence name edge-01", refusal.Fix())

	// A line that names the client alone is read the same way on both routes.
	for _, validated := range []command.ValidatedArgs{
		divergenceArgs(t),
		mustValidate(t, []string{divergenceClient}, defs, nil),
		mustValidate(t, nil, defs, map[string]string{leafDivergenceName: divergenceClient}),
	} {
		client, named, refusal := divergenceClientArg(pathDivergenceShow, validated)
		assert.Nil(t, refusal)
		assert.True(t, named)
		assert.Equal(t, divergenceClient, client)
	}

	hash, err := command.NewStringArg(leafDivergenceVersion, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	hub, err := command.NewStringArg(leafDivergenceHubVersion, nil, nil, command.ArgOptions{})
	require.NoError(t, err)
	adopt := mustValidate(t, []string{divergenceClient, extra, leafDivergenceVersion, "d"},
		[]command.ArgDef{name, hash}, nil)
	_, _, adoptRefusal := adoptArgs(adopt)
	require.NotNil(t, adoptRefusal)
	assert.Equal(t, `update fleet divergence name edge-01 adopt: unexpected word "ze-match-no-such-word"`, adoptRefusal.Problem())
	revert := mustValidate(t, []string{divergenceClient, extra, leafDivergenceVersion, "d", leafDivergenceHubVersion, "v"},
		[]command.ArgDef{name, hash, hub}, nil)
	_, _, _, revertRefusal := revertArgs(revert)
	require.NotNil(t, revertRefusal)
	assert.Equal(t, `request fleet divergence name edge-01 revert: unexpected word "ze-match-no-such-word"`, revertRefusal.Problem())
}

// mustValidate answers ValidateArgs over args, defs and preMatched, failing
// the test on a refusal.
func mustValidate(t *testing.T, args []string, defs []command.ArgDef, preMatched map[string]string) command.ValidatedArgs {
	t.Helper()
	validated, err := command.ValidateArgs(args, defs, preMatched)
	require.NoError(t, err)
	return validated
}

// TestDivergenceReadRefusalKeepsAFixAndNamesOne proves the refusal of a failed
// divergence read: a cause that names its own fix is answered as it is, and any
// other is worded as an unread store whose fix is ze doctor.
//
// VALIDATES: every divergence view refusal carries a fix (ai/rules/user-facing-errors.md).
// PREVENTS: a bare storage error reaching the operator with no fix.
func TestDivergenceReadRefusalKeepsAFixAndNamesOne(t *testing.T) {
	native := opererr.New("show fleet divergence name edge-01: no held copy", "the hub keeps none", "show fleet divergence list")
	assert.Same(t, native, divergenceReadRefusal(pathDivergenceList, fmt.Errorf("reading: %w", native)))

	cause := errors.New("blob: read failed")
	got := divergenceReadRefusal(pathDivergenceList, cause)
	assert.ErrorIs(t, got, cause)
	assert.Equal(t, pathDivergenceList+": the held configurations were not read", got.Problem())
	assert.Equal(t, "ze doctor", got.Fix())
}
