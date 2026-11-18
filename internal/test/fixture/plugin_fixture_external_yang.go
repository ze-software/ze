// Design: docs/plugin-development/schema.md -- an external plugin brings its own YANG module
// Overview: register_external_yang.go -- the fixture name this file serves
// Related: fixture.go -- newObserver, the TLS connect-back every external fixture uses
// Related: plugin_fixture_failure_policy.go -- the generation marker this file follows
// Related: ../../../pkg/plugin/sdk/sdk_query.go -- RunOrDeclare, the declare-mode answer
// Related: ../externalyang/externalyang.go -- the YANG modules the variants declare
//
// The external plugin behind the external-yang-* functional tests. It is a
// REAL external process the daemon forks from a `run` line, built on the public
// Go SDK the way a third-party plugin is: it declares a YANG config module in
// its Stage 1 registration, wants its own config root, and answers a
// declaration query through sdk.RunOrDeclare.
//
// It asserts nothing about the config it is given. It prints what it received
// at Stage 2, at verify and at apply, one line each, and the .ci asserts on
// those lines. It never stops the daemon: each test ends on a line it awaits.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/externalyang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// externalYANGName is the plugin name used when the daemon gave none.
const externalYANGName = "external-yang"

// externalYANGMarker counts the starts of the redeclaring variant, one byte for
// each. It is relative, so it lands in the working directory the daemon gave
// the plugin, and it is a constant so no caller can steer the read elsewhere.
const externalYANGMarker = "external-yang.generation"

// externalYANGRefusedTarget is the `target` value the plugin refuses at verify,
// so a test can watch a commit abort with the plugin's own text.
const externalYANGRefusedTarget = "refuse-me"

// externalYANGDepthMax bounds the walk over one config document. The deepest
// document a variant's module allows is three levels (service, chrony, a leaf).
const externalYANGDepthMax = 16

// externalYANGVariant is one module the fixture can bring, selected by the
// first argument of its run line.
type externalYANGVariant struct {
	// schema is the module the variant declares on its first start. The
	// modules are declared in externalyang, which the config sweep reads too.
	// When the daemon is expected to refuse it (schema.Refused), a transport
	// that ends is the pass and a ready plugin is the failure.
	schema externalyang.Module
	// wants are the config roots the plugin asks for.
	wants []string
	// redeclaredText, when not empty, is the module text of every start after
	// the first; the first start then exits once the daemon is ready, so the
	// daemon starts the plugin again.
	redeclaredText string
	// configRefused says the test gives the daemon a config it must refuse
	// under this plugin's root, after it read the declaration. The daemon then
	// closes the connection before it answers Stage 1, which is how a plugin
	// learns that ze did not start: a transport that ends is the pass and a
	// ready plugin is the failure.
	configRefused bool
	// environment names the env keys whose value the plugin prints when it
	// starts, one line each, before it connects. A variant with none prints no
	// such line.
	environment []string
	// extra are the modules the plugin declares after schema, in order: a
	// command module and its rpc module. A variant with none declares schema
	// alone.
	extra []externalyang.Module
	// commands are the commands the plugin declares at Stage 1 and answers.
	// Each answer is printed, so a .ci tells a command that reached the plugin
	// from one the daemon refused before asking it.
	commands []string
	// redeclaredExtra, when not empty, are the modules declared after schema
	// on every start after the first, in place of extra; the first start then
	// exits once the daemon is ready, so the daemon starts the plugin again.
	redeclaredExtra []externalyang.Module
	// revertsAt, when not zero, is the start from which the variant declares
	// the modules of its first start again, so the daemon accepts it and the
	// plugin serves its commands after the starts it refused.
	revertsAt int
}

// externalYANGRoot is the config root the fixture plugin's module defines.
const externalYANGRoot = "acme-monitor"

