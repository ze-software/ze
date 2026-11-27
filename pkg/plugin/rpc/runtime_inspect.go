// Design: docs/architecture/api/process-protocol.md -- independent runtime inspection callback.
// Related: bridge_runtime_inspect.go -- DirectBridge slot for in-process plugins.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// MethodRuntimeInspect inspects one plugin process's Go runtime on an
// independent read-only callback worker without blocking ordinary callbacks.
const MethodRuntimeInspect = "ze-plugin-callback:runtime-inspect"

// Runtime inspection kinds admitted by ze-plugin-callback:runtime-inspect.
const (
	RuntimeInspectGoroutines   = "goroutines"
	RuntimeInspectProfile      = "profile"
	RuntimeInspectLogRecent    = "log-recent"
	RuntimeInspectRuntimeStats = "runtime-stats"
)

// Profile types admitted when RuntimeInspectInput.Kind is RuntimeInspectProfile.
const (
	RuntimeInspectProfileCPU       = "cpu"
	RuntimeInspectProfileHeap      = "heap"
	RuntimeInspectProfileGoroutine = "goroutine"
	RuntimeInspectProfileAllocs    = "allocs"
	RuntimeInspectProfileMutex     = "mutex"
	RuntimeInspectProfileBlock     = "block"
)

// Wire payload and deadline bounds for ze-plugin-callback:runtime-inspect.
const (
	// RuntimeInspectMaxGoroutineBytes bounds raw runtime.Stack output (4 MiB).
	RuntimeInspectMaxGoroutineBytes = 4 << 20
	// RuntimeInspectMaxProfileBytes bounds raw pprof bytes (8 MiB -> <= 10.7 MiB
	// base64, safely below MaxMessageSize).
	RuntimeInspectMaxProfileBytes = 8 << 20
	// RuntimeInspectMaxLogEntries bounds log-recent snapshots.
	RuntimeInspectMaxLogEntries = 256

	RuntimeInspectDefaultTimeout       = 10 * time.Second
	RuntimeInspectMaxTimeout           = 65 * time.Second
	RuntimeInspectMinCPUDurationMS     = 1000
	RuntimeInspectMaxCPUDurationMS     = 60000
	RuntimeInspectDefaultCPUDurationMS = 10000

	runtimeInspectCPUBudgetOverhead = 5 * time.Second
)

// RuntimeInspectInput selects one bounded read-only runtime diagnostic from a
// plugin process. Callers borrow their input until the call returns.
type RuntimeInspectInput struct {
	Kind         string `json:"kind"`
	DeadlineMS   int64  `json:"deadline-ms"`
	ProfileType  string `json:"profile-type,omitempty"`
	DurationMS   int64  `json:"duration-ms,omitempty"`
	LogLevel     string `json:"log-level,omitempty"`
	LogComponent string `json:"log-component,omitempty"`
	LogCount     int    `json:"log-count,omitempty"`
}

