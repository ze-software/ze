// Design: docs/architecture/plugin/plugin-system.md -- the Stage 1 check table and its judge
// Related: startup_driver.go -- runStartupHandshake runs the driver-phase checks
// Related: startup.go -- onRegistration runs the sink-phase checks
// Related: ../declaration_judge.go -- the seam JudgeDeclaration fills
//
// stage1_checks.go holds the ONE ordered table of the checks a Stage 1
// declaration must pass. The daemon takes its checks from the table, and
// JudgeDeclaration runs the same entries for a program nobody started, so the
// daemon and a checker cannot disagree about what a valid declaration is.
//
// The validator functions stay where they were. The table refers to them and
// adds what the call sites used to carry: the order, the wrap prefix and the
// log line of each refusal.

package server

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// stage1Phase says where the daemon runs a check. The zero value is invalid,
// so an entry that names no phase is refused by the table's test.
type stage1Phase uint8

const (
	stage1PhaseUnspecified stage1Phase = iota
	// stage1PhaseDriver runs in runStartupHandshake, before the sink sees the
	// declaration.
	stage1PhaseDriver
	// stage1PhaseSink runs in onRegistration, before the declaration is
	// converted and stored.
	stage1PhaseSink
	// stage1PhaseInline is judged by its own statements in onRegistration,
	// after the declaration is converted: the check and the write it guards
	// are one step there, and each failure unwinds the writes above it.
	stage1PhaseInline
	// stage1PhaseReady is a rule the daemon enforces at Stage 5, where a
	// failure is a warning and a refused command, never a refused plugin.
	stage1PhaseReady
)

// sectionStage1 and sectionStage5 are the section titles of
// docs/plugin-development/protocol.md the checks enforce.
const (
	sectionStage1 = rpc.DeclarationSection
	sectionStage5 = "Stage 5: Ready (Plugin to Engine)"
)

// stage1Site is what the process that judges a declaration knows of it beyond
// the declaration and the plugin's name, for the isolated checks that read it.
// The daemon builds it from the operator's block and its command registry
// (engineStartupSink.site); a checker that runs one program in isolation holds
// that program alone (programAlone).
type stage1Site struct {
	// compiledIn says the operator's block runs a plugin compiled into ze,
	// whose YANG modules ze registered at init() and judges when ze is built.
	compiledIn bool
	// routing is what decides, beside the declaration, which plugin a command
	// line reaches: the builtin names and the commands other plugins
	// registered so far.
	routing commandRouting
}

// programAlone is the site of a checker that runs one external program and no
// daemon: the program is not compiled in, no builtin and no other plugin's
// command is known, so a check judges the declaration alone.
func programAlone() stage1Site {
	return stage1Site{}
}

// site answers what the daemon knows of the declaration of the sink's plugin:
// whether the operator's block runs a compiled-in plugin, and what the command
// registry holds beside the plugin now (CommandRegistry.routingBeside).
func (e *engineStartupSink) site() stage1Site {
	cfg := e.proc.Config()
	site := stage1Site{compiledIn: !cfg.RunsExternalProgram()}
	// A server with no dispatcher, or a dispatcher with no command registry,
	// routes no command: no builtin and no other plugin's command can take a
	// node from this plugin, so the site knows none.
	if e.s.dispatcher == nil {
		return site
	}
	held := e.s.dispatcher.Registry()
	if held == nil {
		return site
	}
	site.routing = held.routingBeside(cfg.Name)
	return site
}

// stage1Check is one check of a Stage 1 declaration.
//
// Exactly one of isolated, live and the inline phase says how the check is
// judged. isolated needs the declaration, the plugin's name and the site that
// judges it, which a checker holds as the program alone, so JudgeDeclaration
// runs it for a program in isolation. live needs daemon state, so only the
// daemon runs it, and daemonState says which state.
type stage1Check struct {
	// name is the kebab-case name a report prints.
	name string
	// section is the protocol.md section the check enforces.
	section string
	phase   stage1Phase
	// byRegistration says the driver names the plugin by the registration the
	// process runs (sink.registrationName), the name a filter reference
	// carries, and not by the operator's block (sink.pluginName).
	byRegistration bool
	// isolated judges the declaration for the plugin named plugin, at site,
	// and answers the validator's own error.
	isolated func(plugin string, input *rpc.DeclareRegistrationInput, site stage1Site) error
	// live judges the declaration against the daemon the sink belongs to.
	live func(e *engineStartupSink, input *rpc.DeclareRegistrationInput) error
	// daemonState says which daemon state a live or inline check reads. It is
	// the reason a checker gives for not judging the check.
	daemonState string
	// wrap adds the prefix the daemon's answer carries. Nil answers the
	// validator's error unchanged.
	wrap func(plugin string, err error) error
	// refusalLog is the message of the Error line the daemon logs when the
	// check refuses. Empty logs nothing.
	refusalLog string
}

