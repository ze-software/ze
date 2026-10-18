# Handover: lab Docker resources reaped, phases 1-2

Spec: `plan/spec-lab-docker-resources-reaped-and-go-test-native.md`.
Written 2026-10-18 by the agent that landed phases 1-2.

## Landed

| Commit | Subject |
|--------|---------|
| `ffd98a4193` | le: move the process scanner into the leaf internal/le/procscan |
| `c2b69d6519` | interoplab: label lab Docker resources and reap a dead run's leftovers |

The spec commit after these carries the pasted `cleanup dry` and `cleanup`
runs (Functional Tests table) and the `TestKilledLabRunIsCleanedByNextStart`
result.

## What remains of phases 1-2

| Item | State | Next step |
|------|-------|-----------|
| `docs/architecture/testing/interop.md` | This change's hunks (How It Works steps 1 and 5, "Crash-safe teardown", the Prerequisites row) are NOT committed. The file also holds another session's uncommitted dnsmasq row naming `test/interop/Dockerfile.dnsmasq` and two `dhcpv4-*-dnsmasq` scenarios that are not in HEAD, so it was held back (`ai/rules/git-safety.md`) | Commit it once the dhcpv4 work lands (`plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md`, `plan/handover/05-dhcpv4-lease-maintenance.md`), or carry both together when that session's files are in HEAD |
| `TestKilledLabRunIsCleanedByNextStart` | Proved the verdict on the real daemon in an earlier run; no clean pass under host load. The `*-dm8b7wc2wq89` leftovers of the ~23:20 failure are gone (the post-fix `cleanup dry` found none; spec, Functional Tests). Not rerun after `546f386a3c`: load average 72.8 at 23:31, 44.6 at 23:38 | Rerun when `uptime` is under ~16: `ZE_REQUIRE_DOCKER=1 ./le job run label kill quiet command go test -tags docker -count=1 -run TestKilledLabRunIsCleanedByNextStart ./internal/le/interoplab/`, then check `docker ps -a --filter label=ze.lab` holds no `ze-orphan-*` resource, which proves the new per-run `t.Cleanup` on the daemon. Paste both into the spec row |
| ~~`cleanup` reports a client-side timeout as a failure~~ | DONE in `546f386a3c`: `Docker.confirmRemoval` re-observes after any failed removal (removed / failed / unknown), in `RemoveContainer`, `removeNetwork`, `releaseImage`, so `Lab.cleanup` and `claimName` share it. Four fake-runner tests, three red before the fix | none |
| ~~`TestDockerTagNeverJoinsIntegration` red~~ | DONE in `546f386a3c`: an in-test fixture tree is the negative case; red shown by mutating the predicate, green restored | none |
| ~~The kill test leaves its resources when a child fails~~ | DONE in `546f386a3c` (code): each child prints its `ze.lab.run` first and the parent's `t.Cleanup` removes that run's labeled resources through the wrapper. Unproven on the daemon until the rerun above | the rerun above |
| Real `cleanup` through `./le --name` | The 23:38 run through `./le --name lab06` failed to REBUILD on another session's uncommitted `internal/component/bgp/reactor/peer_static_routes.go`; the same binary was run directly and exited 0 (`2 removed, 0 kept, 0 failed, 0 unknown`) | none for this spec; a build break belongs to that session |
| AC-8 signal roots for deployment, perf, verify evidence and the terminal demo | Not done. Only the integration verbs and ipsec/radius `Run` are rooted | Moves into phase 3, whose files they are |
| Owed gates | `./le test unit all`, `./le test functional gating`, full `./le verify worktree`. Recorded in `plan/verification-debt/3e9fd812.md` | Run each in a fresh agent; clear the debt rows with `./le commit debt-clear` |
| Lint coverage of the docker-tagged file | `.golangci.yml` `build-tags` does not list `docker`, so the default lint never reads `orphans_docker_test.go`. `go vet -tags docker` passes after `546f386a3c` | Lint `internal/le/interoplab` with the docker tag added, in a fresh agent |
| Gates owed by `546f386a3c` | Recorded in `plan/verification-debt/ee26b4ec.md` (3 rows). Scoped lint over `internal/le/interoplab` and `internal/le/test/qemu` not run (five-minute rule) | Run in a fresh agent, clear with `./le commit debt-clear` |

## Owed to `docs/architecture/testing/interop.md` (appended 2026-10-18)

Not written into the file: it still carries another session's uncommitted
dnsmasq hunk. When that session lands, add this paragraph to "Crash-safe
teardown", right after the paragraph that starts "`./le test integration
cleanup` runs the cleanup by hand", and commit it with this change's other
held hunks:

> A removal call that fails, or that outlasts the 30 s client bound while the
> daemon completes it, is not reported failed on its own say. The wrapper
> inspects that exact resource again under the same bound
> (`Docker.confirmRemoval`): gone is "removed", still present is "failed" with
> the removal's error, and an inspect that fails too is "unknown", naming both
> errors. The summary counts the unknown outcomes, and either a failure or an
> unknown makes the command exit 1. The scenario teardown and the same-name
> pre-clean go through the same removals, so a container the daemon removed is
> never listed as a cleanup error. A listing that times out names no resource
> and stays a setup error with the command to run.
> `<!-- source: internal/le/interoplab/docker.go -- Docker.confirmRemoval -->`

## Untracked file not this work's

`plan/spec-lab-docker-cleanup-and-go-test-containers.md` is untracked and reads
like an earlier copy of this spec. Nothing in phases 1-2 refers to it; the owner
decides whether it goes.
