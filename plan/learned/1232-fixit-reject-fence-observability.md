# 1232 -- fixit-reject-fence-observability

## Context

Three infra-gated `.ci` tests kept a blind `time.sleep` because a REJECTED or NO-OP
operation left no pollable observable signal: the only evidence was a relayed-stderr line the
runner already checks. A SIGHUP reload that REJECTS a listener rebind changes no observable
state (the l2tp reject is WARN-only, `Subsystem.Reload` returns nil), so an observer had
nothing to wait on. Phase 1 adds a plugin-queryable "reload generation" counter surfaced via
`show reload-status`, so an observer can deterministically wait for "reload processed"
(applied OR rejected) instead of sleeping. Scope is observability only; no reload behavior
changed.

## Decisions

- **Counter via a `show` command, over a new plugin event type.** A monotonic
  reload-generation counter queried with `dispatch_until` is smaller than adding a
  subscribable event, and sufficient for the reload case.
- **Increment at the END of `doReload` (`cmd/ze/hub/main_reload.go`), NOT inside
  `Server.reloadConfig`.** The l2tp reject WARN is produced by `eng.Reload` AFTER
  `s.ReloadConfig`; a counter bumped inside `reloadConfig` would advance BEFORE l2tp
  processed the change, so an observer could read the listener port before the reject ran. The
  test would still pass, but only vacuously ("absence of X proves Y", `ai/rules/testing.md`).
  `doReload` is the only function that knows every reload step has completed.
- **`show reload-status` stays CENTRAL**, not under `config-cli`'s subtree: the counter is
  process-global daemon state with no removable owner, the same class as `show warnings` /
  `show health` (`ai/rules/plugins.md`). Putting a centrally-handled command
  in a plugin's subtree would invert the removal test.

## Consequences

- Phase 2 (external-plugin refuse/warn cases: `as112-external-refuses.ci`,
  `cos-external-warns.ci`) LEFT this spec: the reload counter cannot fence them (the
  refuse/warn fires during plugin STARTUP, no reload occurs) and neither test had an observer
  plugin to poll. Homed to `spec-fixit-reject-fence-observability-deferred-external-plugin-signals`
  (learned 1171); those tests were instead converted onto a non-plugin `await=stderr` `.ci`
  primitive (`internal/test/runner/await_stderr.go`), so the deferred signal was never needed.
- The `test/.ci-sleep-baseline` ratchet now enforces the TRUE count. It had drifted far above
  reality and enforced nothing (see Gotchas).

## Gotchas

- The committed `.ci-sleep-baseline` (463) was NOT the actual `time.sleep(` count in
  `test/**/*.ci` (133 at HEAD). The ratchet only PRINTS advice when the count drops and never
  fails, so the baseline had drifted far above reality and was enforcing nothing. This change
  set it to the true value (132), so the ratchet is now actually tight.
- The l2tp reload test was assumed QEMU-gated throughout the spec; it is NOT. It carries no
  `option=needs-linux`, and sets `option=env:var=ze.l2tp.skip-kernel-probe:value=true` so it
  runs without the kernel L2TP module. Verified PASS on the darwin host. Do not assume a test
  is QEMU-gated from its subject; check its options.
- Fence proven load-bearing by mutation: disabling the `MarkReloadProcessed` call makes the
  observer poll 10s and fail "reload generation never advanced past 0 after SIGHUP" while the
  l2tp reject WARN still fires -- confirming both the fence and the increment-site ordering.

## Files

- `internal/component/plugin/server/reload_generation.go` (new: counter + `MarkReloadProcessed`/`ReloadStatus`)
- `internal/component/plugin/server/server.go` (`reloadGen` field)
- `cmd/ze/hub/main_reload.go` (increment at end of `doReload`)
- `internal/component/cmd/show/reload_status.go` (+ test) (new: `show reload-status` handler)
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` (`container reload-status`)
- `test/plugin/reload-listener-rejected.ci` (converted off `time.sleep`), `test/.ci-sleep-baseline` (ratchet)
- `docs/architecture/api/commands.md` (new `show` command)
