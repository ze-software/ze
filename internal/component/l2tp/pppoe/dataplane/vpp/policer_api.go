// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session-owned VPP policing
// Related: api.go -- generation-bound exchanges and bounded dumps
// API: https://github.com/FDio/vpp, src/vnet/policer/policer.api,
// policer.c and xlate.c at cbba0451; GoVPP v0.13.0 binapi/policer.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"
	"go.fd.io/govpp/binapi/policer_types"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
)

// policerPlan is immutable after validation. Its nil and zero values are not
// plans; only newPolicerPlan constructs values passed to the API methods.
// Burst is explicit: the caller MUST supply its authorized burst in bytes,
// rather than have a rate-to-burst heuristic change the authorization silently.
type policerPlan struct {
	name   string
	config policer_types.PolicerConfig
}

func newPolicerPlan(name string, bitsPerSecond, burstBytes uint64) (*policerPlan, error) {
	if name == "" {
		return nil, errors.New("pppoe-vpp: policer name is empty")
	}
	// Both API name copying and PolicerDetails reserve the final byte for NUL.
	if len(name) > 63 {
		return nil, errors.New("pppoe-vpp: policer name exceeds 63 bytes")
	}
	if strings.ContainsRune(name, '\x00') {
		return nil, errors.New("pppoe-vpp: policer name contains NUL")
	}
	if bitsPerSecond == 0 {
		return nil, errors.New("pppoe-vpp: policer rate must be positive")
	}
	if bitsPerSecond%1000 != 0 {
		return nil, errors.New("pppoe-vpp: policer rate must be a whole number of kilobits per second")
	}
	if bitsPerSecond/1000 > uint64(^uint32(0)) {
		return nil, errors.New("pppoe-vpp: policer rate exceeds the VPP kilobit rate limit")
	}
	if burstBytes == 0 {
		return nil, errors.New("pppoe-vpp: policer burst must be positive")
	}
	// x86_pol_compute_hw_params caps bursts to u32 even though the API is u64.
	if burstBytes > uint64(^uint32(0)) {
		return nil, errors.New("pppoe-vpp: policer burst exceeds the VPP physical bucket limit")
	}
	return &policerPlan{name: name, config: policer_types.PolicerConfig{
		Cir: uint32(bitsPerSecond / 1000), Cb: burstBytes,
		RateType:      policer_types.SSE2_QOS_RATE_API_KBPS,
		RoundType:     policer_types.SSE2_QOS_ROUND_API_TO_DOWN,
		Type:          policer_types.SSE2_QOS_POLICER_TYPE_API_1R2C,
		ConformAction: policer_types.Sse2QosAction{Type: policer_types.SSE2_QOS_ACTION_API_TRANSMIT},
		ExceedAction:  policer_types.Sse2QosAction{Type: policer_types.SSE2_QOS_ACTION_API_DROP},
		ViolateAction: policer_types.Sse2QosAction{Type: policer_types.SSE2_QOS_ACTION_API_DROP},
	}}, nil
}

// createPolicer returns a raw mutation receipt, including uncertainty. Index
// zero is a valid policer pool index. The caller MUST retain the plan and the
// receipt even on error until rollback or reconciliation proves removal.
func (a *binaryAPI) createPolicer(ctx context.Context, plan *policerPlan) (creation, error) {
	message, err := a.exchange(ctx, &policer.PolicerAdd{Name: plan.name, Infos: plan.config})
	if requestWasNotSent(err) {
		return creation{}, err
	}
	reply, ok := message.(*policer.PolicerAddReply)
	if !ok {
		return creation{uncertain: true}, errors.Join(err, fmt.Errorf("pppoe-vpp: policer create reply %T", message))
	}
	if reply == nil {
		return creation{uncertain: true}, errors.Join(err, errors.New("pppoe-vpp: nil policer create reply"))
	}
	if reply.Retval != 0 {
		return creation{}, errors.Join(err, api.RetvalToVPPApiError(reply.Retval))
	}
	receipt := creation{index: reply.PolicerIndex, confirmed: true}
	if reply.PolicerIndex == noInterface {
		return receipt, errors.Join(err, errors.New("pppoe-vpp: created policer index is invalid"))
	}
	return receipt, err
}

