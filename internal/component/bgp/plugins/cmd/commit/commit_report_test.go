package commit

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// These tests cover what `request commit end` and `request commit eor` ANSWER.
// The reactor decides the outcome, so each test states the outcome on the mock
// and asserts how the handler renders it: the status, the sentence, and the
// rows.

// startAndWithdraw opens a named commit and queues one withdrawal, which is the
// only way a named commit can hold work today ((*Transaction).QueueAnnounce has
// no non-test caller). Every end test starts here.
func startAndWithdraw(t *testing.T, reactor *mockReactor, name string) *pluginserver.CommandContext {
	t.Helper()
	ctx := newTestContext(reactor)
	_, err := handleCommit(ctx, []string{"start", name})
	require.NoError(t, err)
	_, err = handleCommit(ctx, []string{"withdraw", name, "route", "10.0.0.0/24"})
	require.NoError(t, err)
	return ctx
}

// twoPeerShortfall is a reactor answer where one matched peer took the queued
// withdrawal and the other was never established.
func twoPeerShortfall() *bgptypes.TransactionResult {
	return &bgptypes.TransactionResult{
		WithdrawalsQueued: 1,
		RoutesWithdrawn:   1,
		UpdatesSent:       1,
		Families:          []string{"ipv4/unicast"},
		Peers: []bgptypes.PeerCommitResult{
			{Address: "10.0.0.2", Name: "up", State: "established", RoutesWithdrawn: 1, UpdatesSent: 1},
			{Address: "10.0.0.3", Name: "down", State: "connecting",
				Reasons: []string{bgptypes.CommitReasonNotEstablished}},
		},
	}
}

// TestCommitEndAnswersErrorAndKeepsThePeerRows covers the status rule and the
// sentence a shortfall produces.
//
// VALIDATES: AC-3 -- the status is error, the sentence names each peer that took
// nothing and why, and the payload still carries every peer row.
// PREVENTS: a `done` envelope over work that reached no peer, which tells a
// script reading the exit code nothing at all.
func TestCommitEndAnswersErrorAndKeepsThePeerRows(t *testing.T) {
	ctx := startAndWithdraw(t, &mockReactor{commitResult: twoPeerShortfall()}, "short")

	resp, err := handleCommit(ctx, []string{"end", "short"})
	require.ErrorIs(t, err, errCommitNotCarriedInFull)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)

	// The sentence is what every transport carries on the error path, so it
	// names the peer and the reason rather than a count.
	assert.Contains(t, resp.Error, "10.0.0.3")
	assert.Contains(t, resp.Error, bgptypes.CommitReasonNotEstablished)
	assert.NotContains(t, resp.Error, "10.0.0.2", "a peer that took everything is not named")

	// The same sentence is the ERROR's own text. Asserting resp.Error alone
	// passes over a caller that never sees it: dispatchCommandResponse
	// (internal/component/plugin/server/dispatch.go) discards the whole response
	// when a handler returns an error beside it, so a plugin driving this
	// command receives err.Error() and nothing else.
	assert.Equal(t, resp.Error, err.Error(), "the transport that survives carries the sentence")

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "the payload survives on the response itself")
	peers, ok := data[jsonKeyPeers].(map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 2, "every matched peer keeps its row")

	down, ok := peers["10.0.0.3"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, down[jsonKeyRoutesWdr])
	assert.Equal(t, []string{bgptypes.CommitReasonNotEstablished}, down[jsonKeyReasons])
}

// TestCommitEndAnswersDoneWhenEveryPeerTookEverything is the other half of the
// status rule.
//
// VALIDATES: AC-4 -- the status is done and no peer row carries a reason.
// PREVENTS: a routine commit answering error because the status rule reads a
// zero count rather than a stated reason.
func TestCommitEndAnswersDoneWhenEveryPeerTookEverything(t *testing.T) {
	ctx := startAndWithdraw(t, &mockReactor{}, "full")

	resp, err := handleCommit(ctx, []string{"end", "full"})
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Empty(t, resp.Error)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 1, data[jsonKeyWithdrawalsQd])
	assert.Equal(t, 1, data[jsonKeyRoutesWdr])

	peers, ok := data[jsonKeyPeers].(map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 1)
	for address, row := range peers {
		fields, fieldsOK := row.(map[string]any)
		require.True(t, fieldsOK)
		_, hasReasons := fields[jsonKeyReasons]
		assert.False(t, hasReasons, "peer %s took everything, so it carries no reason", address)
	}
}

