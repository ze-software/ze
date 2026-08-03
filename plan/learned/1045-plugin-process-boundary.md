# 1045 - plugin-process-boundary

## Context

Follow-up to the AS112/cos advisory-doctor-check hardening
([1032](1032-as112-review-hardening.md)): user asked to generalize the
`sdk.Plugin.IsInternal()` fix pattern from a one-off (as112, cos) into
something that catches the class going forward. A repo-wide sweep for the
same shape -- a plugin calling another in-process package's plain exported
function directly, bypassing DirectBridge/DispatchCommand, for an operation
that only reaches the engine's real shared state when the plugin runs
in-process -- found 3 more live, previously-undiscovered instances before
any tooling was built.

## Decisions

- Swept every plugin package importing `internal/component/iface` for
  direct calls to functions with the same "package-level singleton /
  subscriber-list registration" shape as as112's `RegisterOwnedAddresses`
  and cos's `GetBackend`. Found `iface.SubscribeCollectNotify`/
  `UnsubscribeCollectNotify` (`internal/component/iface/rate.go,101`) --
  registers a callback into a package-level `collectSubsPtr`, invoked only
  by the rate tracker's background loop, which itself only runs wherever
  `iface`'s own plugin instance runs (`internal/component/iface/register.go`).
  Three plugins called it with zero `IsInternal()` guard: `traffic-usage`
  (its only attach/detach mechanism), `flow-export` (its only counter data
  source, unconditional), `ddos-detect` (via a `trafficstat.EnsureGlobal`/
  `SubscribeRates` preferred path with the identical problem, falling back
  to the same `iface` call when trafficstat is unavailable).
