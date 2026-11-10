package command

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// routeFaultsPath is the command the model routes are judged for in these
// tests: an optional selector, two mandatory leaves, an optional one, and a
// rest-of-line text.
const routeFaultsPath = "show test route faults"

// routeFaultsDefs answers the definitions withRouteFaultsSource installs.
func routeFaultsDefs() []ArgDef {
	return []ArgDef{
		mustArgDef(NewStringArg("selector", nil, nil, ArgOptions{})),
		mustArgDef(NewUintArg("asn", 32, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewUintArg("hold-time", 16, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("router-id", nil, nil, ArgOptions{})),
		mustArgDef(NewStringArg("text", nil, nil, ArgOptions{Rest: true})),
	}
}

// withRouteFaultsSource installs a model that declares routeFaultsDefs for
// routeFaultsPath and nothing else. Not safe for parallel tests.
func withRouteFaultsSource(t *testing.T) {
	t.Helper()
	withArgDefSource(t, func(path string) ([]ArgDef, error) {
		if path != routeFaultsPath {
			return nil, nil
		}
		return routeFaultsDefs(), nil
	})
}

// captureStderr answers what fn wrote to os.Stderr. Not safe for parallel
// tests: os.Stderr is process-wide.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = saved }()

	fn()

	require.NoError(t, writer.Close())
	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(out)
}

// TestModelRoutesGiveTheWholeCommandFix proves every route that judges through
// the model restates a list of faults as the daemon dispatcher does.
//
// GOAL: a line refused only for missing arguments is given the whole command
// to type as its fix on every route, and a line with a fault in a typed word
// keeps each fault's line and the general correction, so one refusal reads the
// same whichever route the operator reached.
// METHOD: judge the same lines through ValidateModelArgs (the route of the
// plugin process, the subsystem, the RPC wrapper and SSH streaming), through
// ServeLocal (the in-process local-data route) and through InvokeLocal (the
// `ze <verb>` route), and require the whole refusal or its fix line.
//
// VALIDATES: AC-13 of spec-operator-errors-name-the-fix, through
// ArgumentFaults.RestatedForLine on the model routes.
// PREVENTS: the model routes keeping each leaf's usage line with the general
// fix while the dispatcher gave the whole command, for the same refusal.
func TestModelRoutesGiveTheWholeCommandFix(t *testing.T) {
	withRouteFaultsSource(t)

	t.Run("ValidateModelArgs, only missing arguments", func(t *testing.T) {
		_, err := ValidateModelArgs(routeFaultsPath, []string{"router-id", "192.0.2.1"}, nil)
		require.Error(t, err)
		assert.Equal(t, "this command has 2 faults, so Ze did not run it\n"+
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
			"- required argument missing: asn, as in 'asn <asn>'\n"+
			"- required argument missing: hold-time, as in 'hold-time <hold-time>'\n"+
			"fix: show test route faults router-id 192.0.2.1 asn <asn> hold-time <hold-time>", err.Error())
	})

	t.Run("ValidateModelArgs, a mixed list keeps its lines", func(t *testing.T) {
		_, err := ValidateModelArgs(routeFaultsPath, []string{"asn", "x"}, nil)
		require.Error(t, err)
		defs := routeFaultsDefs()
		assert.Equal(t, "this command has 2 faults, so Ze did not run it\n"+
			"Ze runs a command only when every word of it is valid, and each line below is one fault:\n"+
			"- "+ValidateArgString("x", &defs[1]).Error()+"\n"+
			"- required argument missing: hold-time, as in 'hold-time <hold-time>'\n"+
			"fix: correct each fault listed above, then type the command again", err.Error())
	})

	t.Run("ValidateModelArgs, a bound value is written by its keyword", func(t *testing.T) {
		_, err := ValidateModelArgs(routeFaultsPath, nil, map[string]string{"selector": "192.0.2.7"})
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			"\nfix: show test route faults selector 192.0.2.7 asn <asn> hold-time <hold-time>")
	})

	t.Run("ValidateModelArgs, missing arguments go before a typed text", func(t *testing.T) {
		_, err := ValidateModelArgs(routeFaultsPath, []string{"text", "hello", "world"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			"\nfix: show test route faults asn <asn> hold-time <hold-time> text hello world")
	})

	t.Run("ServeLocal, the local-data route", func(t *testing.T) {
		require.NoError(t, RegisterLocalData(routeFaultsPath, func(ValidatedArgs) LocalAnswer {
			t.Error("handler must not run on a refused line")
			return LocalAnswer{}
		}, registry.Meta{}, func(string, any) int { return 0 }))

		var code int
		stderr := captureStderr(t, func() {
			_, code, _ = ServeLocal(routeFaultsPath+" router-id 192.0.2.1", "")
		})
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr,
			"\nfix: show test route faults router-id 192.0.2.1 asn <asn> hold-time <hold-time>\n")

		stderr = captureStderr(t, func() {
			_, code, _ = ServeLocal(routeFaultsPath+" asn x", "")
		})
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "\n- required argument missing: hold-time, as in 'hold-time <hold-time>'\n")
		assert.Contains(t, stderr, "\nfix: correct each fault listed above, then type the command again\n")
	})

	t.Run("InvokeLocal, the ze verb route", func(t *testing.T) {
		handler := func(ValidatedArgs) int {
			t.Error("handler must not run on a refused line")
			return 0
		}
		var code int
		stderr := captureStderr(t, func() {
			code = InvokeLocal(routeFaultsPath, handler, []string{"router-id", "192.0.2.1"})
		})
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr,
			"\nfix: ze show test route faults router-id 192.0.2.1 asn <asn> hold-time <hold-time>\n")

		stderr = captureStderr(t, func() {
			code = InvokeLocal(routeFaultsPath, handler, []string{"asn", "x"})
		})
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "\n- required argument missing: hold-time, as in 'hold-time <hold-time>'\n")
		assert.Contains(t, stderr, "\nfix: correct each fault listed above, then type the command again\n")
	})
}
