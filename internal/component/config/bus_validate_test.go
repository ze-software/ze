// Design: docs/architecture/api/architecture.md -- unit tests covering all eleven bus commit refusals and retired hints
// Related: bus_validate.go -- ValidateBusConfig and registration seams
// Related: retired.go -- retired keyword hints

package config

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func registerBusTestPlugins(t *testing.T) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, events.RegisterMessage("testbus", "update",
		events.TagDef{Name: "direction", Values: []string{"received", "sent"}},
		events.TagDef{Name: "source"},
	))
	require.NoError(t, events.RegisterMessage("testbus", "state"))
	require.NoError(t, events.RegisterMessage("testbus", "alert"))

	noopEngine := func(net.Conn) int { return 0 }
	noopCLI := func([]string) int { return 0 }

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-emitter",
		Description: "bus validate test emitter",
		RunEngine:   noopEngine,
		CLIHandler:  noopCLI,
		Sends: []rpc.MessageSendDecl{
			{
				Message: "testbus:update",
				Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"received", "sent"}}},
			},
			{Message: "testbus:state"},
		},
		Receives: []rpc.MessageReceiveDecl{
			{Message: "testbus:alert"},
		},
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-collector",
		Description: "bus validate test collector",
		RunEngine:   noopEngine,
		CLIHandler:  noopCLI,
		Sends: []rpc.MessageSendDecl{
			{Message: "testbus:alert"},
		},
		Receives: []rpc.MessageReceiveDecl{
			{
				Message: "testbus:update",
				Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"received"}}},
			},
			{Message: "testbus:state"},
		},
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:              "test-inproc-only",
		Description:       "bus validate test in-process-only plugin",
		RunEngine:         noopEngine,
		CLIHandler:        noopCLI,
		RunsInProcessOnly: true,
		Receives: []rpc.MessageReceiveDecl{
			{Message: "testbus:state"},
		},
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-singleton",
		Description: "bus validate test singleton participant",
		RunEngine:   noopEngine,
		CLIHandler:  noopCLI,
		ParticipantNodes: []rpc.ParticipantNodeDecl{{
			Module:     "ze-test-conf",
			Prefix:     "test",
			SchemaPath: "/test:test/test:service",
		}},
		Sends: []rpc.MessageSendDecl{
			{Message: "testbus:alert"},
		},
		Receives: []rpc.MessageReceiveDecl{
			{Message: "testbus:state"},
		},
	}))
}

func makeBusTree(endpoints map[string]*Tree, participants map[string]*Tree) *Tree {
	root := NewTree()
	bus := NewTree()
	for name, ep := range endpoints {
		bus.AddListEntry("endpoint", name, ep)
	}
	for path, p := range participants {
		bus.AddListEntry("participant", path, p)
	}
	root.SetContainer("bus", bus)
	return root
}

func internalEndpointTree(pluginName string) *Tree {
	ep := NewTree()
	ep.Set("internal", pluginName)
	return ep
}

func assertOperErrContains(t *testing.T, err error, substrs ...string) {
	t.Helper()
	require.Error(t, err)
	opErr, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok, "expected *opererr.Error, got %T: %v", err, err)
	assert.NotEmpty(t, opErr.Fix(), "operator error must carry a fix line")
	for _, s := range substrs {
		assert.Contains(t, err.Error(), s)
	}
}

