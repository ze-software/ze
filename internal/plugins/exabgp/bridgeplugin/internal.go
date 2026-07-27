// Design: plan/learned/1095-followup-subsystem.md AC-1 -- internal exabgp bridge runner
//
// runInternalBridge is the RunEngine entry point for the internal exabgp-bridge
// plugin. It mirrors the external SDK-mode runner (internal/plugins/exabgp
// main_sdk.go runSDKMode) but sources the script command from the exabgp.bridge
// config root delivered via the SDK OnConfigure (Stage 2) callback, since a
// RunEngine(conn net.Conn) runner never sees the process-manager `run` line.

package bridgeplugin

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// familyDecls builds the Stage-1 family declarations. Config is not available
// at Stage-1 registration (it arrives at Stage-2 OnConfigure), so the runner
// declares the CLI-default family here; the `family` leaf refines the ADD-PATH
// capability encoding at Stage-3 (see runInternalBridge).
func familyDecls(families []string) []sdk.FamilyDecl {
	var decls []sdk.FamilyDecl
	for _, f := range families {
		fam, ok := family.LookupFamily(f)
		if !ok {
			continue
		}
		decls = append(decls, sdk.FamilyDecl{Name: f, Mode: "both", AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI)})
	}
	if len(decls) == 0 {
		if fam, ok := family.LookupFamily(defaultFamily); ok {
			decls = append(decls, sdk.FamilyDecl{Name: defaultFamily, Mode: "both", AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI)})
		}
	}
	return decls
}

// capabilityDecls builds the Stage-3 capability declarations from config.
func capabilityDecls(cfg bridgeConfig) []sdk.CapabilityDecl {
	var caps []sdk.CapabilityDecl
	if cfg.RouteRefresh {
		// RFC 2918: route-refresh capability, code 2, zero-length value.
		caps = append(caps, sdk.CapabilityDecl{Code: 2})
	}
	if cfg.AddPath != "" && cfg.AddPath != addPathNone {
		if hex := bridge.EncodeAddPathHex(cfg.Families, cfg.AddPath); hex != "" {
			caps = append(caps, sdk.CapabilityDecl{Code: 69, Encoding: sdk.CapEncodingHex, Payload: hex})
		}
	}
	return caps
}

// bridgeRunner holds the subprocess state shared between the OnConfigure
// (start) callback, OnEvent (write) callback, and the shutdown path.
type bridgeRunner struct {
	log *slog.Logger

	mu       sync.Mutex
	started  bool
	child    *exec.Cmd
	stdin    io.WriteCloser
	readerWG sync.WaitGroup
}

// runInternalBridge runs the ExaBGP bridge in-process. It spawns the operator's
// script as a subprocess and translates between ze JSON events and the external
// text/JSON commands, exactly like the external SDK-mode runner but with the
// script command sourced from config. Returns the plugin exit code.
func runInternalBridge(conn net.Conn) int {
	log := logger()
	r := &bridgeRunner{log: log}

	p := sdk.NewWithConn("exabgp-bridge", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	// Subscribe to all events so the bridge can translate them for the script.
	// Config-independent, so set before Run (Stage 5 reads it after this).
	p.SetStartupSubscriptions([]string{"*"}, nil, "")

	// ze events -> external JSON on the subprocess stdin. Registered before Run
	// so the bridge (Stage 5) captures it; no-ops until the subprocess starts.
	p.OnEvent(r.onEvent)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				return err
			}
			if !cfg.Present {
				continue
			}
			if err := r.applyConfig(ctx, p, cfg); err != nil {
				return err
			}
			return nil
		}
		return nil
	})

	reg := sdk.Registration{
		Families:    familyDecls([]string{defaultFamily}),
		WantsConfig: []string{configRoot},
	}
	runErr := p.Run(ctx, reg)

	// Shutdown: cancel the subprocess context, close stdin (EOF), wait.
	cancel()
	r.stop()

	if runErr != nil && ctx.Err() == nil {
		log.Error("exabgp-bridge internal plugin failed", "error", runErr)
		return 1
	}
	log.Info("exabgp-bridge internal plugin stopped")
	return 0
}

