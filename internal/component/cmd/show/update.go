// Design: plan/spec-cpe-5-firmware-update.md — show system update CLI handler

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func handleShowSystemUpdate(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	st := system.ActiveUpdateStatus()

	data := map[string]any{
		"running-version":  st.RunningVersion,
		"update-available": st.UpdateAvailable,
	}

	if !st.LastCheck.IsZero() {
		data["last-check"] = st.LastCheck.Format("2006-01-02T15:04:05Z07:00")
	}

	if st.RemoteVersion != "" {
		data["remote-version"] = st.RemoteVersion
	}

	if st.LastError != "" {
		data["last-error"] = st.LastError
	}

	switch {
	case st.UpdateAvailable:
		data["status"] = "update available"
	case st.LastError != "":
		data["status"] = "check failed"
	case st.LastCheck.IsZero():
		data["status"] = "not configured"
	default:
		data["status"] = "up to date"
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
