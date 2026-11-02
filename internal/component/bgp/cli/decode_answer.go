// Design: docs/architecture/api/commands.md — local-data commands and their answer
// Overview: decode.go — the decoder every input form runs through
// Related: decode_pcap.go — the capture and standard-input forms
// Related: register.go — the registration and the text drawings

package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The command paths, the envelope keys of their answers, and the YANG leaves
// of ze-bgp-tools-cmd that select what is answered. The leaf names are the
// keywords the validator binds, so each is spelled once, here and in the
// module.
const (
	pathDecode = "show bgp decode"
	pathEncode = "show bgp encode"

	keyDecode = "bgp-decode"
	keyEncode = "bgp-encode"

	leafDecodeType    = "type"
	leafDecodeNLRI    = "nlri"
	leafDecodeFamily  = "family"
	leafDecodePcap    = "pcap"
	leafDecodePlugin  = "plugin"
	leafDecodeMessage = "message"
)

// The input forms of a decode, as its answer names them. The form decides
// where the operator reads a message that did not decode: the single payload
// has always printed its reason on the output, and the capture and
// standard-input forms on stderr, beside the messages that did decode.
const (
	inputFormMessage = "message"
	inputFormStdin   = "stdin"
	inputFormPcap    = "pcap"
)

// stdinRefusal is the answer of a form that reads standard input on a route
// that has none. It names the local command, which is the fix.
func stdinRefusal(invocation string) command.LocalAnswer {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(invocation).
		Str(" reads standard input, which reaches only the ze process the operator runs; run 'ze ").
		Str(invocation).Str("' on the host that holds the input\n")
	return command.LocalRefusal(1, tb.String())
}

// decodeAnswer is the answer of show bgp decode: one envelope key, so the text
// drawing registered under it renders the text route.
//
// asJSON records the local `--json` rendering flag. It is not part of the
// data, so JSON never carries it: the daemon route has no rendering flag, and
// its caller's `| json` renders the same fields.
type decodeAnswer struct {
	Decode decodeResult `json:"bgp-decode"`
	asJSON bool
}

// decodeResult is every message one decode read, the form it was read from,
// and the reading it was decoded under. MessageType is empty when each
// message's own header named its type. Plugins is the plugins the operator
// named, as typed, which decoded capabilities and NLRI before the registered
// decoders.
type decodeResult struct {
	Input       string           `json:"input"`
	MessageType string           `json:"message-type,omitempty"`
	Family      string           `json:"family,omitempty"`
	Plugins     []string         `json:"plugin,omitempty"`
	Messages    []decodedMessage `json:"messages"`
}