// TestBusCommitRefusals verifies structural endpoint checks and all eleven
// commit refusals, plus acceptance of the corrected configurations.
//
// VALIDATES: Structural checks (internal vs external, process separate on
// RunsInProcessOnly, Q-1 none exclusivity) and Refusals 1 through 11.
// PREVENTS: Invalid bus filters, conflicting two-place definitions, dangling
// references, or mismatched tag filters passing commit validation.
func TestBusCommitRefusals(t *testing.T) {
	registerBusTestPlugins(t)

	t.Run("structural_internal_external_exclusive", func(t *testing.T) {
		ep := internalEndpointTree("test-emitter")
		ext := NewTree()
		ext.Set("run", "/bin/true")
		ep.SetContainer("external", ext)
		tree := makeBusTree(map[string]*Tree{"both": ep}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "both", "mutually exclusive")
	})

	t.Run("structural_neither_internal_nor_external", func(t *testing.T) {
		tree := makeBusTree(map[string]*Tree{"empty": NewTree()}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "empty", "either internal or external")
	})

	t.Run("structural_process_separate_on_inprocess_only_refused", func(t *testing.T) {
		ep := internalEndpointTree("test-inproc-only")
		ep.Set("process", "separate")
		tree := makeBusTree(map[string]*Tree{"inproc": ep}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "inproc", "RunsInProcessOnly", "process in-daemon")

		// Corrected: in-daemon passes.
		ep.Set("process", "in-daemon")
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("structural_process_on_external_refused", func(t *testing.T) {
		ep := NewTree()
		ext := NewTree()
		ext.Set("run", "/bin/true")
		ep.SetContainer("external", ext)
		ep.Set("process", "separate")
		tree := makeBusTree(map[string]*Tree{"ext": ep}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "ext", "internal")
	})

	t.Run("structural_q1_none_mixed_refused", func(t *testing.T) {
		ep := internalEndpointTree("test-emitter")
		send := NewTree()
		send.SetSlice("allow", []string{"none", "testbus:state"})
		ep.SetContainer("send", send)
		tree := makeBusTree(map[string]*Tree{"em": ep}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "none", "send > allow")
	})

	t.Run("refusal_1_and_8_unknown_or_deleted_target", func(t *testing.T) {
		em := internalEndpointTree("test-emitter")
		send := NewTree()
		send.SetSlice("to", []string{"/bus/endpoint/deleted"})
		em.SetContainer("send", send)
		tree := makeBusTree(map[string]*Tree{
			"em":   em,
			"coll": internalEndpointTree("test-collector"),
		}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "/bus/endpoint/deleted", "/bus/endpoint/coll")

		// Corrected: point to existing collector.
		send.SetSlice("to", []string{"/bus/endpoint/coll"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_1_secondary_path_resolver_seam", func(t *testing.T) {
		restore := ResetBusSecondaryPathResolversForTest()
		t.Cleanup(restore)

		RegisterBusSecondaryPathResolver(BusSecondaryPathResolver{
			Prefix: "/proto/ip/",
			Resolve: func(_ *Tree, path string) ([]string, bool, string, error) {
				if path == "/proto/ip/198.51.100.7" {
					return []string{"/bus/endpoint/em"}, true, "accepted range 198.51.100.0/24", nil
				}
				return nil, false, "accepted range 198.51.100.0/24", errors.New("outside accepted range")
			},
		})

		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		recv.SetSlice("from", []string{"/proto/ip/203.0.113.9"})
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{
			"em":   internalEndpointTree("test-emitter"),
			"coll": coll,
		}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "/proto/ip/203.0.113.9", "198.51.100.0/24")

		// Dormant address inside range passes.
		recv.SetSlice("from", []string{"/proto/ip/198.51.100.7"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_2_send_to_nobody_receives_message", func(t *testing.T) {
		em := internalEndpointTree("test-emitter")
		send := NewTree()
		msgRule := NewTree()
		// test-inproc-only receives testbus:state only, NOT testbus:update.
		msgRule.SetSlice("to", []string{"/bus/endpoint/inproc"})
		send.AddListEntry("message", "testbus:update", msgRule)
		em.SetContainer("send", send)
		tree := makeBusTree(map[string]*Tree{
			"em":     em,
			"inproc": internalEndpointTree("test-inproc-only"),
			"coll":   internalEndpointTree("test-collector"),
		}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "/bus/endpoint/inproc", "testbus:update", "/bus/endpoint/coll")

		// Corrected: target collector which declared receiving testbus:update.
		msgRule.SetSlice("to", []string{"/bus/endpoint/coll"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_3_receive_from_nobody_sends_message", func(t *testing.T) {
		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		msgRule := NewTree()
		// test-inproc-only declares no sends at all.
		msgRule.SetSlice("from", []string{"/bus/endpoint/inproc"})
		recv.AddListEntry("message", "testbus:state", msgRule)
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{
			"em":     internalEndpointTree("test-emitter"),
			"inproc": internalEndpointTree("test-inproc-only"),
			"coll":   coll,
		}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "/bus/endpoint/inproc", "testbus:state", "/bus/endpoint/em")

		// Corrected: receive from emitter.
		msgRule.SetSlice("from", []string{"/bus/endpoint/em"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_4_undeclared_message_in_allow_or_message", func(t *testing.T) {
		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		recv.SetSlice("allow", []string{"testbus:alert"})
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{"coll": coll}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "testbus:alert", "testbus:state", "testbus:update")

		// Corrected: allow a declared message.
		recv.SetSlice("allow", []string{"testbus:state"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_5_per_message_target_excluded_by_allow", func(t *testing.T) {
		em := internalEndpointTree("test-emitter")
		send := NewTree()
		send.SetSlice("allow", []string{"testbus:state"})
		msgRule := NewTree()
		msgRule.SetSlice("to", []string{"/bus/endpoint/coll"})
		send.AddListEntry("message", "testbus:update", msgRule)
		em.SetContainer("send", send)
		tree := makeBusTree(map[string]*Tree{
			"em":   em,
			"coll": internalEndpointTree("test-collector"),
		}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "send > allow", "send > message testbus:update > to")

		// Corrected: include testbus:update in allow.
		send.SetSlice("allow", []string{"testbus:state", "testbus:update"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_6_two_places_conflict_vs_identical_accepted", func(t *testing.T) {
		em := internalEndpointTree("test-emitter")
		sendEp := NewTree()
		sendEp.SetSlice("allow", []string{"testbus:state"})
		em.SetContainer("send", sendEp)

		partEntry := NewTree()
		sendPart := NewTree()
		sendPart.SetSlice("allow", []string{"testbus:update"})
		partEntry.SetContainer("send", sendPart)

		tree := makeBusTree(
			map[string]*Tree{"em": em},
			map[string]*Tree{"/bus/endpoint/em": partEntry},
		)
		assertOperErrContains(t, ValidateBusConfig(tree), "bus > endpoint em > send > allow", "bus > participant /bus/endpoint/em > send > allow")

		// Identical sets in different order are accepted.
		sendEp.SetSlice("allow", []string{"testbus:state", "testbus:update"})
		sendPart.SetSlice("allow", []string{"testbus:update", "testbus:state"})
		require.NoError(t, ValidateBusConfig(tree))

		// Singleton plugin config section vs endpoint conflict also refused.
		singletonEp := internalEndpointTree("test-singleton")
		sRecvEp := NewTree()
		sRecvEp.SetSlice("allow", []string{"none"})
		singletonEp.SetContainer("receive", sRecvEp)

		treeSingleton := makeBusTree(map[string]*Tree{"svc": singletonEp}, nil)
		testRoot := NewTree()
		svcNode := NewTree()
		svcBus := NewTree()
		sRecvNode := NewTree()
		sRecvNode.SetSlice("allow", []string{"testbus:state"})
		svcBus.SetContainer("receive", sRecvNode)
		svcNode.SetContainer("bus", svcBus)
		testRoot.SetContainer("service", svcNode)
		treeSingleton.SetContainer("test", testRoot)
		assertOperErrContains(t, ValidateBusConfig(treeSingleton), "/test/service > bus > receive > allow", "bus > endpoint svc > receive > allow")
	})

	t.Run("refusal_7_feature_requirement_validator_seam", func(t *testing.T) {
		restore := ResetBusRequirementValidatorsForTest()
		t.Cleanup(restore)

		RegisterBusRequirementValidator("test-req", func(_ *Tree, ctx *BusValidationContext) []error {
			if p := ctx.Participants[plugin.EndpointPath("coll")]; p != nil && p.Block.Receive.AllowNone {
				return []error{opererr.New(
					"feature test-req requires /bus/endpoint/coll to receive testbus:state",
					"receive > allow none removed the required relationship",
					"bus { endpoint coll { receive { allow [ testbus:state ]; } } }")}
			}
			return nil
		})

		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		recv.SetSlice("allow", []string{"none"})
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{"coll": coll}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "feature test-req", "testbus:state")

		recv.SetSlice("allow", []string{"testbus:state"})
		require.NoError(t, ValidateBusConfig(tree))
	})

	t.Run("refusal_9_undeclared_tag", func(t *testing.T) {
		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		msgRule := NewTree()
		tagEntry := NewTree()
		tagEntry.SetSlice("value", []string{"foo"})
		msgRule.AddListEntry("tag", "nosuchtag", tagEntry)
		recv.AddListEntry("message", "testbus:update", msgRule)
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{"coll": coll}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "nosuchtag", "direction", "source")
	})

	t.Run("refusal_10_tag_value_outside_closed_set", func(t *testing.T) {
		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		msgRule := NewTree()
		tagEntry := NewTree()
		tagEntry.SetSlice("value", []string{"sideways"})
		msgRule.AddListEntry("tag", "direction", tagEntry)
		recv.AddListEntry("message", "testbus:update", msgRule)
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{"coll": coll}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "sideways", "received", "sent")
	})

	t.Run("refusal_11_tag_disjoint_from_registered_filter", func(t *testing.T) {
		coll := internalEndpointTree("test-collector")
		recv := NewTree()
		msgRule := NewTree()
		tagEntry := NewTree()
		// test-collector registered direction=[received], so writing [sent] shares no value.
		tagEntry.SetSlice("value", []string{"sent"})
		msgRule.AddListEntry("tag", "direction", tagEntry)
		recv.AddListEntry("message", "testbus:update", msgRule)
		coll.SetContainer("receive", recv)
		tree := makeBusTree(map[string]*Tree{"coll": coll}, nil)
		assertOperErrContains(t, ValidateBusConfig(tree), "sent", "received", "share no value")

		// Corrected: received matches registered filter.
		tagEntry.SetSlice("value", []string{"received"})
		require.NoError(t, ValidateBusConfig(tree))
	})
}

// TestRetiredKeywordsBusHints verifies AC-10 retired keyword hints for plugin,
// attach, process, and exabgp point operators to bus syntax.
//
// VALIDATES: RetiredKeywordHint returns bus-syntax guidance for all four retired keywords.
// PREVENTS: Bare unknown-keyword parse failures with no migration hint.
func TestRetiredKeywordsBusHints(t *testing.T) {
	for _, kw := range []string{"plugin", "attach", "process", "exabgp"} {
		hint := RetiredKeywordHint(kw)
		assert.NotEmpty(t, hint, "keyword %q must have a retired hint", kw)
		assert.Contains(t, hint, "bus", "hint for %q must point to bus syntax", kw)
	}
}
