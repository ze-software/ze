// Design: docs/architecture/api/commands.md -- show fleet status, a local-data command the daemon serves under the same path
// Design: docs/architecture/fleet-config.md -- request fleet disable and enable: leaving and rejoining the fleet
// Related: baseline.go -- the managed flag, the active hash and the baseline this answer reports
// Related: register.go -- registers the commands, the status shape and its column order
// Related: client.go -- DeclaredEndpoint, the hub block these commands read, and the callbacks that report a connection and a hold

// `show fleet status` says where a device stands with its hub. Most of the
// answer is in the store, which any `ze` process reads: the managed flag, the
// hub the configuration declares, the baseline and the hash of the active
// configuration. The rest lives only in a running daemon: whether its hub
// client runs, whether it is connected, whether it holds a fetch. So the local
// route asks a running daemon first and draws its answer, and answers from the
// store alone, saying no daemon answered, when none does. A failed ask is
// never read as a client that is stopped.
//
// `request fleet disable` and `request fleet enable` take the device out of
// the fleet and put it back. Each stores the managed flag first. A daemon that
// started managed then stops or starts its hub client through the FleetTarget
// the hub installed; a daemon that started unmanaged holds no such target and
// changes the flag alone. With no daemon the local route writes the flag into
// the store it opens, and while a daemon owns the store it sends the same line
// to that daemon.

package managed

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// showFleetStatusCommand is the line the local route sends a running daemon.
const showFleetStatusCommand = "show fleet status"

// pathFleetDisable and pathFleetEnable are the two commands as an operator
// types them: what a refusal names first, and the line the local route sends
// the daemon that owns the store.
const (
	pathFleetDisable = "request fleet disable"
	pathFleetEnable  = "request fleet enable"
)

// What the client, connection and holding fields of an answer say.
const (
	// runtimeNoDaemon: the local route answered from the store, because no
	// daemon answered.
	runtimeNoDaemon = "no-daemon"
	// clientNotStarted: the daemon started unmanaged, so it owns no hub client
	// and starts none until its next start.
	clientNotStarted = "not-started"
	// clientRunning and clientStopped: the daemon started managed, and its hub
	// client runs or does not. clientStopping: a `request fleet disable` is
	// ending the client and waits for it, for instance while it applies a
	// configuration its hub sent.
	clientRunning  = "running"
	clientStopping = "stopping"
	clientStopped  = "stopped"
	// connectionUp: the hub accepted the client's authentication and that
	// connection has not ended.
	connectionUp   = "connected"
	connectionDown = "not-connected"
	// holdYes: the client's last fetch applied nothing, because the
	// configuration the device runs differs from its baseline or could not be
	// compared with it.
	holdYes = "holding"
	holdNo  = "not-holding"
)

// The note of a leave or rejoin answer: what the command did, in one sentence.
const (
	noteLeftClosed    = "this device left the fleet: its hub connection was closed"
	noteLeftUnlinked  = "this device left the fleet: its hub client was stopped, and it held no hub connection to close"
	noteLeftNoClient  = "this device left the fleet: it ran no hub client, so there was no hub connection to close"
	noteLeftStopped   = "this device left the fleet: no daemon runs, and the next ze start runs no hub client"
	noteJoinedStarted = "this device rejoined the fleet: its hub client started and fetches from the hub"
	noteJoinedRunning = "this device is in the fleet: its hub client already runs, so none was started"
	noteJoinsAtStart  = "this device rejoins the fleet at the next ze start: this daemon started unmanaged, so its hub connection starts then"
	noteJoinsStopped  = "this device rejoins the fleet at the next ze start: no daemon runs"
)

// fixCheckStore is the command that says why a store key cannot be read.
const fixCheckStore = "ze show data check"

// ClientState is what a daemon's hub client is doing. The zero value is a
// daemon that started managed and runs no client.
type ClientState struct {
	// Running says the client's goroutine runs: it is connected, connecting
	// or waiting to reconnect.
	Running bool
	// Stopping says a stop ended the client and waits for its goroutine to
	// return. A stopping client is not Running.
	Stopping bool
	// Connected says the hub accepted the client's authentication and that
	// connection has not ended.
	Connected bool
	// Holding says the client's last fetch applied nothing the hub sent.
	Holding bool
}

