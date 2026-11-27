// Design: docs/architecture/config/syntax.md -- fail-closed secret masking in config dump.
// Related: cmd_dump.go -- maskValue, maskMapValues, resolveDumpBytes, printConfig.

package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const dumpMaskSecret = "dump-mask-failure-secret-xyz987" //nolint:gosec // fixture value, not a credential

const dumpSecretConfig = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
            md5 {
                password ` + dumpMaskSecret + `;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
}
`

func captureStdoutForDump(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	runErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = origStdout
	outBytes, readErr := io.ReadAll(r)
	require.NoError(t, r.Close())
	require.NoError(t, readErr)
	return string(outBytes), runErr
}

// TestDumpMaskFailureNeverPrintsTheSecret verifies that a $9$ secret encoding
// failure in config dump refuses with an *opererr.Error naming
// "ze config dump --strip-private" across maskValue, resolveDumpBytes, and
// printConfig while never leaking the plaintext secret, and that normal
// DisplayEncode and DisplayStrip modes remain intact.
//
// VALIDATES: AC-3 -- config dump fails closed on secret encoding failure without leaking plaintext.
// PREVENTS: silently substituting /* SECRET-DATA */ behind slog.Debug or printing cleartext secrets.
func TestDumpMaskFailureNeverPrintsTheSecret(t *testing.T) {
	encodeFailure := errors.New("entropy source unavailable")

	t.Run("maskValue returns opererr and placeholder without leaking secret", func(t *testing.T) {
		origEncode := dumpEncodeSecret
		dumpEncodeSecret = func(string) (string, error) { return "", encodeFailure }
		t.Cleanup(func() { dumpEncodeSecret = origEncode })

		got, err := maskValue(dumpMaskSecret, config.DisplayEncode)
		assert.Equal(t, config.SecretDataPlaceholder, got)
		assert.NotContains(t, got, dumpMaskSecret)
		require.Error(t, err)

		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "expected *opererr.Error, got: "+err.Error())
		assert.Equal(t, "ze config dump --strip-private", opErr.Fix())
		assert.NotContains(t, err.Error(), dumpMaskSecret)
	})

	t.Run("resolveDumpBytes returns opererr and never leaks secret", func(t *testing.T) {
		origEncode := dumpEncodeSecret
		dumpEncodeSecret = func(string) (string, error) { return "", encodeFailure }
		t.Cleanup(func() { dumpEncodeSecret = origEncode })

		var diag textbuf.Buffer
		res, err := resolveDumpBytes([]byte(dumpSecretConfig), false, &diag)
		require.Error(t, err)
		assert.Nil(t, res.dumpMap)

		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "expected *opererr.Error, got: "+err.Error())
		assert.Equal(t, "ze config dump --strip-private", opErr.Fix())
		assert.NotContains(t, err.Error(), dumpMaskSecret)
		assert.NotContains(t, diag.String(), dumpMaskSecret)
	})

	t.Run("printConfig returns opererr and never prints secret", func(t *testing.T) {
		origEncode := dumpEncodeSecret
		dumpEncodeSecret = func(string) (string, error) { return "", encodeFailure }
		t.Cleanup(func() { dumpEncodeSecret = origEncode })

		bgpTree := map[string]any{
			"router-id": "1.1.1.1",
			"peer": map[string]any{
				"peer1": map[string]any{
					"connection": map[string]any{
						"md5": map[string]any{
							"password": dumpMaskSecret,
						},
					},
				},
			},
		}
		sensitiveKeys := map[string]bool{"password": true}

		out, err := captureStdoutForDump(t, func() error {
			return printConfig(bgpTree, nil, sensitiveKeys, config.DisplayEncode)
		})
		assert.NotContains(t, out, dumpMaskSecret)
		require.Error(t, err)

		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "expected *opererr.Error, got: "+err.Error())
		assert.Equal(t, "ze config dump --strip-private", opErr.Fix())
		assert.NotContains(t, err.Error(), dumpMaskSecret)
	})

	t.Run("normal DisplayEncode and DisplayStrip continue working", func(t *testing.T) {
		gotEncoded, err := maskValue(dumpMaskSecret, config.DisplayEncode)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(gotEncoded, "$9$"))
		assert.NotContains(t, gotEncoded, dumpMaskSecret)

		gotStripped, err := maskValue(dumpMaskSecret, config.DisplayStrip)
		require.NoError(t, err)
		assert.Equal(t, config.SecretDataPlaceholder, gotStripped)

		var diagEncode textbuf.Buffer
		resEncode, err := resolveDumpBytes([]byte(dumpSecretConfig), false, &diagEncode)
		require.NoError(t, err)
		encodedJSON, err := json.Marshal(resEncode.dumpMap)
		require.NoError(t, err)
		assert.Contains(t, string(encodedJSON), "$9$")
		assert.NotContains(t, string(encodedJSON), dumpMaskSecret)

		var diagStrip textbuf.Buffer
		resStrip, err := resolveDumpBytes([]byte(dumpSecretConfig), true, &diagStrip)
		require.NoError(t, err)
		strippedJSON, err := json.Marshal(resStrip.dumpMap)
		require.NoError(t, err)
		assert.Contains(t, string(strippedJSON), config.SecretDataPlaceholder)
		assert.NotContains(t, string(strippedJSON), dumpMaskSecret)
	})
}
