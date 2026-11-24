// Design: docs/architecture/api/architecture.md -- Phase 7 implied display, delivery origin, and lifecycle reception functional test fixtures.
// Overview: register_bus_implied_fixtures.go -- driver registrations for Phase 7 bus display tests.
// Related: bus_send_fixtures.go -- busRunPlugin, busPollFile, busWriteAtomicFile, busSIGHUPReloadDriver.
// Related: internal/component/config/implied.go -- ImpliedEnvelope, ImpliedReport, ImpliedFilterMember.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	impliedModeS43Running        = "s43-running"
	impliedModeS43NoRole         = "s43-no-role"
	impliedModeOfflineOK         = "offline-ok"
	impliedModeOfflineRefusal    = "offline-refusal"
	impliedModeConfigShowNotice  = "config-show-notice"
	impliedModeRoundTripExtract  = "round-trip-extract"
	impliedModeRoundTripVerify   = "round-trip-verify"
	impliedModeRSDisplay         = "rs-display"
	impliedModeDeliveryOrigins   = "delivery-origins"
	impliedModeLifecycleDelivery = "lifecycle-delivery"
	impliedModeFileContains      = "file-contains"
	impliedModePollFileContains  = "poll-file-contains"
	impliedModeFileNotContains   = "file-not-contains"
	impliedModeSnapshotFile      = "snapshot-file"
	impliedModePollFileGrew      = "poll-file-grew-needle"
	impliedModeFileUnchanged     = "file-unchanged"
	impliedModeMergeConfig       = "merge-config"

	impliedPeerEdge1 = "/bgp/peer/edge1"
	impliedPeerLab   = "/bgp/peer/lab"
	impliedPeerT1    = "/bgp/peer/t1"
	impliedPeerT2    = "/bgp/peer/t2"
	impliedPathRS    = "/bgp/route-server"
	impliedPathCol   = "/bus/endpoint/collector"
	impliedPathVal   = "/bus/endpoint/val"
	impliedPathProbe = "/bus/endpoint/probe"

	impliedFilterLoop    = "loop-detection"
	impliedFilterOTCIng  = "bgp-role:otc-ingress"
	impliedFilterOTCEgr  = "bgp-role:otc-egress"
	impliedFilterNHAuto  = "next-hop:auto"
	impliedDirImport     = "import"
	impliedDirExport     = "export"
	impliedPosHead       = "head"
	impliedPosTail       = "tail"
	impliedNoticeNeedle  = "implied item(s) active"
	impliedByHeader      = "# implied by:"
	impliedInlineMarker  = "# implied:"
	impliedEdgePresent   = "present"
	impliedEdgeAbsent    = "absent"
	impliedOpenValPlugin = "val"

	cliCmdShowRunningImplied     = "show config running | implied"
	cliCmdShowRunningImpliedJSON = "show config running | implied | json"
	cliCmdShowEventDeliveryJSON  = "show event delivery | json"
	impliedRegCollectorHeader    = "# implied by: registration of collector"
)

type busDeliveryEdgeView struct {
	SenderPath       string              `json:"sender-path,omitempty"`
	SenderEndpoint   string              `json:"sender-endpoint,omitempty"`
	SenderGroup      string              `json:"sender-group,omitempty"`
	ReceiverPath     string              `json:"receiver-path,omitempty"`
	ReceiverEndpoint string              `json:"receiver-endpoint,omitempty"`
	ReceiverGroup    string              `json:"receiver-group,omitempty"`
	Message          string              `json:"message"`
	Origin           string              `json:"origin"`
	Source           string              `json:"source,omitempty"`
	Destinations     []string            `json:"destinations,omitempty"`
	Tags             map[string][]string `json:"tags,omitempty"`
}

type busDeliveryResponseView struct {
	Edges []busDeliveryEdgeView `json:"edges"`
	Count int                   `json:"count"`
}

// busImpliedProbeDriver implements an external endpoint used by AC-24 offline
// query-mode tests (bus-implied-offline.ci). Under query mode it answers its
// Stage 1 declarations via sdk.RunOrDeclare without touching sideEffectFile;
// if its run loop is ever started, it writes sideEffectFile immediately so the
// test detects any unintended activation.
// Usage: plugin/bus-implied-probe <side-effect-file>.
func busImpliedProbeDriver(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-implied-probe <side-effect-file>")
	}
	sideEffectFile := args[0]
	reg := &sdk.Registration{
		Sends: []sdk.MessageSendDecl{
			{Message: busMsgAcmeAlert},
		},
		Receives: []sdk.MessageReceiveDecl{
			{Message: busMsgBGPState},
			{Message: busMsgBGPUpdate},
		},
	}
	code := sdk.RunOrDeclare(reg, func() int {
		if err := busWriteAtomicFile(sideEffectFile, "run-loop-started\n"); err != nil {
			var tb textbuf.Buffer
			_ = tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // best-effort diagnostic on failure exit
			return 1
		}
		return 0
	})
	if code != 0 {
		return fmt.Errorf("bus-implied-probe exited %d", code)
	}
	return nil
}

