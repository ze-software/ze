// Design: plan/learned/748-cpe-6-self-update.md -- update system firmware CLI handlers
// Relocated from internal/component/cmd/update/firmware.go (plugin self-containment).

package cmd

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func activeBackend() (system.UpdateBackend, *plugin.Response) {
	backend := system.ActiveBackend()
	if backend == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Error: "update checker not configured"}
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
		if errors.Is(err, system.ErrFirmwareUnsupported) {
			return unsupportedCheckResponse(backend, ext), nil
		}
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

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
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
		Data:   plugin.Map(res.Map()),
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
		Data:   plugin.Map(res.Map()),
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
		Data:   plugin.Map(res.Map()),
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
		Data:   plugin.Map(res.Map()),
	}, nil
}

func unsupportedCheckResponse(backend system.UpdateBackend, status system.ExtendedUpdateStatus) *plugin.Response {
	msg := status.LastError
	if msg == "" {
		msg = status.Message
	}
	if msg == "" {
		msg = status.StatusText
	}
	if msg == "" {
		msg = status.DownloadStatus
	}
	if msg == "" {
		return errorResponse(backend, system.FirmwareResult{}, system.ErrFirmwareUnsupported)
	}
	return &plugin.Response{Status: plugin.StatusError, Error: "unsupported: " + msg}
}

func errorResponse(backend system.UpdateBackend, res system.FirmwareResult, err error) *plugin.Response {
	if errors.Is(err, system.ErrFirmwareUnsupported) {
		if res.Status == "" {
			res = system.UnsupportedResult(backend.Name())
		}
		return &plugin.Response{Status: plugin.StatusError, Error: res.Status + ": " + res.Message}
	}
	return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
}
