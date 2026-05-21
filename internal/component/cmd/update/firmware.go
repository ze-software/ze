// Design: plan/spec-cpe-6-self-update.md -- update system firmware CLI handlers

package update

import (
	"context"

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

func activeSelfUpdater() (*system.SelfUpdater, *plugin.Response) {
	su := system.ActiveSelfUpdaterInstance()
	if su == nil {
		return nil, &plugin.Response{Status: plugin.StatusError, Data: "update checker not configured"}
	}
	return su, nil
}

func requestContext(ctx *pluginserver.CommandContext) context.Context {
	if ctx != nil && ctx.RequestContext != nil {
		return ctx.RequestContext
	}
	return context.Background()
}

func handleFirmwareCheck(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	su, errResp := activeSelfUpdater()
	if errResp != nil {
		return errResp, nil
	}

	su.ManualCheck(requestContext(ctx))

	st := su.ExtendedStatus()
	data := map[string]any{
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
	su, errResp := activeSelfUpdater()
	if errResp != nil {
		return errResp, nil
	}

	ver, err := su.ManualDownload(requestContext(ctx))
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error reported in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"downloaded-version": ver, "status": "complete"},
	}, nil
}

func handleFirmwareApply(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	su, errResp := activeSelfUpdater()
	if errResp != nil {
		return errResp, nil
	}

	ver, err := su.ManualApply(requestContext(ctx))
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error reported in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"applied-version": ver, "status": "restarting"},
	}, nil
}

func handleFirmwareRestart(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	su, errResp := activeSelfUpdater()
	if errResp != nil {
		return errResp, nil
	}

	if err := su.ManualRestart(); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error reported in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"status": "restarting"},
	}, nil
}

func handleFirmwareRollback(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	su, errResp := activeSelfUpdater()
	if errResp != nil {
		return errResp, nil
	}

	if err := su.Rollback(); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error reported in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"status": "rolling back"},
	}, nil
}
