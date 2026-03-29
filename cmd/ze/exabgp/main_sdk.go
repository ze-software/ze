// Design: docs/architecture/core-design.md — SDK/TLS connect-back mode for engine-launched bridge
// Overview: main.go — exabgp CLI entry point and flag parsing
//
// When ze's process manager launches the exabgp bridge, it sets
// ZE_PLUGIN_HUB_TOKEN (plus host/port). The bridge detects this and
// connects back via TLS using the SDK instead of using stdin/stdout.

package exabgp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/exabgp/bridge"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// runSDKMode runs the ExaBGP bridge as an external plugin via TLS connect-back.
// The SDK handles the 5-stage startup protocol and event loop. The bridge
// translates between ze JSON events and ExaBGP JSON/text formats.
//
// Returns exit code (0 = success, 1 = error).
func runSDKMode(ctx context.Context, pluginCmd, families []string, routeRefresh bool, addPath string) int {
	p, err := sdk.NewFromTLSEnv("exabgp-bridge")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: TLS connect: %v\n", err)
		return exitError
	}
	defer p.Close() //nolint:errcheck // best-effort cleanup

	// Build family declarations for registration.
	var familyDecls []sdk.FamilyDecl
	for _, f := range families {
		familyDecls = append(familyDecls, sdk.FamilyDecl{Name: f, Mode: "both"})
	}

	// Build capability declarations.
	var caps []sdk.CapabilityDecl
	if routeRefresh {
		// RFC 2918: route-refresh capability, code 2, zero-length value.
		caps = append(caps, sdk.CapabilityDecl{Code: 2})
	}
	if addPath != "" {
		hex := bridge.EncodeAddPathHex(families, addPath)
		if hex != "" {
			caps = append(caps, sdk.CapabilityDecl{Code: 69, Encoding: "hex", Payload: hex})
		}
	}
	if len(caps) > 0 {
		p.SetCapabilities(caps)
	}

	// Subscribe to all events using text encoding (the bridge translates text events).
	p.SetStartupSubscriptions([]string{"*"}, nil, "")

	// Start the ExaBGP subprocess.
	//nolint:gosec // User-provided plugin command is intentional.
	cmd := exec.CommandContext(ctx, pluginCmd[0], pluginCmd[1:]...)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stdin pipe: %v\n", err)
		return exitError
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stdout pipe: %v\n", err)
		return exitError
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: start plugin: %v\n", err)
		return exitError
	}

	// Register event handler: ze events -> ExaBGP JSON on subprocess stdin.
	p.OnEvent(func(event string) error {
		var zebgp map[string]any
		if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
			slog.Warn("sdk: invalid JSON event", "error", err)
			return nil
		}
		exabgpJSON := bridge.ZebgpToExabgpJSON(zebgp)
		out, err := json.Marshal(exabgpJSON)
		if err != nil {
			slog.Warn("sdk: marshal ExaBGP JSON failed", "error", err)
			return nil
		}
		if _, err := fmt.Fprintln(stdinPipe, string(out)); err != nil {
			slog.Warn("sdk: write to plugin failed", "error", err)
		}
		return nil
	})

	// Read subprocess stdout in a goroutine: ExaBGP commands -> ze dispatch.
	var wg sync.WaitGroup
	wg.Go(func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}
			line := scanner.Text()
			if line == "" {
				continue
			}

			zebgpCmd := bridge.ExabgpToZebgpCommand(line)
			if zebgpCmd == "" {
				continue
			}

			if _, _, err := p.DispatchCommand(ctx, zebgpCmd); err != nil {
				slog.Warn("sdk: dispatch command failed", "error", err, "cmd", zebgpCmd)
				continue
			}

			// For route commands, inject a flush so the forward pool drains.
			if bridge.IsRouteCommand(zebgpCmd) {
				peerAddr := bridge.ExtractPeerAddress(zebgpCmd)
				if peerAddr != "" {
					flushCmd := fmt.Sprintf("peer %s flush", peerAddr)
					if _, _, err := p.DispatchCommand(ctx, flushCmd); err != nil {
						slog.Warn("sdk: flush failed", "error", err, "peer", peerAddr)
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Warn("sdk: plugin stdout scanner error", "error", err)
		}
	})

	// Run SDK event loop (blocks until bye or context cancel).
	reg := sdk.Registration{
		Families:    familyDecls,
		WantsConfig: []string{"bgp"},
	}
	if sdkErr := p.Run(ctx, reg); sdkErr != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: SDK run: %v\n", sdkErr)
		}
	}

	// Clean up subprocess.
	stdinPipe.Close() //nolint:errcheck,gosec // trigger EOF for subprocess
	_ = cmd.Wait()
	wg.Wait()

	return exitOK
}
