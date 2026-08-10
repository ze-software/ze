//go:build ze_distro

// Design: docs/architecture/config/system-update.md -- Ze self-update backend wrapper

package system

import "context"

func init() {
	registerBackend(BackendZeSelfUpdate, newZeBackend)
}

type zeBackend struct {
	checker *UpdateChecker
	updater *SelfUpdater
	started bool
}

func newZeBackend(cfg UpdateCheckConfig, opts BackendOptions) (UpdateBackend, error) {
	backend := &zeBackend{}
	if cfg.URL == "" {
		return backend, nil
	}
	if err := ValidateUpdateCheckURL(cfg.URL); err != nil {
		return nil, err
	}
	if cfg.Interval == 0 {
		cfg.Interval = 86400
	}
	if err := validateSelfUpdateConfig(cfg.SelfUpdate); err != nil {
		return nil, err
	}
	warnConfigConflicts(cfg.SelfUpdate)

	if cfg.SelfUpdate.AutoApply || cfg.SelfUpdate.RestartImmediate || cfg.SelfUpdate.RestartTime != "" {
		backend.updater = newSelfUpdater(cfg.URL, cfg.Interval, cfg.SelfUpdate, opts.IdentityStore)
		return backend, nil
	}
	backend.checker = newUpdateChecker(cfg.URL, cfg.Interval)
	return backend, nil
}

func (b *zeBackend) Name() BackendName { return BackendZeSelfUpdate }

func (b *zeBackend) Start(ctx context.Context) {
	if b.updater != nil {
		b.updater.Start(ctx)
		b.started = true
		return
	}
	if b.checker != nil {
		b.checker.Start(ctx)
		b.started = true
	}
}

func (b *zeBackend) Stop() {
	if !b.started {
		return
	}
	if b.updater != nil {
		b.updater.Stop()
		return
	}
	if b.checker != nil {
		b.checker.Stop()
	}
}

func (b *zeBackend) Status() ExtendedUpdateStatus {
	if b.updater != nil {
		status := b.updater.extendedStatus()
		status.Backend = BackendZeSelfUpdate
		return status
	}
	if b.checker != nil {
		return ExtendedUpdateStatus{
			Backend:      BackendZeSelfUpdate,
			UpdateStatus: b.checker.Status(),
		}
	}
	return ExtendedUpdateStatus{Backend: BackendZeSelfUpdate}
}

func (b *zeBackend) Check(ctx context.Context) (ExtendedUpdateStatus, error) {
	if b.updater == nil {
		return b.Status(), ErrNotConfigured
	}
	b.updater.manualCheck(ctx)
	return b.Status(), nil
}

func (b *zeBackend) Download(ctx context.Context) (FirmwareResult, error) {
	if b.updater == nil {
		return FirmwareResult{}, ErrNotConfigured
	}
	ver, err := b.updater.manualDownload(ctx)
	if err != nil {
		return FirmwareResult{}, err
	}
	return FirmwareResult{DownloadedVersion: ver, Status: statusComplete}, nil
}

func (b *zeBackend) Apply(ctx context.Context) (FirmwareResult, error) {
	if b.updater == nil {
		return FirmwareResult{}, ErrNotConfigured
	}
	ver, err := b.updater.manualApply(ctx)
	if err != nil {
		return FirmwareResult{}, err
	}
	return FirmwareResult{AppliedVersion: ver, Status: "restarting"}, nil
}

func (b *zeBackend) Restart() (FirmwareResult, error) {
	if b.updater == nil {
		return FirmwareResult{}, ErrNotConfigured
	}
	if err := b.updater.manualRestart(); err != nil {
		return FirmwareResult{}, err
	}
	return FirmwareResult{Status: "restarting"}, nil
}

func (b *zeBackend) Rollback() (FirmwareResult, error) {
	if b.updater == nil {
		return FirmwareResult{}, ErrNotConfigured
	}
	if err := b.updater.Rollback(); err != nil {
		return FirmwareResult{}, err
	}
	return FirmwareResult{Status: "rolling back"}, nil
}

func (b *zeBackend) History() []UpdateEvent {
	if b.updater == nil {
		return nil
	}
	return b.updater.History()
}
