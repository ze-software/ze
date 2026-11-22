// Design: docs/architecture/config/yang-config-design.md — validation CLI
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidateContent runs the same validation as `ze config validate` and returns
// an error containing all validation errors. Warnings do not fail validation,
// matching the CLI command exit-code semantics.
//
// Each error is a block of its own, every line indented two spaces. An enum,
// range, length or pattern refusal is written for a session, which holds no
// file and no position: no line number, and its fix the `set` command from
// the root (sessionFixRefusal), so each block ends with its own `fix:` line.
// errors.As on the answer finds the first refusal's ValidationError and
// operator error (contentRefusal).
func ValidateContent(input, path string) error {
	result := runValidation(input, path)
	if result.Valid {
		return nil
	}
	var b textbuf.Buffer
	b.Str("config validation failed:")
	var refusals []error
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		if d.Severity != diagnostic.SeverityError {
			continue
		}
		if refusal, ok := result.refusals[i]; ok {
			framed := sessionFixRefusal(refusal)
			refusals = append(refusals, framed)
			writeIndentedBlock(&b, framed.Unwrap().Error())
			continue
		}
		var block textbuf.Buffer
		if d.Line > 0 {
			block.Str("line ").Int(int64(d.Line)).Str(": ")
		}
		writeIndentedBlock(&b, block.Str(d.Message).String())
	}
	return &contentRefusal{text: b.String(), refusals: refusals}
}

// sessionFixRefusal answers refusal with the fix a session types: `set`, the
// full setting path and the correction. A parser refusal carries the
// file-form fix of the line it refused, which a session cannot type.
func sessionFixRefusal(refusal *configyang.ValidationError) *configyang.ValidationError {
	var fix textbuf.Buffer
	fix.Str("set ").Str(config.FormatEditPath(refusal.Segments())).Byte(' ').Str(refusal.CorrectionText())
	return refusal.WithFix(fix.String())
}

// writeIndentedBlock writes text on its own lines after b, each line indented
// two spaces, so an accepted-value line (already two) reads four.
func writeIndentedBlock(b *textbuf.Buffer, text string) {
	for line := range strings.SplitSeq(text, "\n") {
		b.Str("\n  ").Str(line)
	}
}

// contentRefusal is ValidateContent's answer: the rendered blocks, and the
// refusals among them in order, so errors.As reaches the first one's typed
// facts and operator error.
type contentRefusal struct {
	text     string
	refusals []error
}

func (e *contentRefusal) Error() string { return e.text }

// Unwrap answers the refusals the text holds, first one first.
func (e *contentRefusal) Unwrap() []error { return e.refusals }

// validationResult holds validation results with structured diagnostics.
// refusals maps the index of an error diagnostic to the enum, range, length
// or pattern refusal it reports, for the surfaces that render the refusal
// again for themselves (ValidateContent).
type validationResult struct {
	Valid       bool
	Path        string
	Diagnostics []diagnostic.Diagnostic
	Config      *validationSummary
	refusals    map[int]*configyang.ValidationError
}

// refusalDiagnostic answers the diagnostic of a parser refusal: the walk's
// code for its kind, message (the refusal with its line left out, SplitLine)
// and line, the accepted values or bounds as expected, the refused value as
// actual unless the leaf is sensitive, and the path in the walk's form.
func refusalDiagnostic(schema *config.Schema, refusal *configyang.ValidationError, message string, line int) diagnostic.Diagnostic {
	d := diagnostic.Diagnostic{
		Code:     yangErrorCode(refusal.Type),
		Severity: diagnostic.SeverityError,
		Message:  message,
		Line:     line,
		Path:     walkFormPath(schema, refusal.Segments()),
		Fix:      refusalFix(refusal),
	}
	if refusal.Expected != "" {
		d.Expected = refusal.Expected
	}
	if refusal.Got != "" && !refusal.Secret() {
		d.Actual = refusal.Got
	}
	if r, s := yangRepair(refusal.Type); r != nil {
		d.Repair = r
		d.FixSafety = s
	}
	return d
}

// refusalFix answers the line that clears refusal, alone and with no label,
// as every finding carries it: the fix the refusal's operator error holds.
func refusalFix(refusal *configyang.ValidationError) string {
	held, ok := errors.AsType[*opererr.Error](error(refusal))
	if !ok {
		// refuse-by-default: guards refusalDiagnostic -- a refusal with no operator error holds no fix line to carry
		return ""
	}
	return held.Fix()
}

