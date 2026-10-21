// Design: docs/architecture/api/commands.md — BGP peer lifecycle and introspection handlers
// Related: peer.go — registration and the teardown handler, the Cease-only form
// RFC: rfc/short/rfc4271.md -- Section 4.5, NOTIFICATION Message Format

package peer

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errNotifyUsage answers a notify line that does not start with two octets.
var errNotifyUsage = errors.New("notify takes an error code and an error subcode, each 0 to 255")

// errNotifyHex answers a hex form whose octets do not decode or do not fit.
var errNotifyHex = errors.New("notify hex refused")

// errNotifyBoth answers a call that gave the Data field twice, as text and as hex.
var errNotifyBoth = errors.New("notify takes a text or hex octets, not both")

// The leaves of the notify container (ze-peer-cmd.yang). text carries
// ze:rest-of-line, so it holds every word after the subcode, or after its
// keyword, and no word in it is read as a keyword (command.ValidateArgs).
const (
	notifyLeafCode    = "code"
	notifyLeafSubcode = "subcode"
	notifyLeafText    = "text"
	notifyLeafHex     = "hex"
)

// handlePeerNotify handles "request peer <sel> notify", whose leaves are typed
// in either form the dispatcher binds:
//
//	notify <code> <subcode> [<text>]           positional; the text is every word left
//	notify <code> <subcode> hex <octets>       the Data field verbatim, in hexadecimal
//	notify code <code> subcode <subcode> [text <text> | hex <octets>]
//
// The keyword form is the one command.WriteInvocation writes for MCP and the
// web form. code and subcode are read after their keywords or, when typed
// bare, in that order from the positional tokens; text and hex are read only
// as keyed values (command.ValidatedArgs.Keyed), so a text holding the word
// hex or code is a text.
//
// The session closes with a NOTIFICATION carrying exactly that code and
// subcode. It is the form a client testing another implementation needs. The
// ExaBGP bridge sends the hex form for `teardown <code> <subcode> <text>`,
// after it has built the octets ExaBGP 6 sends (convertTeardown), so an
// ExaBGP text never passes through the command tokenizer.
//
// Data that does not fit is refused before the session is touched, never cut:
// a text over 128 octets for a Shutdown Communication (RFC 9003 Section 3), a
// text that is not UTF-8, and any Data field over MaxNotificationDataLen. The
// hex form is sent as given, so it can carry a Shutdown Communication that is
// malformed on purpose.
//
// RFC 4271 Section 4.5: "The contents of the Data field depend upon the Error
// Code and Error Subcode." Any value an octet holds is accepted, as ExaBGP
// accepts it, including codes and subcodes IANA has not assigned.
func handlePeerNotify(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	if _, errResp, err := pluginserver.RequireReactor(ctx); err != nil {
		return errResp, err
	}
	free := validated.PositionalTokens()
	code, free, err := notifyOctet(validated, notifyLeafCode, free)
	if err != nil {
		return notifyUsageRefusal(err.Error())
	}
	subcode, free, err := notifyOctet(validated, notifyLeafSubcode, free)
	if err != nil {
		return notifyUsageRefusal(err.Error())
	}
	// Every word after the two octets is the text's (ze:rest-of-line), so a
	// token left here was typed in front of them and fills no leaf.
	if len(free) > 0 {
		var tb textbuf.Buffer
		return notifyUsageRefusal(tb.Str("a word that fills no argument, got '").Str(free[0]).Str("'").String())
	}
	addr, errResp, err := pluginserver.ResolveSinglePeer(ctx, "notify")
	if err != nil {
		return errResp, err
	}

	text, hasText := validated.Keyed(notifyLeafText)
	digits, hasHex := validated.Keyed(notifyLeafHex)
	data, err := notifyData(message.NotifyErrorCode(code), subcode, text, hasText, digits, hasHex)
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error: tb.Str("notify refused, the session stays up: ").Str(err.Error()).
				Str("\nthen run: request peer <selector> notify <code> <subcode> <text>, or ... notify <code> <subcode> hex <octets>").String(),
		}, fmt.Errorf("notify peer %s: %w", addr, err)
	}
	// RFC 4271 Section 4.5
	if err := ctx.Reactor().TeardownPeer(addr, code, subcode, data); err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("notify failed: ").Str(err.Error()).String(),
		}, fmt.Errorf("notify peer %s: %w", addr, err)
	}

	resp := map[string]any{fieldPeer: addr.String(), "code": code, fieldSubcode: subcode}
	// The answer echoes the Data field that went on the wire, in hex.
	if len(data) > 0 {
		resp["data"] = strings.ToUpper(hex.EncodeToString(data))
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(resp)}, nil
}

// notifyOctet reads leaf as one octet: the value typed after its keyword, or
// else the first of the positional tokens free, which it answers without the
// token it took. The error text names what was typed, or that nothing was.
func notifyOctet(validated command.ValidatedArgs, leaf string, free []string) (uint8, []string, error) {
	word, keyed := validated.Keyed(leaf)
	if !keyed {
		if len(free) == 0 {
			var tb textbuf.Buffer
			return 0, free, errors.New(tb.Str("no ").Str(leaf).Str(" was given").String())
		}
		word, free = free[0], free[1:]
	}
	value, err := strconv.ParseUint(word, 10, 8)
	if err != nil {
		var tb textbuf.Buffer
		return 0, free, errors.New(tb.Str(leaf).Str(" '").Str(word).Str("' is not a whole number from 0 to 255").String())
	}
	return uint8(value), free, nil
}

// notifyData builds the Data field: the octets of the hex leaf as they are,
// or the text encoded by message.BuildTeardownData. Every refusal names what
// was typed and the fix.
func notifyData(code message.NotifyErrorCode, subcode uint8, text string, hasText bool, digits string, hasHex bool) ([]byte, error) {
	if hasText && hasHex {
		return nil, fmt.Errorf("%w: give the Data field once, as words after the subcode or as hex <octets>", errNotifyBoth)
	}
	if !hasHex {
		// RFC 4271 Section 4.5, RFC 9003 Section 2
		return message.BuildTeardownData(code, subcode, text)
	}
	data, err := hex.DecodeString(digits)
	if err != nil {
		return nil, fmt.Errorf("%w: '%s' is not hexadecimal octets (%w): write two digits, each 0-9, a-f or A-F, for every octet, such as 0474657374", errNotifyHex, digits, err)
	}
	// RFC 4271 Section 4.1: "The maximum message size is 4096 octets."
	if len(data) > message.MaxNotificationDataLen {
		return nil, fmt.Errorf("%w: the Data field is %d octets and a NOTIFICATION holds %d after its header; give %d octets (%d hex digits) or fewer", errNotifyHex, len(data), message.MaxNotificationDataLen, message.MaxNotificationDataLen, 2*message.MaxNotificationDataLen)
	}
	return data, nil
}

// notifyUsageRefusal names what is wrong with the code and subcode and the
// form to type.
func notifyUsageRefusal(got string) (*plugin.Response, error) {
	var tb textbuf.Buffer
	reason := tb.Str("notify takes an error code and an error subcode, each 0 to 255: ").Str(got).
		Str("\nrun: request peer <selector> notify <code> <subcode> [text | hex <octets>]").String()
	return &plugin.Response{Status: plugin.StatusError, Error: reason}, errNotifyUsage
}
