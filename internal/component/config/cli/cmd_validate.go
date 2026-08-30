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

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidateContent runs the same validation as `ze config validate` and returns
// an error containing all validation errors. Warnings do not fail validation,
// matching the CLI command exit-code semantics.
func ValidateContent(input, path string) error {
	result := runValidation(input, path)
	if result.Valid {
		return nil
	}
	var b textbuf.Buffer
	b.Str("config validation failed:")
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		if d.Severity != diagnostic.SeverityError {
			continue
		}
		b.Str("\n  ")
		if d.Line > 0 {
			b.Str("line ").Int(int64(d.Line)).Str(": ")
		}
		b.Str(d.Message)
	}
	return errors.New(b.String())
}

// validationResult holds validation results with structured diagnostics.
type validationResult struct {
	Valid       bool
	Path        string
	Diagnostics []diagnostic.Diagnostic
	Config      *validationSummary
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
}

// peerSummary shows peer details.
type peerSummary struct {
	Address string `json:"address"`
	PeerAS  uint32 `json:"peer-as"`
	Connect bool   `json:"connect"`
	Accept  bool   `json:"accept"`
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("config validate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	quiet := fs.Bool("q", false, "quiet mode (exit code only)")
	pending := fs.Bool("pending", false, "validate pending draft config")
	limit := fs.String("limit", "", "limit validation to section (environment)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config validate",
			Summary: "Validate a ze configuration file",
			Usage:   []string{"ze config validate [options] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "-v", Desc: "Verbose output"},
					{Name: "-q", Desc: "Quiet mode (exit code only)"},
					{Name: "--pending", Desc: "Validate pending draft config"},
					{Name: "--limit <section>", Desc: "Limit validation to section (environment)"},
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
		return validatePending(configPath, *verbose, *quiet)
	}

	// Read file (or stdin if "-").
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 2
	}

	// Parse and validate.
	result := runValidation(string(data), configPath)

	return outputValidateText(result, *verbose, *quiet)
}

// validateEnvironment reports "valid" when invoked with --limit environment.
// Per-value strict validation lived in the deleted Environment struct; env vars
// are now validated lazily at consumer call sites (env.GetInt/GetDuration
// return the default for unparseable values; YANG restrictions on the
// `environment/` block are enforced at config parse time).
func validateEnvironment(quiet bool) int {
	if !quiet {
		fmt.Println("Environment variables valid")
	}
	return 0
}

func validatePending(configPath string, verbose, quiet bool) int {
	var tb textbuf.Buffer
	draftPath := tb.Str(configPath).Str(".draft").String()
	data, err := os.ReadFile(draftPath) //nolint:gosec // Config path from CLI args
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "error: no pending config: %v\n", err)
		}
		return 2
	}

	result := runValidation(string(data), configPath)
	return outputValidateText(result, verbose, quiet)
}

