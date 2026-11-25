// Design: docs/architecture/api/commands.md — a path this binary does not hold asks the running ze
// Overview: cmdutil.go — RunCommand, which hands such a path to this file
// Related: ../../../../internal/component/cli/sshclient/reach.go — ReachDaemon and Unreached.Explain, the walk and its wording
// Related: ../../../../internal/component/config/declared_schemas.go — PublishDeclaredSchemas, which says which external plugins a config declares
//
// An external plugin brings its commands in its own YANG, and ze joins that
// YANG when `ze start` runs the plugin. The command tree of this binary is
// built from the compiled-in YANG alone, so `ze show acme-monitor status`
// names a path the tree does not hold even when the running ze serves it.
// The command takes the route a compiled-in command takes to the running ze:
// the same line, the same rendering, the same exit code (cli.RunReached). Only
// when the running ze does not know the path either, or no ze answers, is it
// refused here.
//
// With no ze running, ze starts no external plugin (owner ruling), so this
// process cannot know an external plugin's commands. When the config `ze
// start` boots declares one, the refusal says the command may be one of its
// own, which only a running ze holds, and names the command that gets one.
// When it declares none, the path is unknown, and the refusal is the one an
// unknown path always had.

package cmdutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// unheldRoute answers `ze <verb> <path>` for a path this binary's command tree
// does not hold. Each field is one step a test replaces with a stub, so no
// unit test opens a connection or reads a store.
type unheldRoute struct {
	// reach walks to the running ze (sshclient.ReachDaemon). Its error is an
	// *sshclient.Unreached.
	reach func() (sshclient.Reached, error)
	// run sends line to the reached ze as a compiled-in command's line is
	// sent, and answers the exit code (cli.RunReached). notUnderstood answers
	// in its place when the ze holds no such command either.
	run func(reached sshclient.Reached, line, format string, notUnderstood func(message string) int) int
	// declared answers the external plugins the stored config declares that
	// this process holds no schema of (storedConfigExternalPlugins).
	declared func() ([]string, error)
	// stderr receives every refusal of this route.
	stderr io.Writer
}

// unheldPaths is the route RunCommand takes in a ze binary.
var unheldPaths = unheldRoute{
	reach: func() (sshclient.Reached, error) { return sshclient.ReachDaemon("") },
	run: func(reached sshclient.Reached, line, format string, notUnderstood func(message string) int) int {
		return cli.RunReached(reached.Credentials, line, format, notUnderstood)
	},
	declared: storedConfigExternalPlugins,
	stderr:   os.Stderr,
}

// answer runs res, a path this binary's tree does not hold, on the running ze,
// or refuses it. args is argv, verb first, as the operator typed it.
//
// A ze that listens and this account can log in to is the answer, whatever it
// says: its refusal of a value, its rendering under a pipe, its exit code. Only
// its "names no command" is replaced, by the unknown-command refusal of this
// binary, with the ze's own text when it said more than that.
func (r unheldRoute) answer(res Resolution, args []string, cmdName string) int {
	reached, err := r.reach()
	if err == nil {
		return r.run(reached, res.dispatchString(), res.Format, func(message string) int {
			return r.refuseUnknown(res, args, cmdName, message)
		})
	}
	plugins, readErr := r.declared()
	if readErr != nil {
		// Unknown, not absent: the config could not be read, so whether it
		// declares an external plugin is not known. The path is still one
		// this binary does not hold, and the operator is told what was not
		// checked.
		code := r.refuseUnknown(res, args, cmdName, "")
		var hint textbuf.Buffer
		hint.Str("hint: the stored config could not be read to learn whether it declares an external plugin (").
			Err(readErr).Str(")\n")
		r.stderr.Write(hint.Bytes()) //nolint:errcheck // one-shot diagnostic to stderr
		return code
	}
	if len(plugins) == 0 {
		return r.refuseUnknown(res, args, cmdName, "")
	}
	return r.refuseNotAvailable(args, plugins, err)
}

