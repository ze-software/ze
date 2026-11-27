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
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
const configRootBGP = "bgp"

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
var capNames = map[uint8]string{
	2:  "route-refresh",
	70: "enhanced-route-refresh",
}

func capabilityName(codeStr string) (string, bool) {
	code, err := strconv.ParseUint(codeStr, 10, 8)
	if err != nil {
		return "", false
	}
	name, ok := capNames[uint8(code)]
	return name, ok
}

// RunRouteRefreshPlugin runs the route-refresh plugin using the SDK RPC protocol.
func RunRouteRefreshPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-route-refresh", conn)
	defer func() {
		// refuse-by-default: guards sdk.NewWithConn -- teardown close after p.Run returns has no caller recovery
		if err := p.Close(); err != nil {
			logger().Debug("close failed", "err", err)
		}
	}()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		// Route-refresh has no payload — config just enables the capability.
		// Engine handles config-driven capability advertisement in reactor/config.go.
		return nil
	})
	// It keeps WantsConfig so the bgp root counts it as a consumer, and reads
	// no leaf, so a reload has nothing for it to apply.
	p.DeclareAppliesNothing("reads no leaf: the engine advertises route-refresh from the bgp config")

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
	})
	if err != nil {
		logger().Error("route-refresh plugin failed", "error", err)
		return 1
	}

	return 0
}

// formatCapText left-aligns a capability name in a 20-column text field.
func formatCapText(name string) string {
	var tb textbuf.Buffer
	tb.Str(name)
	for tb.Len() < 20 {
		tb.Byte(' ')
	}
	return tb.String()
}

// writeOut writes a string to the output writer and returns any write error.
func writeOut(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
// RFC 2918: Route Refresh capability code 2 (zero payload).
// RFC 7313: Enhanced Route Refresh capability code 70 (zero payload).
func RunDecodeMode(input io.Reader, output io.Writer) int {
	writeUnknown := func() error { return writeOut(output, "decoded unknown\n") }
	writeJSON := func(j []byte) error { return writeOut(output, "decoded json "+string(j)+"\n") }
	writeText := func(t string) error { return writeOut(output, "decoded text "+t+"\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "decode" {
			if err := writeUnknown(); err != nil {
				return 1
			}
			continue
		}

		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 4 {
				if err := writeUnknown(); err != nil {
					return 1
				}
				continue
			}
		}

		if parts[capIdx] != "capability" {
			if err := writeUnknown(); err != nil {
				return 1
			}
			continue
		}

		codeStr := parts[capIdx+1]
		name, ok := capabilityName(codeStr)
		if !ok {
			if err := writeUnknown(); err != nil {
				return 1
			}
			continue
		}

		// RFC 2918/7313: Route Refresh capabilities have zero payload.
		// The hex data field is present but empty or ignored.

		if format == "text" {
			if err := writeText(formatCapText(name)); err != nil {
				return 1
			}
		} else {
			result := map[string]any{
				"name": name,
			}
			jsonBytes, jsonErr := json.Marshal(result)
			if jsonErr != nil {
				if err := writeUnknown(); err != nil {
					return 1
				}
				continue
			}
			if err := writeJSON(jsonBytes); err != nil {
				return 1
			}
		}
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		_ = writeOut(output, eb.Str("decoded error ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// Route Refresh capabilities have zero payload, so hex data is ignored.
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	return RunCLIDecodeWithCode(2, hexData, textOutput, stdout, stderr)
}

// RunCLIDecodeWithCode decodes a route-refresh capability when the caller has
// the capability code. The direct plugin CLI supplies this through --code
// because Route Refresh capability payloads are empty.
func RunCLIDecodeWithCode(capCode uint8, hexData string, textOutput bool, stdout, stderr io.Writer) int {
	// Route Refresh and Enhanced Route Refresh have zero payload.
	// The capability code determines the name; payload is empty.
	name, ok := capNames[capCode]
	if !ok {
		// refuse-by-default: guards capNames -- already returning exit code 1 for unsupported capability code; a broken stderr write has no recovery
		if _, err := fmt.Fprintf(stderr, "error: unsupported route-refresh capability code %d\n", capCode); err != nil { //nolint:errcheck // output
			logger().Debug("write failed", "err", err)
		}
		return 1
	}

	if textOutput {
		if err := writeOut(stdout, formatCapText(name)+"\n"); err != nil {
			return 1
		}
	} else {
		result := map[string]any{
			"code": capCode,
			"name": name,
		}
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			// refuse-by-default: guards capNames -- already returning exit code 1 after JSON marshal failure; a broken stderr write has no recovery
			if _, err := fmt.Fprintf(stderr, "error: JSON encoding: %v\n", jsonErr); err != nil { //nolint:errcheck // output
				logger().Debug("write failed", "err", err)
			}
			return 1
		}
		if _, err := fmt.Fprintln(stdout, string(jsonBytes)); err != nil {
			return 1
		}
	}
	return 0
}
