package fixture

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunDispatchesRegisteredDriver(t *testing.T) {
	name := "fixture-test-driver"
	Register(name, func(_ context.Context, args []string) error {
		if strings.Join(args, ",") != "a,b" {
			t.Fatalf("args = %v", args)
		}
		return nil
	})
	if code := Run([]string{name, "a", "b"}); code != 0 {
		t.Fatalf("Run exited %d", code)
	}
}

func TestRunExpandsEnvironmentInDriverArguments(t *testing.T) {
	name := "fixture-test-environment"
	t.Setenv("FIXTURE_TEST_ROOT", "/fixture/root")
	Register(name, func(_ context.Context, args []string) error {
		if strings.Join(args, ",") != "/fixture/root/kernel,/fixture/root/quoted,literal" {
			t.Fatalf("args = %v", args)
		}
		return nil
	})
	if code := Run([]string{name, "$FIXTURE_TEST_ROOT/kernel", `"$FIXTURE_TEST_ROOT/quoted"`, "literal"}); code != 0 {
		t.Fatalf("Run exited %d", code)
	}
}

func TestRunReportsDriverFailure(t *testing.T) {
	name := "fixture-test-failure"
	Register(name, func(context.Context, []string) error { return errors.New("broken observation") })

	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	code := Run([]string{name})
	_ = writer.Close()
	os.Stderr = old
	body := make([]byte, 512)
	n, _ := reader.Read(body)
	_ = reader.Close()
	if code != 1 || !strings.Contains(string(body[:n]), observerFailure+`: broken observation`) {
		t.Fatalf("code=%d stderr=%q", code, body[:n])
	}
}

func TestRunConvertsPanicToObserverFailure(t *testing.T) {
	name := "fixture-test-panic"
	Register(name, func(context.Context, []string) error { panic("broken invariant") })

	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	code := Run([]string{name})
	_ = writer.Close()

	os.Stderr = old
	body := make([]byte, 512)
	n, _ := reader.Read(body)
	_ = reader.Close()
	if code != 1 || !strings.Contains(string(body[:n]), observerFailure+`: fixture panic: broken invariant`) {
		t.Fatalf("code=%d stderr=%q", code, body[:n])
	}
}
func TestAwaitObserverResultDoesNotLoseALateScenarioFailure(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	close(started)
	go func() {
		time.Sleep(10 * time.Millisecond)
		result <- errors.New("scenario assertion failed")
	}()

	err := awaitObserverResult(started, result, errors.New("transport stopped"))
	if err == nil || !strings.Contains(err.Error(), "scenario assertion failed") ||
		!strings.Contains(err.Error(), "transport stopped") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestAwaitObserverResultAcceptsTransportStopAfterScenarioSuccess(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	close(started)
	result <- nil

	if err := awaitObserverResult(started, result, errors.New("transport stopped")); err != nil {
		t.Fatalf("error = %v, want successful scenario verdict", err)
	}
}

func TestAwaitObserverResultReturnsTransportErrorBeforeScenarioStarts(t *testing.T) {
	started := make(chan struct{})
	result := make(chan error, 1)
	runErr := errors.New("startup transport stopped")
	if err := awaitObserverResult(started, result, runErr); !errors.Is(err, runErr) {
		t.Fatalf("error = %v, want %v", err, runErr)
	}
}

func TestRunRefusesUnknownFixture(t *testing.T) {
	if code := Run([]string{"not-registered"}); code != 2 {
		t.Fatalf("Run exited %d", code)
	}
}
