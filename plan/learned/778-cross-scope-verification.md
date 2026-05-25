# 778 -- Cross-scope verification fixes

## Context

The systemd service work was ready for targeted verification, but full `make ze-verify` exposed a BGP plugin-suite failure. The failing `announce` test was caused by the functional runner loading shared `etc/ze/database.zefs` active state instead of the per-test temp config. Fixing that uncovered a runner JSON-normalization mismatch from decoded attribute flag wrappers.

## Decisions

- Kept the BGP runner fix as its own commit, separate from the `ze service` commit.
- Treated the BGP issue as a verification blocker discovered by the full gate, not as part of the service feature.
- Used targeted validation for the isolated fix: `go test ./internal/test/runner` and `bin/ze-test bgp plugin 3`.
- Used explicit commit scripts with cached patch staging, because the worktree had unrelated dirty changes and some files had mixed hunks.
- Kept the service commit scoped to service files and service-only hunks in shared docs and `cmd/ze/main.go`.

## Consequences

- Reviewers can inspect the BGP test-runner behavior independently from the service feature.
- Service review is not polluted by a runner behavior change that happened only because the global verification gate found it.
- In a dirty worktree, whole-file staging is unsafe for shared files. Cached patch staging is the safe default when files contain multiple logical changes.

## Gotchas

- A full verification failure can be valid and still be outside the feature scope. Fixing it may be the right call, but it must be labelled and committed separately.
- `bin/ze-test` must be rebuilt after runner changes before rerunning functional tests, otherwise the old runner binary hides the effect of the code change.
- Shared `database.zefs` state can leak into functional tests if daemon-style `ze <config>` runs use blob storage. Test daemon runs need filesystem storage unless the test explicitly exercises blob behavior.
- `ze bgp decode --json` can expose attribute metadata wrappers. Plugin-format JSON comparisons must normalize those wrappers back to plain attribute values.

## Files

- `internal/test/runner/runner_exec.go` -- per-test daemon storage isolation
- `internal/test/runner/json.go` -- plugin JSON attribute normalization
- `internal/test/runner/runner_exec_test.go` -- daemon argument detection coverage
- `tmp/commit-*-*.sh` -- explicit staged-patch commit scripts for dirty-worktree safety
