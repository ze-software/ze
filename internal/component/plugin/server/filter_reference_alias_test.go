package server

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// aliasTestRegistration is the registry row an aliased block runs in these
// tests, the way `plugin { internal gr { use bgp-gr; } }` runs bgp-gr as "gr".
const aliasTestRegistration = "test-alias-inserter"

// aliasTestFact is a session fact only these tests register, so an insertion
// can carry a condition without the BGP view linked.
const aliasTestFact = "test-alias-fact"

var registerAliasTestFact sync.Once

// aliasedSink is the driver's view of an aliased block: its process is named
// block and it runs registration.
type aliasedSink struct {
	*recordingSink
	block        string
	registration string
}

func (a *aliasedSink) pluginName() string { return a.block }

func (a *aliasedSink) registrationName() string { return a.registration }

// TestStageOneJudgesAliasedInsertionsUnderTheRegistration proves an aliased
// block's Stage 1 judges its filters and insertions under the registration
// the block runs: the plugin declares `<registration>:<filter>`, because it
// cannot know the operator's block name, and Stage 1 accepts it. The loaded
// config's references are judged under the same name.
//
// VALIDATES: an `internal gr { use bgp-gr; }` block passes Stage 1 with its
// own `bgp-gr:llgr-ingress` insertion.
// PREVENTS: Stage 1 refusing "declares the filter insertion ... which the
// plugin does not serve" for every aliased inserting plugin.
// Method: the shared driver over a net.Pipe, a sink whose block name and
// registration differ, and a recording filter-declaration judge.
func TestStageOneJudgesAliasedInsertionsUnderTheRegistration(t *testing.T) {
	registerAliasTestFact.Do(func() { registry.RegisterSessionFact(aliasTestFact) })
	var judged []string
	previous := registry.SetFilterDeclarationCheck(func(name string, _ []registry.DeclaredFilter) error {
		judged = append(judged, name)
		return nil
	})
	t.Cleanup(func() { registry.SetFilterDeclarationCheck(previous) })

	pc, pluginMux := newDriverPipe(t)
	sink := &aliasedSink{recordingSink: &recordingSink{pc: pc}, block: "gr", registration: aliasTestRegistration}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stage1 := make(chan error, 1)
	go func() {
		_, err := pluginMux.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{
			ProtocolVersion: rpc.ProtocolVersion,
			Filters:         []rpc.FilterDecl{{Name: "ingress", Direction: rpc.FilterImport}},
			FilterInsertions: []rpc.FilterInsertion{{
				Filter: aliasTestRegistration + ":ingress", Direction: rpc.FilterImport, Priority: 10,
				Session: []rpc.FilterSessionFact{{Fact: aliasTestFact}},
			}},
		})
		stage1 <- err
		cancel()
	}()
	// The plugin side stops after Stage 1, so its answer is the verdict and the
	// driver's own error (the canceled Stage 2) says nothing.
	_ = runStartupHandshake(ctx, sink)

	require.NoError(t, <-stage1, "Stage 1 refused the aliased block's own insertion")
	assert.Equal(t, []string{aliasTestRegistration}, judged,
		"the filters were judged under the block name, not the registration a reference names")
}

// aliasedSlotProcess is a block named block running registration, whose
// bridge holds a ready filter slot answering action and counting its calls.
func aliasedSlotProcess(t *testing.T, block, registration string, action rpc.FilterAction, calls *atomic.Int32) *process.Process {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	conn := ipc.NewPluginConn(engineEnd, engineEnd)
	bridge := rpc.NewDirectBridge()
	bridge.SetFilterSlot(func(*rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
		calls.Add(1)
		return &rpc.FilterUpdateOutput{Action: action}, nil
	})
	bridge.SetReady()
	conn.SetBridge(bridge)
	proc := process.NewProcess(plugin.PluginConfig{Name: block, Internal: true, Run: registration})
	proc.SetConn(conn)
	proc.SetRegistration(&plugin.PluginRegistration{Filters: []plugin.FilterRegistration{{
		DeclaredFilter: registry.DeclaredFilter{Name: "ingress", Direction: rpc.FilterImport},
		OnError:        rpc.OnErrorAccept,
	}}})
	return proc
}

// TestFilterReferenceReachesTheAliasedBlock proves a placed member
// `<registration>:<filter>` reaches the block that runs the registration
// whatever the operator called it: the route is judged by that block's slot,
// and the declaration and on-error the chain compiles from are that block's.
// The block name is not a reference.
//
// VALIDATES: an inserted `bgp-gr:llgr-ingress` judged by `internal gr { use
// bgp-gr; }` at run time.
// PREVENTS: an aliased block's members failing closed as "unknown plugin".
// Method: a process manager holding one aliased block with a live slot.
func TestFilterReferenceReachesTheAliasedBlock(t *testing.T) {
	registerSessionReadyPlugin(t, aliasTestRegistration, false)
	s, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	pm := process.NewProcessManager(nil)
	s.procManager.Store(pm)
	var calls atomic.Int32
	pm.AddProcess("gr", aliasedSlotProcess(t, "gr", aliasTestRegistration, rpc.FilterReject, &calls))

	input := rpc.FilterUpdateInput{Filter: "ingress"}
	route := rpc.InputFilterRoute(&input)
	out, err := s.CallFilterUpdate(context.Background(), aliasTestRegistration, &route)
	require.NoError(t, err, "the member naming the registration did not reach the aliased block")
	assert.Equal(t, rpc.FilterReject, out.Action)
	assert.Equal(t, int32(1), calls.Load())

	_, _, onError, ok := s.FilterDeclaration(aliasTestRegistration, "ingress")
	assert.True(t, ok, "the chain found no declaration for the aliased block's filter")
	assert.Equal(t, rpc.OnErrorAccept, onError)
	assert.Equal(t, rpc.OnErrorAccept, s.FilterOnError(aliasTestRegistration, "ingress"))
	declared, ok := s.declaredFilters(aliasTestRegistration)
	assert.True(t, ok, "a reload could not judge a reference against the aliased block")
	assert.Len(t, declared, 1)

	_, err = s.CallFilterUpdate(context.Background(), "gr", &route)
	require.Error(t, err, "the block name answered as a reference")
	assert.True(t, strings.Contains(err.Error(), `unknown plugin "gr"`), err.Error())
}