// stage1Checks is the table, in the order the daemon runs it. The order is
// part of the contract: the first failed check is the reason the plugin reads.
var stage1Checks = []stage1Check{
	{
		// The version is negotiated before anything else: a text-era plugin
		// gets this one refusal and no other (D-2).
		name: "protocol-version", section: sectionStage1, phase: stage1PhaseDriver,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			_, err := rpc.NegotiateProtocolVersion(plugin, input.ProtocolVersion)
			return err
		},
	},
	{
		name: "dissemination", section: sectionStage1, phase: stage1PhaseDriver,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			if err := registry.CheckDisseminationDeclarations(plugin, input.Dissemination); err != nil {
				return err
			}
			return registry.CheckDisseminationEvents(plugin, input.Dissemination)
		},
	},
	{
		// The filters are judged before the sink stores them: a name the
		// vocabulary does not hold, or a chain the declared fields do not fit,
		// fails the plugin here (D-10). The filters, the insertions and the
		// references a loaded config makes to them are judged under the
		// registration the process runs, because the reference's prefix names
		// the plugin and never the operator's block.
		name: "filter-declarations", section: sectionStage1, phase: stage1PhaseDriver,
		byRegistration: true,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return registry.CheckFilterDeclarations(plugin, input.Filters)
		},
	},
	{
		// The auto-insertions are judged by the same static rules config load
		// applies to a compiled-in registration: a reference the plugin does
		// not serve, two directions, no condition, or a required one with no
		// reason fails the plugin here (spec-filter-wire-6-stage-filters AC-2).
		name: "filter-insertions", section: sectionStage1, phase: stage1PhaseDriver,
		byRegistration: true,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return registry.CheckFilterInsertions(plugin, input.Filters,
				registry.OwnedFilterTypes(plugin), input.FilterInsertions)
		},
	},
	{
		// Each condition path is judged against the compiled schema, and a
		// compiled-in plugin's against the config its own module adds (A-10).
		// The schema is what this binary compiles, so the check needs no
		// running daemon.
		name: "filter-insertion-paths", section: sectionStage1, phase: stage1PhaseDriver,
		byRegistration: true,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return config.CheckFilterInsertionPaths(plugin, registry.OwnedYANG(plugin),
				input.FilterInsertions)
		},
	},
	{
		// A chain reference names the plugin, so a second block running the
		// same plugin's filters would leave every reference to it ambiguous.
		name: "ambiguous-registration", section: sectionStage1, phase: stage1PhaseSink,
		live: func(e *engineStartupSink, input *rpc.DeclareRegistrationInput) error {
			return e.s.refuseAmbiguousRegistration(e.proc, input)
		},
		daemonState: "it compares the declaration with the other plugins the daemon runs",
	},
	{
		// The drain declaration is validated before conversion: a budget the
		// daemon could not honor is refused, never clamped.
		name: "drain", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateDrainDecl(plugin, input.DrainBudget, input.DrainHandler)
		},
	},
	{
		// A misspelled resolver cache would leave the plugin out of every
		// clear with nothing said.
		name: "resolver-caches", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateResolverCacheDecls(plugin, input.ResolverCaches)
		},
	},
	{
		name: "doctor-checks", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateDoctorCheckDecls(input.DoctorChecks)
		},
		wrap: func(_ string, err error) error {
			return fmt.Errorf("invalid doctor check: %w", err)
		},
	},
	{
		name: "doctor-check-declared-twice", section: sectionStage1, phase: stage1PhaseSink,
		live: func(e *engineStartupSink, input *rpc.DeclareRegistrationInput) error {
			return refuseDoctorCheckDeclaredTwice(e.proc.Config().Name,
				registryRow(e.proc.Config()), input.DoctorChecks)
		},
		daemonState: "it compares the declared doctor checks with the registry row the daemon holds for the operator's block",
	},
	{
		// The enricher declarations are validated before proxy registration.
		name: "enrichers", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateEnricherDecls(input.Enrichers)
		},
		wrap: func(_ string, err error) error {
			return fmt.Errorf("invalid enricher: %w", err)
		},
	},
	{
		// The pipe resolver reads the alias registry in this process. A
		// declaration that reaches that registry is live for every operator,
		// so a refusal happens before it. The commands travel with the pipes
		// because an alias may sit only on a command path this same message
		// declares.
		name: "pipe-aliases", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validatePipeDecls(input.Pipes, input.Commands)
		},
		wrap: func(_ string, err error) error {
			return fmt.Errorf("invalid pipe alias: %w", err)
		},
	},
	{
		// A shape decides which operators the command publishes and which it
		// refuses before dispatch, so a declaration is checked against a
		// closed set and a bound before any of it is stored.
		name: "answer-shapes", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateShapeDecls(input.Commands)
		},
		wrap: func(plugin string, err error) error {
			return fmt.Errorf("invalid answer shape: %s: %w", plugin, err)
		},
		refusalLog: "plugin answer shape refused",
	},
	{
		// The two declared help texts reach an operator's terminal, an HTML
		// page and a JSON document, so the bound and the control-character
		// refusal happen before the declaration is stored.
		name: "help-texts", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			return validateHelpDecls(input.Commands)
		},
		wrap: func(plugin string, err error) error {
			return fmt.Errorf("invalid help text: %s: %w", plugin, err)
		},
		refusalLog: "plugin help text refused",
	},
	{
		// The plugin has now said what its own failure means, and the
		// configuration said what the operator expects. This is the first
		// moment both are known, so it is where they are checked against each
		// other.
		name: "respawn-agreement", section: sectionStage1, phase: stage1PhaseSink,
		live: func(e *engineStartupSink, input *rpc.DeclareRegistrationInput) error {
			return refuseRespawnDisagreement(e.proc.Config().Name, e.proc.Config().Respawn,
				input.FailurePolicy)
		},
		daemonState: "it compares the declared failure policy with the respawn leaf of the operator's config",
		refusalLog:  "plugin respawn refused",
	},
	{
		// A plugin, and so the config schema it brings, changes only when ze
		// restarts (owner ruling): boot joined and published the module each
		// external plugin declared, and a plugin started again must declare
		// that module, or none when it declared none. The check logs and
		// raises its own refusal, under its own doctor code, so the entry
		// declares no log line.
		name: "schema-unchanged", section: sectionStage1, phase: stage1PhaseSink,
		live: func(e *engineStartupSink, input *rpc.DeclareRegistrationInput) error {
			return refuseChangedSchema(e.proc.Config(), input.Schemas)
		},
		daemonState: "it compares the declared YANG modules with the ones the daemon read from the plugin when it started",
	},
	{
		// Every ze:command node of a module the plugin brought is routed to a
		// command it declares, the rule the compiled-in modules are held to
		// when ze is built (refuseUnservedCommandNodes). The daemon reads the
		// modules it published, which schema-unchanged, just before, holds
		// equal to the declared ones, and the commands its registry holds for
		// other plugins; a checker joins the declared modules itself and knows
		// no other plugin, so it judges the declaration alone.
		name: "command-nodes-served", section: sectionStage1, phase: stage1PhaseSink,
		isolated: func(plugin string, input *rpc.DeclareRegistrationInput, site stage1Site) error {
			return refuseUnservedCommandNodes(plugin, input, site)
		},
		refusalLog: "plugin command module refused",
	},
	{
		name: "dependencies", section: sectionStage1, phase: stage1PhaseInline,
		daemonState: "it looks each declared dependency up among the plugins the daemon configured or loaded",
	},
	{
		name: "registration-conflicts", section: sectionStage1, phase: stage1PhaseInline,
		daemonState: "it writes the plugin's rows, pipe aliases, answer shapes and families into the daemon's registries, where another plugin may hold the same name",
	},
	{
		// CommandRegistry.Register applies the name rule at Stage 5. A bad
		// name there is a warning and a recorded command refusal, so the
		// daemon keeps the plugin.
		name: "command-names", section: sectionStage5, phase: stage1PhaseReady,
		isolated: func(_ string, input *rpc.DeclareRegistrationInput, _ stage1Site) error {
			for i := range input.Commands {
				if err := validateCommandName(input.Commands[i].Name); err != nil {
					return err
				}
			}
			return nil
		},
		wrap: func(_ string, err error) error {
			return fmt.Errorf("%w: the daemon keeps the plugin, logs a warning and refuses this command at Stage 5", err)
		},
	},
}

