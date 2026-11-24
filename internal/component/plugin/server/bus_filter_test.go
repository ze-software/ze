// Design: docs/architecture/api/architecture.md -- compiled bus filter, tag interning, and zero-alloc delivery verification
// Related: bus_filter.go -- CompileBus, CompiledBus, CompiledParticipantFilter
// Related: bus_tag_intern.go -- TagInterner and BusTagMatcher

package server

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testNSProto = "busproto"
	testNSAcme  = "busacme"

	testMsgState       = testNSProto + ":state"
	testMsgUpdate      = testNSProto + ":update"
	testMsgRefresh     = testNSProto + ":refresh"
	testMsgRaw         = testNSProto + ":raw"
	testMsgRouteChange = testNSProto + ":route-change"
	testMsgAcmeAlert   = testNSAcme + ":alert"
	testMsgCLIShow     = "cli:show route"
	testMsgCLIClear    = "cli:clear route"

	testPathEdge1     = "/proto/node/edge1"
	testPathEdge1IP   = "/proto/ip/192.0.2.1"
	testPathLab       = "/proto/node/lab"
	testPathLabIP     = "/proto/ip/192.0.2.9"
	testPathTransit   = "/proto/section/transit"
	testPathT1        = "/proto/section/transit/node/t1"
	testPathT2        = "/proto/section/transit/node/t2"
	testPathCollector = "/bus/endpoint/collector"
	testPathRIB       = "/bus/endpoint/rib"
	testPathSender    = "/bus/endpoint/sender"
)

func registerTestBusMessages(tb testing.TB) {
	tb.Helper()
	defs := []events.MessageDef{
		{Namespace: testNSProto, Name: "state"},
		{Namespace: testNSProto, Name: "update", Tags: []events.TagDef{events.DirectionTagDef()}},
		{Namespace: testNSProto, Name: "refresh", Tags: []events.TagDef{events.DirectionTagDef()}},
		{Namespace: testNSProto, Name: "raw"},
		{
			Namespace: testNSProto,
			Name:      "route-change",
			Tags: []events.TagDef{
				{Name: "source", Values: []string{"proto", "static", "connected"}},
				{Name: "scope"},
			},
		},
		{Namespace: testNSAcme, Name: "alert"},
	}
	for _, def := range defs {
		if err := events.RegisterMessageDef(def); err != nil {
			tb.Fatalf("RegisterMessageDef(%s): %v", def.FullName(), err)
		}
	}
}

func testNodeInput(path, ipPath, parentPath string) BusParticipantInput {
	var secondary []string
	if ipPath != "" {
		secondary = []string{ipPath}
	}
	return BusParticipantInput{
		Path:           path,
		OwnerEndpoint:  "routing-engine",
		SecondaryPaths: secondary,
		ParentPath:     parentPath,
		Sends: []rpc.MessageSendDecl{
			{Message: testMsgState},
			{Message: testMsgUpdate, Tags: []rpc.MessageTagDecl{{Name: events.TagDirection, Values: []string{events.DirectionReceived, events.DirectionSent}}}},
		},
		Receives: []rpc.MessageReceiveDecl{
			{Message: testMsgUpdate, From: []string{"/bus/endpoint"}},
			{Message: testMsgRefresh, From: []string{"/bus/endpoint"}},
			{Message: testMsgRaw, From: []string{"/bus/endpoint"}},
		},
	}
}

func testCollectorInput() BusParticipantInput {
	return BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Receives: []rpc.MessageReceiveDecl{
			{Message: testMsgState},
			{Message: testMsgUpdate, Tags: []rpc.MessageTagDecl{{Name: events.TagDirection, Values: []string{events.DirectionReceived}}}},
		},
	}
}

func testRIBInput() BusParticipantInput {
	return BusParticipantInput{
		Path:         testPathRIB,
		EndpointName: "rib",
		Sends: []rpc.MessageSendDecl{
			{Message: testMsgUpdate},
		},
		Receives: []rpc.MessageReceiveDecl{
			{Message: testMsgState},
			{Message: testMsgUpdate, Tags: []rpc.MessageTagDecl{{Name: events.TagDirection, Values: []string{events.DirectionReceived}}}},
		},
	}
}

func testSenderInput() BusParticipantInput {
	return BusParticipantInput{
		Path:         testPathSender,
		EndpointName: "sender",
		Sends: []rpc.MessageSendDecl{
			{Message: testMsgUpdate},
			{Message: testMsgRefresh},
		},
	}
}