// refuseUnknown writes the refusal of a path nothing holds and answers exit 1.
// With no daemon message it is byte for byte the refusal `ze <verb>` gave an
// unknown path before a running ze was asked: the path, the nearest command of
// this binary's tree, and the help command. message is the running ze's text
// for the path, shown when it says more than "unknown command".
func (r unheldRoute) refuseUnknown(res Resolution, args []string, cmdName, message string) int {
	var text textbuf.Buffer
	text.Str("error: unknown command: ").Join(args, " ").Byte('\n')
	if message != "" && message != pluginserver.ErrUnknownCommand.Error() {
		text.Str("hint: the running ze answered: ").Str(message).Byte('\n')
	}
	if suggestion := SuggestFromTree(res.Relative[0], res.Tree); suggestion != "" {
		text.Str("hint: did you mean 'ze ").Str(cmdName).Byte(' ').Str(suggestion).Str("'?\n")
	}
	text.Str("hint: run 'ze ").Str(cmdName).Str(" help' for available commands\n")
	r.stderr.Write(text.Bytes()) //nolint:errcheck // one-shot diagnostic to stderr
	return 1
}

// refuseNotAvailable writes the refusal of a path this binary does not hold,
// when no ze answered and the stored config declares plugins, external ones
// whose commands only a running ze holds, and answers exit 1. unreached is the
// failure of the walk to the ze.
func (r unheldRoute) refuseNotAvailable(args, plugins []string, unreached error) int {
	line := strings.Join(args, " ")
	typed := "ze " + line

	var why, fix string
	if failure, ok := errors.AsType[*sshclient.Unreached](unreached); ok {
		why, fix = failure.Explain()
	} else {
		// Only a stub answers another error: its text is the reason, and the
		// fix is the one for a ze this account cannot log in to.
		why = unreached.Error()
	}

	var reason textbuf.Buffer
	reason.Str("the config declares ").Str(config.ExternalPluginNames(plugins)).
		Str(": ze reads the commands of an external plugin from the plugin when `ze start` runs it, ").
		Str("and this command starts no plugin. No running ze answered: ").Str(why)
	if fix == "" {
		// A ze listens and this account cannot log in to it: the line is sent
		// from an account that can.
		var account textbuf.Buffer
		fix = account.Str(`ze cli --user <name> -c "`).Str(line).Byte('"').String()
	} else {
		reason.Str(". Run the command below, then `").Str(typed).Str("` again")
	}

	refusal := opererr.New(
		line+" is not available: it is not a command compiled into ze, and it may belong to an external plugin, which only a running ze holds",
		reason.String(), fix)
	var text textbuf.Buffer
	text.Str("error: ").Err(refusal).Byte('\n')
	r.stderr.Write(text.Bytes()) //nolint:errcheck // one-shot diagnostic to stderr
	return 1
}

// storedConfigExternalPlugins answers the external plugins declared by the
// config a bare `ze start` boots (the store's active config, named by the
// instance), that this process holds no schema of, by the step every command
// that parses a config with no daemon takes (config.PublishDeclaredSchemas).
// It starts no program.
//
// A store that does not exist, and a store that holds no such config, declare
// nothing: `ze start` would boot no external plugin from them. Every other
// failure to read is an error, because it leaves the question unanswered.
func storedConfigExternalPlugins() ([]string, error) {
	dir := resolve.StoreDir("")
	store, err := storage.OpenReadOnly(dir, dir)
	if errors.Is(err, storage.ErrNoStore) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer store.Close() //nolint:errcheck // read-only access

	name, err := resolve.DefaultConfig(store)
	if err != nil {
		return nil, err
	}
	text, err := store.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	firstPass, err := config.FirstPass(string(text), name, nil)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(firstPass.External))
	seen := make(map[string]struct{}, len(firstPass.External))
	for _, ext := range firstPass.External {
		if ext.Name == "" {
			continue
		}
		if _, dup := seen[ext.Name]; dup {
			continue
		}
		seen[ext.Name] = struct{}{}
		names = append(names, ext.Name)
	}
	return names, nil
}
