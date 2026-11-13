// Design: docs/functional-tests.md -- privileged executable inspection child ownership.

package fixture

import (
	"errors"
	"strings"
	"testing"
)

func TestSysribExecutableRootChildControl(t *testing.T) {
	err := requireSysribExecutableChildControl(0, func() (bool, error) {
		t.Fatal("root ownership unnecessarily probed capability state")
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSysribExecutableEffectiveKillChildControl(t *testing.T) {
	called := false
	err := requireSysribExecutableChildControl(1000, func() (bool, error) {
		called = true
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("non-root ownership bypassed its effective capability probe")
	}
}

func TestSysribExecutableMissingKillRefusesChild(t *testing.T) {
	err := requireSysribExecutableChildControl(1000, func() (bool, error) {
		return false, nil
	})
	if err == nil {
		t.Fatal("non-root fixture may start an unkillable privileged child")
	}
	if !strings.Contains(err.Error(), "CAP_KILL") {
		t.Fatalf("missing process-control privilege is unnamed: %v", err)
	}
	if !strings.Contains(err.Error(), "./le test qemu") {
		t.Fatalf("missing privilege has no native carrier remedy: %v", err)
	}
}

func TestSysribExecutableUnknownKillRefusesChild(t *testing.T) {
	cause := errors.New("effective capability query refused")
	err := requireSysribExecutableChildControl(1000, func() (bool, error) {
		return true, cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("unknown privilege did not preserve its cause: %v", err)
	}
}