// TestBusFilterCompilation validates default wiring, to/from overrides,
// per-message precedence, parent inheritance, allow-lists, two-place agreement
// vs conflict, central endpoint limits, and canonical/prefix/secondary path
// equivalence across scenarios S1 through S12.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4, AC-5, AC-7, AC-22, S1..S12.
// PREVENTS: Filter rules drifting between compilation, delivery, and send checks.
func TestBusFilterCompilation(t *testing.T) {
	registerTestBusMessages(t)
	nsProto := events.LookupNamespaceID(testNSProto)
	etState := events.LookupEventTypeID("state")
	etUpdate := events.LookupEventTypeID("update")

	t.Run("S1_DefaultWiring", func(t *testing.T) {
		participants := []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			testCollectorInput(),
			testRIBInput(),
		}
		bus, err := CompileBus(NewTagInterner(), participants)
		if err != nil {
			t.Fatalf("CompileBus: %v", err)
		}

		gotState := bus.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil)
		if !slices.Equal(gotState, []string{"collector", "rib"}) {
			t.Fatalf("state receivers = %v, want [collector rib]", gotState)
		}
		gotUpd := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil)
		if !slices.Equal(gotUpd, []string{"collector", "rib"}) {
			t.Fatalf("update(received) receivers = %v, want [collector rib]", gotUpd)
		}
		gotSent := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirSent, nil, nil)
		if len(gotSent) != 0 {
			t.Fatalf("update(sent) receivers = %v, want empty (registered direction=received)", gotSent)
		}

		// Negative: collector declared no send, so sending update is refused undeclared.
		ok, reason := bus.MaySend(testPathCollector, testMsgUpdate, testPathEdge1)
		if ok || reason != SendRefusedUndeclared {
			t.Fatalf("collector MaySend(update) = (%v, %s), want (false, undeclared)", ok, reason)
		}

		// Adding a new node later covers it with no config change.
		participants = append(participants, testNodeInput("/proto/node/new1", "", ""))
		bus2, err := CompileBus(NewTagInterner(), participants)
		if err != nil {
			t.Fatalf("CompileBus after adding new1: %v", err)
		}
		gotNew := bus2.Receivers("/proto/node/new1", nsProto, etState, events.DirUnspecified, nil, nil)
		if !slices.Equal(gotNew, []string{"collector", "rib"}) {
			t.Fatalf("new1 state receivers = %v, want [collector rib]", gotNew)
		}
	})

	t.Run("S2_SendToReplacesDefault", func(t *testing.T) {
		lab := testNodeInput(testPathLab, testPathLabIP, "")
		lab.OwnBlock.Send.TargetsPresent = true
		lab.OwnBlock.Send.Targets = []string{testPathCollector}
		lab.OwnOrigin = BusOriginWritten
		lab.OwnSource = testPathLab + " > bus"

		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			lab,
			testCollectorInput(),
			testRIBInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus: %v", err)
		}

		gotLab := bus.Receivers(testPathLab, nsProto, etUpdate, events.DirReceived, nil, nil)
		if !slices.Equal(gotLab, []string{"collector"}) {
			t.Fatalf("lab receivers = %v, want [collector]", gotLab)
		}
		gotEdge := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil)
		if !slices.Equal(gotEdge, []string{"collector", "rib"}) {
			t.Fatalf("edge1 receivers = %v, want [collector rib]", gotEdge)
		}

		labFilter := bus.Filter(testPathLab)
		rule := labFilter.SendRules[testMsgUpdate]
		if rule.ToOrigin.Origin != BusOriginWritten {
			t.Fatalf("lab send origin = %s, want written", rule.ToOrigin.Origin)
		}
		if !slices.Contains(rule.ReplacedDefaults, testPathRIB) {
			t.Fatalf("lab ReplacedDefaults = %v, want %s included", rule.ReplacedDefaults, testPathRIB)
		}
	})

	t.Run("S3_SendToNone", func(t *testing.T) {
		lab := testNodeInput(testPathLab, testPathLabIP, "")
		lab.OwnBlock.Send.TargetsPresent = true
		lab.OwnBlock.Send.TargetsNone = true

		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			lab,
			testCollectorInput(),
			testRIBInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus: %v", err)
		}

		if got := bus.Receivers(testPathLab, nsProto, etUpdate, events.DirReceived, nil, nil); len(got) != 0 {
			t.Fatalf("lab receivers with to none = %v, want empty", got)
		}
		if ok, reason := bus.MaySend(testPathLab, testMsgUpdate, ""); ok || reason != SendRefusedDestination {
			t.Fatalf("lab MaySend with to none = (%v, %s), want (false, destination)", ok, reason)
		}
		if got := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil); len(got) != 2 {
			t.Fatalf("edge1 receivers = %v, want both collector and rib", got)
		}
	})

	t.Run("S4_ReceiveFromLimits", func(t *testing.T) {
		// Case 1: receive > from limits collector to edge1 only; rib keeps lab.
		colEdge1 := testCollectorInput()
		colEdge1.OwnBlock.Receive.TargetsPresent = true
		colEdge1.OwnBlock.Receive.Targets = []string{testPathEdge1}

		bus1, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			testNodeInput(testPathLab, testPathLabIP, ""),
			colEdge1,
			testRIBInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus case 1: %v", err)
		}
		if got := bus1.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector", "rib"}) {
			t.Fatalf("edge1 receivers = %v, want [collector rib]", got)
		}
		if got := bus1.Receivers(testPathLab, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("lab receivers = %v, want [rib]", got)
		}

		// Case 2: receive > from none silences collector completely.
		colNone := testCollectorInput()
		colNone.OwnBlock.Receive.TargetsPresent = true
		colNone.OwnBlock.Receive.TargetsNone = true

		bus2, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			testNodeInput(testPathLab, testPathLabIP, ""),
			colNone,
			testRIBInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus case 2: %v", err)
		}
		if got := bus2.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("edge1 receivers with collector from none = %v, want [rib]", got)
		}
	})

	t.Run("S5_PerMessageOverridesBlock", func(t *testing.T) {
		lab := testNodeInput(testPathLab, testPathLabIP, "")
		lab.OwnBlock.Send.TargetsPresent = true
		lab.OwnBlock.Send.Targets = []string{testPathRIB}
		lab.OwnBlock.Send.Messages = map[string]*plugin.BusMessageRule{
			testMsgState: {
				TargetsPresent: true,
				Targets:        []string{testPathCollector},
			},
		}

		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			lab,
			testCollectorInput(),
			testRIBInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus: %v", err)
		}

		if got := bus.Receivers(testPathLab, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
			t.Fatalf("lab state receivers = %v, want [collector]", got)
		}
		if got := bus.Receivers(testPathLab, nsProto, etUpdate, events.DirReceived, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("lab update receivers = %v, want [rib]", got)
		}
	})

	t.Run("S6_InheritedAndMemberReplacement", func(t *testing.T) {
		transit := BusParticipantInput{
			Path:      testPathTransit,
			OwnSource: testPathTransit + " > bus",
			OwnBlock: plugin.BusBlock{
				Send: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{testPathRIB},
				},
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{testPathRIB},
				},
			},
		}
		t1 := testNodeInput(testPathT1, "", testPathTransit)
		t1.OwnOrigin = BusOriginWritten
		t1.OwnSource = testPathT1 + " > bus"
		t1.OwnBlock.Send.TargetsPresent = true
		t1.OwnBlock.Send.Targets = []string{testPathCollector}

		t2 := testNodeInput(testPathT2, "", testPathTransit)

		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			transit,
			t1,
			t2,
			testCollectorInput(),
			testRIBInput(),
			testSenderInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus: %v", err)
		}

		if got := bus.Receivers(testPathT2, nsProto, etUpdate, events.DirReceived, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("t2 receivers = %v, want [rib]", got)
		}
		if got := bus.Receivers(testPathT1, nsProto, etUpdate, events.DirReceived, nil, nil); !slices.Equal(got, []string{"collector"}) {
			t.Fatalf("t1 receivers = %v, want [collector]", got)
		}
		t1Filter := bus.Filter(testPathT1)
		if t1Filter.SendRules[testMsgUpdate].ToOrigin.Origin != BusOriginWritten {
			t.Fatalf("t1 send origin = %s, want written", t1Filter.SendRules[testMsgUpdate].ToOrigin.Origin)
		}
		if t1Filter.ReceiveRules[testMsgUpdate].FromOrigin.Origin != BusOriginInherited {
			t.Fatalf("t1 receive origin = %s, want inherited", t1Filter.ReceiveRules[testMsgUpdate].FromOrigin.Origin)
		}

		// sender is blocked by t1's inherited receive > from [ /bus/endpoint/rib ].
		if ok, reason := bus.MaySend(testPathSender, testMsgUpdate, testPathT1); ok || reason != SendRefusedDestination {
			t.Fatalf("sender->t1 MaySend = (%v, %s), want (false, destination)", ok, reason)
		}
		if ok, reason := bus.MaySend(testPathRIB, testMsgUpdate, testPathT1); !ok || reason != SendAllowed {
			t.Fatalf("rib->t1 MaySend = (%v, %s), want (true, allowed)", ok, reason)
		}

		// Member overrides receive > from to allow sender; parent's line no longer applies.
		t1.OwnBlock.Receive.TargetsPresent = true
		t1.OwnBlock.Receive.Targets = []string{testPathSender}
		bus2, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			transit,
			t1,
			t2,
			testCollectorInput(),
			testRIBInput(),
			testSenderInput(),
		})
		if err != nil {
			t.Fatalf("CompileBus override: %v", err)
		}
		if ok, reason := bus2.MaySend(testPathSender, testMsgUpdate, testPathT1); !ok || reason != SendAllowed {
			t.Fatalf("sender->t1 after member override = (%v, %s), want (true, allowed)", ok, reason)
		}
		if ok, reason := bus2.MaySend(testPathRIB, testMsgUpdate, testPathT1); ok || reason != SendRefusedDestination {
			t.Fatalf("rib->t1 after member override = (%v, %s), want (false, destination)", ok, reason)
		}
	})

	t.Run("S7_S8_S9_AllowListsAndRefusals", func(t *testing.T) {
		edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")
		t1 := testNodeInput(testPathT1, "", testPathTransit)
		t2 := testNodeInput(testPathT2, "", testPathTransit)

		// S7 partial send allow-list + S9 block-level to.
		sender := testSenderInput()
		sender.OwnBlock.Send.AllowPresent = true
		sender.OwnBlock.Send.Allow = []string{testMsgUpdate}
		sender.OwnBlock.Send.TargetsPresent = true
		sender.OwnBlock.Send.Targets = []string{testPathTransit}

		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, t1, t2, sender})
		if err != nil {
			t.Fatalf("CompileBus S7/S9: %v", err)
		}
		if ok, reason := bus.MaySend(testPathSender, testMsgUpdate, testPathT1); !ok || reason != SendAllowed {
			t.Fatalf("sender update->t1 = (%v, %s), want (true, allowed)", ok, reason)
		}
		if ok, reason := bus.MaySend(testPathSender, testMsgUpdate, testPathEdge1); ok || reason != SendRefusedDestination {
			t.Fatalf("sender update->edge1 = (%v, %s), want (false, destination)", ok, reason)
		}
		// Refresh must fail by allow-list, not destination.
		if ok, reason := bus.MaySend(testPathSender, testMsgRefresh, testPathT1); ok || reason != SendRefusedAllowList {
			t.Fatalf("sender refresh->t1 = (%v, %s), want (false, allow-list)", ok, reason)
		}
		dests := bus.AllowedDestinations(testPathSender, testMsgUpdate)
		if !slices.Equal(dests, []string{testPathT1, testPathT2}) {
			t.Fatalf("AllowedDestinations = %v, want [%s %s]", dests, testPathT1, testPathT2)
		}

		// S7 allow none refuses both declared sends.
		senderNone := testSenderInput()
		senderNone.OwnBlock.Send.AllowPresent = true
		senderNone.OwnBlock.Send.AllowNone = true
		busNone, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, senderNone})
		if err != nil {
			t.Fatalf("CompileBus allow none: %v", err)
		}
		if ok, reason := busNone.MaySend(testPathSender, testMsgUpdate, testPathEdge1); ok || reason != SendRefusedAllowList {
			t.Fatalf("allow none update = (%v, %s), want (false, allow-list)", ok, reason)
		}

		// S7 Refusal 4: undeclared message in send > allow.
		senderBad := testSenderInput()
		senderBad.OwnBlock.Send.AllowPresent = true
		senderBad.OwnBlock.Send.Allow = []string{testMsgRaw}
		_, err = CompileBus(NewTagInterner(), []BusParticipantInput{edge1, senderBad})
		if err == nil {
			t.Fatal("expected Refusal 4 error for undeclared message in allow, got nil")
		}
		if !strings.Contains(err.Error(), testMsgUpdate) || !strings.Contains(err.Error(), testMsgRefresh) {
			t.Fatalf("Refusal 4 error = %q, expected declared messages listed", err.Error())
		}

		// S9 Refusal 5: per-message to written for message excluded by allow.
		senderRefusal5 := sender
		senderRefusal5.OwnBlock.Send.Messages = map[string]*plugin.BusMessageRule{
			testMsgRefresh: {
				TargetsPresent: true,
				Targets:        []string{testPathEdge1},
			},
		}
		_, err = CompileBus(NewTagInterner(), []BusParticipantInput{edge1, t1, senderRefusal5})
		if err == nil {
			t.Fatal("expected Refusal 5 error for message excluded by allow, got nil")
		}
		if !strings.Contains(err.Error(), "allow") || !strings.Contains(err.Error(), testMsgRefresh) {
			t.Fatalf("Refusal 5 error = %q, expected both allow and message lines", err.Error())
		}

		// S8: receive allow-list partial and none.
		colPartial := testCollectorInput()
		colPartial.OwnBlock.Receive.AllowPresent = true
		colPartial.OwnBlock.Receive.Allow = []string{testMsgState}
		busS8, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, colPartial, testRIBInput()})
		if err != nil {
			t.Fatalf("CompileBus S8: %v", err)
		}
		if got := busS8.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector", "rib"}) {
			t.Fatalf("S8 state receivers = %v, want [collector rib]", got)
		}
		if got := busS8.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("S8 update receivers = %v, want [rib]", got)
		}

		colNone := testCollectorInput()
		colNone.OwnBlock.Receive.AllowPresent = true
		colNone.OwnBlock.Receive.AllowNone = true
		busS8None, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, colNone, testRIBInput()})
		if err != nil {
			t.Fatalf("CompileBus S8 none: %v", err)
		}
		if got := busS8None.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"rib"}) {
			t.Fatalf("S8 allow none state receivers = %v, want [rib]", got)
		}
	})

	t.Run("S10_TwoPlacesAgreeAndDisagree", func(t *testing.T) {
		edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")
		t1 := testNodeInput(testPathT1, "", testPathTransit)
		fwdBlock := plugin.BusBlock{
			Send: plugin.BusDirectionBlock{
				TargetsPresent: true,
				Targets:        []string{testPathTransit},
			},
		}

		// Agreeing in both places -> single filter with both sources recorded.
		fwdAgree := BusParticipantInput{
			Path:          "/proto/forwarder",
			EndpointName:  "forwarder",
			Sends:         []rpc.MessageSendDecl{{Message: testMsgUpdate}},
			OwnBlock:      fwdBlock,
			OwnSource:     "proto > forwarder > bus",
			BusSideBlock:  fwdBlock,
			BusSideSource: "bus > endpoint forwarder",
		}
		busAgree, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, t1, fwdAgree})
		if err != nil {
			t.Fatalf("CompileBus agreeing two places: %v", err)
		}
		rule := busAgree.Filter("/proto/forwarder").SendRules[testMsgUpdate]
		if len(rule.ToOrigin.Sources) != 2 {
			t.Fatalf("agreeing sources = %v, want both sources", rule.ToOrigin.Sources)
		}

		// Disagreeing -> Refusal 6 naming both lines.
		fwdConflict := fwdAgree
		fwdConflict.OwnBlock = plugin.BusBlock{
			Send: plugin.BusDirectionBlock{
				TargetsPresent: true,
				Targets:        []string{testPathEdge1},
			},
		}
		_, err = CompileBus(NewTagInterner(), []BusParticipantInput{edge1, t1, fwdConflict})
		if err == nil {
			t.Fatal("expected Refusal 6 conflict error, got nil")
		}
		if !strings.Contains(err.Error(), "proto > forwarder > bus") || !strings.Contains(err.Error(), "bus > endpoint forwarder") {
			t.Fatalf("Refusal 6 error = %q, want both sources named", err.Error())
		}
	})

	t.Run("S11_CentralEndpointLimit", func(t *testing.T) {
		acme := BusParticipantInput{
			Path:         "/bus/endpoint/acme",
			EndpointName: "acme",
			Sends:        []rpc.MessageSendDecl{{Message: testMsgUpdate}},
			Receives: []rpc.MessageReceiveDecl{
				{Message: testMsgState},
				{Message: testMsgUpdate, Tags: []rpc.MessageTagDecl{{Name: events.TagDirection, Values: []string{events.DirectionReceived}}}},
			},
			BusSideSource: "bus > endpoint acme",
			BusSideBlock: plugin.BusBlock{
				Send: plugin.BusDirectionBlock{
					AllowPresent: true,
					AllowNone:    true,
				},
				Receive: plugin.BusDirectionBlock{
					AllowPresent: true,
					Allow:        []string{testMsgState},
				},
			},
		}
		bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
			testNodeInput(testPathEdge1, testPathEdge1IP, ""),
			acme,
		})
		if err != nil {
			t.Fatalf("CompileBus S11: %v", err)
		}
		if got := bus.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"acme"}) {
			t.Fatalf("acme state receivers = %v, want [acme]", got)
		}
		if got := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil); len(got) != 0 {
			t.Fatalf("acme update receivers = %v, want empty", got)
		}
		if ok, reason := bus.MaySend("/bus/endpoint/acme", testMsgUpdate, testPathEdge1); ok || reason != SendRefusedAllowList {
			t.Fatalf("acme MaySend = (%v, %s), want (false, allow-list)", ok, reason)
		}
	})

	t.Run("S12_PathFormsAndSecondaryEquivalence", func(t *testing.T) {
		edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")
		lab := testNodeInput(testPathLab, testPathLabIP, "")
		t1 := testNodeInput(testPathT1, "", testPathTransit)
		t2 := testNodeInput(testPathT2, "", testPathTransit)

		colByName := testCollectorInput()
		colByName.OwnBlock.Receive.TargetsPresent = true
		colByName.OwnBlock.Receive.Targets = []string{testPathEdge1}

		colByIP := testCollectorInput()
		colByIP.OwnBlock.Receive.TargetsPresent = true
		colByIP.OwnBlock.Receive.Targets = []string{testPathEdge1IP}

		busName, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, lab, t1, t2, colByName})
		if err != nil {
			t.Fatalf("CompileBus by name: %v", err)
		}
		busIP, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, lab, t1, t2, colByIP})
		if err != nil {
			t.Fatalf("CompileBus by secondary IP: %v", err)
		}

		// Canonical and secondary paths resolve to the exact same participant filter and delivery index.
		if busIP.Filter(testPathEdge1) != busIP.Filter(testPathEdge1IP) {
			t.Fatal("Filter(canonical) != Filter(secondary)")
		}
		nameRecvs := busName.Receivers(testPathEdge1, nsProto, etState, events.DirUnspecified, nil, nil)
		ipRecvs := busIP.Receivers(testPathEdge1IP, nsProto, etState, events.DirUnspecified, nil, nil)
		if !slices.Equal(nameRecvs, []string{"collector"}) || !slices.Equal(ipRecvs, []string{"collector"}) {
			t.Fatalf("nameRecvs=%v ipRecvs=%v, want [collector]", nameRecvs, ipRecvs)
		}
		if got := busIP.Receivers(testPathLab, nsProto, etState, events.DirUnspecified, nil, nil); len(got) != 0 {
			t.Fatalf("lab receivers = %v, want empty", got)
		}
		if !slices.Equal(busName.Filter(testPathCollector).ReceiveRules[testMsgState].ResolvedPaths,
			busIP.Filter(testPathCollector).ReceiveRules[testMsgState].ResolvedPaths) {
			t.Fatal("canonical and secondary from paths did not compile to identical resolved paths")
		}

		// Prefix path /proto covers all four nodes; /proto/section/transit covers t1 and t2.
		colPrefix := testCollectorInput()
		colPrefix.OwnBlock.Receive.TargetsPresent = true
		colPrefix.OwnBlock.Receive.Targets = []string{testPathTransit}
		busPrefix, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, lab, t1, t2, colPrefix})
		if err != nil {
			t.Fatalf("CompileBus prefix: %v", err)
		}
		resolved := busPrefix.Filter(testPathCollector).ReceiveRules[testMsgState].ResolvedPaths
		if !slices.Equal(resolved, []string{testPathT1, testPathT2}) {
			t.Fatalf("transit prefix resolved = %v, want [%s %s]", resolved, testPathT1, testPathT2)
		}

		// Dynamic participant covered via ParentPath.
		dyn1Node := testNodeInput("/proto/ip/198.51.100.5", "", "/proto/section/dyn1")
		dyn2Node := testNodeInput("/proto/ip/198.51.100.6", "", "/proto/section/dyn2")
		colDyn := testCollectorInput()
		colDyn.OwnBlock.Receive.TargetsPresent = true
		colDyn.OwnBlock.Receive.Targets = []string{"/proto/section/dyn1"}
		busDyn, err := CompileBus(NewTagInterner(), []BusParticipantInput{dyn1Node, dyn2Node, colDyn})
		if err != nil {
			t.Fatalf("CompileBus dynamic parent: %v", err)
		}
		if got := busDyn.Receivers("/proto/ip/198.51.100.5", nsProto, etState, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
			t.Fatalf("dyn1 receiver = %v, want [collector]", got)
		}
		if got := busDyn.Receivers("/proto/ip/198.51.100.6", nsProto, etState, events.DirUnspecified, nil, nil); len(got) != 0 {
			t.Fatalf("dyn2 receiver = %v, want empty", got)
		}
	})
}

