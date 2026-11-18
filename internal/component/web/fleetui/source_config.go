// Design: docs/architecture/fleet-interface.md -- the configuration, rollout, audit and administration questions
// Overview: source.go -- the data source seam

package fleetui

import (
	"context"
	"time"
)

// The bounds of the configuration, rollout, audit and administration answers.
const (
	// ConfigTextBytesMax bounds one configuration text.
	ConfigTextBytesMax = 1 << 20
	// ConfigVersionsMax bounds the versions the hub keeps for one device.
	ConfigVersionsMax = 100
	// RolloutsMax bounds the rollouts the hub keeps.
	RolloutsMax = 128
	// RolloutBatchesMax bounds the batches of one rollout.
	RolloutBatchesMax = 100
	// AuditEntriesMax bounds the audit entries one answer holds.
	AuditEntriesMax = 1000
	// TemplatesMax bounds the configuration templates of one hub.
	TemplatesMax = 256
	// AccessProfilesMax bounds the authorization profiles of one hub.
	AccessProfilesMax = 64
)

// ConfigSource answers about the configuration the hub holds for its devices,
// what it rolls out, what it recorded, and how it is administered.
type ConfigSource interface {
	// DeviceConfig answers the configuration the hub holds for one device,
	// at most ConfigTextBytesMax of text.
	// Asked by: the device Config tab. Answered by: fleet-1 and fleet-2.
	DeviceConfig(ctx context.Context, device string) (DeviceConfig, error)

	// ConfigVersions answers the versions the hub has held for one device,
	// at most ConfigVersionsMax, newest first.
	// Asked by: the history section of the device Config tab.
	// Answered by: fleet-3.
	ConfigVersions(ctx context.Context, device string) ([]ConfigVersion, error)

	// ConfigText answers the text of one version of one device's
	// configuration. An unknown version wraps ErrObjectUnknown.
	// Asked by: the diff in the side panel of a history row.
	// Answered by: fleet-3.
	ConfigText(ctx context.Context, device, version string) (string, error)

	// Divergence answers both sides of a diverged device's configuration. A
	// device that is not diverged wraps ErrObjectUnknown: there is no
	// divergence to show.
	// Asked by: the divergence banner and diff of the device Dashboard and
	// Config tabs. Answered by: fleet-7.
	Divergence(ctx context.Context, device string) (Divergence, error)

	// Rollouts answers every rollout the hub keeps, at most RolloutsMax,
	// newest first.
	// Asked by: the estate Rollouts tab, the Rollouts and Needs attention
	// cards. Answered by: fleet-5.
	Rollouts(ctx context.Context) ([]Rollout, error)

	// Rollout answers one rollout with its batches and devices. An unknown
	// name wraps ErrObjectUnknown.
	// Asked by: the rollout page. Answered by: fleet-5.
	Rollout(ctx context.Context, name string) (RolloutDetail, error)

	// AuditEntries answers the audit trail, at most AuditEntriesMax, newest
	// first.
	// Asked by: the audit administration page, the Recent changes card, the
	// audit section of the device Tasks tab. Answered by: fleet-3.
	AuditEntries(ctx context.Context) ([]AuditEntry, error)

	// Templates answers the configuration templates, at most TemplatesMax.
	// Asked by: the templates administration page. Answered by: fleet-2.
	Templates(ctx context.Context) ([]Template, error)

	// AccessProfiles answers what each authorization profile may do in the
	// fleet interface, at most AccessProfilesMax.
	// Asked by: the access administration page. Answered by: fleet-1 (hub
	// configuration).
	AccessProfiles(ctx context.Context) ([]AccessProfile, error)

	// Settings answers the fleet settings of the hub.
	// Asked by: the settings administration page. Answered by: fleet-ui-5
	// (hub configuration).
	Settings(ctx context.Context) (Settings, error)
}

// DeviceConfig is the configuration the hub holds for one device.
type DeviceConfig struct {
	Device string
	// Version is the version of Text.
	Version string
	Text    string
}

// ConfigVersion is one version of a device's configuration the hub has held.
type ConfigVersion struct {
	Version string
	At      time.Time
	Actor   string
	Summary string
	// Result is what the device answered when it was sent this version.
	Result string
}

// Divergence is both sides of a configuration a device edited locally.
type Divergence struct {
	Device        string
	HubVersion    string
	HubText       string
	DeviceVersion string
	DeviceText    string
	// Changes counts the local edits.
	Changes int
	// EditedAt is when the last local edit was made.
	EditedAt time.Time
}

