// Design: docs/architecture/fleet-config.md -- the freeze of a managed device's own configuration.
// Related: pointer.go -- WriteCandidateVersionWithGuard asks this admission; StageMachineCandidate does not.
// Related: publication_intent.go -- readAdmissionKey, the key read this admission shares.
// Related: source.go -- the binding that names a daemon's own configuration.
// Related: config_name.go -- InstanceConfigName, the name of a stopped device's own configuration.

package storage

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// managedFlagSet is the one value of meta/instance/managed that means the
// device follows a hub.
const managedFlagSet = "true"

// ErrConfigFrozen is the refusal of an operator's change to the stored
// configuration of a device that follows a hub. It is an operator error, and
// its fix is the command that takes the device out of the fleet. Every route
// that stages through WriteCandidateVersion or WriteCandidateVersionWithGuard
// returns it, wrapped or not, so errors.Is finds it and the fix stays the last
// line of the text.
var ErrConfigFrozen = opererr.New(
	"the configuration was not changed: this device is managed by a hub",
	"local changes are frozen while it follows a hub, so that a hub push cannot overwrite one",
	"request fleet disable")

// ManagedFlagSet reports whether the bytes stored under meta/instance/managed
// say that the device follows a hub. It is the one reading of that value: the
// freeze and the managed component (managed.ManagedFlag) both ask it. The
// caller reads the key and MUST decide what a failed read means before it
// calls: a missing key is "not managed", and any other read failure is not an
// answer at all. Safe for concurrent use.
func ManagedFlagSet(stored []byte) bool {
	return strings.TrimSpace(string(stored)) == managedFlagSet
}

// refuseFrozenConfigLocked is the freeze: it refuses to stage a change to the
// device's own stored configuration while meta/instance/managed is true, and
// admits every other write. It reads the keys through held, the guard the
// staging already holds, and writes nothing, so a refused write leaves no
// version and no candidate pointer. The caller MUST hold held, the config
// lock of configPath, through the staging this admits.
//
// The freeze is best effort: it tells an operator early. Routes that never
// stage a version (a raw key write, a full-store restore, a rename) are not
// refused here, and the managed client's hold is what keeps a local change
// from being overwritten (docs/architecture/fleet-config.md).
//
// Which configuration is the device's own is decided by the handle
// (configFrozen). A key the decision needs and the store cannot read refuses
// the write, naming the key: a store that cannot answer it is not safe to
// write, and reading the failure as "not managed" would lift the freeze
// exactly when the store is damaged.
func refuseFrozenConfigLocked(store Storage, held WriteGuard, configPath string) error {
	frozen, unknown := configFrozen(store, held.ReadKey, func(own string) bool {
		return sameConfigName(configPath, own)
	})
	if unknown != nil {
		return &freezeRefusal{refusal: opererr.Wrap(unknown.cause,
			"the configuration was not changed: "+unknown.key+" could not be read from the store",
			unknown.refusalWhy,
			fixCheckStoreKeys)}
	}
	if frozen {
		return &freezeRefusal{refusal: ErrConfigFrozen}
	}
	return nil
}

// freezeRefusal marks an operator error the freeze produced, so that a route
// which wraps every other failure in its own operator error can answer this
// one as it stands (FreezeRefusal). Unwrap answers the operator error, so
// errors.Is(err, ErrConfigFrozen) still finds the freeze.
type freezeRefusal struct {
	refusal *opererr.Error
}

func (f *freezeRefusal) Error() string { return f.refusal.Error() }

func (f *freezeRefusal) Unwrap() error { return f.refusal }

// FreezeRefusal answers the operator error the freeze produced, when err holds
// one, and false otherwise. Only the freeze's own refusals are answered: an
// operator error some other layer put in the chain (a reload that refused the
// configuration) is not, so the route that wraps it keeps its own first line.
// A route that answers a staging failure MUST ask this before it wraps the
// failure, or the answer ends on its own fix instead of the freeze's.
func FreezeRefusal(err error) (*opererr.Error, bool) {
	marked, found := errors.AsType[*freezeRefusal](err)
	if !found {
		return nil, false
	}
	return marked.refusal, true
}

// OwnConfigFrozen reports whether the device's own configuration is frozen
// for store: whether an operator's change to it, staged through this handle,
// is refused. It applies the rule the freeze applies (configFrozen) through
// store's own key reads, so the answer is the freeze's for that handle: a
// daemon's handle bound to a file or to stdin is never frozen, one bound to
// its stored configuration and a stopped tool's handle are frozen when the
// device is managed. A key the answer needs and the store cannot read is an
// operator error naming the key. Safe for concurrent use as store is.
func OwnConfigFrozen(store Storage) (bool, error) {
	frozen, unknown := configFrozen(store, store.ReadKey, func(string) bool { return true })
	if unknown != nil {
		return false, opererr.Wrap(unknown.cause,
			unknown.key+" could not be read from the store",
			"that key decides whether a change to this device's own configuration is refused, so whether it is frozen is unknown",
			fixCheckStoreKeys)
	}
	return frozen, nil
}

