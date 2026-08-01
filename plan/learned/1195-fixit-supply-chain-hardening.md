# 1195 - fixit-supply-chain-hardening (AC-3: vendored updater hardening guard)

Spec: `plan/spec-fixit-supply-chain-hardening.md` (AC-3 slice only)

## What
The vendored `github.com/gokrazy/updater` is a hard fork carrying local DoS
hardening (two `io.LimitReader(resp.Body, 1<<20)` caps, `http.NoBody` request
bodies, `defer resp.Body.Close()`). After every `go mod vendor` those fixes must
be re-applied by `scripts/dev/reapply-updater-fixes.py`. Nothing guarded that
the markers survived a re-vendor.

## Key discovery
- `internal/appliance/updater/` **does not exist** in the current tree. The spec's
  file paths (`internal/appliance/updater/body_leak_test.go`, `.../updater.go`) are
  stale (that package lived only in an old modcache snapshot). The updater is
  consumed via the vendored package in `internal/appliance/cmd_push.go:24`.
- The vendored copy `v0.0.0-20260620140544` had **already regressed**: a prior
  re-vendor dropped `io.LimitReader` and `http.NoBody` (only `Body.Close` survived
  because upstream now has it). The reapply script had not been re-run. This is the
  exact regression AC-3's guard is designed to catch, caught live.

## Change
1. Ran `scripts/dev/reapply-updater-fixes.py` -> restored all fixes (LimitReader x2,
   http.NoBody x5, Body.Close x9, slices.Contains x1). Fix-at-source, not a workaround.
2. Added `internal/appliance/updater_hardening_markers_test.go` (package `appliance`):
   `TestUpdaterHardeningMarkersPresent` reads the vendored file and asserts each
   marker's minimum count; `TestUpdaterHardeningMarkersDetectRegression` proves the
   checker is not vacuous (fails on a source with only one LimitReader cap).
   Repo root resolved via `runtime.Caller(0)` so it is CWD-independent.
3. Fixed two latent bugs in `reapply-updater-fixes.py` surfaced by the first run:
   - `apply_resp_body_close` dropped the trailing newline (inverted ternary) even
     when it made no change -> not gofmt-clean. Now re-adds newline iff source had one.
   - `slices` import was inserted after `"io"` (wrong sort position) -> now after `"net/url"`.
   - Added a `gofmt -w` normalization pass in `main()` as the durable backstop so
     future re-vendors stay gofmt-clean regardless of transform mangling. Skipped
     with a warning if `gofmt` is absent from PATH.

## Scope parked (NOT done)
Assigned slice was AC-3 only. AC-1 (Makefile `ze-vulncheck` + `.github/workflows/govulncheck.yml`
+ `go.mod` x/vuln), AC-2 (codeql.yml tag set), AC-4 (go.mod pin->tag), AC-5
(appliance-dep-bumps.md cadence), AC-6 (GPLv2 sign-off) touch shared contended files
(Makefile already `M` by a sibling; go.mod needs heavy tidy/vendor) and were left for
their owning sessions. Scope decision recorded here and in the drain recipe
(`tmp/drain-fixit-supply-chain-hardening.md`).

## Trap for next time
Do not trust spec file paths for a "fresh area": verify the package exists before
writing a test into it. And when a manual re-vendor script exists, assume the vendored
copy may already be regressed — check the markers before writing the guard.

## Files

None recorded.