// busOpenValidatorDriver implements an external endpoint declaring
// WantsValidateOpen with no peer attachment for S32 / AC-39
// (bus-lifecycle-deliveries.ci). It subscribes to bgp state events after
// startup and appends every delivered event line to outFile, optionally
// writing doneFile once an event contains doneNeedle.
// Usage: plugin/bus-open-validator <out-file> [<done-needle> <done-file>].
func busOpenValidatorDriver(ctx context.Context, args []string) error {
	if len(args) != 1 && len(args) != 3 {
		return errors.New("usage: plugin/bus-open-validator <out-file> [<done-needle> <done-file>]")
	}
	outFile := args[0]
	doneNeedle, doneFile := "", ""
	if len(args) == 3 {
		doneNeedle, doneFile = args[1], args[2]
	}

	reg := &sdk.Registration{
		WantsValidateOpen: true,
	}
	var mu sync.Mutex
	var recorded []string
	return busRunPlugin(ctx, impliedOpenValPlugin, reg, func(p *sdk.Plugin) error {
		p.OnValidateOpen(func(_ *rpc.ValidateOpenInput) *rpc.ValidateOpenOutput {
			return &rpc.ValidateOpenOutput{Accept: true}
		})
		p.OnEvent(func(event string) error {
			mu.Lock()
			recorded = append(recorded, strings.TrimSpace(event))
			var body textbuf.Buffer
			for _, line := range recorded {
				body.Str(line).Byte('\n')
			}
			snapshot := body.String()
			mu.Unlock()

			if err := busWriteAtomicFile(outFile, snapshot); err != nil {
				return err
			}
			if doneFile != "" && strings.Contains(event, doneNeedle) {
				if err := busWriteAtomicFile(doneFile, "done\n"); err != nil {
					return err
				}
			}
			return nil
		})
		return nil
	}, func(_ context.Context, _ *sdk.Plugin) error {
		var tb textbuf.Buffer
		return tb.Str("OK: val subscribed to bgp:state lifecycle events\n").StdErr()
	})
}

// busAssertImpliedDriver dispatches structural assertions for Phase 7 display,
// offline query-mode, round-trip, route-server display, delivery origin, and
// lifecycle delivery functional tests.
// Usage: plugin/bus-assert-implied <mode> [args...].
func busAssertImpliedDriver(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: plugin/bus-assert-implied <mode> [args...]")
	}
	mode := args[0]
	rest := args[1:]
	switch mode {
	case impliedModeS43Running:
		return busAssertS43Running(ctx, rest)
	case impliedModeS43NoRole:
		return busAssertS43NoRole(ctx, rest)
	case impliedModeOfflineOK:
		return busAssertOfflineOK(ctx, rest)
	case impliedModeOfflineRefusal:
		return busAssertOfflineRefusal(ctx, rest)
	case impliedModeConfigShowNotice:
		return busAssertConfigShowNotice(ctx, rest)
	case impliedModeRoundTripExtract:
		return busAssertRoundTripExtract(ctx, rest)
	case impliedModeRoundTripVerify:
		return busAssertRoundTripVerify(ctx, rest)
	case impliedModeRSDisplay:
		return busAssertRSDisplay(ctx, rest)
	case impliedModeDeliveryOrigins:
		return busAssertDeliveryOrigins(ctx, rest)
	case impliedModeLifecycleDelivery:
		return busAssertLifecycleDelivery(ctx, rest)
	case impliedModeFileContains:
		return busAssertFileNeedle(rest, true)
	case impliedModePollFileContains:
		return busPollFileContains(ctx, rest)
	case impliedModeFileNotContains:
		return busAssertFileNeedle(rest, false)
	case impliedModeSnapshotFile:
		return busSnapshotFile(rest)
	case impliedModePollFileGrew:
		return busPollFileGrewNeedle(ctx, rest)
	case impliedModeFileUnchanged:
		return busAssertFileUnchanged(rest)
	case impliedModeMergeConfig:
		return busMergeConfigFiles(rest)
	default:
		return fmt.Errorf("unknown bus-assert-implied mode %q", mode)
	}
}

func runZeCLICaptured(ctx context.Context, cliCmd string) (code int, stdout, stderr string, err error) {
	return runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", cliCmd)
}

func assertNoDoubleHashColumnZero(label, text string) error {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "##") {
			return fmt.Errorf("%s emitted ## at column 0: %q", label, line)
		}
	}
	return nil
}

func assertNoHumanNotice(label, text string) error {
	if strings.Contains(text, impliedNoticeNeedle) {
		return fmt.Errorf("%s leaked human notice (%q) into machine output:\n%s", label, impliedNoticeNeedle, text)
	}
	if strings.Contains(text, "# note:") {
		return fmt.Errorf("%s leaked # note: footer into machine output:\n%s", label, text)
	}
	return nil
}

func filterMembersByTargetDir(members []config.ImpliedFilterMember, target, dir string) []config.ImpliedFilterMember {
	var out []config.ImpliedFilterMember
	for _, m := range members {
		if m.TargetPath == target {
			if m.Direction == dir {
				out = append(out, m)
			}
		}
	}
	return out
}