// TestOneSendAllowListSpansNamespaces proves a single send allow-list governs
// both subsystem messages and cli: command prefixes uniformly.
//
// VALIDATES: AC-40, S36.
// PREVENTS: A send allow-list narrowing one namespace from leaving cli: sends open.
func TestOneSendAllowListSpansNamespaces(t *testing.T) {
	registerTestBusMessages(t)

	ops := BusParticipantInput{
		Path:         "/bus/endpoint/ops2",
		EndpointName: "ops2",
		Sends: []rpc.MessageSendDecl{
			{Message: testMsgUpdate},
			{Message: testMsgCLIShow},
			{Message: testMsgCLIClear},
		},
		OwnBlock: plugin.BusBlock{
			Send: plugin.BusDirectionBlock{
				AllowPresent: true,
				Allow:        []string{testMsgUpdate},
			},
		},
	}
	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		testNodeInput(testPathEdge1, testPathEdge1IP, ""),
		ops,
	})
	if err != nil {
		t.Fatalf("CompileBus: %v", err)
	}

	if ok, reason := bus.MaySend("/bus/endpoint/ops2", testMsgUpdate, testPathEdge1); !ok || reason != SendAllowed {
		t.Fatalf("update MaySend = (%v, %s), want (true, allowed)", ok, reason)
	}
	if ok, reason := bus.MaySend("/bus/endpoint/ops2", testMsgCLIShow, ""); ok || reason != SendRefusedAllowList {
		t.Fatalf("cli show MaySend = (%v, %s), want (false, allow-list)", ok, reason)
	}
	if ok, reason := bus.MaySend("/bus/endpoint/ops2", "cli:show route detail", ""); ok || reason != SendRefusedAllowList {
		t.Fatalf("cli show detail MaySend = (%v, %s), want (false, allow-list)", ok, reason)
	}
	if ok, reason := bus.MaySend("/bus/endpoint/ops2", "cli:show interface", ""); ok || reason != SendRefusedUndeclared {
		t.Fatalf("undeclared cli MaySend = (%v, %s), want (false, undeclared)", ok, reason)
	}
	if prefixes := bus.Filter("/bus/endpoint/ops2").AllowedCLIPrefixes; len(prefixes) != 0 {
		t.Fatalf("AllowedCLIPrefixes = %v, want empty", prefixes)
	}

	// Without the allow-list, all declared namespaces pass and AllowedCLIPrefixes is populated.
	ops.OwnBlock = plugin.BusBlock{}
	busOpen, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		testNodeInput(testPathEdge1, testPathEdge1IP, ""),
		ops,
	})
	if err != nil {
		t.Fatalf("CompileBus open: %v", err)
	}
	if ok, reason := busOpen.MaySend("/bus/endpoint/ops2", "cli:show route detail", ""); !ok || reason != SendAllowed {
		t.Fatalf("open cli show detail = (%v, %s), want (true, allowed)", ok, reason)
	}
	if prefixes := busOpen.Filter("/bus/endpoint/ops2").AllowedCLIPrefixes; !slices.Equal(prefixes, []string{"clear route", "show route"}) {
		t.Fatalf("open AllowedCLIPrefixes = %v, want [clear route show route]", prefixes)
	}
}

