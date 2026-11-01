package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// keepaliveHex is a whole KEEPALIVE: the marker, length 19, type 4.
const keepaliveHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"

// judged validates tokens against the leaves ze-bgp-tools-cmd declares for
// path, exactly as the local route does before it calls the handler.
func judged(t *testing.T, path string, tokens ...string) command.ValidatedArgs {
	t.Helper()
	validated, err := command.ValidateModelArgs(path, tokens, nil)
	require.NoError(t, err, "the model refused %s %q", path, tokens)
	return validated
}

// TestBGPToolsLocalGrammarSurvivesTheDeclaredLeaves proves A-6 for these two
// commands: every spelling the local route accepted before the leaves were
// declared still passes the validator the local route runs, and the keyword
// spellings the leaves add bind their values.
func TestBGPToolsLocalGrammarSurvivesTheDeclaredLeaves(t *testing.T) {
	for _, tokens := range [][]string{
		{keepaliveHex},
		{"--open", keepaliveHex},
		{"--json", "--family", "ipv4/flow", "-"},
		{"--nlri", "l2vpn/evpn", "02"},
		{"--plugin", "flowspec", "--keepalive", keepaliveHex},
		{"pcap", "session.pcap"},
		{"--json", "pcap", "-"},
		{"-"},
		{"--help"},
	} {
		judged(t, pathDecode, tokens...)
	}
	for _, tokens := range [][]string{
		{"route 10.0.0.0/24 next-hop 1.2.3.4"},
		{"route", "10.0.0.0/24", "next-hop", "1.2.3.4"},
		{"-f", "ipv4/flow", "match destination-ipv4 10.0.0.0/8 then mark 0"},
		{"-n", "-a", "65000", "-z", "65001", "--asn4=false", "route 10.0.0.0/24 next-hop 1.2.3.4"},
		{"--help"},
		{},
	} {
		judged(t, pathEncode, tokens...)
	}

	keyed := judged(t, pathDecode, "type", "open", "family", "ipv4/unicast", keepaliveHex)
	value, found := keyed.Keyed(leafDecodeType)
	require.True(t, found)
	require.Equal(t, "open", value)
	value, found = keyed.Keyed(leafDecodeMessage)
	require.True(t, found)
	require.Equal(t, keepaliveHex, value)

	_, err := command.ValidateModelArgs(pathDecode, []string{"type", "refresh", keepaliveHex}, nil)
	require.Error(t, err, "type is an enumeration, so a name outside it is refused")
	_, err = command.ValidateModelArgs(pathEncode, []string{"local-as", "0", "route 10.0.0.0/24 next-hop 1.2.3.4"}, nil)
	require.Error(t, err, "AS 0 is outside the declared range")
}