func busAssertS43Running(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/bus-assert-implied s43-running")
	}

	// 1. Text output: `show config running | implied`.
	code, textOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImplied)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImplied, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImplied, code, strings.TrimSpace(stderr))
	}
	if err := assertNoDoubleHashColumnZero(cliCmdShowRunningImplied, textOut); err != nil {
		return err
	}
	for _, needle := range []string{
		impliedByHeader,
		impliedInlineMarker,
		impliedFilterLoop,
		impliedFilterOTCIng,
		impliedFilterOTCEgr,
		impliedFilterNHAuto,
		"replaced registered default(s)",
	} {
		if !strings.Contains(textOut, needle) {
			return fmt.Errorf("%s missing %q:\n%s", cliCmdShowRunningImplied, needle, textOut)
		}
	}

	// 2. JSON envelope: `show config running | implied | json`.
	code, jsonOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImpliedJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImpliedJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImpliedJSON, code, strings.TrimSpace(stderr))
	}
	if err := assertNoHumanNotice(cliCmdShowRunningImpliedJSON, jsonOut); err != nil {
		return err
	}
	var envPayload config.ImpliedEnvelope
	if err := json.Unmarshal([]byte(jsonOut), &envPayload); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope JSON: %w\n%s", err, jsonOut)
	}
	cfgMap, ok := envPayload.Config.(map[string]any)
	if !ok {
		return fmt.Errorf("ImpliedEnvelope.config is not a JSON object: %T", envPayload.Config)
	}
	if _, leaked := cfgMap["implied"]; leaked {
		return errors.New("ImpliedEnvelope.config must be pure written config without nested implied key")
	}
	if _, hasBGP := cfgMap["bgp"]; !hasBGP {
		return errors.New("ImpliedEnvelope.config missing bgp section")
	}

	// Verify S43 filter chain members on all 4 peers.
	if err := assertS43FiltersWithRole(envPayload.Implied.Filters); err != nil {
		return err
	}

	// Verify auto-loaded endpoint (bgp-rib or bgp-role) is reported.
	if len(envPayload.Implied.Endpoints) == 0 {
		return fmt.Errorf("implied.endpoints is empty; expected auto-loaded endpoints:\n%s", jsonOut)
	}

	// Verify written bus override on t2 reports origin "written" and replaced-defaults.
	if err := assertReplacedDefaultForPeer(envPayload.Implied.Participants, impliedPeerT2, busMsgBGPUpdate); err != nil {
		return err
	}

	// 3. YAML envelope: `show config running | implied | yaml`.
	code, yamlOut, stderr, err := runZeCLICaptured(ctx, "show config running | implied | yaml")
	if err != nil {
		return fmt.Errorf("run show config running | implied | yaml: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show config running | implied | yaml exited %d: %s", code, strings.TrimSpace(stderr))
	}
	if err := assertNoHumanNotice("show config running | implied | yaml", yamlOut); err != nil {
		return err
	}
	var yamlDoc map[string]any
	if err := yaml.Unmarshal([]byte(yamlOut), &yamlDoc); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope YAML: %w\n%s", err, yamlOut)
	}
	if _, hasCfg := yamlDoc["config"]; !hasCfg {
		return fmt.Errorf("YAML envelope missing config key:\n%s", yamlOut)
	}
	if _, hasImp := yamlDoc["implied"]; !hasImp {
		return fmt.Errorf("YAML envelope missing implied key:\n%s", yamlOut)
	}

	// 4. Pipe composition: `show config running | implied | match otc-egress`.
	code, matchOut, stderr, err := runZeCLICaptured(ctx, "show config running | implied | match otc-egress")
	if err != nil {
		return fmt.Errorf("run show config running | implied | match otc-egress: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show config running | implied | match otc-egress exited %d: %s", code, strings.TrimSpace(stderr))
	}
	matchedLines := 0
	for line := range strings.SplitSeq(matchOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "otc-egress") {
			return fmt.Errorf("| implied | match otc-egress emitted non-matching line: %q", trimmed)
		}
		matchedLines++
	}
	if matchedLines == 0 {
		return fmt.Errorf("| implied | match otc-egress returned zero lines:\n%s", matchOut)
	}

	// 5. Byte-pure written JSON without `| implied`: `show config running | json`.
	code, pureOut, stderr, err := runZeCLICaptured(ctx, "show config running | json")
	if err != nil {
		return fmt.Errorf("run show config running | json: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("show config running | json exited %d: %s", code, strings.TrimSpace(stderr))
	}
	if err := assertNoHumanNotice("show config running | json", pureOut); err != nil {
		return err
	}
	var pureDoc map[string]any
	if err := json.Unmarshal([]byte(pureOut), &pureDoc); err != nil {
		return fmt.Errorf("decode pure running JSON: %w\n%s", err, pureOut)
	}
	if _, hasImp := pureDoc["implied"]; hasImp {
		return fmt.Errorf("show config running | json leaked implied key:\n%s", pureOut)
	}
	if _, hasBGP := pureDoc["bgp"]; !hasBGP {
		return fmt.Errorf("show config running | json missing bgp section:\n%s", pureOut)
	}

	var tb textbuf.Buffer
	return tb.Str("OK: s43-running verified text, json, yaml, match, and pure json\n").StdOut()
}

func findFilterMemberByRef(members []config.ImpliedFilterMember, ref string) (config.ImpliedFilterMember, bool) {
	for _, m := range members {
		if m.Ref == ref {
			return m, true
		}
	}
	return config.ImpliedFilterMember{}, false
}

