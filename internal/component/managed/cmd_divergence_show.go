// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a holding client runs
// Design: docs/architecture/api/commands.md -- show fleet divergence list and show fleet divergence name
// Related: cmd_divergence.go -- the divergence target and the adopt and revert commands
// Related: register.go -- registers the two shows, their shapes, columns and the completer

// `show fleet divergence list` and `show fleet divergence name <client>` read
// the copies a hub keeps of the configurations its holding clients run
// (pluginserver.ReadHeldRecord). Every fact they answer is in the hub's store
// except one: whether a server block declares the client, which the running
// managed server knows (DivergenceTarget.ServesClient). So the local route
// asks a running daemon first and draws its answer, as `show fleet status`
// does, and with no daemon answers from the store, reading the server blocks
// of the hub's stored configuration: the clients a start would serve.
//
// No answer carries configuration text: both texts carry the client's
// secrets. The show of one client answers the diff of the hub's configuration
// to the client's from infra.DiffConfigs, which masks every secret leaf.

package managed

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
)

// leafDivergenceName is the leaf of `show fleet divergence name <client>`.
const leafDivergenceName = "name"

// The revert field of a held copy (pluginserver.RevertDecision): none while
// no decision was taken; pending while the decision holds, so the client is
// offered it at its fetch; void once the hub serves the client another
// configuration than the one the decision was taken on, so it is offered
// nothing and keeps holding.
const (
	divergenceRevertNone    = "none"
	divergenceRevertPending = "pending"
	divergenceRevertVoid    = "void"
)

// The kinds of one change of the diff.
const (
	changeAdded   = "added"
	changeRemoved = "removed"
	changeChanged = "changed"
)

// divergenceRow is one held copy, as both shows answer it. It carries the
// hashes and the times, never the configuration.
type divergenceRow struct {
	// Client is the client the copy is held for.
	Client string `json:"client"`
	// Declared says a server block declares the client. False is an orphaned
	// copy: the hub serves no such client, so it is reported and never served.
	Declared bool `json:"declared"`
	// DeviceVersion is the hash of the held bytes, recomputed by the hub.
	DeviceVersion string `json:"device-version"`
	// HubVersion is the hash of the configuration the hub serves the client,
	// empty when the hub's store holds none for it.
	HubVersion string `json:"hub-version"`
	// EditedAt is when the held version was written on the client (RFC 3339),
	// empty when the client's store records no such time.
	EditedAt string `json:"edited-at"`
	// ReceivedAt is when the hub stored the copy (RFC 3339).
	ReceivedAt string `json:"received-at"`
	// Revert is the state of the revert decision the copy carries: none,
	// pending or void; empty on an unreadable row.
	Revert string `json:"revert"`
	// Unreadable is the problem line of the operator error that says why the
	// copy, or the configuration the hub serves the client, could not be read
	// from the store; empty on a row read whole. A row whose copy is
	// unreadable answers only the client and whether it is declared; a row
	// whose served configuration is unreadable also answers what the copy
	// gives, with no hub version and no revert state. The other rows hold;
	// the show of one client refuses instead.
	Unreadable string `json:"unreadable,omitempty"`
}

// divergenceList is the answer of `show fleet divergence list`, one row per
// held copy, sorted by client.
type divergenceList struct {
	Clients []divergenceRow `json:"clients"`
}

// divergenceDetail is the answer of `show fleet divergence name <client>`.
// With DiffAvailable, ChangeCount and Changes are the diff from the hub's
// configuration to the client's; without it, DiffUnavailable is the operator
// error that says why, and neither is answered: a count of zero would read as
// two configurations that agree.
type divergenceDetail struct {
	divergenceRow
	// UncheckedPlugins names the external plugins whose sections the hub
	// could not check when it took the copy.
	UncheckedPlugins []string `json:"unchecked-plugins"`
	DiffAvailable    bool     `json:"diff-available"`
	DiffUnavailable  string   `json:"diff-unavailable,omitempty"`
	// RevertVoid is the operator error that says why the copy's revert
	// decision is void, with the revert to decide again as its fix; empty
	// unless Revert is void.
	RevertVoid  string             `json:"revert-void,omitempty"`
	ChangeCount *int               `json:"change-count,omitempty"`
	Changes     []divergenceChange `json:"changes,omitempty"`
}

