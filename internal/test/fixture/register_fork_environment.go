// Design: docs/architecture/config/environment.md -- the shell spelling a forked plugin reads.
// Related: test/plugin/forked-plugin-reads-dotted-env-key.ci -- the daemon environment this reads.
package fixture

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The two keys the .ci sets in the daemon's environment, in their dotted
// spelling. One is dotted only, the other also holds a hyphen, the two shapes
// dash dropped before the daemon renamed ze keys for its fork (env.ShellSafe).
const (
	forkEnvironmentDottedKey = "ze.test.fork.environment.dotted"
	forkEnvironmentHyphenKey = "ze.test.fork-environment.hyphen-key"
)

var (
	_ = env.MustRegister(env.EnvEntry{Key: forkEnvironmentDottedKey, Type: "string", Description: "Value a forked fixture reads back to prove a dotted ze key reached it (tests)"})
	_ = env.MustRegister(env.EnvEntry{Key: forkEnvironmentHyphenKey, Type: "string", Description: "Value a forked fixture reads back to prove a hyphenated ze key reached it (tests)"})
)

func init() {
	Register("plugin/fork-environment", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "plugin/fork-environment", &sdk.Registration{}, forkEnvironment)
	}, StopsDaemon)
}

// forkEnvironment reads both keys from this process's own environment, the one
// the daemon handed it through /bin/sh -c. The daemon is the only process that
// was given them, so a value here crossed the fork and nothing else.
func forkEnvironment(_ context.Context, _ *sdk.Plugin) error {
	var ok textbuf.Buffer
	ok.Str("OK: the forked plugin read")
	for _, want := range [][2]string{
		{forkEnvironmentDottedKey, "dotted"},
		{forkEnvironmentHyphenKey, "hyphen"},
	} {
		got := env.Get(want[0])
		if got != want[1] {
			var tb textbuf.Buffer
			tb.Str("the forked plugin read ").Str(want[0]).Str(" as \"").Str(got).
				Str("\", want \"").Str(want[1]).Str("\": the daemon's key did not reach it through the shell")
			return errors.New(tb.String())
		}
		ok.Byte(' ').Str(want[0]).Byte('=').Str(got)
	}
	ok.Byte('\n')
	return ok.StdErr()
}
