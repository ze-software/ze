// Design: docs/architecture/appliance/self-update.md -- self-update configuration and status

package system

import "time"

// SelfUpdateConfig holds the self-update config extracted from config tree.
type SelfUpdateConfig struct {
	AutoApply        bool
	Spread           uint32
	MaintenanceStart string // HH:MM
	MaintenanceEnd   string // HH:MM
	RestartImmediate bool
	RestartTime      string // HH:MM
}

// UpdateEvent records one update attempt for history.
type UpdateEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	FromVersion string    `json:"from-version"`
	ToVersion   string    `json:"to-version"`
	Result      string    `json:"result"`
}

// ExtendedUpdateStatus holds the full update status including self-update state.
type ExtendedUpdateStatus struct {
	UpdateStatus
	Backend          BackendName
	StatusText       string
	Message          string
	GokrazyReachable bool
	GokrazyFeatures  []string
	DownloadStatus   string
	DownloadSHA256   string
	StagedVersion    string
	StagedPath       string
	RestartPolicy    string
	ServerPaused     bool
	HeldVersion      string
}
