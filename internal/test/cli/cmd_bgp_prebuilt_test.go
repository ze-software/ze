package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestEncodingSuitesRunAgainstAPrebuiltSet proves that every suite the encoding
// runner serves runs one selected test against a pre-built ze under
// LE_TEST_NO_BUILD, the chaos suites included.
//
// VALIDATES: `le test bgp chaos-web <test>` and `le test bgp chaos <test>` reach
// the test under LE_TEST_NO_BUILD, which is how `le test functional` and
// `le feature record-run` freeze a run against their isolated binary set. A
// `.ci` `le` head runs the runner's own executable, so a chaos suite needs no
// binary beyond the set's ze and le.
// PREVENTS: the chaos suites asking the runner to build a second le that no
// step ever executes, which Runner.Build refuses under LE_TEST_NO_BUILD, so no
// chaos-web test could be run, or recorded, against a frozen set.
//
// Method: a throwaway tree holds one passing test per suite whose only step runs
// a pre-built `ze` script; the run goes through zeTestRunEncodingOrAPI, the
// function `le test bgp <suite>` calls.
func TestEncodingSuitesRunAgainstAPrebuiltSet(t *testing.T) {
	for _, command := range []string{cmdEncode, cmdPlugin, cmdReload, cmdChaosWeb, cmdChaosIntg} {
		t.Run(command, func(t *testing.T) {
			base := t.TempDir()
			ze := filepath.Join(base, "bin", "ze")
			require.NoError(t, os.MkdirAll(filepath.Dir(ze), 0o755))
			require.NoError(t, os.WriteFile(ze, []byte("#!/bin/sh\nexit 0\n"), 0o755)) //nolint:gosec // a test fixture that must be executable
			// The tree carries the repository's gate manifest, so a run that
			// reads it reads the real one.
			gates, err := os.ReadFile(filepath.Join("..", "..", "..", "feature-gates.txt"))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(base, "feature-gates.txt"), gates, 0o600))
			setBuildEnv(t, "ZE_BIN", ze)
			setBuildEnv(t, "LE_TEST_NO_BUILD", "1")

			suite := map[string]string{cmdChaosWeb: "chaos-web", cmdChaosIntg: "chaos"}[command]
			if suite == "" {
				suite = command
			}
			dir := filepath.Join(base, "test", suite)
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "prebuilt.ci"),
				[]byte("cmd=foreground:seq=1:exec=ze\nexpect=exit:code=0\n"), 0o600))

			cli := &zeTestRunCLIFlags{command: command, testArgs: []string{"prebuilt"},
				timeout: time.Minute, parallel: 1, quiet: true, port: 1790, count: 1}
			require.NoError(t, zeTestRunEncodingOrAPI(context.Background(), cli, base),
				"le test bgp %s must run a selected test against the pre-built ze", command)
		})
	}
}
