// Design: docs/architecture/fleet-config.md -- the hub's resolution of a managed client that holds
// Design: docs/architecture/api/commands.md -- show fleet divergence name, update fleet divergence name adopt, request fleet divergence name revert
// Related: register.go -- registers the four commands, their shapes and column order
// Related: cmd_fleet.go -- FleetTarget, the device-side target this one copies
// Related: cmd_divergence_show.go -- the two shows: the held copies, both hashes and the masked diff

// The four fleet divergence commands run on a hub. A managed client that holds
// uploads the configuration it runs, and the hub keeps it as that client's held
// copy under meta/config/client-<name>.conf/held. `show fleet divergence list`
// and `show fleet divergence name <client>` read those copies from the store,
// so any `ze` process answers them, and the daemon serves the same handlers
// under the same paths, asked first by the local route because only the
// daemon knows which clients its server serves. `update fleet divergence name <client> adopt version
// <hash>` and `request fleet divergence name <client> revert version <hash>
// hub-version <hash>` change what the hub serves or offers the client, so the
// daemon answers them through the DivergenceTarget the hub installs, and the
// local route sends the line to the daemon that owns the store, or, with the
// hub stopped, refuses with the fix `ze start`: only the running hub records
// them in its audit log.
//
// The two shows answer the hashes, the times and a secret-masked diff, never
// configuration text: both texts carry the client's secrets. The two
// shows live in cmd_divergence_show.go. Adopt makes the held copy the
// configuration the hub serves the client, so the client finds its hub current
// and stops holding. Revert stores the operator's decision with the copy,
// bound to the copy's device hash and to the hash of the configuration the
// hub serves the client, both as the operator saw them, and the hub's config-fetch answer offers it to the
// client only while both still hold; the client then applies the hub's
// configuration (client.go, applyReverted).

package managed

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
)

// The four commands as they are registered: the paths they are registered and
// answered under. The client is named by its selector keyword before the
// action (`... divergence name <client> adopt`), so each path but the list's
// carries the keyword name, and no path but the list's is a command an
// operator types: a refusal names the command as typed (divergenceCommandOf).
const (
	pathDivergenceList   = "show fleet divergence list"
	pathDivergenceShow   = "show fleet divergence name"
	pathDivergenceAdopt  = "update fleet divergence name adopt"
	pathDivergenceRevert = "request fleet divergence name revert"
)

// DivergenceTarget is what the adopt and revert commands act on in a daemon
// that serves managed clients: the store the daemon owns and its managed
// server. A daemon that serves no managed client installs none. Every field
// MUST be set.
type DivergenceTarget struct {
	// Store is the store handle the daemon owns. Adopt publishes the held
	// bytes as a new version of client-<name>.conf through it, and adopt and
	// revert change the held copy through it, under the writer lock of that
	// configuration.
	Store storage.Storage
	// ServesClient reports whether a server block of the running
	// configuration declares the named client. Adopt and revert refuse a
	// client it does not declare.
	ServesClient func(name string) bool
	// NotifyConfigChanged tells the named client, when it is connected, that
	// its configuration changed, so it fetches. Revert calls it after storing
	// its decision, because it writes no client-<name>.conf.
	NotifyConfigChanged func(name string)
}

// divergenceTarget holds the target while the daemon's managed server runs.
// Before that, after it, and in a daemon that serves no managed client, it is
// nil.
var divergenceTarget atomic.Pointer[DivergenceTarget]

// InstallDivergenceTarget publishes the target the adopt and revert commands
// act on. The daemon MUST call it once its managed server serves, and MUST call
// it with nil before it stops that server and closes the store.
func InstallDivergenceTarget(target *DivergenceTarget) {
	if target == nil {
		divergenceTarget.Store(nil)
		return
	}
	if target.Store == nil {
		panic("BUG: managed.InstallDivergenceTarget called with no Store")
	}
	if target.ServesClient == nil {
		panic("BUG: managed.InstallDivergenceTarget called with no ServesClient")
	}
	if target.NotifyConfigChanged == nil {
		panic("BUG: managed.InstallDivergenceTarget called with no NotifyConfigChanged")
	}
	divergenceTarget.Store(target)
}

