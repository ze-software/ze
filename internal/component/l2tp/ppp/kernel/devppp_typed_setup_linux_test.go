// Design: docs/architecture/l2tp/bng-5-pppoe.md -- typed setup ownership and rollback

//go:build linux

package kernel

import (
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The private seam proves orchestration only. These logical owners never claim
// native PPP creation, carry a fake descriptor or enter NewSession.
type typedSetupFixture struct {
	channel   *CreatedChannel
	transport *os.File
	unit      *CreatedUnit
	creator   *UnitCreator
	ops       devPPPSetupOps
	order     []string
}

func newTypedSetupFixture() *typedSetupFixture {
	fixture := &typedSetupFixture{channel: &CreatedChannel{}, unit: &CreatedUnit{}, transport: new(os.File)}
	fixture.creator = &UnitCreator{create: func(name string) (*CreatedUnit, error) {
		fixture.order = append(fixture.order, "create:"+name)
		return fixture.unit, nil
	}}
	fixture.ops = devPPPSetupOps{
		createUnit: func(creator *UnitCreator, channel *CreatedChannel, name string) (*CreatedUnit, error) {
			if channel != fixture.channel {
				return nil, errors.New("setup changed the original logical channel")
			}
			return creator.Create(name)
		},
		channel: func(file *os.File) (*CreatedChannel, error) {
			if file != fixture.transport {
				return nil, errors.New("setup changed the original PPPoX file")
			}
			fixture.order = append(fixture.order, "attach")
			return fixture.channel, nil
		},
		closeUnit: func(unit *CreatedUnit) error {
			if unit != fixture.unit {
				return errors.New("setup closed a foreign unit")
			}
			fixture.order = append(fixture.order, "close-unit")
			return nil
		},
		closeChannel: func(channel *CreatedChannel) error {
			if channel != fixture.channel {
				return errors.New("setup closed a foreign channel")
			}
			fixture.order = append(fixture.order, "close-channel")
			return nil
		},
		pause:    func(time.Duration) { fixture.order = append(fixture.order, "pause") },
		rejected: func(*CreatedUnit) bool { return false },
	}
	return fixture
}

// A missing or zero creator refuses before channel attachment or unit creation.
func TestDevPPPSetupTypedRequiresCreatorBeforeAllocation(t *testing.T) {
	for name, creator := range map[string]*UnitCreator{"nil": nil, "zero": {}} {
		t.Run(name, func(t *testing.T) {
			fixture := newTypedSetupFixture()
			channel, unit, err := devPPPSetup(creator, fixture.transport, "zc-setup", fixture.ops)
			if err == nil {
				t.Fatal("missing creator admitted native setup")
			}
			if channel != nil {
				t.Fatal("missing creator allocated a channel")
			}
			if unit != nil {
				t.Fatal("missing creator allocated a unit")
			}
			if len(fixture.order) != 0 {
				t.Fatalf("refused creator performed work: %v", fixture.order)
			}
		})
	}
}

// A named first attempt returns exactly the original owners without closing.
func TestDevPPPSetupTypedNamedFirstAttempt(t *testing.T) {
	fixture := newTypedSetupFixture()
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	if channel != fixture.channel {
		t.Fatal("successful setup replaced its original channel")
	}
	if unit != fixture.unit {
		t.Fatal("successful setup replaced its original unit")
	}
	if want := []string{"attach", "create:zc-setup"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("setup order %v, want %v", fixture.order, want)
	}
}

// An omitted name reaches the same creator; setup does not invent a pppN name.
func TestDevPPPSetupTypedUnnamedUsesCapturedCreator(t *testing.T) {
	fixture := newTypedSetupFixture()
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "", fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	if channel != fixture.channel {
		t.Fatal("unnamed setup replaced its original channel")
	}
	if unit != fixture.unit {
		t.Fatal("unnamed setup replaced its original unit")
	}
	if want := []string{"attach", "create:"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("unnamed setup order %v, want %v", fixture.order, want)
	}
}

// A partial attached-channel result remains observable and closes on refusal.
func TestDevPPPSetupTypedPartialChannelRetainsCleanup(t *testing.T) {
	fixture := newTypedSetupFixture()
	attachFailure := errors.New("attachment outcome unconfirmed")
	closeFailure := errors.New("original channel close unconfirmed")
	fixture.ops.channel = func(*os.File) (*CreatedChannel, error) {
		fixture.order = append(fixture.order, "attach")
		return fixture.channel, attachFailure
	}
	fixture.ops.closeChannel = func(channel *CreatedChannel) error {
		if channel != fixture.channel {
			t.Fatal("partial setup closed another channel")
		}
		fixture.order = append(fixture.order, "close-channel")
		return closeFailure
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if channel != fixture.channel {
		t.Fatal("partial channel receipt was discarded")
	}
	if unit != nil {
		t.Fatal("unit was created after attachment failed")
	}
	if !errors.Is(err, attachFailure) {
		t.Fatalf("setup lost attachment cause: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), closeFailure) {
		t.Fatalf("setup lost original channel cleanup: %v", err)
	}
	if want := []string{"attach", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("partial channel cleanup %v, want %v", fixture.order, want)
	}
}

// Unit refusal keeps both original receipts and attempts both closes in order.
func TestDevPPPSetupTypedPartialUnitRetainsAllCleanup(t *testing.T) {
	fixture := newTypedSetupFixture()
	createFailure := errors.New("creation outcome unconfirmed")
	unitFailure := errors.New("original unit close unconfirmed")
	channelFailure := errors.New("original channel close unconfirmed")
	fixture.creator.create = func(string) (*CreatedUnit, error) {
		fixture.order = append(fixture.order, "create")
		return fixture.unit, createFailure
	}
	fixture.ops.closeUnit = func(unit *CreatedUnit) error {
		if unit != fixture.unit {
			t.Fatal("rollback closed another unit")
		}
		fixture.order = append(fixture.order, "close-unit")
		return unitFailure
	}
	fixture.ops.closeChannel = func(channel *CreatedChannel) error {
		if channel != fixture.channel {
			t.Fatal("rollback closed another channel")
		}
		fixture.order = append(fixture.order, "close-channel")
		return channelFailure
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if channel != fixture.channel {
		t.Fatal("failed setup discarded original channel receipt")
	}
	if unit != fixture.unit {
		t.Fatal("failed setup discarded original unit receipt")
	}
	if !errors.Is(err, createFailure) {
		t.Fatalf("setup lost creation cause: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), unitFailure) {
		t.Fatalf("setup lost unit cleanup: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), channelFailure) {
		t.Fatalf("setup lost channel cleanup: %v", err)
	}
	if want := []string{"attach", "create", "close-unit", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("rollback order %v, want %v", fixture.order, want)
	}
}

// A unit cleanup panic is retained and cannot prevent original channel cleanup.
func TestDevPPPSetupTypedUnitClosePanicStillClosesChannel(t *testing.T) {
	fixture := newTypedSetupFixture()
	createFailure := errors.New("unconfirmed native creation")
	closeFailure := errors.New("original unit closer panic")
	fixture.creator.create = func(string) (*CreatedUnit, error) { return fixture.unit, createFailure }
	fixture.ops.closeUnit = func(unit *CreatedUnit) error {
		if unit != fixture.unit {
			t.Fatal("cleanup panic fixture received another unit")
		}
		fixture.order = append(fixture.order, "close-unit")
		panic(closeFailure)
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if channel != fixture.channel {
		t.Fatal("cleanup panic discarded original channel receipt")
	}
	if unit != fixture.unit {
		t.Fatal("cleanup panic discarded original unit receipt")
	}
	if !errors.Is(err, createFailure) {
		t.Fatalf("cleanup panic erased original creation cause: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), closeFailure) {
		t.Fatalf("cleanup panic was not retained as physical uncertainty: %v", err)
	}
	if want := []string{"attach", "close-unit", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("cleanup panic order %v, want %v", fixture.order, want)
	}
}

// A nil successful result cannot be admitted or trigger the next allocation.
func TestDevPPPSetupTypedMissingChannelRefusesBeforeUnit(t *testing.T) {
	fixture := newTypedSetupFixture()
	fixture.ops.channel = func(*os.File) (*CreatedChannel, error) { return nil, nil } //nolint:nilnil // Deliberately malformed attach result exercises setup refusal.
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if err == nil {
		t.Fatal("nil attached channel was admitted")
	}
	if channel != nil {
		t.Fatal("setup invented a channel receipt")
	}
	if unit != nil {
		t.Fatal("setup allocated a unit without a channel")
	}
	if len(fixture.order) != 0 {
		t.Fatalf("missing channel triggered work: %v", fixture.order)
	}
}

// A missing unit result refuses and releases the already attached channel.
func TestDevPPPSetupTypedMissingUnitClosesChannel(t *testing.T) {
	fixture := newTypedSetupFixture()
	fixture.creator.create = func(string) (*CreatedUnit, error) { return nil, nil } //nolint:nilnil // Deliberately malformed create result exercises setup refusal and channel rollback.
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if err == nil {
		t.Fatal("nil created unit was admitted")
	}
	if channel != fixture.channel {
		t.Fatal("missing unit erased the original channel receipt")
	}
	if unit != nil {
		t.Fatal("setup invented a unit receipt")
	}
	if want := []string{"attach", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("missing unit rollback %v, want %v", fixture.order, want)
	}
}

// A panic at the creation boundary preserves and closes the original channel.
func TestDevPPPSetupTypedPanicRetainsChannel(t *testing.T) {
	fixture := newTypedSetupFixture()
	createFailure := errors.New("creator panic")
	fixture.creator.create = func(string) (*CreatedUnit, error) { panic(createFailure) }
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if !errors.Is(err, createFailure) {
		t.Fatalf("creator panic cause was lost: %v", err)
	}
	if channel != fixture.channel {
		t.Fatal("creator panic lost the original attached channel")
	}
	if unit != nil {
		t.Fatal("creator panic invented a returned unit receipt")
	}
	if want := []string{"attach", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("panic cleanup %v, want %v", fixture.order, want)
	}
}

// A definite original collision releases only its rejected unit before retry.
func TestDevPPPSetupTypedCollisionRetriesAfterConfirmedClose(t *testing.T) {
	fixture := newTypedSetupFixture()
	rejected := &CreatedUnit{}
	attempts := 0
	fixture.creator.create = func(string) (*CreatedUnit, error) {
		attempts++
		if attempts == 1 {
			fixture.order = append(fixture.order, "rejected")
			return rejected, unix.EBUSY
		}
		fixture.order = append(fixture.order, "created")
		return fixture.unit, nil
	}
	fixture.ops.rejected = func(unit *CreatedUnit) bool { return unit == rejected }
	fixture.ops.closeUnit = func(unit *CreatedUnit) error {
		if unit != rejected {
			t.Fatal("collision retry closed the successful original unit")
		}
		fixture.order = append(fixture.order, "close-rejected")
		return nil
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	if channel != fixture.channel {
		t.Fatal("collision retry replaced the original channel")
	}
	if unit != fixture.unit {
		t.Fatal("collision retry published the rejected unit")
	}
	if attempts != 2 {
		t.Fatalf("creation attempts=%d, want2", attempts)
	}
	if want := []string{"attach", "rejected", "close-rejected", "pause", "created"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("collision order %v, want %v", fixture.order, want)
	}
}

// The same collision errno without a native receipt closes once and never retries.
func TestDevPPPSetupTypedUncertainCollisionDoesNotRetry(t *testing.T) {
	for _, code := range []unix.Errno{unix.EBUSY, unix.EEXIST} {
		t.Run(code.Error(), func(t *testing.T) {
			fixture := newTypedSetupFixture()
			attempts := 0
			fixture.creator.create = func(string) (*CreatedUnit, error) {
				attempts++
				return fixture.unit, code
			}
			channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
			if !errors.Is(err, code) {
				t.Fatalf("uncertain creation lost errno: %v", err)
			}
			if channel != fixture.channel {
				t.Fatal("uncertain creation discarded original channel")
			}
			if unit != fixture.unit {
				t.Fatal("uncertain creation discarded original unit")
			}
			if attempts != 1 {
				t.Fatalf("uncertain creation retried %d times", attempts)
			}
			if want := []string{"attach", "close-unit", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
				t.Fatalf("uncertain cleanup %v, want %v", fixture.order, want)
			}
		})
	}
}

// Unnamed RTM_NEWLINK uses the same kernel trylock as named creation; a definite
// EBUSY rejection may retry without ever replacing its original channel.
func TestDevPPPSetupTypedUnnamedContentionRetriesDefiniteRejection(t *testing.T) {
	fixture := newTypedSetupFixture()
	rejected := &CreatedUnit{}
	attempts := 0
	fixture.creator.create = func(name string) (*CreatedUnit, error) {
		if name != "" {
			t.Fatalf("unnamed contention invented a configured name: %q", name)
		}
		attempts++
		if attempts == 1 {
			fixture.order = append(fixture.order, "rejected")
			return rejected, unix.EBUSY
		}
		fixture.order = append(fixture.order, "created")
		return fixture.unit, nil
	}
	fixture.ops.rejected = func(unit *CreatedUnit) bool { return unit == rejected }
	fixture.ops.closeUnit = func(unit *CreatedUnit) error {
		if unit != rejected {
			t.Fatal("unnamed retry closed the successful unit")
		}
		fixture.order = append(fixture.order, "close-rejected")
		return nil
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "", fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	if channel != fixture.channel {
		t.Fatal("unnamed retry replaced the original channel")
	}
	if unit != fixture.unit {
		t.Fatal("unnamed retry returned the rejected unit")
	}
	if attempts != 2 {
		t.Fatalf("unnamed attempts=%d, want2", attempts)
	}
	if want := []string{"attach", "rejected", "close-rejected", "pause", "created"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("unnamed contention order %v, want %v", fixture.order, want)
	}
}

// Failed rejected-file cleanup prevents another creation while retaining its cause.
func TestDevPPPSetupTypedCollisionCloseFailureStopsRetry(t *testing.T) {
	fixture := newTypedSetupFixture()
	closeFailure := errors.New("rejected original file close unconfirmed")
	attempts := 0
	fixture.creator.create = func(string) (*CreatedUnit, error) {
		attempts++
		return fixture.unit, unix.EEXIST
	}
	fixture.ops.rejected = func(*CreatedUnit) bool { return true }
	fixture.ops.closeUnit = func(unit *CreatedUnit) error {
		if unit != fixture.unit {
			t.Fatal("close refusal used another rejected unit")
		}
		fixture.order = append(fixture.order, "close-unit")
		return closeFailure
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if channel != fixture.channel {
		t.Fatal("failed collision cleanup discarded original channel receipt")
	}
	if unit != fixture.unit {
		t.Fatal("failed collision cleanup discarded original unit receipt")
	}
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("cleanup refusal lost original collision: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), closeFailure) {
		t.Fatalf("cleanup refusal lost original close cause: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("unconfirmed rejected-file closure retried %d times", attempts)
	}
	if want := []string{"attach", "close-unit", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("collision close failure order %v, want %v", fixture.order, want)
	}
}

// Definite named collisions keep the existing five-attempt bound and final cleanup.
func TestDevPPPSetupTypedCollisionBudget(t *testing.T) {
	fixture := newTypedSetupFixture()
	attempts := 0
	closes := 0
	pauses := 0
	fixture.creator.create = func(string) (*CreatedUnit, error) {
		attempts++
		return &CreatedUnit{}, unix.EEXIST
	}
	fixture.ops.rejected = func(*CreatedUnit) bool { return true }
	fixture.ops.closeUnit = func(*CreatedUnit) error { closes++; return nil }
	fixture.ops.pause = func(delay time.Duration) {
		if delay != 20*time.Millisecond {
			t.Fatalf("collision backoff=%s, want20ms", delay)
		}
		pauses++
	}
	channel, unit, err := devPPPSetup(fixture.creator, fixture.transport, "zc-setup", fixture.ops)
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("collision budget lost final cause: %v", err)
	}
	if channel != fixture.channel {
		t.Fatal("exhausted collision budget discarded original channel receipt")
	}
	if unit == nil {
		t.Fatal("exhausted collision budget discarded final rejected unit")
	}
	if attempts != 5 {
		t.Fatalf("attempts=%d, want5", attempts)
	}
	if closes != 5 {
		t.Fatalf("original rejected closes=%d, want5", closes)
	}
	if pauses != 4 {
		t.Fatalf("retry waits=%d, want4", pauses)
	}
	if want := []string{"attach", "close-channel"}; !reflect.DeepEqual(fixture.order, want) {
		t.Fatalf("exhausted collision cleanup %v, want %v", fixture.order, want)
	}
}