// TestCentralParticipantBlockCompilesWithOwnSection proves central
// bus > participant blocks compile identically with a container's own block,
// reject same-rank conflicts, and yield to lower-rank member overrides.
//
// VALIDATES: AC-42, S39.
// PREVENTS: Central participant rules diverging from or clobbering member rules.
func TestCentralParticipantBlockCompilesWithOwnSection(t *testing.T) {
	registerTestBusMessages(t)

	recvCollector := plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{
			TargetsPresent: true,
			Targets:        []string{testPathCollector},
		},
	}
	recvEdge1 := plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{
			TargetsPresent: true,
			Targets:        []string{testPathEdge1},
		},
	}

	colSender := BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgUpdate}},
	}
	edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")

	// Case 1 & 2: own section alone vs central bus > participant alone produce equal member filters.
	transitOwn := BusParticipantInput{
		Path:      testPathTransit,
		OwnBlock:  recvCollector,
		OwnSource: testPathTransit + " > bus",
	}
	transitCentral := BusParticipantInput{
		Path:          testPathTransit,
		BusSideBlock:  recvCollector,
		BusSideSource: "bus > participant " + testPathTransit,
	}
	t1 := testNodeInput(testPathT1, "", testPathTransit)
	t2 := testNodeInput(testPathT2, "", testPathTransit)

	busOwn, err := CompileBus(NewTagInterner(), []BusParticipantInput{transitOwn, t1, t2, colSender, edge1})
	if err != nil {
		t.Fatalf("CompileBus own: %v", err)
	}
	busCentral, err := CompileBus(NewTagInterner(), []BusParticipantInput{transitCentral, t1, t2, colSender, edge1})
	if err != nil {
		t.Fatalf("CompileBus central: %v", err)
	}
	if !slices.Equal(busOwn.AllowedDestinations(testPathCollector, testMsgUpdate),
		busCentral.AllowedDestinations(testPathCollector, testMsgUpdate)) {
		t.Fatalf("own=%v vs central=%v differ",
			busOwn.AllowedDestinations(testPathCollector, testMsgUpdate),
			busCentral.AllowedDestinations(testPathCollector, testMsgUpdate))
	}

	// Case 3: both identical -> accepted with both sources on the parent rule.
	transitBoth := transitOwn
	transitBoth.BusSideBlock = recvCollector
	transitBoth.BusSideSource = "bus > participant " + testPathTransit
	busBoth, err := CompileBus(NewTagInterner(), []BusParticipantInput{transitBoth, t1, t2, colSender, edge1})
	if err != nil {
		t.Fatalf("CompileBus both identical: %v", err)
	}
	if got := busBoth.Filter(testPathT1).ReceiveRules[testMsgUpdate].FromOrigin; got.Origin != BusOriginInherited || len(got.Sources) != 2 {
		t.Fatalf("t1 inherited origin = %+v, want inherited with 2 sources", got)
	}

	// Case 4: both differing at parent rank -> Refusal 6 naming both lines.
	transitConflict := transitOwn
	transitConflict.BusSideBlock = recvEdge1
	transitConflict.BusSideSource = "bus > participant " + testPathTransit
	_, err = CompileBus(NewTagInterner(), []BusParticipantInput{transitConflict, t1, t2, colSender, edge1})
	if err == nil {
		t.Fatal("expected Refusal 6 conflict on central vs own parent block")
	}
	if !strings.Contains(err.Error(), testPathTransit+" > bus") || !strings.Contains(err.Error(), "bus > participant "+testPathTransit) {
		t.Fatalf("conflict error = %q, want both lines named", err.Error())
	}

	// Member's own receive > from replaces the parent's central rule without conflict.
	t1Override := t1
	t1Override.OwnBlock = recvEdge1
	t1Override.OwnOrigin = BusOriginWritten
	t1Override.OwnSource = testPathT1 + " > bus"
	busOverride, err := CompileBus(NewTagInterner(), []BusParticipantInput{transitCentral, t1Override, t2, colSender, edge1})
	if err != nil {
		t.Fatalf("CompileBus member override of central parent: %v", err)
	}
	if ok, _ := busOverride.MaySend(testPathCollector, testMsgUpdate, testPathT1); ok {
		t.Fatal("collector should be blocked from t1 after member override")
	}
	if ok, _ := busOverride.MaySend(testPathCollector, testMsgUpdate, testPathT2); !ok {
		t.Fatal("collector should still reach t2 via inherited central parent rule")
	}
}

