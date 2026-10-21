package peer

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// notifyPath is the command path the model declares notify's leaves under.
const notifyPath = "request peer notify"

// notifySelector is the peer selector a route binds before it validates the
// leaves after notify, as the dispatcher does from `request peer <sel>`.
var notifySelector = map[string]string{"selector": "192.0.2.1"}

// notifyArgs judges tokens against the leaves the model declares for notify,
// as every route does before it calls the handler, and fails the test when the
// model refuses them.
func notifyArgs(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	validated, err := command.ValidateModelArgs(notifyPath, tokens, notifySelector)
	require.NoError(t, err, "tokens %q", tokens)
	return validated
}

// TestHandlerNotify pins what request peer <sel> notify hands the reactor.
//
// GOAL: the code and the subcode an operator types reach the reactor
// unchanged with the Data field the text or the hex form builds, and the
// answer echoes that Data field in hex.
// METHOD: judge the tokens against the model, as a route does, call the
// handler on a mock reactor and compare each recorded field. The positional
// form, the keyword form command.WriteInvocation writes for MCP and the web
// form, and a text holding the names of notify's own leaves are each covered.
//
// VALIDATES: handlePeerNotify reading its leaves, and ze:rest-of-line on text.
// PREVENTS: the code dropped (the teardown handler always sends Cease), the
// text not passed on, the keyword form refused, or a text word such as code
// or hex read as a keyword.
func TestHandlerNotify(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		code    uint8
		subcode uint8
		data    string
	}{
		{"cease with no text", []string{"6", "2"}, 6, 2, ""},
		{"another code with text", []string{"1", "2", "bad", "length"}, 1, 2, "626164206C656E677468"},
		{"cease admin shutdown text", []string{"6", "2", "test"}, 6, 2, "0474657374"},
		{"hex verbatim under another code", []string{"3", "1", "hex", "5c78e9ff"}, 3, 1, "5C78E9FF"},
		{"hex verbatim under cease, a wrong length octet kept", []string{"6", "2", "hex", "0961"}, 6, 2, "0961"},
		{"hex at the Data field's maximum", []string{"3", "1", "hex", strings.Repeat("ab", message.MaxNotificationDataLen)}, 3, 1, strings.Repeat("AB", message.MaxNotificationDataLen)},
		{"hex empty", []string{"3", "1", "hex", ""}, 3, 1, ""},
		{"keyword form with hex", []string{"code", "3", "subcode", "1", "hex", "ff"}, 3, 1, "FF"},
		{"keyword form with a quoted text", []string{"code", "6", "subcode", "2", "text", "shut down"}, 6, 2, "097368757420646F776E"},
		{"keyword form, text words naming leaves", []string{"code", "1", "subcode", "2", "text", "a", "code", "b"}, 1, 2, "6120636F64652062"},
		{"subcode by keyword, code positional", []string{"subcode", "1", "3"}, 3, 1, ""},
		{"positional text naming every leaf", []string{"1", "2", "code", "subcode", "text", "hex"}, 1, 2, "636F646520737562636F6465207465787420686578"},
		{"positional text starting with text", []string{"1", "2", "text", "hex", "ff"}, 1, 2, "686578206666"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := newTestContext(reactor)
			ctx.Peer = "192.0.2.1"
			resp, err := handlePeerNotify(ctx, notifyArgs(t, tc.args...))
			require.NoError(t, err)
			assert.Equal(t, plugin.StatusDone, resp.Status)
			require.Len(t, reactor.teardownCalls, 1)
			call := reactor.teardownCalls[0]
			assert.Equal(t, netip.MustParseAddr("192.0.2.1"), call.addr)
			assert.Equal(t, tc.code, call.code)
			assert.Equal(t, tc.subcode, call.subcode)
			assert.Equal(t, strings.ToLower(tc.data), hex.EncodeToString(call.data))
			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok)
			if tc.data == "" {
				assert.NotContains(t, data, "data")
				return
			}
			assert.Equal(t, tc.data, data["data"])
		})
	}
}

