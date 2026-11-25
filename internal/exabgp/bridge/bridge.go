// Design: docs/architecture/core-design.md — startup protocol and bridge runtime
// Detail: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Detail: bridge_command.go — ExaBGP text command translation
// Detail: bridge_muxconn.go — MuxConn wire format parsing for post-startup I/O
//
// Package bridge provides runtime ExaBGP plugin protocol translation.
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// startupProtocol handles the 5-stage ZeBGP plugin registration protocol.
// This must be completed before the bridge can begin JSON translation.
//
// Stages:
//  1. Declaration (Bridge → ZeBGP): send family, encoding declarations, then "declare done"
//  2. Config (ZeBGP → Bridge): wait for "config done" (discard config lines)
//  3. Capability (Bridge → ZeBGP): send capability lines, then "capability done"
//  4. Registry (ZeBGP → Bridge): wait for "registry done" (discard registry lines)
//  5. Ready (Bridge → ZeBGP): send "ready"
type startupProtocol struct {
	output  io.Writer
	scanner *bufio.Scanner

	// Families to declare (ZeBGP format: "ipv4/unicast").
	// Converted to ZeBGP plugin protocol format: "ipv4 unicast".
	Families []string

	// RouteRefresh enables route-refresh capability (RFC 2918, code 2).
	RouteRefresh bool

	// AddPathMode sets ADD-PATH capability mode (RFC 7911, code 69).
	// Valid values: "receive", "send", "both", or "" (disabled).
	AddPathMode string
}

// newStartupProtocol creates a new startup protocol handler.
// The scanner should be reused after startup for JSON event processing
// to avoid losing buffered data.
func newStartupProtocol(scanner *bufio.Scanner, output io.Writer) *startupProtocol {
	return &startupProtocol{
		scanner:  scanner,
		output:   output,
		Families: []string{defaultFamily},
	}
}

// defaultFamily is the fallback when no families are configured.
const defaultFamily = "ipv4/unicast"

// Run executes the full 5-stage startup protocol.
func (sp *startupProtocol) Run() error {
	// Stage 1: Declaration
	sp.sendDeclarations()

	// Stage 2: Wait for config done
	if err := sp.waitForConfigDone(); err != nil {
		return fmt.Errorf("stage 2 (config): %w", err)
	}

	// Stage 3: Capability
	sp.sendCapabilityDone()

	// Stage 4: Wait for registry done
	if err := sp.waitForRegistryDone(); err != nil {
		return fmt.Errorf("stage 4 (registry): %w", err)
	}

	// Stage 5: Ready
	sp.SendReady()

	return nil
}

// sendDeclarations sends Stage 1 declarations.
func (sp *startupProtocol) sendDeclarations() {
	if sp.output == nil {
		return
	}

	// Use default family if none configured
	families := sp.Families
	if len(families) == 0 {
		families = []string{defaultFamily}
		slog.Debug("no families configured, using default", "family", defaultFamily)
	}

	// Declare families (convert "/" to " " for ZeBGP protocol)
	for _, fam := range families {
		// Convert "ipv4/unicast" → "ipv4 unicast"
		zebgpFamily := strings.ReplaceAll(fam, "/", " ")
		_, _ = fmt.Fprintf(sp.output, "declare family %s\n", zebgpFamily) //nolint:errcheck // output
	}

	// Declare encoding
	_, _ = fmt.Fprintln(sp.output, "declare encoding text") //nolint:errcheck // output

	// Declare receive types - ExaBGP plugins expect negotiated messages
	_, _ = fmt.Fprintln(sp.output, "declare receive negotiated") //nolint:errcheck // output

	// Done
	_, _ = fmt.Fprintln(sp.output, "declare done") //nolint:errcheck // output
}