// FleetTarget is what the fleet commands act on in a daemon that started
// managed: the owner of its hub client. A daemon that started unmanaged
// installs none, which is how a handler tells the two apart. Every field MUST
// be set.
//
// StartClient and StopClient each take the managed flag write of the command,
// storeFlag, and MUST run it first and act on the client only when it
// succeeded, both under one lock the two share: the flag stored last and the
// client's state then always agree, whatever order two commands arrive in.
// When storeFlag fails they answer its error unchanged.
type FleetTarget struct {
	// StartClient runs storeFlag, then builds a hub client from the active
	// configuration and starts it. It answers false, with no error, when a
	// client already runs and none was started.
	StartClient func(storeFlag func() error) (bool, error)
	// StopClient runs storeFlag, then ends the hub client, returns once it has
	// ended, and answers its state before the stop. A configuration the client
	// was applying is applied, and its baseline stored, before StopClient
	// returns. Its only error is storeFlag's.
	StopClient func(storeFlag func() error) (ClientState, error)
	// ClientState answers what the hub client is doing now.
	ClientState func() ClientState
}

// fleetTarget holds the target while the daemon's hub client owner exists.
// Before that, after it, and in a daemon that started unmanaged, it is nil.
var fleetTarget atomic.Pointer[FleetTarget]

// InstallFleetTarget publishes the target the fleet commands act on. The
// daemon MUST call it once its hub client owner exists, and MUST call it with
// nil before it joins that owner and returns the store.
func InstallFleetTarget(target *FleetTarget) {
	if target == nil {
		fleetTarget.Store(nil)
		return
	}
	if target.StartClient == nil {
		panic("BUG: managed.InstallFleetTarget called with no StartClient")
	}
	if target.StopClient == nil {
		panic("BUG: managed.InstallFleetTarget called with no StopClient")
	}
	if target.ClientState == nil {
		panic("BUG: managed.InstallFleetTarget called with no ClientState")
	}
	fleetTarget.Store(target)
}

// clientReport is the three fields of an answer that only a running daemon
// fills: its hub client, that client's connection, and whether it holds.
type clientReport struct {
	client     string
	connection string
	holding    string
}

// noDaemonReport is the report of an answer no daemon gave.
var noDaemonReport = clientReport{client: runtimeNoDaemon, connection: runtimeNoDaemon, holding: runtimeNoDaemon}

// daemonClientReport is the report a daemon gives: its hub client's state, or,
// with no target, that it started unmanaged and so started no client.
func daemonClientReport(target *FleetTarget) clientReport {
	if target == nil {
		return clientReport{client: clientNotStarted, connection: connectionDown, holding: holdNo}
	}
	return reportOf(target.ClientState())
}

// reportOf words one state of a hub client.
func reportOf(state ClientState) clientReport {
	report := clientReport{client: clientStopped, connection: connectionDown, holding: holdNo}
	if state.Running {
		report.client = clientRunning
	}
	if state.Stopping {
		report.client = clientStopping
	}
	if state.Connected {
		report.connection = connectionUp
	}
	if state.Holding {
		report.holding = holdYes
	}
	return report
}

// fleetStatus is the `show fleet status` answer, one document. The first
// group is read from the store and the active configuration on every route.
// The last four say what a running daemon knows.
type fleetStatus struct {
	// Managed is meta/instance/managed: the device follows a hub.
	Managed bool `json:"managed"`
	// Frozen says an operator's change to the device's own configuration is
	// refused (storage.OwnConfigFrozen). It is true when the device is managed
	// and the daemon, or the stopped tool, runs the stored configuration; a
	// daemon started from a file or from stdin is never frozen.
	Frozen bool `json:"frozen"`
	// HubDeclared says the active configuration holds a hub-level client
	// block. HubName and HubAddress are that block's, empty without one.
	HubDeclared bool   `json:"hub-declared"`
	HubName     string `json:"hub-name"`
	HubAddress  string `json:"hub-address"`
	// BaselineRecorded says the store holds a baseline. BaselineHash is the
	// version hash of the last configuration the hub gave, empty without one.
	BaselineRecorded bool   `json:"baseline-recorded"`
	BaselineHash     string `json:"baseline-hash"`
	// ActiveRecorded says the store holds the device's own configuration.
	// ActiveHash is its version hash, empty without one.
	ActiveRecorded bool   `json:"active-recorded"`
	ActiveHash     string `json:"active-hash"`
	// Diverged says both hashes are recorded and differ. A device with no
	// baseline is not diverged.
	Diverged bool `json:"diverged"`
	// DaemonAnswered says a running daemon produced this document. It is also
	// how the local route recognizes a daemon's answer (decodeDaemonFleetStatus).
	DaemonAnswered bool `json:"daemon-answered"`
	// Client, Connection and Holding are the state of the daemon's hub client.
	// Client is running, stopping, stopped, not-started or no-daemon.
	Client     string `json:"client"`
	Connection string `json:"connection"`
	Holding    string `json:"holding"`
}

