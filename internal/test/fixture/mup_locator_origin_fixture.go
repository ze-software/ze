// Design: docs/architecture/isis/isis-4-component-config.md -- functional-test fixtures for MUP Segment Discovery locator-originator checks
// Related: plugin_fixture_13.go -- routeServerReplay13 fence observer and runPlugin13WithRegistration
// Related: bus_send_fixtures.go -- busRunPlugin and busWriteAtomicFile

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	mupFixtureSourceName   = "locator-publisher"
	mupFixtureOriginatorID = "0100.0000.0001"
	mupFixtureLocatorCIDR  = "2001:db8::/48"
	mupFixtureOrigAddr     = "2001:db8:140::1"
	mupFixtureOrigPfx64    = "2001:db8:236:10::/64"
	mupFixtureReportSource = "bgp-nlri-mup"
	mupFixtureUnresFix     = "ze show isis database"
	mupFixtureDefaultFence = "10.0.0.0/24"
)

type mupObserverOpts struct {
	fence         string
	readyFile     string
	waitFile      string
	keepHex       []string
	absentHex     []string
	expectWarning string
	expectDoctor  string
	expectError   string
}

func parseMUPObserverArgs(args []string) (mupObserverOpts, error) {
	opts := mupObserverOpts{fence: mupFixtureDefaultFence}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--fence="):
			opts.fence = strings.TrimPrefix(arg, "--fence=")
		case strings.HasPrefix(arg, "--ready-file="):
			val := strings.TrimSpace(strings.TrimPrefix(arg, "--ready-file="))
			if val == "" {
				return opts, errors.New("mup-segment-discovery-observer: --ready-file requires a non-empty path")
			}
			opts.readyFile = val
		case strings.HasPrefix(arg, "--wait-file="):
			val := strings.TrimSpace(strings.TrimPrefix(arg, "--wait-file="))
			if val == "" {
				return opts, errors.New("mup-segment-discovery-observer: --wait-file requires a non-empty path")
			}
			opts.waitFile = val
		case strings.HasPrefix(arg, "--keep="):
			val := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "--keep=")))
			if val == "" {
				return opts, errors.New("mup-segment-discovery-observer: --keep requires a non-empty NLRI hex string")
			}
			opts.keepHex = append(opts.keepHex, val)
		case strings.HasPrefix(arg, "--absent="):
			val := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "--absent=")))
			if val == "" {
				return opts, errors.New("mup-segment-discovery-observer: --absent requires a non-empty NLRI hex string")
			}
			opts.absentHex = append(opts.absentHex, val)
		case strings.HasPrefix(arg, "--expect-warning="):
			opts.expectWarning = strings.TrimPrefix(arg, "--expect-warning=")
		case strings.HasPrefix(arg, "--expect-doctor="):
			opts.expectDoctor = strings.TrimPrefix(arg, "--expect-doctor=")
		case strings.HasPrefix(arg, "--expect-error="):
			opts.expectError = strings.TrimPrefix(arg, "--expect-error=")
		default:
			return opts, fmt.Errorf("mup-segment-discovery-observer: unknown argument %q", arg)
		}
	}
	return opts, nil
}

// buildMUPLocatorFixtureSnapshot builds the deterministic SRv6 locator-originator
// snapshot published by plugin/mup-locator-origin-publisher over the EventBus.
func buildMUPLocatorFixtureSnapshot(source string, generation uint64, active bool) locatororigin.Snapshot {
	snap := locatororigin.Snapshot{
		Source:     source,
		Generation: generation,
		Active:     active,
	}
	if !active {
		return snap
	}
	origAddr := netip.MustParseAddr(mupFixtureOrigAddr)
	snap.Entries = []locatororigin.LocatorEntry{
		{
			Prefix:     netip.MustParsePrefix(mupFixtureLocatorCIDR),
			Known:      true,
			Originator: mupFixtureOriginatorID,
			Addresses:  []netip.Addr{origAddr},
			Prefixes: []netip.Prefix{
				netip.PrefixFrom(origAddr, 128),
				netip.MustParsePrefix(mupFixtureOrigPfx64),
			},
		},
	}
	return snap
}

