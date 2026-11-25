// Design: docs/architecture/api/process-protocol.md -- bus send gate, refusal counter, and plugin command profile lookup
// Related: dispatch_registry.go -- built-in engineOp table with BusMessage / BusExemptReason declarations
// Related: register_engine_op.go -- owner-registered EngineOperation declarations
// Related: bus_filter.go -- CompiledBus and CompiledParticipantFilter

package server

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/aaa"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// Dynamic bus message sentinels for operations whose exact message name is
	// derived from request parameters at dispatch time.
	busMsgDynamicCLI         = "cli:*"
	busMsgDynamicEmit        = "emit:*"
	busMsgDynamicReceive     = "receive:*"
	busMsgDynamicUpdateRoute = "bgp:update|refresh"

	// Static bus message names carried by built-in engine operations.
	busMsgBGPUpdate        = "bgp:update"
	busMsgBGPRefresh       = "bgp:refresh"
	busMsgBGPRaw           = "bgp:raw"
	busMsgSystemRIBInstall = "system-rib:route-install"
	busMsgSystemRIBMetrics = "system-rib:route-metrics"
	busMsgResolveDNS       = "resolve:dns"
	busMsgBatchValidate    = busCLIPrefix + rpc.BatchValidateCommand

	// Shared exemption reasons repeated across multiple engine operations.
	busExemptStateGrants = "governed by state key grants on compiled registration identity"

	// Inspection command named when a participant's registration omits a message.
	busInspectDeclarationsFix = "show config running | implied"
)

type busSendRefusedKey struct {
	participant string
	message     string
}

type busSendRefusedMetrics struct {
	counter metrics.CounterVec // labels: participant, message
}

var (
	busSendRefusedMetricsPtr atomic.Pointer[busSendRefusedMetrics]
	busSendRefusedCountsMu   sync.Mutex
	busSendRefusedCounts     = make(map[busSendRefusedKey]uint64)

	activeProfileServer atomic.Pointer[Server]
)

// SetBusSendRefusedMetrics wires the Prometheus counter
// ze_bus_send_refused_total{participant, message} onto reg. Safe for concurrent
// use. A nil registry leaves the Prometheus sink unset while in-process counts
// remain readable through BusSendRefusedCount.
func SetBusSendRefusedMetrics(reg metrics.Registry) {
	if reg == nil {
		busSendRefusedMetricsPtr.Store(nil)
		return
	}
	busSendRefusedMetricsPtr.Store(&busSendRefusedMetrics{
		counter: reg.CounterVec(
			"ze_bus_send_refused_total",
			"Bus messages refused by the compiled bus filter, by participant and message.",
			[]string{"participant", "message"},
		),
	})
}

// RecordBusSendRefused increments ze_bus_send_refused_total for (participant,
// message). Safe for concurrent use.
func RecordBusSendRefused(participant, message string) {
	if m := busSendRefusedMetricsPtr.Load(); m != nil {
		m.counter.With(participant, message).Inc()
	}
	busSendRefusedCountsMu.Lock()
	busSendRefusedCounts[busSendRefusedKey{participant: participant, message: message}]++
	busSendRefusedCountsMu.Unlock()
}

// BusSendRefusedCount returns the cumulative recorded send refusals for
// (participant, message) in this process. Safe for concurrent use.
func BusSendRefusedCount(participant, message string) uint64 {
	busSendRefusedCountsMu.Lock()
	count := busSendRefusedCounts[busSendRefusedKey{participant: participant, message: message}]
	busSendRefusedCountsMu.Unlock()
	return count
}

// ReportBusSendRefusal records a bus send refusal on the single refusal
// counter, logs WARN with CodeDoctorPluginRequestRefused, and raises the
// bounded doctor-plugin-request-refused event on the report bus.
func ReportBusSendRefusal(participant, message, reason, fix string) {
	RecordBusSendRefused(participant, message)
	logger().Warn("bus send refused",
		"participant", participant,
		"message", message,
		"reason", reason,
		"fix", fix,
		"code", diagnostic.CodeDoctorPluginRequestRefused)
	var detail textbuf.Buffer
	detail.Str("participant ").Str(participant).Str(" refused sending ").Str(message).Str(": ").Str(reason)
	if fix != "" {
		detail.Str("; fix: ").Str(fix)
	}
	refusePluginRequestWithFix(participant, detail.String(), fix)
}