// errNotFleetStatus is a daemon answer that is not a daemon's fleet status.
var errNotFleetStatus = errors.New("the daemon's answer is not a fleet status a daemon produced")

// daemonAsk asks the running daemon for its fleet status. An error means no
// daemon answered with one. askDaemonOverSSH is the production ask; a test
// hands a stub.
type daemonAsk func() (fleetStatus, error)

// answerFleetStatus is the `show fleet status` handler on both routes.
func answerFleetStatus(validated command.ValidatedArgs) command.LocalAnswer {
	return answerFleetStatusWith(validated, askDaemonOverSSH)
}

// answerFleetStatusWith is the handler over one ask. In the operator's own
// process it draws the daemon's answer when a daemon gives one, and otherwise
// answers from the store. In the daemon it answers from the store and from the
// owner of its hub client: the daemon is the one being asked, so it asks
// nobody.
func answerFleetStatusWith(validated command.ValidatedArgs, ask daemonAsk) command.LocalAnswer {
	if !validated.Route().CallerLocal() {
		return storeFleetStatus(true, daemonClientReport(fleetTarget.Load()))
	}
	status, err := ask()
	if err != nil {
		// Going on is correct, so the error is not returned: a stopped daemon
		// is an ordinary state of a device, the store still answers, and the
		// answer says that no daemon did (daemon-answered, no-daemon).
		return storeFleetStatus(false, noDaemonReport)
	}
	return command.LocalAnswer{Payload: status}
}

// storeFleetStatus answers from the store of this host, with report in the
// three fields only a running hub client can fill.
func storeFleetStatus(daemonAnswered bool, report clientReport) command.LocalAnswer {
	folder := resolve.StoreDir("")
	store, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		refusal := opererr.Wrap(err,
			"show fleet status: the store was not read",
			"the store of this host could not be opened, so whether the device follows a hub is unknown",
			"ze doctor")
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	defer store.Close() //nolint:errcheck // read-only handle, nothing to flush

	status, err := readFleetStatus(store, frozenHandle(daemonAnswered, store))
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}
	status.DaemonAnswered = daemonAnswered
	status.Client = report.client
	status.Connection = report.connection
	status.Holding = report.holding
	return command.LocalAnswer{Payload: status}
}

// frozenHandle is the handle whose binding says whether the device's own
// configuration is frozen. In the daemon it is the daemon's own handle
// (statestore.Store), bound to the source it started from: a daemon started
// from a file is not frozen, which a fresh handle cannot know. In the
// operator's own process it is the handle the status opened, with no binding,
// as every stopped tool's.
func frozenHandle(inDaemon bool, opened storage.Storage) storage.Storage {
	if !inDaemon {
		return opened
	}
	// storage.Storage is statestore.Storage: the daemon publishes the handle
	// it owns, with its binding.
	if own := statestore.Store(); own != nil {
		return own
	}
	// No handle published: a test's daemon route. A daemon with no store
	// published none either, and its status refused at the open above.
	return opened
}