// sendCapabilityDone sends Stage 3 capability lines and done marker.
//
// Capability format: "capability <enc> <code> [payload]".
// - Route-refresh (RFC 2918): code 2, no payload (0-length value per RFC).
// - ADD-PATH (RFC 7911): code 69, payload is hex-encoded AFI/SAFI/mode tuples.
func (sp *startupProtocol) sendCapabilityDone() {
	if sp.output == nil {
		return
	}

	// Route-refresh capability (RFC 2918, code 2).
	// RFC 2918: "The Capability Length of the Route Refresh Capability is zero."
	if sp.RouteRefresh {
		_, _ = fmt.Fprintln(sp.output, "capability hex 2") //nolint:errcheck // output
	}

	// ADD-PATH capability (RFC 7911, code 69)
	if sp.AddPathMode != "" {
		payload := sp.encodeAddPath()
		if payload != "" {
			_, _ = fmt.Fprintf(sp.output, "capability hex 69 %s\n", payload) //nolint:errcheck // output
		}
	}

	_, _ = fmt.Fprintln(sp.output, "capability done") //nolint:errcheck // output
}

// encodeAddPath encodes ADD-PATH capability payload for configured families.
//
// RFC 7911 Section 4: Each tuple is 4 octets: AFI (2) + SAFI (1) + Send/Receive (1).
// Mode values: 1=receive, 2=send, 3=both.
func (sp *startupProtocol) encodeAddPath() string {
	mode := sp.addPathModeValue()
	if mode == 0 {
		return ""
	}

	families := sp.Families
	if len(families) == 0 {
		families = []string{defaultFamily}
	}

	var result []byte
	for _, fam := range families {
		afi, safi := parseFamilyToAFISAFI(fam)
		if afi == 0 {
			slog.Warn("unknown family ignored for ADD-PATH", "family", fam)
			continue
		}
		// AFI (2 bytes big-endian) + SAFI (1 byte) + Mode (1 byte)
		result = append(result, byte(afi>>8), byte(afi), byte(safi), mode)
	}

	return fmt.Sprintf("%x", result)
}

// addPathModeValue converts mode string to RFC 7911 value.
func (sp *startupProtocol) addPathModeValue() byte {
	switch strings.ToLower(sp.AddPathMode) {
	case modeReceive:
		return 1
	case modeSend:
		return 2
	case modeBoth:
		return 3
	default:
		return 0
	}
}

// EncodeAddPathHex encodes ADD-PATH capability payload for the given families and mode.
// Standalone version of encodeAddPath for use outside StartupProtocol (e.g., SDK mode).
//
// RFC 7911 Section 4: Each tuple is 4 octets: AFI (2) + SAFI (1) + Send/Receive (1).
// Returns "" if mode is invalid or no valid families.
func EncodeAddPathHex(families []string, mode string) string {
	var m byte
	switch strings.ToLower(mode) {
	case modeReceive:
		m = 1
	case modeSend:
		m = 2
	case modeBoth:
		m = 3
	}
	// Unrecognized mode -- validated by cmdPlugin before reaching here.
	if m == 0 {
		return ""
	}

	if len(families) == 0 {
		families = []string{defaultFamily}
	}

	type afiSafi struct {
		afi  uint16
		safi uint16
	}
	pairs := make([]afiSafi, 0, len(families))
	for _, fam := range families {
		afi, safi := parseFamilyToAFISAFI(fam)
		if afi == 0 {
			slog.Warn("unknown family ignored for ADD-PATH", "family", fam)
			continue
		}
		pairs = append(pairs, afiSafi{afi: afi, safi: safi})
	}
	slices.SortFunc(pairs, func(a, b afiSafi) int {
		if a.afi != b.afi {
			if a.afi < b.afi {
				return -1
			}
			return 1
		}
		if a.safi < b.safi {
			return -1
		}
		if a.safi > b.safi {
			return 1
		}
		return 0
	})
	pairs = slices.Compact(pairs)

	result := make([]byte, 0, len(pairs)*4)
	for _, p := range pairs {
		result = append(result, byte(p.afi>>8), byte(p.afi), byte(p.safi), m)
	}

	return fmt.Sprintf("%x", result)
}