func assertS43FiltersWithRole(filters []config.ImpliedFilterMember) error {
	allPeers := []string{impliedPeerEdge1, impliedPeerLab, impliedPeerT1, impliedPeerT2}
	for _, peer := range allPeers {
		imp := filterMembersByTargetDir(filters, peer, impliedDirImport)
		if len(imp) == 0 {
			return fmt.Errorf("%s import has zero implied filters, want head %s", peer, impliedFilterLoop)
		}
		if imp[0].Ref != impliedFilterLoop {
			return fmt.Errorf("%s import[0] ref = %q, want %s", peer, imp[0].Ref, impliedFilterLoop)
		}
		if imp[0].Position != impliedPosHead {
			return fmt.Errorf("%s import[0] position = %q, want %s", peer, imp[0].Position, impliedPosHead)
		}
		otcIn, hasOTCIn := findFilterMemberByRef(imp, impliedFilterOTCIng)
		if peer == impliedPeerEdge1 {
			if !hasOTCIn {
				return fmt.Errorf("edge1 import missing %s: %+v", impliedFilterOTCIng, imp)
			}
			if otcIn.Position != impliedPosHead {
				return fmt.Errorf("edge1 import %s position = %q, want %s", impliedFilterOTCIng, otcIn.Position, impliedPosHead)
			}
			if !otcIn.Required {
				return fmt.Errorf("edge1 import %s must be required", impliedFilterOTCIng)
			}
			if !strings.Contains(otcIn.ImpliedBy, "edge1") {
				return fmt.Errorf("edge1 import %s implied-by %q must cite edge1", impliedFilterOTCIng, otcIn.ImpliedBy)
			}
		} else if hasOTCIn {
			return fmt.Errorf("%s import must not carry %s, got %+v", peer, impliedFilterOTCIng, imp)
		}

		exp := filterMembersByTargetDir(filters, peer, impliedDirExport)
		otcOut, hasOTCOut := findFilterMemberByRef(exp, impliedFilterOTCEgr)
		if !hasOTCOut {
			return fmt.Errorf("%s export missing %s: %+v", peer, impliedFilterOTCEgr, exp)
		}
		if otcOut.Position != impliedPosHead {
			return fmt.Errorf("%s export %s position = %q, want %s", peer, impliedFilterOTCEgr, otcOut.Position, impliedPosHead)
		}
		if !otcOut.Required {
			return fmt.Errorf("%s export %s must be required", peer, impliedFilterOTCEgr)
		}
		if !strings.Contains(otcOut.ImpliedBy, "edge1") {
			return fmt.Errorf("%s export %s implied-by %q must cite edge1", peer, impliedFilterOTCEgr, otcOut.ImpliedBy)
		}
		nhAuto, hasNHAuto := findFilterMemberByRef(exp, impliedFilterNHAuto)
		switch peer {
		case impliedPeerEdge1, impliedPeerT2:
			if !hasNHAuto {
				return fmt.Errorf("%s export missing %s: %+v", peer, impliedFilterNHAuto, exp)
			}
			if nhAuto.Position != impliedPosTail {
				return fmt.Errorf("%s export %s position = %q, want %s", peer, impliedFilterNHAuto, nhAuto.Position, impliedPosTail)
			}
		case impliedPeerLab, impliedPeerT1:
			if hasNHAuto {
				return fmt.Errorf("%s export must not carry implied %s, got %+v", peer, impliedFilterNHAuto, exp)
			}
		}
	}
	return nil
}

func assertReplacedDefaultForPeer(participants []config.ImpliedParticipant, peerPath, message string) error {
	for i := range participants {
		p := &participants[i]
		if p.Path != peerPath {
			continue
		}
		for j := range p.Send {
			r := &p.Send[j]
			if r.Message != message {
				continue
			}
			if r.Origin != config.ImpliedOriginWritten {
				return fmt.Errorf("%s send %s origin = %q, want %q", peerPath, message, r.Origin, config.ImpliedOriginWritten)
			}
			if len(r.ReplacedDefaults) == 0 {
				return fmt.Errorf("%s send %s missing replaced-defaults", peerPath, message)
			}
			return nil
		}
	}
	return fmt.Errorf("participant %s send rule %s not found in implied.participants", peerPath, message)
}

func busAssertS43NoRole(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/bus-assert-implied s43-no-role")
	}
	code, jsonOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImpliedJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImpliedJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImpliedJSON, code, strings.TrimSpace(stderr))
	}
	var envPayload config.ImpliedEnvelope
	if err := json.Unmarshal([]byte(jsonOut), &envPayload); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope JSON: %w\n%s", err, jsonOut)
	}
	for _, m := range envPayload.Implied.Filters {
		if m.Ref == impliedFilterOTCIng {
			return fmt.Errorf("role filter %s must disappear after removing role from edge1: %+v", m.Ref, m)
		}
		if m.Ref == impliedFilterOTCEgr {
			return fmt.Errorf("role filter %s must disappear after removing role from edge1: %+v", m.Ref, m)
		}
	}
	allPeers := []string{impliedPeerEdge1, impliedPeerLab, impliedPeerT1, impliedPeerT2}
	for _, peer := range allPeers {
		imp := filterMembersByTargetDir(envPayload.Implied.Filters, peer, impliedDirImport)
		if len(imp) == 0 {
			return fmt.Errorf("%s import missing head %s: %+v", peer, impliedFilterLoop, imp)
		}
		if imp[0].Ref != impliedFilterLoop {
			return fmt.Errorf("%s import missing head %s: %+v", peer, impliedFilterLoop, imp)
		}
		exp := filterMembersByTargetDir(envPayload.Implied.Filters, peer, impliedDirExport)
		nhAuto, hasNHAuto := findFilterMemberByRef(exp, impliedFilterNHAuto)
		switch peer {
		case impliedPeerEdge1, impliedPeerT2:
			if !hasNHAuto {
				return fmt.Errorf("%s export missing tail %s after role removal: %+v", peer, impliedFilterNHAuto, exp)
			}
			if nhAuto.Position != impliedPosTail {
				return fmt.Errorf("%s export missing tail %s after role removal: %+v", peer, impliedFilterNHAuto, exp)
			}
		case impliedPeerLab, impliedPeerT1:
			if hasNHAuto {
				return fmt.Errorf("%s export must not carry implied %s after role removal, got %+v", peer, impliedFilterNHAuto, exp)
			}
		}
	}
	var tb textbuf.Buffer
	return tb.Str("OK: s43-no-role verified role filters gone and built-in filters retained\n").StdOut()
}