// divergenceReverted is the answer of revert: the client, the device hash D
// of the held copy the client is to discard, and the hash V of the
// configuration the hub serves it, the one the client takes.
type divergenceReverted struct {
	plugin.DataMarker
	Client        string `json:"client"`
	DeviceVersion string `json:"device-version"`
	HubVersion    string `json:"hub-version"`
}

// handleDivergenceRevert answers `request fleet divergence name <client>
// revert version <hash> hub-version <hash>` in the daemon. It stores the decision on the store
// handle the daemon owns, then tells the client, which takes the revert at
// the fetch that follows. The dispatcher records the audit action its
// registration declares once this answers.
func handleDivergenceRevert(cmdCtx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	client, version, hubVersion, refusal := revertArgs(validated)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	target := divergenceTarget.Load()
	if target == nil {
		return plugin.OperatorErrorResponse(opererr.New(
			revertProblem(client),
			"this daemon runs no managed server now: no server block declares a client, or the daemon is starting or stopping, "+
				"so it serves no client a revert could be decided for",
			"ze doctor")), nil
	}
	var user string
	if cmdCtx != nil {
		user = cmdCtx.Username
	}
	reverted, refusal := revertHeld(target.Store, target.ServesClient, client, version, hubVersion, user, time.Now())
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	// The decision writes no client-<name>.conf, so the store's write
	// observer tells the client nothing: it is told here. A client that is
	// not connected takes the revert at its next connection's fetch.
	target.NotifyConfigChanged(client)
	return &plugin.Response{Status: plugin.StatusDone, Data: reverted}, nil
}

// requestDivergenceRevertLocal is `ze request fleet divergence name <client>
// revert version <hash> hub-version <hash>`.
func requestDivergenceRevertLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathDivergenceRevert, revertLocal(validated, daemonroute.OverSSH))
}

// revertLocal is revert in the operator's own process, over one daemon ask.
// While a daemon owns the store, opening it answers storage.ErrBusy, and the
// line goes to that daemon, which decides the revert on its own handle,
// records it in its audit log and tells the client. With the store free the
// hub is stopped, and the revert is refused: only the running hub records the
// decision in the audit log it keeps and offers it to the client, and a
// client fetches nothing from a stopped hub, so deciding later loses nothing.
func revertLocal(validated command.ValidatedArgs, ask daemonroute.Ask) command.LocalAnswer {
	client, version, hubVersion, refusal := revertArgs(validated)
	if refusal != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(refusal))
	}
	folder := resolve.StoreDir("")
	store, err := storage.Open(folder)
	if errors.Is(err, storage.ErrBusy) {
		return revertThroughDaemon(folder, err, ask, client, version, hubVersion)
	}
	if err != nil {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.Wrap(err,
			revertProblem(client),
			"the store of this host could not be opened, and it keeps the copy the decision is stored with",
			"ze doctor")))
	}
	if closeErr := store.Close(); closeErr != nil {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.Wrap(closeErr,
			revertProblem(client),
			"the store of this host was opened to find the hub, and could not be closed again",
			"ze doctor")))
	}
	return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
		revertProblem(client),
		"the hub is stopped: a revert is decided by the running hub, which records it in its audit log and offers it to "+
			client+" when it fetches, and a client fetches nothing from a stopped hub. Start the hub, then run the revert again",
		"ze start")))
}

// revertThroughDaemon sends the revert of client at version and hubVersion to
// the daemon that owns the store in folder and answers what it answered. busy is the
// store's refusal of the local open, which a refusal for no answer names
// first.
func revertThroughDaemon(folder string, busy error, ask daemonroute.Ask, client, version, hubVersion string) command.LocalAnswer {
	var line textbuf.Buffer
	line.Str(divergenceCommandOf(pathDivergenceRevert, client))
	line.Byte(' ').Str(leafDivergenceVersion).Byte(' ').Str(version)
	line.Byte(' ').Str(leafDivergenceHubVersion).Byte(' ').Str(hubVersion)

	var reverted divergenceReverted
	err := daemonroute.Send(ask, filepath.Join(folder, storage.TreeName), line.String(), &reverted)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		refusal := daemonroute.UnansweredRefusal("ze "+divergenceCommandOf(pathDivergenceRevert, client), busy, unanswered)
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(revertDaemonRefusal(client, err)))
	}
	// Guard: client is empty in every JSON document of another shape, so only
	// the answer of this revert passes.
	if reverted.Client != client {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.New(
			"ze "+divergenceCommandOf(pathDivergenceRevert, client)+": the daemon's answer does not say what it did",
			revertUnknown(client),
			divergenceShowCommand(client))))
	}
	return command.LocalAnswer{Payload: reverted}
}