// parseFamilyToAFISAFI converts a family name (such as "ipv4/unicast" or
// "ipv4/mpls-label") to IANA AFI and SAFI values. Returns (0, 0) for
// invalid or unsupported families.
//
// It delegates to internal/core/family.LookupFamily first (after mapping
// legacy ExaBGP SAFI aliases to Ze's canonical names) so every registered Ze
// family works seamlessly with --family, and falls back to the static IANA
// table so standalone/unit callers without linked NLRI plugins parse the same
// families.
func parseFamilyToAFISAFI(name string) (afi, safi uint16) {
	lower := strings.ToLower(name)
	if fam, ok := family.LookupFamily(lower); ok {
		return uint16(fam.AFI), uint16(fam.SAFI)
	}

	afiName, safiName, ok := strings.Cut(lower, "/")
	if !ok {
		return 0, 0
	}
	if strings.Contains(safiName, "/") {
		return 0, 0
	}

	canonicalSAFI := normalizeBridgeFamilySAFI(safiName)
	if canonicalSAFI != safiName {
		if fam, found := family.LookupFamily(afiName + "/" + canonicalSAFI); found {
			return uint16(fam.AFI), uint16(fam.SAFI)
		}
	}

	return staticFamilyAFISAFI(afiName, canonicalSAFI)
}

// normalizeBridgeFamilySAFI maps legacy ExaBGP/CLI SAFI aliases to Ze's
// canonical SAFI spelling.
func normalizeBridgeFamilySAFI(safi string) string {
	switch safi {
	case "mpls", bridgeLabeledSAFI, "labeled-unicast":
		return zeLabeledSAFI
	case bridgeMVPNSAFI:
		return zeMVPNSAFI
	case "vpn":
		return bridgeVPNSAFI
	case "flowspec":
		return bridgeFlowSAFI
	case "flowspec-vpn":
		return bridgeFlowVPNSAFI
	default:
		return safi
	}
}

// staticFamilyAFISAFI resolves canonical Ze AFI/SAFI names using IANA numbers
// when the corresponding NLRI plugin is not linked into the process.
func staticFamilyAFISAFI(afiName, safiName string) (afi, safi uint16) {
	switch afiName {
	case bridgeAFIv4:
		afi = uint16(family.AFIIPv4)
	case bridgeAFIv6:
		afi = uint16(family.AFIIPv6)
	case "l2vpn":
		switch safiName {
		case "vpls":
			return uint16(family.AFIL2VPN), uint16(family.SAFIVPLS)
		case "evpn":
			return uint16(family.AFIL2VPN), uint16(family.SAFIEVPN)
		default:
			return 0, 0
		}
	case "bgp-ls":
		switch safiName {
		case "bgp-ls":
			return uint16(family.AFIBGPLS), uint16(family.SAFIBGPLinkState)
		case "bgp-ls-vpn":
			return uint16(family.AFIBGPLS), uint16(family.SAFIBGPLinkStateVPN)
		default:
			return 0, 0
		}
	default:
		return 0, 0
	}

	switch safiName {
	case bridgeUnicastSAFI:
		safi = uint16(family.SAFIUnicast)
	case bridgeMulticastSAFI:
		safi = uint16(family.SAFIMulticast)
	case zeLabeledSAFI:
		safi = uint16(family.SAFIMPLSLabel)
	case zeMVPNSAFI:
		safi = uint16(family.SAFIMVPN)
	case "sr-policy":
		safi = uint16(family.SAFISRPolicy)
	case bridgeMUPSAFI:
		safi = uint16(family.SAFIMUP)
	case bridgeVPNSAFI:
		safi = uint16(family.SAFIVPN)
	case bridgeRTCSAFI:
		if afi != uint16(family.AFIIPv4) {
			return 0, 0
		}
		safi = uint16(family.SAFIRTC)
	case bridgeFlowSAFI:
		safi = uint16(family.SAFIFlowSpec)
	case bridgeFlowVPNSAFI:
		safi = uint16(family.SAFIFlowSpecVPN)
	default:
		return 0, 0
	}

	return afi, safi
}

