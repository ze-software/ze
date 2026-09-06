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

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/sdk"
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
		fam, ok := family.LookupFamily(f)
		if !ok {
			fmt.Fprintf(os.Stderr, "error: unknown family: %s\n", f)
			return exitError
		}
		familyDecls = append(familyDecls, sdk.FamilyDecl{
			Name: f,
			Mode: "both",
			AFI:  uint16(fam.AFI),
			SAFI: uint8(fam.SAFI),
		})
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
			caps = append(caps, sdk.CapabilityDecl{Code: 69, Encoding: sdk.CapEncodingHex, Payload: hex})
		}
	}
	if len(caps) > 0 {
		p.SetCapabilities(caps)
	}

	// Subscribe to all events using text encoding (the bridge translates text events).
	p.SetStartupSubscriptions([]string{"*"}, nil, "")

	// Build the ExaBGP subprocess and its pipes. It is NOT started here: the
	// OnAllPluginsReady registration below starts it, and says what an early
	// start cost.
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
		if _, err := fmt.Fprintln(stdinPipe, string(out)); err != nil { //nolint:errcheck // output
			slog.Warn("sdk: write to plugin failed", "error", err)
		}
		return nil
	})

	// Read subprocess stdout: ExaBGP commands -> ze dispatch.
	ack := bridge.NewAckMode()
	var wg sync.WaitGroup
	readScript := func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}
			line := scanner.Text()
			if line == "" {
				continue
			}

			translation, err := bridge.TranslateLine(line)
			if err != nil {
				// The bridge names the line it refused rather than handing an
				// untranslated line to ze's dispatcher, where it would die as an
				// unknown command with no mention of the bridge.
				slog.Warn("sdk: line refused", "error", err)
				continue
			}
			// A local action is answered by the bridge itself, so it is read
			// BEFORE Nothing: it carries no command and is not an empty line.
			if translation.Local != bridge.LocalNone {
				ack.AnswerLocal(stdinPipe, translation.Local)
				continue
			}
			if translation.Nothing() {
				continue
			}

			// One ExaBGP line can be several ze commands, because ExaBGP puts
			// each prefix on its own UPDATE. The script wrote one line and
			// blocks for one answer, so it is acked once, after the last.
			failed := false
			for _, command := range translation.Commands {
				if _, _, err := p.DispatchCommand(ctx, command); err != nil {
					slog.Warn("sdk: dispatch command failed", "error", err, "cmd", command)
					ack.WriteError(stdinPipe, err.Error())
					failed = true
					break
				}
			}
			if failed {
				continue
			}
			// The script is waiting for this. An ExaBGP API client sends one
			// command, blocks for `done`, and gives up after two seconds, so a
			// runner that dispatches without acking delivers exactly one command
			// per script however well the translation works.
			ack.WriteAck(stdinPipe)

			// For route commands, inject a flush so the forward pool drains. The
			// selector is the one the translator used.
			if translation.Route {
				var tb textbuf.Buffer
				flushCmd := tb.Str("request peer ").Str(translation.Selector).Str(" flush").String()
				if _, _, err := p.DispatchCommand(ctx, flushCmd); err != nil {
					slog.Warn("sdk: flush failed", "error", err, "peer", translation.Selector)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Warn("sdk: plugin stdout scanner error", "error", err)
		}
	}

	// The script starts once the engine has loaded every plugin in every
	// startup phase and frozen the dispatcher registry.
	//
	// The script's first line is a command, and this runner dispatches it the
	// moment it is read. Starting the script before p.Run therefore raced the
	// 5-stage handshake: the engine sat at stage 5 waiting for `ready`, read a
	// dispatch-command frame instead, and aborted the startup barrier, which
	// took the bgp plugin and the daemon down with it. A script line can also
	// address ANOTHER plugin -- `announce watchdog <name>` reaches bgp-watchdog
	// -- so even OnStarted is too early, because a target in a later startup
	// phase has not registered its command yet
	// (pkg/plugin/sdk/sdk_callbacks.go, OnAllPluginsReady).
	p.OnAllPluginsReady(func() error {
		if err := cmd.Start(); err != nil {
			return err
		}
		wg.Go(readScript)
		return nil
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