// TestDormantReception validates that a reception with no active sender or an
// empty runtime tag intersection compiles cleanly as dormant and wakes when a
// matching sender is configured.
//
// VALIDATES: AC-8, S21.
// PREVENTS: Missing senders causing compile errors or silent unrecorded drops.
func TestDormantReception(t *testing.T) {
	registerTestBusMessages(t)
	nsAcme := events.LookupNamespaceID(testNSAcme)
	etAlert := events.LookupEventTypeID("alert")

	col := BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgAcmeAlert}},
	}

	busBefore, err := CompileBus(NewTagInterner(), []BusParticipantInput{col})
	if err != nil {
		t.Fatalf("CompileBus dormant: %v", err)
	}
	f := busBefore.Filter(testPathCollector)
	if len(f.DormantReceptions) != 1 || f.DormantReceptions[0].Message != testMsgAcmeAlert {
		t.Fatalf("DormantReceptions = %+v, want 1 entry for %s", f.DormantReceptions, testMsgAcmeAlert)
	}
	if len(busBefore.Edges()) != 1 || busBefore.Edges()[0].Origin != BusOriginDormant {
		t.Fatalf("Edges = %+v, want 1 dormant edge", busBefore.Edges())
	}

	// Add the sender endpoint and recompile: reception wakes up and delivers.
	alerter := BusParticipantInput{
		Path:         "/bus/endpoint/alerter",
		EndpointName: "alerter",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgAcmeAlert}},
	}
	busAfter, err := CompileBus(NewTagInterner(), []BusParticipantInput{col, alerter})
	if err != nil {
		t.Fatalf("CompileBus active: %v", err)
	}
	if len(busAfter.Filter(testPathCollector).DormantReceptions) != 0 {
		t.Fatalf("DormantReceptions after adding sender = %+v, want empty", busAfter.Filter(testPathCollector).DormantReceptions)
	}
	if got := busAfter.Receivers("/bus/endpoint/alerter", nsAcme, etAlert, events.DirUnspecified, nil, nil); !slices.Equal(got, []string{"collector"}) {
		t.Fatalf("active receivers = %v, want [collector]", got)
	}

	// Runtime tag filter intersecting empty with configured tag filter marks reception dormant.
	rcSender := BusParticipantInput{
		Path:         "/bus/endpoint/rc-sender",
		EndpointName: "rc-sender",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgRouteChange}},
	}
	rcCol := BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		OwnBlock: plugin.BusBlock{
			Receive: plugin.BusDirectionBlock{
				Messages: map[string]*plugin.BusMessageRule{
					testMsgRouteChange: {Tags: map[string][]string{"source": {"proto"}}},
				},
			},
		},
		RuntimeReceiveTags: map[string]map[string][]string{
			testMsgRouteChange: {"source": {"static"}},
		},
	}
	busRtDormant, err := CompileBus(NewTagInterner(), []BusParticipantInput{rcSender, rcCol})
	if err != nil {
		t.Fatalf("CompileBus runtime dormant: %v", err)
	}
	dormant := busRtDormant.Filter(testPathCollector).DormantReceptions
	if len(dormant) != 1 || !strings.Contains(dormant[0].Reason, "runtime filter") {
		t.Fatalf("runtime DormantReceptions = %+v, want 1 runtime-filter conflict entry", dormant)
	}
}