// ParseFamilyAFISAFI converts a family name to its IANA AFI and SAFI values,
// returning false when the family is unknown or invalid.
func ParseFamilyAFISAFI(name string) (afi uint16, safi uint8, ok bool) {
	a, s := parseFamilyToAFISAFI(name)
	if a == 0 {
		return 0, 0, false
	}
	if s == 0 {
		return 0, 0, false
	}
	return a, uint8(s), true
}

// ValidateFamily checks if a family string is supported.
// Uses parseFamilyToAFISAFI as single source of truth.
func ValidateFamily(name string) error {
	afi, safi := parseFamilyToAFISAFI(name)
	if afi == 0 {
		return fmt.Errorf("unsupported address family: %s", name)
	}
	if safi == 0 {
		return fmt.Errorf("unsupported address family: %s", name)
	}
	return nil
}

// SendReady sends Stage 5 ready signal.
func (sp *startupProtocol) SendReady() {
	if sp.output == nil {
		return
	}
	_, _ = fmt.Fprintln(sp.output, "ready") //nolint:errcheck // output
}

// waitForConfigDone waits for Stage 2 "config done" marker.
func (sp *startupProtocol) waitForConfigDone() error {
	return sp.waitForLine("config done")
}

// waitForRegistryDone waits for Stage 4 "registry done" marker.
func (sp *startupProtocol) waitForRegistryDone() error {
	return sp.waitForLine("registry done")
}

// waitForLine reads lines until the expected line is found.
func (sp *startupProtocol) waitForLine(expected string) error {
	if sp.scanner == nil {
		return nil
	}

	for sp.scanner.Scan() {
		line := sp.scanner.Text()
		if line == expected {
			return nil
		}
	}

	if err := sp.scanner.Err(); err != nil {
		return err
	}

	return fmt.Errorf("EOF before %q", expected)
}

// Bridge wraps an ExaBGP plugin process and translates between ZeBGP and ExaBGP formats.
type Bridge struct {
	// sessions holds each peer's negotiated block for its UPDATE lines.
	sessions Sessions

	pluginCmd []string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	running   bool
	mu        sync.Mutex

	// nextRequestID generates unique MuxConn request IDs for dispatch commands.
	nextRequestID atomic.Uint64

	// ack captures the exabgp.api.ack mode snapshot from os.Getenv at
	// construction; drives post-dispatch done/error emission on plugin stdin.
	ack AckMode

	// apiSetting is `exabgp.api.version`, read at each Start, and api the
	// version the script is held to (ScriptVersion). Encoder controls the
	// script-side event encoding (default EncoderJSON in NewBridge; SetEncoder
	// or setting Encoder before Start switches to EncoderText).
	apiSetting atomic.Uint32
	api        *ScriptVersion
	Encoder    Encoder

	// group is the script's `group start` buffer, emptied at each Start.
	group Group

	// Families to declare during startup (ZeBGP format: "ipv4/unicast")
	Families []string

	// RouteRefresh enables route-refresh capability (RFC 2918).
	RouteRefresh bool

	// AddPathMode sets ADD-PATH capability mode: "receive", "send", "both", or "" (disabled).
	AddPathMode string
}

// NewBridge creates a new bridge for the given ExaBGP plugin command.
func NewBridge(pluginCmd []string) *Bridge {
	b := &Bridge{
		pluginCmd: pluginCmd,
		Families:  []string{defaultFamily},
		ack:       NewAckMode(),
		Encoder:   EncoderJSON,
	}
	b.apiSetting.Store(uint32(APIVersionAuto))
	b.api = NewScriptVersion(pluginCmd[0], slog.Default(), b.Encoder, &b.apiSetting)
	return b
}

// SetEncoder configures the script-side event encoder (EncoderText or
// EncoderJSON) for standalone runs.
func (b *Bridge) SetEncoder(encoder Encoder) {
	if encoder == EncoderUnspecified {
		return
	}
	b.Encoder = encoder
	b.api = NewScriptVersion(b.pluginCmd[0], slog.Default(), b.Encoder, &b.apiSetting)
}

