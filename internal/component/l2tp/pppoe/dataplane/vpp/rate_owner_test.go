// Design: docs/architecture/l2tp/bng-5-pppoe.md -- bounded session policer ownership
// VALIDATES: two-direction updates, rollback and safe lifetime cleanup.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/policer"
)

// Initial installation owns the API-returned indices, including index zero,
// and applies one policer in each direction without stacking a feature.
func TestVPPPolicerOwnerInstallsBothDirections(t *testing.T) {
	owner, wire := rateOwnerFixture(t)
	change := ratePreparedFixture(t, owner, 8_000, 16_000)
	if err := change.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := change.Commit(); err != nil {
		t.Fatal(err)
	}
	if wire.next != 2 {
		t.Fatalf("created %d policers", wire.next)
	}
	if wire.bound != [2]uint32{0, 1} {
		t.Fatalf("directional indices = %v", wire.bound)
	}
	if wire.objects[0].config.Cir != 16 {
		t.Fatal("upload used the download rate")
	}
	if wire.objects[1].config.Cir != 8 {
		t.Fatal("download used the upload rate")
	}
	if !wire.inputEnabled {
		t.Fatal("patched input arcs remain disabled")
	}
	if owner.objects[0] == nil {
		t.Fatal("owner did not retain upload object")
	}
	if owner.objects[1] == nil {
		t.Fatal("owner did not retain download object")
	}
}