// mupLocatorOriginPublisherDriver connects over TLS RPC as an external plugin
// declaring locator-origin:snapshot (send) and locator-origin:snapshot-request
// (receive), emits a known SRv6 locator-originator snapshot once all daemon
// plugins are ready, writes the readiness marker file for option=await-file,
// answers replay requests, and emits Active=false on shutdown.
//
// Usage: plugin/mup-locator-origin-publisher <ready-file>.
func mupLocatorOriginPublisherDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/mup-locator-origin-publisher <ready-file>")
	}
	readyFile := strings.TrimSpace(args[0])
	if readyFile == "" {
		return errors.New("usage: plugin/mup-locator-origin-publisher <ready-file>")
	}
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: locatororigin.Namespace + ":" + locatororigin.EventSnapshot},
		},
		Receives: []sdk.MessageReceiveDecl{
			{Message: locatororigin.Namespace + ":" + locatororigin.EventSnapshotRequest},
		},
	}
	var gen atomic.Uint64
	emitSnapshot := func(emitCtx context.Context, p *sdk.Plugin, active bool) error {
		source := busCurrentEndpointName(mupFixtureSourceName)
		snap := buildMUPLocatorFixtureSnapshot(source, gen.Add(1), active)
		payload, err := json.Marshal(&snap)
		if err != nil {
			return fmt.Errorf("marshal locator-origin snapshot: %w", err)
		}
		_, err = p.EmitEvent(emitCtx, locatororigin.Namespace, locatororigin.EventSnapshot, "", "", string(payload))
		if err != nil {
			return fmt.Errorf("emit locator-origin snapshot: %w", err)
		}
		return nil
	}

	setup := func(p *sdk.Plugin) error {
		p.SetEnvelope(true)
		p.OnEvent(func(text string) error {
			if isLocatorOriginReplayRequest(text) {
				replayCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				_ = emitSnapshot(replayCtx, p, true)
			}
			return nil
		})
		p.OnBye(func(_ string) error {
			byeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = emitSnapshot(byeCtx, p, false)
			return nil
		})
		return nil
	}

	return busRunPlugin(ctx, mupFixtureSourceName, reg, setup, func(scCtx context.Context, p *sdk.Plugin) error {
		if err := emitSnapshot(scCtx, p, true); err != nil {
			return err
		}
		if err := busWriteAtomicFile(readyFile, "ready\n"); err != nil {
			return fmt.Errorf("write locator ready file %q: %w", readyFile, err)
		}
		var tb textbuf.Buffer
		tb.WriteString("OK: published locator-origin snapshot for ")
		tb.WriteString(mupFixtureLocatorCIDR)
		tb.WriteString(" -> ")
		tb.WriteString(mupFixtureOriginatorID)
		tb.WriteByte('\n')
		_, _ = os.Stderr.Write(tb.Bytes())
		return nil
	})
}

func isLocatorOriginReplayRequest(text string) bool {
	if text == "" {
		return false
	}
	if text[0] != '{' {
		return false
	}
	env, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return false
	}
	return env.Namespace == locatororigin.Namespace && env.Event == locatororigin.EventSnapshotRequest
}

