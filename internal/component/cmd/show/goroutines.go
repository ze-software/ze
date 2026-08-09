// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- goroutine dump via runtime.Stack
// Related: system.go -- existing system-cpu handler reports goroutine count

package show

import (
	"runtime"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	goroutineFullBufSize = 16 << 20 // 16 MB
	goroutineModeSummary = "summary"
	goroutineModeBlocked = "blocked"
	goroutineModeFull    = "full"
)

// goroutineFullGuard deduplicates concurrent full goroutine dumps so only
// one 16 MB allocation happens at a time. Waiters share the result.
var goroutineFullGuard struct {
	mu      sync.Mutex
	running bool
	result  string
	done    chan struct{}
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-goroutines", Handler: handleShowSystemGoroutines},
	)
}

func handleShowSystemGoroutines(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	mode := goroutineModeSummary
	for _, a := range args {
		switch a {
		case goroutineModeSummary, goroutineModeBlocked, goroutineModeFull:
			mode = a
		}
	}

	switch mode {
	case goroutineModeFull:
		return goroutinesFull()
	case goroutineModeBlocked:
		return goroutinesFiltered(true)
	default:
		return goroutinesFiltered(false)
	}
}

func goroutinesFull() (*plugin.Response, error) {
	g := &goroutineFullGuard
	g.mu.Lock()
	if g.running {
		ch := g.done
		g.mu.Unlock()
		<-ch
		g.mu.Lock()
		res := g.result
		g.mu.Unlock()
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"mode": goroutineModeFull, "stacks": res},
		}, nil
	}
	g.running = true
	g.done = make(chan struct{})
	g.mu.Unlock()

	buf := make([]byte, goroutineFullBufSize)
	n := runtime.Stack(buf, true)
	stacks := string(buf[:n])

	g.mu.Lock()
	g.result = stacks
	g.running = false
	close(g.done)
	g.mu.Unlock()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"mode": goroutineModeFull, "stacks": stacks},
	}, nil
}

var blockedStates = map[string]bool{
	"chan receive":   true,
	"select":         true,
	"semacquire":     true,
	"IO wait":        true,
	"chan send":      true,
	"sync.Cond.Wait": true,
}

type goroutineInfo struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Stack string `json:"stack"`
}

func goroutinesFiltered(onlyBlocked bool) (*plugin.Response, error) {
	buf := make([]byte, 1<<20) // 1 MB initial; grows if needed
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		if len(buf) >= goroutineFullBufSize {
			buf = buf[:n]
			break
		}
		buf = make([]byte, min(len(buf)*2, goroutineFullBufSize))
	}
	stacks := string(buf)

	goroutines := parseGoroutineStacks(stacks)

	counts := map[string]int{}
	var filtered []goroutineInfo
	for _, g := range goroutines {
		counts[g.State]++
		if onlyBlocked && !blockedStates[g.State] {
			continue
		}
		filtered = append(filtered, g)
	}

	result := map[string]any{
		"total":    len(goroutines),
		"by-state": counts,
	}
	if onlyBlocked {
		result["mode"] = goroutineModeBlocked
		result["blocked"] = filtered
		result["blocked-count"] = len(filtered)
	} else {
		result["mode"] = goroutineModeSummary
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}

func parseGoroutineStacks(stacks string) []goroutineInfo {
	var result []goroutineInfo
	for block := range strings.SplitSeq(stacks, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		header, stack, _ := strings.Cut(block, "\n")
		id, state := parseGoroutineHeader(header)
		if id == "" {
			continue
		}
		result = append(result, goroutineInfo{
			ID:    id,
			State: state,
			Stack: stack,
		})
	}
	return result
}

func parseGoroutineHeader(header string) (id, state string) {
	if !strings.HasPrefix(header, "goroutine ") {
		return "", ""
	}
	rest := header[len("goroutine "):]
	id, rest, ok := strings.Cut(rest, " ")
	if !ok {
		return "", ""
	}
	bracketStart := strings.IndexByte(rest, '[')
	bracketEnd := strings.IndexByte(rest, ']')
	if bracketStart < 0 || bracketEnd < 0 || bracketEnd <= bracketStart {
		return id, "unknown"
	}
	state = rest[bracketStart+1 : bracketEnd]
	if s, _, found := strings.Cut(state, ","); found {
		state = s
	}
	return id, state
}
