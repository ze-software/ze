// Design: docs/architecture/core-design.md — plugin self-containment carve-out

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/plugins/crashes"
)

func handleShowCrashes(args []string) (*plugin.Response, error) {
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

// showCrashList answers the stored artifacts of both kinds, plus the readiness
// block. Readiness travels with the listing rather than under a second noun: an
// operator who sees no kernel report needs to know whether that means no fault
// happened or that nothing was ever going to be captured.
func showCrashList() (*plugin.Response, error) {
	entries := crashlog.CrashListFields()
	data := plugin.Map{
		"crashes":   entries,
		"count":     len(entries),
		"dir":       crashlog.CrashDir(),
		"readiness": crashes.Readiness("").Fields(),
	}
	if len(entries) == 0 {
		data["message"] = "no crashes recorded"
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
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