func busAssertOfflineOK(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-assert-implied offline-ok <config-file> <side-effect-file>")
	}
	cfgFile, sideEffectFile := args[0], args[1]

	var srcCmd, dumpCmd textbuf.Buffer
	commands := []string{
		srcCmd.Str("show config source file ").Str(cfgFile).Str(" | implied | json").String(),
		dumpCmd.Str("show config dump ").Str(cfgFile).Str(" | implied | json").String(),
	}
	for _, cmdStr := range commands {
		code, stdout, stderr, err := runZeCLICaptured(ctx, cmdStr)
		if err != nil {
			return fmt.Errorf("run %q: %w", cmdStr, err)
		}
		if code != 0 {
			return fmt.Errorf("%q exited %d: %s", cmdStr, code, strings.TrimSpace(stderr))
		}
		if err := assertNoHumanNotice(cmdStr, stdout); err != nil {
			return err
		}
		var envPayload config.ImpliedEnvelope
		if err := json.Unmarshal([]byte(stdout), &envPayload); err != nil {
			return fmt.Errorf("%q invalid JSON: %w\n%s", cmdStr, err, stdout)
		}
		if err := assertOfflineProbeParticipant(cmdStr, envPayload.Implied.Participants); err != nil {
			return err
		}
	}

	if _, statErr := os.Stat(sideEffectFile); statErr == nil {
		return fmt.Errorf("offline query mode must never start external run loop, but %s was created", sideEffectFile)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: offline-ok verified source and dump implied declarations without starting run loop\n").StdOut()
}

func assertOfflineProbeParticipant(label string, participants []config.ImpliedParticipant) error {
	for i := range participants {
		p := &participants[i]
		if p.Path != impliedPathProbe {
			continue
		}
		hasSend := slices.ContainsFunc(p.Send, func(r config.ImpliedDirectionRule) bool {
			return r.Message == busMsgAcmeAlert
		})
		hasRecvState := slices.ContainsFunc(p.Receive, func(r config.ImpliedDirectionRule) bool {
			return r.Message == busMsgBGPState
		})
		hasRecvUpdate := slices.ContainsFunc(p.Receive, func(r config.ImpliedDirectionRule) bool {
			return r.Message == busMsgBGPUpdate
		})
		if !hasSend {
			return fmt.Errorf("%s: %s missing declared send %s: %+v", label, impliedPathProbe, busMsgAcmeAlert, p.Send)
		}
		if !hasRecvState {
			return fmt.Errorf("%s: %s missing declared receive %s: %+v", label, impliedPathProbe, busMsgBGPState, p.Receive)
		}
		if !hasRecvUpdate {
			return fmt.Errorf("%s: %s missing declared receive %s: %+v", label, impliedPathProbe, busMsgBGPUpdate, p.Receive)
		}
		return nil
	}
	return fmt.Errorf("%s: %s not present in implied.participants", label, impliedPathProbe)
}

func busAssertOfflineRefusal(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-assert-implied offline-refusal <config-file> <endpoint> <run-path>")
	}
	cfgFile, epName, runPath := args[0], args[1], args[2]

	var confCmd textbuf.Buffer
	wantFix := confCmd.Str(`ze show plugin conformance run "`).Str(runPath).Byte('"').String()

	var srcCmd, dumpCmd textbuf.Buffer
	commands := []string{
		srcCmd.Str("show config source file ").Str(cfgFile).Str(" | implied").String(),
		dumpCmd.Str("show config dump ").Str(cfgFile).Str(" | implied").String(),
	}
	for _, cmdStr := range commands {
		code, stdout, stderr, err := runZeCLICaptured(ctx, cmdStr)
		if err != nil {
			return fmt.Errorf("run %q: %w", cmdStr, err)
		}
		if code == 0 {
			return fmt.Errorf("%q unexpectedly succeeded with exit 0; stdout:\n%s", cmdStr, stdout)
		}
		if strings.Contains(stdout, impliedByHeader) {
			return fmt.Errorf("%q leaked partial implied block to stdout:\n%s", cmdStr, stdout)
		}
		if strings.Contains(stdout, impliedInlineMarker) {
			return fmt.Errorf("%q leaked partial implied annotation to stdout:\n%s", cmdStr, stdout)
		}
		combined := stderr + "\n" + stdout
		for _, needle := range []string{epName, runPath, wantFix} {
			if !strings.Contains(combined, needle) {
				return fmt.Errorf("%q refusal missing %q:\nstderr:\n%s\nstdout:\n%s", cmdStr, needle, stderr, stdout)
			}
		}
	}
	var tb textbuf.Buffer
	return tb.Str("OK: offline-refusal verified source and dump refuse unqueryable endpoint cleanly\n").StdOut()
}

func busAssertConfigShowNotice(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-assert-implied config-show-notice <config-file>")
	}
	cfgFile := args[0]
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "config", "show", cfgFile)
	if err != nil {
		return fmt.Errorf("run ze config show %s: %w", cfgFile, err)
	}
	if code != 0 {
		return fmt.Errorf("ze config show %s exited %d: %s", cfgFile, code, strings.TrimSpace(stderr))
	}
	if strings.Contains(stdout, impliedByHeader) {
		return fmt.Errorf("ze config show leaked implied block onto stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, impliedInlineMarker) {
		return fmt.Errorf("ze config show leaked implied annotation onto stdout:\n%s", stdout)
	}
	if strings.Contains(stdout, impliedNoticeNeedle) {
		return fmt.Errorf("ze config show leaked implied notice onto stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, impliedNoticeNeedle) {
		return fmt.Errorf("ze config show stderr missing implied count notice:\n%s", stderr)
	}
	if !strings.Contains(stderr, "| implied") {
		return fmt.Errorf("ze config show stderr missing | implied hint:\n%s", stderr)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: config-show-notice verified pure stdout and stderr implied notice\n").StdOut()
}

func busAssertRoundTripExtract(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-assert-implied round-trip-extract <out-bus-snippet>")
	}
	outFile := args[0]

	code, textOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImplied)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImplied, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImplied, code, strings.TrimSpace(stderr))
	}
	if err := assertNoDoubleHashColumnZero("round-trip text", textOut); err != nil {
		return err
	}
	if !strings.Contains(textOut, impliedRegCollectorHeader) {
		return fmt.Errorf("missing %q:\n%s", impliedRegCollectorHeader, textOut)
	}

	code, jsonOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImpliedJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImpliedJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImpliedJSON, code, strings.TrimSpace(stderr))
	}
	var envPayload config.ImpliedEnvelope
	if err := json.Unmarshal([]byte(jsonOut), &envPayload); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope JSON: %w\n%s", err, jsonOut)
	}
	origin, err := findReceiveOrigin(envPayload.Implied.Participants, impliedPathCol, busMsgBGPUpdate)
	if err != nil {
		return err
	}
	if origin != config.ImpliedOriginRegistered {
		return fmt.Errorf("collector %s origin before paste = %q, want %q", busMsgBGPUpdate, origin, config.ImpliedOriginRegistered)
	}

	snippet, err := extractImpliedBusBlock(textOut, impliedRegCollectorHeader)
	if err != nil {
		return fmt.Errorf("extract collector bus block: %w\n%s", err, textOut)
	}
	if err := busWriteAtomicFile(outFile, snippet); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: round-trip-extract captured collector bus block with origin registered\n").StdOut()
}

