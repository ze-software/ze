// Design: docs/research/l2tpv2-ze-integration.md -- owned backend publication
// Related: kernel_backend.go -- reject an absent owner before publication

package l2tp

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

func TestKernelBackendRejectsTypedNilBeforePublicationAndUnwind(t *testing.T) {
	sub := NewSubsystem(&Parameters{})
	var missing *kernelBackendProbe
	calls := 0
	sub.openKernelBackend = func(string) (iface.Backend, error) {
		calls++
		return missing, nil
	}
	if err := sub.prepareKernelBackend(); err == nil {
		t.Fatal("typed nil factory result was accepted")
	}
	if sub.kernelBackend != nil {
		t.Fatal("typed nil factory result was published")
	}
	// This is the real unwind consumer; invoking a method on the typed nil
	// factory result would panic in kernelBackendProbe.Close.
	if err := sub.closeKernelBackend(); err != nil {
		t.Fatal(err)
	}
	owner := &kernelBackendProbe{}
	sub.openKernelBackend = func(string) (iface.Backend, error) {
		calls++
		return owner, nil
	}
	if err := sub.prepareKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || sub.kernelBackend != owner {
		t.Fatalf("valid retry did not publish its independent owner: calls=%d", calls)
	}
	if err := sub.closeKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if owner.closes != 1 {
		t.Fatalf("valid backend closes=%d", owner.closes)
	}
}

func TestKernelBackendStartupPreparationRefusalIsFatal(t *testing.T) {
	factoryFailure := errors.New("original backend factory failed")
	for _, outcome := range []string{"nil", "typed nil", "factory error"} {
		t.Run(outcome, func(t *testing.T) {
			useEnv(t, map[string]string{"ze.l2tp.skip-kernel-probe": "true"})
			previous := newSubsystemKernelWorkerFn
			preparations := 0
			newSubsystemKernelWorkerFn = func(_ chan<- kernelSetupFailed, _ chan<- kernelSetupSucceeded, _ *slog.Logger, prepare kernelBackendPreparation) (*kernelWorker, error) {
				preparations++
				_, err := prepare()
				if err == nil {
					t.Fatal("native worker preparation accepted a missing backend")
				}
				return nil, &kernelBackendPreparationFailure{cause: err}
			}
			t.Cleanup(func() { newSubsystemKernelWorkerFn = previous })
			sub := NewSubsystem(&Parameters{Enabled: true, ListenAddrs: []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0")}})
			acquisitions := 0
			sub.openKernelBackend = func(string) (iface.Backend, error) {
				acquisitions++
				switch outcome {
				case "nil":
					return nil, nil
				case "typed nil":
					var missing *kernelBackendProbe
					return missing, nil
				case "factory error":
					return nil, factoryFailure
				default:
					t.Fatal("unknown backend preparation outcome")
					return nil, nil
				}
			}
			t.Cleanup(func() {
				if err := sub.Stop(t.Context()); err != nil {
					t.Error(err)
				}
			})
			err := sub.Start(t.Context(), nil, nil)
			if err == nil {
				t.Fatal("backend ownership refusal became optional kernel unavailability")
			}
			if outcome == "factory error" {
				if !errors.Is(err, factoryFailure) {
					t.Fatalf("startup lost original factory cause: %v", err)
				}
			}
			if preparations != 1 || acquisitions != 1 {
				t.Fatalf("preparations=%d acquisitions=%d", preparations, acquisitions)
			}
			if sub.started || sub.kernelBackend != nil || len(sub.reactors) != 0 {
				t.Fatal("failed preparation published startup or retained a reactor/backend")
			}
		})
	}
}
