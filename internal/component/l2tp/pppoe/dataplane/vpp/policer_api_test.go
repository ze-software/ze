// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session-owned VPP policing
// VALIDATES: exact API units, retained mutation receipts and four-arc ordering.
package vpp

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/memclnt"
	"go.fd.io/govpp/binapi/policer"
	"go.fd.io/govpp/binapi/policer_types"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
)

// The API takes whole kilobits per second and a 32-bit physical burst. Reject
// truncation before opening a stream, while preserving exact boundary values.
func TestVPPPolicerPlanExactUnits(t *testing.T) {
	plan, err := newPolicerPlan("ze-pppoe/73/upload", 80_000_000, 125_000)
	if err != nil {
		t.Fatal(err)
	}
	if plan.config.Cir != 80_000 || plan.config.Cb != 125_000 {
		t.Fatalf("conversion %+v", plan.config)
	}
	if plan.config.Type != policer_types.SSE2_QOS_POLICER_TYPE_API_1R2C || plan.config.ColorAware {
		t.Fatalf("policer mode %+v", plan.config)
	}
	if plan.config.ConformAction.Type != policer_types.SSE2_QOS_ACTION_API_TRANSMIT || plan.config.ExceedAction.Type != policer_types.SSE2_QOS_ACTION_API_DROP || plan.config.ViolateAction.Type != policer_types.SSE2_QOS_ACTION_API_DROP {
		t.Fatalf("policer actions %+v", plan.config)
	}
	for _, tc := range []struct {
		name        string
		rate, burst uint64
	}{
		{"", 1000, 1}, {strings.Repeat("x", 64), 1000, 1}, {"a\x00b", 1000, 1},
		{"owned", 0, 1}, {"owned", 1001, 1}, {"owned", uint64(^uint32(0))*1000 + 1000, 1},
		{"owned", 1000, 0}, {"owned", 1000, uint64(^uint32(0)) + 1},
	} {
		if _, err := newPolicerPlan(tc.name, tc.rate, tc.burst); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := newPolicerPlan("owned", uint64(^uint32(0))*1000, uint64(^uint32(0))); err != nil {
		t.Fatal(err)
	}
}

// Successful mutation receipts survive a stream-close error; a lost or wrong
// reply leaves uncertainty, while a native refusal does not invent ownership.
func TestVPPPolicerCreationRetainsReceipt(t *testing.T) {
	plan, err := newPolicerPlan("ze-pppoe/73/upload", 1000, 1500)
	if err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("close failed")
	for _, tc := range []struct {
		name                 string
		reply                api.Message
		receiveErr           error
		confirmed, uncertain bool
	}{
		{"confirmed", &policer.PolicerAddReply{PolicerIndex: 0}, nil, true, false},
		{"refused", &policer.PolicerAddReply{Retval: -1}, nil, false, false},
		{"lost", nil, errors.New("lost reply"), false, true},
		{"wrong", &policer.PolicerDelReply{}, nil, false, true},
		{"nil", (*policer.PolicerAddReply)(nil), nil, false, true},
		{"invalid-index", &policer.PolicerAddReply{PolicerIndex: noInterface}, nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := &testStream{replies: []api.Message{tc.reply}, err: tc.receiveErr, closeErr: closeErr}
			client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
			receipt, err := client.createPolicer(context.Background(), plan)
			if !errors.Is(err, closeErr) {
				t.Fatalf("lost close error: %v", err)
			}
			if receipt.confirmed != tc.confirmed || receipt.uncertain != tc.uncertain {
				t.Fatalf("receipt %+v", receipt)
			}
			request, ok := stream.sent[0].(*policer.PolicerAdd)
			if !ok {
				t.Fatalf("request %T", stream.sent[0])
			}
			if request.Name != plan.name || request.Infos != plan.config {
				t.Fatalf("request %+v", request)
			}
		})
	}
}

