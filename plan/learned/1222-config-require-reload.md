# 1222 -- config-require-reload

## Context

`ze config set` and `ze config deactivate`/`activate` notified a running daemon
to reload BY DEFAULT and took `--no-reload` to opt out. That default is backwards
for the common case (offline editing of a stored config): editing a file should
not reach for the daemon unless the operator asks. The goal was to flip the
default to no-notify and require an explicit `--reload` to opt in, removing the
old `--no-reload` flag. Prompted by an owner review of the IRR terminal demo,
whose commands all carried `--no-reload` purely because the daemon was stopped.

## Decisions

- Flipped `set`, `deactivate`, and `activate` together (they share
  `runDeactivateLike`'s notify block), not just `set`, so the three siblings stay
  consistent. Chose this over a `set`-only change, which would have left
  `deactivate`/`activate` on the old default.
- Removed `--no-reload` entirely and added `--reload` (opt-in), over keeping
  `--no-reload` as a hidden deprecated no-op: all callers are in-repo, so a clean
  surface beat a compat shim. The gate inverts from `if !*noReload` to `if *reload`.
- Left `ze config edit` unchanged (it reloads only when a daemon is reachable, no
  flag): applying is edit's purpose, so opt-out-by-reachability is correct there.
- Proved the flip with a CLI-level functional `.ci` (default exits 0; `--reload`
  exits 0; `--no-reload` exits 2 with a flag error; the persisted value is the
  `--reload` branch's) rather than a live-daemon convergence test: the `.ci`
  harness dispatches plugin commands in-process and has no path to run
  `ze config set --reload` against a live SSH daemon, and the SIGHUP reload
  mechanism itself is already covered by test/firewall/002-reload.ci.

## Consequences

- Scripts that passed `--no-reload` now error (unknown flag); scripts that relied
  on the old default-reload must add `--reload`. All in-repo callers (7 `.ci`, 3
  docs, the irr-filter demo, 3 Go test files) were migrated in the same change.
- The safe option is now the default: offline stored-config editing never contacts
  the daemon unless `--reload` is given.
- Any future config-mutating CLI command should follow the same opt-in `--reload`
  convention.

## Gotchas

- Bare `go test ./internal/component/config/cli/` FAKES REDS: without the feature
  build tags (`ze_core $(ZE_FEATURES) $(ZE_TAGS)`), validators/plugins are not
  registered and unrelated `validate`/`fix` tests fail ("expected a diagnostic").
  Run tagged (`make`-driven) to get a true result. This bit mid-implementation.
- `ze config dump` renders a resolved view that dropped a bare `session/asn/local`
  from a minimal config, so the functional test asserts on `ze config cat` (raw
  stored form `asn local <n>`) instead.
- `flag.ExitOnError` makes an unknown flag `os.Exit(2)`, so `--no-reload` rejection
  is testable only through the real binary (the `.ci`), not a Go unit test that
  calls `cmdSet` directly (it would exit the test process).

## Files

- internal/component/config/cli/cmd_set.go, cmd_deactivate.go (flag + gate)
- internal/component/config/cli/{cmd_set_test,cmd_deactivate_test,cmd_stdin_test}.go
- test/parse/cli-config-reload-flag.ci (new); 7 existing .ci migrated
- docs/guide/{config-deactivate,irr-filtering,authentication}.md
- demos/terminal/irr-filter/{demo.tape,validate.sh,transcript.txt}