// readFleetStatus reads the store facts of the answer from store, and whether
// the device's own configuration is frozen from frozenOn, the handle the
// freeze would be asked through (frozenHandle). Each failure is an operator
// error naming what could not be read: a status with a guessed field would
// report a device as unmanaged, or in sync, when nobody looked.
func readFleetStatus(store, frozenOn storage.Storage) (fleetStatus, error) {
	managed, err := ManagedFlag(store)
	if err != nil {
		return fleetStatus{}, opererr.Wrap(err,
			"the managed flag ("+zefs.KeyInstanceManaged.Pattern+") could not be read from the store",
			"it says whether this device follows a hub, so its fleet status is unknown",
			fixCheckStore)
	}
	baseline, err := BaselineVersion(store)
	if err != nil {
		return fleetStatus{}, opererr.Wrap(err,
			"the baseline ("+zefs.KeyManagedBaseVersion.Pattern+") could not be read from the store",
			"it is the hash of the last configuration the hub gave, so whether this device differs from it is unknown",
			fixCheckStore)
	}
	content, stored, err := activeConfig(store)
	if err != nil {
		return fleetStatus{}, opererr.Wrap(err,
			"the configuration of this device could not be read from the store",
			"its hash and its hub block are part of the fleet status",
			fixCheckStore)
	}

	frozen, err := storage.OwnConfigFrozen(frozenOn)
	if err != nil {
		return fleetStatus{}, err
	}

	status := fleetStatus{
		Managed:          managed,
		Frozen:           frozen,
		BaselineRecorded: baseline.Recorded(),
		BaselineHash:     baseline.Hash(),
	}
	if !stored {
		return status, nil
	}
	active := versionOf(content)
	status.ActiveRecorded = true
	status.ActiveHash = active.Hash()
	status.Diverged = Diverged(active, baseline)

	hub, err := DeclaredEndpoint(content)
	if errors.Is(err, ErrNoHubClient) {
		return status, nil
	}
	if err != nil {
		return fleetStatus{}, opererr.Wrap(err,
			"the plugin hub block of this device's configuration could not be read",
			"it names the hub this device follows, which is part of the fleet status",
			"ze config validate")
	}
	status.HubDeclared = true
	status.HubName = hub.Name
	status.HubAddress = hub.Server
	return status, nil
}

// askDaemonOverSSH is the production daemonAsk: the SSH client every
// `ze <verb>` daemon command uses, with the stored credential and no prompt,
// asking for the dispatcher's JSON whatever display format the operator set.
func askDaemonOverSSH() (fleetStatus, error) {
	reached, err := sshclient.ReachDaemon("")
	if err != nil {
		return fleetStatus{}, err
	}
	output, err := sshclient.ExecCommandRaw(reached.Credentials, showFleetStatusCommand)
	if err != nil {
		return fleetStatus{}, err
	}
	return decodeDaemonFleetStatus(output)
}

// decodeDaemonFleetStatus reads a daemon's `show fleet status` JSON. An answer
// that is not a document a daemon marked as its own is no answer: drawing it
// would show an unmanaged device with a running daemon that no daemon reported.
func decodeDaemonFleetStatus(output string) (fleetStatus, error) {
	var status fleetStatus
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return fleetStatus{}, err
	}
	// Guard: daemon-answered is false in every JSON document of another shape,
	// and in a store-only answer, so only a daemon's own status passes.
	if !status.DaemonAnswered {
		return fleetStatus{}, errNotFleetStatus
	}
	return status, nil
}

// fleetChange is the answer of `request fleet disable` and `request fleet
// enable`: where the device stands once the command has run. Client and
// Connection use the words of the fleet status, and Note says what the command
// did.
type fleetChange struct {
	plugin.DataMarker
	// Managed is the managed flag the command stored.
	Managed bool `json:"managed"`
	// Client and Connection are the daemon's hub client and its connection
	// after the command. Client is never empty in an answer a handler gave,
	// which is how the local route recognizes one (changeFleetThroughDaemon).
	Client     string `json:"client"`
	Connection string `json:"connection"`
	Note       string `json:"note"`
}

// keyWrite writes one store key through the handle that owns the store: the
// store's own WriteKey in a process that opened it, statestore.Write in the
// daemon.
type keyWrite func(key string, data []byte) error

// storeManagedFlag stores the managed flag. It is the first act of a leave and
// of a rejoin, before the hub client is touched: a daemon that stopped between
// the two acts then starts as the operator asked.
func storeManagedFlag(path string, write keyWrite, managed bool) *opererr.Error {
	value := "false\n"
	if managed {
		value = managedFlagTrue + "\n"
	}
	key := zefs.KeyInstanceManaged.Pattern
	if err := write(key, []byte(value)); err != nil {
		return opererr.Wrap(err,
			path+": the managed flag ("+key+") was not stored",
			"the device stands with its hub as it did before the command, and its hub client was not changed",
			fixCheckStore)
	}
	return nil
}

