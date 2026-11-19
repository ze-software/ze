// Design: docs/guide/l2tp.md -- clearing a subscriber session by user name or address
// Related: session_teardown.go -- the id form, and the audit and teardown every form shares
// Related: session_teardown_completion.go -- the id form's live completion

package cmd

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The answer keys of a clear by subscriber attribute. They match the RPC
// input leaves, and username matches the key a session row carries.
const (
	keyUsername = "username"
	keyAddress  = "address"
)

// userNameLengthMax is the RPC leaf's bound, the longest RADIUS User-Name,
// in octets. Section 5.1 leaves the NAS its own limit:
//
// RFC 2865 Section 5.1: "The String field is one or more octets.  The NAS may limit the maximum length of the User-Name but the ability to handle at least 63 octets is recommended."
//
// and the attribute carries at most 253 octets of it:
//
// RFC 2865 Section 5: "text      1-253 octets containing UTF-8 encoded 10646 [7] characters."
const userNameLengthMax = 253

// sessionMatchRequest is the cold input of a clear by one subscriber
// attribute. match reads snapshot rows and grants nothing: only the guarded
// capture of the single matching original selects what is torn down.
type sessionMatchRequest struct {
	// subject names the selector in messages: `user "alice@example.net"`.
	subject string
	// key and value are the answer's echo of what the operator selected.
	key   string
	value string
	match func(*l2tp.SessionSnapshot) bool
	disconnectAudit
}

// handleSessionTeardownUser serves `clear l2tp session user <name>`.
func handleSessionTeardownUser(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	request, refusal := parseSessionTeardownUser(validated.Tokens())
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	return teardownMatchedSession(ctx, request)
}

// handleSessionTeardownAddress serves `clear l2tp session address <ipv4>`.
func handleSessionTeardownAddress(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	request, refusal := parseSessionTeardownAddress(validated.Tokens())
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	return teardownMatchedSession(ctx, request)
}

// parseSessionTeardownUser reads the user name and the optional audit
// keywords. The name is compared byte for byte, as the RADIUS
// Disconnect-Request listener compares User-Name.
func parseSessionTeardownUser(args []string) (sessionMatchRequest, *opererr.Error) {
	if len(args) == 0 {
		return sessionMatchRequest{}, sessionMatchInputError(nil, "L2TP user name is required")
	}
	name := args[0]
	if name == "" {
		return sessionMatchRequest{}, sessionMatchInputError(nil, "L2TP user name is empty")
	}
	// RFC 2865 Section 5.1: the bound counts octets, not characters.
	if len(name) > userNameLengthMax {
		return sessionMatchRequest{}, sessionMatchInputError(nil, "L2TP user name is longer than 253 octets")
	}
	// A dash-leading token is an option to the CLI, so it is refused as a
	// value; the session ID still reaches such a subscriber.
	if strings.HasPrefix(name, "-") {
		return sessionMatchRequest{}, opererr.New(
			"L2TP user name "+strconv.Quote(name)+" is an option, not a user name",
			"A value cannot start with a dash; clear this subscriber by its session ID from show l2tp sessions",
			"show l2tp sessions")
	}
	request := sessionMatchRequest{
		subject: "user " + strconv.Quote(name),
		key:     keyUsername,
		value:   name,
		match:   func(session *l2tp.SessionSnapshot) bool { return session.Username == name },
	}
	return request.withAudit(args)
}

// parseSessionTeardownAddress reads the IPv4 address IPCP assigned to the
// subscriber, and the optional audit keywords.
func parseSessionTeardownAddress(args []string) (sessionMatchRequest, *opererr.Error) {
	if len(args) == 0 {
		return sessionMatchRequest{}, sessionMatchInputError(nil, "L2TP session address is required")
	}
	address, err := netip.ParseAddr(args[0])
	if err != nil {
		return sessionMatchRequest{}, sessionMatchInputError(err, "L2TP session address "+strconv.Quote(args[0])+" is not an IPv4 address")
	}
	if !address.Is4() {
		return sessionMatchRequest{}, sessionMatchInputError(nil, "L2TP session address "+address.String()+" is not an IPv4 address")
	}
	request := sessionMatchRequest{
		subject: "address " + address.String(),
		key:     keyAddress,
		value:   address.String(),
		match:   func(session *l2tp.SessionSnapshot) bool { return session.AssignedAddr == address },
	}
	return request.withAudit(args)
}

// withAudit reads the actor, reason and cause after the selector value in
// args[0]. The id form's qualifiers (expected-id, tunnel-id, listener) are
// refused here: they narrow a session ID, and this form names no ID.
func (request sessionMatchRequest) withAudit(args []string) (sessionMatchRequest, *opererr.Error) {
	var err error
	request.actor, request.reason, request.cause, err = parseKeywordArgs(args)
	if err != nil {
		return sessionMatchRequest{}, sessionMatchInputError(err, "L2TP disconnect details are invalid")
	}
	return request, nil
}

