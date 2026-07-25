package process

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: SetStartupError records the cause of a failed 5-stage handshake and
// StartupError returns it, so the engine can report WHY startup failed and not
// only the stage it stopped at.
// PREVENTS: the cause being dropped, which made ze exit with the unactionable
// "plugin interface failed during startup at stage Config" while the real reason
// (`iface: create dummy "zdiag0": operation not permitted`) went only to a Debug
// log below the default WARN level.
func TestSetStartupErrorRecordsCause(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "interface"})

	if got := proc.StartupError(); got != nil {
		t.Fatalf("StartupError on a fresh process = %v, want nil", got)
	}

	cause := errors.New(`interface config: iface: create dummy "zdiag0": operation not permitted`)
	proc.SetStartupError(cause)

	got := proc.StartupError()
	if !errors.Is(got, cause) {
		t.Fatalf("StartupError = %v, want it to wrap/equal %v", got, cause)
	}
}

// VALIDATES: SetStartupError keeps the FIRST cause and ignores later ones.
// PREVENTS: the real reason startup stopped being overwritten by its own
// consequences -- once a plugin dies, follow-on errors ("io: read/write on
// closed pipe") arrive from every stage still in flight and would otherwise
// replace the diagnosis with a symptom.
func TestSetStartupErrorKeepsFirstCause(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "interface"})

	first := errors.New("operation not permitted")
	consequence := errors.New("io: read/write on closed pipe")

	proc.SetStartupError(first)
	proc.SetStartupError(consequence)

	if got := proc.StartupError(); !errors.Is(got, first) {
		t.Fatalf("StartupError = %v, want the first cause %v", got, first)
	}
}

// VALIDATES: SetStartupError(nil) is a no-op and never clears a recorded cause.
// PREVENTS: a caller that unconditionally forwards its err variable erasing the
// diagnosis, and a nil store panicking the atomic pointer.
func TestSetStartupErrorIgnoresNil(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "interface"})

	proc.SetStartupError(nil)
	if got := proc.StartupError(); got != nil {
		t.Fatalf("StartupError after SetStartupError(nil) = %v, want nil", got)
	}

	cause := errors.New("operation not permitted")
	proc.SetStartupError(cause)
	proc.SetStartupError(nil)
	if got := proc.StartupError(); !errors.Is(got, cause) {
		t.Fatalf("StartupError = %v, want the recorded cause %v to survive a nil store", got, cause)
	}
}

// VALIDATES: concurrent SetStartupError calls carrying DIFFERENT concrete error
// types are safe and yield exactly one recorded cause.
// PREVENTS: the atomic.Value "inconsistently typed value" panic -- the tier
// handshake records causes from several goroutines (startup.go runs one per
// process) and real causes are a mix of *errors.errorString, *fmt.wrapError and
// syscall.Errno.
func TestSetStartupErrorConcurrentDistinctTypes(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "interface"})

	causes := []error{
		errors.New("plain"),
		fmt.Errorf("wrapped: %w", errors.New("inner")),
		errors.Join(errors.New("a"), errors.New("b")),
	}

	var wg sync.WaitGroup
	for _, c := range causes {
		wg.Add(1)
		go func(e error) {
			defer wg.Done()
			proc.SetStartupError(e)
		}(c)
	}
	wg.Wait()

	got := proc.StartupError()
	if got == nil {
		t.Fatal("StartupError = nil after concurrent recording, want one of the causes")
	}
	for _, c := range causes {
		if errors.Is(got, c) {
			return
		}
	}
	t.Fatalf("StartupError = %v, want one of the recorded causes", got)
}