// TestHandlerNotifyRefusals pins the refusals for a line that does not carry
// two octets.
//
// VALIDATES: a missing subcode, a word, and a value above 255 are refused by
// the model's leaves before the handler runs; a word in front of the keyword
// form, which fills no leaf, is refused by the handler with the form to type;
// nothing reaches the reactor.
// PREVENTS: a malformed line sent as a NOTIFICATION with a zero code, or a
// stray word dropped in silence.
func TestHandlerNotifyRefusals(t *testing.T) {
	for _, args := range [][]string{{"6"}, {"six", "2"}, {"6", "256"}, {"code", "6"}} {
		_, err := command.ValidateModelArgs(notifyPath, args, notifySelector)
		require.Error(t, err, "args %v", args)
	}
	for _, args := range [][]string{{"x", "code", "6", "subcode", "2"}, {"x", "code", "6", "subcode", "2", "hex", "ff"}} {
		reactor := &mockReactor{}
		ctx := newTestContext(reactor)
		ctx.Peer = "192.0.2.1"
		resp, err := handlePeerNotify(ctx, notifyArgs(t, args...))
		require.Error(t, err)
		assert.True(t, errors.Is(err, errNotifyUsage), "args %v: %v", args, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "got 'x'")
		assert.Contains(t, resp.Error, "request peer <selector> notify <code> <subcode> [text | hex <octets>]")
		assert.Empty(t, reactor.teardownCalls)
	}
}

// TestHandlerNotifyRefusesTextThatDoesNotFit pins that notify refuses a text it
// cannot send whole, before the reactor is asked to close anything.
//
// GOAL: an operator's NOTIFICATION text reaches the wire whole or not at all.
// METHOD: one octet past the RFC 9003 Section 3 limit under Cease 2 and 4, one
// past the Data field's maximum under another pair, and an invalid UTF-8
// octet; each must be refused with the fix, and no teardown recorded.
//
// VALIDATES: handlePeerNotify's check through message.BuildTeardownData.
// PREVENTS: a silent cut that sends a text the operator never typed.
func TestHandlerNotifyRefusesTextThatDoesNotFit(t *testing.T) {
	cases := []struct {
		name string
		args []string
		fix  string
	}{
		{"communication past 128 octets", []string{"6", "2", strings.Repeat("a", message.MaxShutdownMessageLen+1)}, "shorten the text to 128 octets or fewer"},
		{"reset communication past 128 octets", []string{"6", "4", strings.Repeat("a", 100), strings.Repeat("b", 28)}, "shorten the text to 128 octets or fewer"},
		// 2038 two-octet characters: inside the model's 4075 characters, one
		// octet past the Data field's 4075 octets.
		{"Data past the field's maximum", []string{"3", "1", strings.Repeat("é", (message.MaxNotificationDataLen+1)/2)}, "shorten it to 4075 octets or fewer"},
		{"invalid UTF-8", []string{"6", "2", "bad\xff"}, "retype the text in UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := newTestContext(reactor)
			ctx.Peer = "192.0.2.1"
			resp, err := handlePeerNotify(ctx, notifyArgs(t, tc.args...))
			require.Error(t, err)
			assert.True(t, errors.Is(err, message.ErrTeardownTextRefused), "%v", err)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, tc.fix)
			assert.Contains(t, resp.Error, "the session stays up")
			assert.Empty(t, reactor.teardownCalls, "a refused text must not close the session")
		})
	}
}

