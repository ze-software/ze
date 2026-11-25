// Design: docs/architecture/config/yang-config-design.md — `ze config fmt`, `dump`, `diff` and `graph` ask the running ze first
// Overview: daemon_route.go — the route to the running ze every local config command shares
// Related: config_supplied.go — suppliedFmt, suppliedDump, suppliedDiff and suppliedGraph, the daemon commands this file sends
// Related: cmd_fmt.go — fmtAnswer and fmtFormattedAnswer, the local renderer of fmt
// Related: cmd_dump.go — cmdDump and printConfig, the local renderer of dump
// Related: cmd_diff.go — readDiffInputs and compareDiffInputs, the two halves of the local diff
// Related: cmd_graph.go — graphAnswer, the local renderer of graph
//
// The four commands that read a config and answer about it take the route
// `ze config validate` takes (daemon_route.go): a config that declares an
// external plugin goes to the running ze, in the command's own
// `show config supplied` form, and the daemon's answer is drawn with the
// command's local renderer, so the text, the formats and the exit codes are
// the local ones.
//
// What each command sends, and how its answer is drawn:
//
//   - fmt sends the bytes with no check or diff leaf, and takes the formatted
//     text back. The --check verdict, the --diff hunks and the -w write are
//     made here from that text, by the code that makes them from a local
//     format (fmtFormattedAnswer): a check that found changes exits 1 with a
//     line on stderr, which the daemon route carries as a failed status, and a
//     failed status must stay readable as the daemon's refusal. The file -w
//     rewrites is the operator's, so this process writes it.
//   - graph sends the bytes and prints the JSON text it gets back.
//   - dump sends the bytes with the strip-private leaf, and takes the resolved
//     document back: `ze show config dump` answers it, and `ze config dump`
//     prints its bgp block and its plugin list as printConfig does.
//   - diff reads its two sides here, in every form the command has (two files,
//     a revision and a file, a backup artifact, standard input), and sends
//     them as one JSON object with a before and an after string.
//
// The daemon's refusal is drawn only when this process refuses the config too:
// the daemon holds every schema this binary compiles in, so a config this
// process answers about is one the daemon's refusal says nothing about (a
// denied account, a command it does not serve), and the local answer stands.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The daemon commands the four inspections send, as ze-config-cli-cmd.yang
// declares them: the supplied form, which takes the bytes, and the form that
// reads a file the daemon holds, sent for content too large for one command.
const (
	pathSuppliedFmt   = "show config supplied fmt"
	pathSuppliedDump  = "show config supplied dump"
	pathSuppliedDiff  = "show config supplied diff"
	pathSuppliedGraph = "show config supplied graph"
	pathShowFmt       = "show config fmt"
	pathShowDump      = "show config dump"
	pathShowDiff      = "show config diff"
	pathShowGraph     = "show config graph"

	localFmt   = "ze config fmt"
	localDump  = "ze config dump"
	localDiff  = "ze config diff"
	localGraph = "ze config graph"

	// blobAnswerKey is the one key of a Blob's JSON answer
	// (command.Blob.MarshalJSON), under which command.DecodeBlob reads.
	blobAnswerKey = "blob"
	// keyBusRoot, keyBusEndpoint, keyPluginExternal, keyPluginRun and
	// keyPluginEncoding are the names of the external endpoint declarations in a
	// dump document, as ze-plugin-conf.yang declares them.
	keyBusRoot        = "bus"
	keyBusEndpoint    = "endpoint"
	keyPluginExternal = "external"
	keyPluginRun      = "run"
	keyPluginEncoding = "encoding"
	keyBGP            = "bgp"
)

// routedAnswer is one command's answer about a config, asked of the running
// ze first when the config needs it (suppliedQuestion.needsDaemon) and made in
// this process otherwise.
type routedAnswer[T any] struct {
	question suppliedQuestion
	// sources are the configs the answer is about.
	sources []config.ConfigSource
	// local makes the answer in this process.
	local func() T
	// refuses reports whether an answer local made is a refusal of the config.
	refuses func(T) bool
	// drawn draws what the daemon answered under an accepted status. Its
	// error says what the daemon sent instead of an answer this ze reads.
	drawn func(daemonVerdict) (T, error)
	// refusal draws the text the daemon refused the config with.
	refusal func(text string) T
}