// walkFormPath answers segments, a setting path as the operator types it, in
// the form the tree walk's diagnostics name a leaf: the section, a dot, then
// the slash path with each list entry's address in brackets
// (`bgp.bgp/peer[peer1]/session/capability/route-refresh`). It answers "" when
// a word names no schema node, so the diagnostic omits a path it cannot name.
// The loop is bounded by len(segments): every pass consumes one word or more.
func walkFormPath(schema *config.Schema, segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	var path textbuf.Buffer
	path.Str(segments[0]).Byte('.')
	var current config.ChildProvider = schema
	for i := 0; i < len(segments); {
		node := current.Get(segments[i])
		if node == nil {
			return ""
		}
		if i > 0 {
			path.Byte('/')
		}
		path.Str(segments[i])
		i++
		if list, isList := node.(*config.ListNode); isList {
			if n := config.ListAddressLen(list, segments[i:]); n > 0 {
				path.Byte('[').Str(textbuf.Join(segments[i:i+n], " ")).Byte(']')
				i += n
			}
		}
		if i == len(segments) {
			break
		}
		next, hasChildren := node.(config.ChildProvider)
		if !hasChildren {
			return ""
		}
		current = next
	}
	return path.String()
}

// addRefusal appends d, an error diagnostic, and records the refusal it
// reports.
func (r *validationResult) addRefusal(d diagnostic.Diagnostic, refusal *configyang.ValidationError) {
	r.Valid = false
	if r.refusals == nil {
		r.refusals = make(map[int]*configyang.ValidationError)
	}
	r.refusals[len(r.Diagnostics)] = refusal
	r.Diagnostics = append(r.Diagnostics, d)
}

func (r *validationResult) addError(code, message string) {
	r.Valid = false
	r.Diagnostics = append(r.Diagnostics, diagnostic.Diagnostic{
		Code: code, Severity: diagnostic.SeverityError, Message: message,
	})
}

func (r *validationResult) addErrorLine(code, message string, line int) {
	r.Valid = false
	r.Diagnostics = append(r.Diagnostics, diagnostic.Diagnostic{
		Code: code, Severity: diagnostic.SeverityError, Message: message, Line: line,
	})
}

func (r *validationResult) addErrorWithRepair(code, message string, repair *diagnostic.Repair, safety diagnostic.FixSafety) {
	r.Valid = false
	r.Diagnostics = append(r.Diagnostics, diagnostic.Diagnostic{
		Code: code, Severity: diagnostic.SeverityError, Message: message,
		Repair: repair, FixSafety: safety,
	})
}

func (r *validationResult) addWarning(code, message string) { //nolint:unparam // code varies as more warning codes are added
	r.Diagnostics = append(r.Diagnostics, diagnostic.Diagnostic{
		Code: code, Severity: diagnostic.SeverityWarning, Message: message,
	})
}

// validationSummary shows what was parsed.
type validationSummary struct {
	RouterID    string        `json:"router-id"`
	LocalAS     uint32        `json:"local-as"`
	Listen      string        `json:"listen,omitempty"`
	Peers       int           `json:"peers"`
	Plugins     int           `json:"plugins"`
	PeerDetails []peerSummary `json:"peer-details,omitempty"`
	// IncompletePeers names the configured peers the daemon skips, each with
	// the parser's reason. They carry no peerSummary, whose zero AS and
	// address would read as configured values.
	IncompletePeers []incompletePeer `json:"incomplete-peers,omitempty"`
}

// incompletePeer is one peer the daemon skips (infra.BGPIncompletePeer).
type incompletePeer struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// peerSummary is one peer as the daemon builds it (infra.BGPPeerSummary).
type peerSummary struct {
	Name    string `json:"name"`
	Group   string `json:"group,omitempty"`
	Address string `json:"address"`
	PeerAS  uint32 `json:"peer-as"`
	LocalAS uint32 `json:"local-as"`
	Connect bool   `json:"connect"`
	Accept  bool   `json:"accept"`
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("config validate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	quiet := fs.Bool("q", false, "quiet mode (exit code only)")
	pending := fs.Bool("pending", false, "validate pending draft config")
	limit := fs.String("limit", "", "limit validation to section (environment)")
	user := fs.String("user", "", "account that asks the running ze about a config that declares an external plugin")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config validate",
			ShortHelp: "Validate a ze configuration file",
			Usage:     []string{"ze config validate [options] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "-v", Desc: "Verbose output"},
					{Name: "-q", Desc: "Quiet mode (exit code only)"},
					{Name: "--pending", Desc: "Validate pending draft config"},
					{Name: "--limit <section>", Desc: "Limit validation to section (environment)"},
					accountOptionHelp(),
				}},
				{Title: "Limit values", Entries: []helpfmt.HelpEntry{
					{Name: "environment", Desc: "Validate environment variables only (no config file needed)"},
				}},
				{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "Configuration is valid"},
					{Name: "1", Desc: "Configuration has errors"},
					{Name: "2", Desc: "File not found or unreadable"},
				}},
			},
			Examples: []string{
				"ze config validate config.conf",
				"ze config validate -v config.conf       # verbose output",
				"ze config validate -q config.conf       # quiet mode",
				"ze config validate --limit environment  # validate env vars only",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Handle --limit environment (no file needed).
	if *limit == "environment" {
		return validateEnvironment(*quiet)
	}

	if *limit != "" {
		fmt.Fprintf(os.Stderr, "error: unknown --limit value: %s (valid: environment)\n", *limit)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	if *pending {
		if configPath == "-" {
			fmt.Fprintf(os.Stderr, "error: --pending cannot read from stdin\n")
			return 1
		}
		return validatePending(configPath, *user, *verbose, *quiet)
	}

	// Read file (or stdin if "-").
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 2
	}

	// The running ze judges a config that declares an external plugin, and
	// this process judges every other config and any config no ze answered
	// for (judgeConfig).
	result := judgeConfig(string(data), configPath, *user, askOverSSH)

	return outputValidateText(result, *verbose, *quiet)
}