func init() {
	plugin.SetDeclarationJudge(JudgeDeclaration)
}

// refusal runs an isolated check for the plugin named name, at site, and
// answers the error the daemon relays to the plugin: the validator's error
// under the check's wrap prefix. logged says whether the refusal is also
// written to the daemon log, which the daemon asks for and a checker does not.
//
// MUST be called only for a check whose isolated function is set.
func (c *stage1Check) refusal(name string, input *rpc.DeclareRegistrationInput, site stage1Site, logged bool) error {
	return c.relayed(name, c.isolated(name, input, site), logged)
}

// relayed turns a validator's error into the daemon's answer: nil stays nil,
// and a refusal is logged when the check declares a log line and wrapped when
// it declares a prefix. The log line carries the validator's own error, as
// the call sites it replaces did.
func (c *stage1Check) relayed(name string, cause error, logged bool) error {
	if cause == nil {
		return nil
	}
	if logged {
		c.logRefusal(name, cause)
	}
	if c.wrap == nil {
		return cause
	}
	return c.wrap(name, cause)
}

func (c *stage1Check) logRefusal(name string, cause error) {
	if c.refusalLog == "" {
		return
	}
	// A refused declaration fails the startup handshake, and the plugin is
	// then recorded as stopped: that is the finding doctor reports for it.
	logger().Error(c.refusalLog, "plugin", name, "error", cause,
		"code", diagnostic.CodeDoctorPluginStopped)
}

