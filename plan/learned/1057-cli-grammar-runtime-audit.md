# 1057 -- cli-grammar-runtime-audit (Feeder 3)

## Context

Parent 1056 shipped Feeders 1 (static YANG gate) and 2 (registration-time
`validateCommandName`) of the CLI grammar gate, and carved a Feeder 3 "runtime
all-plugins `system command list` audit + drift guard" to this follow-up. The
approved design (user gate) was a live harness: boot the daemon with an all-plugins
config, dump the merged surface over the `ze-system:command-list` RPC, grammar-check
each path, plus a drift guard so the config could not silently miss a plugin.
Implementation-phase audit (mandatory `ai/rules/no-fabrication.md` read-the-producer
pass) proved that design provably redundant and its central assumption broken.

## Decisions

- **Pivoted from a daemon-boot audit to two in-process Go regression guards**
  (user-approved, replacing AC-1/AC-2 intent). The daemon-boot / all-plugins-config /
  drift-guard machinery is dropped entirely. Evidence, all read at the producer:
  - **Built-ins are 100% YANG-derived.** `LoadBuiltinsWithAliases`
    (`internal/component/plugin/server/command.go:80-98`, sibling `LoadBuiltins:53-66`)
    sets each built-in's dispatch name from `wireToPaths[reg.WireMethod]` and *skips*
    any handler with no YANG path. Production call `server.go:190` from
    `yang.WireMethodToPaths(loader)`. So every runtime built-in is a strict subset of
    `BuildCommandTree` -- exactly Feeder 1's domain.
  - **Plugin commands are rejected at registration.** The only two writers to the
    plugin command registry (`command_registry.go:229` Register, `:319`
    RegisterDeprecated) both run `validateCommandName` first (`:202`, `:303`), which
    calls `grammar.CheckName` (`:82`) + `command.IsVerb` (`:77`); on violation
    `OK=false; continue` -- the command never enters `r.commands`. No bypass path exists.
  - Therefore the merged `system command list` surface can contain only conforming
    commands *by construction*; a boot-and-dump audit is guaranteed to find nothing.
- **Feeder 3 = two deterministic unit tests** in
  `internal/component/plugin/server/grammar_audit_test.go` (package `server_test`,
  blank-imports `plugin/all` like `all_import_test.go`):
  - `TestRuntimeBuiltinSurfaceGrammar` iterates `AllBuiltinRPCs()`, skips
    `grammar.ExemptCategory(reg.WireMethod)`, runs `grammar.CheckName` over every alias
    path in `WireMethodToPaths[reg.WireMethod]`. This validates the *actual runtime
    built-in assembly source* (the LoadBuiltinsWithAliases inputs) and is the one check
    the live RPC could not do: `Completion` (`command_registry.go:115-120`) strips the
    wire method, so exemption by namespace is only possible in-process.
  - `TestRegistrationRejectsBadGrammar` exercises the `CommandRegistry.Register`
    boundary (not `validateCommandName` in isolation), proving a noun-first / `--flag` /
    mutation-token / non-lowercase name is rejected and never enters the registry, with
    a conforming control that registers. Locks Feeder 2's *wiring* against a refactor
    that drops the validate call (every existing unit test calls `validateCommandName`
    directly and would not catch that).
- Reused `grammar.CheckName`/`ExemptCategory` verbatim (AC-3): no rule logic duplicated.

## Consequences

- Feeder 3 runs as a normal unit test (no daemon, no config, no `.ci`, ~0.03s), so it
  is in every `go test` of the package and needs no all-plugins config that (per
  `startup_autoload.go:78-140`, startup is `ConfiguredPaths`-gated) cannot exist.
- Measured surface: 233 non-exempt built-in command paths checked, 0 findings; exempt
  bridge:5 (announce/withdraw/peer-raw/peer-update/help), editor:2, wire-protocol:24 --
  confirming the exemption branch is exercised on the real surface.
- `ai/rules/cli-grammar.md` Feeder table + `scripts/checks/cli_grammar.go` header
  updated: Feeder 3 is the in-process guard, not a `.ci` audit.
- Grammar is now enforced by three feeders that between them cover 100% of the surface
  with no fragile moving parts: static tree (Feeder 1), registration reject (Feeder 2),
  runtime-assembly + reject-boundary regression locks (Feeder 3).

## Gotchas

- The "belt-and-suspenders runtime audit" instinct is a trap here: when built-ins are
  YANG-derived and plugin registration already rejects, dumping the merged surface adds
  no catch value -- verify the *producers* (LoadBuiltins skip, Register reject) before
  building a live harness to re-check what cannot be non-conforming.
- Exempt built-ins (announce/withdraw/peer-raw/peer-update/help, `plugin *`, `system *`)
  DO appear in `AllBuiltinRPCs()` and in `system command list` output -- they are
  RegisterRPCs built-ins with YANG paths. An audit MUST skip them via
  `ExemptCategory(WireMethod)`; the RPC payload cannot (no wire method), the in-process
  test can. Editor RPCs (`ze-editor:`) have nil handlers / no YANG path, so they never
  reach the dispatcher and never appear in the surface.
- `TestRegistrationRejectsBadGrammar` must hit `Register`, not `validateCommandName`:
  the value is proving the *boundary* is wired, since a dropped validate call is
  invisible to every direct-function test.
- Test 2 lives in `package server_test` (external) to blank-import `plugin/all` without
  an import cycle -- the same reason `all_import_test.go` is external.

## Files

**Feeder 3:** `internal/component/plugin/server/grammar_audit_test.go` (new).
**Docs:** `ai/rules/cli-grammar.md` (Feeder 3 row + rationale), `scripts/checks/cli_grammar.go` (header comment).
**Spec:** `plan/learned/1057-cli-grammar-runtime-audit.md` (Design Pivot, assumptions A-1/A-2 broken/moot, A-3 broken).