// validateEnvironment retains the quiet local form while sharing the daemon's
// actual environment verdict. Only the process answering the command is judged.
func validateEnvironment(quiet bool) int {
	answer := environmentValidationAnswer(command.ValidatedArgs{})
	if quiet {
		return answer.Code
	}
	return command.PrintLocalAnswerWith("show config environment-validation", answer, func(_ string, payload any) int {
		result, ok := payload.(diagnostic.ValidateResult)
		if !ok {
			panic("BUG: environment validation answered something other than its verdict")
		}
		if result.Valid {
			fmt.Println("Environment variables valid")
		}
		return exitOK
	})
}

func validatePending(configPath, user string, verbose, quiet bool) int {
	data, selected, err := readValidationSource(command.RouteLocal, nil, configPath, true)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "error: no pending config: %v\n", err)
		}
		return 2
	}

	result := judgeConfig(string(data), selected, user, askOverSSH)
	return outputValidateText(result, verbose, quiet)
}

// runValidation judges input, the config named path, in this process, for a
// caller that asked no running ze about it.
func runValidation(input, path string) *validationResult {
	return runValidationUnanswered(input, path, nil)
}

// runValidationUnanswered judges input, the config named path, in this
// process. unanswered is why no running ze judged it, for the one caller that
// asked one first (judgeConfig), and nil for a caller that asked none: the
// refusal of a name only an external plugin's schema could define says which
// of the two happened.
func runValidationUnanswered(input, path string, unanswered *config.DaemonUnanswered) *validationResult {
	result := &validationResult{
		Path:  path,
		Valid: true,
	}

	// Parse with the YANG-derived schema of this process's module set
	// (config.SchemaWithDeclaredPlugins). With no daemon that is the
	// compiled-in set: ze starts no external plugin here, so a section only
	// such a plugin's schema defines is refused below, with the command that
	// gets a running ze to check it. Served by a daemon, it is the set the
	// daemon joined from its plugins when it started.
	schema, declared, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: input, Path: path})
	if err != nil {
		result.addError("config-parse", "YANG schema: "+err.Error())
		return result
	}
	if unanswered != nil {
		declared = declared.Unanswered(*unanswered)
	}
	// Match the loader's format detection (ParseTreeWithYANG): a config file may
	// be hierarchical (block) or the flat set-command form the first-boot
	// bootstrap writes via EmitSetConfigWithDHCP. Validating only the block
	// grammar rejected every set-command file with "unknown top-level keyword:
	// set", so `ze config validate` disagreed with what `ze start` actually loads.
	var tree *config.Tree
	var warnings []string
	var parseErrors []error
	switch format := config.DetectFormat(input); format {
	case config.FormatSet, config.FormatSetMeta:
		// The set parser's own refusal, like the hierarchical arm's: the
		// loader's "parse config" segment belongs to daemon start and reload,
		// so `line N: ` is followed by the path directly (AC-24).
		tree, parseErrors = config.ParseSetDiagnostics(schema, input, format)
	case config.FormatHierarchical:
		p := config.NewParser(schema)
		tree, parseErrors = p.ParseDiagnostics(input)
		if len(parseErrors) == 0 {
			warnings = p.Warnings()
		}
	default:
		panic("BUG: unknown detected config format")
	}
	for _, err := range parseErrors {
		// The parser names the line once (config.LineError); the diagnostic
		// holds it in its own field and every printer writes it back, so the
		// message is the refusal with that line left out.
		line, message := config.SplitLine(err)
		// An enum, range, length or pattern refusal carries the walk's code
		// and typed fields, so a script reads a parser refusal and a walk
		// refusal of one leaf the same way (AC-11).
		if refusal, ok := errors.AsType[*configyang.ValidationError](err); ok && refusal.HoldsOperatorError() {
			result.addRefusal(refusalDiagnostic(schema, refusal, message, line), refusal)
			continue
		}
		// A node the schema carries only on another OS (ze:os) is a verdict
		// about this host, not about the config, so it carries its own code.
		code := "config-parse"
		if _, ok := errors.AsType[*config.NodeOfAnotherOSError](err); ok {
			code = diagnostic.CodeConfigNodeOtherOS
		}
		result.addErrorLine(code, message, line)
	}
	if len(parseErrors) > 0 {
		// A section only an external plugin's schema defines is an unknown
		// keyword when ze holds no schema of that plugin, and the parser's
		// refusal cannot name the plugin. One more diagnostic says which
		// external plugins the config declares, why ze holds no schema of
		// them here, and the command that gets a running ze to check it: the
		// fix of the failed ask when one was asked first (judgeConfig), and
		// `ze config validate <file>` for a caller that asked none.
		// Only beside the refusal of a name the schema does not define: no
		// plugin's missing schema explains any other refusal.
		unknownNode := false
		for _, parseErr := range parseErrors {
			if config.RefusesUnknownNode(parseErr) {
				unknownNode = true
			}
		}
		if unknownNode {
			if unchecked := declared.Unchecked(path); unchecked != nil {
				result.addError("config-parse", unchecked.Error())
			}
		}
		return result
	}

	// Surface parser warnings (e.g., inactive: prefix on leaf nodes).
	for _, w := range warnings {
		result.addWarning("config-warning", w)
	}
	// A declared external plugin ze holds no schema of refuses nothing by
	// itself and is not reported here: the config holds no name only its
	// schema defines, or the parse above would have refused.

	// Fail closed if a secret leaf carries the display placeholder: a masked
	// `show config` pasted into a file (or a web upload) must not clobber the
	// stored secret with the placeholder. The guard reads config.LeafHoldsSecret,
	// so it covers a ze:sensitive leaf as well as a ze:bcrypt one. This guards
	// the web upload path (ValidateContent) and `ze config validate` at once.
	if maskErr := config.RejectMaskedSecretLeaves(tree, schema); maskErr != nil {
		result.addError("config-secret-masked", maskErr.Error())
	}

	// Prune inactive nodes before resolution so the validation summary
	// reflects only active config (inactive peers are not started).
	config.PruneInactive(tree, schema)

	// YANG tree validation: cardinality, patterns, ranges, mandatory fields and
	// the ze:validate custom validators, over the section list it owns. BGP has
	// its own deeper path below.
	//
	// The walk itself lives in the config package because LoadConfig runs it
	// too. One walk means `ze config validate` and the daemon cannot reach
	// different verdicts on the same bytes, which is what the upgrade note in
	// docs/guide/configuration.md promises an operator.
	failures, walkErr := config.ValidateCustomSections(tree)
	if walkErr != nil {
		// The old code skipped the whole walk when the validator would not
		// build, and reported the config valid. A guard that cannot run must
		// say so, not wave the config through.
		result.addError("config-validator-unavailable", walkErr.Error())
	}
	for _, f := range failures {
		ve := f.Err
		var tb textbuf.Buffer
		d := diagnostic.Diagnostic{
			Code:    yangErrorCode(ve.Type),
			Message: f.Message(),
			Line:    ve.LineNumber,
		}
		if ve.Path != "" {
			d.Path = tb.Reset().Str(f.Section).Byte('.').Str(ve.Path).String()
		}
		if ve.Expected != "" {
			d.Expected = ve.Expected
		}
		if ve.Got != "" && !f.Sensitive {
			d.Actual = ve.Got
		}
		if r, s := yangRepair(ve.Type); r != nil {
			d.Repair = r
			d.FixSafety = s
		}
		// Every failure refuses, a missing mandatory node included: LoadConfig
		// refuses the same set, so this verdict is the daemon's.
		d.Severity = diagnostic.SeverityError
		// A refusal is rendered again by ValidateContent only where it masks
		// a sensitive value itself, the rule SectionValidationError.Message
		// applies; otherwise the redacted message stands.
		if ve.HoldsOperatorError() && (ve.Secret() || !f.Sensitive) {
			result.addRefusal(d, &ve)
			continue
		}
		result.Valid = false
		result.Diagnostics = append(result.Diagnostics, d)
	}

	// The kernel capability gate, the same enrolment ze doctor renders and the
	// daemon start refuses on (internal/component/kernelcap). One registry
	// answers all three, so a config that validates here starts there.
	//
	// This verdict is about THIS host. A config written for another machine and
	// validated on a workstation is answered against the workstation, which is
	// why the gate is not inside LoadConfig: that loader also runs under
	// `ze doctor` and under offline validation, where a host verdict would be
	// wrong. `ze config validate` carries it deliberately, and
	// docs/guide/operations.md says so.
	if capErr := kernelcap.Refuse(tree); capErr != nil {
		result.addError("config-kernel-capability", capErr.Error())
	}

	// MCP semantic validation. The same check runs in config.ValidateSemantics
	// for the `ze doctor` surface.
	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		if verr := mcpCfg.Validate(); verr != nil {
			result.addError("config-mcp-invalid", verr.Error())
		}
	}

	// gNMI semantic validation: a non-loopback listener needs a token and TLS.
	// The daemon refuses to boot without them (config.GNMIExposure, the rule the
	// boot guard applies), so validation reports each exposure, with its reason
	// and fix, rather than let the operator find out at start.
	if gnmiCfg, ok := config.ExtractGNMIConfig(tree); ok {
		for _, refusal := range gnmiCfg.Refusals() {
			result.Valid = false
			result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
				Code:     "config-gnmi-invalid",
				Severity: diagnostic.SeverityError,
				Message:  refusal.Problem(),
				Help:     refusal.Why(),
				Fix:      refusal.Fix(),
			})
		}
	}

	// Generic plugin config verification. VerifyPluginConfig also runs the
	// loader-level plugin extraction (ExtractPluginsFromTree), the same static
	// structural checks the boot path enforces (internal plugin requires `use`,
	// duplicate plugin names across internal/external, reserved underscore
	// prefix, run/use mutual exclusion), and answers its failure once as a
	// *config.PluginExtractionError, reported here under its own code. Skipping
	// it gave false confidence: configs that fail at boot were passing
	// validation. A failure relayed from the bgp section's extractor is skipped:
	// it is the bgp tree failing to resolve, which the bgp block below reports
	// from infra.ResolveBGPTree whenever a bgp {} block is present, the only
	// case in which that extractor runs.
	for _, verr := range config.VerifyPluginConfig(tree) {
		var extractErr *config.PluginExtractionError
		if !errors.As(verr, &extractErr) {
			// A verifier that marks its refusal a verdict about this host
			// (no OS default backend, no hardware inventory) is reported
			// under its own code, so it is never read as a config defect.
			code := "config-plugin-verify"
			if diagnostic.IsHostVerdict(verr) {
				code = diagnostic.CodeConfigHostBackend
			}
			result.addError(code, verr.Error())
			continue
		}
		if extractErr.Section == string(config.ConfigTypeBGP) {
			continue
		}
		result.addError("config-plugin-extract", extractErr.Error())
	}

	// BGP-specific validation only when bgp {} is present.
	if tree.GetContainer("bgp") != nil {
		// ResolveBGPTree fails closed when a bgp{} block is present but no
		// engine is compiled in, so that case surfaces here as a resolve error
		// rather than as a config validated against nothing.
		bgpTree, resolveErr := infra.ResolveBGPTree(tree)
		if resolveErr != nil {
			result.addError("config-bgp-resolve", resolveErr.Error())
			return result
		}

		addSemanticWarnings(result, bgpTree)

		if authzErr := infra.ValidateAuthzConfig(tree); authzErr != nil {
			result.addErrorWithRepair("config-bgp-authz", authzErr.Error(),
				&diagnostic.Repair{ID: "fix-peer-reference", Summary: "Correct the authz profile reference"}, diagnostic.SafetyRequiresHumanReview)
			return result
		}

		// The peer walk explains both external RFC8212 rejection and ordinary
		// internal learned-export rejection. The graph, not a plugin name,
		// determines the internal recipient whose missing policy is reported.
		absences, peersErr := infra.ValidateBGPPeersWithPolicy(tree)
		if peersErr != nil {
			result.addErrorWithRepair("config-bgp-peer", peersErr.Error(),
				&diagnostic.Repair{ID: "fix-peer-reference", Summary: "Correct peer settings or cross-reference"}, diagnostic.SafetyRequiresHumanReview)
			return result
		}
		for _, a := range absences {
			addRFC8212Warning(result, a)
		}

		// The summary is built after the peer walk accepted the config, from
		// the peers that walk's parser builds, so it shows what the daemon runs.
		peers, summaryErr := infra.SummarizeBGPPeers(tree)
		if summaryErr != nil {
			result.addErrorWithRepair("config-bgp-peer", summaryErr.Error(),
				&diagnostic.Repair{ID: "fix-peer-reference", Summary: "Correct peer settings or cross-reference"}, diagnostic.SafetyRequiresHumanReview)
			return result
		}
		result.Config = buildValidationSummary(bgpTree, peers, tree)
	}

	// Generic ze:required enforcement for non-BGP sections.
	// BGP is handled above via CheckRequiredFields (inheritance-aware).
	if config.HasNonBGPRequired(schema) {
		nonBGPData := tree.ToMap()
		delete(nonBGPData, "bgp")
		for _, v := range config.CheckRequired(schema, nonBGPData) {
			result.addWarning("config-required-missing",
				v.AnchorPath+" "+v.EntryKey+": missing required field \""+v.FieldPath+"\" ("+v.SetHint+")")
		}
	}

	// Hub config validation (secret length, client blocks).
	if tree.GetContainer("plugin") != nil {
		if _, hubErr := config.ExtractHubConfig(tree); hubErr != nil {
			result.addError("config-hub-invalid", hubErr.Error())
			return result
		}
	}

	// Listener port conflict detection.
	listeners := config.CollectListeners(tree, schema)
	if c := config.FindListenerConflict(listeners); c != nil {
		d := diagnostic.Diagnostic{
			Code:      "config-listener-conflict",
			Severity:  diagnostic.SeverityError,
			Message:   c.Err.Error(),
			Repair:    &diagnostic.Repair{ID: "resolve-listener-conflict", Summary: "Change address or port on one of the conflicting listeners"},
			FixSafety: diagnostic.SafetyRequiresHumanReview,
			Related:   listenerConflictRelated(c),
		}
		result.Valid = false
		result.Diagnostics = append(result.Diagnostics, d)
		return result
	}

	return result
}

