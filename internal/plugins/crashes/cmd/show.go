// Design: docs/architecture/core-design.md — plugin self-containment carve-out

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/crashlog"
)

func HandleShowCrashes(args []string) (*plugin.Response, error) {
	switch len(args) {
	case 0:
		return showCrashList()
	case 1:
		if args[0] == "latest" {
			return showCrashContent("latest")
		}
	case 2:
		if args[0] == "name" {
			return showCrashContent(args[1])
		}
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "usage: show crashes [latest | name <filename>]",
	}, nil
}

func showCrashList() (*plugin.Response, error) {
	summaries := crashlog.ListCrashes()
	if len(summaries) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
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
		Data: plugin.Map{
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
			Data: plugin.Map{
				"message": "no crash report found",
			},
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"content": content,
		},
	}, nil
}