// The owner's single terminal slot supports retry of the current result, but
// preparing a newer authorization revokes the older transaction's authority.
func TestVPPPolicerOwnerRejectsSupersededTransaction(t *testing.T) {
	owner, wire := rateOwnerFixture(t)
	old := ratePreparedFixture(t, owner, 8_000, 16_000)
	if _, err := owner.Prepare(t.Context(), 24_000, 32_000, 1500, 1500); err == nil {
		t.Fatal("admitted a second pending transaction")
	}
	if err := old.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := old.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := old.Commit(); err != nil {
		t.Fatal(err)
	}
	current := ratePreparedFixture(t, owner, 24_000, 32_000)
	wire.calls = nil
	if err := old.Rollback(t.Context()); err == nil {
		t.Fatal("old transaction compensated newer authorization")
	}
	if len(wire.calls) != 0 {
		t.Fatalf("superseded transaction touched VPP: %v", wire.calls)
	}
	if err := current.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// A CoA changes the two existing indexed objects in place. No rebind, new
// allocation or unpoliced interval is needed to replace their rates.
func TestVPPPolicerOwnerUpdatesExistingIndices(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	wire.calls = nil
	change := ratePreparedFixture(t, owner, 24_000, 32_000)
	if err := change.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := change.Commit(); err != nil {
		t.Fatal(err)
	}
	if wire.next != 2 {
		t.Fatalf("replacement allocated %d objects", wire.next)
	}
	if wire.bound != [2]uint32{0, 1} {
		t.Fatalf("replacement moved indices = %v", wire.bound)
	}
	for _, call := range wire.calls {
		if strings.HasPrefix(call, "bind") {
			t.Fatalf("replacement rebound: %v", wire.calls)
		}
		if strings.HasPrefix(call, "delete") {
			t.Fatalf("replacement deleted: %v", wire.calls)
		}
	}
	if wire.objects[0].config.Cir != 32 {
		t.Fatal("upload replacement absent")
	}
	if wire.objects[1].config.Cir != 24 {
		t.Fatal("download replacement absent")
	}
}

// A lost second-direction update reply may follow an actual native write.
// Compensation restores both old plans before another transaction is admitted.
func TestVPPPolicerOwnerCompensatesPartialUpdate(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	want := errors.New("download update reply lost")
	wire.updateFailure = want
	wire.updateFailureIndex = 1
	change := ratePreparedFixture(t, owner, 24_000, 32_000)
	if err := change.Apply(t.Context()); !errors.Is(err, want) {
		t.Fatalf("apply error = %v", err)
	}
	if wire.objects[0].config.Cir != 16 {
		t.Fatal("upload was not restored")
	}
	if wire.objects[1].config.Cir != 8 {
		t.Fatal("download was not restored")
	}
	if owner.pending != nil {
		t.Fatal("successful compensation kept a pending transaction")
	}
	if err := change.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// A first-bind request with no reply may never have allocated VPP's slot
// vector. The owner quarantines the lifetime rather than issuing unsafe unbind.
func TestVPPPolicerOwnerUnknownFirstBindingQuarantines(t *testing.T) {
	owner, wire := rateOwnerFixture(t)
	wire.unknownInputBind = true
	change := ratePreparedFixture(t, owner, 8_000, 16_000)
	if err := change.Apply(t.Context()); err == nil {
		t.Fatal("unknown bind reported success")
	}
	wire.calls = nil
	if err := owner.Close(t.Context()); err == nil {
		t.Fatal("unknown bind reported released")
	}
	for _, call := range wire.calls {
		if call == "unbind/0" {
			t.Fatal("unknown first bind was blindly unbound")
		}
	}
	if owner.objects[0] == nil {
		t.Fatal("unknown binding owner was discarded")
	}
}

// An unacknowledged create retains its exact name without adopting an index
// from dump order. Other confirmed objects may still be released safely.
func TestVPPPolicerOwnerUnknownCreationKeepsName(t *testing.T) {
	owner, wire := rateOwnerFixture(t)
	wire.unknownCreate = true
	change := ratePreparedFixture(t, owner, 8_000, 16_000)
	if err := change.Apply(t.Context()); err == nil {
		t.Fatal("unknown create reported success")
	}
	if err := owner.Close(t.Context()); err == nil {
		t.Fatal("unknown create reported absent")
	}
	if owner.objects[0] == nil {
		t.Fatal("unknown creation reservation discarded")
	}
	if owner.objects[0].plan().name != "ze-pppoe/epoch/73/rx" {
		t.Fatalf("unknown name = %s", owner.objects[0].plan().name)
	}
	for _, call := range wire.calls {
		if strings.HasPrefix(call, "delete/") {
			t.Fatal("guessed a deletion index")
		}
	}
}

// All input and output features must be confirmed off before either binding
// is removed. One refused feature fence preserves both objects and their indices.
func TestVPPPolicerOwnerFailedFeatureFencePreservesObjects(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	wire.featureFailure = errors.New("input arc could not close")
	wire.calls = nil
	if err := owner.Close(t.Context()); err == nil {
		t.Fatal("feature failure reported clean release")
	}
	if len(wire.objects) != 2 {
		t.Fatal("feature failure recycled an object")
	}
	for _, call := range wire.calls {
		if strings.HasPrefix(call, "unbind/") {
			t.Fatalf("unbound before all features closed: %v", wire.calls)
		}
		if strings.HasPrefix(call, "delete/") {
			t.Fatalf("deleted before all features closed: %v", wire.calls)
		}
	}
}

// The cleanup sequence removes all seven feature arcs, then each directional
// binding, and only then its exact owned policer. It never deletes a bound slot.
func TestVPPPolicerOwnerCloseOrdersFeaturesBindingsAndObjects(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	wire.calls = nil
	if err := owner.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := []string{"features/off", "read/0", "unbind/0", "read/0", "delete/0", "read/1", "unbind/1", "read/1", "delete/1"}
	if !reflect.DeepEqual(wire.calls, want) {
		t.Fatalf("cleanup calls = %v, want %v", wire.calls, want)
	}
	if len(wire.objects) != 0 {
		t.Fatal("cleanup left policers")
	}
	if err := owner.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire.calls, want) {
		t.Fatal("completed close repeated native cleanup")
	}
}

// A delete whose reply is lost is complete only after indexed readback proves
// absence. The original transport error remains visible without retaining a ghost.
func TestVPPPolicerOwnerLostDeleteReplyRequiresAbsence(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	want := errors.New("delete reply lost")
	wire.deleteFailure = want
	if err := owner.Close(t.Context()); !errors.Is(err, want) {
		t.Fatalf("close error = %v", err)
	}
	if owner.objects[0] != nil {
		t.Fatal("confirmed absence retained an object")
	}
	if owner.objects[1] != nil {
		t.Fatal("confirmed absence retained an object")
	}
	if err := owner.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// An index reused by a foreign owner cannot be rebound, updated or deleted,
// even when the index is the one this lifetime originally received.
func TestVPPPolicerOwnerRefusesForeignIndexedObject(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	foreign := *wire.objects[0]
	foreign.name = "foreign"
	wire.objects[0] = &foreign
	if _, err := owner.Prepare(t.Context(), 24_000, 32_000, 1500, 1500); err == nil {
		t.Fatal("foreign indexed object accepted")
	}
	wire.calls = nil
	if err := owner.Close(t.Context()); err == nil {
		t.Fatal("foreign indexed object released")
	}
	for _, call := range wire.calls {
		if call == "delete/0" {
			t.Fatal("foreign object deleted")
		}
		if call == "unbind/0" {
			t.Fatal("foreign object used for an unbind")
		}
	}
}

// A departed session rejects a late transaction, so compensation cannot write
// a recycled session interface after Close has invalidated the lifetime.
func TestVPPPolicerOwnerCloseInvalidatesPendingTransaction(t *testing.T) {
	owner, wire := rateInstalledFixture(t)
	change := ratePreparedFixture(t, owner, 24_000, 32_000)
	if err := owner.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	wire.calls = nil
	if err := change.Apply(t.Context()); err == nil {
		t.Fatal("closed owner applied a pending change")
	}
	if err := change.Rollback(t.Context()); err == nil {
		t.Fatal("closed owner restored a departed session")
	}
	if len(wire.calls) != 0 {
		t.Fatalf("late transaction reached VPP: %v", wire.calls)
	}
}

func rateOwnerFixture(t *testing.T) (*policerRates, *rateWireFixture) {
	t.Helper()
	wire := &rateWireFixture{objects: make(map[uint32]*policerPlan), bound: [2]uint32{noInterface, noInterface}}
	owner, err := newPolicerRates(73, "ze-pppoe/epoch/73", wire)
	if err != nil {
		t.Fatal(err)
	}
	return owner, wire
}

func ratePreparedFixture(t *testing.T, owner *policerRates, download, upload uint64) *policerRateTransaction {
	t.Helper()
	change, err := owner.Prepare(t.Context(), download, upload, 1500, 1500)
	if err != nil {
		t.Fatal(err)
	}
	return change
}

func rateInstalledFixture(t *testing.T) (*policerRates, *rateWireFixture) {
	t.Helper()
	owner, wire := rateOwnerFixture(t)
	change := ratePreparedFixture(t, owner, 8_000, 16_000)
	if err := change.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := change.Commit(); err != nil {
		t.Fatal(err)
	}
	return owner, wire
}

type rateWireFixture struct {
	objects            map[uint32]*policerPlan
	next               uint32
	bound              [2]uint32
	inputEnabled       bool
	outputEnabled      bool
	calls              []string
	unknownCreate      bool
	unknownInputBind   bool
	updateFailure      error
	updateFailureIndex uint32
	featureFailure     error
	deleteFailure      error
}

func (w *rateWireFixture) createPolicer(_ context.Context, plan *policerPlan) (creation, error) {
	w.calls = append(w.calls, "create/"+plan.name)
	if w.unknownCreate {
		return creation{uncertain: true}, errors.New("create reply lost")
	}
	index := w.next
	w.next++
	w.objects[index] = plan
	return creation{index: index, confirmed: true}, nil
}

func (w *rateWireFixture) readPolicer(_ context.Context, index uint32, name string) (*policer.PolicerDetails, error) {
	w.calls = append(w.calls, fmt.Sprintf("read/%d", index))
	plan := w.objects[index]
	if plan == nil {
		return nil, nil
	}
	if plan.name != name {
		return nil, errors.New("indexed owner changed")
	}
	return &policer.PolicerDetails{Name: plan.name, Cir: plan.config.Cir, Eir: plan.config.Eir,
		Cb: plan.config.Cb, Eb: plan.config.Eb, RateType: plan.config.RateType,
		RoundType: plan.config.RoundType, Type: plan.config.Type, ColorAware: plan.config.ColorAware,
		ConformAction: plan.config.ConformAction, ExceedAction: plan.config.ExceedAction,
		ViolateAction: plan.config.ViolateAction}, nil
}

func (w *rateWireFixture) verifyPolicer(ctx context.Context, index uint32, plan *policerPlan) error {
	detail, err := w.readPolicer(ctx, index, plan.name)
	if err != nil {
		return err
	}
	if detail == nil {
		return errors.New("owned policer absent")
	}
	if *w.objects[index] != *plan {
		return errors.New("indexed configuration differs")
	}
	return nil
}

func (w *rateWireFixture) bindPolicerInput(_ context.Context, _ uint32, index uint32, apply bool) (policerReceipt, error) {
	if apply {
		w.calls = append(w.calls, fmt.Sprintf("bind/0/%d", index))
		if w.unknownInputBind {
			return policerUnknown, errors.New("input bind reply lost")
		}
		w.bound[0] = index
		return policerConfirmed, nil
	}
	w.calls = append(w.calls, fmt.Sprintf("unbind/%d", index))
	if w.inputEnabled {
		return policerRefused, errors.New("input features still reach policer")
	}
	w.bound[0] = noInterface
	return policerConfirmed, nil
}

func (w *rateWireFixture) bindPolicerOutput(_ context.Context, _ uint32, index uint32, apply bool) (policerReceipt, error) {
	if apply {
		w.calls = append(w.calls, fmt.Sprintf("bind/1/%d", index))
		w.bound[1] = index
		w.outputEnabled = true
		return policerConfirmed, nil
	}
	w.calls = append(w.calls, fmt.Sprintf("unbind/%d", index))
	if w.outputEnabled {
		return policerRefused, errors.New("output features still reach policer")
	}
	w.bound[1] = noInterface
	return policerConfirmed, nil
}

func (w *rateWireFixture) setPolicerInputFeatures(_ context.Context, _ uint32, enable bool) error {
	w.calls = append(w.calls, fmt.Sprintf("input-features/%t", enable))
	w.inputEnabled = enable
	return nil
}

func (w *rateWireFixture) quarantinePolicers(_ context.Context, _ uint32, input, output bool) error {
	w.calls = append(w.calls, "features/off")
	if w.featureFailure != nil {
		return w.featureFailure
	}
	if input {
		w.inputEnabled = false
	}
	if output {
		w.outputEnabled = false
	}
	return nil
}

func (w *rateWireFixture) updatePolicer(_ context.Context, index uint32, plan *policerPlan) (policerReceipt, error) {
	w.calls = append(w.calls, fmt.Sprintf("update/%d", index))
	w.objects[index] = plan
	if index == w.updateFailureIndex {
		if w.updateFailure != nil {
			err := w.updateFailure
			w.updateFailure = nil
			return policerUnknown, err
		}
	}
	return policerConfirmed, nil
}

func (w *rateWireFixture) deletePolicer(_ context.Context, index uint32) (policerReceipt, error) {
	w.calls = append(w.calls, fmt.Sprintf("delete/%d", index))
	if w.bound[0] == index {
		return policerRefused, errors.New("deleted input binding")
	}
	if w.bound[1] == index {
		return policerRefused, errors.New("deleted output binding")
	}
	delete(w.objects, index)
	if w.deleteFailure != nil {
		return policerUnknown, w.deleteFailure
	}
	return policerConfirmed, nil
}