// revertDaemonRefusal is the local revert's final refusal boundary, as
// adoptDaemonRefusal is adopt's: a native operator error keeps its parts, a
// daemon refusal is decoded from its wire text, and an incomplete or unknown
// answer sends the operator to the show, because the revert may have been
// decided.
func revertDaemonRefusal(client string, err error) *opererr.Error {
	switch refusal := err.(type) { //nolint:errorlint // Only these direct producer contracts carry the complete native refusal.
	case *opererr.Error:
		return refusal
	case *daemonroute.Refusal:
		native, decodeErr := opererr.FromRendered(refusal.Message)
		if decodeErr == nil {
			return native
		}
		return opererr.Wrap(errors.Join(err, decodeErr),
			"ze "+divergenceCommandOf(pathDivergenceRevert, client)+": the daemon's refusal is incomplete", revertUnknown(client), divergenceShowCommand(client))
	}
	return opererr.Wrap(err,
		"ze "+divergenceCommandOf(pathDivergenceRevert, client)+": the daemon did not confirm the revert", revertUnknown(client), divergenceShowCommand(client))
}

// revertUnknown is the reason of a revert whose outcome the local route
// cannot tell.
func revertUnknown(client string) string {
	return "whether the hub stored the revert of " + client + " is not known; the show answers revert pending when it did"
}

// revertArgs answers the client, the device hash and the hub hash a revert
// names. The three leaves are mandatory in the YANG, so the validator refused
// a line without one before; the refusal here answers a caller that bypassed
// it. A word the command does not declare is refused (divergenceClientArg).
func revertArgs(validated command.ValidatedArgs) (client, version, hubVersion string, refusal *opererr.Error) {
	client, named, refusal := divergenceClientArg(pathDivergenceRevert, validated)
	if refusal != nil {
		return "", "", "", refusal
	}
	if !named {
		return "", "", "", opererr.New(
			divergenceCommandOf(pathDivergenceRevert, "")+": no client was named",
			"the command reverts the held configuration of one managed client",
			pathDivergenceList)
	}
	version, given := validated.Value(leafDivergenceVersion)
	if !given {
		return "", "", "", opererr.New(
			revertProblem(client),
			"the command reverts only the held copy whose device hash the operator saw, and no hash was given",
			divergenceShowCommand(client))
	}
	hubVersion, given = validated.Value(leafDivergenceHubVersion)
	if !given {
		return "", "", "", opererr.New(
			revertProblem(client),
			"the command reverts the client only to the hub configuration whose hash the operator saw, and no hub hash was given",
			divergenceShowCommand(client))
	}
	return client, version, hubVersion, nil
}