// answer asks the running ze when the config needs it and draws its answer,
// and answers in this process when no ze is needed or none answered. A failed
// ask is never read as an answer: it is recorded, so the local refusal of a
// name no compiled-in schema defines says why no ze answered and the one
// command that gets an answer.
func (r routedAnswer[T]) answer(user string, ask daemonAsk) T {
	if !r.question.needsDaemon(r.sources) {
		return r.local()
	}
	line, err := r.question.line()
	if err != nil {
		return r.unanswered(err)
	}
	verdict, err := ask(user, line)
	if err != nil {
		return r.unanswered(err)
	}
	asked := r.question.asked(line)
	if verdict.refused {
		if verdict.refusal == "" {
			return r.unanswered(foreignAnswer(verdict.address, asked, "a failed status and no reason"))
		}
		// The daemon refused. Its refusal is about the config only when this
		// process refuses the config too.
		own := r.local()
		if !r.refuses(own) {
			return own
		}
		return r.refusal(verdict.refusal)
	}
	drawn, err := r.drawn(verdict)
	if err != nil {
		return r.unanswered(foreignAnswer(verdict.address, asked, err.Error()))
	}
	return drawn
}

// unanswered records why no running ze answered and answers in this process.
func (r routedAnswer[T]) unanswered(err error) T {
	config.RecordDaemonUnanswered(r.question.unanswered(err))
	return r.local()
}

// fileQuestion is what a command that reads one config file asks the running
// ze: content read from path, sent to supplied, or named to byPath when it is
// too large to send. options are the operator's options, for the command a
// failed ask names.
func fileQuestion(supplied, byPath, local, path string, content []byte, options []string) suppliedQuestion {
	question := suppliedQuestion{
		supplied:     supplied,
		content:      content,
		labelPath:    path,
		byPath:       byPath,
		local:        local,
		subject:      configSubject(path),
		placeholders: []string{placeholderConfigFile},
	}
	operand := placeholderConfigFile
	if namesConfigFile(path) {
		question.files = []string{path}
		operand = path
	}
	question.operands = strings.Join(append(slices.Clone(options), operand), " ")
	return question
}

// daemonRefusal is the refusal of subject by the running ze, as the one
// operator error the local route prints: that the running ze refused it, what
// the daemon answered, and one fix.
//
// text is what the daemon sent with its failed status, which reaches this
// process as text alone. The daemon ends an operator error with its fix line,
// and that fix is the one kept: text is cut before it, so the operator reads
// one fix and never two. Every fix line is cut out of what the daemon
// answered, and the last one that names a command is the fix. A text with no
// such line gets fallback, the command that lists what the running ze refuses
// in the config. A text that held fix lines alone is drawn as a refusal with
// no reason.
func daemonRefusal(subject, fallback, text string) *opererr.Error {
	fix := fallback
	var kept []string
	for line := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
		own, isFix := strings.CutPrefix(line, daemonFixLabel)
		if !isFix {
			kept = append(kept, line)
			continue
		}
		if own = strings.TrimSpace(own); own != "" {
			fix = own
		}
	}
	answered := strings.Join(kept, "\n")
	if strings.TrimSpace(answered) == "" {
		answered = "a refusal with no reason"
	}
	return opererr.Wrap(errors.New(answered),
		"the running ze refused "+subject,
		"It checked the config with the schemas of the plugins it started. What it answered",
		fix)
}

// daemonFixLabel starts the fix line of an operator error, as opererr renders
// one and as the daemon sends it.
const daemonFixLabel = "fix: "