// RuntimeInspectLogEntry is one structured log record captured from the plugin's
// process-local log ring.
type RuntimeInspectLogEntry struct {
	Timestamp  string         `json:"timestamp"`
	Level      string         `json:"level"`
	Component  string         `json:"component"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// RuntimeInspectStats reports Go runtime memory and scheduler statistics from
// inside a plugin process.
type RuntimeInspectStats struct {
	Alloc         uint64  `json:"alloc"`
	TotalAlloc    uint64  `json:"total-alloc"`
	Sys           uint64  `json:"sys"`
	HeapAlloc     uint64  `json:"heap-alloc"`
	HeapSys       uint64  `json:"heap-sys"`
	HeapInuse     uint64  `json:"heap-in-use"`
	HeapObjects   uint64  `json:"heap-objects"`
	StackInuse    uint64  `json:"stack-in-use"`
	NumGC         uint32  `json:"num-gc"`
	GCCPUPct      float64 `json:"gc-cpu-pct"`
	NumGoroutines int     `json:"num-goroutines"`
	NumCPU        int     `json:"num-cpu"`
	MaxProcs      int     `json:"max-procs"`
	GoVersion     string  `json:"go-version"`
}

// RuntimeInspectOutput carries one bounded runtime inspection snapshot from a
// plugin process. Nil output is invalid.
type RuntimeInspectOutput struct {
	Kind            string                   `json:"kind"`
	PID             int                      `json:"pid"`
	Truncated       bool                     `json:"truncated,omitempty"`
	GoroutineStacks string                   `json:"goroutine-stacks,omitempty"`
	ProfileType     string                   `json:"profile-type,omitempty"`
	ProfileCount    int                      `json:"profile-count,omitempty"`
	ProfileData     string                   `json:"profile-data,omitempty"`
	SamplingRate    *int                     `json:"sampling-rate,omitempty"`
	LogEntries      []RuntimeInspectLogEntry `json:"log-entries,omitempty"`
	RuntimeStats    *RuntimeInspectStats     `json:"runtime-stats,omitempty"`
}

// Validate checks raw inspection input at either transport's admission boundary.
func (input RuntimeInspectInput) Validate(now time.Time) error {
	switch input.Kind {
	case RuntimeInspectGoroutines, RuntimeInspectProfile, RuntimeInspectLogRecent, RuntimeInspectRuntimeStats:
	default:
		return errors.New("runtime-inspect kind is invalid; use goroutines, profile, log-recent, or runtime-stats")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("runtime-inspect deadline expired; retry with a live deadline")
	}
	if input.DeadlineMS > now.Add(RuntimeInspectMaxTimeout).UnixMilli() {
		return errors.New("runtime-inspect deadline exceeds 65 seconds; bound the inspection timeout")
	}
	switch input.Kind {
	case RuntimeInspectProfile:
		if err := validateRuntimeInspectProfileInput(input); err != nil {
			return err
		}
	case RuntimeInspectLogRecent:
		if input.LogCount < 0 {
			return errors.New("runtime-inspect log-count is negative; use 0..256")
		}
		if input.LogCount > RuntimeInspectMaxLogEntries {
			return errors.New("runtime-inspect log-count exceeds 256; reduce the requested log count")
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if idPrefixMax+len(MethodRuntimeInspect)+1+len(encoded) > MaxMessageSize {
		return errors.New("runtime-inspect request exceeds the frame limit; reduce filter arguments")
	}
	return nil
}

func validateRuntimeInspectProfileInput(input RuntimeInspectInput) error {
	switch input.ProfileType {
	case RuntimeInspectProfileCPU, RuntimeInspectProfileHeap, RuntimeInspectProfileGoroutine,
		RuntimeInspectProfileAllocs, RuntimeInspectProfileMutex, RuntimeInspectProfileBlock:
	default:
		return errors.New("runtime-inspect profile-type is invalid; use cpu, heap, goroutine, allocs, mutex, or block")
	}
	if input.DurationMS < 0 {
		return errors.New("runtime-inspect duration-ms is negative; use 1000..60000 for cpu profiles")
	}
	if input.ProfileType != RuntimeInspectProfileCPU {
		return nil
	}
	if input.DurationMS == 0 {
		return nil
	}
	if input.DurationMS < RuntimeInspectMinCPUDurationMS {
		return errors.New("runtime-inspect cpu duration-ms is below 1000; use 1000..60000")
	}
	if input.DurationMS > RuntimeInspectMaxCPUDurationMS {
		return errors.New("runtime-inspect cpu duration-ms exceeds 60000; use 1000..60000")
	}
	return nil
}

// Validate checks a runtime-inspect reply on both DirectBridge and MuxConn.
func (out *RuntimeInspectOutput) Validate(input RuntimeInspectInput) error {
	if out == nil {
		return errors.New("runtime-inspect reply is missing; return a populated inspection output")
	}
	if out.Kind != input.Kind {
		return errors.New("runtime-inspect reply kind differs; echo the requested kind")
	}
	if out.PID <= 0 {
		return errors.New("runtime-inspect reply pid is not positive; report the plugin process id")
	}
	switch input.Kind {
	case RuntimeInspectGoroutines:
		if len(out.GoroutineStacks) > RuntimeInspectMaxGoroutineBytes {
			return errors.New("runtime-inspect goroutine-stacks exceeds 4 MiB; truncate before replying")
		}
	case RuntimeInspectProfile:
		if out.ProfileType != input.ProfileType {
			return errors.New("runtime-inspect reply profile-type differs; echo the requested profile type")
		}
	case RuntimeInspectLogRecent:
		if len(out.LogEntries) > RuntimeInspectMaxLogEntries {
			return errors.New("runtime-inspect log-entries exceeds 256; bound entries before replying")
		}
	case RuntimeInspectRuntimeStats:
		if out.RuntimeStats == nil {
			return errors.New("runtime-inspect runtime-stats payload is missing; populate runtime-stats")
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if idPrefixMax+len("ok ")+len(encoded) > MaxMessageSize {
		return errors.New("runtime-inspect reply exceeds the frame limit; truncate output before replying")
	}
	return nil
}

// RuntimeInspectCallContext bounds the inspection call and stamps its wire
// deadline. CPU profiles receive duration + 5s up to RuntimeInspectMaxTimeout;
// all other kinds receive RuntimeInspectDefaultTimeout (or the caller's tighter
// deadline). The returned cancel MUST be called.
func RuntimeInspectCallContext(ctx context.Context, input *RuntimeInspectInput) (context.Context, context.CancelFunc) {
	timeout := RuntimeInspectDefaultTimeout
	if input.Kind == RuntimeInspectProfile {
		if input.ProfileType == RuntimeInspectProfileCPU {
			durMS := input.DurationMS
			if durMS <= 0 {
				durMS = RuntimeInspectDefaultCPUDurationMS
			}
			if durMS > RuntimeInspectMaxCPUDurationMS {
				durMS = RuntimeInspectMaxCPUDurationMS
			}
			timeout = min(time.Duration(durMS)*time.Millisecond+runtimeInspectCPUBudgetOverhead, RuntimeInspectMaxTimeout)
		}
	}
	work, cancel := context.WithTimeout(ctx, timeout)
	if input.DeadlineMS > 0 {
		deadline, _ := work.Deadline()
		requested := time.UnixMilli(input.DeadlineMS)
		if requested.Before(deadline) {
			cancel()
			work, cancel = context.WithDeadline(ctx, requested)
		}
	}
	deadline, _ := work.Deadline()
	input.DeadlineMS = deadline.UnixMilli()
	return work, cancel
}

// RuntimeInspect queries this exact connection, bypassing ordinary callback
// execution at the recipient through its registered independent ingress lane.
func (m *MuxConn) RuntimeInspect(ctx context.Context, input RuntimeInspectInput) (*RuntimeInspectOutput, error) {
	ctx, cancel := RuntimeInspectCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	data, err := m.CallRPC(ctx, MethodRuntimeInspect, input)
	if err != nil {
		return nil, err
	}
	var output RuntimeInspectOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("runtime-inspect reply: %w", err)
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return &output, nil
}
