// Design: docs/architecture/exabgp-bridge.md -- external SDK/TLS runner for an ExaBGP bus endpoint.
// Overview: main.go -- exabgp CLI entry point and flag parsing.
// Related: bridgerun/fleet.go -- single-script lifecycle and bus filter wiring.
//
// When ze's process manager launches `ze exabgp plugin`, it sets
// ZE_PLUGIN_HUB_TOKEN (plus host/port/CA/name). The wrapper detects this and
// connects back via TLS as an ordinary bus endpoint. When started with
// ZE_PLUGIN_MODE=declare, sdk.RunOrDeclare emits the Stage 1 declaration JSON
// line to stdout and exits 0 without connecting or starting the script.

package exabgp

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// buildRegistration constructs the pure Stage 1 sdk.Registration for an ExaBGP
// wrapper endpoint from its validated --family list and translator tables.
func buildRegistration(families []string) (sdk.Registration, error) {
	familyDecls := make([]sdk.FamilyDecl, 0, len(families))
	for _, f := range families {
		afi, safi, ok := bridge.ParseFamilyAFISAFI(f)
		if !ok {
			return sdk.Registration{}, fmt.Errorf("unknown family: %s", f)
		}
		familyDecls = append(familyDecls, sdk.FamilyDecl{
			Name: f,
			Mode: "encode",
			AFI:  afi,
			SAFI: safi,
		})
	}
	return sdk.Registration{
		FailurePolicy: sdk.FailureRestart,
		Families:      familyDecls,
		WantsConfig:   []string{"bgp"},
		Sends:         bridgerun.BridgeSends(),
		Receives:      bridgerun.BridgeReceives(),
	}, nil
}

// buildCapabilityDecls builds the Stage 3 virtual session capability
// declarations for the wrapper's --route-refresh and --add-path options,
// scoped to peers when `send > to` narrows the endpoint (Q-E9).
func buildCapabilityDecls(families []string, routeRefresh bool, addPath string, peers []string) []sdk.CapabilityDecl {
	var caps []sdk.CapabilityDecl
	if routeRefresh {
		// RFC 2918: route-refresh capability, code 2, zero-length value.
		caps = append(caps, sdk.CapabilityDecl{
			Code:  2,
			Peers: slices.Clone(peers),
		})
	}
	if addPath != "" {
		hex := bridge.EncodeAddPathHex(families, addPath)
		if hex != "" {
			caps = append(caps, sdk.CapabilityDecl{
				Code:     69,
				Encoding: sdk.CapEncodingHex,
				Payload:  hex,
				Peers:    slices.Clone(peers),
			})
		}
	}
	return caps
}

// resolveEndpointName returns the endpoint identity from ZE_PLUGIN_NAME,
// falling back to the script's basename when ZE_PLUGIN_NAME is unset.
func resolveEndpointName(pluginCmd []string) string {
	if name := env.Get("ze.plugin.name"); name != "" {
		return name
	}
	return filepath.Base(pluginCmd[0])
}

// runSDKMode runs the ExaBGP wrapper in query mode (ZE_PLUGIN_MODE=declare) or
// as an external bus endpoint over TLS connect-back.
func runSDKMode(ctx context.Context, opts pluginOptions) int {
	reg, err := buildRegistration(opts.Families)
	if err != nil {
		writeStderrError("", err)
		return exitError
	}

	return sdk.RunOrDeclare(&reg, func() int {
		endpointName := resolveEndpointName(opts.Cmd)
		p, err := sdk.NewFromTLSEnv(endpointName)
		if err != nil {
			writeStderrError("TLS connect", err)
			return exitError
		}
		defer p.Close() //nolint:errcheck // best-effort cleanup
		return runSDKWithPlugin(ctx, p, endpointName, opts, &reg)
	})
}

// runSDKWithPlugin wires one ExaBGP script Fleet onto an initialized *sdk.Plugin,
// runs the SDK event loop, and mirrors the script's exit code on exit (E-6).
func runSDKWithPlugin(ctx context.Context, p *sdk.Plugin, endpointName string, opts pluginOptions, reg *sdk.Registration) int {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	scripts := []bridgerun.Script{{
		Name:    endpointName,
		Argv:    opts.Cmd,
		Encoder: opts.Encoder,
		Forms:   opts.Forms,
	}}
	fleet := bridgerun.New(slogutil.Logger("exabgp-bridge"), opts.Families, scripts)

	var scriptExited atomic.Bool
	var scriptExitCode atomic.Int32
	fleet.SetOnExit(func(code int) {
		scriptExitCode.Store(int32(code))
		scriptExited.Store(true)
		cancelRun()
	})

	// Set initial format before Stage 2; Stage 2 OnConfigure refines it from
	// the participant's compiled bus filter.
	p.SetEventFormat(bridgerun.EventFormat(nil, opts.Forms))

	p.OnConfigure(func([]sdk.ConfigSection) error {
		filter := p.ParticipantFilter()
		fleet.ApplyFilter(filter)
		p.SetEventFormat(bridgerun.EventFormat(filter, opts.Forms))
		return nil
	})
	p.OnParticipantFilter(func(filter *rpc.BusParticipantFilter) {
		fleet.ApplyFilter(filter)
		p.SetEventFormat(bridgerun.EventFormat(filter, opts.Forms))
	})

	// Stage 3 virtual session capabilities (E-4, Q-E9): scoped to the peers
	// permitted by `send > to` when narrowed, global when unscoped, and empty
	// when `send > to none` or `send > allow none` denies all sends.
	p.DeclareCapabilitiesFrom(func([]sdk.ConfigSection) (sdk.CapabilityDeclaration, error) {
		peers, noPeers := fleet.CapabilityScope()
		if noPeers {
			return sdk.CapabilityDeclaration{}, nil
		}
		return sdk.CapabilityDeclaration{
			Capabilities: buildCapabilityDecls(opts.Families, opts.RouteRefresh, opts.AddPath, peers),
		}, nil
	})
	p.DeclareAppliesNothing("takes its config from the command line, not from the config tree")

	p.OnEvent(func(event string) error {
		fleet.Broadcast(event)
		return nil
	})

	p.OnAllPluginsReady(func() error { return fleet.Start(runCtx, p) })

	sdkErr := p.Run(runCtx, reg)
	if !scriptExited.Load() {
		if sdkErr != nil && ctx.Err() == nil {
			writeStderrError("SDK run", sdkErr)
		}
		fleet.Shutdown()
	}
	cancelRun()
	fleet.Stop()

	if scriptExited.Load() {
		return int(scriptExitCode.Load())
	}
	if sdkErr != nil && ctx.Err() == nil {
		return exitError
	}
	return exitOK
}