// buildValidationSummary extracts a validationSummary from the resolved BGP tree
// and the peers the daemon builds from it (infra.SummarizeBGPPeers). The peer
// lines come from the engine's own parser, never from the resolved leaves: a
// reader here that guessed the paths printed an empty address and AS0 for
// every peer.
func buildValidationSummary(bgpTree map[string]any, peers infra.BGPPeerSummaries, tree *config.Tree) *validationSummary {
	summary := &validationSummary{}

	if rid, ok := bgpTree["router-id"]; ok {
		summary.RouterID = fmt.Sprint(rid)
	}
	summary.LocalAS = asnFromSession(bgpTree, "local")
	if listen, ok := bgpTree["listen"]; ok {
		summary.Listen = fmt.Sprint(listen)
	}

	summary.Peers = len(peers.Peers) + len(peers.Incomplete)
	for _, p := range peers.Peers {
		summary.PeerDetails = append(summary.PeerDetails, peerSummary{
			Name:    p.Name,
			Group:   p.Group,
			Address: p.Address,
			PeerAS:  p.PeerAS,
			LocalAS: p.LocalAS,
			Connect: p.Connect,
			Accept:  p.Accept,
		})
	}
	for _, p := range peers.Incomplete {
		summary.IncompletePeers = append(summary.IncompletePeers, incompletePeer{Name: p.Name, Reason: p.Reason})
	}

	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		// Count both external (subprocess) and internal (in-process) plugins;
		// counting only "external" undercounts configs that use the `internal` keyword.
		summary.Plugins = len(pluginContainer.GetList("external")) + len(pluginContainer.GetList("internal"))
	}

	return summary
}