- Severity judged per-plugin, not copy-pasted: `traffic-usage` and
  `flow-export` refuse to start external (same as as112 -- the call IS the
  plugin's entire purpose, total silent feature loss otherwise).
  `ddos-detect` warns (same as cos -- it already frames the
  trafficstat-unavailable case as graceful degradation, so a warning fits
  the plugin's existing failure-handling posture even though both its
  subscribe paths are actually broken external).
- Built a durable mechanical check
  (`scripts/checks/plugin_process_boundary.go`, `make ze-plugin-boundary-check`,
  wired into `ze-verify`/`ze-verify-changed`) rather than doing a one-time
  audit: a curated dangerous-pattern regex list (grows as new instances are
  found) + a presence heuristic (does the same plugin PACKAGE also contain
  an `.IsInternal()`/`warnIfExternal(` call anywhere) -- deliberately not a
  full call-graph static analyzer (considered, rejected as disproportionate
  to two known instances at decision time, five after the sweep). Modeled
  directly on the existing `scripts/checks/iface_resolution.go` /
  `ze-iface-resolution-check` gate (same shape of problem: allowlist +
  regex pattern scan + `go run` + `//go:build ignore` + companion smoke
  test asserting the real tree passes), not invented from scratch.
- Scanned `internal/plugins/` AND `internal/component/bgp/plugins/` (BGP
  plugins use the identical `sdk.NewWithConn`/internal-external mechanism,
  confirmed before scoping the check) -- the first real run found zero
  additional unguarded instances beyond the 5 already fixed, across the
  whole scan.

- A second `/ze-review` pass on the check itself found and fixed 4 more
  issues: (1) BLOCKER -- `ze-plugin-boundary-check` (and its two
  pre-existing siblings `ze-tier-check`/`ze-iface-resolution-check`) was
  added to the Makefile's `_ze-verify-impl`/`_ze-verify-changed-impl`
  targets, which have ZERO callers anywhere in the repo: `ze-verify`/
  `ze-verify-changed` actually invoke `scripts/status/verify_run.go`, which
  has its OWN separate, hardcoded `stagesForMode` stage list that had
  silently never included any of the three. Fixed by adding all three to
  `stagesForMode` (both branches) instead, with a regression test
  (`TestStagesForModeIncludesStaticAnalysisGates`) and a loud comment on the
  now-confirmed-dead `_impl` targets pointing at the real source of truth.
  (2) ISSUE -- the dangerous-pattern regexes required the literal unaliased
  package name (`\biface\.`), missing a renamed import
  (`ifcomp "internal/component/iface"`, confirmed live in
  `internal/plugins/ospf/interface_addr.go`/`origination_v6.go`, though not
  yet exploited). Fixed by resolving each file's actual import alias via
  `go/parser` (`parser.ImportsOnly`) instead of assuming the default name,
  with a `--selftest` mode (temp-dir fixtures) proving alias resolution
  actually works -- the real tree has zero aliased dangerous calls today, so
  only a synthetic fixture can catch a regression here. (3) ISSUE -- no
  functional test proved the refuse/warn behavior through a REAL externally
  launched plugin process for any of the 5 instances (all coverage was a
  synthetic `net.Pipe()` unit test). Investigating this uncovered that NO
  plugin in `internal/plugins/*` had any way to run external at all --
  `ze plugin <name>` is an unrelated CLI verb (event-delivery config), not a
  launcher; the `"ze plugin bgp-rib"`-style strings in
  `inprocess_test.go`/`process.go` are an INTERNAL-only resolution
  convention, never shell-executed. The only real external-plugin bootstrap
  in the whole codebase was the hand-written `exabgp` bridge
  (`internal/plugins/exabgp/main_sdk.go`). Built a small, generic one:
  `sdk.DialTLSEnvRaw` (refactored out of `NewFromTLSEnv`'s existing dial+auth
  logic) returns the raw, post-auth `net.Conn` instead of an
  already-wrapped `*Plugin`, safe to hand to any
  `registry.Registration.RunEngine(conn net.Conn)` func -- which is exactly
  what `GetInternalPluginRunner` already does for internal plugins, just
  with a `net.Pipe()` end instead of a TLS one. `ze-test plugin-external
  <name>` (`internal/test/cli/cmd_plugin_external.go`, test-only, not a
  production launcher) wires this to `registry.Lookup` +
  `ConfigureEngineLogger` + `RunEngine`. Built 5 `.ci` functional tests on
  top of it -- as112 and cos (no `interface` dependency) verified fully
  passing locally; traffic-usage/flow-export/ddos-detect all declare
  `Dependencies: []string{"interface"}`, which has no OS-default backend on
  darwin, so the daemon fails before their external subprocess ever starts
  -- marked `option=needs-linux` with a STATUS comment (matching the
  existing `ddos-detect-mitigate.ci` precedent for this exact class of
  limitation). Then actually run under a real QEMU Linux VM
  (`make ze-qemu-needs-linux-test`) -- all 3 PASS (5.6s each, clean output),
  closing the gap for real rather than leaving it as a documented
  limitation. First QEMU attempt (`make ze-qemu-debug`, a faster/targeted
  path) failed across ALL 5 tests including the already-locally-passing
  as112/cos, with garbled Mach-O magic bytes (`\xcf\xfa\xed\xfe`) in the
  relayed stderr -- `ze-qemu-debug`'s recipe never sets `ZE_TEST_BIN` for
  the VM (unlike `ze-qemu-needs-linux-test`, which does), so `run "ze-test
  ..."` inside the VM resolved via 9p mount to the STALE, host-format
  (darwin) `bin/ze-test` built earlier that session, not the cross-compiled
  Linux one -- confirmed environmental (not my new code) since a
  pre-existing, unrelated fixture (`show-enricher-external.ci`) hit the
  identical "exec format error" in the same run. (4)
  NOTE -- the per-package guard heuristic is presence-based, not
  proof-based (documented limitation, not fixed -- verified not exploitable
  today across all 5 real instances).

## Consequences

- A 6th plugin authoring the same shortcut against one of the 4 currently-
  known dangerous functions will fail `make ze-verify`/`ze-verify-changed`
  immediately instead of shipping a silent runtime bug discoverable only by
  a reviewer manually asking "does this generalize?" (how all 5 known
  instances were actually found).
- The check's dangerous-pattern list is NOT self-updating -- a 6th
  same-process-effect function in a DIFFERENT package (not `iface` or
  `trafficstat`) will not be caught until someone adds it to
  `dangerousPatterns` in `scripts/checks/plugin_process_boundary.go`. This
  is an accepted, stated limitation (documented in the check's own package
  doc comment and `ai/rules/plugins.md`), not a gap masked
  as coverage.

## Gotchas

- Verifying the "refuse" fix with a unit test is not as simple as asserting
  the return code: `runEngine(pluginEnd)` against a plain (non-bridged)
  `net.Pipe()` end returns 1 EITHER WAY -- with the guard, immediately; without
  it, only after `p.Run(ctx, ...)`'s SDK handshake protocol times out
  (~30s), since the plugin falls through to the generic error path. A test
  asserting only `code == 1` passes even with zero guard present (a false
  green). Fixed by also asserting elapsed time is well under the timeout
  (`< 2s`), which genuinely distinguishes "refused at the guard" from
  "failed after the handshake timeout" -- confirmed by watching this exact
  test go from a false-passing 30s run to a correctly-red 30s-timeout
  failure to a true green 0.00s pass across the TDD cycle.
- Validating a brand-new static checker's OWN correctness needs more than
  "it reported OK" -- that's equally consistent with the regexes silently
  matching nothing. Cross-checked with an independent `grep -rln` over the
  same dangerous-pattern list, confirmed it hit exactly the 5 expected
  package directories (no more, no less) before trusting the checker's
  "OK" result.
- `scripts/checks/*.go` files use `//go:build ignore` so they're excluded
  from golangci-lint's type-checking pipeline (confirmed by
  `scripts/checks/checks_test.go`'s own doc comment) -- their only real
  compile/logic verification is the companion `_test.go`'s `go run
  scripts/checks/<name>.go` subprocess smoke test, not `go vet`/lint on the
  ignore-tagged file itself.
- Wiring a NEW check into `_ze-verify-impl`/`_ze-verify-changed-impl` felt
  correct because that's where the two pre-existing siblings already lived
  -- but that similarity was exactly the trap: those two ALSO never ran
  under `make ze-verify` before this fix, for the same reason. A `/ze-review`
  pass catching this required directly reading `ze-verify:`'s Makefile
  recipe and following it to `scripts/status/verify_run.go`, not trusting
  that a target existing in the Makefile means it's on the executed path.
- `NewFromTLSEnv`'s "authenticates" step reads the auth RESPONSE via a
  throwaway `rpc.Conn(conn, conn)` wrapper, then discards it -- reusing that
  SAME approach for a second, fresh `rpc.Conn` (the one `RunEngine`'s own
  `sdk.NewWithConn` builds) risks losing bytes to the first wrapper's
  internal buffering. The existing `ipc.Authenticate`/`ipc.ReadLineRaw` (used
  for the SERVER's symmetric read of the client's auth REQUEST) already
  solved this exact problem with a documented byte-by-byte, no-buffering
  read -- reusing it (exported as `ipc.ReadLineRaw`/`ipc.MaxAuthFrameSize`)
  for the CLIENT's response read was safer than inventing new wire-parsing
  code for a shared, security-relevant transport.
- `.ci` file `test-relax:` comments must use the literal `//` prefix
  (`_RELAX_TOKEN` regex in `pretool-writeedit.py`) -- a `.ci`-native `#
  test-relax:` comment does NOT satisfy the test-weakening guard, since the
  guard was written for `_test.go` files only. For an uncommitted, never-
  merged `.ci` draft with no real prior coverage to protect, the guard still
  fires on any line-count decrease; the workaround is a same-or-larger-line
  surgical edit (or asking the user to approve `rm` and starting fresh), not
  fighting the heuristic with a comment it cannot parse in this file type.
- `Dependencies: []string{"interface"}` (declared by traffic-usage,
  flow-export, ddos-detect, NOT as112/cos) makes ANY functional `.ci` test
  for these three plugins implicitly need a real interface backend --
  darwin has no OS-default one ("no backend configured and no OS default
  available"), so the daemon fails at the INTERFACE plugin's own startup,
  before ever reaching the plugin under test. This is invisible until you
  actually run the test; grep for `Dependencies:` in a plugin's
  `register.go` before assuming a minimal functional-test config will boot
  on a dev machine.

## Files

- `internal/plugins/trafficusage/register.go` + `register_test.go` --
  refuse-external guard (only attach/detach mechanism)
- `internal/plugins/flowexport/register.go` + `register_test.go` --
  refuse-external guard (only counter data source)
- `internal/plugins/ddos/detect/register.go` + `register_test.go` (new) --
  `warnIfExternal` (both trafficstat and iface fallback paths)
- `scripts/checks/plugin_process_boundary.go` + `_test.go` (new) -- the
  mechanical gate: import-alias-aware scan, `--selftest` fixture mode
- `Makefile` -- `ze-plugin-boundary-check` target (runs `--selftest` then
  the real scan, mirroring `ze-tier-check`'s pattern); loud "not the live
  path" comment on `_ze-verify-impl`/`_ze-verify-changed-impl`
- `scripts/status/verify_run.go` + `_test.go` -- `stagesForMode` now
  includes `ze-tier-check`/`ze-iface-resolution-check`/
  `ze-plugin-boundary-check` in both branches; this is the actual fix that
  makes `make ze-verify`/`ze-verify-changed` run all three
- `scripts/dev/dep_audit.py` -- excludes `.claude` from `collect_edges`'s
  walk (was scanning sibling Claude agent worktrees under
  `.claude/worktrees/`, reporting THEIR uncommitted tier violations against
  this repo's own gate); new `--selftest` fixture proving the exclusion
- `pkg/plugin/sdk/sdk.go` -- `dialAndAuth` (extracted shared TLS dial+auth
  from `NewFromTLSEnv`, no behavior change) + new `DialTLSEnvRaw` (returns
  the raw authenticated `net.Conn` instead of a wrapped `*Plugin`)
- `internal/component/plugin/ipc/tls.go` -- `readLineRaw`/`maxAuthFrameSize`
  exported to `ReadLineRaw`/`MaxAuthFrameSize` (mechanical rename, reused by
  `DialTLSEnvRaw`)
- `internal/test/cli/cmd_plugin_external.go` + `_test.go` (new) -- `ze-test
  plugin-external <name>`, registered in `internal/test/cli/register.go`
- `test/plugin/as112-external-refuses.ci`, `cos-external-warns.ci` (new,
  fully passing) -- genuine external-subprocess functional proof
- `test/plugin/trafficusage-external-refuses.ci`,
  `flowexport-external-refuses.ci`, `ddos-detect-external-warns.ci` (new,
  `option=needs-linux`, config-validated but not run locally -- see Gotchas)
- `ai/rules/plugins.md` (new) -- the rule, `ai/rules/INDEX.md`
  regenerated
- `ai/rules/plugins.md` (DirectBridge section) -- new anti-pattern
  entry
- `ai/INDEX.md` -- 3 cross-reference rows (I Want To, Adding a Feature,
  Cross-Cutting Rules)
- `ai/patterns/plugin.md` -- Also Read row