// RolloutState is where a rollout stands. The zero value is not a state.
type RolloutState uint8

const (
	// RolloutStateUnspecified is the invalid zero value.
	RolloutStateUnspecified RolloutState = iota
	// RolloutStatePending means the rollout has not started its first batch.
	RolloutStatePending
	// RolloutStateRolling means a batch is in progress.
	RolloutStateRolling
	// RolloutStatePaused means the rollout stopped between batches and waits
	// for an operator.
	RolloutStatePaused
	// RolloutStateComplete means every batch finished.
	RolloutStateComplete
	// RolloutStateFailed means the rollout stopped on a failure.
	RolloutStateFailed
	// RolloutStateAborted means an operator stopped the rollout.
	RolloutStateAborted
)

var (
	rolloutStateWords = []string{enumWordUnspecified, enumWordPending, "Rolling", "Paused", "Complete", "Failed", "Aborted"}
	rolloutStateKeys  = []string{enumKeyUnspecified, enumKeyPending, "rolling", "paused", "complete", enumKeyFailed, "aborted"}
)

// String answers the word a page shows for the state.
func (state RolloutState) String() string {
	return enumText(rolloutStateWords, state)
}

// Key answers the state's name in a filter.
func (state RolloutState) Key() string {
	return enumText(rolloutStateKeys, state)
}

// ParseRolloutState answers the state a filter names, or false.
func ParseRolloutState(key string) (RolloutState, bool) {
	return enumParse[RolloutState](rolloutStateKeys, key)
}

// Rollout is one staged push of a configuration to many devices, without its
// batches and devices.
type Rollout struct {
	Name string
	// Target says what the devices were chosen from: a filter or a group.
	Target       string
	State        RolloutState
	Actor        string
	Started      time.Time
	BatchesTotal int
	BatchesDone  int
	DevicesTotal int
	DevicesDone  int
	// PauseReason says why the rollout paused, in the hub's own words. It
	// means nothing unless State is RolloutStatePaused.
	PauseReason string
}

// RolloutDetail is one rollout with its batches and devices.
type RolloutDetail struct {
	Rollout Rollout
	// Batches holds every batch in order, at most RolloutBatchesMax.
	Batches []RolloutBatch
	// Devices holds every targeted device, at most DevicesMax.
	Devices []RolloutDevice
	// PauseDevices names the devices that caused the pause.
	PauseDevices []string
}

// RolloutBatch is one batch of a rollout.
type RolloutBatch struct {
	// Number is the batch's position, from one.
	Number int
	State  RolloutState
}

// RolloutDevice is what one device of a rollout answered.
type RolloutDevice struct {
	Device string
	// Batch is the number of the batch the device is in.
	Batch int
	// Result is the result word, and Answer the device's answer.
	Result string
	Answer string
	// Answered reports whether the device has been sent the configuration.
	Answered bool
	// At is when it answered. It means nothing while Answered is false.
	At time.Time
}

// AuditEntry is one permanent record of who did what.
type AuditEntry struct {
	ID    string
	At    time.Time
	Actor string
	Event string
	// Target is the device or the set of devices the event was about.
	Target string
	// Devices counts the devices the event touched.
	Devices int
	Result  string
	Summary string
	// TaskID is the task the entry belongs to. It is empty for an entry no
	// task wrote.
	TaskID string
}

// Template is one configuration template.
type Template struct {
	Name string
	// Groups names the configuration groups assigned to the template.
	Groups []string
	// Devices counts the devices whose configuration comes from it.
	Devices int
	Changed time.Time
	Actor   string
}

// AccessProfile is what one authorization profile may do in the fleet
// interface.
type AccessProfile struct {
	Name        string
	FleetMode   bool
	Console     bool
	Diagnostics bool
	ConfigPush  bool
	Rollouts    bool
}

// Settings is the fleet settings of a hub.
type Settings struct {
	// OfflineAlarmDelay is how long a device stays without a connection
	// before its offline alarm opens.
	OfflineAlarmDelay time.Duration
	// ResourceAlarmPercent is the use above which a resource alarm opens,
	// and ResourceAlarmDelay how long it must stay above.
	ResourceAlarmPercent float64
	ResourceAlarmDelay   time.Duration
	// FinishedTasksKept is how many finished tasks the hub keeps.
	FinishedTasksKept int
	// EventStreamsMax is the maximum of live event streams.
	EventStreamsMax int
}