// TestShowBGPDecodeAnswersDataAndWritesNothing calls the registered handler
// with both output streams captured: nothing may reach either, and the answer
// is the decoded message as data, under one envelope key.
func TestShowBGPDecodeAnswersDataAndWritesNothing(t *testing.T) {
	var answer command.LocalAnswer
	stdout, stderr, _ := run(t, func() int {
		answer = dataDecode(judged(t, pathDecode, keepaliveHex))
		return 0
	})
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Equal(t, 0, answer.Code)
	require.Empty(t, answer.Diagnostic)

	encoded, err := json.Marshal(answer.Payload)
	require.NoError(t, err)
	var envelope map[string]struct {
		Input    string `json:"input"`
		Messages []struct {
			Hex     string         `json:"hex"`
			Decoded map[string]any `json:"decoded"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	require.Len(t, envelope, 1)
	result, ok := envelope[keyDecode]
	require.True(t, ok, "answer %s has no %s key", encoded, keyDecode)
	require.Equal(t, inputFormMessage, result.Input)
	require.Len(t, result.Messages, 1)
	require.Equal(t, keepaliveHex, result.Messages[0].Hex)
	require.Equal(t, "bgp", result.Messages[0].Decoded["type"])
}

// TestShowBGPDecodeLocalOutputIsUnchanged renders the handler's answer on both
// local routes and compares the bytes with what the decoder printed before the
// conversion: its human text, or its compact JSON, and a newline.
func TestShowBGPDecodeLocalOutputIsUnchanged(t *testing.T) {
	human, err := decodeHexPacket(keepaliveHex, "", "", false)
	require.NoError(t, err)
	compact, err := decodeHexPacket(keepaliveHex, "", "", true)
	require.NoError(t, err)

	stdout, stderr, code := run(t, func() int {
		return renderDecode(pathDecode, dataDecode(judged(t, pathDecode, keepaliveHex)).Payload)
	})
	require.Equal(t, 0, code)
	require.Empty(t, stderr)
	require.Equal(t, human+"\n", stdout)

	stdout, _, code = run(t, func() int {
		return renderDecode(pathDecode, dataDecode(judged(t, pathDecode, "--json", keepaliveHex)).Payload)
	})
	require.Equal(t, 0, code)
	require.Equal(t, compact+"\n", stdout)

	stdout, _, code = run(t, func() int { return cmdDecode([]string{keepaliveHex}) })
	require.Equal(t, 0, code)
	require.Equal(t, human+"\n", stdout)

	// A payload that does not decode printed its reason on stdout and exited 1.
	stdout, stderr, code = run(t, func() int { return cmdDecode([]string{"ZZ"}) })
	require.Equal(t, 1, code)
	require.Empty(t, stderr)
	require.Equal(t, "Error: invalid hex: encoding/hex: invalid byte: U+005A 'Z'\n", stdout)
	stdout, _, code = run(t, func() int { return cmdDecode([]string{"--json", "ZZ"}) })
	require.Equal(t, 1, code)
	require.Equal(t, `{"error":"invalid hex: encoding/hex: invalid byte: U+005A 'Z'","parsed":false}`+"\n", stdout)
}

// TestShowBGPDecodeBadFlagIsRefusedNotExited proves a word the flag parser
// refuses comes back as the refusal, with the parser's own text, instead of
// exiting the process from inside the handler: this test would not continue
// past the call if it exited.
func TestShowBGPDecodeBadFlagIsRefusedNotExited(t *testing.T) {
	var answer command.LocalAnswer
	stdout, stderr, _ := run(t, func() int {
		answer = dataDecode(judged(t, pathDecode, "--bogus", keepaliveHex))
		return 0
	})
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Nil(t, answer.Payload)
	require.Equal(t, 2, answer.Code, "the exiting parser exited 2 on a refused word")
	require.Contains(t, answer.Diagnostic, "flag provided but not defined: -bogus\n")
	require.Contains(t, answer.Diagnostic, "ze bgp decode")

	answer = dataDecode(judged(t, pathDecode, "--help"))
	require.Nil(t, answer.Payload)
	require.Equal(t, 0, answer.Code, "the exiting parser exited 0 on a help request")
	require.Contains(t, answer.Diagnostic, "The hex payload can include colons or spaces")

	answer = dataDecode(judged(t, pathDecode, "type", "open", "--keepalive", keepaliveHex))
	require.Nil(t, answer.Payload)
	require.Equal(t, "error: type is given twice, as a keyword and as an option; give it once\n", answer.Diagnostic)
}

// TestBGPToolsStdinFormsNeedTheLocalProcess proves a route with no standard
// input refuses every form that reads it, naming the local command, while the
// same words on the local route are not refused for that reason.
func TestBGPToolsStdinFormsNeedTheLocalProcess(t *testing.T) {
	answer := answerDecode(command.ValidatedArgs{}, []string{"-"}, inputRouteRemote)
	require.Nil(t, answer.Payload)
	require.Equal(t, 1, answer.Code)
	require.Equal(t, "error: show bgp decode - reads standard input, which reaches only the ze process "+
		"the operator runs; run 'ze show bgp decode -' on the host that holds the input\n", answer.Diagnostic)

	answer = answerDecode(command.ValidatedArgs{}, []string{"pcap", "-"}, inputRouteRemote)
	require.Nil(t, answer.Payload)
	require.Contains(t, answer.Diagnostic, "run 'ze show bgp decode pcap -'")

	answer = answerEncode(command.ValidatedArgs{}, nil, inputRouteRemote)
	require.Nil(t, answer.Payload)
	require.Equal(t, 1, answer.Code)
	require.Contains(t, answer.Diagnostic, "error: missing route command: give the route after the command")

	answer = answerDecode(command.ValidatedArgs{}, []string{keepaliveHex}, inputRouteRemote)
	require.Equal(t, 0, answer.Code, "a payload on the line needs no standard input")
}

// TestShowBGPEncodeAnswersDataAndWritesNothing calls the registered handler
// with every output captured and compares its answer with what the root route
// prints, which is the output the command printed before the conversion.
func TestShowBGPEncodeAnswersDataAndWritesNothing(t *testing.T) {
	const route = "route 10.0.0.0/24 next-hop 192.168.1.1"

	var printed, printedErr bytes.Buffer
	oldStdout, oldStderr := encodeStdout, encodeStderr
	encodeStdout, encodeStderr = &printed, &printedErr
	defer func() { encodeStdout, encodeStderr = oldStdout, oldStderr }()

	var answer command.LocalAnswer
	stdout, stderr, _ := run(t, func() int {
		answer = dataEncode(judged(t, pathEncode, "-n", route))
		return 0
	})
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Empty(t, printed.String(), "the handler wrote to the root route's writer")
	require.Equal(t, 0, answer.Code)
	encoded, ok := answer.Payload.(encodeAnswer)
	require.True(t, ok)
	require.Equal(t, encodePartNLRI, encoded.Encode.Part)

	require.Equal(t, 0, cmdEncode([]string{"-n", route}))
	require.Equal(t, encoded.Encode.Hex+"\n", printed.String())
	require.Empty(t, printedErr.String())

	stdout, _, code := run(t, func() int {
		return command.RenderLocalAnswer(pathEncode, answer.Payload)
	})
	require.Equal(t, 0, code)
	require.Equal(t, printed.String(), stdout, "the text route prints what the root route prints")

	keyword := dataEncode(judged(t, pathEncode, "part", "nlri", route))
	require.Equal(t, answer.Payload, keyword.Payload, "the part keyword answers what -n answers")

	refused := dataEncode(judged(t, pathEncode, "-q", route))
	require.Nil(t, refused.Payload)
	require.Equal(t, 1, refused.Code)
	require.Contains(t, refused.Diagnostic, "flag provided but not defined: -q\n")
}