// divergenceChange is one entry of the diff. Old is null for an added entry
// and New for a removed one. A value under a secret leaf is the mask.
type divergenceChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
	Old    any    `json:"old"`
	New    any    `json:"new"`
}

// divergenceAsk sends line to the running daemon and answers its JSON. An
// error means no daemon answered. askDivergenceOverSSH is the production ask;
// a test hands a stub.
type divergenceAsk func(line string) (string, error)

// askDivergenceOverSSH is the production divergenceAsk: the SSH client every
// `ze <verb>` daemon command uses, with the stored credential and no prompt.
func askDivergenceOverSSH(line string) (string, error) {
	reached, err := sshclient.ReachDaemon("")
	if err != nil {
		return "", err
	}
	return sshclient.ExecCommandRaw(reached.Credentials, line)
}

// answerDivergenceList answers `show fleet divergence list` on both routes.
func answerDivergenceList(validated command.ValidatedArgs) command.LocalAnswer {
	return answerDivergenceListWith(validated, askDivergenceOverSSH)
}

// answerDivergenceListWith is the list handler over one ask. In the daemon it
// answers from the store and the running server's clients; in the operator's
// own process it draws the daemon's answer when one gives it.
func answerDivergenceListWith(validated command.ValidatedArgs, ask divergenceAsk) command.LocalAnswer {
	if !validated.Route().CallerLocal() {
		return storeDivergence(pathDivergenceList, daemonServedClients(), readDivergenceList)
	}
	if list, answered := daemonDivergenceList(ask); answered {
		return command.LocalAnswer{Payload: list}
	}
	return storeDivergence(pathDivergenceList, nil, readDivergenceList)
}

// answerDivergenceShow answers `show fleet divergence name <client>` on both
// routes.
func answerDivergenceShow(validated command.ValidatedArgs) command.LocalAnswer {
	return answerDivergenceShowWith(validated, askDivergenceOverSSH)
}

// answerDivergenceShowWith is the show handler over one ask, routed as
// answerDivergenceListWith routes the list.
func answerDivergenceShowWith(validated command.ValidatedArgs, ask divergenceAsk) command.LocalAnswer {
	name, named := validated.Value(leafDivergenceName)
	if !named {
		// The YANG leaf is mandatory, so the validator refused this before.
		return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
			divergenceShowCommand("")+": no client was named",
			"the command shows the held configuration of one managed client",
			pathDivergenceList)))
	}
	read := func(store storage.Storage, serves func(string) bool) (any, error) {
		return readDivergenceDetail(store, name, serves)
	}
	if !validated.Route().CallerLocal() {
		return storeDivergence(divergenceShowCommand(name), daemonServedClients(), read)
	}
	if detail, answered := daemonDivergenceDetail(ask, name); answered {
		return command.LocalAnswer{Payload: detail}
	}
	return storeDivergence(divergenceShowCommand(name), nil, read)
}

// handleDivergenceShow is the builtin RPC ze-managed:fleet-divergence-show: the
// daemon form of the show, registered as a builtin so that it can declare the
// completer of its client name (completeDivergenceClients). It runs the same
// handler on the daemon route, as the local-data bridge would.
func handleDivergenceShow(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return pluginserver.AnswerLocalData(pathDivergenceShow, answerDivergenceShow, validated)
}

// storeDivergence opens this host's store read-only and answers read over it.
// serves is the running server's clients in the daemon; nil in the operator's
// own process, where the store's own configuration answers them.
func storeDivergence(path string, serves func(string) bool, read func(storage.Storage, func(string) bool) (any, error)) command.LocalAnswer {
	folder := resolve.StoreDir("")
	store, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		refusal := opererr.Wrap(err,
			path+": the store was not read",
			"the store of this host could not be opened, and it keeps the copies of the configurations managed clients hold",
			"ze doctor")
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	defer store.Close() //nolint:errcheck // read-only handle, nothing to flush

	if serves == nil {
		serves, err = storedServedClients(store)
		if err != nil {
			return command.LocalRefusal(1, command.LocalErrorLine(err))
		}
	}
	payload, err := read(store, serves)
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(divergenceReadRefusal(path, err)))
	}
	return command.LocalAnswer{Payload: payload}
}