// revertHeld stores the decision that client drops its held copy and takes
// the configuration the hub serves it. Under one guard, the writer lock of
// client-<client>.conf, it reads the copy, refuses unless its device hash is
// version, reads the configuration the hub serves the client, refuses unless
// it hashes to hubVersion, and writes the copy back carrying the decision
// bound to version (D) and hubVersion (V), taken by user at decidedAt. Both
// hashes are the ones the operator saw in the show, so the decision never
// binds a configuration the operator did not see. The config-held arm and
// every writer of client-<client>.conf take the same lock, so a newer copy or
// a hub write lands either before, and the revert is refused, or after, and
// voids the decision. serves is the served-client predicate of the running
// server. The caller tells the client once this answers.
func revertHeld(store storage.Storage, serves func(string) bool, client, version, hubVersion, user string, decidedAt time.Time) (divergenceReverted, *opererr.Error) {
	if refusal := storagecli.ServedClientRefusal(revertProblem(client), client, serves); refusal != nil {
		return divergenceReverted{}, refusal
	}
	configName := pluginserver.ClientConfigKey(client)
	guard, err := store.AcquireLock(configName)
	if err != nil {
		return divergenceReverted{}, opererr.Wrap(err,
			revertProblem(client),
			"the writer lock of "+configName+" was not taken, so no decision was stored",
			"ze doctor")
	}
	reverted, refusal := revertHeldLocked(guard, client, version, hubVersion, user, decidedAt)
	releaseErr := guard.Release()
	if releaseErr == nil {
		return reverted, refusal
	}
	cause := releaseErr
	if refusal != nil {
		cause = errors.Join(refusal, releaseErr)
	}
	return divergenceReverted{}, opererr.Wrap(cause,
		revertProblem(client),
		"the store could not finish the change of the copy of "+client+", so whether the decision was stored is not known; "+
			"the show answers revert pending when it was",
		divergenceShowCommand(client))
}

// revertHeldLocked is revertHeld under guard, which the caller MUST hold and
// MUST release for the decision to be published. The decision is bound to
// hubVersion, the hash the operator saw, never to the configuration as it
// stands when this runs: a configuration that hashes otherwise is refused.
func revertHeldLocked(guard storage.WriteGuard, client, version, hubVersion, user string, decidedAt time.Time) (divergenceReverted, *opererr.Error) {
	record, held, err := pluginserver.ReadHeldRecordWithGuard(guard, client)
	if err != nil {
		return divergenceReverted{}, opererr.Wrap(err,
			revertProblem(client),
			"the hub's copy of the configuration "+client+" holds could not be read, so no decision was stored",
			"ze doctor")
	}
	if !held {
		return divergenceReverted{}, opererr.New(
			revertProblem(client),
			"the hub keeps no copy of a configuration "+client+" holds: the client does not hold, or its copy was adopted or cleared",
			pathDivergenceList)
	}
	if record.Version() != version {
		return divergenceReverted{}, opererr.New(
			revertProblem(client),
			"the copy the hub keeps for "+client+" is version "+record.Version()+", not "+version+
				": the client sent another one after that version was shown, so no decision was stored",
			divergenceShowCommand(client))
	}
	configName := pluginserver.ClientConfigKey(client)
	served, err := guard.ReadFile(configName)
	if errors.Is(err, fs.ErrNotExist) {
		return divergenceReverted{}, opererr.New(
			revertProblem(client),
			"this hub holds no configuration for "+client+", so there is none the client could take in place of its own",
			"show config list")
	}
	if err != nil {
		return divergenceReverted{}, opererr.Wrap(err,
			revertProblem(client),
			"the configuration this hub serves "+client+" could not be read, so no decision was stored",
			fixCheckStore)
	}
	if servedVersion := fleet.VersionHash(served); servedVersion != hubVersion {
		return divergenceReverted{}, opererr.New(
			revertProblem(client),
			"the configuration this hub serves "+client+" is version "+servedVersion+", not "+hubVersion+
				": it was written after that version was shown, so no decision was stored",
			divergenceShowCommand(client))
	}
	if err := pluginserver.WriteHeldRecordWithGuard(guard, client, record.WithRevert(hubVersion, user, decidedAt)); err != nil {
		return divergenceReverted{}, opererr.Wrap(err,
			revertProblem(client),
			"the store refused the decision, so the copy is unchanged and "+client+" keeps holding",
			"ze doctor")
	}
	return divergenceReverted{Client: client, DeviceVersion: version, HubVersion: hubVersion}, nil
}

// revertProblem is the first line of a refused revert of client.
func revertProblem(client string) string {
	return divergenceCommandOf(pathDivergenceRevert, client) + ": the held configuration of " + client + " was not reverted"
}

// leafDivergenceVersion is the leaf of adopt and revert that names the device
// hash of the held copy the operator saw.
const leafDivergenceVersion = "version"