// validateFix is the command that lists every problem the running ze finds in
// the config at path, with a placeholder for a config with no file.
func validateFix(path string) string {
	if namesConfigFile(path) {
		return localValidate + " " + path
	}
	return localValidate + " " + placeholderConfigFile
}

// stdinValidateFix is the fix line the producers of dump, fmt and graph end a
// refusal of the config with. A producer judges bytes, for a file and for
// content a caller supplied alike, and holds no file name, so its fix names
// standard input.
const stdinValidateFix = "fix: " + localValidate + " -"

// fixNamingFile answers diag, the lines a command prints for the operator,
// with a final stdinValidateFix line naming path, the file the operator gave
// the command, in place of standard input: the operator did not pipe the
// config, so `-` is not a command they can run as written. The running ze
// judged content with no file name and this process's own producer judged
// bytes, so the command that read the file is the one place that knows its
// name. diag is answered unchanged when the operator did give standard input,
// and when it does not end with that line.
func fixNamingFile(diag, path string) string {
	if !namesConfigFile(path) {
		return diag
	}
	body := strings.TrimSuffix(diag, "\n")
	head, ends := strings.CutSuffix(body, stdinValidateFix)
	if !ends {
		return diag
	}
	if head != "" && !strings.HasSuffix(head, "\n") {
		return diag
	}
	return head + "fix: " + localValidate + " " + path + diag[len(body):]
}

// writeDaemonWarnings writes the lines a daemon command wrote beside an answer
// it gave, as the local route prints them on stderr.
func writeDaemonWarnings(diag *textbuf.Buffer, warnings string) {
	if warnings == "" {
		return
	}
	diag.Str(warnings)
	if !strings.HasSuffix(warnings, "\n") {
		diag.Byte('\n')
	}
}

// daemonBlob reads the text of a daemon answer that is a Blob.
func daemonBlob(body []byte) (command.Blob, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, errors.New("something that is not a text answer (" + err.Error() + ")")
	}
	value, held := envelope[blobAnswerKey]
	if !held {
		return nil, errors.New("an answer that holds no text")
	}
	if len(envelope) != 1 {
		return nil, errors.New("an answer that holds more than its text")
	}
	blob, err := command.DecodeBlob(value)
	if err != nil {
		return nil, errors.New("a text this ze does not read (" + err.Error() + ")")
	}
	return blob, nil
}

// localAnswerRefuses reports whether a local answer of fmt or graph is a
// refusal of the config. The --check verdict of fmt exits 1 and is an answer.
func localAnswerRefuses(answer command.LocalAnswer) bool {
	return answer.Code == exitError
}

// fmtAsking answers `ze config fmt` for input, read from configPath, on the
// operator's own route: the running ze formats a config that declares an
// external plugin, and this process draws the result and makes the --check,
// --diff and -w choices from it.
func fmtAsking(configPath string, input []byte, check, diff, write bool, user string, ask daemonAsk) command.LocalAnswer {
	var options []string
	if write {
		options = append(options, "-w")
	}
	if check {
		options = append(options, "--check")
	}
	if diff {
		options = append(options, "--diff")
	}
	route := routedAnswer[command.LocalAnswer]{
		question: fileQuestion(pathSuppliedFmt, pathShowFmt, localFmt, configPath, input, options),
		sources:  []config.ConfigSource{{Text: string(input), Path: configPath}},
		local: func() command.LocalAnswer {
			return fmtBytesAnswer(configPath, input, check, diff, write)
		},
		refuses: localAnswerRefuses,
		drawn: func(verdict daemonVerdict) (command.LocalAnswer, error) {
			formatted, err := daemonBlob(verdict.body)
			if err != nil {
				return command.LocalAnswer{}, err
			}
			var diag textbuf.Buffer
			writeDaemonWarnings(&diag, verdict.warnings)
			return fmtFormattedAnswer(&diag, configPath, input, string(formatted), check, diff, write), nil
		},
		refusal: func(text string) command.LocalAnswer {
			return command.LocalRefusal(exitError,
				command.LocalErrorLine(daemonRefusal(configSubject(configPath), validateFix(configPath), text)))
		},
	}
	answer := route.answer(user, ask)
	answer.Diagnostic = fixNamingFile(answer.Diagnostic, configPath)
	return answer
}

