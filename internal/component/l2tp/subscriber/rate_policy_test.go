// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact rate ownership
// VALIDATES: immutable rate inputs and local versus remote authorization owners.

package subscriber

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A rate command is an explicit declaration, not a backend-name convention.
// Exercise valid owners alongside malformed and missing declarations.
func TestRateOwnerCommandRequiresDeclaredRequest(t *testing.T) {
	for _, value := range []string{"request l2tp shaper rate", "request pppoe rate", "request example rate"} {
		owner, err := NewRateOwnerCommand(value)
		if err != nil {
			t.Fatal(err)
		}
		if owner.Command() != value {
			t.Fatalf("command = %q", owner.Command())
		}
	}
	for _, value := range []string{"", "show example rate", "request", "request  rate", "request rate\n", "request --rate", "request Rate", "request " + strings.Repeat("x", 256)} {
		if _, err := NewRateOwnerCommand(value); err == nil {
			t.Fatalf("accepted command %q", value)
		}
	}
}

// Both directions must be supplied by the policy boundary before a concrete
// backend can translate their units. Construction never invents an upload rate.
func TestRatesRequireBothPositiveDirections(t *testing.T) {
	for _, pair := range [][2]uint64{{0, 1000}, {1000, 0}, {0, 0}} {
		if _, err := NewRates(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
	rates, err := NewRates(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if rates.Download() != 8000 {
		t.Fatalf("download = %d", rates.Download())
	}
	if rates.Upload() != 16000 {
		t.Fatalf("upload = %d", rates.Upload())
	}
}

// The local variant borrows one exact controller and returns its transaction,
// while preserving both input rates and errors from that owner.
func TestLocalRatePolicyUsesExactController(t *testing.T) {
	want := errors.New("native preparation unavailable")
	controller := &rateControllerFixture{failure: want}
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewLocalRatePolicy(owner, controller)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := NewRates(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Prepare(t.Context(), rates); !errors.Is(err, want) {
		t.Fatalf("prepare error = %v", err)
	}
	if controller.rates != rates {
		t.Fatal("policy replaced its immutable rates")
	}
	controller.failure = nil
	transaction, err := policy.Prepare(t.Context(), rates)
	if err != nil {
		t.Fatal(err)
	}
	if transaction != controller.transaction {
		t.Fatal("policy replaced its owner's transaction")
	}
	if policy.Owner() != owner {
		t.Fatal("policy replaced its command declaration")
	}
}

// A controller can retain a prepared owner while reporting a failure. Preserve
// that capability so the caller can compensate rather than lose its ownership.
func TestLocalRatePolicyRetainsTransactionOnPreparationError(t *testing.T) {
	want := errors.New("preparation readback failed")
	controller := &rateControllerFixture{failure: want, transactionOnFailure: true}
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewLocalRatePolicy(owner, controller)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := NewRates(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := policy.Prepare(t.Context(), rates)
	if !errors.Is(err, want) {
		t.Fatalf("preparation error = %v", err)
	}
	if transaction != controller.transaction {
		t.Fatal("failed preparation lost its transaction owner")
	}
}

// Invalid or forged capabilities must fail at construction or before reaching
// a controller. Neither a nil result nor zero rates are usable authorization.
func TestLocalRatePolicyRefusesUnusableCapabilities(t *testing.T) {
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	var absent *rateControllerFixture
	for _, controller := range []RateController{nil, absent} {
		if _, err := NewLocalRatePolicy(owner, controller); err == nil {
			t.Fatal("accepted nil controller")
		}
	}
	controller := &rateControllerFixture{noTransaction: true}
	policy, err := NewLocalRatePolicy(owner, controller)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Prepare(t.Context(), nil); err == nil {
		t.Fatal("accepted missing rates")
	}
	if controller.calls != 0 {
		t.Fatal("invalid rates reached the native controller")
	}
	rates, err := NewRates(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Prepare(t.Context(), rates); err == nil {
		t.Fatal("accepted absent prepared transaction")
	}
	controller.noTransaction = false
	controller.typedNilTransaction = true
	if _, err := policy.Prepare(t.Context(), rates); err == nil {
		t.Fatal("accepted typed-nil prepared transaction")
	}
}

// Cancellation is an admission refusal before the borrowed controller gets a
// chance to reserve its transaction slot, even if that controller is idle.
func TestLocalRatePolicyCanceledPreparationNeverTouchesOwner(t *testing.T) {
	controller := &rateControllerFixture{}
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewLocalRatePolicy(owner, controller)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := NewRates(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := policy.Prepare(ctx, rates); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	if controller.calls != 0 {
		t.Fatal("canceled preparation reached the controller")
	}
}

// A remote policy carries a pure command declaration without manufacturing an
// unsupported local controller or passing a resource through a plugin boundary.
func TestRemoteRatePolicyContainsOnlyItsDeclaredOwner(t *testing.T) {
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewRemoteRatePolicy(owner)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Owner() != owner {
		t.Fatal("remote declaration changed")
	}
	if _, err := NewRemoteRatePolicy(nil); err == nil {
		t.Fatal("accepted missing remote owner")
	}
	if _, err := NewLocalRatePolicy(nil, &rateControllerFixture{}); err == nil {
		t.Fatal("accepted missing local owner")
	}
}

// Sealed values must survive the constructor boundary, not merely satisfy the
// interface by embedding one. An uninitialized variant is never a valid policy.
func TestRatePolicyValidationRefusesForgedAndZeroVariants(t *testing.T) {
	owner, err := NewRateOwnerCommand("request example rate")
	if err != nil {
		t.Fatal(err)
	}
	remote, err := NewRemoteRatePolicy(owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []RatePolicy{nil, &localRatePolicy{}, &remoteRatePolicy{}, &forgedRatePolicy{RatePolicy: remote}} {
		if err := ValidateRatePolicy(policy); err == nil {
			t.Fatalf("accepted policy %T", policy)
		}
	}
	if err := ValidateRatePolicy(remote); err != nil {
		t.Fatal(err)
	}
}

type forgedRatePolicy struct{ RatePolicy }

type rateControllerFixture struct {
	rates                Rates
	failure              error
	calls                int
	transaction          *rateTransactionFixture
	noTransaction        bool
	typedNilTransaction  bool
	transactionOnFailure bool
}

func (f *rateControllerFixture) Prepare(_ context.Context, rates Rates) (RateTransaction, error) {
	f.calls++
	f.rates = rates
	if f.failure != nil {
		if f.transactionOnFailure {
			f.transaction = &rateTransactionFixture{}
			return f.transaction, f.failure
		}
		return nil, f.failure
	}
	if f.noTransaction {
		return nil, nil //nolint:nilnil // Deliberately malformed controller result exercises refusal of an absent transaction.
	}
	if f.typedNilTransaction {
		var absent *rateTransactionFixture
		return absent, nil
	}
	f.transaction = &rateTransactionFixture{}
	return f.transaction, nil
}

type rateTransactionFixture struct{}

func (*rateTransactionFixture) Apply(context.Context) error    { return nil }
func (*rateTransactionFixture) Commit() error                  { return nil }
func (*rateTransactionFixture) Rollback(context.Context) error { return nil }