// leafDivergenceHubVersion is the leaf of revert that names the hash of the
// configuration the hub serves the client, as the operator saw it.
const leafDivergenceHubVersion = "hub-version"

// fixEditClientConfig is the fix of an adopt refused by a staged change of
// the client's configuration: the editor of that configuration, where the
// change is committed or discarded (docs/guide/fleet-config.md).
const fixEditClientConfig = "ze config edit "

// divergenceAdopted is the answer of adopt: the client, the device hash of
// the held copy the hub now serves it, and the stamp of the version of
// client-<name>.conf that copy became.
type divergenceAdopted struct {
	plugin.DataMarker
	Client        string `json:"client"`
	DeviceVersion string `json:"device-version"`
	Stamp         string `json:"stamp"`
}

// handleDivergenceAdopt answers `update fleet divergence name <client> adopt
// version <hash>` in the daemon. It acts on the store handle the daemon owns,
// so the write observer of the hub's managed server tells the client.
func handleDivergenceAdopt(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	client, version, refusal := adoptArgs(validated)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	target := divergenceTarget.Load()
	if target == nil {
		return plugin.OperatorErrorResponse(opererr.New(
			adoptProblem(client),
			"this daemon runs no managed server now: no server block declares a client, or the daemon is starting or stopping, "+
				"so it serves no client a copy could be adopted for",
			"ze doctor")), nil
	}
	adopted, refusal := adoptHeld(target.Store, target.ServesClient, client, version)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: adopted}, nil
}

// updateDivergenceAdoptLocal is `ze update fleet divergence name <client>
// adopt version <hash>`.
func updateDivergenceAdoptLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathDivergenceAdopt, adoptLocal(validated, daemonroute.OverSSH))
}

// adoptLocal is adopt in the operator's own process, over one daemon ask.
// While a daemon owns the store the open answers storage.ErrBusy, and the line
// goes to that daemon, which adopts on its own handle, records the audit
// action and tells the client. With the store free the hub is stopped, and
// adopt is refused: only the running hub records the decision in its audit
// log, and a resolution of a hold no audit log names is not taken. Revert
// refuses the same case for the same reason (revertLocal).
func adoptLocal(validated command.ValidatedArgs, ask daemonroute.Ask) command.LocalAnswer {
	client, version, refusal := adoptArgs(validated)
	if refusal != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(refusal))
	}
	folder := resolve.StoreDir("")
	store, err := storage.Open(folder)
	if errors.Is(err, storage.ErrBusy) {
		return adoptThroughDaemon(folder, err, ask, client, version)
	}
	if err != nil {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.Wrap(err,
			adoptProblem(client),
			"the store of this host could not be opened, and it keeps both the copy and the configuration it would become",
			"ze doctor")))
	}
	if closeErr := store.Close(); closeErr != nil {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.Wrap(closeErr,
			adoptProblem(client),
			"the store of this host was opened to find the hub, and could not be closed again",
			"ze doctor")))
	}
	return command.LocalRefusal(1, command.LocalErrorLine(opererr.New(
		adoptProblem(client),
		"the hub is stopped: an adopt is taken by the running hub, which records it in its audit log and tells "+
			client+" to fetch the adopted configuration. Start the hub, then run the adopt again",
		"ze start")))
}

// adoptThroughDaemon sends the adopt of client at version to the daemon that
// owns the store in folder and answers what it answered. busy is the store's
// refusal of the local open, which a refusal for no answer names first.
func adoptThroughDaemon(folder string, busy error, ask daemonroute.Ask, client, version string) command.LocalAnswer {
	var line textbuf.Buffer
	line.Str(divergenceCommandOf(pathDivergenceAdopt, client))
	line.Byte(' ').Str(leafDivergenceVersion).Byte(' ').Str(version)

	var adopted divergenceAdopted
	err := daemonroute.Send(ask, filepath.Join(folder, storage.TreeName), line.String(), &adopted)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		refusal := daemonroute.UnansweredRefusal("ze "+divergenceCommandOf(pathDivergenceAdopt, client), busy, unanswered)
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(adoptDaemonRefusal(client, err)))
	}
	// Guard: client is empty in every JSON document of another shape, so only
	// the answer of this adopt passes.
	if adopted.Client != client {
		return command.LocalRefusal(2, command.LocalErrorLine(opererr.New(
			"ze "+divergenceCommandOf(pathDivergenceAdopt, client)+": the daemon's answer does not say what it did",
			"whether the hub now serves the held configuration of "+client+" is not known; it does when the list no longer names "+client,
			pathDivergenceList)))
	}
	return command.LocalAnswer{Payload: adopted}
}

