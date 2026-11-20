// Design: docs/architecture/fleet-config.md -- what a managed client's store says about its hub: the flag, the active hash and the baseline
// Related: cmd_fleet.go -- show fleet status, which reports all three
// Related: doctor.go -- checkHubClientDeclared and checkManagedDiverged, which read the flag through ManagedFlag
// Related: client.go -- fetchAndProcess, which holds on what BaselineAccess reads

// One reader each for the three store facts the fleet code compares: the
// managed flag, the hash of the configuration the device runs, and the
// baseline (the hash of the last configuration its hub gave it). Every caller
// goes through these, so a command, a doctor check and the client cannot
// disagree on which bytes are "the active configuration" or on what an absent
// key means. The baseline has one writer too (StoreBaseline), and Diverged is
// the one rule that compares the two hashes.

package managed

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// managedFlagTrue is the value this package writes to meta/instance/managed
// for a managed device. storage.ManagedFlagSet is what reads it back.
const managedFlagTrue = "true"

// ConfigVersion is the version hash of a configuration (fleet.VersionHash), or
// the fact that the store holds none. The zero value is "not recorded", so an
// absent baseline or an absent configuration is never read as a hash. Two
// values compare with ==.
type ConfigVersion struct {
	hash string
}

// Recorded reports whether the store held the configuration or the baseline
// this version was read from.
func (v ConfigVersion) Recorded() bool {
	return v.hash != ""
}

// Hash answers the version hash, empty when the version is not recorded. A
// caller MUST ask Recorded before it compares or prints the hash.
func (v ConfigVersion) Hash() string {
	return v.hash
}

