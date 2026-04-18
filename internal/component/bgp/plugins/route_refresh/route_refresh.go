// Design: docs/architecture/core-design.md — route refresh capability plugin
// RFC: rfc/short/rfc2918.md — route refresh capability
// RFC: rfc/short/rfc7313.md — enhanced route refresh
//
// Package bgp_routerefresh implements a Route Refresh capability plugin for ze.
// It handles Route Refresh (code 2, RFC 2918) and Enhanced Route Refresh
// (code 70, RFC 7313) capability decoding.
//
// RFC 2918: Route Refresh Capability for BGP-4.
// RFC 7313: Enhanced Route Refresh Capability for BGP-4.
package route_refresh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// capNames maps capability codes to names.
var capNames = map[string]string{
	"2":  "route-refresh",
	"70": "enhanced-route-refresh",
}

// RunRouteRefreshPlugin runs the route-refresh plugin using the SDK RPC protocol.
func RunRouteRefreshPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-route-refresh", conn)
	defer func() {
		if err := p.Close(); err != nil {
			logger().Debug("close failed", "err", err)
		}
	}()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		// Route-refresh has no payload — config just enables the capability.
		// Engine handles config-driven capability advertisement in reactor/config.go.
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("route-refresh plugin failed", "error", err)
		return 1
	}

	return 0
}

// writeOut writes a string to the output writer, discarding errors.
// Decode mode writes to an in-memory buffer; write failures are not actionable.
func writeOut(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		logger().Debug("decode write failed", "err", err)
	}
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// RFC 2918: Route Refresh capability code 2 (zero payload).
// RFC 7313: Enhanced Route Refresh capability code 70 (zero payload).
func RunDecodeMode(input io.Reader, output io.Writer) int {
	writeUnknown := func() { writeOut(output, "decoded unknown\n") }
	writeJSON := func(j []byte) { writeOut(output, "decoded json "+string(j)+"\n") }
	writeText := func(t string) { writeOut(output, "decoded text "+t+"\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 4 {
				writeUnknown()
				continue
			}
		}

		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		codeStr := parts[capIdx+1]
		name, ok := capNames[codeStr]
		if !ok {
			writeUnknown()
			continue
		}

		// RFC 2918/7313: Route Refresh capabilities have zero payload.
		// The hex data field is present but empty or ignored.

		if format == "text" {
			writeText(fmt.Sprintf("%-20s", name))
		} else {
			result := map[string]any{
				"name": name,
			}
			jsonBytes, jsonErr := json.Marshal(result)
			if jsonErr != nil {
				writeUnknown()
				continue
			}
			writeJSON(jsonBytes)
		}
	}
	return 0
}

// GetYANG returns the embedded YANG schema for the route-refresh plugin.
func GetYANG() string {
	return schema.ZeRouteRefreshYANG
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// Route Refresh capabilities have zero payload, so hex data is ignored.
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	// Route Refresh and Enhanced Route Refresh have zero payload.
	// The capability code determines the name; payload is empty.
	name := "route-refresh"

	if textOutput {
		if _, err := fmt.Fprintf(stdout, "%-20s\n", name); err != nil {
			logger().Debug("write failed", "err", err)
		}
	} else {
		result := map[string]any{
			"code": 2,
			"name": name,
		}
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			if _, err := fmt.Fprintf(stderr, "error: JSON encoding: %v\n", jsonErr); err != nil {
				logger().Debug("write failed", "err", err)
			}
			return 1
		}
		if _, err := fmt.Fprintln(stdout, string(jsonBytes)); err != nil {
			logger().Debug("write failed", "err", err)
		}
	}
	return 0
}
