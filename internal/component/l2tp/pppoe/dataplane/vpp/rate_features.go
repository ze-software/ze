// Design: docs/architecture/l2tp/bng-5-pppoe.md -- feature fence before policer removal
// Related: policer_api.go -- patched input arcs and raw mutation receipts.
package vpp

import (
	"context"
	"errors"
	"fmt"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"

	feature "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
)

// quarantinePolicers closes every arc a possibly bound directional slot can
// reach, before its caller unbinds or frees either object. A false direction
// means no binding request was issued for it, not an unavailable native read.
// Callers MUST retain unknown first-bind outcomes: the pinned unbind function
// writes its slot vector without first checking that it was ever allocated.
func (a *binaryAPI) quarantinePolicers(ctx context.Context, index uint32, input, output bool) error {
	var result error
	if input {
		result = errors.Join(result, a.setPolicerInputFeatures(ctx, index, false))
		result = errors.Join(result, a.setPolicerInputFeature(ctx, index, "device-input", false))
	}
	if output {
		for _, arc := range [...]string{"ip4-output", "ip6-output"} {
			result = errors.Join(result, a.closePolicerOutputFeature(ctx, index, arc))
		}
	}
	return result
}

func (a *binaryAPI) closePolicerOutputFeature(ctx context.Context, index uint32, arc string) error {
	current, err := a.ipGate(ctx, index, arc, "policer-output")
	if err != nil {
		return err
	}
	if !current {
		return nil
	}
	message, mutationErr := a.exchange(ctx, &feature.FeatureEnableDisable{
		SwIfIndex: interface_types.InterfaceIndex(index), ArcName: arc,
		FeatureName: "policer-output", Enable: false,
	})
	reply, ok := message.(*feature.FeatureEnableDisableReply)
	switch {
	case !ok:
		mutationErr = errors.Join(mutationErr, fmt.Errorf("pppoe-vpp: output feature reply %T", message))
	case reply == nil:
		mutationErr = errors.Join(mutationErr, errors.New("pppoe-vpp: nil output feature reply"))
	default:
		mutationErr = errors.Join(mutationErr, api.RetvalToVPPApiError(reply.Retval))
	}
	current, readErr := a.ipGate(ctx, index, arc, "policer-output")
	if readErr == nil {
		if current {
			readErr = fmt.Errorf("pppoe-vpp: %s policer feature remains enabled", arc)
		}
	}
	return errors.Join(mutationErr, readErr)
}
