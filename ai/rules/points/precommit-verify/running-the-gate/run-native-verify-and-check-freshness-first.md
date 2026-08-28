---
kind: note
level:
stage:
---
`./le verify worktree`, in the foreground ("Running ./le verify current mode full" below). Not `go test`,
not any subset.
Before any verify target, check freshness. `./le verify-status check`
with no arguments asks about the whole tree, and `check <PATH>...` asks about the
named paths alone. A FRESH answer covers the paths it was asked about and forbids
rerunning `./le verify worktree` or
`./le verify worktree`. The check output is qualified by mode:
`FRESH(./le verify current mode full)` covers everything; `FRESH(./le verify current mode changed)` is a
weaker pass (no full lint, no vet evidence, no cached full unit pass) --
for commit preparation treat both as FRESH, but when the work explicitly
needs the full gate (release evidence, repo-wide changes), a full
`./le verify worktree` on a FRESH(./le verify current mode changed) tree is permitted. A pass
recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. `./le verify current mode full` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `./le verify current mode changed` scopes to the packages the change set
reaches: the uncommitted and untracked paths, PLUS the paths committed
since the last green verify (`runSelector`,
`internal/le/changed/scope.go`; baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. What that
selection answers is below. For reactor concurrency changes, also run
`./le job run label reactor-race command go test -race -count=20 ./internal/component/bgp/reactor/...`. Output writes: `tmp/ze-verify.log`, per-stage logs
under that run's own `tmp/verify/run-<start>-<mode>-<id>/` (reach one through the
`detail-log` field of the failure index, never by guessing a path),
`tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`. The full mode writes
`tmp/ze-verify-full.json` as well, which is the coverage record a Go-carrying
commit is gated on: the changed mode never writes it, so a cheaper run cannot
certify one.