// adoptDaemonRefusal is the local adopt's final refusal boundary, as
// fleetDaemonRefusal is the fleet commands': a native operator error keeps its
// parts, a daemon refusal is decoded from its wire text, and an incomplete or
// unknown answer sends the operator to the list, because the adopt may have
// run.
func adoptDaemonRefusal(client string, err error) *opererr.Error {
	unknown := "whether the hub now serves the held configuration of " + client +
		" is not known; it does when the list no longer names " + client
	switch refusal := err.(type) { //nolint:errorlint // Only these direct producer contracts carry the complete native refusal.
	case *opererr.Error:
		return refusal
	case *daemonroute.Refusal:
		native, decodeErr := opererr.FromRendered(refusal.Message)
		if decodeErr == nil {
			return native
		}
		return opererr.Wrap(errors.Join(err, decodeErr),
			"ze "+divergenceCommandOf(pathDivergenceAdopt, client)+": the daemon's refusal is incomplete", unknown, pathDivergenceList)
	}
	return opererr.Wrap(err,
		"ze "+divergenceCommandOf(pathDivergenceAdopt, client)+": the daemon did not confirm the adopt", unknown, pathDivergenceList)
}

// adoptArgs answers the client and the device hash an adopt names. Both
// leaves are mandatory in the YANG, so the validator refused a line without
// either before; the refusal here answers a caller that bypassed it. A word
// the command does not declare is refused (divergenceClientArg).
func adoptArgs(validated command.ValidatedArgs) (client, version string, refusal *opererr.Error) {
	client, named, refusal := divergenceClientArg(pathDivergenceAdopt, validated)
	if refusal != nil {
		return "", "", refusal
	}
	if !named {
		return "", "", opererr.New(
			divergenceCommandOf(pathDivergenceAdopt, "")+": no client was named",
			"the command adopts the held configuration of one managed client",
			pathDivergenceList)
	}
	version, given := validated.Value(leafDivergenceVersion)
	if !given {
		return "", "", opererr.New(
			adoptProblem(client),
			"the command adopts only the held copy whose device hash the operator saw, and no hash was given",
			divergenceShowCommand(client))
	}
	return client, version, nil
}

// adoptHeld makes the held copy of client the configuration the hub serves
// it. Under one guard, the writer lock of client-<client>.conf, it reads the
// copy, refuses unless its device hash is version, publishes its bytes as a
// new active version and removes the copy. The config-held arm takes the same
// lock to write a newer copy, so holding one guard across the three is what
// keeps that copy out of the middle: it either lands before, and the hash
// check refuses, or after, and is kept. serves is the served-client predicate
// of the running server: a stopped hub adopts nothing (adoptLocal). The
// running hub's store has a write observer, which tells the client once the
// guard is released.
func adoptHeld(store storage.Storage, serves func(string) bool, client, version string) (divergenceAdopted, *opererr.Error) {
	if refusal := storagecli.ServedClientRefusal(adoptProblem(client), client, serves); refusal != nil {
		return divergenceAdopted{}, refusal
	}
	configName := pluginserver.ClientConfigKey(client)
	guard, err := store.AcquireLock(configName)
	if err != nil {
		return divergenceAdopted{}, opererr.Wrap(err,
			adoptProblem(client),
			"the writer lock of "+configName+" was not taken, so nothing was written and the copy is kept",
			"ze doctor")
	}
	adopted, refusal := adoptHeldLocked(store, guard, client, version)
	releaseErr := guard.Release()
	if releaseErr == nil {
		return adopted, refusal
	}
	cause := releaseErr
	if refusal != nil {
		cause = errors.Join(refusal, releaseErr)
	}
	return divergenceAdopted{}, opererr.Wrap(cause,
		adoptProblem(client),
		"the store could not finish the change of "+configName+", so whether the hub now serves the held configuration is not known; "+
			"it does when the list no longer names "+client,
		pathDivergenceList)
}

