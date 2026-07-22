### `ze-test install` kernel tests (1-6, 20-31, 39, 40) -- broken by the isolated-binary layout, pre-existing

Observed 2026-07-22 on darwin. Under `make ze-verify` / `make ze-functional-test`
the install suite reports `fail 17/37 ... failed 20 [1, 2, 3, 4, 5, 6, 20, 21, 22,
23, 24, 25, 26, 27, 28, 29, 30, 31, 39, 40]`, every one an `exit-code-mismatch`
with `ModuleNotFoundError: No module named 'run'` (or no client output at all).

Mechanism. Each of these tests locates the repo from the binary under test:

    repo=$(CDPATH= cd -- "$(dirname -- "$(command -v ze)")/.." && pwd)
    cd "$repo"
    python3 - <<'PY'
    sys.path.insert(0, "tools/kernel-builder")
    import run

That resolves correctly only while `ze` lives at `<repo>/bin/ze`. `ze-verify`
runs the functional suites through the ISOLATED binary set
(`mk/test-functional.mk`, `ZE_ALT_BIN = $(ZE_ALT_DIR)/bin`, the `ifeq
($(ZE_TEST_CANONICAL),)` branch), where `ze` is at
`tmp/testbin-<suffix>/bin/ze`. `dirname/..` is then the throwaway root, which
has no `tools/kernel-builder`, so the import fails before the test body runs.

PRE-EXISTING, NOT caused by `spec-feature-gate-10-bgp`. The attribution isolates
the variable to binary LOCATION with the SAME binaries -- no rebuild, no code
difference:

- `./bin/ze-test install 2 40` (canonical layout) -> `pass 2/2`.
- `cp bin/ze bin/ze-test tmp/isolab/bin/` then
  `ZE_BIN=.../tmp/isolab/bin/ze ZE_TEST_BIN=.../tmp/isolab/bin/ze-test
  ./tmp/isolab/bin/ze-test install 2 40` -> `fail 0/2`.

Identical bytes, opposite results, so nothing in the working tree's code decides
it. Independently: `git diff HEAD -- test/install/ tools/kernel-builder/
mk/test-functional.mk` shows this spec changed ONE line, adding `$(ZE_FEATURES)`
to the `ze-test` build in `ZE_ALT_BUILD`; the `.ci` files, `run.py`, and the
`ZE_ALT_BIN` layout are untouched.

Environmental scope UNVERIFIED and NO root cause asserted beyond the resolution
mechanism above -- in particular why this suite is red now is not established.
`test/install/ze-kernel-overlay.ci` carries a `// test-relax:
kernel-build-consolidation` marker and `e7698a048 fix(test): ze-kernel-overlay
asserts cache HIT/MISS not build message` is among the unpushed local commits, so
this suite is under active rework by other work; that is context, not a cause.

Triage owner is the install/kernel-builder suite, not BGP. The fix is for the
tests to learn the repo root from something the runner controls rather than from
the binary's neighbour directory (the runner already exports `ZE_BIN` /
`ZE_TEST_BIN` / `ZE_TEST_NO_BUILD`), or for the isolated layout to expose the
repo. Twenty `.ci` files in a suite another session is actively editing were
deliberately NOT touched here to avoid a cross-session collision.
