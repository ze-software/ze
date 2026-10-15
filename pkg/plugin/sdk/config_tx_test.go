package sdk

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

var errTestParse = errors.New("candidate refused")

// testTxPlugin is the plugin state a ConfigTx drives in these tests: the
// installed value and every install, so a test sees what ran and in which
// order.
type testTxPlugin struct {
	installed string
	installs  []string
	failNext  bool
	// failWith, when set, is the error of every install until it is cleared.
	failWith error
}

func (pl *testTxPlugin) install(cfg string) error {
	if pl.failNext {
		pl.failNext = false
		return errors.New("install failed")
	}
	if pl.failWith != nil {
		return pl.failWith
	}
	pl.installed = cfg
	pl.installs = append(pl.installs, cfg)
	return nil
}

// testTxParse reads the section of root "own": absent means not present, the
// data "bad" is a refused candidate.
func testTxParse(sections []ConfigSection) (string, bool, error) {
	for _, s := range sections {
		if s.Root != "own" {
			continue
		}
		if s.Data == "bad" {
			return "", false, errTestParse
		}
		return s.Data, true, nil
	}
	return "", false, nil
}

func ownSection(data string) []ConfigSection {
	return []ConfigSection{{Root: "own", Data: data}}
}

func newTestConfigTx() (*ConfigTx[string], *testTxPlugin) {
	pl := &testTxPlugin{}
	return NewConfigTx("unconfigured", testTxParse, pl.install), pl
}

func mustNoErr(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// VALIDATES: spec-plugin-reload-applies-or-refuses A-1. An apply with no
// accepted verify before it, a second apply of one candidate, and an apply
// after a refused verify are each refused, never answered OK.
// PREVENTS: the fail-open apply that reports a commit the plugin never took.
func TestConfigTxApplyWithoutVerifyIsRefused(t *testing.T) {
	tx, pl := newTestConfigTx()
	if err := tx.Apply(); !errors.Is(err, ErrConfigApplyUnverified) {
		t.Fatalf("apply without verify: err %v, want ErrConfigApplyUnverified", err)
	}
	mustNoErr(t, "verify", tx.Verify(ownSection("a")))
	mustNoErr(t, "apply", tx.Apply())
	if err := tx.Apply(); !errors.Is(err, ErrConfigApplyUnverified) {
		t.Fatalf("second apply: err %v, want ErrConfigApplyUnverified", err)
	}
	if err := tx.Verify(ownSection("bad")); !errors.Is(err, errTestParse) {
		t.Fatalf("verify of a bad candidate: err %v, want errTestParse", err)
	}
	if err := tx.Apply(); !errors.Is(err, ErrConfigApplyUnverified) {
		t.Fatalf("apply after a refused verify: err %v, want ErrConfigApplyUnverified", err)
	}
	if pl.installed != "a" {
		t.Fatalf("installed %q, want a", pl.installed)
	}
}

// VALIDATES: AC-1, AC-3, AC-4 at the helper. Verify installs nothing, apply
// installs the candidate, rollback installs the replaced config back, and a
// refused verify leaves the running config.
// PREVENTS: a verify with side effects, and a rollback that leaves the
// candidate of a failed transaction running.
func TestConfigTxVerifyApplyRollback(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(ownSection("boot")))

	mustNoErr(t, "verify", tx.Verify(ownSection("next")))
	if pl.installed != "boot" {
		t.Fatalf("verify installed %q, want boot until apply", pl.installed)
	}
	mustNoErr(t, "apply", tx.Apply())
	if pl.installed != "next" {
		t.Fatalf("after apply: %q, want next", pl.installed)
	}
	mustNoErr(t, "rollback", tx.Rollback())
	if pl.installed != "boot" {
		t.Fatalf("after rollback: %q, want boot", pl.installed)
	}
	mustNoErr(t, "second rollback", tx.Rollback())
	if len(pl.installs) != 3 {
		t.Fatalf("installs %v, want boot, next, boot", pl.installs)
	}

	if err := tx.Verify(ownSection("bad")); !errors.Is(err, errTestParse) {
		t.Fatalf("verify of a bad candidate: err %v, want errTestParse", err)
	}
	if pl.installed != "boot" {
		t.Fatalf("refused verify changed the running config to %q", pl.installed)
	}
}

// VALIDATES: AC-4 for a plugin that booted with no config. The rollback of
// its first apply installs the unconfigured value.
// PREVENTS: a rolled-back first apply that leaves the candidate running.
func TestConfigTxRollbackOfFirstApplyUnconfigures(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(nil))
	if len(pl.installs) != 0 {
		t.Fatalf("configure without a section installed %v", pl.installs)
	}
	mustNoErr(t, "verify", tx.Verify(ownSection("first")))
	mustNoErr(t, "apply", tx.Apply())
	mustNoErr(t, "rollback", tx.Rollback())
	if pl.installed != "unconfigured" {
		t.Fatalf("after rollback: %q, want unconfigured", pl.installed)
	}
}