// noArguments refuses a leave or rejoin typed with words after it. Both act on
// this device alone and take none.
func noArguments(path string, validated command.ValidatedArgs) *opererr.Error {
	if len(validated.Tokens()) == 0 {
		return nil
	}
	return opererr.New(
		path+": the command takes no argument",
		"it acts on this device alone, so nothing was changed",
		path)
}

// flagWrite is the managed flag write of a leave or a rejoin in a daemon,
// handed to the target, which runs it first under the lock its start and stop
// share. refusal keeps what the write answered, so the command tells a flag
// that was not stored from a client that did not start.
type flagWrite struct {
	path    string
	write   keyWrite
	managed bool
	refusal *opererr.Error
}

// store writes the flag, and is the storeFlag a FleetTarget runs.
func (f *flagWrite) store() error {
	f.refusal = storeManagedFlag(f.path, f.write, f.managed)
	if f.refusal == nil {
		return nil
	}
	return f.refusal
}

// failed answers the refusal of a target that answered err: the flag's own
// refusal when the write failed, and otherwise err, named as a failure of the
// command that left the flag as it was.
func (f *flagWrite) failed(err error) *opererr.Error {
	if f.refusal != nil {
		return f.refusal
	}
	return opererr.Wrap(err,
		f.path+": the hub client of this device was not changed",
		"the daemon answered an error it does not give for this command",
		"ze doctor")
}

// disableOnDaemon is `request fleet disable` in a daemon: the flag is stored
// as false FIRST, then the hub client is stopped through target and joined,
// both as one operation of the target. It needs no hub, no connection and no
// client. A nil target is a daemon that started unmanaged: it runs no hub
// client, and the flag is all there is to change.
func disableOnDaemon(write keyWrite, target *FleetTarget) (fleetChange, *opererr.Error) {
	flag := &flagWrite{path: pathFleetDisable, write: write, managed: false}
	if target == nil {
		if err := flag.store(); err != nil {
			return fleetChange{}, flag.refusal
		}
		return fleetChange{Client: clientNotStarted, Connection: connectionDown, Note: noteLeftNoClient}, nil
	}
	before, err := target.StopClient(flag.store)
	if err != nil {
		return fleetChange{}, flag.failed(err)
	}
	left := fleetChange{Client: clientStopped, Connection: connectionDown, Note: noteLeftNoClient}
	if before.Running {
		left.Note = noteLeftUnlinked
	}
	if before.Connected {
		left.Note = noteLeftClosed
	}
	return left, nil
}

// enableOnDaemon is `request fleet enable` in a daemon. It refuses, leaving
// the flag as it was, when the active configuration read names no hub. It then
// stores the flag as true and starts the hub client, both as one operation of
// target. A nil target is a daemon that started unmanaged: it owns no hub
// client, so the connection starts at the next `ze start`.
func enableOnDaemon(read storage.Storage, write keyWrite, target *FleetTarget) (fleetChange, *opererr.Error) {
	if refusal := requireDeclaredHub(read); refusal != nil {
		return fleetChange{}, refusal
	}
	flag := &flagWrite{path: pathFleetEnable, write: write, managed: true}
	if target == nil {
		if err := flag.store(); err != nil {
			return fleetChange{}, flag.refusal
		}
		return fleetChange{Managed: true, Client: clientNotStarted, Connection: connectionDown, Note: noteJoinsAtStart}, nil
	}
	started, err := target.StartClient(flag.store)
	if flag.refusal != nil {
		return fleetChange{}, flag.refusal
	}
	if err != nil {
		return fleetChange{}, clientStartRefusal(err)
	}
	report := reportOf(target.ClientState())
	joined := fleetChange{Managed: true, Client: report.client, Connection: report.connection, Note: noteJoinedRunning}
	if started {
		joined.Note = noteJoinedStarted
	}
	return joined, nil
}

