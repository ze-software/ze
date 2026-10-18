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
| `TestKilledLabRunIsCleanedByNextStart` | Proved the verdict on the real daemon in an earlier run; no clean pass under host load (see the spec row) The 2026-10-18 ~23:20 rerun failed in the child (`docker run -d` past 1 min, load average 20-34) and left five labeled resources (`*-dm8b7wc2wq89`, tag `ze-interop-retained:APM5SD5J2FUONMAX6J3ZOY32GH`) on the daemon; removing them by hand was refused by the permission system, so run `./le test integration cleanup` (with an le built from HEAD) first. Then rerun on a quiet daemon: `ZE_REQUIRE_DOCKER=1 ./le job run label kill quiet command go test -tags docker -count=1 -run TestKilledLabRunIsCleanedByNextStart ./internal/le/interoplab/` |
| `cleanup` reports a client-side timeout as a failure | On the 2026-10-18 run the daemon completed all four removals, but three `docker rm -f` / `docker network rm` calls outlasted the 30 s bound, so the report said "3 failed"; the named rerun converged with exit 0 | Decide whether a timed-out removal re-checks presence before reporting failure. Not built: it changes the verb's contract |
| `TestDockerTagNeverJoinsIntegration` red | Never forced: the hook asked for approval to delete a probe `_test.go` | Force the red with a temporary `//go:build docker && integration` file, confirm red, remove it, confirm green |
| The kill test leaves its resources when a child fails | The ~23:20 failure left both children's labeled container, network and tag behind; the test's own end-of-run sweep did not run | Check whether the test registers a `t.Cleanup` sweep for the failure path; it is this spec's test |
| AC-8 signal roots for deployment, perf, verify evidence and the terminal demo | Not done. Only the integration verbs and ipsec/radius `Run` are rooted | Moves into phase 3, whose files they are |
| Owed gates | `./le test unit all`, `./le test functional gating`, full `./le verify worktree`. Recorded in `plan/verification-debt/3e9fd812.md` | Run each in a fresh agent; clear the debt rows with `./le commit debt-clear` |
| Lint coverage of the docker-tagged file | `./le go lint run scope` was clean over the touched packages; whether its flavors include the `docker` tag (`orphans_docker_test.go`) was not checked | Confirm, or lint with `-tags docker` |

## Untracked file not this work's

`plan/spec-lab-docker-cleanup-and-go-test-containers.md` is untracked and reads
like an earlier copy of this spec. Nothing in phases 1-2 refers to it; the owner
decides whether it goes.
