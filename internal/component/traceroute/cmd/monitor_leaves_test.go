package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/probe"
)

// TestMonitorTracerouteLeavesKeepTheLocalGrammar proves the leaves
// ze-traceroute-cmd.yang declares on `monitor traceroute` accept every
// spelling the command took when its handler read the tokens itself, that
// each value reaches the handler, and that the model refuses a value outside
// a declared range before the handler runs.
//
// Method: each spelling goes through command.ValidateModelArgs and
// monitorTracerouteArgsOf, as on every route, and the parsed request is
// compared with the spelling.
func TestMonitorTracerouteLeavesKeepTheLocalGrammar(t *testing.T) {
	base := tracerouteArgs{maxHops: 30, timeout: 3 * time.Second, probes: 3, opts: tracerouteOpts{df: probe.DFOff}}
	with := func(edit func(*tracerouteArgs)) tracerouteArgs {
		req := base
		edit(&req)
		return req
	}
	cases := []struct {
		words []string
		want  tracerouteArgs
	}{
		{[]string{"127.0.0.1"}, base},
		{[]string{"127.0.0.1", "max-hops", "10"}, with(func(r *tracerouteArgs) { r.maxHops = 10 })},
		{[]string{"127.0.0.1", "timeout", "2s", "probes", "5"}, with(func(r *tracerouteArgs) { r.timeout = 2 * time.Second; r.probes = 5 })},
		{[]string{"127.0.0.1", "do-not-fragment", "honor-cache"}, with(func(r *tracerouteArgs) { r.opts.df = probe.DFHonorCache })},
		{[]string{"max-hops", "2", "127.0.0.1"}, with(func(r *tracerouteArgs) { r.maxHops = 2 })},
		{[]string{"destination", "::1", "max-hops", "64"}, with(func(r *tracerouteArgs) { r.maxHops = 64 })},
	}
	for _, tc := range cases {
		validated, err := command.ValidateModelArgs("monitor traceroute", tc.words, nil)
		require.NoError(t, err, "monitor traceroute %q", tc.words)
		got, err := monitorTracerouteArgsOf(validated)
		require.NoError(t, err, "monitor traceroute %q", tc.words)
		require.True(t, got.target.IsLoopback(), "monitor traceroute %q: target %v", tc.words, got.target)
		tc.want.target = got.target
		require.Equal(t, tc.want, got, "monitor traceroute %q", tc.words)
	}

	for _, words := range [][]string{
		{},
		{"127.0.0.1", "max-hops", "65"},
		{"127.0.0.1", "probes", "0"},
		{"127.0.0.1", "do-not-fragment", "always"},
		{"127.0.0.1", "timeout", "soon"},
	} {
		_, err := command.ValidateModelArgs("monitor traceroute", words, nil)
		require.Error(t, err, "monitor traceroute %q passed the model", words)
	}
}
