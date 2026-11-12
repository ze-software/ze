package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	plugin "github.com/ze-software/ze/internal/component/plugin"
)

// routeFaultsPath is the command the model routes are judged for in these
// tests: two mandatory leaves and an optional one.
const routeFaultsPath = "show test route faults"

// Fix lines of the two kinds of list these tests refuse.
const (
	routeFaultsWholeFix   = "\nfix: show test route faults router-id 192.0.2.1 asn <asn> hold-time <hold-time>"
	routeFaultsGeneralFix = "\nfix: correct each fault listed above, then type the command again"
	routeFaultsHoldLine   = "\n- required argument missing: hold-time, as in 'hold-time <hold-time>'\n"
)

// Lines of the two kinds of list these tests refuse: only missing arguments,
// and a value its leaf refuses beside a missing argument.
var (
	routeFaultsMissingOnly = []string{"router-id", "192.0.2.1"}
	routeFaultsMixed       = []string{"asn", "x"}
)

// withRouteFaultsModel makes the model declare the definitions of
// routeFaultsPath, and answer every other path as before. Not safe for
// parallel tests: the model source is process-wide.
func withRouteFaultsModel(t *testing.T) {
	t.Helper()
	defs := []command.ArgDef{
		commandtest.Must(command.NewUintArg("asn", 32, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewUintArg("hold-time", 16, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("router-id", nil, nil, command.ArgOptions{})),
	}
	var replaced command.ArgDefSource
	replaced = command.ReplaceArgDefSourceForTest(func(path string) ([]command.ArgDef, error) {
		if path == routeFaultsPath {
			return defs, nil
		}
		return replaced(path)
	})
	t.Cleanup(func() { command.ReplaceArgDefSourceForTest(replaced) })
}

// requireRouteRefusals runs refuse on a line missing only arguments and on a
// mixed line, and requires the whole-command fix for the first and each fault's
// line with the general fix for the second.
func requireRouteRefusals(t *testing.T, refuse func(args []string) error) {
	t.Helper()
	err := refuse(routeFaultsMissingOnly)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "this command has 2 faults, so Ze did not run it")
	assert.Contains(t, err.Error(), routeFaultsWholeFix)

	err = refuse(routeFaultsMixed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), routeFaultsHoldLine)
	assert.Contains(t, err.Error(), routeFaultsGeneralFix)
}