// TestBusTagFilter verifies closed-set direction pre-resolution, interned
// per-message tag filtering, multi-tag AND, outbound tag checks, and commit
// refusals 9, 10, and 11.
//
// VALIDATES: AC-44, S41, M-16.
// PREVENTS: Tag mismatches, missing AND conjunction, or runtime direction overhead.
func TestBusTagFilter(t *testing.T) {
	registerTestBusMessages(t)
	nsProto := events.LookupNamespaceID(testNSProto)
	etUpdate := events.LookupEventTypeID("update")
	etRouteChange := events.LookupEventTypeID("route-change")

	interner := NewTagInterner()
	rcSender := BusParticipantInput{
		Path:         "/bus/endpoint/rc-sender",
		EndpointName: "rc-sender",
		Sends:        []rpc.MessageSendDecl{{Message: testMsgRouteChange}},
	}
	colFiltered := BusParticipantInput{
		Path:         testPathCollector,
		EndpointName: "collector",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		OwnBlock: plugin.BusBlock{
			Receive: plugin.BusDirectionBlock{
				Messages: map[string]*plugin.BusMessageRule{
					testMsgRouteChange: {
						Tags: map[string][]string{
							"source": {"proto"},
							"scope":  {"global", "vrf1"},
						},
					},
				},
			},
		},
	}
	colUnfiltered := BusParticipantInput{
		Path:         "/bus/endpoint/all-watcher",
		EndpointName: "all-watcher",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
	}

	bus, err := CompileBus(interner, []BusParticipantInput{rcSender, colFiltered, colUnfiltered})
	if err != nil {
		t.Fatalf("CompileBus tag filter: %v", err)
	}

	// Unfiltered receiver holds nil InboundTags matcher (zero tag comparisons).
	if bus.Filter("/bus/endpoint/all-watcher").ReceiveRules[testMsgRouteChange].InboundTags != nil {
		t.Fatal("unfiltered receiver should have nil InboundTags matcher")
	}

	tagSourceProto := interner.InternTagValue("source", "proto")
	tagSourceStatic := interner.InternTagValue("source", "static")
	tagScopeGlobal := interner.InternTagValue("scope", "global")
	tagScopeLocal := interner.InternTagValue("scope", "local")

	var dst []string
	// Both source=proto AND scope=global match collector + all-watcher.
	gotBoth := bus.Receivers("/bus/endpoint/rc-sender", nsProto, etRouteChange, events.DirUnspecified,
		[]BusTagValue{tagSourceProto, tagScopeGlobal}, dst[:0])
	if !slices.Equal(gotBoth, []string{"all-watcher", "collector"}) {
		t.Fatalf("matching both tags = %v, want [all-watcher collector]", gotBoth)
	}

	// Wrong source excludes collector, keeps all-watcher.
	gotWrongSource := bus.Receivers("/bus/endpoint/rc-sender", nsProto, etRouteChange, events.DirUnspecified,
		[]BusTagValue{tagSourceStatic, tagScopeGlobal}, dst[:0])
	if !slices.Equal(gotWrongSource, []string{"all-watcher"}) {
		t.Fatalf("wrong source = %v, want [all-watcher]", gotWrongSource)
	}

	// Matching source but wrong scope fails multi-tag AND for collector.
	gotWrongScope := bus.Receivers("/bus/endpoint/rc-sender", nsProto, etRouteChange, events.DirUnspecified,
		[]BusTagValue{tagSourceProto, tagScopeLocal}, dst[:0])
	if !slices.Equal(gotWrongScope, []string{"all-watcher"}) {
		t.Fatalf("wrong scope = %v, want [all-watcher]", gotWrongScope)
	}

	// Closed-set direction tag is pre-resolved into busDeliveryKey with nil runtime TagFilter.
	edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")
	colDir := testCollectorInput()
	busDir, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, colDir})
	if err != nil {
		t.Fatalf("CompileBus direction tag: %v", err)
	}
	if busDir.Filter(testPathCollector).ReceiveRules[testMsgUpdate].InboundTags != nil {
		t.Fatal("direction-only tag filter must pre-resolve into deliveryKey with nil runtime InboundTags")
	}
	if got := busDir.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil); !slices.Equal(got, []string{"collector"}) {
		t.Fatalf("DirReceived receivers = %v, want [collector]", got)
	}
	if got := busDir.Receivers(testPathEdge1, nsProto, etUpdate, events.DirSent, nil, nil); len(got) != 0 {
		t.Fatalf("DirSent receivers = %v, want empty", got)
	}

	// Outbound tag filter enforced in MaySendWithTags.
	rcSenderOut := rcSender
	rcSenderOut.OwnBlock.Send.Messages = map[string]*plugin.BusMessageRule{
		testMsgRouteChange: {Tags: map[string][]string{"source": {"proto"}}},
	}
	outInterner := NewTagInterner()
	busOut, err := CompileBus(outInterner, []BusParticipantInput{rcSenderOut, colUnfiltered})
	if err != nil {
		t.Fatalf("CompileBus outbound tag: %v", err)
	}
	ok, reason := busOut.MaySendWithTags("/bus/endpoint/rc-sender", testMsgRouteChange, "/bus/endpoint/all-watcher",
		[]BusTagValue{outInterner.InternTagValue("source", "proto")})
	if !ok || reason != SendAllowed {
		t.Fatalf("outbound matching tag = (%v, %s), want (true, allowed)", ok, reason)
	}
	ok, reason = busOut.MaySendWithTags("/bus/endpoint/rc-sender", testMsgRouteChange, "/bus/endpoint/all-watcher",
		[]BusTagValue{outInterner.InternTagValue("source", "static")})
	if ok || reason != SendRefusedOutboundTag {
		t.Fatalf("outbound mismatched tag = (%v, %s), want (false, outbound-tag)", ok, reason)
	}

	// Refusal 9: undeclared tag name.
	colBadTag := colUnfiltered
	colBadTag.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
		testMsgRouteChange: {Tags: map[string][]string{"nosuch": {"x"}}},
	}
	if _, err := CompileBus(NewTagInterner(), []BusParticipantInput{rcSender, colBadTag}); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("Refusal 9 error = %v, want declared tags listed", err)
	}

	// Refusal 10: value outside closed tag set.
	colBadVal := colUnfiltered
	colBadVal.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
		testMsgRouteChange: {Tags: map[string][]string{"source": {"nosuch"}}},
	}
	if _, err := CompileBus(NewTagInterner(), []BusParticipantInput{rcSender, colBadVal}); err == nil || !strings.Contains(err.Error(), "proto") {
		t.Fatalf("Refusal 10 error = %v, want closed values listed", err)
	}

	// Refusal 11: configured tag shares no value with registered tag filter.
	colDisjoint := testCollectorInput() // registered direction=[received]
	colDisjoint.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
		testMsgUpdate: {Tags: map[string][]string{events.TagDirection: {events.DirectionSent}}},
	}
	if _, err := CompileBus(NewTagInterner(), []BusParticipantInput{edge1, colDisjoint}); err == nil || !strings.Contains(err.Error(), "shares no value") {
		t.Fatalf("Refusal 11 error = %v, want empty intersection refusal", err)
	}
}

