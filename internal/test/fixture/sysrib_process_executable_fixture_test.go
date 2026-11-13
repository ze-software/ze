// Design: docs/functional-tests.md -- executable identity in isolated process proofs.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSysribExecutableDirectReadPreservesIdentity(t *testing.T) {
	const identity = "/tmp/current daemon with spaces "
	got, err := readSysribProcessExecutable(t.Context(), 42, func(path string) (string, error) {
		if path != "/proc/42/exe" {
			t.Fatalf("looked up %q", path)
		}
		return identity, nil
	}, func(context.Context, []string) ([]byte, error) {
		t.Fatal("an ordinary identity read invoked sudo")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != identity {
		t.Fatalf("identity %q, want exact %q", got, identity)
	}
}

func TestSysribExecutablePermissionReadsSameNumericProcess(t *testing.T) {
	const identity = "/tmp/current daemon with spaces "
	called := false
	got, err := readSysribProcessExecutable(t.Context(), 4242, func(path string) (string, error) {
		return "", fmt.Errorf("readlink %s: %w", path, syscall.EACCES)
	}, func(ctx context.Context, argv []string) ([]byte, error) {
		called = true
		if want := []string{"sudo", "-n", "readlink", "/proc/4242/exe"}; !slices.Equal(argv, want) {
			t.Fatalf("privileged identity argv %q, want %q", argv, want)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("privileged metadata read has no deadline")
		}
		if remaining := time.Until(deadline); remaining > sysribExecutableProbeMaximum {
			t.Fatalf("privileged metadata read budget %s exceeds %s", remaining, sysribExecutableProbeMaximum)
		}
		return []byte(identity + "\n"), nil
	})
	if err != nil {
		t.Fatalf("permission-only identity read failed: %v", err)
	}
	if !called {
		t.Fatal("permission refusal did not reach privileged identity read")
	}
	if got != identity {
		t.Fatalf("identity %q, want exact %q", got, identity)
	}
}

func TestSysribExecutableOnlyPermissionEscalates(t *testing.T) {
	other := errors.New("procfs inspection unavailable")
	for _, cause := range []error{syscall.ENOENT, syscall.ESRCH, syscall.EPERM, other} {
		t.Run(cause.Error(), func(t *testing.T) {
			got, err := readSysribProcessExecutable(t.Context(), 42, func(string) (string, error) {
				return "", cause
			}, func(context.Context, []string) ([]byte, error) {
				t.Fatal("non-EACCES procfs failure invoked sudo")
				return nil, nil
			})
			if !errors.Is(err, cause) {
				t.Fatalf("lost original failure %v: %v", cause, err)
			}
			if got != "" {
				t.Fatalf("failed inspection invented identity %q", got)
			}
		})
	}
}

func TestSysribExecutableInvalidPID(t *testing.T) {
	for _, pid := range []int{0, -1} {
		_, err := readSysribProcessExecutable(t.Context(), pid, func(string) (string, error) {
			t.Fatal("invalid PID reached procfs")
			return "", nil
		}, func(context.Context, []string) ([]byte, error) {
			t.Fatal("invalid PID invoked sudo")
			return nil, nil
		})
		if err == nil {
			t.Fatalf("accepted PID %d", pid)
		}
	}
}

func TestSysribExecutableCanceledRead(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := readSysribProcessExecutable(ctx, 42, func(string) (string, error) {
		t.Fatal("canceled lookup reached procfs")
		return "", nil
	}, func(context.Context, []string) ([]byte, error) {
		t.Fatal("canceled lookup invoked sudo")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read returned %v", err)
	}
}

func TestSysribExecutableCancellationBeforeEscalation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	_, err := readSysribProcessExecutable(ctx, 42, func(string) (string, error) {
		cancel()
		return "", syscall.EACCES
	}, func(context.Context, []string) ([]byte, error) {
		t.Fatal("lookup canceled during procfs inspection invoked sudo")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled permission read returned %v", err)
	}
}

func TestSysribExecutablePrivilegedFailureRetainsCause(t *testing.T) {
	for _, cause := range []error{exec.ErrNotFound, errors.New("sudo exited with status 1")} {
		t.Run(cause.Error(), func(t *testing.T) {
			got, err := readSysribProcessExecutable(t.Context(), 42, func(string) (string, error) {
				return "", syscall.EACCES
			}, func(context.Context, []string) ([]byte, error) {
				return nil, cause
			})
			if !errors.Is(err, syscall.EACCES) {
				t.Fatalf("lost original procfs permission cause: %v", err)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("lost privileged inspection failure: %v", err)
			}
			if !strings.Contains(err.Error(), "sudo -n readlink /proc/42/exe") {
				t.Fatalf("failure lacks exact inspection command: %v", err)
			}
			if got != "" {
				t.Fatalf("failed privileged read invented identity %q", got)
			}
		})
	}
}

func TestSysribExecutablePrivilegedReplyRefusals(t *testing.T) {
	for _, raw := range []string{"", "\n", "/tmp/one", "/tmp/one\n/tmp/two\n", "/tmp/one\n\n", "/tmp/one\x00\n", "relative/path\n", strings.Repeat("x", sysribExecutableOutputLimit+1)} {
		called := false
		got, err := readSysribProcessExecutable(t.Context(), 42, func(string) (string, error) {
			return "", syscall.EACCES
		}, func(context.Context, []string) ([]byte, error) {
			called = true
			return []byte(raw), nil
		})
		if !called {
			t.Fatalf("privileged reply with %d bytes never reached validation", len(raw))
		}
		if err == nil {
			t.Fatalf("accepted privileged reply with %d bytes: %q", len(raw), raw)
		}
		if !errors.Is(err, syscall.EACCES) {
			t.Fatalf("malformed reply lost original permission refusal: %v", err)
		}
		if got != "" {
			t.Fatalf("malformed reply published identity %q", got)
		}
	}
}

func TestSysribExecutablePrivilegedReadKeepsParentDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("parent control has no deadline")
	}
	_, err := readSysribProcessExecutable(ctx, 42, func(string) (string, error) {
		return "", syscall.EACCES
	}, func(child context.Context, _ []string) ([]byte, error) {
		got, ok := child.Deadline()
		if !ok {
			t.Fatal("privileged reader lost parent deadline")
		}
		if got != deadline {
			t.Fatalf("privileged reader changed earlier parent deadline %s to %s", deadline, got)
		}
		return []byte("/tmp/zebin/ze\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSysribExecutableDifferentIdentityRemainsDifferent(t *testing.T) {
	const expected = "/tmp/zebin/ze"
	const other = "/tmp/different/ze"
	got, err := readSysribProcessExecutable(t.Context(), 42, func(string) (string, error) {
		return "", syscall.EACCES
	}, func(context.Context, []string) ([]byte, error) {
		return []byte(other + "\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Fatalf("changed observed executable %q to %q", other, got)
	}
	if got == expected {
		t.Fatal("a different executable became an identity match")
	}
}
