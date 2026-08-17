---
kind: note
level:
stage:
---
`make ze-precommit-verify`, in the foreground ("Running ze-precommit-verify" below). Not `go test`,
not any subset.
Before any verify target, check freshness. A FRESH status covers the
byte-identical tree and forbids rerunning `make ze-precommit-verify` or
`make ze-precommit-verify-changed`. The check output is qualified by mode:
`FRESH(ze-precommit-verify)` covers everything; `FRESH(ze-precommit-verify-changed)` is a
weaker pass (no full lint, no vet evidence, no cached full unit pass) --
for commit preparation treat both as FRESH, but when the work explicitly
needs the full gate (release evidence, repo-wide changes), a full
`make ze-precommit-verify` on a FRESH(ze-precommit-verify-changed) tree is permitted. A pass
recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. `ze-precommit-verify` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `ze-precommit-verify-changed` scopes to packages with uncommitted
`.go` changes PLUS packages committed since the last green verify
(`scripts/dev/changed-pkgs.sh`, baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. For reactor concurrency changes, also run `make
ze-unit-reactor-test-race`. Output writes: `tmp/ze-verify.log`, per-stage logs
under `tmp/verify/`, `tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`. The full mode writes
`tmp/ze-verify-full.json` as well, which is the coverage record a Go-carrying
commit is gated on: the changed mode never writes it, so a cheaper run cannot
certify one.