// TestDeliveryEdgeOrigins verifies that Edges reports all five origins
// (registered, written, inherited, required, dormant) and preserves parent
// paths on dynamic participants.
//
// VALIDATES: AC-25, S31.
// PREVENTS: Missing or misattributed edge origins on inspection surfaces.
func TestDeliveryEdgeOrigins(t *testing.T) {
	registerTestBusMessages(t)

	for _, tc := range []struct {
		origin BusEdgeOrigin
		want   string
	}{
		{BusOriginRegistered, "registered"},
		{BusOriginWritten, "written"},
		{BusOriginInherited, "inherited"},
		{BusOriginRequired, "required"},
		{BusOriginDormant, "dormant"},
	} {
		if got := tc.origin.String(); got != tc.want {
			t.Fatalf("BusEdgeOrigin(%d).String() = %q, want %q", tc.origin, got, tc.want)
		}
	}

	dynNode := testNodeInput("/proto/ip/198.51.100.5", "", "/proto/section/dyn1")
	reqNode := testNodeInput(testPathEdge1, testPathEdge1IP, "")
	reqNode.Required = true

	transit := BusParticipantInput{
		Path: testPathTransit,
		OwnBlock: plugin.BusBlock{
			Send: plugin.BusDirectionBlock{
				TargetsPresent: true,
				Targets:        []string{testPathCollector},
			},
		},
	}
	t1 := testNodeInput(testPathT1, "", testPathTransit)

	lab := testNodeInput(testPathLab, testPathLabIP, "")
	lab.OwnBlock.Send.TargetsPresent = true
	lab.OwnBlock.Send.Targets = []string{testPathCollector}

	col := testCollectorInput()
	col.Receives = append(col.Receives, rpc.MessageReceiveDecl{Message: testMsgAcmeAlert})

	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{dynNode, reqNode, transit, t1, lab, col})
	if err != nil {
		t.Fatalf("CompileBus: %v", err)
	}

	seenOrigins := make(map[BusEdgeOrigin]bool)
	sawDynParent := false
	for _, e := range bus.Edges() {
		seenOrigins[e.Origin] = true
		if e.SenderPath == "/proto/ip/198.51.100.5" && e.ParentPath == "/proto/section/dyn1" {
			sawDynParent = true
		}
	}
	for _, want := range []BusEdgeOrigin{
		BusOriginRegistered,
		BusOriginWritten,
		BusOriginInherited,
		BusOriginRequired,
		BusOriginDormant,
	} {
		if !seenOrigins[want] {
			t.Fatalf("missing origin %s in edges: %+v", want, bus.Edges())
		}
	}
	if !sawDynParent {
		t.Fatalf("expected dynamic sender edge with ParentPath /proto/section/dyn1 in %+v", bus.Edges())
	}
}