// applyConfig starts the subprocess on the first committed config. Reloads do
// not restart the subprocess: the script owns its own lifecycle, and
// family/capability changes need a plugin restart (engine reconcile), matching
// the external mode.
func (r *bridgeRunner) applyConfig(ctx context.Context, p *sdk.Plugin, cfg bridgeConfig) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if cfg.Run == "" {
		return errRunRequired
	}

	if caps := capabilityDecls(cfg); len(caps) > 0 {
		p.SetCapabilities(caps)
	}

	if err := r.startScript(ctx, p, cfg); err != nil {
		return err
	}
	r.log.Info("exabgp-bridge started", "run", cfg.Run, "families", cfg.Families)
	return nil
}

// onEvent translates a ze JSON event into external JSON and writes it to the
// subprocess stdin. No-ops until the subprocess is running.
func (r *bridgeRunner) onEvent(event string) error {
	r.mu.Lock()
	w := r.stdin
	r.mu.Unlock()
	if w == nil {
		return nil
	}
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		r.log.Warn("invalid JSON event", "error", err)
		return nil
	}
	out, err := json.Marshal(bridge.ZebgpToExabgpJSON(zebgp))
	if err != nil {
		r.log.Warn("marshal external JSON failed", "error", err)
		return nil
	}
	out = append(out, '\n')
	if _, werr := w.Write(out); werr != nil {
		r.log.Warn("write to plugin failed", "error", werr)
	}
	return nil
}

// startScript launches the script subprocess and the stdout reader goroutine
// that dispatches translated commands to the engine.
func (r *bridgeRunner) startScript(ctx context.Context, p *sdk.Plugin, cfg bridgeConfig) error {
	argv := splitCommand(cfg.Run)
	if len(argv) == 0 {
		return errRunRequired
	}

	//nolint:gosec // G204: operator-provided script command is intentional (parity with external mode).
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	sin, err := c.StdinPipe()
	if err != nil {
		return err
	}
	sout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	c.Stderr = os.Stderr

	if err := c.Start(); err != nil {
		return err
	}

	r.mu.Lock()
	r.child = c
	r.stdin = sin
	r.started = true
	r.mu.Unlock()

	r.readerWG.Add(1)
	go r.readLoop(ctx, p, sout)
	return nil
}

// readLoop reads the script's stdout, translating each line into a ze command
// and dispatching it to the engine.
func (r *bridgeRunner) readLoop(ctx context.Context, p *sdk.Plugin, sout io.Reader) {
	defer r.readerWG.Done()
	scanner := bufio.NewScanner(sout)
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
		if _, _, derr := p.DispatchCommand(ctx, zebgpCmd); derr != nil {
			r.log.Warn("dispatch command failed", "error", derr, "cmd", zebgpCmd)
			continue
		}
		// Route commands: inject a per-peer flush so the forward pool drains.
		if bridge.IsRouteCommand(zebgpCmd) {
			if peerAddr := bridge.ExtractPeerAddress(zebgpCmd); peerAddr != "" {
				var tb textbuf.Buffer
				flushCmd := tb.Str("request peer ").Str(peerAddr).Str(" flush").String()
				if _, _, ferr := p.DispatchCommand(ctx, flushCmd); ferr != nil {
					r.log.Warn("flush failed", "error", ferr, "peer", peerAddr)
				}
			}
		}
	}
	if serr := scanner.Err(); serr != nil {
		r.log.Warn("plugin stdout scanner error", "error", serr)
	}
}

// stop closes the subprocess stdin (EOF), waits for it and the reader goroutine.
func (r *bridgeRunner) stop() {
	r.mu.Lock()
	w := r.stdin
	c := r.child
	r.mu.Unlock()
	if w != nil {
		if err := w.Close(); err != nil {
			r.log.Debug("close plugin stdin", "error", err)
		}
	}
	if c != nil {
		if err := c.Wait(); err != nil {
			r.log.Debug("subprocess exited", "error", err)
		}
	}
	r.readerWG.Wait()
}

// splitCommand splits an operator run string into argv on whitespace. The
// external mode receives argv already split by the shell/flag parser; internal
// mode receives one config string, so it splits here.
func splitCommand(cmd string) []string {
	return strings.Fields(cmd)
}