// mupSegmentDiscoveryObserverDriver waits until the route server replays the
// fence prefix to both clients, then verifies Adj-RIB-In retention/absence and
// optional report-bus warning/error/doctor entries before shutting down Ze.
func mupSegmentDiscoveryObserverDriver(ctx context.Context, args []string) error {
	opts, err := parseMUPObserverArgs(args)
	if err != nil {
		return err
	}
	events := make(chan string, 128)
	setup := func(plugin *sdk.Plugin) {
		plugin.SetEventFormat("parsed")
		plugin.OnEvent(func(event string) error {
			select {
			case events <- event:
			default:
				return errors.New("mup-segment-discovery-observer event queue full")
			}
			return nil
		})
	}
	reg := &sdk.Registration{
		Receives: bgpReceiveDecls([]string{eventUpdate}, nil),
	}
	return runPlugin13WithRegistration(ctx, "mup-sd-observer", reg, setup, func(scCtx context.Context, plugin *sdk.Plugin) error {
		if opts.readyFile != "" {
			if opts.waitFile != "" {
				if err := busPollFile(scCtx, opts.waitFile); err != nil {
					return fmt.Errorf("wait for %s before releasing peer: %w", opts.waitFile, err)
				}
			}
			if err := waitMUPRouteServerPeersReady(scCtx, plugin); err != nil {
				return err
			}
			if err := busWriteAtomicFile(opts.readyFile, "ready\n"); err != nil {
				return fmt.Errorf("write observer ready file %q: %w", opts.readyFile, err)
			}
		}
		if err := waitRouteServerReplay13(scCtx, plugin, events, opts.fence); err != nil {
			return err
		}
		if len(opts.keepHex) > 0 || len(opts.absentHex) > 0 {
			if err := verifyMUPAdjRIBIn(scCtx, plugin, opts.keepHex, opts.absentHex); err != nil {
				return err
			}
		}
		if opts.expectWarning != "" {
			if err := verifyMUPReportWarning(scCtx, plugin, opts.expectWarning); err != nil {
				return err
			}
		}
		if opts.expectDoctor != "" {
			if err := verifyMUPDoctorCode(scCtx, plugin, opts.expectDoctor); err != nil {
				return err
			}
		}
		if opts.expectError != "" {
			if err := verifyMUPReportError(scCtx, plugin, opts.expectError); err != nil {
				return err
			}
		}
		return nil
	}, true)
}