// ManagedFlag reads meta/instance/managed: true when the device follows a
// hub. An absent key answers false, because a device nobody provisioned as
// managed has no flag. Every other read failure is an error: the caller
// decides what a flag it cannot read means for its own action, and never
// reads it as "not managed". What a stored value means is the store's rule
// (storage.ManagedFlagSet), which its freeze applies to the same key.
func ManagedFlag(store storage.Storage) (bool, error) {
	flag, err := store.ReadFile(zefs.KeyInstanceManaged.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return storage.ManagedFlagSet(flag), nil
}

// ActiveVersion answers the version hash of the configuration the device
// runs: the active version of the stored configuration the instance name
// selects (resolve.DefaultConfig). A store that holds
// no such configuration answers a version that is not recorded.
func ActiveVersion(store storage.Storage) (ConfigVersion, error) {
	content, stored, err := activeConfig(store)
	if err != nil {
		return ConfigVersion{}, err
	}
	if !stored {
		return ConfigVersion{}, nil
	}
	return versionOf(content), nil
}

// BaselineVersion answers the baseline: the version hash under
// meta/managed/base-version. An absent key, or one holding only white space,
// answers a version that is not recorded, which is a valid state.
func BaselineVersion(store storage.Storage) (ConfigVersion, error) {
	recorded, err := store.ReadFile(zefs.KeyManagedBaseVersion.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return ConfigVersion{}, nil
	}
	if err != nil {
		return ConfigVersion{}, err
	}
	return ConfigVersion{hash: strings.TrimSpace(string(recorded))}, nil
}

// errNoActiveConfig is what RecordActiveAsBaseline answers for a store that
// holds no configuration under the device's own name: there is nothing a hub
// could have made active.
var errNoActiveConfig = errors.New("the store holds no configuration for this device")

// Diverged reports whether the configuration the device runs differs from the
// last one its hub gave it. A device with no baseline is not diverged: it was
// provisioned before baselines existed, or has not yet taken a configuration
// from its hub. A device with no stored configuration is not diverged either:
// it holds nothing a hub configuration could overwrite.
func Diverged(active, baseline ConfigVersion) bool {
	if !baseline.Recorded() {
		return false
	}
	if !active.Recorded() {
		return false
	}
	return active != baseline
}

// StoreBaseline records version as the baseline under
// meta/managed/base-version. It is the one writer of that key. A caller MUST
// pass a version the hub made active or answered "current" for, never the hash
// of bytes the hub sent: RecordActiveAsBaseline reads it back for a
// configuration that became active.
func StoreBaseline(store storage.Storage, version ConfigVersion) error {
	if !version.Recorded() {
		return errors.New("a version that is not recorded cannot be the baseline")
	}
	key := zefs.KeyManagedBaseVersion.Pattern
	if err := store.WriteKey(key, []byte(version.hash+"\n")); err != nil {
		return fmt.Errorf("write %s: %w", key, err)
	}
	return nil
}

// RecordActiveAsBaseline reads the configuration the device runs back from
// the store and records its hash as the baseline. A caller MUST call it only
// once a configuration from the hub has become active: after the reload of a
// hub push returned success, or after the first start cached the fetched
// configuration. It answers the version it recorded.
func RecordActiveAsBaseline(store storage.Storage) (ConfigVersion, error) {
	active, err := ActiveVersion(store)
	if err != nil {
		return ConfigVersion{}, fmt.Errorf("read the active configuration: %w", err)
	}
	if !active.Recorded() {
		return ConfigVersion{}, errNoActiveConfig
	}
	if err := StoreBaseline(store, active); err != nil {
		return ConfigVersion{}, err
	}
	return active, nil
}

// BaselineAccess is what a managed client reads and writes in the device's
// store to decide whether it may apply what its hub sends, and to send its hub
// a copy of the configuration it holds. The client holds these four
// operations and never the store, as it holds CheckManaged and CommitOver. The
// zero value is not usable: build one with NewBaselineAccess.
type BaselineAccess struct {
	active   func() (ConfigVersion, error)
	baseline func() (ConfigVersion, error)
	store    func(ConfigVersion) error
	held     func() (heldConfig, error)
}

// NewBaselineAccess binds the four operations to store through the readers
// and the writer of this file. store is borrowed: the caller MUST keep it open
// until the client that holds the result has returned (RunManagedClient).
func NewBaselineAccess(store storage.Storage) *BaselineAccess {
	return &BaselineAccess{
		active:   func() (ConfigVersion, error) { return ActiveVersion(store) },
		baseline: func() (ConfigVersion, error) { return BaselineVersion(store) },
		store:    func(version ConfigVersion) error { return StoreBaseline(store, version) },
		held:     func() (heldConfig, error) { return readHeldConfig(store) },
	}
}

// read answers the two versions the client compares before a fetch. An error
// means the comparison has no answer, which the caller MUST NOT read as "not
// diverged".
func (access *BaselineAccess) read() (active, baseline ConfigVersion, err error) {
	active, err = access.active()
	if err != nil {
		return ConfigVersion{}, ConfigVersion{}, fmt.Errorf("read the active configuration: %w", err)
	}
	baseline, err = access.baseline()
	if err != nil {
		return ConfigVersion{}, ConfigVersion{}, fmt.Errorf("read the baseline (%s): %w", zefs.KeyManagedBaseVersion.Pattern, err)
	}
	return active, baseline, nil
}

// activeConfig reads the bytes ActiveVersion hashes: the configuration the
// daemon runs, which is the version the active pointer names and the plain
// file only while nothing is pointed (storage.ReadActiveConfig). The plain
// file alone is not it: a write that changes that file and not what boots
// would make the device hold over bytes it never ran. stored is false when the
// store holds no configuration under the device's own name.
func activeConfig(store storage.Storage) (content []byte, stored bool, err error) {
	content, _, stored, err = activeConfigStamped(store)
	return content, stored, err
}

// activeConfigStamped is activeConfig that also answers when the version it
// read was written into the store, from the same pointer read
// (storage.ReadActiveConfigStamped). stamp is the zero time when no version
// stamp describes the bytes: the plain file, or the bytes of a pending boot.
func activeConfigStamped(store storage.Storage) (content []byte, stamp time.Time, stored bool, err error) {
	configName, err := resolve.DefaultConfig(store)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	content, stamp, _, err = storage.ReadActiveConfigStamped(store, configName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, err
	}
	return content, stamp, true, nil
}

// heldConfig is the configuration a holding device runs, from one read: its
// bytes, their version, and when that version was written into the store. The
// version is the hash of exactly these bytes, so the copy a device sends its
// hub is the configuration whose hash it reported.
type heldConfig struct {
	content []byte
	version ConfigVersion
	// editedAt is the zero time when the store records no stamp for the
	// bytes (activeConfigStamped). fleet.ConfigHeld omits a zero EditedAt,
	// which is how the hub learns that the device knows no such time.
	editedAt time.Time
}

// readHeldConfig reads the configuration the device runs for an upload to its
// hub. A store that holds no configuration under the device's own name is
// errNoActiveConfig: a device with none never holds (Diverged), so reaching
// here without one means the store changed under the hold.
func readHeldConfig(store storage.Storage) (heldConfig, error) {
	content, stamp, stored, err := activeConfigStamped(store)
	if err != nil {
		return heldConfig{}, err
	}
	if !stored {
		return heldConfig{}, errNoActiveConfig
	}
	return heldConfig{content: content, version: versionOf(content), editedAt: stamp}, nil
}

// versionOf is the version of configuration bytes a store held.
func versionOf(content []byte) ConfigVersion {
	return ConfigVersion{hash: fleet.VersionHash(content)}
}