// Indexed readback must identify the exact owner and requested configuration;
// absence, foreign ownership, stale config and incomplete dumps all refuse.
func TestVPPPolicerReadbackChecksOwnership(t *testing.T) {
	plan, err := newPolicerPlan("ze-pppoe/73/upload", 8000, 1500)
	if err != nil {
		t.Fatal(err)
	}
	detail := &policer.PolicerDetails{Name: plan.name, Cir: plan.config.Cir, Cb: plan.config.Cb,
		RateType: plan.config.RateType, RoundType: plan.config.RoundType, Type: plan.config.Type,
		ConformAction: plan.config.ConformAction, ExceedAction: plan.config.ExceedAction, ViolateAction: plan.config.ViolateAction}
	foreign := *detail
	foreign.Name = "someone-else"
	stale := *detail
	stale.Cir++
	for _, tc := range []struct {
		name    string
		replies []api.Message
		wantErr bool
	}{
		{"owned", []api.Message{detail, &memclnt.ControlPingReply{}}, false},
		{"absent", []api.Message{&memclnt.ControlPingReply{}}, true},
		{"foreign", []api.Message{&foreign, &memclnt.ControlPingReply{}}, true},
		{"stale", []api.Message{&stale, &memclnt.ControlPingReply{}}, true},
		{"duplicate", []api.Message{detail, detail, &memclnt.ControlPingReply{}}, true},
		{"incomplete", []api.Message{detail}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := &testStream{replies: tc.replies}
			client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
			err := client.verifyPolicer(context.Background(), 71, plan)
			if (err != nil) != tc.wantErr {
				t.Fatalf("readback %v", err)
			}
			request, ok := stream.sent[0].(*policer.PolicerDumpV2)
			if !ok {
				t.Fatalf("request %T", stream.sent[0])
			}
			if request.PolicerIndex != 71 {
				t.Fatalf("readback index %d", request.PolicerIndex)
			}
		})
	}
}

// Cleanup can release an uncertain deletion only after a completed successful
// dump proves absence; a failed completion remains unknown.
func TestVPPPolicerAbsenceRequiresCompletedRead(t *testing.T) {
	for _, tc := range []struct {
		name     string
		replies  []api.Message
		closeErr error
		wantErr  bool
	}{
		{"absent", []api.Message{&memclnt.ControlPingReply{}}, nil, false},
		{"lost", nil, nil, true},
		{"close-failed", []api.Message{&memclnt.ControlPingReply{}}, errors.New("close failed"), true},
		{"refused-completion", []api.Message{&memclnt.ControlPingReply{Retval: -1}}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := &testStream{replies: tc.replies, closeErr: tc.closeErr}
			client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
			found, err := client.readPolicer(context.Background(), 71, "owned")
			if found != nil {
				t.Fatalf("invented owner %+v", found)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("absence error %v", err)
			}
		})
	}
}

type policerFeatureStream struct {
	*testStream
	enabled map[string]bool
	calls   *[]string
	failArc string
	lostArc string
}

func (s *policerFeatureStream) SendMsg(message api.Message) error {
	switch request := message.(type) {
	case *feature.FeatureIsEnabled:
		s.replies = []api.Message{&feature.FeatureIsEnabledReply{IsEnabled: s.enabled[request.ArcName]}}
	case *feature.FeatureEnableDisable:
		*s.calls = append(*s.calls, request.ArcName)
		if request.ArcName == s.failArc {
			s.replies = []api.Message{&feature.FeatureEnableDisableReply{Retval: -1}}
			break
		}
		s.enabled[request.ArcName] = request.Enable
		s.replies = []api.Message{&feature.FeatureEnableDisableReply{}}
		if request.ArcName == s.lostArc {
			return errors.New("mutation receipt lost")
		}
	default:
		return errors.New("unexpected request")
	}
	return s.testStream.SendMsg(message)
}

// A lost feature receipt remains an error even when readback proves the write;
// a subsequent cleanup retries the read, without duplicating the native toggle.
func TestVPPPolicerInputFeatureLostReceipt(t *testing.T) {
	enabled := map[string]bool{}
	var calls []string
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		return &policerFeatureStream{testStream: &testStream{ctx: ctx}, enabled: enabled, calls: &calls, lostArc: "ip4-unicast"}, nil
	}}
	if err := client.setPolicerInputFeatures(context.Background(), 73, true); err == nil {
		t.Fatal("lost receipt reported success")
	}
	if len(calls) != 4 {
		t.Fatalf("only attempted %v", calls)
	}
	calls = nil
	if err := client.setPolicerInputFeatures(context.Background(), 73, true); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("stacked toggles %v", calls)
	}
}

