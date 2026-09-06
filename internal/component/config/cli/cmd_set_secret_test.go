// Design: docs/architecture/config/syntax.md — `ze config set` never echoes a secret

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// The values these cases type at the prompt. Each carries a distinctive TAIL,
// asserted separately from the whole, because a message can escape the value
// and then contain no copy of it while still publishing every character. That
// is how the 2026-08-15 leak in maskSecretInMessage passed a whole-value
// assertion.
const (
	typedSecret     = "hunter2-a4471bc-tail"
	typedSecretTail = "a4471bc-tail"
	typedNonSecret  = "10.9.8.7"
)

// setSecretConfig is the smallest config `ze config set` opens for these cases.
const setSecretConfig = "bgp {\n    router-id 1.2.3.4;\n}\n"

// TestConfigSetNeverEchoesASecret drives the operator's own command and reads
// the stderr they read.
//
// VALIDATES: the acknowledgement `ze config set` writes for a ze:sensitive leaf
// carries config.SecretDataPlaceholder and no part of the value.
// PREVENTS: a credential typed at the prompt landing in scrollback, in a
// terminal recording, and in anything capturing the session. cmdSetImpl wrote
// `set %s %s` and `dry-run: would set %s %s` with the operator's RAW value, and
// the refusal from Completer.ValidateValueAtPath quoted it verbatim. Every mask
// this defect class had built cleans a TREE, a DIFF or an ERROR MESSAGE about a
// leaf node, and an acknowledgement is written from the operator's own tokens
// before any tree is read.
func TestConfigSetNeverEchoesASecret(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "the success line"
		if dryRun {
			name = "the dry-run line"
		}

		t.Run(name, func(t *testing.T) {
			configPath := writeTestConfig(t, setSecretConfig)

			args := []string{}
			if dryRun {
				args = append(args, "--dry-run")
			}
			args = append(args, configPath, "environment", "api-server", "token", typedSecret)

			code, stderr := captureStderr(t, func() int { return cmdSet(args) })
			require.Equal(t, exitOK, code, "the command must succeed, or this case proves nothing; stderr: %s", stderr)

			require.Contains(t, stderr, "environment api-server token",
				"the acknowledgement must name the leaf, or this case proves nothing")
			assert.NotContains(t, stderr, typedSecret, "the acknowledgement published the value")
			assert.NotContains(t, stderr, typedSecretTail, "the acknowledgement published the tail of the value")
			assert.Contains(t, stderr, config.SecretDataPlaceholder, "the acknowledgement wrote no placeholder")
		})
	}
}

// TestConfigSetStillEchoesAValueTheSchemaDoesNotMark is the other polarity.
//
// VALIDATES: an unmarked leaf still echoes what the operator typed.
// PREVENTS: a vacuous pass above, and the cost of a mask that fails closed on
// every path it cannot resolve. secretAtPath answers "secret" for a path that
// resolves to nothing, so a resolver that cannot walk an ordinary token path
// would mask every acknowledgement the command writes and the test above would
// still pass.
func TestConfigSetStillEchoesAValueTheSchemaDoesNotMark(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "set"
		args := []string{}
		if dryRun {
			name = "dry-run"
			args = append(args, "--dry-run")
		}

		t.Run(name, func(t *testing.T) {
			configPath := writeTestConfig(t, setSecretConfig)
			args = append(args, configPath, "bgp", "router-id", typedNonSecret)

			code, stderr := captureStderr(t, func() int { return cmdSet(args) })
			require.Equal(t, exitOK, code, "stderr: %s", stderr)

			assert.Contains(t, stderr, typedNonSecret,
				"the command masked a leaf the schema does not mark, so its mask reads something other than the schema")
			assert.NotContains(t, stderr, config.SecretDataPlaceholder)
		})
	}
}

// TestConfigSetRefusalNeverEchoesASecret covers the second shape a value
// reaches the terminal in: inside the sentence that refuses it.
//
// VALIDATES: the refusal for a ze:sensitive leaf names the rule and not the
// value.
// PREVENTS: `invalid value %q for %s` publishing the credential the operator
// just typed. The leaf here carries a pattern, so the refusal is reachable: an
// OSPFv3 IPsec integrity key is written in hex characters, and the value typed
// here is not.
func TestConfigSetRefusalNeverEchoesASecret(t *testing.T) {
	configPath := writeTestConfig(t, setSecretConfig)

	code, stderr := captureStderr(t, func() int {
		return cmdSet([]string{
			configPath, "ospf", "address-family", "ipv6", "interfaces",
			"interface", "eth0", "ipsec", "key", typedSecret,
		})
	})

	require.Equal(t, exitError, code, "the value must be refused, or this case proves nothing; stderr: %s", stderr)
	require.Contains(t, stderr, "invalid value",
		"the refusal must be the value validator's, or this case proves nothing")
	assert.NotContains(t, stderr, typedSecretTail, "the refusal published the tail of the value")
	assert.Contains(t, stderr, config.SecretDataPlaceholder, "the refusal wrote no placeholder")
}
