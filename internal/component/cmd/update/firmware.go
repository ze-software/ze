// Design: plan/spec-cpe-6-self-update.md -- update system firmware CLI handlers

package update

import (
	"context"
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-check",
			Handler:    handleFirmwareCheck,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-download",
			Handler:    handleFirmwareDownload,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-apply",
			Handler:    handleFirmwareApply,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-restart",
			Handler:    handleFirmwareRestart,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-update:system-firmware-rollback",
			Handler:    handleFirmwareRollback,
		},
	)
}

func activeBackend() (system.UpdateBackend, *plugin.Response) {
	backend := system.ActiveBackend()
	if backend == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Data: "update checker not configured"}
	}
	return backend, nil
}

func reqCtx(ctx *pluginserver.CommandContext) context.Context {
	if ctx != nil && ctx.RequestContext != nil {
		return ctx.RequestContext
	}
	return context.Background()
}

func handleFirmwareCheck(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend, errResp := activeBackend()
	if errResp != nil {
		return errResp, nil
	}

	ext, err := backend.Check(reqCtx(ctx))
	if err != nil {
		return errorResponse(backend, system.FirmwareResult{}, err), nil
	}
	st := ext.UpdateStatus
	data := map[string]any{
		"backend":          string(backend.Name()),
		"running-version":  st.RunningVersion,
		"update-available": st.UpdateAvailable,
	}
	if st.RemoteVersion != "" {
		data["remote-version"] = st.RemoteVersion
	}
	if !st.LastCheck.IsZero() {
		data["last-check"] = st.LastCheck.Format("2006-01-02T15:04:05Z07:00")
	}
	if st.LastError != "" {
		data["last-error"] = st.LastError
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}

func handleFirmwareDownload(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend, errResp := activeBackend()
	if errResp != nil {
		return errResp, nil
	}

	res, err := backend.Download(reqCtx(ctx))
	if err != nil {
		return errorResponse(backend, res, err), nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   res.Map(),
	}, nil
}

func handleFirmwareApply(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend, errResp := activeBackend()
	if errResp != nil {
		return errResp, nil
	}

	res, err := backend.Apply(reqCtx(ctx))
	if err != nil {
		return errorResponse(backend, res, err), nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   res.Map(),
	}, nil
}

func handleFirmwareRestart(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend, errResp := activeBackend()
	if errResp != nil {
		return errResp, nil
	}

	res, err := backend.Restart()
	if err != nil {
		return errorResponse(backend, res, err), nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   res.Map(),
	}, nil
}

func handleFirmwareRollback(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend, errResp := activeBackend()
	if errResp != nil {
		return errResp, nil
	}

	res, err := backend.Rollback()
	if err != nil {
		return errorResponse(backend, res, err), nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   res.Map(),
	}, nil
}

func errorResponse(backend system.UpdateBackend, res system.FirmwareResult, err error) *plugin.Response {
	if errors.Is(err, system.ErrFirmwareUnsupported) {
		if res.Status == "" {
			res = system.UnsupportedResult(backend.Name())
		}
		return &plugin.Response{Status: plugin.StatusError, Data: res.Map()}
	}
	return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}
}
