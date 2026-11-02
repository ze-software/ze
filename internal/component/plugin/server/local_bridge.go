// Design: docs/architecture/api/commands.md — local-data commands, served by the daemon too
// Related: command.go — LoadBuiltins and loadBuiltinsWithAliases, which call this
//
// local_bridge.go serves the local-data commands from the daemon dispatcher.
//
// A local-data command (command.MustRegisterLocalData) answered only in the
// caller's own process, so MCP, REST, gRPC, SSH and the web, which reach the
// daemon, never offered it: `show env list` answered on `ze` and nowhere a
// remote operator could ask. Its handler already returns its whole answer as
// a value (command.LocalAnswer), so the daemon runs the SAME handler and hands
// that value back: one producer, two routes.

package server

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// loadLocalData registers every local-data command whose path a YANG
// ze:command node declares, under that path, so the dispatcher serves it like
// a builtin: authorization and accounting first, then the validator over the
// leaves the node declares, then the handler.
//
// The set is derived from the two registries, never listed: the local
// registry names the commands and their metadata, and the YANG maps say which
// paths a node declares. A plain local command with no data form is not
// served here, because its handler prints rather than answering a value.
//
// ReadOnly comes from the verb unless the registration declares the daemon
// form does more than read (registry.Meta.NotReadOnly). A path whose node's
// own RPC is a builtin is left to that builtin: the owner registered it as the
// daemon form of the same producer. A path some OTHER registration holds is
// refused by RegisterWithOptions (ErrCommandHeld), as two builtins on one path
// are, so neither owner wins in silence.
//
// A local command at an offline root (`help command`) cannot carry a YANG
// node at its own path, so its registration names the path that answers it on
// the daemon (registry.Meta.DaemonPath), and the handler is served there. A
// declared daemon path no YANG node carries is refused: the command would
// reach no remote surface while its declaration reads as served.
func loadLocalData(d *Dispatcher, wireToPaths map[string][]string, pathToDesc, pathToHelp map[string]string, pathToArgDefs map[string][]command.ArgDef) error {
	declared := make(map[string]string)
	for wire, paths := range wireToPaths {
		for _, path := range paths {
			declared[path] = wire
		}
	}
	builtinWires := make(map[string]bool)
	for _, reg := range AllBuiltinRPCs() {
		builtinWires[reg.WireMethod] = true
	}
	for _, entry := range command.ListLocal() {
		servedAt := entry.Path
		if entry.Meta.DaemonPath != "" {
			servedAt = entry.Meta.DaemonPath
		}
		wire, hasNode := declared[servedAt]
		if !hasNode {
			if entry.Meta.DaemonPath != "" {
				var tb textbuf.Buffer
				return errors.New(tb.Str("local command ").Str(entry.Path).Str(" declares daemon path ").Str(servedAt).
					Str(", which no YANG ze:command node declares: add the node or correct registry.Meta.DaemonPath").String())
			}
			continue
		}
		if builtinWires[wire] {
			// The node's own RPC is the daemon form: its owner registered a
			// builtin that calls the same producer with what only the daemon
			// holds (doctor's plugin checks), so this handler is that
			// command's no-daemon route and the builtin already serves the path.
			continue
		}
		handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
		if handler == nil {
			continue // A plain local command: it prints, and answers no value.
		}
		if len(rest) != 0 {
			continue // A shorter data path covers it; that path is served on its own.
		}
		if err := d.RegisterWithOptions(servedAt, localDataHandler(servedAt, handler), pathToDesc[servedAt], RegisterOptions{
			ReadOnly:    IsReadOnlyPath(servedAt) && !entry.Meta.NotReadOnly,
			ArgDefs:     pathToArgDefs[servedAt],
			Description: pathToHelp[servedAt],
			Owner:       wire,
		}); err != nil {
			return err
		}
	}
	return nil
}

// localDataHandler is the daemon form of a local-data handler: it runs the
// handler on the remote route (command.AnswerRemote), so a handler that reads
// what only the operator's own process holds refuses instead, and maps the
// answer onto the response.
func localDataHandler(path string, handler command.LocalDataHandler) Handler {
	return func(cc *CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		return AnswerLocalData(path, handler, args.WithContext(cc.Context()))
	}
}

// AnswerLocalData runs a local-data handler on the daemon route and answers
// its LocalAnswer as the dispatcher's response, the way the bridge serves every
// local-data command. A builtin that is the daemon form of a local-data
// command (the doctor RPCs) answers through here, so its response maps the
// answer exactly as the bridge would.
func AnswerLocalData(path string, handler command.LocalDataHandler, args command.ValidatedArgs) (*plugin.Response, error) {
	return localAnswerResponse(path, command.AnswerRemote(handler, args))
}

// localAnswerResponse maps a local-data answer onto a daemon response, the
// way the local route maps it onto stdout, stderr and the exit code.
//
//   - The payload is the response Data. It is encoded once here; the declared
//     shape of the path (command.RegisterShape) governs the pipes on it, as
//     it does locally.
//   - A non-zero code is a failed status, and the error is a
//     plugin.RefusalError holding the Diagnostic the local route prints on
//     stderr. Response.Error and the error's text are the reason without the
//     "error: " the Diagnostic may open with, for a program to read; a
//     surface that prints for a person asks plugin.RefusalLine and prints the
//     Diagnostic as it stands instead of adding its own prefix, so a usage
//     refusal reads "usage: ..." on SSH as it does locally, and an "error: "
//     refusal is not printed twice. A refusal with no Diagnostic is a plain
//     error naming the exit code: there is no text to print verbatim. The
//     payload stays beside it: `show config validation` answers the
//     diagnostics of a config it rejects and exits 1.
//   - A Diagnostic beside a zero code is a warning about an answer the
//     command still gave, such as the parse warnings of `show config dump`.
//     It travels as the response Warnings, one line each, under
//     StatusWarning, so every surface shows it as the local route shows it on
//     stderr.
//
// Nothing is written to the daemon's stdout or stderr.
func localAnswerResponse(path string, answer command.LocalAnswer) (*plugin.Response, error) {
	resp := &plugin.Response{Status: plugin.StatusDone}
	if answer.Payload != nil {
		encoded, err := json.Marshal(answer.Payload)
		if err != nil {
			var tb textbuf.Buffer
			reason := tb.Str("the answer of ").Str(path).Str(" could not be encoded: ").Err(err).String()
			return &plugin.Response{Status: plugin.StatusError, Error: reason}, errors.New(reason)
		}
		resp.Data = plugin.RawJSON(encoded)
	}
	if answer.Code == 0 {
		if answer.Diagnostic != "" {
			resp.Status = plugin.StatusWarning
			resp.Warnings = strings.Split(strings.TrimSuffix(answer.Diagnostic, "\n"), "\n")
		}
		return resp, nil
	}
	resp.Status = plugin.StatusError
	if strings.TrimSuffix(answer.Diagnostic, "\n") == "" {
		var tb textbuf.Buffer
		reason := tb.Str(path).Str(" failed with exit code ").Int(int64(answer.Code)).Str(" and gave no reason").String()
		resp.Error = reason
		return resp, errors.New(reason)
	}
	refusal := plugin.NewRefusalError(answer.Diagnostic)
	resp.Error = refusal.Error()
	return resp, refusal
}
