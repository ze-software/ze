// Design: plan/learned/727-diag-core.md -- runtime profiling via runtime/pprof
// Related: system.go -- existing system memory/cpu handlers

package show

import (
	"bytes"
	"encoding/base64"
	"errors"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	defaultCPUProfileDuration = 10 * time.Second
	maxCPUProfileDuration     = 60 * time.Second
	profileTypeCPU            = "cpu"
	profileTypeHeap           = "heap"
)

var cpuProfileMu sync.Mutex

var errCPUProfileInProgress = errors.New("CPU profile already in progress")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-profile", Handler: handleShowSystemProfile},
	)
}

func handleShowSystemProfile(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	profileType := profileTypeHeap
	duration := defaultCPUProfileDuration

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case profileTypeCPU, profileTypeHeap, "goroutine", "allocs":
			profileType = args[i]
		case "duration":
			if i+1 < len(args) {
				i++
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return &plugin.Response{Status: plugin.StatusError, Error: "profile: invalid duration: " + args[i]}, nil //nolint:nilerr // operational error in Response
				}
				if d < time.Second || d > maxCPUProfileDuration {
					return &plugin.Response{Status: plugin.StatusError, Error: "profile: duration must be between 1s and 60s"}, nil
				}
				duration = d
			}
		}
	}

	switch profileType {
	case profileTypeCPU:
		return profileCPU(duration)
	default:
		return profileSnapshot(profileType)
	}
}

func profileCPU(duration time.Duration) (*plugin.Response, error) {
	if !cpuProfileMu.TryLock() {
		return &plugin.Response{Status: plugin.StatusError, Error: errCPUProfileInProgress.Error()}, nil
	}
	defer cpuProfileMu.Unlock()

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	time.Sleep(duration)
	pprof.StopCPUProfile()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"type":     "cpu",
			"duration": duration.String(),
			"format":   "pprof-base64",
			"data":     base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}, nil
}

func profileSnapshot(profileType string) (*plugin.Response, error) {
	p := pprof.Lookup(profileType)
	if p == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "unknown profile type: " + profileType,
		}, nil
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, 0); err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"type":   profileType,
			"count":  p.Count(),
			"format": "pprof-base64",
			"data":   base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}, nil
}