// graphAsking answers `ze config graph` for data, read from path, on the
// operator's own route.
func graphAsking(path string, data []byte, user string, ask daemonAsk) command.LocalAnswer {
	question := fileQuestion(pathSuppliedGraph, pathShowGraph, localGraph, path, data, nil)
	question.fileLeaf = true
	route := routedAnswer[command.LocalAnswer]{
		question: question,
		sources:  []config.ConfigSource{{Text: string(data), Path: path}},
		local:    func() command.LocalAnswer { return graphBytesAnswer(data) },
		refuses:  localAnswerRefuses,
		drawn: func(verdict daemonVerdict) (command.LocalAnswer, error) {
			graph, err := daemonBlob(verdict.body)
			if err != nil {
				return command.LocalAnswer{}, err
			}
			var diag textbuf.Buffer
			writeDaemonWarnings(&diag, verdict.warnings)
			return command.LocalAnswer{Payload: graph, Code: exitOK, Diagnostic: diag.String()}, nil
		},
		refusal: func(text string) command.LocalAnswer {
			return command.LocalRefusal(exitError,
				command.LocalErrorLine(daemonRefusal(configSubject(path), validateFix(path), text)))
		},
	}
	answer := route.answer(user, ask)
	answer.Diagnostic = fixNamingFile(answer.Diagnostic, path)
	return answer
}

// dumpOutcome is one dump with the lines the operator reads beside it. A
// refusal holds a nonzero code and no result.
type dumpOutcome struct {
	result dumpResult
	diag   string
	code   int
}

// dumpAsking answers the dump of the config at configPath, read from readPath
// (the path the route may read, command.CallerPath), with what the operator
// must read in diag.
//
// On the operator's own route the running ze resolves a config that declares
// an external plugin. On the daemon route nothing is asked: the daemon is the
// process that answers.
func dumpAsking(configPath, readPath string, stripPrivate bool, route command.Route, user string, ask daemonAsk, diag *textbuf.Buffer) (dumpResult, int) {
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		diag.Str("Error reading config: ").Err(err).Byte('\n')
		return dumpResult{}, 1
	}
	local := func() dumpOutcome {
		var own textbuf.Buffer
		result, err := resolveDumpBytes(data, stripPrivate, &own)
		if err != nil {
			own.Str(command.LocalErrorLine(err))
			return dumpOutcome{diag: own.String(), code: 1}
		}
		return dumpOutcome{result: result, diag: own.String()}
	}
	if !route.CallerLocal() {
		outcome := local()
		diag.Str(outcome.diag)
		return outcome.result, outcome.code
	}
	var options, flags []string
	if stripPrivate {
		options = append(options, flagStripPrivate)
		flags = append(flags, leafStripPrivate)
	}
	question := fileQuestion(pathSuppliedDump, pathShowDump, localDump, configPath, data, options)
	question.flags = flags
	routed := routedAnswer[dumpOutcome]{
		question: question,
		sources:  []config.ConfigSource{{Text: string(data), Path: configPath}},
		local:    local,
		refuses:  func(outcome dumpOutcome) bool { return outcome.code != 0 },
		drawn: func(verdict daemonVerdict) (dumpOutcome, error) {
			result, err := daemonDump(verdict.body, string(data), configPath, stripPrivate)
			if err != nil {
				return dumpOutcome{}, err
			}
			var warned textbuf.Buffer
			writeDaemonWarnings(&warned, verdict.warnings)
			return dumpOutcome{result: result, diag: warned.String()}, nil
		},
		refusal: func(text string) dumpOutcome {
			refused := daemonRefusal(configSubject(configPath), validateFix(configPath), text)
			return dumpOutcome{diag: command.LocalErrorLine(refused), code: 1}
		},
	}
	outcome := routed.answer(user, ask)
	diag.Str(fixNamingFile(outcome.diag, configPath))
	return outcome.result, outcome.code
}