// externalYANGVariants maps the run-line argument to its variant.
var externalYANGVariants = map[string]externalYANGVariant{
	"valid": {
		schema: externalyang.Valid(),
		wants:  []string{externalYANGRoot},
	},
	"valid-config-refused": {
		schema:        externalyang.Valid(),
		wants:         []string{externalYANGRoot},
		configRefused: true,
	},
	"valid-environment": {
		schema:      externalyang.Valid(),
		wants:       []string{externalYANGRoot},
		environment: []string{"exabgp.api.ack"},
	},
	"redeclare": {
		schema:         externalyang.Valid(),
		wants:          []string{externalYANGRoot},
		redeclaredText: externalyang.RedeclaredText,
	},
	"invalid": {
		schema: externalyang.Invalid(),
		wants:  []string{"acme-broken"},
	},
	"clash": {
		schema: externalyang.Clash(),
		wants:  []string{"acme-clash"},
	},
	"augment": {
		schema: externalyang.Augment(),
		wants:  []string{"service/chrony"},
	},
	"commands": {
		schema:   externalyang.Valid(),
		wants:    []string{externalYANGRoot},
		extra:    []externalyang.Module{externalyang.Commands(), externalyang.API()},
		commands: []string{externalyang.CommandStatus, externalyang.CommandTarget},
	},
	// The second start brings the command module with the target bound cut to
	// 1..8, which the daemon refuses. A refused restart leaves the plugin down
	// and is not retried (pluginserver restartPluginWithin), so the daemon makes
	// no third start of its own; one an operator orders brings the first
	// modules again.
	"commands-redeclare": {
		schema:          externalyang.Valid(),
		wants:           []string{externalYANGRoot},
		extra:           []externalyang.Module{externalyang.Commands(), externalyang.API()},
		commands:        []string{externalyang.CommandStatus, externalyang.CommandTarget},
		redeclaredExtra: []externalyang.Module{externalyang.CommandsNarrowed(), externalyang.API()},
		revertsAt:       3,
	},
	"command-clash": {
		schema:   externalyang.Valid(),
		wants:    []string{externalYANGRoot},
		extra:    []externalyang.Module{externalyang.CommandClash()},
		commands: []string{externalyang.CommandClashPath},
	},
}

// externalYANGVariantNamed answers the variant an argument names, or an error
// that lists the names the fixture accepts.
func externalYANGVariantNamed(name string) (externalYANGVariant, error) {
	variant, known := externalYANGVariants[name]
	if !known {
		names := make([]string, 0, len(externalYANGVariants))
		for accepted := range externalYANGVariants {
			names = append(names, accepted)
		}
		slices.Sort(names)
		return externalYANGVariant{}, fmt.Errorf("unknown variant %q; use one of: %s", name, strings.Join(names, ", "))
	}
	return variant, nil
}

// redeclares reports whether the variant brings another module after its
// first start.
func (variant externalYANGVariant) redeclares() bool {
	return variant.redeclaredText != "" || len(variant.redeclaredExtra) > 0
}

// redeclaring reports whether the start generation brings the other modules:
// a start after the first, before the one the variant reverts at.
func (variant externalYANGVariant) redeclaring(generation int) bool {
	if !variant.redeclares() || generation < 2 {
		return false
	}
	return variant.revertsAt == 0 || generation < variant.revertsAt
}

// refused reports whether the daemon is expected to refuse a module the
// variant brings, so a transport that ends is the pass.
func (variant externalYANGVariant) refused() bool {
	if variant.schema.Refused {
		return true
	}
	return slices.ContainsFunc(variant.extra, func(module externalyang.Module) bool { return module.Refused })
}