// Start starts the plugin subprocess with the given context.
func (b *Bridge) Start(ctx context.Context) error {
	setting, err := APIVersionFromEnv()
	if err != nil {
		return fmt.Errorf("exabgp-bridge: %w", err)
	}
	b.apiSetting.Store(uint32(setting))
	if b.Encoder != EncoderUnspecified {
		b.api = NewScriptVersion(b.pluginCmd[0], slog.Default(), b.Encoder, &b.apiSetting)
	}
	b.api.Reset()
	b.group = Group{}

	//nolint:gosec // User-provided plugin command is intentional.
	b.cmd = exec.CommandContext(ctx, b.pluginCmd[0], b.pluginCmd[1:]...)

	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	b.mu.Lock()
	b.running = true
	b.mu.Unlock()
	return nil
}

// Stop stops the plugin subprocess.
func (b *Bridge) Stop() {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()

	if b.cmd != nil && b.cmd.Process != nil && b.cmd.ProcessState == nil {
		_ = b.cmd.Process.Kill()
	}
}

// Run runs the bridge, translating between ZeBGP (stdin/stdout) and the plugin.
// It first completes the 5-stage startup protocol (raw text), then switches to
// MuxConn wire format for runtime I/O. When the script exits, Run writes no
// withdrawals and returns the script's exit error directly (E-6).
func (b *Bridge) Run(ctx context.Context) error {
	return b.runIO(ctx, os.Stdin, os.Stdout, os.Stderr)
}

// runIO is Run parameterized over its ZeBGP I/O streams so tests can drive the
// standalone runner end-to-end without replacing process stdio.
func (b *Bridge) runIO(ctx context.Context, zeIn io.Reader, zeOutWriter, zeErr io.Writer) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// Create a single scanner for zeIn - used for both startup and MuxConn events.
	// This prevents data loss from buffered reads.
	//
	// An answer line states its own width, so it is taken by that width rather
	// than by searching for a newline: a record payload may hold a raw newline,
	// and bufio.ScanLines would split the line on it and strip a trailing
	// carriage return besides (rpc.ScanAnswerLines).
	stdinScanner := bufio.NewScanner(zeIn)
	stdinScanner.Split(rpc.ScanAnswerLines)

	// Stage 1-5: Complete startup protocol with ZeBGP (raw text, no MuxConn)
	sp := newStartupProtocol(stdinScanner, zeOutWriter)
	sp.Families = b.Families
	sp.RouteRefresh = b.RouteRefresh
	sp.AddPathMode = b.AddPathMode
	if err := sp.Run(); err != nil {
		return fmt.Errorf("startup protocol: %w", err)
	}

	// Stage 6: Running - start plugin and begin MuxConn translation.
	// After stage 5, ze wraps the connection in MuxConn. All subsequent I/O
	// uses #<id> verb [payload]\n framing.
	if err := b.Start(runCtx); err != nil {
		return err
	}
	defer b.Stop()

	// Shared writer for ze stdout -- both goroutines write MuxConn lines.
	zeOut := &syncWriter{w: zeOutWriter}

	// Pending responses: command goroutine registers flush requests,
	// event goroutine signals when responses arrive.
	pending := newPendingResponses()

	var commands sync.WaitGroup
	var stderrWG sync.WaitGroup

	// ZeBGP stdin -> plugin stdin (parse MuxConn deliver-batch, translate events).
	// Uses the SAME scanner that completed startup to avoid losing buffered data.
	// Not awaited after b.cmd.Wait() returns: once the script has exited, the
	// standalone runner mirrors its exit status immediately (E-6) rather than
	// blocking on os.Stdin EOF. Closing b.stdin on EOF lets a running script see
	// the upstream close.
	go func() {
		b.zebgpToPluginWithScanner(runCtx, stdinScanner, b.stdin, zeOut, pending)
		_ = b.stdin.Close() //nolint:errcheck // best-effort EOF to child stdin
	}()

	// Plugin stdout -> ZeBGP stdout (translate commands, wrap in MuxConn dispatch + flush)
	commands.Go(func() {
		b.pluginToZebgp(runCtx, b.stdout, b.stdin, zeOut, pending)
	})

	// Plugin stderr -> ZeBGP stderr (pass through)
	stderrWG.Go(func() {
		if _, err := io.Copy(zeErr, b.stderr); err != nil {
			slog.Warn("stderr copy ended", "error", err)
		}
	})

	// Wait for the plugin's stdout and stderr pipes to reach EOF before Wait,
	// as required by os/exec.Cmd.StdoutPipe/StderrPipe, then reap the process.
	// Per E-6, standalone mode writes no withdrawals on script exit and returns
	// the child's exit error (preserving *exec.ExitError) directly.
	commands.Wait()
	stderrWG.Wait()
	err := b.cmd.Wait()
	b.Stop()
	cancelRun()
	_ = b.stdin.Close() //nolint:errcheck // child already exited
	return err
}