// fixCheckStoreKeys is the command that says why a store key cannot be read.
const fixCheckStoreKeys = "ze show data check"

// frozenUnknown is a key the freeze decides on that the store could not read,
// when what the store did answer leaves the decision open. refusalWhy is why
// that refuses a write.
type frozenUnknown struct {
	key        string
	refusalWhy string
	cause      error
}

// configFrozen is the rule of the freeze, for the configuration selects
// accepts as the one asked about: it answers whether that configuration is
// the device's own and the device is managed. read reads one store key: the
// held guard's under a staging, the store's own outside one.
//
// "The device's own stored configuration" is decided by the handle:
//
//   - a handle bound to a stored source (a daemon started from its store)
//     names its own configuration, and only that name is frozen
//   - a handle bound to a file or to stdin is never frozen: that input is the
//     authority, and such a daemon runs no hub client
//   - a handle with no binding (a tool that opened the store with the daemon
//     stopped) is scoped by the instance name, as the launcher is
//
// The name is compared before the flag is read, so a configuration that is
// not the device's own is admitted whatever the flag. A read failure answers
// unknown only when the keys that were read leave the decision open.
func configFrozen(store Storage, read func(key string) ([]byte, error), selects func(own string) bool) (bool, *frozenUnknown) {
	switch SourceMode(store) {
	case ConfigSourceFile, ConfigSourceStdin:
		return false, nil
	case ConfigSourceStored:
		if !selects(SourcePath(store)) {
			return false, nil
		}
		return managedFlagRead(read)
	case ConfigSourceUnspecified:
		return unboundConfigFrozen(store, read, selects)
	default:
		panic("BUG: unknown bound configuration source mode")
	}
}

// unboundConfigFrozen is configFrozen for a handle that holds no source
// binding. An explicit blob artifact is a backup, never a device: its
// configurations are not frozen whatever flag the backup carries. On a live
// store the instance name is read first, and a configuration it does not
// select is admitted without the flag being read. An instance name the store
// cannot read leaves the device's own configuration unknown, which matters
// only when the flag says the device is managed.
func unboundConfigFrozen(store Storage, read func(key string) ([]byte, error), selects func(own string) bool) (bool, *frozenUnknown) {
	if isBlobArtifact(store) {
		return false, nil
	}
	own, nameUnknown := ownConfigNameRead(read)
	if nameUnknown == nil && !selects(own) {
		return false, nil
	}
	managed, flagUnknown := managedFlagRead(read)
	if flagUnknown != nil {
		return false, flagUnknown
	}
	if !managed {
		return false, nil
	}
	if nameUnknown != nil {
		return false, nameUnknown
	}
	return true, nil
}

// isBlobArtifact reports whether store is an explicit blob artifact (OpenBlob,
// CreateBlob), the test PublishArtifactConfig applies. A handle this package
// did not open is not one.
func isBlobArtifact(store Storage) bool {
	owned, ok := ownedStore(store)
	if !ok {
		return false
	}
	if owned == nil {
		return false
	}
	return owned.blob != nil
}

// sameConfigName reports whether two configuration paths name the same stored
// configuration. The version pointers are keyed by the base name
// (pointerPath), so the base name is what the freeze compares.
func sameConfigName(configPath, other string) bool {
	return filepath.Base(configPath) == filepath.Base(other)
}

// managedFlagRead reads meta/instance/managed through read. An absent flag is
// "not managed": a device nobody provisioned as managed has none. A flag the
// store cannot read is unknown.
func managedFlagRead(read func(key string) ([]byte, error)) (bool, *frozenUnknown) {
	key := zefs.KeyInstanceManaged.Pattern
	stored, present, err := readAdmissionKey(read, key)
	if err != nil {
		return false, &frozenUnknown{key: key, cause: err,
			refusalWhy: "that key says whether this device is managed by a hub, and a managed device refuses a local change"}
	}
	if !present {
		return false, nil
	}
	return ManagedFlagSet(stored), nil
}

// ownConfigNameRead answers the name of the device's own configuration for a
// handle with no binding, from meta/instance/name through read, by the rule
// the launcher applies (InstanceConfigName). An absent name is the default
// configuration name. A name the store cannot read is unknown.
func ownConfigNameRead(read func(key string) ([]byte, error)) (string, *frozenUnknown) {
	key := zefs.KeyInstanceName.Pattern
	stored, _, err := readAdmissionKey(read, key)
	if err != nil {
		return "", &frozenUnknown{key: key, cause: err,
			refusalWhy: "this device is managed by a hub, and that key says which configuration is its own and so frozen"}
	}
	return InstanceConfigName(stored), nil
}