// declaration answers the Stage 1 registration of one start. generation is
// one for the first start, and for a declaration query.
func (variant externalYANGVariant) declaration(generation int) *sdk.Registration {
	text := variant.schema.Text
	extra := variant.extra
	if variant.redeclaring(generation) && variant.redeclaredText != "" {
		text = variant.redeclaredText
	}
	if variant.redeclaring(generation) && len(variant.redeclaredExtra) > 0 {
		extra = variant.redeclaredExtra
	}
	declaration := &sdk.Registration{
		WantsConfig: variant.wants,
		Schemas: []sdk.SchemaDecl{{
			Module:    variant.schema.Name,
			Namespace: variant.schema.Namespace,
			YANGText:  text,
		}},
	}
	for _, module := range extra {
		declaration.Schemas = append(declaration.Schemas, sdk.SchemaDecl{
			Module:    module.Name,
			Namespace: module.Namespace,
			YANGText:  module.Text,
		})
	}
	// Each command also states its two help texts at Stage 1, worded unlike
	// the ze:help and description its YANG node declares, so a test can tell
	// which of the two a surface shows: the YANG one, by the approved decision
	// of spec plugin-external-yang-at-startup (AC-30).
	for _, name := range variant.commands {
		declaration.Commands = append(declaration.Commands, sdk.CommandDecl{
			Name:        name,
			ShortHelp:   externalyang.DeclaredSummary,
			Description: externalyang.DeclaredExplanation,
		})
	}
	if variant.redeclares() {
		// The first start exits, and only a plugin that declared restart is
		// started again.
		declaration.FailurePolicy = sdk.FailureRestart
	}
	return declaration
}

// externalYANGGeneration answers how many times the fixture has been started
// in this working directory, this start included.
//
// A file carries the count, because a restart is a new process: nothing in
// this process's memory survives to the start the test waits for.
func externalYANGGeneration() (int, error) {
	previous, err := os.ReadFile(externalYANGMarker)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", externalYANGMarker, err)
	}
	generation := len(previous) + 1
	if err := os.WriteFile(externalYANGMarker, append(previous, 'x'), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", externalYANGMarker, err)
	}
	return generation, nil
}

// externalYANGLeaves lowers one JSON config document to its leaves, each a
// `path=value` word, sorted. An empty document and an empty object, which is
// how a removed root arrives, both have no leaves.
func externalYANGLeaves(document string) ([]string, error) {
	if document == "" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal([]byte(document), &value); err != nil {
		return nil, fmt.Errorf("decode config document: %w", err)
	}
	words, err := externalYANGLeafWords(nil, "", value, 0)
	if err != nil {
		return nil, err
	}
	slices.Sort(words)
	return words, nil
}

