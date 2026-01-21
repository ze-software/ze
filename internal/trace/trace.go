// Package trace provides debug tracing for ZeBGP.
//
// Enable tracing with the ze.bgp.debug.trace environment variable:
//
//	ze_bgp_debug_trace=config,routes,session ze bgp server config.conf
//
// Or with dot notation:
//
//	ze.bgp.debug.trace=all ze bgp server config.conf
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

	// Check dot notation first (higher priority)
	env := os.Getenv("ze.bgp.debug.trace")
	if env == "" {
		// Fall back to underscore notation
		env = os.Getenv("ze_bgp_debug_trace")
	}
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
func ConfigParsed(path string, peerCount int, warnings []string) {
	if !Enabled(Config) {
		return
	}
	Log(Config, "parsed %s: %d peers", path, peerCount)
	for _, w := range warnings {
		Log(Config, "  warning: %s", w)
	}
}

// ConfigLoaded logs when config is converted to reactor format.
func ConfigLoaded(peerCount int) {
	Log(Config, "loaded config with %d peers", peerCount)
}

// PeerRoutes logs static routes for a peer.
func PeerRoutes(addr string, routeCount int) {
	Log(Routes, "peer %s: %d static routes configured", addr, routeCount)
}

// RouteSent logs when a route is sent.
func RouteSent(addr string, prefix string, nextHop string) {
	Log(Routes, "peer %s: sent route %s via %s", addr, prefix, nextHop)
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
	Log(FSM, "peer %s: %s -> %s", addr, from, to)
}

// UpdateFamilyMismatch logs when UPDATE contains non-negotiated AFI/SAFI.
// RFC 4760 Section 6: speaker MAY treat this as error.
func UpdateFamilyMismatch(afi uint16, safi uint8, ignored bool) {
	action := "rejected"
	if ignored {
		action = "ignored (ignore-mismatch enabled)"
	}
	Log(Session, "UPDATE with non-negotiated family AFI=%d SAFI=%d: %s", afi, safi, action)
}

// RFC7606TreatAsWithdraw logs when RFC 7606 treat-as-withdraw is applied.
// RFC 7606 Section 2: Routes in UPDATE treated as withdrawn.
func RFC7606TreatAsWithdraw(attrCode uint8, description string) {
	Log(Session, "RFC 7606 treat-as-withdraw: attr=%d %s", attrCode, description)
}

// RFC7606AttributeDiscard logs when RFC 7606 attribute-discard is applied.
// RFC 7606 Section 2: Malformed attribute discarded, UPDATE continues.
func RFC7606AttributeDiscard(attrCode uint8, description string) {
	Log(Session, "RFC 7606 attribute-discard: attr=%d %s", attrCode, description)
}

// RFC7606SessionReset logs when RFC 7606 requires session reset.
// RFC 7606 Section 3.g: Multiple MP_REACH/MP_UNREACH requires reset.
func RFC7606SessionReset(description string) {
	Log(Session, "RFC 7606 session-reset: %s", description)
}