func extractImpliedBusBlock(text, header string) (string, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("header %q not found", header)
	}
	var out textbuf.Buffer
	depth := 0
	startedBlock := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		out.Str(line).Byte('\n')
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		code := trimmed
		if hashIdx := strings.IndexByte(code, '#'); hashIdx >= 0 {
			code = code[:hashIdx]
		}
		for j := range len(code) {
			switch code[j] {
			case '{':
				depth++
				startedBlock = true
			case '}':
				depth--
			}
		}
		if startedBlock && depth == 0 {
			return out.String(), nil
		}
	}
	return "", fmt.Errorf("unterminated bus block after %q", header)
}

func findReceiveOrigin(participants []config.ImpliedParticipant, path, message string) (string, error) {
	for i := range participants {
		p := &participants[i]
		if p.Path != path {
			continue
		}
		for j := range p.Receive {
			if p.Receive[j].Message == message {
				return p.Receive[j].Origin, nil
			}
		}
	}
	return "", fmt.Errorf("participant %s receive rule %s not found", path, message)
}

func busAssertRoundTripVerify(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/bus-assert-implied round-trip-verify")
	}
	code, jsonOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImpliedJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImpliedJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImpliedJSON, code, strings.TrimSpace(stderr))
	}
	var envPayload config.ImpliedEnvelope
	if err := json.Unmarshal([]byte(jsonOut), &envPayload); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope JSON: %w\n%s", err, jsonOut)
	}
	origin, err := findReceiveOrigin(envPayload.Implied.Participants, impliedPathCol, busMsgBGPUpdate)
	if err != nil {
		return err
	}
	if origin != config.ImpliedOriginWritten {
		return fmt.Errorf("collector %s origin after paste = %q, want %q", busMsgBGPUpdate, origin, config.ImpliedOriginWritten)
	}

	code, textOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImplied)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImplied, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImplied, code, strings.TrimSpace(stderr))
	}
	if strings.Contains(textOut, impliedRegCollectorHeader) {
		return fmt.Errorf("collector still has duplicate implied block after writing it explicitly:\n%s", textOut)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: round-trip-verify confirmed origin written with no duplicate implied block\n").StdOut()
}

func busAssertRSDisplay(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: plugin/bus-assert-implied rs-display <peer-path>...")
	}
	wantPeers := args

	// 1. `show config running | implied | json`
	code, jsonOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowRunningImpliedJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowRunningImpliedJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowRunningImpliedJSON, code, strings.TrimSpace(stderr))
	}
	var envPayload config.ImpliedEnvelope
	if err := json.Unmarshal([]byte(jsonOut), &envPayload); err != nil {
		return fmt.Errorf("decode ImpliedEnvelope JSON: %w\n%s", err, jsonOut)
	}
	var rsRules []config.ImpliedDirectionRule
	for i := range envPayload.Implied.Participants {
		p := &envPayload.Implied.Participants[i]
		if p.Path != impliedPathRS {
			continue
		}
		for j := range p.Send {
			if p.Send[j].Message == busMsgBGPUpdate {
				rsRules = append(rsRules, p.Send[j])
			}
		}
	}
	if len(rsRules) != 1 {
		return fmt.Errorf("%s bgp:update send rules in implied.participants = %d, want 1 (%+v)", impliedPathRS, len(rsRules), rsRules)
	}
	if rsRules[0].Origin != config.ImpliedOriginRegistered {
		return fmt.Errorf("%s bgp:update send origin = %q, want %q", impliedPathRS, rsRules[0].Origin, config.ImpliedOriginRegistered)
	}
	for _, peer := range wantPeers {
		if !slices.Contains(rsRules[0].Targets, peer) {
			return fmt.Errorf("%s bgp:update implied targets %v missing %s", impliedPathRS, rsRules[0].Targets, peer)
		}
	}

	// 2. `show event delivery | json`
	code, delOut, stderr, err := runZeCLICaptured(ctx, cliCmdShowEventDeliveryJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowEventDeliveryJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowEventDeliveryJSON, code, strings.TrimSpace(stderr))
	}
	var delPayload busDeliveryResponseView
	if err := json.Unmarshal([]byte(delOut), &delPayload); err != nil {
		return fmt.Errorf("decode show event delivery JSON: %w\n%s", err, delOut)
	}
	var rsEdges []busDeliveryEdgeView
	for i := range delPayload.Edges {
		e := &delPayload.Edges[i]
		if e.SenderPath == impliedPathRS && e.Message == busMsgBGPUpdate {
			rsEdges = append(rsEdges, *e)
		}
	}
	if len(rsEdges) != 1 {
		return fmt.Errorf("show event delivery %s bgp:update sender edges = %d, want 1 (%+v)", impliedPathRS, len(rsEdges), rsEdges)
	}
	if rsEdges[0].Origin != config.ImpliedOriginRegistered {
		return fmt.Errorf("show event delivery %s origin = %q, want %q", impliedPathRS, rsEdges[0].Origin, config.ImpliedOriginRegistered)
	}
	for _, peer := range wantPeers {
		if !slices.Contains(rsEdges[0].Destinations, peer) {
			return fmt.Errorf("show event delivery %s destinations %v missing %s", impliedPathRS, rsEdges[0].Destinations, peer)
		}
	}
	var tb textbuf.Buffer
	return tb.Str("OK: rs-display verified single /bgp/route-server bgp:update entry listing all clients\n").StdOut()
}

