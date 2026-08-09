// Design: docs/architecture/appliance/self-update.md -- show system update CLI handler (extended)
// Relocated from internal/component/cmd/show/update.go (plugin self-containment).

package cmd

import (
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func handleShowSystemUpdate(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	ext := system.ActiveExtendedUpdateStatus()
	st := ext.UpdateStatus

	data := map[string]any{
		"backend":          string(ext.Backend),
		"running-version":  st.RunningVersion,
		"update-available": st.UpdateAvailable,
	}
	if ext.Message != "" {
		data["message"] = ext.Message
	}
	if ext.Backend == system.BackendGokrazyAB {
		data["gokrazy-reachable"] = ext.GokrazyReachable
		if len(ext.GokrazyFeatures) > 0 {
			data["gokrazy-features"] = ext.GokrazyFeatures
		}
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

	if ext.DownloadStatus != "" {
		data["download-status"] = ext.DownloadStatus
	}
	if ext.DownloadSHA256 != "" {
		data["download-sha256"] = ext.DownloadSHA256
	}
	if ext.StagedVersion != "" {
		data["staged-version"] = ext.StagedVersion
	}
	if ext.StagedPath != "" {
		data["staged-path"] = ext.StagedPath
	}
	if ext.RestartPolicy != "" {
		data["restart"] = ext.RestartPolicy
	}
	data["server-paused"] = ext.ServerPaused

	switch {
	case ext.StatusText != "":
		data["status"] = ext.StatusText
	case ext.DownloadStatus == "staged":
		data["status"] = "staged"
	case ext.DownloadStatus == "downloading":
		data["status"] = "downloading"
	case ext.DownloadStatus == "verifying":
		data["status"] = "verifying"
	case ext.DownloadStatus == "paused by server":
		data["status"] = "paused by server"
	case ext.DownloadStatus == "waiting for maintenance window":
		data["status"] = "waiting for maintenance window"
	case ext.DownloadStatus == "waiting for spread":
		data["status"] = "waiting for spread"
	case ext.DownloadStatus != "" && len(ext.DownloadStatus) > 6 && ext.DownloadStatus[:6] == "error:":
		data["status"] = ext.DownloadStatus
	case st.UpdateAvailable:
		data["status"] = "update available"
	case st.LastError != "":
		data["status"] = "check failed"
	case st.LastCheck.IsZero():
		data["status"] = "not configured"
	default:
		data["status"] = "up to date"
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

func handleShowSystemUpdateHistory(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	backend := system.ActiveBackend()
	if backend == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"history": []any{}, "count": 0},
		}, nil
	}

	events := backend.History()
	rows := make([]map[string]any, 0, len(events))
	for i := range events {
		rows = append(rows, map[string]any{
			"timestamp": events[i].Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			"from":      events[i].FromVersion,
			"to":        events[i].ToVersion,
			"result":    events[i].Result,
		})
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"history": rows, "count": len(rows)},
	}, nil
}
