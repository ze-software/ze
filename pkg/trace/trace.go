// Package trace provides debug tracing for ZeBGP.
//
// Enable tracing with the ZEBGP_TRACE environment variable:
//
//	ZEBGP_TRACE=config,routes,session zebgp server config.conf
//
// Available trace categories:
//   - config: Configuration parsing and loading
//   - routes: Route handling (static routes, updates)
//   - session: BGP session events
//   - fsm: FSM state transitions
//   - all: Enable all categories
package trace

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Category represents a trace category.
type Category string

// Trace categories.
const (
	Config  Category = "config"
	Routes  Category = "routes"
	Session Category = "session"
	FSM     Category = "fsm"
)

var (
	enabled    map[Category]bool
	enabledMu  sync.RWMutex
	initOnce   sync.Once
	traceAll   bool
	timeFormat = "15:04:05.000"
)

func init() {
	initOnce.Do(initialize)
}

func initialize() {
	enabled = make(map[Category]bool)

	env := os.Getenv("ZEBGP_TRACE")
	if env == "" {
		return
	}

	for _, cat := range strings.Split(env, ",") {
		cat = strings.TrimSpace(strings.ToLower(cat))
		if cat == "all" {
			traceAll = true
			continue
		}
		enabled[Category(cat)] = true
	}
}

// Enabled returns true if the given category is enabled.
func Enabled(cat Category) bool {
	if traceAll {
		return true
	}
	enabledMu.RLock()
	defer enabledMu.RUnlock()
	return enabled[cat]
}

// Log logs a trace message if the category is enabled.
func Log(cat Category, format string, args ...any) {
	if !Enabled(cat) {
		return
	}

	timestamp := time.Now().Format(timeFormat)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[TRACE %s] %s: %s\n", timestamp, cat, msg)
}

// ConfigParsed logs when config is parsed.
func ConfigParsed(path string, neighborCount int, warnings []string) {
	if !Enabled(Config) {
		return
	}
	Log(Config, "parsed %s: %d neighbors", path, neighborCount)
	for _, w := range warnings {
		Log(Config, "  warning: %s", w)
	}
}

// ConfigLoaded logs when config is converted to reactor format.
func ConfigLoaded(neighborCount int) {
	Log(Config, "loaded config with %d neighbors", neighborCount)
}

// NeighborRoutes logs static routes for a neighbor.
func NeighborRoutes(addr string, routeCount int) {
	Log(Routes, "neighbor %s: %d static routes configured", addr, routeCount)
}

// RouteSent logs when a route is sent.
func RouteSent(addr string, prefix string, nextHop string) {
	Log(Routes, "neighbor %s: sent route %s via %s", addr, prefix, nextHop)
}

// SessionConnected logs when a session connects.
func SessionConnected(addr string, port int) {
	Log(Session, "connected to %s:%d", addr, port)
}

// SessionEstablished logs when a session becomes established.
func SessionEstablished(addr string, localAS, peerAS uint32) {
	Log(Session, "session established with %s (local-as=%d, peer-as=%d)", addr, localAS, peerAS)
}

// SessionClosed logs when a session closes.
func SessionClosed(addr string, reason string) {
	Log(Session, "session closed with %s: %s", addr, reason)
}

// FSMTransition logs FSM state changes.
func FSMTransition(addr string, from, to string) {
	Log(FSM, "neighbor %s: %s -> %s", addr, from, to)
}