// treeUint32 parses a tree value (string) as uint32. Returns 0 on nil or error.
func treeUint32(v any) uint32 {
	if v == nil {
		return 0
	}
	n, err := strconv.ParseUint(fmt.Sprint(v), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n) //nolint:gosec // Validated by ParseUint with bitSize 32
}

// asnFromSession reads session/asn/<leaf> from a bgp-global or peer subtree.
// The AS numbers live under session { asn { local; remote } } in the config
// tree (see internal/component/bgp/reactor/config.go), not under bare
// local/remote containers.
func asnFromSession(tree map[string]any, leaf string) uint32 {
	sessionMap, ok := tree["session"].(map[string]any)
	if !ok {
		return 0
	}
	asnMap, ok := sessionMap["asn"].(map[string]any)
	if !ok {
		return 0
	}
	return treeUint32(asnMap[leaf])
}

func listenerConflictRelated(c *config.ListenerConflict) []diagnostic.Related {
	format := func(ep config.ListenerEndpoint) string {
		var tb textbuf.Buffer
		return tb.Str(config.ProtocolLabel(ep.Protocol)).Byte(' ').Str(ep.IP.String()).Byte(':').Uint16(ep.Port).String()
	}
	return []diagnostic.Related{
		{Path: c.A.Service, Message: format(c.A)},
		{Path: c.B.Service, Message: format(c.B)},
	}
}

