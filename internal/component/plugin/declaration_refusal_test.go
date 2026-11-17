package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDeclarationFromStdoutKeepsTheDecoderRefusal proves the reader of a
// queried plugin's output reads a declaration as the daemon does. A line that
// carries the retired "schema" key, or another key ze does not know, is a
// declaration read and refused: declared, with the decoder's operator error as
// the refusal and no declaration, so the plugin is never listed as declaring a
// module the daemon would not start. The same module under "schemas" is read
// whole. A line that is no JSON is skipped for the next one, and a declaration
// with no protocol version is read and left to the version check, which names
// its real problem.
//
// Method: the output bytes a child would write, handed to the reader.
func TestDeclarationFromStdoutKeepsTheDecoderRefusal(t *testing.T) {
	const module = `{"module":"acme-conf","namespace":"urn:acme","yang-text":"module acme-conf {}"}`
	line := func(params string) string {
		return "#1 " + rpc.MethodDeclareRegistration + " " + params + "\n"
	}

	refused := []struct {
		name   string
		stdout string
		want   string
	}{
		{"the retired schema key", line(`{"protocol-version":1,"schema":` + module + `}`),
			`plugin "queried" sent its YANG module under "schema", a key ze no longer reads`},
		{"an unknown key in a module entry", line(`{"protocol-version":1,"schemas":[{"module":"acme-conf","revision":"x"}]}`),
			`plugin "queried" sent a Stage 1 declaration that carries a key ze does not know`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			declaration, declared, refusal := declarationFromStdout("queried", []byte(tc.stdout))

			assert.True(t, declared, "a refused line is a declaration read")
			require.Error(t, refusal)
			_, operator := errors.AsType[*opererr.Error](refusal)
			assert.True(t, operator, "the refusal is the decoder's operator error, got %T", refusal)
			assert.Contains(t, refusal.Error(), tc.want)
			assert.Contains(t, refusal.Error(), rpc.DeclarationSection)
			assert.Empty(t, declaration.Schemas, "a refused declaration declares no module")
			assert.Zero(t, declaration.ProtocolVersion, "a refused declaration carries nothing")
		})
	}

	t.Run("schemas is read whole", func(t *testing.T) {
		declaration, declared, refusal := declarationFromStdout("queried", []byte(line(`{"protocol-version":1,"schemas":[`+module+`]}`)))

		assert.True(t, declared)
		require.NoError(t, refusal)
		require.Len(t, declaration.Schemas, 1)
		assert.Equal(t, "acme-conf", declaration.Schemas[0].Module)
		assert.Equal(t, "urn:acme", declaration.Schemas[0].Namespace)
	})

	t.Run("a line that is no JSON is skipped", func(t *testing.T) {
		stdout := line(`{"protocol-version":`) + line(`{"protocol-version":1,"schemas":[`+module+`]}`)
		declaration, declared, refusal := declarationFromStdout("queried", []byte(stdout))

		assert.True(t, declared)
		require.NoError(t, refusal)
		require.Len(t, declaration.Schemas, 1)
	})

	t.Run("no protocol version is left to the version check", func(t *testing.T) {
		stdout := line(`{"filters":[{"name":"legacy","direction":"import","attributes":["as-path"]}]}`)
		declaration, declared, refusal := declarationFromStdout("queried", []byte(stdout))

		assert.True(t, declared)
		require.NoError(t, refusal, "the version check refuses it, with the reason that names its problem")
		assert.Zero(t, declaration.ProtocolVersion)
		require.Len(t, declaration.Filters, 1)
		assert.Equal(t, "legacy", declaration.Filters[0].Name)
	})
}