// decodedMessage is one message: the hexadecimal it was read as, and either
// its decoding in the Ze event format or the reason it has none. Time and
// Flow name where a capture held it, and are empty for the other forms.
type decodedMessage struct {
	Time    string          `json:"time,omitempty"`
	Flow    string          `json:"flow,omitempty"`
	Hex     string          `json:"hex"`
	Decoded json.RawMessage `json:"decoded,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// decodeRequest is the decode the words asked for, before any input is read.
type decodeRequest struct {
	msgType string
	family  string
	plugins decodePlugins
	asJSON  bool
	// inputs is the positional words: one hex payload (possibly split on
	// spaces), the stdin token, or the pcap keyword and its path.
	inputs []string
}

// answer wraps the messages read from one input form in the answer of this
// request.
func (r *decodeRequest) answer(input string, messages []decodedMessage, code int, diagnostic string) command.LocalAnswer {
	return command.LocalAnswer{
		Payload: decodeAnswer{
			Decode: decodeResult{
				Input: input, MessageType: r.msgType, Family: r.family,
				Plugins: r.plugins.spellings(), Messages: messages,
			},
			asJSON: r.asJSON,
		},
		Code:       code,
		Diagnostic: diagnostic,
	}
}

// dataDecode is the show bgp decode handler. It writes nothing: every word it
// has for the operator is in the answer.
func dataDecode(validated command.ValidatedArgs) command.LocalAnswer {
	return refuseUndecoded(answerDecode(validated, restWords(validated, leafDecodeMessage), validated.Route()))
}

// refuseUndecoded turns the one payload typed on the line that did not decode
// into a refusal that names the reason. The root `ze bgp decode` route keeps
// printing that reason on stdout, which its scripts read. show bgp decode
// answers on every surface, and a nonzero answer that carried its reason only
// inside the payload reached the daemon route as an error with no reason.
func refuseUndecoded(answer command.LocalAnswer) command.LocalAnswer {
	if answer.Code == 0 {
		return answer
	}
	decoded, ok := answer.Payload.(decodeAnswer)
	if !ok {
		return answer
	}
	if decoded.Decode.Input != inputFormMessage {
		return answer
	}
	if len(decoded.Decode.Messages) != 1 {
		return answer
	}
	var tb textbuf.Buffer
	tb.Str("error: ").Str(pathDecode).Str(" could not decode the payload: ").
		Str(decoded.Decode.Messages[0].Error).
		Str("; give one BGP message in hexadecimal, as 'ze bgp decode --help' describes\n")
	return command.LocalRefusal(answer.Code, tb.String())
}

// restWords answers the words a rest-of-line leaf took, split again: on the
// local route they hold the offline flags and the input, which the flag
// parser reads as it always did.
func restWords(validated command.ValidatedArgs, leaf string) []string {
	text, found := validated.Keyed(leaf)
	if !found {
		return nil
	}
	return strings.Fields(text)
}

// answerDecode answers a decode. validated carries the leaves the model bound
// by keyword; words carries the rest of the line, offline flags included. The
// root `ze bgp decode` route passes the zero ValidatedArgs, which binds none.
func answerDecode(validated command.ValidatedArgs, words []string, route command.Route) command.LocalAnswer {
	request, refusal := parseDecode(validated, words, route)
	if refusal != nil {
		return *refusal
	}

	// Three input forms, and the keyword decides between them before any value
	// is read (`ai/rules/cli.md`). A capture is named after `pcap`, hexadecimal
	// lines arrive on standard input under the "-" token, and a bare word is
	// the single hex payload this command has always taken.
	first := request.inputs[0]
	if first == inputKeywordPcap {
		return decodePcapInput(request.inputs[1:], &request, route)
	}
	if cliio.IsStdin(first) {
		if len(request.inputs) > 1 {
			var tb textbuf.Buffer
			tb.Str("error: ").Quoted(cliio.StdinToken).
				Str(" reads every message from standard input, so it takes no further argument, and ").
				Str(countWord(len(request.inputs) - 1)).Str(" given\n")
			return command.LocalRefusal(1, tb.String())
		}
		if !route.CallerLocal() {
			return stdinRefusal(pathDecode + " " + cliio.StdinToken)
		}
		return decodeHexStdin(&request)
	}
	return decodeHexInput(strings.Join(request.inputs, " "), &request)
}

// decodeHexInput decodes the one payload typed on the line. A payload that
// does not decode is still an answer, as it always printed one: the reason, on
// the output, with a nonzero exit.
func decodeHexInput(payload string, request *decodeRequest) command.LocalAnswer {
	message := decodedMessage{Hex: payload}
	decoded, err := decodeHexPacket(request.plugins, payload, request.msgType, request.family, true)
	if err != nil {
		message.Error = err.Error()
		return request.answer(inputFormMessage, []decodedMessage{message}, 1, "")
	}
	message.Decoded = json.RawMessage(decoded)
	return request.answer(inputFormMessage, []decodedMessage{message}, 0, "")
}

// parseDecode reads the decode the words and the bound leaves ask for. A word
// the flag parser refuses, a help request, a plugin that cannot decode on this
// route and a missing payload come back as the refusal, with the text the
// parser wrote; nothing reaches stderr.
func parseDecode(validated command.ValidatedArgs, words []string, route command.Route) (decodeRequest, *command.LocalAnswer) {
	var diag textbuf.Buffer
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	fs.SetOutput(&diag)

	openMsg := fs.Bool("open", false, "decode as OPEN message")
	updateMsg := fs.Bool("update", false, "decode as UPDATE message")
	notifMsg := fs.Bool("notification", false, "decode as NOTIFICATION message")
	keepaliveMsg := fs.Bool("keepalive", false, "decode as KEEPALIVE message")
	nlriFamily := fs.String("nlri", "", "decode as NLRI with family (e.g., 'ipv4/flow')")
	fam := fs.String("f", "", "address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')")
	fs.StringVar(fam, "family", "", "address family for UPDATE (long form of -f)")
	outputJSON := fs.Bool("json", false, "output JSON instead of human-readable format")
	var plugins pluginFlags
	fs.Var(&plugins, "plugin", "plugin that decodes capabilities and NLRI first (e.g., ze.bgp-hostname, bgp-nlri-flowspec)")
	fs.Usage = func() { writeDecodeUsage(&diag) }

	if err := fs.Parse(words); err != nil {
		// The help page is what was asked for, so it exits 0, as the exiting
		// parser this replaced did; a refused word exits 2, as it did too.
		if errors.Is(err, flag.ErrHelp) {
			refusal := command.LocalRefusal(0, diag.String())
			return decodeRequest{}, &refusal
		}
		refusal := command.LocalRefusal(2, diag.String())
		return decodeRequest{}, &refusal
	}

	if refusal := applyDecodeLeaves(fs, validated); refusal != nil {
		return decodeRequest{}, refusal
	}

	inputs := fs.Args()
	if path, found := validated.Keyed(leafDecodePcap); found {
		inputs = append([]string{inputKeywordPcap, path}, inputs...)
	}
	if len(inputs) == 0 {
		diag.Str("error: missing hex payload\n")
		writeDecodeUsage(&diag)
		refusal := command.LocalRefusal(1, diag.String())
		return decodeRequest{}, &refusal
	}

	named, err := parseDecodePlugins(plugins, route.CallerLocal())
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		refusal := command.LocalRefusal(1, tb.String())
		return decodeRequest{}, &refusal
	}

	request := decodeRequest{asJSON: *outputJSON, plugins: named, inputs: inputs}
	switch {
	case *openMsg:
		request.msgType = msgTypeOpen
	case *updateMsg:
		request.msgType = msgTypeUpdate
	case *notifMsg:
		request.msgType = msgTypeNotification
	case *keepaliveMsg:
		request.msgType = msgTypeKeepalive
	case *nlriFamily != "":
		request.msgType = msgTypeNLRI
	}

	// NLRI mode names its family in its own option, which wins over -f.
	request.family = *fam
	if *nlriFamily != "" {
		request.family = *nlriFamily
	}
	return request, nil
}

// applyDecodeLeaves sets the options the keyword leaves stand for. A choice
// given both as a keyword and as an option is refused rather than one of the
// two silently winning.
func applyDecodeLeaves(fs *flag.FlagSet, validated command.ValidatedArgs) *command.LocalAnswer {
	given := givenFlags(fs)
	if value, found := validated.Keyed(leafDecodeType); found {
		if given["open"] || given["update"] || given["notification"] || given["keepalive"] || given["nlri"] {
			return givenTwice(leafDecodeType)
		}
		// The enumeration's names are the option names.
		if err := fs.Set(value, "true"); err != nil {
			return setRefusal(leafDecodeType, err)
		}
	}
	if value, found := validated.Keyed(leafDecodeNLRI); found {
		if given["nlri"] {
			return givenTwice(leafDecodeNLRI)
		}
		if err := fs.Set("nlri", value); err != nil {
			return setRefusal(leafDecodeNLRI, err)
		}
	}
	if value, found := validated.Keyed(leafDecodeFamily); found {
		if given["f"] || given["family"] {
			return givenTwice(leafDecodeFamily)
		}
		if err := fs.Set("family", value); err != nil {
			return setRefusal(leafDecodeFamily, err)
		}
	}
	// A list: the keyword's plugin joins the ones the options name, so
	// naming plugins both ways is not a choice made twice.
	if value, found := validated.Keyed(leafDecodePlugin); found {
		if err := fs.Set("plugin", value); err != nil {
			return setRefusal(leafDecodePlugin, err)
		}
	}
	return nil
}

// givenFlags answers the options the words set.
func givenFlags(fs *flag.FlagSet) map[string]bool {
	given := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	return given
}

// givenTwice refuses a choice made twice, once by its keyword and once by its
// offline option.
func givenTwice(leaf string) *command.LocalAnswer {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(leaf).
		Str(" is given twice, as a keyword and as an option; give it once\n")
	refusal := command.LocalRefusal(1, tb.String())
	return &refusal
}

// setRefusal refuses a keyword value the option would not take.
func setRefusal(leaf string, err error) *command.LocalAnswer {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(leaf).Str(": ").Err(err).Byte('\n')
	refusal := command.LocalRefusal(1, tb.String())
	return &refusal
}

// writeDecodeUsage writes the decode help page into diag, colored as stderr
// would be, since the local routes print it there.
func writeDecodeUsage(diag *textbuf.Buffer) {
	p := helpfmt.Page{
		Command:   "ze bgp decode",
		ShortHelp: "Decode BGP message from hexadecimal and output Ze-format JSON",
		Usage: []string{
			"ze bgp decode [options] <hex-payload>",
			"ze bgp decode [options] -                Read one hex message for each line of standard input",
			"ze bgp decode [options] pcap <file>      Decode every BGP message in a capture",
			"ze bgp decode [options] pcap -           Read the capture from standard input",
		},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--open", Desc: "Decode as OPEN message"},
				{Name: "--update", Desc: "Decode as UPDATE message"},
				{Name: "--notification", Desc: "Decode as NOTIFICATION message"},
				{Name: "--keepalive", Desc: "Decode as KEEPALIVE message"},
				{Name: "--nlri <family>", Desc: "Decode as NLRI with family (e.g., 'ipv4/flow')"},
				{Name: "-f <family>", Desc: "Address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')"},
				{Name: "--json", Desc: "Output JSON instead of human-readable format"},
				{Name: "--plugin <name>", Desc: "Plugin that decodes capabilities and NLRI before the registered ones: <name>, ze.<name>, ze-<name> or a program path (repeatable)"},
			}},
		},
		Examples: []string{
			"ze bgp decode --open FFFF...                          Decode OPEN message",
			"ze bgp decode --update FFFF...                        Decode UPDATE message",
			"ze bgp decode --plugin ze.bgp-hostname --open FFFF... Decode with the hostname plugin",
			"ze bgp decode --nlri l2vpn/evpn 02...                 Decode NLRI with family",
			"ze bgp decode --plugin bgp-nlri-flowspec --nlri ipv4/flow 07...  Decode NLRI via plugin",
			"ze bgp decode pcap session.pcap                       Decode a tcpdump or ze capture",
			"tcpdump -w - port 179 | ze bgp decode pcap -           Decode a capture from a pipe",
		},
	}
	p.WriteTo(diag, slogutil.UseColor(os.Stderr))
	diag.Str("\nThe hex payload can include colons or spaces which will be stripped.\n")
}

// drawDecode is the text drawing of a decode: each message as the operator has
// always read it, a capture's messages headed by where they were captured.
// The human reading is decoded again from the hexadecimal the answer carries,
// so the drawing is the decoder's own text and cannot drift from it.
//
// The answer names its plugins as typed, and the drawing asks them again. A
// program path among them was judged by the handler that built the answer,
// which refuses one on a route that may not start programs, so the drawing
// reads them as the local route does.
func drawDecode(result *decodeResult) (string, error) {
	plugins, err := parseDecodePlugins(result.Plugins, true)
	if err != nil {
		return "", err
	}
	var out textbuf.Buffer
	for i := range result.Messages {
		message := &result.Messages[i]
		if message.Time != "" {
			out.Str("# ").Str(message.Time).Str("  ").Str(message.Flow).Byte('\n')
		}
		if message.Error != "" {
			// The capture and standard-input forms reported it on stderr.
			if result.Input == inputFormMessage {
				out.Str("Error: ").Str(message.Error).Byte('\n')
			}
			continue
		}
		text, err := decodeHexPacket(plugins, message.Hex, result.MessageType, result.Family, false)
		if err != nil {
			return "", err
		}
		out.Str(text).Byte('\n')
	}
	return out.String(), nil
}

// drawDecodeJSON is the local `--json` rendering: one compact Ze event for
// each message, and for a payload that did not decode the error object it has
// always printed.
func drawDecodeJSON(result *decodeResult) (string, error) {
	var out textbuf.Buffer
	for i := range result.Messages {
		message := &result.Messages[i]
		if message.Error != "" {
			// The capture and standard-input forms reported it on stderr.
			if result.Input != inputFormMessage {
				continue
			}
			line, err := json.Marshal(map[string]any{"error": message.Error, jsonKeyParsed: false})
			if err != nil {
				return "", err
			}
			out.Str(string(line)).Byte('\n')
			continue
		}
		out.Str(string(message.Decoded)).Byte('\n')
	}
	return out.String(), nil
}

// drawDecodeText is the text renderer registered under keyDecode.
func drawDecodeText(raw json.RawMessage) (string, error) {
	var result decodeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return drawDecode(&result)
}

// renderDecode prints dataDecode's answer on the local route. `--json` prints
// the compact events, as it always did; every other answer goes through the
// configured default format, whose text form is drawDecodeText.
func renderDecode(path string, payload any) int {
	answer, ok := payload.(decodeAnswer)
	if !ok || !answer.asJSON {
		return command.RenderLocalAnswer(path, payload)
	}
	text, err := drawDecodeJSON(&answer.Decode)
	if err != nil {
		os.Stderr.WriteString(command.LocalErrorLine(err)) //nolint:errcheck // CLI diagnostic
		return 1
	}
	os.Stdout.WriteString(text) //nolint:errcheck // CLI output
	return 0
}

// cmdDecode is the root `ze bgp decode` route: it answers through the same
// handler body and prints the tool's own drawing, human or `--json`, which is
// what scripts of that spelling have always read.
func cmdDecode(args []string) int {
	return printDecode(answerDecode(command.ValidatedArgs{}, args, command.RouteLocal))
}

// printDecode prints a decode answer as the root route does: its Diagnostic on
// stderr, then the drawing on stdout, and answers the exit code.
func printDecode(answer command.LocalAnswer) int {
	os.Stderr.WriteString(answer.Diagnostic) //nolint:errcheck // CLI diagnostic
	if answer.Payload == nil {
		return answer.Code
	}
	decoded, ok := answer.Payload.(decodeAnswer)
	if !ok {
		panic("BUG: printDecode: the decode handler answered a foreign payload")
	}
	draw := drawDecode
	if decoded.asJSON {
		draw = drawDecodeJSON
	}
	text, err := draw(&decoded.Decode)
	if err != nil {
		os.Stderr.WriteString(command.LocalErrorLine(err)) //nolint:errcheck // CLI diagnostic
		return 1
	}
	os.Stdout.WriteString(text) //nolint:errcheck // CLI output
	return answer.Code
}