// divergenceReadRefusal is the refusal of a divergence view whose read
// failed: a refusal that names its own fix is kept, and any other cause is
// worded as an unread store, whose fix is ze doctor.
func divergenceReadRefusal(path string, err error) *opererr.Error {
	if native, ok := errors.AsType[*opererr.Error](err); ok {
		return native
	}
	return opererr.Wrap(err,
		path+": the held configurations were not read",
		"the store of this host keeps the copies of the configurations managed clients hold, and reading them failed",
		"ze doctor")
}

// servesNoClient is the served-client predicate of a process that serves none.
func servesNoClient(string) bool { return false }

// daemonServedClients is the served-client predicate of the daemon: its
// managed server's. A daemon that installed no target runs no managed server,
// so it serves no client.
func daemonServedClients() func(string) bool {
	target := divergenceTarget.Load()
	if target == nil {
		return servesNoClient
	}
	return target.ServesClient
}

// storedServedClients is the served-client predicate of the hub's stored
// configuration: the clients its server blocks declare, read with the first
// pass the daemon's managed server is built from. A store with no
// configuration of its own declares none.
func storedServedClients(store storage.Storage) (func(string) bool, error) {
	content, stored, err := activeConfig(store)
	if err != nil {
		return nil, opererr.Wrap(err,
			"the configuration of this hub could not be read from the store",
			"its server blocks declare the managed clients it serves, which both divergence views report",
			fixCheckStore)
	}
	if !stored {
		return servesNoClient, nil
	}
	firstPass, err := config.FirstPass(string(content), "", nil)
	if err != nil {
		return nil, opererr.Wrap(err,
			"the plugin hub block of this hub's configuration could not be read",
			"its server blocks declare the managed clients it serves, which both divergence views report",
			"ze config validate")
	}
	served := servedClientNames(firstPass.Hub)
	return func(name string) bool { return served[name] }, nil
}

// readDivergenceList reads every held copy in store. A copy, or a served
// configuration, that cannot be read is answered as an unreadable row saying
// why, so one damaged key hides no other client that holds.
func readDivergenceList(store storage.Storage, serves func(string) bool) (any, error) {
	names, err := heldClientNames(store)
	if err != nil {
		return nil, err
	}
	list := divergenceList{Clients: make([]divergenceRow, 0, len(names))}
	for _, name := range names {
		record, held, err := readHeld(store, name)
		if err != nil {
			list.Clients = append(list.Clients, unreadableDivergenceRow(name, serves, err))
			continue
		}
		if !held {
			continue // Cleared between the listing and the read.
		}
		hubConfig, _, err := readServedConfig(store, name)
		if err != nil {
			list.Clients = append(list.Clients, unservedDivergenceRow(name, record, serves, err))
			continue
		}
		list.Clients = append(list.Clients, divergenceRowOf(name, record, hubConfig, serves))
	}
	return list, nil
}

// unreadableDivergenceRow is the list row of client name whose copy could not
// be read: it answers the client, whether it is declared, and why
// (unreadableReason).
func unreadableDivergenceRow(name string, serves func(string) bool, refusal error) divergenceRow {
	return divergenceRow{Client: name, Declared: serves(name), Unreadable: unreadableReason(refusal)}
}

// unservedDivergenceRow is the list row of client name whose copy, record, was
// read and whose served configuration could not be: it answers every field the
// copy gives, no hub version, no revert state, since whether a decision is
// pending depends on the hub version (divergenceRowOf), and why
// (unreadableReason).
func unservedDivergenceRow(name string, record pluginserver.HeldRecord, serves func(string) bool, refusal error) divergenceRow {
	row := divergenceRowOf(name, record, nil, serves)
	row.Revert = ""
	row.Unreadable = unreadableReason(refusal)
	return row
}