func waitMUPRouteServerPeersReady(ctx context.Context, plugin *sdk.Plugin) error {
	var lastErr error
	ok := Poll(ctx, 100, 100*time.Millisecond, func() bool {
		peerStatus := command13(ctx, plugin, "show bgp peer * detail")
		if err := requireDone13("show bgp peer * detail", peerStatus); err != nil {
			lastErr = err
			return false
		}
		eorCount := 0
		for _, value := range peerRows13(peerStatus) {
			row, ok := value.(map[string]any)
			if ok && number13(row["eor-sent"]) >= 1 {
				eorCount++
			}
		}
		if eorCount < 2 {
			lastErr = fmt.Errorf("only %d/2 peers have sent EOR", eorCount)
			return false
		}
		rsStatus := command13(ctx, plugin, "show bgp rs peers")
		if err := requireDone13("show bgp rs peers", rsStatus); err != nil {
			lastErr = err
			return false
		}
		if err := checkMUPRSPeersReady(rsStatus.object(), 2); err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !ok {
		if lastErr != nil {
			return fmt.Errorf("route server peers did not become ready: %w", lastErr)
		}
		return errors.New("route server peers did not become ready before timeout")
	}
	return nil
}

func checkMUPRSPeersReady(doc map[string]any, wantPeers int) error {
	if doc == nil {
		return errors.New("show bgp rs peers returned empty JSON object")
	}
	peers, _ := doc["peers"].([]any)
	ready := make(map[string]struct{}, len(peers))
	for _, raw := range peers {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		up, _ := row["up"].(bool)
		if !up {
			continue
		}
		fams, _ := row["families"].([]any)
		if len(fams) < 2 {
			continue
		}
		if addr, _ := row["address"].(string); addr != "" {
			ready[addr] = struct{}{}
		}
	}
	if len(ready) < wantPeers {
		return fmt.Errorf("bgp-rs has only %d/%d up peer(s) with negotiated families", len(ready), wantPeers)
	}
	return nil
}

func verifyMUPAdjRIBIn(ctx context.Context, plugin *sdk.Plugin, keepHex, absentHex []string) error {
	var lastErr error
	ok := Poll(ctx, 40, 100*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp adj-rib-in")
		if err := requireDone13("show bgp adj-rib-in", result); err != nil {
			lastErr = err
			return false
		}
		if err := checkMUPAdjRIBInData(result.object(), keepHex, absentHex); err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !ok {
		if lastErr != nil {
			return lastErr
		}
		return errors.New("show bgp adj-rib-in did not settle before timeout")
	}
	return nil
}

func checkMUPAdjRIBInData(doc map[string]any, keepHex, absentHex []string) error {
	if doc == nil {
		return errors.New("show bgp adj-rib-in returned empty JSON object")
	}
	adjMap, _ := doc["adj-rib-in"].(map[string]any)
	present := make(map[string]struct{})
	for _, rawList := range adjMap {
		rows, _ := rawList.([]any)
		for _, rawRow := range rows {
			row, ok := rawRow.(map[string]any)
			if !ok {
				continue
			}
			if nlri, _ := row["nlri-hex"].(string); nlri != "" {
				present[strings.ToLower(nlri)] = struct{}{}
			}
		}
	}
	for _, want := range keepHex {
		if _, found := present[strings.ToLower(want)]; !found {
			return fmt.Errorf("adj-rib-in missing expected nlri-hex %s (present=%v)", want, present)
		}
	}
	for _, forbid := range absentHex {
		if _, found := present[strings.ToLower(forbid)]; found {
			return fmt.Errorf("adj-rib-in unexpectedly retained withdrawn nlri-hex %s", forbid)
		}
	}
	return nil
}

func verifyMUPReportWarning(ctx context.Context, plugin *sdk.Plugin, wantCode string) error {
	var lastErr error
	ok := Poll(ctx, 40, 100*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show warnings")
		if err := requireDone13("show warnings", result); err != nil {
			lastErr = err
			return false
		}
		if err := checkMUPReportIssues(result.object(), "warnings", wantCode, mupFixtureUnresFix); err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !ok {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("show warnings never reported %s", wantCode)
	}
	return nil
}

func verifyMUPReportError(ctx context.Context, plugin *sdk.Plugin, wantCode string) error {
	var lastErr error
	ok := Poll(ctx, 40, 100*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show errors")
		if err := requireDone13("show errors", result); err != nil {
			lastErr = err
			return false
		}
		if err := checkMUPReportIssues(result.object(), "errors", wantCode, ""); err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !ok {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("show errors never reported %s", wantCode)
	}
	return nil
}

func checkMUPReportIssues(doc map[string]any, listKey, wantCode, wantFix string) error {
	if doc == nil {
		return fmt.Errorf("show %s returned empty JSON object", listKey)
	}
	rows, _ := doc[listKey].([]any)
	for _, raw := range rows {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["code"] != wantCode {
			continue
		}
		if entry["source"] != mupFixtureReportSource {
			return fmt.Errorf("show %s row for %s has source %v, want %q", listKey, wantCode, entry["source"], mupFixtureReportSource)
		}
		if wantFix != "" && entry["fix"] != wantFix {
			return fmt.Errorf("show %s row for %s has fix %v, want %q", listKey, wantCode, entry["fix"], wantFix)
		}
		return nil
	}
	return fmt.Errorf("show %s has no entry with code %q (count=%d)", listKey, wantCode, len(rows))
}

func verifyMUPDoctorCode(ctx context.Context, plugin *sdk.Plugin, wantCode string) error {
	var lastErr error
	ok := Poll(ctx, 40, 100*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show doctor")
		if err := requireDone13("show doctor", result); err != nil {
			lastErr = err
			return false
		}
		if err := checkMUPDoctorData(result.object(), wantCode); err != nil {
			lastErr = err
			return false
		}
		lastErr = nil
		return true
	})
	if !ok {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("show doctor never reported %s", wantCode)
	}
	return nil
}

func checkMUPDoctorData(doc map[string]any, wantCode string) error {
	if doc == nil {
		return errors.New("show doctor returned empty JSON object")
	}
	diagnostics, _ := doc["diagnostics"].([]any)
	for _, raw := range diagnostics {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["code"] == wantCode {
			return nil
		}
	}
	return fmt.Errorf("show doctor diagnostics missing code %q (count=%d)", wantCode, len(diagnostics))
}
