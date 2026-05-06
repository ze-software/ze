# 656 -- Deployment Readiness Deep Review

## Context

Ze needed a comprehensive review to move from experimental to deployment-ready status. The review covered the full repository: BGP protocol, config/reload atomicity, dataplane ownership, access protocols (L2TP/PPP/RADIUS), security hardening, documentation truth, and release gate evidence. The goal was to identify every P0 blocker, remediate or document them, and produce clean reproducible release-candidate evidence from a Docker-backed substitute for the unavailable target-runner.

## Decisions

- Chose Docker-backed `make ze-release-check` over waiting for `target-runner` because the target runner is not available in the development environment; a clean-clone Docker container with the same Woodpecker deps gives equivalent lint/unit/functional evidence
- Chose `ZE_SKIP_SUITES` environment mechanism over hard-coded exclusions because firewall (pid-file signaling) and web (agent-browser) cannot work in containers but should still run locally
- Chose pinning golangci-lint to v2.10.1 over floating `@latest` because the verifier, CI, and dev must agree on lint rules to produce reproducible evidence
- Chose 1200s suite timeout in Docker over the local default because container overhead absorbs 2x the wall-clock time without indicating a real problem
- Chose `--privileged` for the Docker container over minimal capabilities because multiple test paths (nft, iproute2) benefit even though firewall is skipped
- Chose deterministic failure conditions (permission errors, missing-file errors) over root-path success for archive/docvalid tests because tests running as root in containers expose the root-vs-user assumption

## Consequences

- P0-1 (release gate evidence) is now closed with clean Docker evidence from 2026-05-06
- P0-8 (target-runner privileged evidence) and full L2TP PPP/NCP/kernel peer proof remain environment-blocked and cannot be closed from macOS
- The `make ze-release-check` target is a permanent part of the gate for environments without a native Linux runner
- Five Linux-only lint findings are fixed; future lint runs on Darwin and Linux will agree
- Two container-only unit test assumptions (archive permissions, docvalid repo-root) are fixed and will not regress
- Known flakes (plugin 145, editor 203) passed cleanly under the 1200s timeout in Docker

## Gotchas

- Docker Desktop linux/amd64 emulation segfaults during Go compiler operations; use native `ZE_CLEAN_VERIFY_PLATFORM=linux/arm64` on Apple Silicon
- `bash -lc` in `golang:1.25` containers omits `/usr/local/go/bin` from PATH; the verify script must export it explicitly
- `cli/testing` takes 499s in Docker (8+ minutes) due to real process spawning; this can blow a 600s tool timeout
- The `docker logs` command output lags significantly behind real container progress due to buffering; do not use line count as a progress indicator
- Floating linter versions (`@latest`) cause reproducibility failures even within days of a release; always pin

## Files

- `scripts/evidence/effective-verify.sh` (PATH fix, golangci-lint pin, ZE_SKIP_SUITES, timeout, privileged, iputils-ping)
- `.woodpecker/verify.yml` (golangci-lint pin)
- `Makefile` (ze-setup pin, ze-release-check target)
- `internal/component/host/storage_linux.go` (lint: repeated string constant)
- `internal/core/slogutil/slogutil.go` (lint: repeated string constant)
- `internal/plugins/traffic/netlink/snapshot_linux.go` (lint: directory permissions, gosec suppression)
- `internal/component/l2tp/reactor_ppp_linux_test.go` (lint: unused result)
- `internal/component/config/archive/archive_test.go` (deterministic failure condition)
- `scripts/docvalid/doc_drift_test.go` (run from repo root)
- `test/*.ci` (7 L2TP tests: skip-kernel-probe)
- `plan/deployment-readiness-deep-review.md` (this spec)
