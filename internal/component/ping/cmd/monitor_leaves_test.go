package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// monitorPingOf validates words as the arguments of `monitor ping` through
// the model, the validator every route calls before the handler, and reads
// them back the way the handlers do.
func monitorPingOf(t *testing.T, words ...string) (monitorPingArgs, error) {
	t.Helper()
	validated, err := command.ValidateModelArgs("monitor ping", words, nil)
	if err != nil {
		return monitorPingArgs{}, err
	}
	return monitorPingArgsOf(validated)
}

// TestMonitorPingLeavesKeepTheLocalGrammar proves the leaves ze-ping-cmd.yang
// declares on `monitor ping` accept every spelling the command took when its
// handler read the tokens itself, and that each value reaches the handler.
//
// Method: each spelling goes through command.ValidateModelArgs and
// monitorPingArgsOf, as on every route; the parsed values are compared with
// the spelling. The keyword form of the destination is new and is checked too.
func TestMonitorPingLeavesKeepTheLocalGrammar(t *testing.T) {
	cases := []struct {
		words []string
		want  monitorPingArgs
	}{
		{[]string{"127.0.0.1"}, monitorPingArgs{Interval: time.Second, Timeout: 5 * time.Second}},
		{[]string{"127.0.0.1", "interval", "500ms"}, monitorPingArgs{Interval: 500 * time.Millisecond, Timeout: 5 * time.Second}},
		{[]string{"127.0.0.1", "timeout", "2s", "count", "3", "size", "1400"}, monitorPingArgs{Interval: time.Second, Timeout: 2 * time.Second, Count: 3, Size: 1400}},
		{[]string{"count", "3", "127.0.0.1"}, monitorPingArgs{Interval: time.Second, Timeout: 5 * time.Second, Count: 3}},
		{[]string{"destination", "127.0.0.1", "interval", "1.5s"}, monitorPingArgs{Interval: 1500 * time.Millisecond, Timeout: 5 * time.Second}},
		{[]string{"::1", "interval", "30s", "timeout", "30s"}, monitorPingArgs{Interval: 30 * time.Second, Timeout: 30 * time.Second}},
	}
	for _, tc := range cases {
		got, err := monitorPingOf(t, tc.words...)
		require.NoError(t, err, "monitor ping %q", tc.words)
		tc.want.Dest = got.Dest
		require.Equal(t, tc.want, got, "monitor ping %q", tc.words)
		require.True(t, got.Dest.IsLoopback(), "monitor ping %q: destination %v", tc.words, got.Dest)
	}
}