// VALIDATES: A-2, AC-5. A verify that carries no section for the plugin, and
// a candidate equal to the running config, install nothing; either drops the
// undo set of the apply before it, so a later rollback does not replay it.
// PREVENTS: a shared-root edit restarting the plugin, and a rollback of an
// unrelated transaction reinstalling a config an earlier commit replaced.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(ownSection("boot")))
	mustNoErr(t, "verify", tx.Verify(ownSection("next")))
	mustNoErr(t, "apply", tx.Apply())

	mustNoErr(t, "verify absent", tx.Verify([]ConfigSection{{Root: "other", Data: "x"}}))
	mustNoErr(t, "apply absent", tx.Apply())
	mustNoErr(t, "rollback after absent", tx.Rollback())
	if pl.installed != "next" {
		t.Fatalf("absent root: %q, want next", pl.installed)
	}

	mustNoErr(t, "verify equal", tx.Verify(ownSection("next")))
	mustNoErr(t, "apply equal", tx.Apply())
	mustNoErr(t, "rollback after equal", tx.Rollback())
	if len(pl.installs) != 2 {
		t.Fatalf("installs %v, want boot then next only", pl.installs)
	}
}

// VALIDATES: R-13 at the helper. An install that fails refuses the apply,
// keeps the running config and leaves nothing for a rollback to replay.
// PREVENTS: a failed apply recorded as the running config.
func TestConfigTxInstallFailureKeepsRunning(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(ownSection("boot")))
	mustNoErr(t, "verify", tx.Verify(ownSection("next")))
	pl.failNext = true
	if err := tx.Apply(); err == nil {
		t.Fatal("apply with a failing install: err nil")
	}
	mustNoErr(t, "rollback", tx.Rollback())
	mustNoErr(t, "verify again", tx.Verify(ownSection("next")))
	mustNoErr(t, "apply again", tx.Apply())
	if pl.installed != "next" {
		t.Fatalf("installed %q, want next", pl.installed)
	}
	if len(pl.installs) != 2 {
		t.Fatalf("installs %v, want boot then next", pl.installs)
	}
}

var errTestLost = fmt.Errorf("listener 127.0.0.1:53 not bound again: %w", ErrRunningConfigLost)

// VALIDATES: an install that fails with ErrRunningConfigLost leaves the
// transaction broken, so committing the running config again installs it
// rather than being skipped as unchanged (DeepEqual against running).
// PREVENTS: a DNS plugin left with no listener after a failed reload, which a
// re-commit of the running config answered OK without binding anything.
func TestConfigTxRunningConfigLostRecommitInstalls(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(ownSection("boot")))
	mustNoErr(t, "verify", tx.Verify(ownSection("next")))
	pl.failWith = errTestLost
	if err := tx.Apply(); !errors.Is(err, ErrRunningConfigLost) {
		t.Fatalf("apply: %v, want ErrRunningConfigLost", err)
	}
	pl.failWith = nil
	mustNoErr(t, "verify the running config", tx.Verify(ownSection("boot")))
	mustNoErr(t, "apply the running config", tx.Apply())
	if want := []string{"boot", "boot"}; !slices.Equal(pl.installs, want) {
		t.Fatalf("installs %v, want %v: the re-commit of the running config installed nothing", pl.installs, want)
	}
	mustNoErr(t, "verify again", tx.Verify(ownSection("boot")))
	mustNoErr(t, "apply again", tx.Apply())
	if len(pl.installs) != 2 {
		t.Fatalf("installs %v: a repaired transaction installed an unchanged config again", pl.installs)
	}
}

// VALIDATES: the rollback of a transaction whose install lost the running
// config installs the running config again, and answers an error when that
// install fails too (the error the engine turns into CodeBroken and a restart).
// PREVENTS: a rollback answering OK over a plugin that serves nothing.
func TestConfigTxRollbackAfterRunningConfigLost(t *testing.T) {
	tx, pl := newTestConfigTx()
	mustNoErr(t, "configure", tx.Configure(ownSection("boot")))
	mustNoErr(t, "verify", tx.Verify(ownSection("next")))
	pl.failWith = errTestLost
	if err := tx.Apply(); err == nil {
		t.Fatal("apply with a lost running config: err nil")
	}
	if err := tx.Rollback(); !errors.Is(err, ErrRunningConfigLost) {
		t.Fatalf("rollback whose reinstall fails: %v, want an error wrapping the install failure", err)
	}
	pl.failWith = nil
	mustNoErr(t, "rollback once the reinstall can succeed", tx.Rollback())
	if pl.installed != "boot" {
		t.Fatalf("installed %q after the rollback, want boot", pl.installed)
	}
	mustNoErr(t, "a second rollback", tx.Rollback())
	if len(pl.installs) != 2 {
		t.Fatalf("installs %v, want boot then the one reinstall", pl.installs)
	}
}