// TestRoutesRestateArgumentFaultsAlike proves the routes that hold no *Command
// restate a list of faults as Dispatch does.
//
// GOAL: the same refused line reads the same on every route: the whole command
// as the fix when every fault is a missing argument, each fault's line and the
// general correction otherwise.
// METHOD: make the model declare two mandatory leaves for one path, then refuse
// a line missing both and a line with a bad value and a missing leaf through
// the plugin-process route (R9), the subsystem route, the RPC wrapper (R4),
// SSH streaming (R8) and Dispatch, which is the reference.
//
// VALIDATES: AC-13 of spec-operator-errors-name-the-fix, through
// command.ArgumentFaults.RestatedForLine on every route.
// PREVENTS: the plugin-process, subsystem and local routes keeping each leaf's
// usage line with the general fix while Dispatch gave the whole command.
func TestRoutesRestateArgumentFaultsAlike(t *testing.T) {
	withRouteFaultsModel(t)

	t.Run("plugin process (R9)", func(t *testing.T) {
		d := NewDispatcher()
		cmd := &RegisteredCommand{Name: routeFaultsPath, LowerName: routeFaultsPath}
		requireRouteRefusals(t, func(args []string) error {
			resp, err := d.routeToProcess(nil, cmd, args, "*")
			require.False(t, errors.Is(err, ErrPluginProcessNotRunning), "a refused line reached the process lookup")
			require.NotNil(t, resp)
			assert.Equal(t, err.Error(), resp.Error, "the response carries the refusal the error does")
			return err
		})
	})

	t.Run("plugin process (R9) writes a bound selector by its keyword", func(t *testing.T) {
		selectorDefs := []command.ArgDef{
			commandtest.Must(command.NewStringArg("selector", nil, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewUintArg("asn", 32, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewUintArg("hold-time", 16, nil, command.ArgOptions{Mandatory: true})),
		}
		const selectorPath = "show test route selector faults"
		var replaced command.ArgDefSource
		replaced = command.ReplaceArgDefSourceForTest(func(path string) ([]command.ArgDef, error) {
			if path == selectorPath {
				return selectorDefs, nil
			}
			return replaced(path)
		})
		t.Cleanup(func() { command.ReplaceArgDefSourceForTest(replaced) })

		d := NewDispatcher()
		cmd := &RegisteredCommand{Name: selectorPath, LowerName: selectorPath}
		ctx := &CommandContext{Selectors: map[string]string{selectorLeaf: "192.0.2.7"}}
		_, err := d.routeToProcess(ctx, cmd, nil, "192.0.2.7")
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			"\nfix: show test route selector faults selector 192.0.2.7 asn <asn> hold-time <hold-time>")
	})

	t.Run("subsystem", func(t *testing.T) {
		d := NewDispatcher()
		handler := NewSubsystemHandler(SubsystemConfig{Name: "test"})
		handler.commands = []string{routeFaultsPath}
		requireRouteRefusals(t, func(args []string) error {
			line := routeFaultsPath
			for _, arg := range args {
				line += " " + arg
			}
			_, err := d.dispatchSubsystem(&CommandContext{}, handler, line)
			require.False(t, errors.Is(err, ErrSubsystemNotRunning), "a refused line reached the subsystem")
			return err
		})
	})

	t.Run("RPC wrapper (R4)", func(t *testing.T) {
		s := &Server{dispatcher: NewDispatcher()}
		rpcHandler := s.wrapHandler(func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			t.Error("handler must not run on a refused line")
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, routeFaultsPath, true)
		requireRouteRefusals(t, func(args []string) error {
			encoded, err := json.Marshal(rpcParams{Args: args})
			require.NoError(t, err)
			_, err = rpcHandler("m", encoded)
			return err
		})
	})

	t.Run("SSH streaming (R8)", func(t *testing.T) {
		streamingHandlersMu.Lock()
		saved := streamingHandlers
		streamingHandlers = make(map[string]streamingRegistration)
		streamingHandlersMu.Unlock()
		t.Cleanup(func() {
			streamingHandlersMu.Lock()
			streamingHandlers = saved
			streamingHandlersMu.Unlock()
		})
		RegisterStreamingHandler(routeFaultsPath, OneShotSample, func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error {
			t.Error("handler must not run on a refused line")
			return nil
		})
		requireRouteRefusals(t, func(args []string) error {
			line := routeFaultsPath
			for _, arg := range args {
				line += " " + arg
			}
			handler, _, err := GetStreamingHandlerForCommand(line)
			assert.Nil(t, handler, "a refused line answered a handler")
			return err
		})
	})

	t.Run("Dispatch, the reference", func(t *testing.T) {
		d := NewDispatcher()
		require.NoError(t, d.RegisterWithOptions(routeFaultsPath, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			t.Error("handler must not run on a refused line")
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, "Show route faults", RegisterOptions{ReadOnly: true, ArgDefs: []command.ArgDef{
			commandtest.Must(command.NewUintArg("asn", 32, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewUintArg("hold-time", 16, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewStringArg("router-id", nil, nil, command.ArgOptions{})),
		}}))
		requireRouteRefusals(t, func(args []string) error {
			line := routeFaultsPath
			for _, arg := range args {
				line += " " + arg
			}
			_, err := d.Dispatch(&CommandContext{}, line)
			return err
		})
	})
}

// TestDispatcherRestatesOneValueFault proves a line whose one fault is a value
// its leaf refuses is restated with the whole command, the same on the daemon
// dispatcher and on a model route.
//
// GOAL: `show demo raw gzip DEADBEEF` names the command and the argument, lists
// the accepted values with their help, and gives the line to type with the
// placeholder where gzip was, whichever route judged it.
// METHOD: register the command with an enum and a hex argument on a
// dispatcher, and declare the same definitions to the model, then refuse the
// line through Dispatch and through command.ValidateModelArgs.
// VALIDATES: AC-12 one-fault case on the dispatcher route (restateArgumentFaults)
// and a model route (ValidateModelArgs), A-10.
// PREVENTS: a lone value fault returned bare, which no route restated, so it
// named no command to type.
func TestDispatcherRestatesOneValueFault(t *testing.T) {
	const path = "show demo raw"
	defs := []command.ArgDef{
		commandtest.Must(command.NewEnumArg("encoding", []string{"hex", "b64"}, command.ArgOptions{
			Mandatory: true,
			EnumHelp:  map[string]string{"hex": "Hexadecimal digits", "b64": "Base64 text"},
		})),
		commandtest.Must(command.NewStringArg("data", nil, []*regexp.Regexp{regexp.MustCompile(`^[0-9A-Fa-f]+$`)}, command.ArgOptions{Mandatory: true})),
	}
	const want = `show demo raw encoding: "gzip" is not an accepted value` + "\n" +
		"encoding accepts only these values:\n" +
		"  hex  Hexadecimal digits\n" +
		"  b64  Base64 text\n" +
		"fix: show demo raw <hex|b64> DEADBEEF"

	d := NewDispatcher()
	require.NoError(t, d.RegisterWithOptions(path, func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("handler must not run on a refused line")
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "Demo", RegisterOptions{ArgDefs: defs, ReadOnly: true}))

	resp, err := d.Dispatch(&CommandContext{}, path+" gzip DEADBEEF")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, want, err.Error(), "the dispatcher restates the one fault")
	assert.Equal(t, want, resp.Error, "the response carries the refusal the error does")

	var replaced command.ArgDefSource
	replaced = command.ReplaceArgDefSourceForTest(func(p string) ([]command.ArgDef, error) {
		if p == path {
			return defs, nil
		}
		return replaced(p)
	})
	t.Cleanup(func() { command.ReplaceArgDefSourceForTest(replaced) })
	_, err = command.ValidateModelArgs(path, []string{"gzip", "DEADBEEF"}, nil)
	require.Error(t, err)
	assert.Equal(t, want, err.Error(), "a model route restates the one fault as the dispatcher does")
}
