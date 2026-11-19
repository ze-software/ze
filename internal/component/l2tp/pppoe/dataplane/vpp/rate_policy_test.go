// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session-bound rate policy
// VALIDATES: generic authorization reaches its captured native policer owner.
package vpp

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/traffic"
)

// The generic policy reaches the same native owner and uses the existing
// burst derivation in both directions, rather than introducing another formula.
func TestVPPPolicerRatePolicyUsesExistingBurstPolicy(t *testing.T) {
	_, wire := rateOwnerFixture(t)
	command, err := subscriber.NewRateOwnerCommand("request example rate")
	if err != nil { t.Fatal(err) }
	policy, native, err := newPolicerRatePolicy(command, 73, "ze-pppoe/epoch/73", wire)
	if err != nil { t.Fatal(err) }
	rates, err := subscriber.NewRates(8_000_000, 16_000_000)
	if err != nil { t.Fatal(err) }
	prepared, err := policy.Prepare(t.Context(), rates)
	if err != nil { t.Fatal(err) }
	change, valid := prepared.(*policerRateTransaction)
	if !valid { t.Fatalf("transaction = %T", prepared) }
	if change.owner != native { t.Fatal("policy created another native owner") }
	if change.after[0].config.Cir != 16_000 { t.Fatal("upload rate changed") }
	if change.after[1].config.Cir != 8_000 { t.Fatal("download rate changed") }
	if change.after[0].config.Cb != traffic.PolicerBurstBytes(rates.Upload()) { t.Fatal("upload burst diverged from shared policy") }
	if change.after[1].config.Cb != traffic.PolicerBurstBytes(rates.Download()) { t.Fatal("download burst diverged from shared policy") }
	if policy.Owner() != command { t.Fatal("policy changed its declared RPC owner") }
}

// A valid generic rate that native VPP cannot express is refused before any
// transaction or mutation, preserving exact-or-reject at the backend boundary.
func TestVPPPolicerRatePolicyRefusesNativeUnitApproximation(t *testing.T) {
	_, wire := rateOwnerFixture(t)
	command, err := subscriber.NewRateOwnerCommand("request example rate")
	if err != nil { t.Fatal(err) }
	policy, native, err := newPolicerRatePolicy(command, 73, "ze-pppoe/epoch/73", wire)
	if err != nil { t.Fatal(err) }
	rates, err := subscriber.NewRates(1001, 2000)
	if err != nil { t.Fatal(err) }
	if _, err := policy.Prepare(t.Context(), rates); err == nil { t.Fatal("unrepresentable rate was silently rounded") }
	if native.pending != nil { t.Fatal("refused rate reserved a pending transaction") }
	if len(wire.calls) != 0 { t.Fatalf("refused rate reached VPP: %v", wire.calls) }
}

// Only the native session owner closes the policers. A borrowed policy cannot
// resurrect that lifetime after close, and repeated owner close stays harmless.
func TestVPPPolicerRatePolicyCannotOutliveItsSessionOwner(t *testing.T) {
	_, wire := rateOwnerFixture(t)
	command, err := subscriber.NewRateOwnerCommand("request example rate")
	if err != nil { t.Fatal(err) }
	policy, native, err := newPolicerRatePolicy(command, 73, "ze-pppoe/epoch/73", wire)
	if err != nil { t.Fatal(err) }
	rates, err := subscriber.NewRates(8000, 16000)
	if err != nil { t.Fatal(err) }
	change, err := policy.Prepare(t.Context(), rates)
	if err != nil { t.Fatal(err) }
	if err := change.Apply(t.Context()); err != nil { t.Fatal(err) }
	if err := change.Commit(); err != nil { t.Fatal(err) }
	if err := native.Close(t.Context()); err != nil { t.Fatal(err) }
	wire.calls = nil
	if _, err := policy.Prepare(t.Context(), rates); err == nil { t.Fatal("borrowed policy reopened a closed session") }
	if err := native.Close(t.Context()); err != nil { t.Fatal(err) }
	if len(wire.calls) != 0 { t.Fatalf("closed policy touched VPP: %v", wire.calls) }
}