// verifyPolicer checks the indexed owner and logical configuration, not merely
// that a dump succeeded. Physical token quantization depends on VPP's TSC;
// logical readback is not evidence of an exact physical token refill rate.
func (a *binaryAPI) verifyPolicer(ctx context.Context, index uint32, plan *policerPlan) error {
	reply, err := a.readPolicer(ctx, index, plan.name)
	if err != nil {
		return err
	}
	if reply == nil {
		return errors.New("pppoe-vpp: owned policer absent from indexed readback")
	}
	actual := policer_types.PolicerConfig{
		Cir: reply.Cir, Eir: reply.Eir, Cb: reply.Cb, Eb: reply.Eb,
		RateType: reply.RateType, RoundType: reply.RoundType, Type: reply.Type,
		ColorAware: reply.ColorAware, ConformAction: reply.ConformAction,
		ExceedAction: reply.ExceedAction, ViolateAction: reply.ViolateAction,
	}
	if actual != plan.config {
		return errors.New("pppoe-vpp: policer configuration differs from the requested rate")
	}
	return nil
}

// readPolicer returns nil only when a completed indexed dump proves absence.
// A foreign name, failed read or failed stream close is an error, so cleanup
// cannot treat an unavailable read as evidence that deletion succeeded.
func (a *binaryAPI) readPolicer(ctx context.Context, index uint32, name string) (*policer.PolicerDetails, error) {
	var found *policer.PolicerDetails
	err := a.dump(ctx, &policer.PolicerDumpV2{PolicerIndex: index}, 1, func(message api.Message) error {
		reply, ok := message.(*policer.PolicerDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: policer readback %T", message)
		}
		if reply == nil {
			return errors.New("pppoe-vpp: nil policer readback")
		}
		if found != nil {
			return errors.New("pppoe-vpp: duplicate indexed policer readback")
		}
		if strings.TrimRight(reply.Name, "\x00") != name {
			return errors.New("pppoe-vpp: policer index belongs to another owner")
		}
		found = reply
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// setPolicerInputFeatures reconciles the four arcs the PPPoE decapsulation
// patch adds. Callers MUST confirm all four disabled before unbinding or
// deleting an input policer; device-input alone cannot guard a PPPoE session.
// It attempts all four bounded arcs, including after one failed mutation.
func (a *binaryAPI) setPolicerInputFeatures(ctx context.Context, index uint32, enable bool) error {
	var result error
	for _, arc := range [...]string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast"} {
		result = errors.Join(result, a.setPolicerInputFeature(ctx, index, arc, enable))
	}
	return result
}

func (a *binaryAPI) setPolicerInputFeature(ctx context.Context, index uint32, arc string, enable bool) error {
	current, err := a.ipGate(ctx, index, arc, "policer-input")
	if err != nil {
		return err
	}
	if current == enable {
		return nil
	}
	message, mutationErr := a.exchange(ctx, &feature.FeatureEnableDisable{SwIfIndex: interface_types.InterfaceIndex(index), ArcName: arc, FeatureName: "policer-input", Enable: enable})
	reply, ok := message.(*feature.FeatureEnableDisableReply)
	switch {
	case !ok:
		mutationErr = errors.Join(mutationErr, fmt.Errorf("pppoe-vpp: policer feature reply %T", message))
	case reply == nil:
		mutationErr = errors.Join(mutationErr, errors.New("pppoe-vpp: nil policer feature reply"))
	default:
		mutationErr = errors.Join(mutationErr, api.RetvalToVPPApiError(reply.Retval))
	}
	current, readErr := a.ipGate(ctx, index, arc, "policer-input")
	if readErr == nil {
		if current != enable {
			readErr = fmt.Errorf("pppoe-vpp: %s policer feature did not reach requested state", arc)
		}
	}
	return errors.Join(mutationErr, readErr)
}

// policerReceipt describes only the raw native mutation outcome. Its zero is
// unknown, never refusal or success. An owner MUST retain its exact operation
// on unknown and MUST NOT retry an enable that can stack feature references.
type policerReceipt uint8

const (
	policerUnknown policerReceipt = iota
	policerRefused
	policerConfirmed
)

// bindPolicerInput changes the per-interface RX slot. Before apply the caller
// MUST verify indexed ownership, then enable the four patched arcs afterward.
// Before removal the caller MUST confirm setPolicerInputFeatures(false)
// succeeded on all four arcs. This primitive never deletes a policer.
func (a *binaryAPI) bindPolicerInput(ctx context.Context, index, policerIndex uint32, apply bool) (policerReceipt, error) {
	message, err := a.exchange(ctx, &policer.PolicerInputV2{SwIfIndex: interface_types.InterfaceIndex(index), PolicerIndex: policerIndex, Apply: apply})
	if requestWasNotSent(err) {
		return policerRefused, err
	}
	reply, ok := message.(*policer.PolicerInputV2Reply)
	if !ok {
		return policerUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: input binding reply %T", message))
	}
	if reply == nil {
		return policerUnknown, errors.Join(err, errors.New("pppoe-vpp: nil input binding reply"))
	}
	return policerMutationReceipt(reply.Retval, err)
}

// bindPolicerOutput changes the TX slot and both output features. The caller
// MUST verify indexed ownership before a write, and retain an unknown write
// instead of repeating an enable. VPP has one TX slot on this interface.
func (a *binaryAPI) bindPolicerOutput(ctx context.Context, index, policerIndex uint32, apply bool) (policerReceipt, error) {
	message, err := a.exchange(ctx, &policer.PolicerOutputV2{SwIfIndex: interface_types.InterfaceIndex(index), PolicerIndex: policerIndex, Apply: apply})
	if requestWasNotSent(err) {
		return policerRefused, err
	}
	// The pinned policer_api.c v2 handler replies using the old
	// VL_API_POLICER_OUTPUT_REPLY ID, not POLICER_OUTPUT_V2_REPLY.
	reply, ok := message.(*policer.PolicerOutputReply)
	if !ok {
		return policerUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: output binding reply %T", message))
	}
	if reply == nil {
		return policerUnknown, errors.Join(err, errors.New("pppoe-vpp: nil output binding reply"))
	}
	return policerMutationReceipt(reply.Retval, err)
}

// updatePolicer changes an indexed object in place. The caller MUST verify its
// indexed name first, keep the previous plan through compensation and confirm
// the requested logical configuration with verifyPolicer after the mutation.
func (a *binaryAPI) updatePolicer(ctx context.Context, index uint32, plan *policerPlan) (policerReceipt, error) {
	message, err := a.exchange(ctx, &policer.PolicerUpdate{PolicerIndex: index, Infos: plan.config})
	if requestWasNotSent(err) {
		return policerRefused, err
	}
	reply, ok := message.(*policer.PolicerUpdateReply)
	if !ok {
		return policerUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: policer update reply %T", message))
	}
	if reply == nil {
		return policerUnknown, errors.Join(err, errors.New("pppoe-vpp: nil policer update reply"))
	}
	return policerMutationReceipt(reply.Retval, err)
}

// deletePolicer consumes a previously owned index, never an index inferred
// from name-dump order. The caller MUST confirm input features disabled and
// both directional bindings removed before deletion, and MUST retain an
// unknown deletion until an indexed readback proves absence or ownership.
func (a *binaryAPI) deletePolicer(ctx context.Context, index uint32) (policerReceipt, error) {
	message, err := a.exchange(ctx, &policer.PolicerDel{PolicerIndex: index})
	if requestWasNotSent(err) {
		return policerRefused, err
	}
	reply, ok := message.(*policer.PolicerDelReply)
	if !ok {
		return policerUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: policer delete reply %T", message))
	}
	if reply == nil {
		return policerUnknown, errors.Join(err, errors.New("pppoe-vpp: nil policer delete reply"))
	}
	return policerMutationReceipt(reply.Retval, err)
}

func policerMutationReceipt(retval int32, err error) (policerReceipt, error) {
	if retval != 0 {
		return policerRefused, errors.Join(err, api.RetvalToVPPApiError(retval))
	}
	return policerConfirmed, err
}
