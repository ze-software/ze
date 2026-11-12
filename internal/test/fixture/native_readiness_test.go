package fixture

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNativeReadinessWaitsForAuthenticatedStateWithinContext(t *testing.T) {
	done := make(chan struct{})
	attempts := 0
	result, err := nativeWaitForLogin(t.Context(), done, func() error {
		t.Error("wait ran before daemon exit")
		return nil
	}, func(context.Context) fixture10ProcessResult {
		attempts++
		if attempts < 40 {
			return fixture10ProcessResult{code: 1, stderr: "not listening yet"}
		}
		return fixture10ProcessResult{code: 0, stdout: "authenticated version"}
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 40 || result.code != 0 || result.stdout != "authenticated version" {
		t.Fatalf("readiness result=%+v attempts=%d", result, attempts)
	}
}

func TestNativeReadinessStopsWhenDaemonExits(t *testing.T) {
	done := make(chan struct{})
	exit := errors.New("daemon exited 17")
	waitCalls := 0
	wait := func() error { waitCalls++; return exit }
	close(done)
	_, err := nativeWaitForLogin(t.Context(), done, wait, func(context.Context) fixture10ProcessResult {
		t.Fatal("authenticated probe ran after daemon exit")
		return fixture10ProcessResult{}
	}, 0)
	if !errors.Is(err, exit) {
		t.Fatalf("exit cause lost: %v", err)
	}
	if waitCalls != 1 {
		t.Fatalf("daemon exit was not read from its owner exactly once: %d", waitCalls)
	}
	if status := wait(); status != exit {
		t.Fatalf("process owner lost exit status: %v", status)
	}
}

func TestNativeReadinessDaemonExitCancelsInFlightProbe(t *testing.T) {
	done := make(chan struct{})
	exit := errors.New("daemon exited during authentication")
	waitCalls := 0
	wait := func() error { waitCalls++; return exit }
	_, err := nativeWaitForLogin(t.Context(), done, wait, func(ctx context.Context) fixture10ProcessResult {
		close(done)
		<-ctx.Done()
		return fixture10ProcessResult{code: 124, stderr: "authentication interrupted"}
	}, 0)
	if !errors.Is(err, exit) {
		t.Fatalf("in-flight probe lost daemon exit: %v", err)
	}
	if waitCalls != 1 {
		t.Fatalf("daemon exit was not read from its owner exactly once: %d", waitCalls)
	}
	if status := wait(); status != exit {
		t.Fatalf("process owner lost exit status: %v", status)
	}
}

func TestNativeReadinessRejectsContextCanceledDuringSuccessfulProbe(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	cause := errors.New("lifecycle budget ended")
	done := make(chan struct{})
	_, err := nativeWaitForLogin(ctx, done, func() error {
		t.Error("wait ran before daemon exit")
		return nil
	}, func(context.Context) fixture10ProcessResult {
		cancel(cause)
		return fixture10ProcessResult{code: 0, stdout: "late result"}
	}, 0)
	if !errors.Is(err, cause) {
		t.Fatalf("accepted an out-of-budget result: %v", err)
	}
}

func TestNativeReadinessFailureRetainsClientAndDaemonOutput(t *testing.T) {
	output := new(lockedBuffer)
	if _, err := output.Write([]byte("daemon stdout marker\ndaemon stderr marker\n")); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("daemon exited before readiness")
	err := nativeLoginFailure(output, fixture10ProcessResult{code: 23, stdout: "client stdout marker", stderr: "client stderr marker"}, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("failure cause lost: %v", err)
	}
	for _, required := range []string{"daemon stdout marker", "daemon stderr marker", "client stdout marker", "client stderr marker", "23"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("diagnostic missing %q: %v", required, err)
		}
	}
}
