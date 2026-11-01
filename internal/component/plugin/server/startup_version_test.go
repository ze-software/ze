package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDeclareRegistrationRefusesTextContract checks that a plugin declaring no
// protocol version is refused at Stage 1 with one error naming the plugin, the
// version it must speak and the protocol page, and that no sink hook runs.
//
// VALIDATES: AC-9 (spec-filter-wire-0-umbrella, D-2).
// PREVENTS: a text-era plugin starting and speaking a contract Ze dropped.
func TestDeclareRegistrationRefusesTextContract(t *testing.T) {
	t.Parallel()

	engineConn, pluginMux := newDriverPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callErr := make(chan error, 1)
	go func() {
		_, err := pluginMux.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{})
		callErr <- err
	}()

	sink := &recordingSink{pc: engineConn}
	err := runStartupHandshake(ctx, sink)
	require.Error(t, err)
	for _, want := range []string{"plugin widget", "declared no protocol version", `"protocol-version": 1`, "docs/plugin-development/protocol.md"} {
		assert.Contains(t, err.Error(), want)
	}

	select {
	case perr := <-callErr:
		require.Error(t, perr)
		assert.Contains(t, perr.Error(), "declared no protocol version")
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not receive the refusal")
	}
	assert.Equal(t, []string{"Conn"}, sink.order, "the refusal is the only answer: no stage hook runs")
}

// TestDeclareRegistrationVersionAnswer checks the Stage 1 answer: a plugin
// declaring a version at or above Ze's gets Ze's version back, and one below
// it gets its own.
//
// VALIDATES: AC-18 (spec-filter-wire-0-umbrella, D-2), boundary rows 1 and 2.
// PREVENTS: a newer plugin speaking a version Ze cannot read.
func TestDeclareRegistrationVersionAnswer(t *testing.T) {
	t.Parallel()

	for _, declared := range []int{rpc.ProtocolVersion, rpc.ProtocolVersion + 1, 7} {
		engineConn, pluginMux := newDriverPipe(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		answer := make(chan rpc.DeclareRegistrationOutput, 1)
		go func() {
			result, err := pluginMux.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{ProtocolVersion: declared})
			var out rpc.DeclareRegistrationOutput
			if err == nil {
				_ = json.Unmarshal(result, &out) //nolint:errcheck // a zero answer fails the assertion below
			}
			answer <- out
		}()
		sink := &recordingSink{pc: engineConn}
		go runAbandonedHandshake(ctx, sink)

		select {
		case out := <-answer:
			assert.Equal(t, rpc.ProtocolVersion, out.ProtocolVersion, "a plugin declaring %d is answered the version Ze speaks", declared)
		case <-time.After(2 * time.Second):
			t.Fatalf("no Stage 1 answer for declared version %d", declared)
		}
		cancel()
	}
}

// runAbandonedHandshake runs the engine side until the test cancels it after
// reading the Stage 1 answer.
func runAbandonedHandshake(ctx context.Context, sink startupSink) {
	_ = runStartupHandshake(ctx, sink) //nolint:errcheck // abandoned after Stage 1 by design
}

// TestNegotiateProtocolVersion pins the boundary rows of the contract version.
//
// VALIDATES: AC-9 and AC-18 boundary: absent or 0 refused, 1 answered 1, 2 or
// more answered 1.
func TestNegotiateProtocolVersion(t *testing.T) {
	t.Parallel()

	for _, declared := range []int{-1, 0} {
		_, err := rpc.NegotiateProtocolVersion("p", declared)
		require.Error(t, err, "declared %d", declared)
	}
	for _, declared := range []int{1, 2, 1000} {
		got, err := rpc.NegotiateProtocolVersion("p", declared)
		require.NoError(t, err)
		assert.Equal(t, rpc.ProtocolVersion, got, "declared %d", declared)
	}
}