func yangRepair(t configyang.ErrorType) (*diagnostic.Repair, diagnostic.FixSafety) {
	// Only errors with a known repair contribute repair metadata.
	//exhaustive:ignore
	switch t {
	case configyang.ErrTypeMissing:
		return &diagnostic.Repair{ID: "add-missing-field", Summary: "Insert mandatory field with its default or required value"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypeType:
		return &diagnostic.Repair{ID: "fix-type-mismatch", Summary: "Replace value with one matching the expected type"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypeEnum:
		return &diagnostic.Repair{ID: "fix-type-mismatch", Summary: "Replace value with one of the allowed enum values"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypeRange:
		return &diagnostic.Repair{ID: "fix-range-value", Summary: "Adjust value to fall within the allowed range"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypeLength:
		return &diagnostic.Repair{ID: "fix-range-value", Summary: "Adjust string length to fall within the allowed range"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypePattern:
		return &diagnostic.Repair{ID: "fix-pattern-value", Summary: "Correct value to match the required pattern"}, diagnostic.SafetySectionLocal
	case configyang.ErrTypeReference:
		return &diagnostic.Repair{ID: "fix-reference", Summary: "Configure the node the reference names, or name one that exists"}, diagnostic.SafetySectionLocal
	default:
		return nil, ""
	}
}

func yangErrorCode(t configyang.ErrorType) string {
	switch t {
	case configyang.ErrTypeMissing:
		return diagnostic.CodeConfigYANGMissing
	case configyang.ErrTypeType:
		return diagnostic.CodeConfigYANGType
	case configyang.ErrTypeRange:
		return diagnostic.CodeConfigYANGRange
	case configyang.ErrTypePattern:
		return diagnostic.CodeConfigYANGPattern
	case configyang.ErrTypeEnum:
		return diagnostic.CodeConfigYANGEnum
	case configyang.ErrTypeLength:
		return diagnostic.CodeConfigYANGLength
	case configyang.ErrTypeCardinality:
		return diagnostic.CodeConfigYANGCardinality
	case configyang.ErrTypeReference:
		return diagnostic.CodeConfigYANGReference
	case configyang.ErrTypeUnknown:
		return diagnostic.CodeConfigYANGType
	default:
		panic("BUG: unknown YANG diagnostic error type")
	}
}

// rfc8212WarningCode is the diagnostic code of an RFC 8212 policy absence.
const rfc8212WarningCode = "config-bgp-rfc8212-no-policy"

// addRFC8212Warning reports one policy absence in the daemon's words. The
// code distinguishes ordinary learned export from RFC8212 rejection. The editor
// command gives the direction an explicit policy. `final accept` is only an
// example: which routes to accept is the operator's decision.
func addRFC8212Warning(result *validationResult, a infra.BGPPolicyAbsence) {
	code := rfc8212WarningCode
	if a.Code == diagnostic.CodeDoctorBGPLearnedExportNoPolicy {
		code = a.Code
	}
	var tb textbuf.Buffer
	tb.Str(a.Owner).Str(": ").Str(a.Message).Str("; to exchange routes, run `").Str(a.Fix).Str("`, then `commit now`")
	result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
		Code:      code,
		Severity:  diagnostic.SeverityWarning,
		Message:   tb.String(),
		Help:      a.Fix,
		Fix:       a.Fix,
		Repair:    &diagnostic.Repair{ID: "configure-" + a.Direction + "-policy", Summary: "Give the " + a.Direction + " direction an explicit policy"},
		FixSafety: diagnostic.SafetyRequiresHumanReview,
	})
}

// addSemanticWarnings carries no router-id warning: bgp/router-id is YANG
// mandatory (RFC 7950 Section 7.6.5), so a config without it, global or on
// every session, is refused as an error before it reaches this point.
func addSemanticWarnings(result *validationResult, bgpTree map[string]any) {
	globalLocalAS := asnFromSession(bgpTree, "local")
	if globalLocalAS == 0 {
		result.addWarning("config-warning", "local-as not configured globally")
	}

	peers, _ := bgpTree["peer"].(map[string]any)
	for name, v := range peers {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		peerLocalAS := asnFromSession(peer, "local")
		if peerLocalAS == 0 && globalLocalAS == 0 {
			result.addWarning("config-warning", "peer "+name+": local-as not configured")
		}
		peerAS := asnFromSession(peer, "remote")
		if peerAS == 0 {
			result.addWarning("config-warning", "peer "+name+": remote as not configured")
		}
		if timerMap, ok := peer["timer"].(map[string]any); ok {
			holdTime := treeUint32(timerMap["receive-hold-time"])
			if holdTime > 0 && holdTime < 3 {
				result.addWarning("config-warning", "peer "+name+": receive-hold-time too low (minimum 3)")
			}
			sendHoldTime := treeUint32(timerMap["send-hold-time"])
			if sendHoldTime > 0 && sendHoldTime < 480 {
				result.addWarning("config-warning", "peer "+name+": send-hold-time too low (RFC 9687 minimum 480)")
			}
		}
	}
}

//nolint:errcheck // CLI text output to stdout is fire-and-forget
func outputValidateText(result *validationResult, verbose, quiet bool) int {
	if quiet {
		if result.Valid {
			return 0
		}
		return 1
	}

	if result.Valid {
		fmt.Fprintf(os.Stdout, "configuration valid: %s\n", result.Path) //nolint:errcheck // output

		if verbose && result.Config != nil {
			fmt.Fprintln(os.Stdout)                           //nolint:errcheck // output
			fmt.Fprintln(os.Stdout, "Configuration summary:") //nolint:errcheck // output
			if result.Config.RouterID != "" {
				fmt.Fprintf(os.Stdout, "  router-id: %s\n", result.Config.RouterID) //nolint:errcheck // output
			}
			if result.Config.LocalAS != 0 {
				fmt.Fprintf(os.Stdout, "  local-as:  %d\n", result.Config.LocalAS) //nolint:errcheck // output
			}
			if result.Config.Listen != "" {
				fmt.Fprintf(os.Stdout, "  listen:    %s\n", result.Config.Listen) //nolint:errcheck // output
			}
			fmt.Fprintf(os.Stdout, "  peers: %d\n", result.Config.Peers)     //nolint:errcheck // output
			fmt.Fprintf(os.Stdout, "  plugins: %d\n", result.Config.Plugins) //nolint:errcheck // output

			if len(result.Config.PeerDetails)+len(result.Config.IncompletePeers) > 0 {
				fmt.Fprintln(os.Stdout)           //nolint:errcheck // output
				fmt.Fprintln(os.Stdout, "Peers:") //nolint:errcheck // output
				for _, n := range result.Config.PeerDetails {
					fmt.Fprintln(os.Stdout, peerSummaryLine(n)) //nolint:errcheck // output
				}
				for _, p := range result.Config.IncompletePeers {
					fmt.Fprintf(os.Stdout, "  - %s: incomplete, the daemon does not start it: %s\n", p.Name, p.Reason) //nolint:errcheck // output
				}
			}
		}

		warnings := diagsBySeverity(result.Diagnostics, diagnostic.SeverityWarning)
		if len(warnings) > 0 {
			fmt.Fprintln(os.Stdout)              //nolint:errcheck // output
			fmt.Fprintln(os.Stdout, "Warnings:") //nolint:errcheck // output
			for i := range warnings {
				fmt.Fprintf(os.Stdout, "  warning: %s\n", warnings[i].Message) //nolint:errcheck // output
			}
		}

		return 0
	}

	fmt.Fprintf(os.Stdout, "configuration invalid: %s\n", result.Path) //nolint:errcheck // output
	fmt.Fprintln(os.Stdout)                                            //nolint:errcheck // output
	fmt.Fprintln(os.Stdout, "Errors:")                                 //nolint:errcheck // output
	errs := diagsBySeverity(result.Diagnostics, diagnostic.SeverityError)
	for i := range errs {
		if errs[i].Line > 0 {
			fmt.Fprintf(os.Stdout, "  line %d: %s\n", errs[i].Line, errs[i].Message) //nolint:errcheck // output
		} else {
			fmt.Fprintf(os.Stdout, "  %s\n", errs[i].Message) //nolint:errcheck // output
		}
	}
	return 1
}

// peerSummaryLine renders one built peer: its name, address and remote AS,
// then its group and any disabled connection direction.
func peerSummaryLine(n peerSummary) string {
	var tb textbuf.Buffer
	tb.Str("  - ").Str(n.Name).Byte(' ').Str(n.Address).Str(" AS").Uint32(n.PeerAS)
	if n.Group != "" {
		tb.Str(" group ").Str(n.Group)
	}
	var flags []string
	if !n.Connect {
		flags = append(flags, "connect disabled")
	}
	if !n.Accept {
		flags = append(flags, "accept disabled")
	}
	if len(flags) > 0 {
		tb.Str(" (").Str(textbuf.Join(flags, ", ")).Byte(')')
	}
	return tb.String()
}

func diagsBySeverity(diags []diagnostic.Diagnostic, sev diagnostic.Severity) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Severity == sev {
			out = append(out, diags[i])
		}
	}
	return out
}