// zebgpToPluginWithScanner reads MuxConn lines from ze, extracts deliver-batch events,
// translates each from ZeBGP JSON to ExaBGP JSON, writes to the plugin, and sends
// back a MuxConn ok response to ze. It also routes "ok"/"error" responses to pending
// flush waiters via the pending parameter.
func (b *Bridge) zebgpToPluginWithScanner(ctx context.Context, scanner *bufio.Scanner, pluginW io.Writer, zeOut *syncWriter, pending *pendingResponses) {
	for scanner.Scan() {
		if ctx.Err() != nil {
			slog.Debug("zebgp->plugin: context canceled")
			return
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse MuxConn wire format: #<id> <verb> [<payload>]
		id, verb, payload, err := parseMuxLine(line)
		if err != nil {
			slog.Warn("zebgp->plugin: invalid MuxConn line",
				"error", err,
				"line", truncate(line, 100))
			continue
		}

		switch verb {
		case "ze-plugin-callback:deliver-batch":
			b.handleDeliverBatch(id, payload, pluginW, zeOut)

		case "ze-plugin-callback:deliver-event":
			b.handleDeliverEvent(id, payload, pluginW, zeOut)

		case "ok": // response to a flush we sent
			if !pending.signal(id, pendingResult{ok: true}) {
				slog.Debug("zebgp->plugin: orphaned ok", "id", id)
			}

		// A dispatch is answered by a head, its records and a terminator
		// (rpc.WriteRecordAnswer). The terminator releases the waiter, because
		// it is the one line that states the outcome (rpc.Verdict). Naming
		// every kind here is what keeps them off the unknown-verb arm below,
		// which would acknowledge a line ze never asked to have acknowledged.
		case rpc.AnswerKindHead, rpc.AnswerKindFault, rpc.AnswerKindRecord:
		case rpc.AnswerKindTerminator, rpc.AnswerKindWarning:
			if !pending.signal(id, terminatorResult(payload)) {
				slog.Debug("zebgp->plugin: answer nobody is waiting for", "id", id)
			}

		case "error", rpc.AnswerKindNotUnderstood:
			if !pending.signal(id, pendingResult{ok: false, errText: answerErrorText(verb, payload)}) {
				slog.Debug("zebgp->plugin: orphaned error", "id", id)
			}

		default: // unknown verb -- ack to avoid stalling ze
			slog.Warn("zebgp->plugin: unknown MuxConn verb", "verb", verb, "id", id)
			zeOut.Fprintln(formatMuxOK(id))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("zebgp->plugin: scanner error", "error", err)
	}
}

// answerErrorText is the reason a failing line states, in the form an ExaBGP
// plugin reads it. A not-understood answer states its reason as a positional
// field, so the tail is DECODED rather than echoed: the operator would otherwise
// read the counted lengths in front of the message as part of it.
//
// An `error` response carries JSON, which the ack layer already forwards whole,
// and a tail this build cannot read is forwarded rather than dropped, because a
// failure nobody can read still has to reach the plugin as a failure.
func answerErrorText(verb, payload string) string {
	if verb != rpc.AnswerKindNotUnderstood {
		return payload
	}
	tail, err := rpc.ParseAnswerTail(verb, []byte(payload))
	if err != nil {
		return payload
	}
	if tail.Message != "" {
		return tail.Message
	}
	return tail.Code
}

// handleDeliverBatch processes a deliver-batch request: extracts events, translates
// each from ZeBGP JSON to ExaBGP JSON, writes to the plugin, sends ok to ze.
func (b *Bridge) handleDeliverBatch(id uint64, payload string, pluginW io.Writer, zeOut *syncWriter) {
	events, err := extractBatchEvents(payload)
	if err != nil {
		slog.Warn("zebgp->plugin: failed to extract batch events",
			"error", err,
			"id", id)
		zeOut.Fprintln(formatMuxOK(id))
		return
	}

	for _, eventStr := range events {
		b.translateAndForward(eventStr, pluginW)
	}

	zeOut.Fprintln(formatMuxOK(id))
}

// handleDeliverEvent processes a single deliver-event request.
func (b *Bridge) handleDeliverEvent(id uint64, payload string, pluginW io.Writer, zeOut *syncWriter) {
	var input struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal([]byte(payload), &input); err != nil {
		slog.Warn("zebgp->plugin: failed to parse deliver-event",
			"error", err,
			"id", id)
		zeOut.Fprintln(formatMuxOK(id))
		return
	}

	b.translateAndForward(input.Event, pluginW)
	zeOut.Fprintln(formatMuxOK(id))
}

// translateAndForward parses a ZeBGP JSON event string, renders it in the
// format the script reads now (ScriptVersion.Format), and writes it to the
// plugin's stdin.
func (b *Bridge) translateAndForward(eventStr string, pluginW io.Writer) {
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(eventStr), &zebgp); err != nil {
		slog.Warn("zebgp->plugin: invalid JSON event",
			"error", err,
			"event", truncate(eventStr, 100))
		return
	}

	read := ReadEvent(zebgp)
	if read.Kind == "" {
		// An envelope that carries no BGP message: neither ExaBGP format has
		// anything to write for one.
		slog.Debug("zebgp->plugin: event carries no BGP message", "event", truncate(eventStr, 100))
		return
	}
	// This runner has no per-peer feed: its one script is written every event
	// of every peer, the `negotiated` event among them (sendDeclarations), so
	// every peer grants it `negotiated` and every message line carries the
	// block, whatever the script's API version (Event.ForGrant).
	b.sessions.Note(&read)
	read = read.ForGrant(true)
	line := read.Line(b.api.Format(), slog.Default())
	if len(line) == 0 {
		return
	}
	if _, err := pluginW.Write(line); err != nil {
		slog.Warn("zebgp->plugin: write to plugin failed", "error", err)
	}
}

// pluginToZebgp reads ExaBGP text commands from the plugin, cuts them into
// batches, nets each batch, dispatches what survives over MuxConn, flushes each
// selector the batch reached, and answers every line of the batch.
//
// One read is one batch, and the reader does no wire work: it hands each batch
// to the dispatcher goroutine and returns to the pipe at once. A reader that
// waited for the flush would let the script's NEXT write join the batch it was
// still dispatching, and the netting would then cancel two commands the script
// wrote in two separate writes (bridge_batch.go).
//
// When exabgp.api.ack is enabled (the default), each line is answered with
// `done\n`, or its reason and `error\n` (AckMode.WriteError), after the batch's
// flush.
func (b *Bridge) pluginToZebgp(ctx context.Context, r io.Reader, pluginW io.Writer, zeOut *syncWriter, pending *pendingResponses) {
	batches := make(chan []string, BatchQueueDepth)

	var dispatching sync.WaitGroup
	dispatching.Go(func() {
		for lines := range batches {
			b.dispatchBatch(ctx, lines, pluginW, zeOut, pending)
		}
	})

	defer func() {
		close(batches)
		dispatching.Wait()
	}()

	reader := NewBatchReader(r)
	for {
		if ctx.Err() != nil {
			slog.Debug("plugin->zebgp: context canceled")
			return
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		lines, err := reader.Next()
		if len(lines) > 0 {
			select {
			case batches <- lines:
			case <-ctx.Done():
				return
			}
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			slog.Warn("plugin->zebgp: read error", "error", err)
		}
		return
	}
}

// dispatchBatch reads one batch of the script's lines through the steps every
// runner shares (ScriptLines.Batch), reaching ze over MuxConn.
//
// It runs OFF the read path, on the goroutine pluginToZebgp owns.
func (b *Bridge) dispatchBatch(ctx context.Context, lines []string, pluginW io.Writer, zeOut *syncWriter, pending *pendingResponses) {
	ScriptLines{
		Name:       b.pluginCmd[0],
		Log:        slog.Default(),
		Translator: Translator{Families: b.Families},
		API:        b.api,
		Ack:        &b.ack,
		Group:      &b.group,
	}.Batch(ctx, muxRunner{b: b, zeOut: zeOut, pending: pending}, pluginW, lines)
}

// muxRunner is how the standalone runner reaches ze: one MuxConn request per
// command, each awaited for ze's answer.
type muxRunner struct {
	b       *Bridge
	zeOut   *syncWriter
	pending *pendingResponses
}

// muxDispatchTimeout and muxFlushTimeout bound the wait for ze's answer, so
// the bridge does not stall indefinitely if ze drops the response line.
const (
	muxDispatchTimeout = 30 * time.Second
	muxFlushTimeout    = 30 * time.Second
)

// errMuxDispatchTimeout is a dispatch ze did not answer in time.
var errMuxDispatchTimeout = errors.New("ze dispatch timeout")

// Dispatch sends ze one command and waits for its answer.
func (r muxRunner) Dispatch(ctx context.Context, command string) error {
	reqID := r.b.nextRequestID.Add(1)
	ackCh := r.pending.register(reqID)
	r.zeOut.Fprintln(formatDispatchRequest(reqID, command))

	ackCtx, cancel := context.WithTimeout(ctx, muxDispatchTimeout)
	result, err := r.pending.wait(ackCtx, reqID, ackCh)
	cancel()
	if err != nil {
		slog.Warn("plugin->zebgp: dispatch ack wait error", "error", err, "id", reqID)
		return errMuxDispatchTimeout
	}
	if !result.ok {
		return errors.New(result.errText)
	}
	return nil
}

// errMuxFlushTimeout is a flush ze did not answer within muxFlushTimeout.
var errMuxFlushTimeout = errors.New("ze flush timeout")

// Flush drains the forward pool of the peers selector names, and answers ze's
// refusal or the missing answer. A flush that did not happen is a barrier
// that did not hold, so the caller owes the script an error for it.
func (r muxRunner) Flush(ctx context.Context, selector string) error {
	flushID := r.b.nextRequestID.Add(1)
	flushCh := r.pending.register(flushID)
	r.zeOut.Fprintln(formatFlushRequest(flushID, selector))

	flushCtx, cancel := context.WithTimeout(ctx, muxFlushTimeout)
	result, err := r.pending.wait(flushCtx, flushID, flushCh)
	cancel()
	if err != nil {
		slog.Warn("plugin->zebgp: flush wait error", "error", err, "peer", selector)
		return errMuxFlushTimeout
	}
	if !result.ok {
		return errors.New(result.errText)
	}
	return nil
}

// Status refuses a `status` line. The MuxConn dispatch answer carries ok or an
// error, never a payload, so this runner cannot read the peers' states the
// status answer lists, and it refuses by name rather than answer with a peer
// list that is not ze's.
func (r muxRunner) Status(context.Context, bool) ([]byte, BatchAnswer) {
	slog.Warn("plugin->zebgp: line refused", "error", errStatusStandalone)
	return nil, BatchAnswer{Failed: true, Error: errStatusStandalone.Error()}
}

// truncate returns s truncated to maxLen runes with "..." suffix if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
