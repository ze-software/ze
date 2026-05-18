// Design: plan/spec-diag-crash-capture.md -- show crashes CLI handler
// Related: show.go -- show verb RPC registration

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:crashes", Handler: handleShowCrashes},
	)
}

func handleShowCrashes(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) > 0 {
		return showCrashContent(args[0])
	}
	return showCrashList()
}

func showCrashList() (*plugin.Response, error) {
	summaries := crashlog.ListCrashes()
	if len(summaries) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"crashes": []any{},
				"count":   0,
				"dir":     crashlog.CrashDir(),
				"message": "no crashes recorded",
			},
		}, nil
	}

	entries := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		entries = append(entries, map[string]any{
			"name": s.Name,
			"size": s.Size,
		})
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"crashes": entries,
			"count":   len(entries),
			"dir":     crashlog.CrashDir(),
		},
	}, nil
}

func showCrashContent(name string) (*plugin.Response, error) {
	var content string
	if name == "latest" {
		content = crashlog.LatestCrash()
	} else {
		content = crashlog.ReadCrash(name)
	}

	if content == "" {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"message": "no crash report found",
			},
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"content": content,
		},
	}, nil
}
