// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- runtime profiling via runtime/pprof
// Related: system.go -- existing system memory/cpu handlers
// Related: profile_summary.go -- pure-stdlib pprof protobuf summary decoder

package show

import (
	"bytes"
	"encoding/base64"
	"errors"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	defaultCPUProfileDuration = 10 * time.Second
	maxCPUProfileDuration     = 60 * time.Second
	profileTimeoutHeadroom    = 15 * time.Second
	profileTypeCPU            = "cpu"
	profileTypeHeap           = "heap"
	profileTypeGoroutine      = "goroutine"
	profileTypeAllocs         = "allocs"
	profileTypeMutex          = "mutex"
	profileTypeBlock          = "block"
	pipeOpSummary             = "summary"
	profileFormatPprofBase64  = "pprof-base64"
	keyFormat                 = "format"
	keyData                   = "data"
	keyDuration               = "duration"
	keySamplingRate           = "sampling-rate"
)

var cpuProfileMu sync.Mutex

var errCPUProfileInProgress = errors.New("CPU profile already in progress")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod:     "ze-cmd:show-system-profile",
			Handler:        handleShowSystemProfile,
			HandlerTimeout: profileHandlerTimeout,
		},
	)
	command.RegisterPipeFilters(
		[]string{"show system profile"},
		command.PipeFilter{
			Name:        pipeOpSummary,
			Description: "Summarize top functions from the captured pprof profile",
			Argument:    command.ArgNone,
		},
		pluginPipeFilter,
	)
}

func handleShowSystemProfile(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	profileType := profileTypeHeap
	duration := defaultCPUProfileDuration

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case keyType:
			if i+1 < len(args) {
				i++
				if isSupportedProfileType(args[i]) {
					profileType = args[i]
				}
			}
		case profileTypeCPU, profileTypeHeap, profileTypeGoroutine, profileTypeAllocs, profileTypeMutex, profileTypeBlock:
			profileType = args[i]
		case keyDuration:
			if i+1 < len(args) {
				i++
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return &plugin.Response{Status: plugin.StatusError, Error: "profile: invalid duration: " + args[i]}, nil //nolint:nilerr // operational error in Response
				}
				if d < time.Second {
					return plugin.OperatorErrorResponse(opererr.New(
						"profile: duration must be between 1s and 60s",
						"CPU profiling requires a sampling duration from 1s to 60s",
						"ze show system profile type cpu duration 10s",
					)), nil
				}
				if d > maxCPUProfileDuration {
					return plugin.OperatorErrorResponse(opererr.New(
						"profile: duration must be between 1s and 60s",
						"CPU profiling requires a sampling duration from 1s to 60s",
						"ze show system profile type cpu duration 10s",
					)), nil
				}
				duration = d
			}
		}
	}

	pluginName, hasPlugin, refusal := extractPluginPipeTarget(validated, "ze show system profile type heap | plugin <name>")
	if refusal != nil {
		return refusal, nil
	}

	wantSummary := hasPipeOp(validated, pipeOpSummary)
	if hasPlugin {
		return inspectPluginProfile(ctx, pluginName, profileType, duration, wantSummary)
	}

	switch profileType {
	case profileTypeCPU:
		return profileCPU(duration, wantSummary)
	default:
		return profileSnapshot(profileType, wantSummary)
	}
}

func profileHandlerTimeout(validated command.ValidatedArgs) time.Duration {
	args := validated.Tokens()
	profileType := profileTypeHeap
	duration := defaultCPUProfileDuration
	hasDuration := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case keyType:
			if i+1 < len(args) {
				i++
				if isSupportedProfileType(args[i]) {
					profileType = args[i]
				}
			}
		case profileTypeCPU, profileTypeHeap, profileTypeGoroutine, profileTypeAllocs, profileTypeMutex, profileTypeBlock:
			profileType = args[i]
		case keyDuration:
			if i+1 < len(args) {
				i++
				if d, err := time.ParseDuration(args[i]); err == nil && d > 0 {
					duration = min(d, maxCPUProfileDuration)
					hasDuration = true
				}
			}
		}
	}

	if profileType == profileTypeCPU {
		return duration + profileTimeoutHeadroom
	}
	if hasDuration {
		return duration + profileTimeoutHeadroom
	}
	return 0
}

func isSupportedProfileType(name string) bool {
	switch name {
	case profileTypeCPU, profileTypeHeap, profileTypeGoroutine, profileTypeAllocs, profileTypeMutex, profileTypeBlock:
		return true
	default:
		return false
	}
}

func hasPipeOp(validated command.ValidatedArgs, name string) bool {
	for _, op := range validated.PipeOperations() {
		if strings.EqualFold(op.Name, name) {
			return true
		}
	}
	return false
}

func profileCPU(duration time.Duration, wantSummary bool) (*plugin.Response, error) {
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

	if wantSummary {
		summary, err := summarizePprof(profileTypeCPU, buf.Bytes())
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				err,
				"cannot summarize CPU profile",
				"the captured pprof data could not be decoded",
				"ze show system profile type cpu",
			)), nil //nolint:nilerr // operator error in Response
		}
		summary["duration"] = duration.String()
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map(summary),
		}, nil
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			keyType:    profileTypeCPU,
			"duration": duration.String(),
			keyFormat:  profileFormatPprofBase64,
			keyData:    base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}, nil
}

func profileSnapshot(profileType string, wantSummary bool) (*plugin.Response, error) {
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

	if wantSummary {
		summary, err := summarizePprof(profileType, buf.Bytes())
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				err,
				"cannot summarize "+profileType+" profile",
				"the captured pprof data could not be decoded",
				"ze show system profile type "+profileType,
			)), nil //nolint:nilerr // operator error in Response
		}
		summary["profile-count"] = p.Count()
		addProfileSamplingRate(summary, profileType)
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map(summary),
		}, nil
	}

	data := plugin.Map{
		keyType:   profileType,
		keyCount:  p.Count(),
		keyFormat: profileFormatPprofBase64,
		keyData:   base64.StdEncoding.EncodeToString(buf.Bytes()),
	}
	addProfileSamplingRate(data, profileType)
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

func addProfileSamplingRate(dst map[string]any, profileType string) {
	switch profileType {
	case profileTypeMutex:
		dst[keySamplingRate] = runtime.SetMutexProfileFraction(-1)
	case profileTypeBlock:
		dst[keySamplingRate] = flightrec.BlockProfileRate()
	}
}