func busAssertDeliveryOrigins(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return errors.New("usage: plugin/bus-assert-implied delivery-origins <config-file> <dyn-group-path> <dyn-ip> <static-peer-path>")
	}
	cfgFile, dynGroupPath, dynIP, staticPeerPath := args[0], args[1], args[2], args[3]

	// Poll show event delivery | json until the dynamic peer's edge is visible.
	var delPayload busDeliveryResponseView
	var lastErr error
	foundDyn := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		code, out, stderr, err := runZeCLICaptured(ctx, cliCmdShowEventDeliveryJSON)
		if err != nil {
			lastErr = err
			return false
		}
		if code != 0 {
			lastErr = fmt.Errorf("exit %d: %s", code, strings.TrimSpace(stderr))
			return false
		}
		var parsed busDeliveryResponseView
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			lastErr = err
			return false
		}
		for i := range parsed.Edges {
			e := &parsed.Edges[i]
			if e.SenderGroup == dynGroupPath && strings.Contains(e.SenderPath, dynIP) {
				delPayload = parsed
				lastErr = nil
				return true
			}
		}
		lastErr = fmt.Errorf("dynamic peer %s (%s) not yet in %d edges", dynIP, dynGroupPath, len(parsed.Edges))
		return false
	})
	if !foundDyn {
		return fmt.Errorf("timed out waiting for dynamic peer in show event delivery: %w", lastErr)
	}

	validOrigins := map[string]bool{
		config.ImpliedOriginWritten:    true,
		config.ImpliedOriginInherited:  true,
		config.ImpliedOriginRegistered: true,
		config.ImpliedOriginRequired:   true,
		config.ImpliedOriginDormant:    true,
	}
	seenOrigins := make(map[string]bool)
	seenStatic := false
	for i := range delPayload.Edges {
		e := &delPayload.Edges[i]
		if !validOrigins[e.Origin] {
			return fmt.Errorf("delivery edge carries invalid origin %q: %+v", e.Origin, *e)
		}
		seenOrigins[e.Origin] = true
		if e.SenderPath == staticPeerPath {
			if e.SenderGroup != "" {
				return fmt.Errorf("static peer %s edge must have empty sender-group, got %q", staticPeerPath, e.SenderGroup)
			}
			seenStatic = true
		}
	}
	if !seenStatic {
		return fmt.Errorf("static peer %s edge not found in show event delivery", staticPeerPath)
	}
	for _, wantOrigin := range []string{
		config.ImpliedOriginWritten,
		config.ImpliedOriginInherited,
		config.ImpliedOriginRegistered,
		config.ImpliedOriginDormant,
	} {
		if !seenOrigins[wantOrigin] {
			return fmt.Errorf("show event delivery missing origin %q (seen: %v)", wantOrigin, seenOrigins)
		}
	}

	// Verify `ze config graph <config-file>` carries origin-annotated process-binds edges.
	code, graphOut, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "config", "graph", cfgFile)
	if err != nil {
		return fmt.Errorf("run ze config graph %s: %w", cfgFile, err)
	}
	if code != 0 {
		return fmt.Errorf("ze config graph %s exited %d: %s", cfgFile, code, strings.TrimSpace(stderr))
	}
	var graphDoc config.Graph
	if err := json.Unmarshal([]byte(graphOut), &graphDoc); err != nil {
		return fmt.Errorf("decode ze config graph JSON: %w\n%s", err, graphOut)
	}
	busEdgesWithOrigin := 0
	for _, edge := range graphDoc.Edges {
		if edge.Kind != config.EdgeProcessBinds {
			continue
		}
		if !validOrigins[edge.Origin] {
			return fmt.Errorf("config graph process-binds edge missing valid origin: %+v", edge)
		}
		busEdgesWithOrigin++
	}
	if busEdgesWithOrigin == 0 {
		return fmt.Errorf("ze config graph %s reported zero origin-annotated process-binds edges", cfgFile)
	}

	var tb textbuf.Buffer
	return tb.Str("OK: delivery-origins verified all origins, dynamic group attribution, static peer, and config graph\n").StdOut()
}

func busAssertLifecycleDelivery(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/bus-assert-implied lifecycle-delivery <present|absent>")
	}
	want := args[0]
	if want != impliedEdgePresent && want != impliedEdgeAbsent {
		return fmt.Errorf("lifecycle-delivery mode %q must be %s or %s", want, impliedEdgePresent, impliedEdgeAbsent)
	}
	code, out, stderr, err := runZeCLICaptured(ctx, cliCmdShowEventDeliveryJSON)
	if err != nil {
		return fmt.Errorf("run %s: %w", cliCmdShowEventDeliveryJSON, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d: %s", cliCmdShowEventDeliveryJSON, code, strings.TrimSpace(stderr))
	}
	var payload busDeliveryResponseView
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return fmt.Errorf("decode show event delivery JSON: %w\n%s", err, out)
	}
	foundVal := false
	for i := range payload.Edges {
		e := &payload.Edges[i]
		if e.ReceiverPath != impliedPathVal && e.ReceiverEndpoint != impliedOpenValPlugin {
			continue
		}
		if e.Message != busMsgBGPState {
			continue
		}
		if want == impliedEdgePresent && e.Origin != config.ImpliedOriginRegistered {
			return fmt.Errorf("val bgp:state edge origin = %q, want %q", e.Origin, config.ImpliedOriginRegistered)
		}
		foundVal = true
	}
	if want == impliedEdgePresent && !foundVal {
		return fmt.Errorf("show event delivery missing registered bgp:state edge for %s:\n%s", impliedPathVal, out)
	}
	if want == impliedEdgeAbsent && foundVal {
		return fmt.Errorf("show event delivery still lists bgp:state edge for %s after allow none:\n%s", impliedPathVal, out)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: lifecycle-delivery verified val bgp:state is ").Str(want).Byte('\n').StdOut()
}

