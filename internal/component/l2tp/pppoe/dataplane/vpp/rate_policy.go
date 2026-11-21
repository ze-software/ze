// Design: docs/architecture/l2tp/bng-5-pppoe.md -- borrowed session rate policy
// Related: rate_owner.go -- sole native cleanup owner; rate_transaction.go -- changes
package vpp

import (
	"context"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/traffic"
)

// newPolicerRatePolicy constructs the local capability and its native owner
// together. The successful session factory MUST retain native and close it
// before its session interface can be deleted; policy only borrows that owner.
// A failed construction has issued no native mutation or resource acquisition.
func newPolicerRatePolicy(command subscriber.RateOwnerCommand, index uint32, prefix string,
	client policerRateAPI) (policy subscriber.LocalRatePolicy, native *policerRates, err error) {
	native, err = newPolicerRates(index, prefix, client)
	if err != nil {
		return nil, nil, err
	}
	policy, err = subscriber.NewLocalRatePolicy(command, &sessionRateController{native: native})
	if err != nil {
		return nil, nil, err
	}
	return policy, native, nil
}

// sessionRateController is private so only the constructed LocalRatePolicy can
// invoke it. That policy validates the sealed Rates before calling Prepare.
// Its actual native owner serializes every operation and owns all cleanup.
type sessionRateController struct{ native *policerRates }

func (c *sessionRateController) Prepare(ctx context.Context, rates subscriber.Rates) (subscriber.RateTransaction, error) {
	change, err := c.native.Prepare(ctx, rates.Download(), rates.Upload(),
		traffic.PolicerBurstBytes(rates.Download()), traffic.PolicerBurstBytes(rates.Upload()))
	if err != nil {
		return nil, err
	}
	return change, nil
}