// clientStartRefusal is the answer of a rejoin that stored the flag as true
// and whose hub client did not start: it says what failed and carries that
// failure's fix. A configuration that lost its hub block since the check is
// the refusal requireDeclaredHub gives; an operator error from the client's
// builder or its owner (a store that could not be read, a hub block that does
// not parse, a daemon that is stopping) is restated under the command.
func clientStartRefusal(err error) *opererr.Error {
	problem := pathFleetEnable + ": the device is managed again, and its hub client did not start"
	if errors.Is(err, ErrNoHubClient) {
		return opererr.New(problem,
			"the managed flag is stored as true, and the configuration this device runs now names no hub, so there is no hub to follow",
			hubClientMissingFix)
	}
	if cause, ok := errors.AsType[*opererr.Error](err); ok {
		return opererr.Wrap(cause.Unwrap(), problem,
			"the managed flag is stored as true, and "+cause.Problem()+": "+cause.Why(),
			cause.Fix())
	}
	return opererr.Wrap(err, problem,
		"the managed flag is stored as true, and this daemon could not start its hub client",
		"ze doctor")
}

// disableStopped is `request fleet disable` on a store no daemon holds: the
// flag is the whole change, and the next start follows it.
func disableStopped(store storage.Storage) (fleetChange, *opererr.Error) {
	if refusal := storeManagedFlag(pathFleetDisable, store.WriteKey, false); refusal != nil {
		return fleetChange{}, refusal
	}
	return fleetChange{Client: runtimeNoDaemon, Connection: runtimeNoDaemon, Note: noteLeftStopped}, nil
}

// enableStopped is `request fleet enable` on a store no daemon holds. It
// refuses as enableOnDaemon does, then stores the flag for the next start.
func enableStopped(store storage.Storage) (fleetChange, *opererr.Error) {
	if refusal := requireDeclaredHub(store); refusal != nil {
		return fleetChange{}, refusal
	}
	if refusal := storeManagedFlag(pathFleetEnable, store.WriteKey, true); refusal != nil {
		return fleetChange{}, refusal
	}
	return fleetChange{Managed: true, Client: runtimeNoDaemon, Connection: runtimeNoDaemon, Note: noteJoinsStopped}, nil
}

// requireDeclaredHub refuses a rejoin while the configuration the device runs
// names no hub: a managed device with no hub block is frozen and follows
// nobody, and the only way out of that state is the hub-side fix
// doctor-hub-client-missing names. A configuration that cannot be read refuses
// too: "names a hub" is claimed only from a configuration that was read.
func requireDeclaredHub(store storage.Storage) *opererr.Error {
	content, stored, err := activeConfig(store)
	if err != nil {
		return opererr.Wrap(err,
			pathFleetEnable+": the configuration of this device could not be read from the store",
			"whether it names a hub is unknown, so the device was left as it was",
			fixCheckStore)
	}
	if !stored {
		return noDeclaredHub()
	}
	_, err = DeclaredEndpoint(content)
	if errors.Is(err, ErrNoHubClient) {
		return noDeclaredHub()
	}
	if err != nil {
		return opererr.Wrap(err,
			pathFleetEnable+": the plugin hub block of this device's configuration could not be read",
			"whether it names a hub is unknown, so the device was left as it was",
			"ze config validate")
	}
	return nil
}

// noDeclaredHub is the refusal of a rejoin on a device whose configuration
// names no hub. Its fix is the one checkHubClientDeclared gives.
func noDeclaredHub() *opererr.Error {
	return opererr.New(
		pathFleetEnable+": the configuration of this device names no hub",
		"it has no plugin hub client block, so there is no hub to follow and the device was left as it was; "+
			"add the client block to this device's configuration on the hub, then provision the device again so that it fetches it",
		hubClientMissingFix)
}

// handleFleetDisable answers `request fleet disable` in the daemon. It writes
// the flag through the store handle the daemon owns: opening the store afresh
// here would be a second writer, which the owner lock refuses.
func handleFleetDisable(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if refusal := noArguments(pathFleetDisable, validated); refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	return fleetChangeResponse(disableOnDaemon(statestore.Write, fleetTarget.Load()))
}

// handleFleetEnable answers `request fleet enable` in the daemon. It reads the
// active configuration through a read-only handle beside the daemon's own, as
// the fleet status does, and writes the flag through the daemon's handle.
func handleFleetEnable(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if refusal := noArguments(pathFleetEnable, validated); refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	folder := resolve.StoreDir("")
	store, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err,
			pathFleetEnable+": the store was not read",
			"the configuration it holds says whether this device names a hub, so the device was left as it was",
			"ze doctor")), nil
	}
	defer store.Close() //nolint:errcheck // read-only handle, nothing to flush
	return fleetChangeResponse(enableOnDaemon(store, statestore.Write, fleetTarget.Load()))
}