// adoptHeldLocked is adoptHeld under guard, which the caller MUST hold and
// MUST release for the adopt to be published.
func adoptHeldLocked(store storage.Storage, guard storage.WriteGuard, client, version string) (divergenceAdopted, *opererr.Error) {
	record, held, err := pluginserver.ReadHeldRecordWithGuard(guard, client)
	if err != nil {
		return divergenceAdopted{}, opererr.Wrap(err,
			adoptProblem(client),
			"the hub's copy of the configuration "+client+" holds could not be read, so nothing was written",
			"ze doctor")
	}
	if !held {
		return divergenceAdopted{}, opererr.New(
			adoptProblem(client),
			"the hub keeps no copy of a configuration "+client+" holds: the client does not hold, or its copy was adopted or cleared",
			pathDivergenceList)
	}
	if record.Version() != version {
		return divergenceAdopted{}, opererr.New(
			adoptProblem(client),
			"the copy the hub keeps for "+client+" is version "+record.Version()+", not "+version+
				": the client sent another one after that version was shown, so nothing was written",
			divergenceShowCommand(client))
	}
	configName := pluginserver.ClientConfigKey(client)
	stamp, err := storage.CommitConfigWithGuard(store, guard, configName, record.Config())
	if errors.Is(err, storage.ErrCandidateExists) {
		return divergenceAdopted{}, opererr.Wrap(err,
			adoptProblem(client),
			"a change to "+configName+" is already staged, and adopting would replace it; commit or discard it first. The copy is kept",
			fixEditClientConfig+configName)
	}
	if refusal, given := storage.FreezeRefusal(err); given {
		return divergenceAdopted{}, refusal
	}
	if err != nil {
		return divergenceAdopted{}, opererr.Wrap(err,
			adoptProblem(client),
			"the store refused the new version of "+configName+", so it is unchanged and the copy is kept",
			"ze doctor")
	}
	if err := guard.RemoveKey(pluginserver.HeldRecordKey(client)); err != nil {
		return divergenceAdopted{}, opererr.Wrap(err,
			divergenceCommandOf(pathDivergenceAdopt, client)+": the hub serves "+client+" its held configuration as version "+stamp+", and its copy was not removed",
			"the copy is removed when "+client+" next fetches and the hub answers that it is current",
			"ze doctor")
	}
	return divergenceAdopted{Client: client, DeviceVersion: record.Version(), Stamp: stamp}, nil
}

// adoptProblem is the first line of a refused adopt of client.
func adoptProblem(client string) string {
	return divergenceCommandOf(pathDivergenceAdopt, client) + ": the held configuration of " + client + " was not adopted"
}

// divergenceShowCommand is the show of client, as an operator types it: the
// fix of a refusal whose answer that show gives.
func divergenceShowCommand(client string) string {
	return divergenceCommandOf(pathDivergenceShow, client)
}

// placeholderClient stands for the client in a command written for a line
// that named none.
const placeholderClient = "<client>"

// divergenceCommandOf is a divergence command of client as an operator types
// it, without its hashes: path, the registered path of the show, the adopt or
// the revert, with the client written after its name keyword, as `request
// fleet divergence name edge-01 revert`. It is the one writer of the problem
// line of every refusal of those commands, and of the line the local route
// sends the daemon. An empty client is written as placeholderClient, for the
// refusal of a line that named none. path MUST hold the name keyword, as the
// three paths do.
func divergenceCommandOf(path, client string) string {
	selector, action, _ := strings.Cut(path, " "+leafDivergenceName)
	var line textbuf.Buffer
	line.Str(selector).Byte(' ').Str(leafDivergenceName).Byte(' ')
	if client == "" {
		line.Str(placeholderClient)
	} else {
		command.WriteInvocationValue(&line, client)
	}
	line.Str(action)
	return line.String()
}