// refuseDriverChecks runs the driver-phase checks in table order and answers
// the first refusal. pluginName is the operator's block and registrationName
// the registration the process runs; each check says which one names it. No
// driver-phase check reads the site, so they are judged at programAlone, as a
// checker judges them.
func refuseDriverChecks(pluginName, registrationName string, input *rpc.DeclareRegistrationInput) error {
	for i := range stage1Checks {
		check := &stage1Checks[i]
		if check.phase != stage1PhaseDriver {
			continue
		}
		name := pluginName
		if check.byRegistration {
			name = registrationName
		}
		if err := check.refusal(name, input, programAlone(), true); err != nil {
			return err
		}
	}
	return nil
}

// refuseSinkChecks runs the sink-phase checks in table order, against the
// daemon the sink belongs to, and answers the first refusal.
func (e *engineStartupSink) refuseSinkChecks(input *rpc.DeclareRegistrationInput) error {
	name := e.proc.Config().Name
	site := e.site()
	for i := range stage1Checks {
		check := &stage1Checks[i]
		if check.phase != stage1PhaseSink {
			continue
		}
		if check.live != nil {
			if err := check.relayed(name, check.live(e, input), true); err != nil {
				return err
			}
			continue
		}
		if err := check.refusal(name, input, site, true); err != nil {
			return err
		}
	}
	return nil
}

// JudgeDeclaration judges a Stage 1 declaration for the plugin named name the
// way the daemon does, with no daemon running. It runs every table entry that
// needs no daemon state, at programAlone, and answers one verdict for each, in
// the daemon's order, with every failure and not only the first. The entries
// that need daemon state come back as unjudged, each with the state it needs.
//
// A verdict whose KeepsPlugin is true is a rule the daemon enforces at Stage 5
// with a warning: its failure refuses a command, never the plugin.
//
// Safe for concurrent use: the table is never written after init.
func JudgeDeclaration(name string, declaration *rpc.DeclareRegistrationInput) plugin.DeclarationJudgement {
	var judgement plugin.DeclarationJudgement
	for i := range stage1Checks {
		check := &stage1Checks[i]
		if check.isolated == nil {
			judgement.Unjudged = append(judgement.Unjudged, plugin.UnjudgedCheck{
				Check:   check.name,
				Section: check.section,
				Reason:  check.daemonState,
			})
			continue
		}
		judgement.Verdicts = append(judgement.Verdicts, plugin.DeclarationVerdict{
			Check:       check.name,
			Section:     check.section,
			KeepsPlugin: check.phase == stage1PhaseReady,
			Err:         check.refusal(name, declaration, programAlone(), false),
		})
	}
	return judgement
}