// fleetChangeResponse answers a leave or rejoin, or its refusal.
func fleetChangeResponse(changed fleetChange, refusal *opererr.Error) (*plugin.Response, error) {
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: changed}, nil
}

// stoppedChange is a leave or rejoin on a store no daemon holds.
type stoppedChange func(store storage.Storage) (fleetChange, *opererr.Error)

// requestFleetDisableLocal is `ze request fleet disable`.
func requestFleetDisableLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathFleetDisable,
		changeFleetLocal(pathFleetDisable, validated, daemonroute.OverSSH, disableStopped))
}

// requestFleetEnableLocal is `ze request fleet enable`.
func requestFleetEnableLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathFleetEnable,
		changeFleetLocal(pathFleetEnable, validated, daemonroute.OverSSH, enableStopped))
}

// changeFleetLocal is a leave or rejoin in the operator's own process, over
// one daemon ask, so a test stubs the daemon. With the store free it opens the
// store as its writer and runs change. While a daemon owns the store that open
// answers storage.ErrBusy, and the line goes to that daemon, which acts on its
// own handle and on its hub client.
func changeFleetLocal(path string, validated command.ValidatedArgs, ask daemonroute.Ask, change stoppedChange) command.LocalAnswer {
	if refusal := noArguments(path, validated); refusal != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(refusal))
	}
	folder := resolve.StoreDir("")
	store, err := storage.Open(folder)
	if errors.Is(err, storage.ErrBusy) {
		return changeFleetThroughDaemon(path, folder, err, ask)
	}
	if err != nil {
		refusal := opererr.Wrap(err,
			path+": the store was not opened",
			"the store of this host holds the managed flag, so the device stands with its hub as it did before the command",
			"ze doctor")
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	defer store.Close() //nolint:errcheck // the one write of this command returned before the close

	var changed fleetChange
	var refusal *opererr.Error
	changed, refusal = change(store)
	if refusal != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(refusal))
	}
	return command.LocalAnswer{Payload: changed}
}

// changeFleetThroughDaemon sends path to the daemon that owns the store in
// folder and answers what it answered. busy is the store's refusal of the
// local open, which a refusal for no answer names first.
func changeFleetThroughDaemon(path, folder string, busy error, ask daemonroute.Ask) command.LocalAnswer {
	var changed fleetChange
	err := daemonroute.Send(ask, filepath.Join(folder, storage.TreeName), path, &changed)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		refusal := daemonroute.UnansweredRefusal("ze "+path, busy, unanswered)
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(fleetDaemonRefusal(path, err)))
	}
	// Guard: client is empty in every JSON document of another shape, so only
	// an answer a fleet handler gave passes.
	if changed.Client == "" {
		refusal := opererr.New(
			path+": the daemon's answer does not say what it did",
			"whether this device left or rejoined the fleet is not known",
			"ze show fleet status")
		return command.LocalRefusal(2, command.LocalErrorLine(refusal))
	}
	return command.LocalAnswer{Payload: changed}
}

// fleetDaemonRefusal is the local command's final refusal boundary. A native
// operator error keeps its cause and parts. Only a direct daemon refusal is
// decoded from wire text, preserving its complete problem, reason and fix.
// An incomplete or unknown answer keeps its cause and asks the operator to
// inspect the device's state before retrying: the command may have acted.
func fleetDaemonRefusal(path string, err error) *opererr.Error {
	switch refusal := err.(type) { //nolint:errorlint // Only these direct producer contracts carry the complete native refusal.
	case *opererr.Error:
		return refusal
	case *daemonroute.Refusal:
		native, decodeErr := opererr.FromRendered(refusal.Message)
		if decodeErr == nil {
			return native
		}
		return opererr.Wrap(errors.Join(err, decodeErr),
			"ze "+path+": the daemon's refusal is incomplete",
			"whether this device left or rejoined the fleet is not known; check its status before trying again",
			"ze show fleet status")
	}
	return opererr.Wrap(err,
		"ze "+path+": the daemon did not confirm the change",
		"whether this device left or rejoined the fleet is not known; check its status before trying again",
		"ze show fleet status")
}