// daemonDump reads the document a daemon answered for a dump of text, the
// config at path, into what the two local spellings draw: the document itself
// for `ze show config dump`, and the bgp block and the plugin declarations for
// `ze config dump`.
//
// The daemon masked the document, with the mask the strip-private leaf asked
// for. The names printConfig masks by are read from the schema compiled into
// this binary, as the local dump reads them: the block it prints, bgp, is
// compiled in.
func daemonDump(body []byte, text, path string, stripPrivate bool) (dumpResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	// A number keeps the digits the daemon wrote, where a float would print
	// in another form.
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return dumpResult{}, errors.New("something that is not a config dump (" + err.Error() + ")")
	}
	if document == nil {
		return dumpResult{}, errors.New("an empty answer in place of a config dump")
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return dumpResult{}, errors.New("a config dump this ze cannot draw, because its own schema does not build (" + err.Error() + ")")
	}
	mode := config.DisplayEncode
	if stripPrivate {
		mode = config.DisplayStrip
	}
	bgpTree, _ := document[keyBGP].(map[string]any)
	return dumpResult{
		dumpMap:       document,
		bgpTree:       bgpTree,
		plugins:       documentPlugins(document, text, path),
		sensitiveKeys: config.SensitiveKeys(schema),
		mode:          mode,
	}, nil
}

// documentPlugins answers the external plugin declarations of a dump document
// in the order printConfig prints them: the order the config declares them
// (config.FirstPass over text), then any declaration the first pass does not
// answer (one with no run line), by name.
func documentPlugins(document map[string]any, text, path string) []dumpPlugin {
	root, _ := document[keyBusRoot].(map[string]any)
	allEndpoints, _ := root[keyBusEndpoint].(map[string]any)
	if len(allEndpoints) == 0 {
		return nil
	}
	entries := make(map[string]map[string]any, len(allEndpoints))
	for name, raw := range allEndpoints {
		ep, _ := raw.(map[string]any)
		if ep == nil {
			continue
		}
		if _, hasExt := ep[keyPluginExternal].(map[string]any); hasExt {
			entries[name] = ep
		}
	}
	if len(entries) == 0 {
		return nil
	}
	var names []string
	if firstPass, err := config.FirstPass(text, path, nil); err == nil {
		for _, external := range firstPass.External {
			if _, held := entries[external.Name]; held {
				names = append(names, external.Name)
			}
		}
	}
	var others []string
	for name := range entries {
		if !slices.Contains(names, name) {
			others = append(others, name)
		}
	}
	slices.Sort(others)
	names = append(names, others...)

	plugins := make([]dumpPlugin, 0, len(names))
	for _, name := range names {
		plugin := dumpPlugin{name: name}
		entry := entries[name]
		ext, _ := entry[keyPluginExternal].(map[string]any)
		if run, held := ext[keyPluginRun].(string); held {
			plugin.run, plugin.hasRun = run, true
		}
		if encoder, held := entry[keyPluginEncoding].(string); held {
			plugin.encoder, plugin.hasEncoder = encoder, true
		}
		plugins = append(plugins, plugin)
	}
	return plugins
}

// diffOutcome is one comparison with the lines the operator reads beside it.
// A refusal holds a nonzero code and no diff.
type diffOutcome struct {
	diff *config.ConfigDiff
	diag string
	code int
}

