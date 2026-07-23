# 1253 -- fixit-ci-schedule-evidence

## Context

CI ran ONLY the fast gate (`make ze-verify` on push/pull_request); the entire heavy evidence surface -- Linux-integration, QEMU, fuzz, mutation, interop -- existed as make targets that no automated pipeline invoked, so a regression in any heavy suite silently reached main. `ai/rules/qemu-testing.md` called QEMU tests blocking while nothing enforced them. The goal was a scheduled nightly pipeline invoking a representative evidence subset, advisory-first, converting "silently reaches main" into next-day detection.

## Decisions

- Validation moved to GitHub Actions over Codeberg Woodpecker (Thomas's direction, 2026-07-20): the Woodpecker impasse was `privileged: true` -- a blocking lint error on the shared instance unless an admin marks the repo trusted, and the lint runs before the `when:` match so it discards EVERY workflow, breaking `verify.yml` on each push. GitHub's `ubuntu-latest` runs the integration job under `sudo` as root, so the six kernel suites get `CAP_NET_ADMIN`/`CAP_NET_BIND_SERVICE` and actually RUN instead of `t.Skipf`-ing to a vacuous green.
- Nightly scope is `make ze-fuzz-test` + the non-QEMU `make ze-integration-test`, both `continue-on-error: true` (advisory-first), invoking make targets by NAME over copied suite recipes (registration over hardcoding). Flip to blocking only after a green baseline.
- Go guard tests over "CI config needs no tests": seven guards in `scripts/dev/github_workflows_test.go` pin the workflow shapes, each proven non-vacuous by mutation. String-based matching over YAML parsing on purpose -- importing `gopkg.in/yaml.v3` would promote an indirect dependency to direct and churn `go.mod`/`go.sum` (supply-chain concern); `#` comments are stripped so a commented-out command cannot satisfy a check.

## Consequences

- **AC-6 (QEMU) remains OPEN as a deliberate runner-gated follow-up:** `ze-qemu-integration-test` is absent from the nightly because GitHub-hosted runners do not reliably provide nested virt / KVM, and it is still enforced by review alone (the `ai/rules/qemu-testing.md` "What actually RUNS these suites" table says so explicitly). The guard makes the follow-up self-announcing: `TestEvidenceNightlyRunsFuzzAndIntegration` FAILS if `ze-qemu-integration-test` is added, so the follow-up cannot land without also updating the guard. When a KVM-capable runner is confirmed, either add the target or switch the step to the `ze-release-evidence` composite (which self-skips QEMU/Docker via its probes). Closure must home this as a deferral row pointing at this summary.
- The fast merge gate is pinned: `TestVerifyWorkflowIsTheFastMergeGate` fails if `verify.yml` gains a schedule trigger or any heavy suite; `TestValidationIsNotOnWoodpecker` fails if a `.woodpecker/` pipeline reappears; `TestWorkflowMakeTargetsExist` fails if any workflow names a make target with no rule head (the failure mode that lets night-only pipelines rot).
- Interop and mutation suites are still unscheduled; they are later expansions once nightly timing is known.

## Gotchas

- The spec's own verification claim was fabricated once: an `evidence-pipeline-dryrun` check was cited that DOES NOT EXIST (grep found the name only inside the spec; no woodpecker binary was present, so the claimed config lint never ran). The real verification is the Go guard tests plus `make -n` on the invoked targets.
- The pre-migration fuzz-only scope rested on a circular justification (an implementer-authored "AUTONOMOUS DEFAULT" note cited as approval). Thomas's explicit instruction is what resolved AC-2, not the note.
- "No cron config exists in the repo" was stale when written: `.woodpecker/perf-nightly.yml` already existed (and was the template); the true gap was that no cron ran any integration/QEMU/fuzz suite.
- Even the non-QEMU `ze-integration-test` needs capabilities (`mk/test-integration.mk:83-105` recipe comments name `CAP_NET_ADMIN` / root); an unprivileged runner would skip all six suites and report a vacuous green. Confirmed by reading the producer, not assumed.

## Files

- `.github/workflows/evidence-nightly.yml` (nightly cron `17 3 * * *`; fuzz + sudo integration jobs, both advisory)
- `.github/workflows/verify.yml` (fast merge gate, ported from Woodpecker; push/pull_request -> `make ze-verify`)
- `.github/workflows/perf-nightly.yml` (ported; scheduled-only, guarded by `TestPerfNightlyIsScheduled`)
- `scripts/dev/github_workflows_test.go` (7 shape guards)
- `ai/rules/qemu-testing.md` ("What actually RUNS these suites" enforcement table)
- Removed: `.woodpecker/` (validation off Codeberg)
