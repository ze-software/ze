package callsink_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

type fakeSink struct {
	last     callsink.Request
	accepted bool
	err      error
	calls    int
}

func (f *fakeSink) Relay(req callsink.Request) (bool, error) {
	f.calls++
	f.last = req
	return f.accepted, f.err
}

// VALIDATES: R-1 -- Register/Lookup/Unregister wire a Sink without pppoe
// importing l2tp; Lookup is nil until a Sink is registered and after it is
// cleared.
func TestCallSink_RegisterLookupUnregister(t *testing.T) {
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)

	require.Nil(t, callsink.Lookup(), "no sink registered yet")

	s := &fakeSink{accepted: true}
	callsink.Register(s)
	got := callsink.Lookup()
	require.NotNil(t, got)

	accepted, err := got.Relay(callsink.Request{Service: "wholesale", SessionID: 7})
	require.NoError(t, err)
	require.True(t, accepted)
	require.Equal(t, 1, s.calls)
	require.Equal(t, "wholesale", s.last.Service)
	require.EqualValues(t, 7, s.last.SessionID)

	callsink.Unregister()
	require.Nil(t, callsink.Lookup(), "cleared after Unregister")
}

// VALIDATES: Register(nil) is equivalent to Unregister.
func TestCallSink_RegisterNilClears(t *testing.T) {
	t.Cleanup(callsink.Unregister)
	callsink.Register(&fakeSink{})
	require.NotNil(t, callsink.Lookup())
	callsink.Register(nil)
	require.Nil(t, callsink.Lookup())
}

// VALIDATES: a matched-but-failed relay surfaces its error to the caller.
func TestCallSink_RelayError(t *testing.T) {
	t.Cleanup(callsink.Unregister)
	boom := errors.New("dial failed")
	callsink.Register(&fakeSink{accepted: false, err: boom})
	accepted, err := callsink.Lookup().Relay(callsink.Request{Service: "x"})
	require.ErrorIs(t, err, boom)
	require.False(t, accepted)
}
