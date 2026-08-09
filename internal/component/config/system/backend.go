// Design: docs/architecture/config/system-update.md -- active update backend registry

package system

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/identity"
)

// BackendName identifies the active system update backend in status output.
type BackendName string

const (
	BackendZeSelfUpdate BackendName = "ze-self-update"
	BackendGokrazyAB    BackendName = "gokrazy-ab"
)

var (
	ErrFirmwareUnsupported = errors.New("firmware update operation unsupported")
	ErrNotConfigured       = errors.New("update backend not configured")
)

// BackendOptions carries runtime-only backend settings from the hub.
type BackendOptions struct {
	GokrazySocketPath string
	IdentityStore     identity.Storage
}

// FirmwareResult is the structured result for manual firmware operations.
type FirmwareResult struct {
	Backend           BackendName
	Status            string
	Message           string
	DownloadedVersion string
	AppliedVersion    string
}

func (r FirmwareResult) Map() map[string]any {
	data := make(map[string]any)
	if r.Backend != "" {
		data["backend"] = string(r.Backend)
	}
	if r.Status != "" {
		data["status"] = r.Status
	}
	if r.Message != "" {
		data["message"] = r.Message
	}
	if r.DownloadedVersion != "" {
		data["downloaded-version"] = r.DownloadedVersion
	}
	if r.AppliedVersion != "" {
		data["applied-version"] = r.AppliedVersion
	}
	return data
}

func UnsupportedResult(name BackendName) FirmwareResult {
	return FirmwareResult{
		Backend: name,
		Status:  "unsupported",
		Message: "updates managed by gokrazy",
	}
}

// UpdateBackend is the single daemon-facing update backend surface.
type UpdateBackend interface {
	Name() BackendName
	Start(context.Context)
	Stop()
	Status() ExtendedUpdateStatus
	Check(context.Context) (ExtendedUpdateStatus, error)
	Download(context.Context) (FirmwareResult, error)
	Apply(context.Context) (FirmwareResult, error)
	Restart() (FirmwareResult, error)
	Rollback() (FirmwareResult, error)
	History() []UpdateEvent
}

type backendFactory func(UpdateCheckConfig, BackendOptions) (UpdateBackend, error)

var factories = map[BackendName]backendFactory{}

func registerBackend(name BackendName, fn backendFactory) {
	if name == "" {
		panic("BUG: system update backend: empty name")
	}
	if fn == nil {
		slog.Error("system update backend registration: nil factory", "backend", string(name))
		panic("BUG: system update backend: nil factory")
	}
	if _, exists := factories[name]; exists {
		slog.Error("system update backend registration: duplicate factory", "backend", string(name))
		panic("BUG: system update backend: duplicate factory")
	}
	factories[name] = fn
}

// NewBackend selects and constructs the update backend for a platform.
func NewBackend(platform host.PlatformType, cfg UpdateCheckConfig, opts BackendOptions) (UpdateBackend, error) {
	name := BackendZeSelfUpdate
	if platform == host.PlatformGokrazy {
		name = BackendGokrazyAB
	}
	factory, ok := factories[name]
	if !ok {
		return nil, errors.New("system update backend not registered: " + string(name))
	}
	return factory(cfg, opts)
}

var (
	activeBackendMu sync.RWMutex
	activeBackend   UpdateBackend
)

// ActiveBackend returns the backend registered by the daemon, or nil.
func ActiveBackend() UpdateBackend {
	activeBackendMu.RLock()
	defer activeBackendMu.RUnlock()
	return activeBackend
}

// SetActiveBackend registers the daemon's update backend for CLI queries.
func SetActiveBackend(backend UpdateBackend) {
	activeBackendMu.Lock()
	activeBackend = backend
	activeBackendMu.Unlock()
}

// ActiveExtendedUpdateStatus returns the extended status from the active backend.
func ActiveExtendedUpdateStatus() ExtendedUpdateStatus {
	backend := ActiveBackend()
	if backend == nil {
		return ExtendedUpdateStatus{Backend: BackendZeSelfUpdate}
	}
	status := backend.Status()
	if status.Backend == "" {
		status.Backend = backend.Name()
	}
	return status
}