// ValidateEngineOpEntry checks that one engine operation entry declares either
// BusMessage or BusExemptReason, and never both or neither.
func ValidateEngineOpEntry(name, busMessage, exemptReason string) error {
	msg := strings.TrimSpace(busMessage)
	reason := strings.TrimSpace(exemptReason)
	if msg == "" {
		if reason == "" {
			var tb textbuf.Buffer
			return opererr.New(
				tb.Str("engine operation ").Quoted(name).Str(" declares neither a bus message nor an exemption reason").String(),
				"every engine operation must declare its bus message or a written exemption reason so no operation ships ungated by omission",
				"set BusMessage or BusExemptReason on the operation entry")
		}
		return nil
	}
	if reason != "" {
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("engine operation ").Quoted(name).Str(" declares both a bus message and an exemption reason").String(),
			"an engine operation is either gated by a bus message or explicitly exempt, never both",
			"keep either BusMessage or BusExemptReason on the operation entry")
	}
	return nil
}

// ValidateAllEngineOps walks every built-in engineOp and every owner-registered
// EngineOperation and fails if any entry omits or duplicates its bus
// declaration.
func ValidateAllEngineOps() error {
	seen := make(map[string]bool, len(engineOps))
	var errs []error
	for i := range engineOps {
		op := &engineOps[i]
		seen[op.method] = true
		if err := ValidateEngineOpEntry(op.method, op.BusMessage, op.BusExemptReason); err != nil {
			errs = append(errs, err)
		}
	}
	for _, op := range allEngineOps() {
		if seen[op.method] {
			continue
		}
		if err := ValidateEngineOpEntry(op.method, op.BusMessage, op.BusExemptReason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// FormatBusTargetRefusal builds the Reason 3 refusal when a send > to or
// receive > from list does not cover delivery of msg from senderPath to
// targetPath.
func FormatBusTargetRefusal(senderPath, targetPath, msg, listSource string) *opererr.Error {
	var problem, why, fix textbuf.Buffer
	problem.Str("participant ").Str(senderPath).Str(" cannot send ").Str(msg).Str(" to ").Str(targetPath)
	if listSource != "" {
		why.Str("the target list at ").Str(listSource).Str(" does not cover delivery of ").
			Str(msg).Str(" from ").Str(senderPath).Str(" to ").Str(targetPath)
		fix.Str("update ").Str(listSource).Str(" to cover ").Str(targetPath)
	} else {
		why.Str("the compiled send > to or receive > from list does not cover delivery of ").
			Str(msg).Str(" from ").Str(senderPath).Str(" to ").Str(targetPath)
		fix.Str("update send > to on ").Str(senderPath).Str(" or receive > from on ").Str(targetPath)
	}
	return opererr.New(problem.String(), why.String(), fix.String())
}

func formatBusUndeclaredRefusal(participantPath, msg string) *opererr.Error {
	var problem, why textbuf.Buffer
	problem.Str("participant ").Str(participantPath).Str(" cannot send ").Str(msg).
		Str(": its registration does not declare sending ").Str(msg)
	why.Str("a participant may send only messages its registration declares; inspect declared messages with ").
		Str(busInspectDeclarationsFix).Str(" or show plugin conformance")
	return opererr.New(problem.String(), why.String(), busInspectDeclarationsFix)
}

func formatBusAllowListRefusal(participantPath, endpointName, msg string, origin BusRuleOrigin) *opererr.Error {
	var problem, why textbuf.Buffer
	problem.Str("participant ").Str(participantPath).Str(" cannot send ").Str(msg).
		Str(": removed by send allow-list")
	why.Str("the participant's registration declares ").Str(msg).Str(", but its send allow-list")
	if src := firstOriginSource(origin); src != "" {
		why.Str(" (").Str(src).Byte(')')
	}
	why.Str(" excludes it")
	return opererr.New(problem.String(), why.String(), formatBusSetAllowFix(endpointName, participantPath, busDirSend, msg))
}

func formatBusSetAllowFix(endpointName, participantPath, direction, msg string) string {
	var fix textbuf.Buffer
	if endpointName != "" {
		fix.Str("set bus endpoint ").Str(endpointName)
	} else {
		fix.Str("set bus participant ").Str(participantPath)
	}
	fix.Byte(' ').Str(direction).Str(" allow ")
	if strings.ContainsAny(msg, " \t") {
		fix.Quoted(msg)
	} else {
		fix.Str(msg)
	}
	return fix.String()
}

func firstOriginSource(origin BusRuleOrigin) string {
	if len(origin.Sources) == 0 {
		return ""
	}
	return origin.Sources[0]
}

// isDynamicBusMessage reports whether busMessage is one of the dynamic
// sentinels whose exact message name is resolved inside the operation's shared
// handler.
func isDynamicBusMessage(busMessage string) bool {
	switch busMessage {
	case busMsgDynamicCLI, busMsgDynamicEmit, busMsgDynamicReceive, busMsgDynamicUpdateRoute:
		return true
	default:
		return false
	}
}

// gateEngineOp enforces the bus send gate for every static-message engine
// operation before its handler or native adapter executes. Exempt operations
// (BusMessage == "") pass immediately; dynamic operations check their exact
// request-derived message inside their shared core handlers.
func (s *Server) gateEngineOp(proc *process.Process, op *engineOp) error {
	if op == nil {
		return nil
	}
	if op.BusMessage == "" {
		return nil
	}
	if isDynamicBusMessage(op.BusMessage) {
		return nil
	}
	if proc != nil {
		if err := proc.RequirePeerDeclarations(); err != nil {
			return err
		}
	}
	return checkEngineOpBusSend(s, proc, op.BusMessage)
}

// resolveParticipantFilter returns proc's compiled filter from s's active
// DeliveryGraph, falling back to compiling proc's live registration + BusConfig
// when the graph has not yet been published.
func resolveParticipantFilter(s *Server, proc *process.Process) (*CompiledParticipantFilter, error) {
	if proc == nil {
		return nil, opererr.New(
			"bus send refused: request carries no authenticated participant",
			"every engine operation that sends a bus message requires an authenticated endpoint process",
			busInspectDeclarationsFix)
	}
	epName := proc.Name()
	epPath := plugin.EndpointPath(epName)
	if s != nil {
		if compiled := s.DeliveryGraph().Compiled(); compiled != nil {
			if f := findEndpointFilter(compiled, epName, epPath); f != nil {
				return f, nil
			}
		}
	}
	return compileSingleProcessFilter(s, proc)
}

func findEndpointFilter(compiled *CompiledBus, epName, epPath string) *CompiledParticipantFilter {
	if compiled == nil {
		return nil
	}
	if f := compiled.Filter(epPath); f != nil {
		return f
	}
	for _, cand := range compiled.filters {
		if cand == nil {
			continue
		}
		if cand.EndpointName == epName {
			return cand
		}
	}
	return nil
}

func compileSingleProcessFilter(s *Server, proc *process.Process) (*CompiledParticipantFilter, error) {
	epName := proc.Name()
	epPath := plugin.EndpointPath(epName)
	var sends []rpc.MessageSendDecl
	var receives []rpc.MessageReceiveDecl

	if liveReg := proc.Registration(); liveReg != nil {
		sends = liveReg.Sends
		receives = liveReg.Receives
	}
	// Compiled-in plugins carry a non-empty RegistrationName and fall back to
	// their compiled registration when Stage 1 has not yet populated liveReg.
	// Foreign external endpoints have RegistrationName() == "" and never inherit
	// a compiled registration of the same name.
	if len(sends) == 0 && len(receives) == 0 {
		if regName := proc.RegistrationName(); regName != "" {
			if compiled := registry.Lookup(regName); compiled != nil {
				sends = compiled.Sends
				receives = compiled.Receives
			}
		}
	}

	input := BusParticipantInput{
		Path:         epPath,
		EndpointName: epName,
		Sends:        sends,
		Receives:     receives,
	}
	if s != nil {
		applyEndpointBusConfig(&input, s.BusConfig())
	}
	compiled, err := CompileBus(NewTagInterner(), []BusParticipantInput{input})
	if err != nil {
		return nil, err
	}
	return compiled.Filter(epPath), nil
}

// checkEngineOpBusSend enforces proc's compiled bus send filter for msg on the
// shared dispatch path. On refusal it records the refusal counter, logs WARN
// with CodeDoctorPluginRequestRefused, raises the report-bus event, and returns
// the formatted operator error.
func checkEngineOpBusSend(s *Server, proc *process.Process, msg string) error {
	if strings.HasPrefix(msg, busCLIPrefix) {
		return checkEngineOpBusSendCLI(s, proc, msg, msg)
	}
	f, err := resolveParticipantFilter(s, proc)
	if err != nil {
		participant := "unknown"
		if proc != nil {
			participant = plugin.EndpointPath(proc.Name())
		}
		ReportBusSendRefusal(participant, msg, err.Error(), busInspectDeclarationsFix)
		return err
	}
	if f == nil {
		participant := plugin.EndpointPath(proc.Name())
		refusal := formatBusUndeclaredRefusal(participant, msg)
		ReportBusSendRefusal(participant, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	rule, declared := f.SendRules[msg]
	if !declared {
		refusal := formatBusUndeclaredRefusal(f.Path, msg)
		ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	if rule == nil {
		refusal := formatBusUndeclaredRefusal(f.Path, msg)
		ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	if f.SendAllowNone {
		refusal := formatBusAllowListRefusal(f.Path, f.EndpointName, msg, f.SendAllowOrigin)
		ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	if !rule.Allowed {
		refusal := formatBusAllowListRefusal(f.Path, f.EndpointName, msg, f.SendAllowOrigin)
		ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	if rule.ToNone {
		refusal := FormatBusTargetRefusal(f.Path, plugin.BusKeywordNone, msg, firstOriginSource(rule.ToOrigin))
		ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	return nil
}

// checkEngineOpBusSendCLI checks a cli:<command> send against proc's compiled
// filter. matchMsg is the full normalized cli:<command> [args...] used for
// prefix matching; reportMsg is the bounded cli:<command> reported in errors
// and metrics.
func checkEngineOpBusSendCLI(s *Server, proc *process.Process, matchMsg, reportMsg string) error {
	f, err := resolveParticipantFilter(s, proc)
	if err != nil {
		participant := "unknown"
		if proc != nil {
			participant = plugin.EndpointPath(proc.Name())
		}
		ReportBusSendRefusal(participant, reportMsg, err.Error(), busInspectDeclarationsFix)
		return err
	}
	if f == nil {
		participant := plugin.EndpointPath(proc.Name())
		refusal := formatBusUndeclaredRefusal(participant, reportMsg)
		ReportBusSendRefusal(participant, reportMsg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	declared := false
	for declMsg := range f.SendRules {
		if !strings.HasPrefix(declMsg, busCLIPrefix) {
			continue
		}
		if cliPrefixMatchesFold(declMsg, matchMsg) {
			declared = true
			break
		}
	}
	if !declared {
		refusal := formatBusUndeclaredRefusal(f.Path, reportMsg)
		ReportBusSendRefusal(f.Path, reportMsg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	if f.SendAllowNone {
		refusal := formatBusAllowListRefusal(f.Path, f.EndpointName, reportMsg, f.SendAllowOrigin)
		ReportBusSendRefusal(f.Path, reportMsg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	cmdPath := strings.TrimPrefix(matchMsg, busCLIPrefix)
	admitted := false
	for _, prefix := range f.AllowedCLIPrefixes {
		if cliPrefixMatchesFold(prefix, cmdPath) {
			admitted = true
			break
		}
	}
	if !admitted {
		refusal := formatBusAllowListRefusal(f.Path, f.EndpointName, reportMsg, f.SendAllowOrigin)
		ReportBusSendRefusal(f.Path, reportMsg, refusal.Problem(), refusal.Fix())
		return refusal
	}
	return nil
}

// cliPrefixMatchesFold mirrors authz.Entry.matches: case-insensitive match at
// a word boundary (exact match or followed by a single space).
func cliPrefixMatchesFold(prefix, full string) bool {
	lp := strings.ToLower(prefix)
	lf := strings.ToLower(full)
	return cliPrefixMatches(lp, lf)
}

// checkEngineOpBusReceive verifies that every requested runtime event
// subscription in namespace is declared in proc's registered receptions and
// admitted by its compiled receive allow-list.
func checkEngineOpBusReceive(s *Server, proc *process.Process, namespace string, requested []string) error {
	f, err := resolveParticipantFilter(s, proc)
	if err != nil {
		return err
	}
	if f == nil {
		participant := plugin.EndpointPath(proc.Name())
		var problem textbuf.Buffer
		problem.Str("participant ").Str(participant).Str(" cannot subscribe to namespace ").Str(namespace).
			Str(": its registration declares no receptions")
		return opererr.New(problem.String(),
			"runtime subscriptions are bounded by the participant's registered receptions",
			busInspectDeclarationsFix)
	}
	for _, rawEvent := range requested {
		if rawEvent == "*" {
			if err := checkWildcardReceive(f, namespace); err != nil {
				return err
			}
			continue
		}
		fields := strings.Fields(rawEvent)
		if len(fields) == 0 {
			continue
		}
		var tb textbuf.Buffer
		msg := tb.Str(namespace).Byte(':').Str(fields[0]).String()
		rule, declared := f.ReceiveRules[msg]
		if !declared {
			return refuseUndeclaredReceive(f, msg)
		}
		if rule == nil {
			return refuseUndeclaredReceive(f, msg)
		}
		if f.ReceiveAllowNone {
			return refuseAllowExcludedReceive(f, msg)
		}
		if !rule.Allowed {
			return refuseAllowExcludedReceive(f, msg)
		}
	}
	return nil
}

func refuseUndeclaredReceive(f *CompiledParticipantFilter, msg string) error {
	var problem, why textbuf.Buffer
	problem.Str("participant ").Str(f.Path).Str(" cannot receive ").Str(msg).
		Str(": its registration does not declare receiving ").Str(msg)
	why.Str("runtime subscriptions are bounded by the participant's registered receptions; inspect with ").
		Str(busInspectDeclarationsFix).Str(" or show plugin conformance")
	refusal := opererr.New(problem.String(), why.String(), busInspectDeclarationsFix)
	ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
	return refusal
}

func refuseAllowExcludedReceive(f *CompiledParticipantFilter, msg string) error {
	var problem, why textbuf.Buffer
	problem.Str("participant ").Str(f.Path).Str(" cannot receive ").Str(msg).
		Str(": removed by receive allow-list")
	why.Str("the participant's registration declares receiving ").Str(msg).
		Str(", but its receive allow-list excludes it")
	fix := formatBusSetAllowFix(f.EndpointName, f.Path, busDirReceive, msg)
	refusal := opererr.New(problem.String(), why.String(), fix)
	ReportBusSendRefusal(f.Path, msg, refusal.Problem(), refusal.Fix())
	return refusal
}

func checkWildcardReceive(f *CompiledParticipantFilter, namespace string) error {
	nsPrefix := namespace + ":"
	hasDeclared := false
	hasAllowed := false
	for msg, rule := range f.ReceiveRules {
		if !strings.HasPrefix(msg, nsPrefix) {
			continue
		}
		hasDeclared = true
		if rule != nil && rule.Allowed {
			hasAllowed = true
		}
	}
	if !hasDeclared {
		var problem textbuf.Buffer
		problem.Str("participant ").Str(f.Path).Str(" cannot subscribe to ").Str(nsPrefix).Str("*: its registration declares no receptions in ").Str(namespace)
		refusal := opererr.New(problem.String(),
			"runtime subscriptions are bounded by the participant's registered receptions",
			busInspectDeclarationsFix)
		ReportBusSendRefusal(f.Path, nsPrefix+"*", refusal.Problem(), refusal.Fix())
		return refusal
	}
	if f.ReceiveAllowNone {
		return refuseWildcardAllowExcluded(f, namespace, nsPrefix)
	}
	if !hasAllowed {
		return refuseWildcardAllowExcluded(f, namespace, nsPrefix)
	}
	return nil
}

func refuseWildcardAllowExcluded(f *CompiledParticipantFilter, namespace, nsPrefix string) error {
	var problem textbuf.Buffer
	problem.Str("participant ").Str(f.Path).Str(" cannot subscribe to ").Str(nsPrefix).Str("*: removed by receive allow-list")
	fix := formatBusSetAllowFix(f.EndpointName, f.Path, busDirReceive, nsPrefix+"*")
	refusal := opererr.New(problem.String(),
		"the participant's receive allow-list excludes all receptions in "+namespace,
		fix)
	ReportBusSendRefusal(f.Path, nsPrefix+"*", refusal.Problem(), refusal.Fix())
	return refusal
}

// validateUpdateRouteSubcommand restricts update-route commands to UPDATE
// subcommands so callers cannot smuggle `raw` or unrelated commands past the
// bgp:update gate (Q-T4(b)).
func validateUpdateRouteSubcommand(command string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return opererr.New(
			"update-route command is empty",
			"an update-route request without a marker must supply an UPDATE subcommand",
			"use an UPDATE subcommand (update, unicast, blackhole, flowspec, announce, withdraw, or cached)")
	}
	switch strings.ToLower(fields[0]) {
	case "update", "unicast", "blackhole", "flowspec", "announce", "withdraw", "cached":
		return nil
	default:
		var tb textbuf.Buffer
		return opererr.New(
			tb.Str("update-route refuses subcommand ").Quoted(fields[0]).String(),
			"update-route is restricted to UPDATE subcommands under bgp:update and cannot invoke raw or arbitrary commands",
			"use an UPDATE subcommand (update, unicast, blackhole, flowspec, announce, withdraw, or cached), or dispatch-command with a declared cli: send")
	}
}

// updateRouteBusMessage returns the bus message governed by an update-route
// request: bgp:refresh for borr/eorr markers, bgp:update otherwise (Q-T4(b)).
func updateRouteBusMessage(input rpc.UpdateRouteInput) string {
	switch input.PeerMarker {
	case "borr", "eorr":
		return busMsgBGPRefresh
	default:
		return busMsgBGPUpdate
	}
}

// normalizeBusCommandPath collapses runs of whitespace into single spaces so
// cli:<command path> matching and refusal messages are canonical.
func normalizeBusCommandPath(command string) string {
	return strings.Join(strings.Fields(command), " ")
}

// cliMessagesForCommandArgs builds the match and report cli: message strings
// for dispatch-command-args. reportMsg stays bounded to the command path while
// matchMsg includes args so narrower declared prefixes still match.
func cliMessagesForCommandArgs(command string, args []string) (matchMsg, reportMsg string) {
	base := normalizeBusCommandPath(command)
	if base == "" {
		joined := normalizeBusCommandPath(strings.Join(args, " "))
		msg := busCLIPrefix + joined
		return msg, msg
	}
	reportMsg = busCLIPrefix + base
	if len(args) == 0 {
		return reportMsg, reportMsg
	}
	var tb textbuf.Buffer
	tb.Str(reportMsg)
	for _, a := range args {
		if a == "" {
			continue
		}
		tb.Byte(' ').Str(a)
	}
	return tb.String(), reportMsg
}

// extractClaimedSenderFromJSON reads any caller-supplied string "sender" field
// from a JSON-object event payload, returning an error if a JSON object fails
// to decode. Non-object payloads return ("", nil).
func extractClaimedSenderFromJSON(payload any) (string, error) {
	raw, ok := payload.(string)
	if !ok {
		if rm, isRaw := payload.(json.RawMessage); isRaw {
			raw = string(rm)
		} else {
			return "", nil
		}
	}
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil
	}
	if trimmed[0] != '{' {
		return "", nil
	}
	var header struct {
		Sender string `json:"sender"`
	}
	if err := json.Unmarshal([]byte(trimmed), &header); err != nil {
		return "", err
	}
	return header.Sender, nil
}

// stampSenderInJSON sets the top-level "sender" field of a JSON object payload
// to stampedSender, overwriting any caller-supplied sender value (AC-35, S25).
// Non-object payloads (such as "null" for signal events) pass through intact.
func stampSenderInJSON(eventJSON, stampedSender string) string {
	if stampedSender == "" {
		return eventJSON
	}
	trimmed := strings.TrimSpace(eventJSON)
	if len(trimmed) == 0 {
		return eventJSON
	}
	if trimmed[0] != '{' {
		return eventJSON
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return eventJSON
	}
	if obj == nil {
		return eventJSON
	}
	encodedSender, err := json.Marshal(stampedSender)
	if err != nil {
		return eventJSON
	}
	obj["sender"] = json.RawMessage(encodedSender)
	out, err := json.Marshal(obj)
	if err != nil {
		return eventJSON
	}
	return string(out)
}

// filterReceiversByCompiledSender filters candidate subscriber processes
// against compiled.Receivers(stampedSender, ...) when both sender and receiver
// are represented in the compiled bus (enforcing receive > from, send > to,
// and non-direction per-message tag filters on emitted events, AC-35 / S25 /
// AC-44 / S41). Tag extraction runs ONLY when at least one receiver on the
// (sender, message, direction) edge carries a runtime tag filter.
func filterReceiversByCompiledSender(
	compiled *CompiledBus,
	stampedSender string,
	nsID events.NamespaceID,
	etID events.EventTypeID,
	dirID events.Direction,
	payload any,
	procs []*process.Process,
) []*process.Process {
	if compiled == nil {
		return procs
	}
	if len(procs) == 0 {
		return procs
	}
	if compiled.Filter(stampedSender) == nil {
		return filterReceiversByInboundTags(compiled, nsID, etID, payload, procs)
	}
	filterTags := compiled.deliveryFilterTags(stampedSender, nsID, etID, dirID)
	var admitted []string
	if len(filterTags) == 0 {
		admitted = compiled.Receivers(stampedSender, nsID, etID, dirID, nil, nil)
	} else {
		var tagBuf [busTagStackCap]BusTagValue
		msgTags := extractEventBusTags(compiled.Interner(), filterTags, payload, tagBuf[:0])
		var recvBuf [busTagStackCap]string
		admitted = compiled.Receivers(stampedSender, nsID, etID, dirID, msgTags, recvBuf[:0])
	}
	kept := procs[:0]
	for _, p := range procs {
		if !compiled.hasEndpoint(p.Name()) {
			kept = append(kept, p)
			continue
		}
		if slices.Contains(admitted, p.Name()) {
			kept = append(kept, p)
		}
	}
	return kept
}

// publishPluginProfileServer registers s as the active publisher for
// aaa.LookupPluginCommandProfile. Safe for concurrent use.
func publishPluginProfileServer(s *Server) {
	if s == nil {
		return
	}
	activeProfileServer.Store(s)
	aaa.SetPluginCommandProfileLookup(lookupActivePluginCommandProfile)
}

// withdrawPluginProfileServer releases s from the active profile lookup if it
// is still the installed publisher. Safe for concurrent use.
func withdrawPluginProfileServer(s *Server) {
	if s == nil {
		return
	}
	if activeProfileServer.CompareAndSwap(s, nil) {
		aaa.SetPluginCommandProfileLookup(nil)
	}
}

func lookupActivePluginCommandProfile(endpoint string) (aaa.PluginCommandProfile, bool) {
	s := activeProfileServer.Load()
	if s == nil {
		return aaa.PluginCommandProfile{}, false
	}
	return s.lookupPluginCommandProfile(endpoint)
}

// lookupPluginCommandProfile resolves endpoint's compiled CLI command profile
// from s's active DeliveryGraph (or live process registration fallback) for
// authz.Store.authorize. Safe for concurrent use.
func (s *Server) lookupPluginCommandProfile(endpoint string) (aaa.PluginCommandProfile, bool) {
	if endpoint == "" {
		return aaa.PluginCommandProfile{}, false
	}
	epPath := plugin.EndpointPath(endpoint)
	if compiled := s.DeliveryGraph().Compiled(); compiled != nil {
		if f := findEndpointFilter(compiled, endpoint, epPath); f != nil {
			return compiledFilterToCommandProfile(endpoint, f), true
		}
	}
	if pm := s.procManager.Load(); pm != nil {
		if proc := pm.GetProcess(endpoint); proc != nil {
			if f, err := compileSingleProcessFilter(s, proc); err == nil && f != nil {
				return compiledFilterToCommandProfile(endpoint, f), true
			}
		}
	}
	if v, ok := s.adHocProcs.Load(endpoint); ok {
		if proc, ok := v.(*process.Process); ok && proc != nil {
			if f, err := compileSingleProcessFilter(s, proc); err == nil && f != nil {
				return compiledFilterToCommandProfile(endpoint, f), true
			}
		}
	}
	return aaa.PluginCommandProfile{}, false
}

func compiledFilterToCommandProfile(endpoint string, f *CompiledParticipantFilter) aaa.PluginCommandProfile {
	return aaa.PluginCommandProfile{
		Endpoint:        endpoint,
		AllowNone:       f.SendAllowNone || len(f.AllowedCLIPrefixes) == 0,
		AllowedPrefixes: slices.Clone(f.AllowedCLIPrefixes),
	}
}