// unreadableReason is the cell an unreadable row of the list carries: the
// problem line of the operator error refusal, one line, which the table
// renders on the row's own line (a cell's newline starts a line of the row).
// The read helpers refuse with an operator error; any other error is answered
// by its first line.
func unreadableReason(refusal error) string {
	if native, ok := errors.AsType[*opererr.Error](refusal); ok {
		return native.Problem()
	}
	reason, _, _ := strings.Cut(refusal.Error(), "\n")
	return reason
}

// readDivergenceDetail reads the held copy of client name and compares it with
// the configuration the hub serves the client. A configuration the hub cannot
// parse leaves the diff unavailable, saying why: the rest of the answer holds.
func readDivergenceDetail(store storage.Storage, name string, serves func(string) bool) (divergenceDetail, error) {
	record, held, err := readHeld(store, name)
	if err != nil {
		return divergenceDetail{}, err
	}
	if !held {
		return divergenceDetail{}, opererr.New(
			divergenceShowCommand(name)+": the hub keeps no held configuration for client "+name,
			"a managed client sends the configuration it holds when it connects; this one sent none, or its hold ended",
			pathDivergenceList)
	}
	hubConfig, served, err := readServedConfig(store, name)
	if err != nil {
		return divergenceDetail{}, err
	}
	detail := divergenceDetail{
		divergenceRow:    divergenceRowOf(name, record, hubConfig, serves),
		UncheckedPlugins: record.UncheckedPlugins(),
	}
	if detail.UncheckedPlugins == nil {
		detail.UncheckedPlugins = []string{}
	}
	if decision, decided := record.Revert(); decided && detail.Revert == divergenceRevertVoid {
		detail.RevertVoid = revertVoidReason(name, record.Version(), decision, detail.HubVersion).Error()
	}
	if !served {
		detail.DiffUnavailable = opererr.New(
			"the diff of client "+name+" is not available: this hub holds no configuration for it",
			"the diff compares the configuration the hub serves the client with the one the client holds",
			"show config list").Error()
		return detail, nil
	}
	diff, err := divergenceDiff(name, hubConfig, record.Config())
	if err != nil {
		detail.DiffUnavailable = err.Error()
		return detail, nil //nolint:nilerr // a diff the hub cannot build degrades the show: the reason is the answer's diff-unavailable, never a failed command (spec R-5)
	}
	count := infra.DiffChangeCount(diff)
	detail.DiffAvailable = true
	detail.ChangeCount = &count
	detail.Changes = divergenceChanges(diff)
	return detail, nil
}

// readHeld reads the held copy of client name, refusing as an operator error.
func readHeld(store storage.Storage, name string) (pluginserver.HeldRecord, bool, error) {
	record, held, err := pluginserver.ReadHeldRecord(store, name)
	if err != nil {
		return pluginserver.HeldRecord{}, false, opererr.Wrap(err,
			"the held configuration of client "+name+" could not be read from the store",
			"it is the only copy the hub keeps of the configuration the client runs",
			fixCheckStore)
	}
	return record, held, nil
}