func buildAllocationTestBus(tb testing.TB) (*CompiledBus, *TagInterner, events.NamespaceID, events.EventTypeID, events.EventTypeID, []BusTagValue) {
	tb.Helper()
	registerTestBusMessages(tb)

	interner := NewTagInterner()
	edge1 := testNodeInput(testPathEdge1, testPathEdge1IP, "")
	edge1.Sends = append(edge1.Sends, rpc.MessageSendDecl{Message: testMsgRouteChange})
	t1 := testNodeInput(testPathT1, "", testPathTransit)

	col := testCollectorInput()
	col.OwnBlock.Receive.TargetsPresent = true
	col.OwnBlock.Receive.Targets = []string{testPathTransit, testPathEdge1IP, "/proto"}

	colTagged := BusParticipantInput{
		Path:         "/bus/endpoint/tagged",
		EndpointName: "tagged",
		Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		OwnBlock: plugin.BusBlock{
			Receive: plugin.BusDirectionBlock{
				Messages: map[string]*plugin.BusMessageRule{
					testMsgRouteChange: {
						Tags: map[string][]string{
							"source": {"proto"},
							"scope":  {"global"},
						},
					},
				},
			},
		},
	}

	sender := testSenderInput()
	sender.OwnBlock.Send.TargetsPresent = true
	sender.OwnBlock.Send.Targets = []string{testPathTransit, testPathEdge1}

	bus, err := CompileBus(interner, []BusParticipantInput{edge1, t1, col, colTagged, testRIBInput(), sender})
	if err != nil {
		tb.Fatalf("CompileBus: %v", err)
	}

	nsProto := events.LookupNamespaceID(testNSProto)
	etUpdate := events.LookupEventTypeID("update")
	etRC := events.LookupEventTypeID("route-change")
	tags := []BusTagValue{
		interner.InternTagValue("source", "proto"),
		interner.InternTagValue("scope", "global"),
	}
	return bus, interner, nsProto, etUpdate, etRC, tags
}

// TestBusDeliveryAllocatesNothing asserts 0.0 allocs/op across broadcast
// delivery, addressed delivery, send checks, and interned tag-filtered delivery.
//
// VALIDATES: AC-17, S24, AC-44.
// PREVENTS: Per-message heap allocations or path parsing leaking onto the delivery hot path.
func TestBusDeliveryAllocatesNothing(t *testing.T) {
	bus, _, nsProto, etUpdate, etRC, msgTags := buildAllocationTestBus(t)
	scratch := make([]string, 0, 8)

	broadcastAllocs := testing.AllocsPerRun(1000, func() {
		got := bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil)
		if len(got) != 2 {
			t.Fatalf("broadcast receivers len = %d, want 2", len(got))
		}
	})
	if broadcastAllocs != 0 {
		t.Fatalf("broadcast Receivers allocs/op = %v, want 0", broadcastAllocs)
	}

	addressedAllocs := testing.AllocsPerRun(1000, func() {
		got := bus.Receivers(testPathEdge1IP, nsProto, etUpdate, events.DirReceived, nil, scratch[:0])
		if len(got) != 2 {
			t.Fatalf("addressed receivers len = %d, want 2", len(got))
		}
	})
	if addressedAllocs != 0 {
		t.Fatalf("addressed Receivers allocs/op = %v, want 0", addressedAllocs)
	}

	sendAllocs := testing.AllocsPerRun(1000, func() {
		ok, reason := bus.MaySend(testPathSender, testMsgUpdate, testPathEdge1IP)
		if !ok || reason != SendAllowed {
			t.Fatalf("MaySend = (%v, %s), want (true, allowed)", ok, reason)
		}
	})
	if sendAllocs != 0 {
		t.Fatalf("MaySend allocs/op = %v, want 0", sendAllocs)
	}

	tagFilteredAllocs := testing.AllocsPerRun(1000, func() {
		got := bus.Receivers(testPathEdge1, nsProto, etRC, events.DirUnspecified, msgTags, scratch[:0])
		if len(got) != 1 || got[0] != "tagged" {
			t.Fatalf("tag-filtered receivers = %v, want [tagged]", got)
		}
	})
	if tagFilteredAllocs != 0 {
		t.Fatalf("tag-filtered Receivers allocs/op = %v, want 0", tagFilteredAllocs)
	}
}

// BenchmarkBusDelivery measures throughput and allocations of broadcast
// delivery, addressed delivery, send checks, and interned tag-filtered delivery.
func BenchmarkBusDelivery(b *testing.B) {
	bus, _, nsProto, etUpdate, etRC, msgTags := buildAllocationTestBus(b)
	scratch := make([]string, 0, 8)

	b.Run("Broadcast", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = bus.Receivers(testPathEdge1, nsProto, etUpdate, events.DirReceived, nil, nil)
		}
	})

	b.Run("Addressed", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = bus.Receivers(testPathEdge1IP, nsProto, etUpdate, events.DirReceived, nil, scratch[:0])
		}
	})

	b.Run("SendCheck", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_, _ = bus.MaySend(testPathSender, testMsgUpdate, testPathEdge1IP)
		}
	})

	b.Run("TagFiltered", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = bus.Receivers(testPathEdge1, nsProto, etRC, events.DirUnspecified, msgTags, scratch[:0])
		}
	})
}