// diffAsking answers the comparison `ze config diff` and `ze show config diff`
// print for args, with what the operator must read in diag. typed is args as
// the operator gave them to `ze config diff`, with the --backup option when
// one was given, for the command a failed ask names.
//
// Both sides are read here, once, in every form the command has. On the
// operator's own route the running ze compares two configs of which one
// declares an external plugin. On the daemon route nothing is asked.
func diffAsking(store storage.Storage, args []string, typed string, route command.Route, user string, ask daemonAsk, diag *textbuf.Buffer) (*config.ConfigDiff, int) {
	inputs, code := readDiffInputs(store, args, diag)
	if code != exitOK {
		return nil, code
	}
	local := func() diffOutcome {
		var own textbuf.Buffer
		diff, code := compareDiffInputs(inputs, &own)
		return diffOutcome{diff: diff, diag: own.String(), code: code}
	}
	asks := route.CallerLocal()
	if inputs.secondErr != nil {
		// A side that could not be read is refused by the local comparison,
		// after any refusal of the first side. There is nothing to send.
		asks = false
	}
	if !asks {
		outcome := local()
		diag.Str(outcome.diag)
		return outcome.diff, outcome.code
	}
	routed := routedAnswer[diffOutcome]{
		question: diffQuestion(store, inputs, typed),
		sources: []config.ConfigSource{
			{Text: string(inputs.first), Path: inputs.firstName},
			{Text: string(inputs.second), Path: inputs.secondName},
		},
		local:   local,
		refuses: func(outcome diffOutcome) bool { return outcome.diff == nil },
		drawn: func(verdict daemonVerdict) (diffOutcome, error) {
			diff, err := daemonDiff(verdict.body)
			if err != nil {
				return diffOutcome{}, err
			}
			var warned textbuf.Buffer
			writeDaemonWarnings(&warned, verdict.warnings)
			return diffOutcome{diff: diff, diag: warned.String(), code: exitOK}, nil
		},
		refusal: func(text string) diffOutcome {
			// A comparison has two sides, so the fix names neither.
			refused := daemonRefusal("the two configs", validateFix(""), text)
			return diffOutcome{diag: command.LocalErrorLine(refused), code: exitError}
		},
	}
	outcome := routed.answer(user, ask)
	diag.Str(outcome.diag)
	return outcome.diff, outcome.code
}

// diffQuestion is what `ze config diff` asks the running ze about inputs: the
// two sides as one JSON object, which is the content `show config supplied
// diff` takes, and whose encoded size is the one the content bound judges.
//
// The form that reads files itself, `show config diff <file1> <file2>`, is
// named only for two files the operator typed: a revision, a backup artifact
// and standard input are read here and have no file the daemon could open.
func diffQuestion(store storage.Storage, inputs diffInputs, typed string) suppliedQuestion {
	// Two strings always encode: the error of json.Marshal is for a value
	// JSON cannot hold.
	envelope, err := json.Marshal(map[string]string{
		suppliedBefore: string(inputs.first),
		suppliedAfter:  string(inputs.second),
	})
	if err != nil {
		panic("BUG: two config texts did not encode as JSON: " + err.Error())
	}
	question := suppliedQuestion{
		supplied:     pathSuppliedDiff,
		content:      envelope,
		local:        localDiff,
		operands:     typed,
		subject:      "the two configs",
		placeholders: []string{"<before-file>", "<after-file>"},
	}
	if store != nil {
		return question
	}
	if inputs.fromRevision {
		return question
	}
	if !namesConfigFile(inputs.firstName) {
		return question
	}
	if !namesConfigFile(inputs.secondName) {
		return question
	}
	question.byPath = pathShowDiff
	question.files = []string{inputs.firstName, inputs.secondName}
	return question
}

// daemonDiff reads the document a daemon answered for a comparison (diffAnswer)
// into the diff both local spellings draw.
func daemonDiff(body []byte) (*config.ConfigDiff, error) {
	var document struct {
		Added   map[string]any             `json:"added"`
		Removed map[string]any             `json:"removed"`
		Changed map[string]config.DiffPair `json:"changed"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("something that is not a config comparison (" + err.Error() + ")")
	}
	diff := &config.ConfigDiff{Added: document.Added, Removed: document.Removed, Changed: document.Changed}
	if diff.Added == nil {
		diff.Added = map[string]any{}
	}
	if diff.Removed == nil {
		diff.Removed = map[string]any{}
	}
	if diff.Changed == nil {
		diff.Changed = map[string]config.DiffPair{}
	}
	return diff, nil
}
