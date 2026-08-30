// Design: docs/architecture/config/system-update.md -- minimal-build Ze self-update stub
//
//go:build !ze_distro

package system

import "context"

func init() {
	registerBackend(BackendZeSelfUpdate, newStrippedZeBackend)
}

const (
	strippedStatusText = "unsupported in minimal build"
	strippedMessage    = "self-update unavailable in minimal build"
)

type strippedZeBackend struct{}

func newStrippedZeBackend(cfg UpdateCheckConfig, _ BackendOptions) (UpdateBackend, error) {
	backend := &strippedZeBackend{}
	if cfg.URL == "" {
		return backend, nil
	}
	if err := ValidateUpdateCheckURL(cfg.URL); err != nil {
		return nil, err
	}
	if err := validateSelfUpdateConfig(cfg.SelfUpdate); err != nil {
		return nil, err
	}
	warnConfigConflicts(cfg.SelfUpdate)
	return backend, nil
}

func (b *strippedZeBackend) Name() BackendName { return BackendZeSelfUpdate }

func (b *strippedZeBackend) Start(context.Context) {}

func (b *strippedZeBackend) Stop() {}

func (b *strippedZeBackend) Status() ExtendedUpdateStatus {
	return ExtendedUpdateStatus{
		Backend:        BackendZeSelfUpdate,
		StatusText:     strippedStatusText,
		Message:        strippedMessage,
		DownloadStatus: statusUnsupported,
	}
}

func (b *strippedZeBackend) Check(context.Context) (ExtendedUpdateStatus, error) {
	return b.Status(), ErrFirmwareUnsupported
}

func (b *strippedZeBackend) Download(context.Context) (FirmwareResult, error) {
	return FirmwareResult{Backend: BackendZeSelfUpdate, Status: statusUnsupported, Message: strippedMessage}, ErrFirmwareUnsupported
}

func (b *strippedZeBackend) Apply(context.Context) (FirmwareResult, error) {
	return FirmwareResult{Backend: BackendZeSelfUpdate, Status: statusUnsupported, Message: strippedMessage}, ErrFirmwareUnsupported
}

func (b *strippedZeBackend) Restart() (FirmwareResult, error) {
	return FirmwareResult{Backend: BackendZeSelfUpdate, Status: statusUnsupported, Message: strippedMessage}, ErrFirmwareUnsupported
}

func (b *strippedZeBackend) Rollback() (FirmwareResult, error) {
	return FirmwareResult{Backend: BackendZeSelfUpdate, Status: statusUnsupported, Message: strippedMessage}, ErrFirmwareUnsupported
}

func (b *strippedZeBackend) History() []UpdateEvent { return nil }