// externalYANGLeafWords appends the leaves under value to words. It recurses
// once per container level, and externalYANGDepthMax bounds the depth.
func externalYANGLeafWords(words []string, path string, value any, depth int) ([]string, error) {
	if depth > externalYANGDepthMax {
		return nil, fmt.Errorf("config document is deeper than %d levels at %s", externalYANGDepthMax, path)
	}
	// encoding/json decodes into exactly these six types; any other is a
	// defect of this file, never of the document.
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			var err error
			words, err = externalYANGLeafWords(words, externalYANGChildPath(path, key), child, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return words, nil
	case []any:
		for index, child := range typed {
			var err error
			words, err = externalYANGLeafWords(words, externalYANGChildPath(path, strconv.Itoa(index)), child, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return words, nil
	case string:
		return append(words, path+"="+typed), nil
	case float64:
		return append(words, path+"="+strconv.FormatFloat(typed, 'f', -1, 64)), nil
	case bool:
		return append(words, path+"="+strconv.FormatBool(typed)), nil
	case nil:
		return append(words, path+"=null"), nil
	default:
		return nil, fmt.Errorf("config document holds a value of an unexpected kind at %s", path)
	}
}

// externalYANGChildPath joins a path and one more segment.
func externalYANGChildPath(path, segment string) string {
	if path == "" {
		return segment
	}
	return path + "/" + segment
}

// externalYANGSay prints one line of what the plugin received, where the
// daemon relays it and a .ci reads it.
func externalYANGSay(event, root string, leaves []string) {
	var line textbuf.Buffer
	line.Str("FIXTURE: external-yang ").Str(event).Str(" root=").Str(root)
	line.Str(" leaves=[").Join(leaves, " ").Str("]\n")
	line.StdErr() //nolint:errcheck // fixture progress output
}

// externalYANGSayGeneration prints one line about a start of the plugin.
func externalYANGSayGeneration(generation int, what string) {
	var line textbuf.Buffer
	line.Str("FIXTURE: external-yang generation ").Int(int64(generation)).Byte(' ').Str(what).Byte('\n')
	line.StdErr() //nolint:errcheck // fixture progress output
}

// externalYANGSayEnvironment prints, for each key, the value this process was
// started with, read the way a Ze program reads it (env.LookupIn: any case and
// separator spelling). A key the environment does not carry prints an empty
// value, so a .ci tells a setting that reached the plugin from one that did not.
func externalYANGSayEnvironment(keys []string) {
	environ := os.Environ()
	for _, key := range keys {
		var line textbuf.Buffer
		line.Str("FIXTURE: external-yang environment ").Str(key).Str("=[").Str(env.LookupIn(environ, key)).Str("]\n")
		line.StdErr() //nolint:errcheck // fixture progress output
	}
}

// externalYANGConfigure prints each section the plugin received at Stage 2.
func externalYANGConfigure(sections []sdk.ConfigSection) error {
	var count textbuf.Buffer
	count.Str("FIXTURE: external-yang configure sections=").Int(int64(len(sections))).Byte('\n')
	count.StdErr() //nolint:errcheck // fixture progress output
	for _, section := range sections {
		leaves, err := externalYANGLeaves(section.Data)
		if err != nil {
			return fmt.Errorf("configure root %s: %w", section.Root, err)
		}
		externalYANGSay("configure", section.Root, leaves)
	}
	return nil
}

// externalYANGVerify prints each candidate section and refuses the candidate
// whose target is externalYANGRefusedTarget.
func externalYANGVerify(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		leaves, err := externalYANGLeaves(section.Data)
		if err != nil {
			return fmt.Errorf("verify root %s: %w", section.Root, err)
		}
		externalYANGSay("verify", section.Root, leaves)
		for _, leaf := range leaves {
			if strings.HasSuffix(leaf, "/target="+externalYANGRefusedTarget) {
				return errors.New("acme-monitor cannot probe target " + externalYANGRefusedTarget)
			}
		}
	}
	return nil
}

// externalYANGApply prints the three parts of each changed section.
func externalYANGApply(sections []sdk.ConfigDiffSection) error {
	for _, section := range sections {
		added, err := externalYANGLeaves(section.Added)
		if err != nil {
			return fmt.Errorf("apply root %s, added: %w", section.Root, err)
		}
		removed, err := externalYANGLeaves(section.Removed)
		if err != nil {
			return fmt.Errorf("apply root %s, removed: %w", section.Root, err)
		}
		changed, err := externalYANGLeaves(section.Changed)
		if err != nil {
			return fmt.Errorf("apply root %s, changed: %w", section.Root, err)
		}
		var line textbuf.Buffer
		line.Str("FIXTURE: external-yang apply root=").Str(section.Root)
		line.Str(" added=[").Join(added, " ").Str("] removed=[").Join(removed, " ")
		line.Str("] changed=[").Join(changed, " ").Str("]\n")
		line.StdErr() //nolint:errcheck // fixture progress output
	}
	return nil
}

// externalYANGCommand answers a command the plugin declared, and prints the
// command and its arguments as the daemon sent them: a value the daemon
// refused never reaches here, so it is never printed. The answer carries the
// arguments too, so the command's own output shows what the plugin received,
// with no wait on the relay of the plugin's stderr through the daemon.
func externalYANGCommand(_, command string, args []string, _ string) (string, any, error) {
	var line textbuf.Buffer
	line.Str("FIXTURE: external-yang command ").Str(command).Str(" args=[").Join(args, " ").Str("]\n")
	line.StdErr() //nolint:errcheck // fixture progress output
	var received textbuf.Buffer
	received.Str("[").Join(args, " ").Str("]")
	return statusDone, map[string]any{fieldResult: command, fieldExternalYANGArgs: received.String()}, nil
}

// fieldExternalYANGArgs is the key of the arguments in the answer of
// externalYANGCommand, written as the stderr line writes them.
const fieldExternalYANGArgs = "args"

// externalYANGRun runs the plugin transport and reports why it ended. One
// goroutine for one plugin lifecycle, ended by the send; the channel is
// buffered, so it never blocks.
func externalYANGRun(ctx context.Context, plugin *sdk.Plugin, declaration *sdk.Registration, ended chan<- error) {
	ended <- plugin.Run(ctx, declaration)
}

// externalYANGServe is the live start: it connects, declares, prints what the
// daemon sends, and returns when the daemon closes the connection.
func externalYANGServe(ctx context.Context, variant externalYANGVariant, generation int) error {
	externalYANGSayEnvironment(variant.environment)
	plugin, err := newObserver(externalYANGName)
	if err != nil {
		return fmt.Errorf("connect %s: %w", externalYANGName, err)
	}
	defer plugin.Close() //nolint:errcheck // the run result carries the useful transport error

	plugin.OnConfigure(externalYANGConfigure)
	plugin.OnConfigVerify(externalYANGVerify)
	plugin.OnConfigApply(externalYANGApply)
	if len(variant.commands) > 0 {
		plugin.OnExecuteCommand(externalYANGCommand)
	}

	// The daemon reports ready once in the life of a process, and the channel
	// holds that one report, so the send never blocks.
	ready := make(chan struct{}, 1)
	plugin.OnAllPluginsReady(func() error {
		externalYANGSayGeneration(generation, "ready")
		ready <- struct{}{}
		return nil
	})

	ended := make(chan error, 1)
	go externalYANGRun(ctx, plugin, variant.declaration(generation), ended)

	// The loop ends when the transport ends or the context does; it turns at
	// most once more than that, for the one ready report.
	for {
		select {
		case <-ready:
			if variant.refused() {
				return errors.New("the daemon accepted a module it must refuse: " + variant.schema.Name)
			}
			if variant.configRefused {
				return errors.New("the daemon started with a config it must refuse under the root of " + variant.schema.Name)
			}
			if variant.redeclares() && generation == 1 {
				// Returning ends the process, and the daemon starts it again.
				externalYANGSayGeneration(generation, "is exiting")
				return nil
			}
		case runErr := <-ended:
			return externalYANGEnded(variant, generation, runErr)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// externalYANGEnded judges the end of the transport. A refusal the test is
// written to produce is asserted on the daemon's stderr, where an operator
// reads it, so this process reports the line and no failure of its own.
func externalYANGEnded(variant externalYANGVariant, generation int, runErr error) error {
	if runErr == nil {
		return nil
	}
	if variant.configRefused {
		// Ze refused the config after it read the declaration and closed the
		// connection with Stage 1 unanswered: the normal end for this plugin.
		var line textbuf.Buffer
		line.Str("FIXTURE: external-yang ended with the refused config: ").Err(runErr).Byte('\n')
		line.StdErr() //nolint:errcheck // fixture progress output
		return nil
	}
	// refused judges every module the variant brings, its config module and
	// its extra ones (a command module that clashes), as the ready branch of
	// externalYANGPlugin does: a transport the daemon closed after refusing
	// any of them is this plugin's normal end.
	if variant.refused() {
		var line textbuf.Buffer
		line.Str("FIXTURE: external-yang was refused: ").Err(runErr).Byte('\n')
		line.StdErr() //nolint:errcheck // fixture progress output
		return nil
	}
	if variant.redeclares() && generation > 1 {
		var line textbuf.Buffer
		line.Str("FIXTURE: external-yang generation ").Int(int64(generation))
		line.Str(" was refused: ").Err(runErr).Byte('\n')
		line.StdErr() //nolint:errcheck // fixture progress output
		return nil
	}
	return runErr
}

// externalYANGDriver brings the module its first argument names. Under a
// declaration query it writes its Stage 1 line and starts nothing: no
// connection, no marker file.
func externalYANGDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("plugin/external-yang takes one argument, the variant")
	}
	variant, err := externalYANGVariantNamed(args[0])
	if err != nil {
		return err
	}

	generation := 1
	if variant.redeclares() && !sdk.QueryModeRequested() {
		generation, err = externalYANGGeneration()
		if err != nil {
			return err
		}
	}

	var serveErr error
	code := sdk.RunOrDeclare(variant.declaration(generation), func() int {
		serveErr = externalYANGServe(ctx, variant, generation)
		if serveErr != nil {
			return 1
		}
		return 0
	})
	if serveErr != nil {
		return serveErr
	}
	if code != 0 {
		return fmt.Errorf("declaration query failed with exit code %d", code)
	}
	return nil
}