func busAssertFileNeedle(args []string, wantPresent bool) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-assert-implied file-contains|file-not-contains <file> <needle>")
	}
	path, needle := args[0], args[1]
	raw, err := os.ReadFile(path) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		if !wantPresent && errors.Is(err, os.ErrNotExist) {
			var tb textbuf.Buffer
			return tb.Str("OK: file ").Str(path).Str(" absent (does not contain ").Str(needle).Str(")\n").StdOut()
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	has := strings.Contains(string(raw), needle)
	if wantPresent && !has {
		return fmt.Errorf("file %s missing expected %q:\n%s", path, needle, string(raw))
	}
	if !wantPresent && has {
		return fmt.Errorf("file %s unexpectedly contains %q:\n%s", path, needle, string(raw))
	}
	var tb textbuf.Buffer
	if wantPresent {
		return tb.Str("OK: file ").Str(path).Str(" contains ").Str(needle).Byte('\n').StdOut()
	}
	return tb.Str("OK: file ").Str(path).Str(" does not contain ").Str(needle).Byte('\n').StdOut()
}

func busPollFileContains(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-assert-implied poll-file-contains <file> <needle>")
	}
	path, needle := args[0], args[1]
	var lastErr error
	matched := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		raw, err := os.ReadFile(path) //nolint:gosec // test scratch file in runner workdir
		if err != nil {
			lastErr = err
			return false
		}
		if strings.Contains(string(raw), needle) {
			lastErr = nil
			return true
		}
		lastErr = fmt.Errorf("file %s does not yet contain %q", path, needle)
		return false
	})
	if !matched {
		return fmt.Errorf("timed out waiting for %q in %s: %w", needle, path, lastErr)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: file ").Str(path).Str(" contains ").Str(needle).Byte('\n').StdOut()
}

func readFileOrEmpty(path string) (string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(raw), nil
}

func busSnapshotFile(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-assert-implied snapshot-file <src-file> <snap-file>")
	}
	src, dst := args[0], args[1]
	content, err := readFileOrEmpty(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := busWriteAtomicFile(dst, content); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: snapshotted ").Str(src).Str(" -> ").Str(dst).Byte('\n').StdOut()
}

func busPollFileGrewNeedle(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-assert-implied poll-file-grew-needle <file> <snap-file> <needle>")
	}
	path, snapPath, needle := args[0], args[1], args[2]
	baseContent, err := readFileOrEmpty(snapPath)
	if err != nil {
		return fmt.Errorf("read snapshot %s: %w", snapPath, err)
	}
	baseCount := strings.Count(baseContent, needle)
	var lastErr error
	grew := Poll(ctx, busPollAttempts, busPollDelay, func() bool {
		cur, readErr := readFileOrEmpty(path)
		if readErr != nil {
			lastErr = readErr
			return false
		}
		if strings.Count(cur, needle) > baseCount {
			lastErr = nil
			return true
		}
		lastErr = fmt.Errorf("file %s count(%q)=%d, want > %d", path, needle, strings.Count(cur, needle), baseCount)
		return false
	})
	if !grew {
		return fmt.Errorf("timed out waiting for new %q in %s: %w", needle, path, lastErr)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: file ").Str(path).Str(" recorded new ").Str(needle).Byte('\n').StdOut()
}

func busAssertFileUnchanged(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/bus-assert-implied file-unchanged <file> <snap-file>")
	}
	path, snapPath := args[0], args[1]
	baseContent, err := readFileOrEmpty(snapPath)
	if err != nil {
		return fmt.Errorf("read snapshot %s: %w", snapPath, err)
	}
	curContent, err := readFileOrEmpty(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if curContent != baseContent {
		return fmt.Errorf("file %s changed after suppression:\nbefore:\n%s\nafter:\n%s", path, baseContent, curContent)
	}
	var tb textbuf.Buffer
	return tb.Str("OK: file ").Str(path).Str(" unchanged relative to ").Str(snapPath).Byte('\n').StdOut()
}

func busMergeConfigFiles(args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/bus-assert-implied merge-config <base-file> <snippet-file> <out-file>")
	}
	baseRaw, err := os.ReadFile(args[0]) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		return fmt.Errorf("read base %s: %w", args[0], err)
	}
	snipRaw, err := os.ReadFile(args[1]) //nolint:gosec // test scratch file in runner workdir
	if err != nil {
		return fmt.Errorf("read snippet %s: %w", args[1], err)
	}
	var combined textbuf.Buffer
	combined.Str(string(baseRaw))
	if len(baseRaw) > 0 && baseRaw[len(baseRaw)-1] != '\n' {
		combined.Byte('\n')
	}
	combined.Byte('\n').Str(string(snipRaw))
	if err := busWriteAtomicFile(args[2], combined.String()); err != nil {
		return err
	}
	var tb textbuf.Buffer
	return tb.Str("OK: merged ").Str(args[0]).Str(" + ").Str(args[1]).Str(" -> ").Str(args[2]).Byte('\n').StdOut()
}
