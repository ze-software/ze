# 1233 -- fixit-supply-chain-hardening

## Context

An audit found gaps in Ze's software supply-chain posture: no scheduled SCA (govulncheck)
scan; CodeQL analyzed only `go build ./...`, missing the entire feature-gated attack surface
behind `//go:build ze_core/ze_distro/ze_appliance/ze_setup/ze_<feature>` (most of `cmd/ze` +
the whole appliance); six direct dependencies pinned to pseudo-versions with no hygiene
record; the vendored gokrazy updater had silently regressed its DoS-hardening (lost
`io.LimitReader` + `http.NoBody`); and the appliance's builddir pins plus a shipped GPLv2
`rtr7/kernel` had no review cadence or license sign-off. Goal: close each gap without slowing
the inline dev/merge loop.

## Decisions

- **SCA is SCHEDULED, not an inline gate.** `ze-vulncheck` is an on-demand Makefile target
  run by a cron workflow (`.github/workflows/govulncheck.yml`), deliberately absent from
  `stagesForMode`/`ze-verify`: govulncheck needs a network fetch of the vuln DB, and a
  transient fetch failure or a newly published advisory must never wedge the merge gate.
- **`go run govulncheck@latest`, over a `go.mod` tool dependency.** This repo VENDORS; adding
  x/vuln as a module `tool` dep would force vendoring govulncheck's large analysis tree
  (x/tools SSA, callgraph) into `vendor/` -- heavy and build-fragile for a CI-only tool. The
  `@latest` form resolves the tool outside the main module, so zero go.mod/go.sum/vendor
  churn. Verified `go run ...@latest` works despite the vendor dir.
- **CodeQL builds the SHIPPED tag combos**, not `./...`: `ze_core ze_distro $(ZE_FEATURES)`,
  `ze_core ze_appliance $(ZE_FEATURES)`, `ze_setup` -- mirroring bin/ze, bin/ze-appliance,
  bin/ze-setup exactly. The 13 `$(ZE_FEATURES)` service tags (feature-gates.txt) are
  duplicated as literals because a static workflow cannot expand a Makefile variable;
  `TestCodeQLBuildUsesShippedTags` guards against drift by reading feature-gates.txt and
  asserting every ze_ tag appears in codeql.yml. Without the feature tags, the thin
  service registration/glue files stay out of the CodeQL DB.
- **The always-run supply-chain guard is the vendored-updater fix-marker unit test**
  (`internal/appliance/updater_hardening_markers_test.go`), NOT the scheduled scan: it is
  deterministic and gates merges, catching a re-vendor that drops the DoS hardening.
- **Pin hygiene = move-where-a-tag-exists, document where none does.** All 6 direct
  pseudo-version pins have NO upstream semver tag (proxy `@v/list` empty for each), so none
  moved (build-safe); each is documented in `ai/rules/platform-linux.md`. Documentation
  is a valid AC-4 outcome.

## Consequences

- `make ze-vulncheck` runs on demand and on cron; pinned by `TestGovulncheckScheduledWorkflow`.
  CodeQL tag coverage is pinned by `TestCodeQLBuildUsesShippedTags` including a feature-gates.txt
  drift guard, so adding a service tag there without teaching CodeQL to build it fails the test.
- `security-extended` CodeQL queries stay commented: an extended query pack cannot be
  validated locally and could red/slow the analysis job (spec R-2 keeps it optional).
- The GPLv2 `rtr7/kernel` source-offer sign-off is FLAGGED (unresolved), not adjudicated --
  a legal task recorded for a human, not code.

## Gotchas

- A bare `go build ./...` compiles NONE of the `//go:build`-gated files, so CodeQL was
  silently analyzing a fraction of the code. Feature-gated projects must build every shipped
  tag combo for SCA/SAST to see the real surface.
- `go run pkg@latest` works even when the current module has a `vendor/` directory (the tool
  resolves outside the main module), so a CI-only tool need not be vendored.
- The shipped binary's tag set is `ze_core ze_distro` PLUS the 13 default-on `$(ZE_FEATURES)`
  service tags -- easy to under-count. Cross-check against feature-gates.txt, not memory.

## Files

- `Makefile` (`ze-vulncheck` target + .PHONY + help)
- `.github/workflows/govulncheck.yml` (new, scheduled SCA)
- `.github/workflows/codeql.yml` (shipped tag-combo build incl. `$(ZE_FEATURES)`)
- `scripts/status/verify_run_test.go` (`TestGovulncheckScheduledWorkflow`, `TestCodeQLBuildUsesShippedTags` + feature-drift guard)
- `ai/rules/platform-linux.md` (pin-hygiene table, review cadence, GPLv2 sign-off note)
- `internal/appliance/updater_hardening_markers_test.go` (AC-3, prior commit 7a54527d0)