// Reconcile all four patched arcs by read/mutate/read. Repeating the desired
// state must not stack features, and one refusal must not skip other arcs.
func TestVPPPolicerInputFeaturesReconcileFourArcs(t *testing.T) {
	arcs := []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast"}
	for _, fail := range []string{"", "ip4-multicast"} {
		t.Run("failure="+fail, func(t *testing.T) {
			enabled := map[string]bool{}
			for _, arc := range arcs {
				enabled[arc] = true
			}
			var calls []string
			client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
				return &policerFeatureStream{testStream: &testStream{ctx: ctx}, enabled: enabled, calls: &calls, failArc: fail}, nil
			}}
			err := client.setPolicerInputFeatures(context.Background(), 73, false)
			if (err != nil) != (fail != "") {
				t.Fatalf("disable error %v", err)
			}
			if !reflect.DeepEqual(calls, arcs) {
				t.Fatalf("arcs %v", calls)
			}
			calls = nil
			err = client.setPolicerInputFeatures(context.Background(), 73, false)
			if (err != nil) != (fail != "") {
				t.Fatalf("retry error %v", err)
			}
			if fail == "" && len(calls) != 0 {
				t.Fatalf("duplicate mutations %v", calls)
			}
			if fail != "" && !reflect.DeepEqual(calls, []string{fail}) {
				t.Fatalf("retry mutations %v", calls)
			}
		})
	}
}

// Every mutation distinguishes a confirmed native write, a native refusal and
// an unknown reply. Cleanup errors must not erase a successful native receipt.
func TestVPPPolicerMutationReceipts(t *testing.T) {
	closeErr := errors.New("close failed")
	plan, err := newPolicerPlan("ze-pppoe/73/upload", 8000, 1500)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []struct {
		name      string
		run       func(*binaryAPI) (policerReceipt, error)
		confirmed api.Message
		refused   api.Message
		nilReply  api.Message
	}{
		{"input", func(a *binaryAPI) (policerReceipt, error) {
			return a.bindPolicerInput(context.Background(), 73, 9, false)
		}, &policer.PolicerInputV2Reply{}, &policer.PolicerInputV2Reply{Retval: -1}, (*policer.PolicerInputV2Reply)(nil)},
		{"output", func(a *binaryAPI) (policerReceipt, error) {
			return a.bindPolicerOutput(context.Background(), 73, 9, false)
		}, &policer.PolicerOutputReply{}, &policer.PolicerOutputReply{Retval: -1}, (*policer.PolicerOutputReply)(nil)},
		{"delete", func(a *binaryAPI) (policerReceipt, error) { return a.deletePolicer(context.Background(), 9) }, &policer.PolicerDelReply{}, &policer.PolicerDelReply{Retval: -1}, (*policer.PolicerDelReply)(nil)},
		{"update", func(a *binaryAPI) (policerReceipt, error) { return a.updatePolicer(context.Background(), 9, plan) }, &policer.PolicerUpdateReply{}, &policer.PolicerUpdateReply{Retval: -1}, (*policer.PolicerUpdateReply)(nil)},
	} {
		for _, tc := range []struct {
			name    string
			reply   api.Message
			receipt policerReceipt
		}{
			{"confirmed", operation.confirmed, policerConfirmed}, {"refused", operation.refused, policerRefused}, {"unknown", &memclnt.ControlPingReply{}, policerUnknown},
			{"nil", operation.nilReply, policerUnknown}, {"lost", nil, policerUnknown},
		} {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				stream := &testStream{replies: []api.Message{tc.reply}, closeErr: closeErr}
				client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
				receipt, err := operation.run(&client)
				if receipt != tc.receipt {
					t.Fatalf("receipt %v, want %v", receipt, tc.receipt)
				}
				if !errors.Is(err, closeErr) {
					t.Fatalf("lost close failure: %v", err)
				}
				switch request := stream.sent[0].(type) {
				case *policer.PolicerInputV2:
					if request.SwIfIndex != 73 || request.PolicerIndex != 9 || request.Apply {
						t.Fatalf("input %+v", request)
					}
				case *policer.PolicerOutputV2:
					if request.SwIfIndex != 73 || request.PolicerIndex != 9 || request.Apply {
						t.Fatalf("output %+v", request)
					}
				case *policer.PolicerDel:
					if request.PolicerIndex != 9 {
						t.Fatalf("delete %+v", request)
					}
				case *policer.PolicerUpdate:
					if request.PolicerIndex != 9 || request.Infos != plan.config {
						t.Fatalf("update %+v", request)
					}
				default:
					t.Fatalf("request %T", request)
				}
			})
		}
	}
}
