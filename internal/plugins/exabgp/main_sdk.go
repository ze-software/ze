// Design: docs/architecture/exabgp-bridge.md -- external SDK/TLS runner
// Overview: main.go -- exabgp CLI entry point and flag parsing
// Related: bridgerun/fleet.go -- the scripts both runners share
//
// When ze's process manager launches the exabgp bridge, it sets
// ZE_PLUGIN_HUB_TOKEN (plus host/port). The bridge detects this and
// connects back via TLS using the SDK instead of using stdin/stdout.

package exabgp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// runSDKMode runs the ExaBGP bridge as an external plugin via TLS connect-back.
// The SDK handles the 5-stage startup protocol and event loop. The scripts are
// run by a bridgerun.Fleet, the same one the internal runner uses, so the fan-out
// to every script's stdin and the read of every script's stdout are written once
// (internal/plugins/exabgp/bridgeplugin/internal.go).
//
// This entry point carries ONE script, the command line the CLI was given. The
// config route carries the whole set an ExaBGP config declares, because that is
// the route `ze exabgp migrate` writes.
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

	// The script is NOT started here: the OnAllPluginsReady registration below
	// starts it, and says what an early start cost.
	//
	// The process manager gives no respawn setting to this entry point, so the
	// script keeps ExaBGP's default: a script that exits is started again
	// (bridgerun/respawn.go states the rate limit that bounds it).
	// The process manager gives this entry point no encoder setting, so the
	// script gets the format the 6.0.0 envelope names, which is the one this
	// runner has always written. The config route carries the operator's own
	// choice (internal/plugins/exabgp/bridgeplugin/config.go, parseEncoder).
	//
	// It gets no peer list either, so it is fed by every peer and its
	// unaddressed commands reach every peer. One script and one command line
	// name no relation to narrow.
	fleet := bridgerun.New(slogutil.Logger("exabgp-bridge"), families, []bridgerun.Script{{
		Name:    filepath.Base(pluginCmd[0]),
		Argv:    pluginCmd,
		Encoder: bridge.EncoderJSON,
		Respawn: true,
	}})

	// ze events -> ExaBGP JSON on the script's stdin.
	p.OnEvent(func(event string) error {
		fleet.Broadcast(event)
		return nil
	})

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
	p.OnAllPluginsReady(func() error { return fleet.Start(ctx, p) })

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

	fleet.Stop()

	return exitOK
}