// TestCommitEORReportsTheMarkersThatLeft covers the End-of-RIB half through the
// handler. `eor-sent` used to echo the operator's request, so a marker that
// never left was reported as sent.
//
// VALIDATES: AC-6 -- each row reports the markers that left for that peer, and
// the top level states the request separately from the count.
// PREVENTS: `request commit eor` claiming an End-of-RIB reached a peer whose
// send failed.
func TestCommitEORReportsTheMarkersThatLeft(t *testing.T) {
	result := &bgptypes.TransactionResult{
		WithdrawalsQueued: 1,
		RoutesWithdrawn:   2,
		UpdatesSent:       3,
		EORSent:           1,
		EORRequested:      true,
		Families:          []string{"ipv4/unicast"},
		Peers: []bgptypes.PeerCommitResult{
			{Address: "10.0.0.2", State: "established", RoutesWithdrawn: 1, UpdatesSent: 2, EORSent: 1},
			{Address: "10.0.0.3", State: "established", RoutesWithdrawn: 1, UpdatesSent: 1, EORSent: 0,
				Reasons: []string{bgptypes.CommitReasonEORRefused}},
		},
	}
	ctx := startAndWithdraw(t, &mockReactor{commitResult: result}, "markers")

	resp, err := handleCommit(ctx, []string{"eor", "markers"})
	require.ErrorIs(t, err, errCommitNotCarriedInFull)
	assert.Equal(t, plugin.StatusError, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, actionEOR, data[jsonKeyAction])
	assert.Equal(t, true, data[jsonKeyEORRequested], "the request is stated as a request")
	assert.Equal(t, 1, data[jsonKeyEORSent], "one of the two markers left")

	peers, ok := data[jsonKeyPeers].(map[string]any)
	require.True(t, ok)
	accepted, ok := peers["10.0.0.2"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, accepted[jsonKeyEORSent])
	refused, ok := peers["10.0.0.3"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, refused[jsonKeyEORSent])
	assert.Equal(t, []string{bgptypes.CommitReasonEORRefused}, refused[jsonKeyReasons])
}

// kebabKey matches the one key spelling ai/rules/cli.md admits.
var kebabKey = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// TestCommitAnswerKeysAreKebabCase reads every key of every commit answer.
//
// VALIDATES: AC-7 -- no key is snake_case or camelCase, at the top level or
// inside a peer row.
// PREVENTS: `routes_announced`, `routes_withdrawn`, `updates_sent`, `eor_sent`
// and `routes_discarded` coming back, each of which shipped in this answer.
func TestCommitAnswerKeysAreKebabCase(t *testing.T) {
	ctx := startAndWithdraw(t, &mockReactor{commitResult: twoPeerShortfall()}, "keys")

	commands := [][]string{
		{"list"},
		{"start", "other"},
		{"show", "other"},
		{"withdraw", "other", "route", "10.9.0.0/24"},
		{"rollback", "other"},
		{"end", "keys"},
	}
	for _, args := range commands {
		resp, _ := handleCommit(ctx, args)
		require.NotNil(t, resp, "%v", args)
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			continue
		}
		for key, value := range data {
			assert.Regexp(t, kebabKey, key, "top-level key of %v", args)
			rows, isRows := value.(map[string]any)
			if !isRows {
				continue
			}
			for _, row := range rows {
				fields, isFields := row.(map[string]any)
				if !isFields {
					continue
				}
				for field := range fields {
					assert.Regexp(t, kebabKey, field, "row key of %v", args)
				}
			}
		}
	}
}

// TestCommitEndAnswerRendersAsRows drives the answer through the renderer an
// operator reaches with `| table`.
//
// VALIDATES: AC-7 -- the payload is structured data that marshals to JSON, so
// `| json` and `| yaml` render it, and `| table` prints one row per peer.
// PREVENTS: the peer outcome being written as a finished sentence, which would
// pick the reader's format for them (ai/rules/cli.md).
func TestCommitEndAnswerRendersAsRows(t *testing.T) {
	ctx := startAndWithdraw(t, &mockReactor{commitResult: twoPeerShortfall()}, "render")

	resp, _ := handleCommit(ctx, []string{"end", "render"})
	require.NotNil(t, resp)

	encoded, err := json.Marshal(resp.Data)
	require.NoError(t, err, "the payload must be renderable as JSON")

	table := command.ApplyTable(string(encoded))
	require.NotEqual(t, string(encoded), table, "the table renderer must have recognized the payload")
	for _, address := range []string{"10.0.0.2", "10.0.0.3"} {
		assert.True(t, strings.Contains(table, address), "table must carry a row for %s:\n%s", address, table)
	}
	assert.Contains(t, table, bgptypes.CommitReasonNotEstablished,
		"the reason is a field on the row, so the renderer shows it")
}
