---
kind: note
level:
stage:
---
`make ze-verify` (timeout 240s). Not `go test`, not any subset.
Before any verify target, check freshness. A FRESH status covers the
byte-identical tree and forbids rerunning `make ze-verify` or
`make ze-verify-changed`. The check output is qualified by mode:
`FRESH(ze-verify)` covers everything; `FRESH(ze-verify-changed)` is a
weaker pass (no full lint, no vet evidence, no cached full unit pass) --
for commit preparation treat both as FRESH, but when the work explicitly
needs the full gate (release evidence, repo-wide changes), a full
`make ze-verify` on a FRESH(ze-verify-changed) tree is permitted. A pass
recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. `ze-verify` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `ze-verify-changed` scopes to packages with uncommitted
`.go` changes PLUS packages committed since the last green verify
(`scripts/dev/changed-pkgs.sh`, baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. For reactor concurrency changes, also run `make
ze-race-reactor`. Output writes: `tmp/ze-verify.log`, per-stage logs
under `tmp/verify/`, `tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`.