// readServedConfig reads the configuration the hub serves client name, as the
// managed server reads it. served is false when the store holds none.
func readServedConfig(store storage.Storage, name string) (content []byte, served bool, err error) {
	content, err = store.ReadFile(pluginserver.ClientConfigKey(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, opererr.Wrap(err,
			"the configuration this hub serves client "+name+" could not be read from the store",
			"its hash and its diff with the client's configuration are what the divergence views compare",
			fixCheckStore)
	}
	return content, true, nil
}

// divergenceRowOf words one held copy. hubConfig is nil when the hub serves
// the client no configuration.
func divergenceRowOf(name string, record pluginserver.HeldRecord, hubConfig []byte, serves func(string) bool) divergenceRow {
	row := divergenceRow{
		Client:        name,
		Declared:      serves(name),
		DeviceVersion: record.Version(),
		ReceivedAt:    record.ReceivedAt().UTC().Format(time.RFC3339),
		Revert:        divergenceRevertNone,
	}
	if hubConfig != nil {
		row.HubVersion = fleet.VersionHash(hubConfig)
	}
	if decision, decided := record.Revert(); decided {
		// The same test the hub's fetch answer makes for a client that sends
		// the copy's version (HandleConfigFetch), so pending is what the
		// client is offered.
		row.Revert = divergenceRevertVoid
		if decision.OfferedFor(record.Version(), row.HubVersion) {
			row.Revert = divergenceRevertPending
		}
	}
	if edited := record.EditedAt(); !edited.IsZero() {
		row.EditedAt = edited.UTC().Format(time.RFC3339)
	}
	return row
}

// revertVoidReason says why the revert decision of the copy of client name,
// version deviceVersion, is void: it was taken while the hub served the
// client another configuration than the one it serves now, hubVersion ("" for
// none). Its fix is the revert to decide again over the two hashes the show
// answers now, after the operator looked at the configuration the hub serves.
func revertVoidReason(name, deviceVersion string, decision pluginserver.RevertDecision, hubVersion string) *opererr.Error {
	now := "holds no configuration for it"
	if hubVersion != "" {
		now = "serves it version " + hubVersion
	}
	var fix textbuf.Buffer
	fix.Str("request fleet divergence ").Str(leafDivergenceName).Byte(' ')
	command.WriteInvocationValue(&fix, name)
	fix.Str(" revert ").Str(leafDivergenceVersion).Byte(' ').Str(deviceVersion)
	fix.Byte(' ').Str(leafDivergenceHubVersion).Byte(' ').Str(placeholderOr(hubVersion, placeholderHubVersion))
	return opererr.New(
		"the revert of client "+name+" is void: it was decided while the hub served it version "+decision.HubVersion()+
			", and the hub now "+now,
		"a client takes only the configuration the operator saw when deciding, so "+name+" is offered nothing and keeps holding; "+
			"look at the diff above, then decide again",
		fix.String())
}

// heldClientNames answers the clients store keeps a held copy for, sorted, as
// an operator error when the store cannot list them.
func heldClientNames(store storage.Storage) ([]string, error) {
	names, err := pluginserver.HeldClients(store)
	if err != nil {
		return nil, opererr.Wrap(err,
			"the held configurations could not be listed from the store",
			"each is the copy the hub keeps of the configuration a managed client holds",
			fixCheckStore)
	}
	slices.Sort(names)
	return names, nil
}

// divergenceDiff compares the configuration the hub serves client name with
// the one the client holds, over the whole tree with every secret masked. A
// refusal is an operator error that quotes no configuration text.
func divergenceDiff(name string, hubConfig, heldConfig []byte) (*config.ConfigDiff, error) {
	schema, declared, err := config.SchemaWithDeclaredPlugins(
		config.ConfigSource{Text: string(hubConfig)}, config.ConfigSource{Text: string(heldConfig)})
	if err != nil {
		return nil, opererr.Wrap(err,
			"the diff of client "+name+" is not available: the config schema to read it with could not be built",
			"the hub parses both configurations to compare them",
			"ze doctor")
	}
	before, err := divergenceSide(schema, declared, hubConfig, "the configuration this hub serves client "+name)
	if err != nil {
		return nil, err
	}
	after, err := divergenceSide(schema, declared, heldConfig, "the configuration client "+name+" holds")
	if err != nil {
		return nil, err
	}
	return infra.DiffConfigs(schema, before, after)
}

// divergenceSide parses one configuration of the diff. subject names it in
// the refusal. The parser's own refusal is not kept: it can quote the
// configuration, which carries secrets.
func divergenceSide(schema *config.Schema, declared *config.DeclaredSchemas, content []byte, subject string) (*infra.DiffSide, error) {
	tree, err := config.NewParser(schema).Parse(string(content))
	if err != nil {
		return nil, divergenceParseRefusal(subject, declared, err)
	}
	side, err := infra.NewDiffSide(tree)
	if err != nil {
		return nil, opererr.New(
			"the diff is not available: the bgp block of "+subject+" does not resolve on this hub",
			"the diff compares the resolved bgp values; the resolver's refusal is not shown because it can quote the configuration, which carries secrets",
			"ze doctor")
	}
	return side, nil
}

// divergenceParseRefusal words the refusal of a parse of subject. A name no
// schema defines, in a configuration that declares an external plugin this
// process holds no schema of, is that plugin's section: the hub parses only
// the sections of the plugins it started.
func divergenceParseRefusal(subject string, declared *config.DeclaredSchemas, refusal error) *opererr.Error {
	if config.RefusesUnknownNode(refusal) {
		if plugins := declared.WithoutSchema(); len(plugins) > 0 {
			return opererr.New(
				"the diff is not available: "+subject+" holds a section of the "+config.ExternalPluginNames(plugins)+
					", whose config schema this ze does not hold",
				"ze reads the config schema of an external plugin only from a plugin it started, and the diff parses both configurations; "+
					"declare the plugin in this hub's configuration so the restart starts it. The hashes and times still hold, and adopt and revert do not need the diff",
				"restart ze")
		}
	}
	return opererr.New(
		"the diff is not available: "+subject+" does not parse on this hub",
		"the diff parses both configurations; the parser's refusal is not shown because it can quote the configuration, which carries secrets",
		"ze doctor")
}

// divergenceChanges lists the entries of diff sorted by path, each once.
func divergenceChanges(diff *config.ConfigDiff) []divergenceChange {
	changes := make([]divergenceChange, 0, infra.DiffChangeCount(diff))
	for _, path := range slices.Sorted(maps.Keys(diff.Added)) {
		changes = append(changes, divergenceChange{Path: path, Change: changeAdded, New: diff.Added[path]})
	}
	for _, path := range slices.Sorted(maps.Keys(diff.Removed)) {
		changes = append(changes, divergenceChange{Path: path, Change: changeRemoved, Old: diff.Removed[path]})
	}
	for _, path := range slices.Sorted(maps.Keys(diff.Changed)) {
		pair := diff.Changed[path]
		changes = append(changes, divergenceChange{Path: path, Change: changeChanged, Old: pair.Old, New: pair.New})
	}
	slices.SortStableFunc(changes, func(a, b divergenceChange) int { return strings.Compare(a.Path, b.Path) })
	return changes
}

// daemonDivergenceList asks the running daemon for the list. answered is
// false when no daemon answered with one.
func daemonDivergenceList(ask divergenceAsk) (divergenceList, bool) {
	output, err := ask(pathDivergenceList)
	if err != nil {
		return divergenceList{}, false
	}
	var list divergenceList
	if decodeDivergence(output, &list) != nil {
		return divergenceList{}, false
	}
	// Guard: a document of another shape decodes with no clients key, so
	// only a list answers.
	if list.Clients == nil {
		return divergenceList{}, false
	}
	return list, true
}

// daemonDivergenceDetail asks the running daemon for the show of client name.
// answered is false when no daemon answered with the show of that client.
func daemonDivergenceDetail(ask divergenceAsk, name string) (divergenceDetail, bool) {
	output, err := ask(divergenceShowCommand(name))
	if err != nil {
		return divergenceDetail{}, false
	}
	var detail divergenceDetail
	if decodeDivergence(output, &detail) != nil {
		return divergenceDetail{}, false
	}
	// Guard: only the show of the client asked about answers.
	if detail.Client != name {
		return divergenceDetail{}, false
	}
	return detail, true
}

// decodeDivergence decodes a daemon's JSON answer strictly: a field neither
// show answers makes it another document.
func decodeDivergence(output string, into any) error {
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

// completeDivergenceClients completes the client name of the show from the
// held copies in the daemon's store. The name is the one value, after its
// keyword.
func completeDivergenceClients(_ *pluginserver.Server, completed []string) []pluginserver.Completion {
	if len(completed) != 1 {
		return nil
	}
	if completed[0] != leafDivergenceName {
		return nil
	}
	store := statestore.Store()
	if store == nil {
		return nil // A daemon with no store keeps no held copy.
	}
	names, err := heldClientNames(store)
	if err != nil {
		// Going on is correct: completion has no error answer, and the show
		// itself refuses with the same failure when it runs.
		logger().Warn("the held configurations could not be listed for completion",
			"code", diagnostic.CodeDoctorStoreIntegrity, "error", err)
		return nil
	}
	candidates := make([]pluginserver.Completion, 0, len(names))
	for _, name := range names {
		candidates = append(candidates, pluginserver.Completion{Value: name, ShortHelp: "Managed client whose held configuration this hub keeps"})
	}
	return candidates
}