// teardownMatchedSession clears the one live session the request matches.
// The walk is the RADIUS Disconnect-Request listener's (l2tp.MatchSessions),
// and so is the answer to several matches: that listener sends a
// Disconnect-NAK with Error-Cause 508 (Multiple Session Selection
// Unsupported) and ends none of them. This command refuses too, and lists one
// clear by ID for each match, so the operator chooses.
func teardownMatchedSession(ctx *pluginserver.CommandContext, request sessionMatchRequest) (*plugin.Response, error) {
	service := l2tp.LookupService()
	if service == nil {
		return sessionMatchFailure(request, errSubsystemUnavailable), nil
	}
	snapshot := service.Snapshot()
	matched := l2tp.MatchSessions(&snapshot, request.match)
	if len(matched) == 0 {
		return sessionMatchNotFound(request), nil
	}
	if len(matched) > 1 {
		return sessionMatchAmbiguity(request, matched), nil
	}
	session := &matched[0]
	captured, err := l2tp.CaptureMatchedSession(service, session)
	if err != nil {
		if errors.Is(err, l2tp.ErrSessionNotFound) {
			return sessionTeardownNotFound(session.LocalSID, err), nil
		}
		return sessionTeardownFailure(session.LocalSID, err), nil
	}
	if captured == nil {
		return sessionTeardownNotFound(session.LocalSID, l2tp.ErrSessionNotFound), nil
	}
	if refusal := finishSessionTeardown(ctx, captured, session.LocalSID, request.disconnectAudit); refusal != nil {
		return refusal, nil
	}
	result := sessionTeardownResult(session.LocalSID, request.disconnectAudit)
	result[keyTunnelID] = int(session.TunnelLocalTID)
	result[request.key] = request.value
	return jsonResponse("l2tp session teardown", result)
}

func sessionMatchInputError(cause error, problem string) *opererr.Error {
	return opererr.Wrap(cause, problem,
		"The request must name one live subscriber session before it can be cleared",
		"show l2tp sessions")
}

func sessionMatchFailure(request sessionMatchRequest, cause error) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.Wrap(cause,
		"No L2TP session matching "+request.subject+" was cleared",
		"The L2TP subsystem is not running, so no session can be selected",
		"show l2tp sessions"))
}

func sessionMatchNotFound(request sessionMatchRequest) *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"No L2TP session matches "+request.subject,
		"Nothing was cleared: the value must equal the username or assigned-addr of a live session",
		"show l2tp sessions"))
}

// sessionMatchAmbiguity lists one qualified clear by ID per match. The loop
// is bounded by the matched rows of one snapshot.
func sessionMatchAmbiguity(request sessionMatchRequest, matched []l2tp.SessionSnapshot) *plugin.Response {
	candidates := make([]string, 0, len(matched))
	for index := range matched {
		candidates = append(candidates, sessionCandidateCommand("clear", &matched[index]))
	}
	return plugin.OperatorErrorResponse(opererr.New(
		textbuf.StringInt(int64(len(matched)))+" L2TP sessions match "+request.subject,
		"Nothing was cleared: this form ends one session, as a RADIUS Disconnect-Request does. Clear each one with: "+textbuf.Join(candidates, "; "),
		"show l2tp sessions"))
}

// completeSessionTeardownUser offers the user names of live sessions, then the
// audit keywords not yet typed.
func completeSessionTeardownUser(_ *pluginserver.Server, completed []string) []pluginserver.Completion {
	return completeSessionMatch(completed, "Live subscriber user name.", func(session *l2tp.SessionSnapshot) string {
		return session.Username
	})
}

// completeSessionTeardownAddress offers the assigned IPv4 addresses of live
// sessions, then the audit keywords not yet typed.
func completeSessionTeardownAddress(_ *pluginserver.Server, completed []string) []pluginserver.Completion {
	return completeSessionMatch(completed, "Live subscriber IPv4 address.", func(session *l2tp.SessionSnapshot) string {
		if !session.AssignedAddr.Is4() {
			return ""
		}
		return session.AssignedAddr.String()
	})
}

// completeSessionMatch reads the cold inventory only. value answers the
// selector a row offers, or empty for a row that offers none. Walks are
// bounded by the completed tokens and the snapshot's rows.
func completeSessionMatch(completed []string, help string, value func(*l2tp.SessionSnapshot) string) []pluginserver.Completion {
	if len(completed) == 0 {
		return completeSessionMatchValues(help, value)
	}
	if isDisconnectAuditKeyword(completed[len(completed)-1]) {
		return nil
	}
	seen := make(map[string]bool, len(completed))
	for _, token := range completed[1:] {
		seen[token] = true
	}
	var candidates []pluginserver.Completion
	for _, candidate := range []pluginserver.Completion{
		{Value: kwActor, ShortHelp: "Record the disconnect request's actor label."},
		{Value: kwReason, ShortHelp: "Record the disconnect reason."},
		{Value: kwCause, ShortHelp: "Record the RADIUS disconnect cause."},
	} {
		if seen[candidate.Value] {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func completeSessionMatchValues(help string, value func(*l2tp.SessionSnapshot) string) []pluginserver.Completion {
	service := l2tp.LookupService()
	if service == nil {
		return nil
	}
	snapshot := service.Snapshot()
	offering := l2tp.MatchSessions(&snapshot, func(session *l2tp.SessionSnapshot) bool { return value(session) != "" })
	seen := make(map[string]bool, len(offering))
	var candidates []pluginserver.Completion
	for index := range offering {
		offered := value(&offering[index])
		if seen[offered] {
			continue
		}
		seen[offered] = true
		candidates = append(candidates, pluginserver.Completion{Value: offered, ShortHelp: help})
	}
	return candidates
}