func runValidation(input, path string) *validationResult {
	result := &validationResult{
		Path:  path,
		Valid: true,
	}

	// Parse with YANG-derived schema.
	schema, err := config.YANGSchema()
	if err != nil {
		result.addError("config-parse", "YANG schema: "+err.Error())
		return result
	}
	// Match the loader's format detection (ParseTreeWithYANG): a config file may
	// be hierarchical (block) or the flat set-command form the first-boot
	// bootstrap writes via EmitSetConfigWithDHCP. Validating only the block
	// grammar rejected every set-command file with "unknown top-level keyword:
	// set", so `ze config validate` disagreed with what `ze start` actually loads.
	var tree *config.Tree
	var warnings []string
	switch config.DetectFormat(input) {
	case config.FormatSet, config.FormatSetMeta:
		tree, err = config.ParseTreeForValidation(input)
	default:
		p := config.NewParser(schema)
		tree, err = p.Parse(input)
		if err == nil {
			warnings = p.Warnings()
		}
	}
	if err != nil {
		result.addErrorLine("config-parse", err.Error(), extractLine(err.Error()))
		return result
	}

	// Surface parser warnings (e.g., inactive: prefix on leaf nodes).
	for _, w := range warnings {
		result.addWarning("config-warning", w)
	}

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
		if f.Blocking() {
			d.Severity = diagnostic.SeverityError
			result.Valid = false
		} else {
			d.Severity = diagnostic.SeverityWarning
		}
		result.Diagnostics = append(result.Diagnostics, d)
	}

	// MCP semantic validation. The same check runs in config.ValidateSemantics
	// for the `ze doctor` surface.
	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		if verr := mcpCfg.Validate(); verr != nil {
			result.addError("config-mcp-invalid", verr.Error())
		}
	}

	// gNMI semantic validation: a non-loopback listener with no token accepts
	// unauthenticated Get and Set. The daemon refuses to boot on it, so
	// validation must report it rather than let the operator find out at start.
	if gnmiCfg, ok := config.ExtractGNMIConfig(tree); ok {
		if verr := gnmiCfg.Validate(); verr != nil {
			result.addError("config-gnmi-invalid", verr.Error())
		}
	}

	// Generic plugin config verification.
	for _, verr := range config.VerifyPluginConfig(tree) {
		result.addError("config-plugin-verify", verr.Error())
	}

	// Loader-level plugin extraction checks. ExtractPluginsFromTree runs the
	// same static structural checks the boot path enforces (internal plugin
	// requires `use`, duplicate plugin names across internal/external, reserved
	// underscore prefix, run/use mutual exclusion). It operates only on the
	// parsed tree (plus side-effect-free registered extractors), so it is safe
	// to run offline without a booted runtime. Skipping it gave false
	// confidence: configs that fail at boot were passing validation.
	if _, extractErr := config.ExtractPluginsFromTree(tree); extractErr != nil {
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

		result.Config = buildValidationSummary(bgpTree, tree)
		addSemanticWarnings(result, bgpTree)

		if authzErr := infra.ValidateAuthzConfig(tree); authzErr != nil {
			result.addErrorWithRepair("config-bgp-authz", authzErr.Error(),
				&diagnostic.Repair{ID: "fix-peer-reference", Summary: "Correct the authz profile reference"}, diagnostic.SafetyRequiresHumanReview)
			return result
		}

		if peersErr := infra.ValidateBGPPeers(tree); peersErr != nil {
			result.addErrorWithRepair("config-bgp-peer", peersErr.Error(),
				&diagnostic.Repair{ID: "fix-peer-reference", Summary: "Correct peer settings or cross-reference"}, diagnostic.SafetyRequiresHumanReview)
			return result
		}
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

// buildValidationSummary extracts a validationSummary from the resolved BGP tree.
func buildValidationSummary(bgpTree map[string]any, tree *config.Tree) *validationSummary {
	summary := &validationSummary{}

	if rid, ok := bgpTree["router-id"]; ok {
		summary.RouterID = fmt.Sprint(rid)
	}
	if localMap, ok := bgpTree["local"].(map[string]any); ok {
		summary.LocalAS = treeUint32(localMap["as"])
	}
	if listen, ok := bgpTree["listen"]; ok {
		summary.Listen = fmt.Sprint(listen)
	}

	if peers, ok := bgpTree["peer"].(map[string]any); ok {
		summary.Peers = len(peers)
		for name, v := range peers {
			peer, ok := v.(map[string]any)
			if !ok {
				continue
			}
			connect := true
			if remoteMap, ok := peer["remote"].(map[string]any); ok {
				if v, ok := remoteMap["connect"]; ok {
					if b, err := config.ParseBoolStrict(fmt.Sprint(v)); err == nil {
						connect = b
					}
				}
			}
			accept := true
			if localMap, ok := peer["local"].(map[string]any); ok {
				if v, ok := localMap["accept"]; ok {
					if b, err := config.ParseBoolStrict(fmt.Sprint(v)); err == nil {
						accept = b
					}
				}
			}
			var peerAS uint32
			var addr string
			if remoteMap, ok := peer["remote"].(map[string]any); ok {
				peerAS = treeUint32(remoteMap["as"])
				if ip, ok := remoteMap["ip"]; ok {
					addr = fmt.Sprint(ip)
				}
			}
			_ = name // peer name is the map key
			summary.PeerDetails = append(summary.PeerDetails, peerSummary{
				Address: addr,
				PeerAS:  peerAS,
				Connect: connect,
				Accept:  accept,
			})
		}
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
	switch t { //nolint:exhaustive // only codes with known repairs
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
	default:
		return nil, ""
	}
}

func yangErrorCode(t configyang.ErrorType) string {
	switch t { //nolint:exhaustive // default handles unknown
	case configyang.ErrTypeMissing:
		return "config-yang-missing"
	case configyang.ErrTypeType:
		return "config-yang-type"
	case configyang.ErrTypeRange:
		return "config-yang-range"
	case configyang.ErrTypePattern:
		return "config-yang-pattern"
	case configyang.ErrTypeEnum:
		return "config-yang-enum"
	case configyang.ErrTypeLength:
		return "config-yang-length"
	case configyang.ErrTypeCardinality:
		return "config-yang-cardinality"
	default:
		return "config-yang-type"
	}
}

func addSemanticWarnings(result *validationResult, bgpTree map[string]any) {
	if _, ok := bgpTree["router-id"]; !ok {
		result.addWarning("config-warning", "router-id not configured (will use system default)")
	}

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

			if len(result.Config.PeerDetails) > 0 {
				fmt.Fprintln(os.Stdout)           //nolint:errcheck // output
				fmt.Fprintln(os.Stdout, "Peers:") //nolint:errcheck // output
				for _, n := range result.Config.PeerDetails {
					var flags []string
					if !n.Connect {
						flags = append(flags, "connect disabled")
					}
					if !n.Accept {
						flags = append(flags, "accept disabled")
					}
					if len(flags) == 0 {
						fmt.Fprintf(os.Stdout, "  - %s AS%d\n", n.Address, n.PeerAS) //nolint:errcheck // output
					} else {
						fmt.Fprintf(os.Stdout, "  - %s AS%d (%s)\n", n.Address, n.PeerAS, textbuf.Join(flags, ", ")) //nolint:errcheck // output
					}
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

func diagsBySeverity(diags []diagnostic.Diagnostic, sev diagnostic.Severity) []diagnostic.Diagnostic {
	var out []diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Severity == sev {
			out = append(out, diags[i])
		}
	}
	return out
}

func extractLine(errMsg string) int {
	// Extract line number from "line N:" format.
	idx := strings.Index(errMsg, "line ")
	if idx < 0 {
		return 0
	}
	var line int
	// Best effort extraction - if it fails, return 0.
	if n, err := fmt.Sscanf(errMsg[idx:], "line %d", &line); n != 1 || err != nil {
		return 0
	}
	return line
}