// TestHandlerNotifyHexRefusals pins the refusals of the hex form.
//
// GOAL: hex octets reach the wire exactly as typed or not at all.
// METHOD: one octet past the Data field's maximum, an odd digit count, a
// character that is not a digit, and hex given beside a text; each must be
// refused with what was typed and the fix, and no teardown recorded. The
// keyword hex with nothing after it never reaches the handler: the model
// refuses it, naming the form to type, and the second loop pins that.
//
// VALIDATES: notifyData's hex branch, and missingValueError for a bare hex.
// PREVENTS: a hex word decoded partly, a Data field too long for the wire
// reported as a text refusal, or a refusal with no fix.
func TestHandlerNotifyHexRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want error
		fix  string
	}{
		{"odd digit count", []string{"3", "1", "hex", "abc"}, errNotifyHex, "each 0-9, a-f or A-F"},
		{"not a hex digit", []string{"3", "1", "hex", "zz"}, errNotifyHex, "'zz' is not hexadecimal octets"},
		{"hex and a text", []string{"3", "1", "hex", "00", "11"}, errNotifyBoth, "give the Data field once"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := newTestContext(reactor)
			ctx.Peer = "192.0.2.1"
			resp, err := handlePeerNotify(ctx, notifyArgs(t, tc.args...))
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "%v", err)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, tc.fix)
			assert.Contains(t, resp.Error, "the session stays up")
			assert.Empty(t, reactor.teardownCalls, "a refused hex form must not close the session")
			assert.NotContains(t, resp.Error, message.ErrTeardownTextRefused.Error(), "hex octets are not a text")
		})
	}
	// Past 4075 octets the model's length on hex refuses the digits before the
	// handler runs; notifyData refuses the same octets again, its own wording
	// naming octets rather than a text, for a caller that skips the model.
	_, err := command.ValidateModelArgs(notifyPath, []string{"3", "1", "hex", strings.Repeat("00", message.MaxNotificationDataLen+1)}, notifySelector)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "length 8152 out of range 0..8150: give hex a value of 0..8150 characters")
	_, err = notifyData(3, 1, "", false, strings.Repeat("00", message.MaxNotificationDataLen+1), true)
	require.ErrorIs(t, err, errNotifyHex)
	assert.Contains(t, err.Error(), "give 4075 octets (8150 hex digits) or fewer")
	assert.NotContains(t, err.Error(), message.ErrTeardownTextRefused.Error())
	for _, args := range [][]string{{"3", "1", "hex"}, {"code", "3", "subcode", "1", "hex"}} {
		_, err := command.ValidateModelArgs(notifyPath, args, notifySelector)
		require.Error(t, err, "args %v", args)
		assert.Contains(t, err.Error(), "write the value right after the keyword, as in 'hex <hex>'")
	}
}

// TestHandlerTeardownStaysCease pins that teardown keeps sending Cease with the
// subcode's name as the communication when none is typed.
//
// VALIDATES: handleTeardown passes code 6 and the Shutdown Communication of the
// default text under 2 and 4,
// no text under another subcode, and a long communication cut to 128 octets as
// teardown always has (notify refuses it instead).
// PREVENTS: teardown's octets changing because notify shares its reactor call.
func TestHandlerTeardownStaysCease(t *testing.T) {
	cases := []struct {
		args []string
		text string
	}{
		{[]string{"2"}, "\x17Administrative Shutdown"},
		{[]string{"4", "back", "soon"}, "\x09back soon"},
		{[]string{"3", "dropped"}, ""},
		{[]string{"2", strings.Repeat("a", 130)}, "\x80" + strings.Repeat("a", message.MaxShutdownMessageLen)},
	}
	for _, tc := range cases {
		reactor := &mockReactor{}
		ctx := newTestContext(reactor)
		ctx.Peer = "192.0.2.1"
		_, err := handleTeardown(ctx, commandtest.Args(tc.args...))
		require.NoError(t, err)
		require.Len(t, reactor.teardownCalls, 1)
		assert.Equal(t, uint8(6), reactor.teardownCalls[0].code)
		assert.Equal(t, tc.text, string(reactor.teardownCalls[0].data), "args %v", tc.args)
	}
}
