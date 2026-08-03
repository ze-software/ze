# 1056 -- cli-grammar-gate

## Context

`ai/rules/cli.md` defined the verb-first CLI grammar (verb-noun-action,
typed selectors, no `--flag` in YANG) but nothing enforced it: agents kept drifting
(noun-first commands, `--flag` leaks). `plan/learned/829-command-verb-first.md`
migrated plugin commands to verb-first and explicitly noted a mechanical gate was
"anticipated but never built." The verb vocabulary was defined in three disagreeing
places: learned-829 (8 verbs), `command_registry.go` `commandVerbs` (7, added
`cache`, dropped `monitor`/`resolve`), and the live built-in surface (also `delete`,
`create`, `config`, `metrics`). This spec built the gate and unified the vocabulary.

## Decisions

- Reverse-engineered the prose into an explicit 8-rule set R1-R8 (verb-first, token
  form, no `--flag`, namespace discipline, keyword-before-value, action-before-id,
  config-tree-mutation stays in `set`/`delete`, string identifiers), implemented once
  in `internal/component/command/grammar` and read by every feeder.
- One canonical verb registry `command.Verbs` (`internal/component/command/verbs.go`);
  `command_registry.go` deleted its `commandVerbs` map and derives from it. Chose
  `internal/component/command` (imported by the plugin server, no reverse import) over
  `internal/core` for the registry home.
- Three feeders, one checker: static gate over the compile-time YANG tree
  (`scripts/checks/cli_grammar.go`, in `_ze-verify-impl`); strengthened
  `validateCommandName` at plugin registration; a runtime audit (carved to a follow-up
  spec, see below). Exemptions (`grammar.ExemptCategory`) keyed on wire-method
  namespace (bridge `ze-bgp:announce/…`, wire-protocol `ze-plugin:/ze-system:/ze-bgp:plugin-`,
  editor `ze-editor:`) -- a structural identity, never a per-command allowlist.
- `create` blessed as a canonical runtime-resource-lifecycle verb (user-ratified): the
  iface `create interface …` commands are immediate netlink operations, NOT config-tree
  mutation (which has a separate `ze-iface-conf.yang` path), and cli.md permits
  operational verbs for runtime actions.
- `archive` stays a noun: `request config archive`, not an `archive` verb -- keeps the
  verb set small (learned-829) and consistent with `request commit`/`request reload`.
- Fixed every violation the gate found rather than allowlisting: `config archive`->`request
  config archive`, `metrics pool`->`show metrics pool`, mpls `--limit` prose, `show capture`
  numeric `tunnel-id`->string, and the iface `create`/`delete` family to typed `name` selectors.

## Consequences

- The grammar is now mechanically enforced: `make ze-cli-grammar-check` (in `ze-verify`)
  checks all built-ins; `validateCommandName` checks every plugin command at registration.
  Adding a verb is a one-line edit to `command.Verbs`, read everywhere.
- `plan/spec-command-naming` was already fully implemented by prior command work
  (bgp-prefix, fib-hyphen) -- closed as done; those are a namespace convention, not R1-R8.
- Feeder 3 (runtime all-plugins-config `system command list` audit + drift guard) carved
  to a follow-up: assumption A-3 (one config starts every command-registering plugin) is
  unproven -- no such config exists; startup is per-plugin config-path-driven. Feeders 1+2
  already enforce grammar on 100% of commands, so the runtime audit is belt-and-suspenders.

## Gotchas

- The checker rules over-flag if naive. R6 (value-before-keyword) must be mandatory-only
  AND exempt typed selector-kind nodes (`name`/`id`/…), else it flags the legitimate
  `show route [<cidr>] | lookup` fork and the correct `... name <n> unit` shape. R7 must
  NOT list `create` (a verb) or `del` (only the completion prefix of `delete`).
- Moving a `ze:command` container is a wire break: `config archive`->`request config archive`
  broke `cmd_archive.go`, which sent the bare path `"config archive "+name` over SSH from
  the offline `ze config archive` tool. Grep for programmatic senders (`ExecCommand`,
  `DispatchCommand`) of any renamed path. iface `create`/`delete` had none (user-typed only).
- The keyword/value distinction is structural, not naming: YANG container -> `Node.Children`
  (closed keyword; zero `list` nodes in any `-cmd.yang`), YANG leaf -> `Node.ArgDefs` (value).
- The hooks block `fmt.Printf` in scripts (use `fmt.Fprintf(os.Stdout, …)` literally, not via
  a variable), `+` string concat (use `textbuf`), and any "ExaBGP"/"exabgp" string in engine
  packages (name the E1 category `bridge`, not `exabgp-bridge`).

## Files

**Gate:** `internal/component/command/verbs.go` (+test), `internal/component/command/grammar/checker.go` (+test), `scripts/checks/cli_grammar.go` (+`cli_grammar_test.go`), `mk/inventory.mk`, `Makefile` (`_ze-verify-impl`/`_ze-verify-changed-impl`)
**Feeder 2:** `internal/component/plugin/server/command_registry.go` (+`command_registry_test.go`)
**Fixes:** `internal/plugins/config-archive-cmd/yang/ze-config-archive-cmd.yang`, `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-poolstats-cmd.yang`, `internal/plugins/mpls-cmd/yang/ze-mpls-cmd.yang`, `internal/plugins/diag/yang/ze-diag-cmd.yang`, `internal/component/iface/yang/ze-iface-cmd.yang`, `internal/component/config/cli/cmd_archive.go`
**Docs:** `ai/rules/cli.md`, `ai/rules/cli.md`, `ai/INDEX.md`, `docs/features/interfaces.md`, `docs/architecture/api/commands.md`, `docs/features.md`
