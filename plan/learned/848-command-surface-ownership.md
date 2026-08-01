# 848 -- command-surface-ownership

## Context

`cmd/ze` owned the entire offline command surface: ~31 `cmd/ze/<domain>/register.go`
packages registered root metadata in `cmd/ze/internal/cmdregistry`, but dispatch lived
in a large static `switch` in `cmd/ze/main.go`, and the command *logic* lived under
`cmd/ze` rather than with the component that owns the behaviour. The registry being under
`cmd/ze/internal` made it unimportable from `internal/component`/`internal/plugins`, so an
owner could not register its own command. The goal: make the command surface
owner-registered -- a command, its root handler, and its `show X` shortcuts live in the
package that owns the behaviour; `cmd/ze` becomes the process entry point that consumes
registrations and keeps only no-owner / process-global commands.

## Decisions

- **Importable leaf registry.** Moved the offline command registry to
  `internal/component/command/registry` (stdlib-only leaf) so any owner imports it from
  `init()` with no cycle. `cmd/ze/internal/cmdregistry` became a re-export shim so the
  ~35 existing importers compiled unchanged through the migration. Same shim pattern
  relocated three shared leaf helpers (`helpfmt`, `suggest`, `ssh/client`) to
  `internal/core/` -- owners needed them and they must not import `cmd/ze`.
- **Dispatch ownership, not just metadata.** Added `RegisterRootHandler(name, handler,
  meta)` returning the dispatch handler, plus `LookupRoot`. `cmd/ze/main.go` asks the
  registry (`dispatchRegisteredRoot`) **before** the legacy static switch; un-migrated
  roots return nil and fall through, so the cutover was zero-behaviour-change until an
  owner migrated. Real ownership requires dispatch ownership; help-only metadata lets the
  switch lie.
- **RuntimeContext for init-unsafe deps.** Root handlers get a `*RuntimeContext` built by
  `main.go` after flag parsing (storage resolver, plugin list, version printer, web/MCP
  flags). Storage is exposed as `func() any` + `StorageAs[T]` so the registry stays
  leaf-like (never imports storage). Local shortcuts (`func(args)int`, no context) that
  need storage use a package-level `SetRuntimeStorage`/`RuntimeStorage()` the owner
  type-asserts -- this replaced `config`'s old `BindStorageCommands(resolve)` thunk.
- **No-owner allowlist as a fixture.** Roots with no narrower owner stay in `cmd/ze`
  (help, version, start, completion, install/service/uninstall, support, skills, ping,
  generate, signal, status, init, passwd, debug, remote, exabgp, crashes, doctor,
  explain, host). Enforced by `scripts/checks/command_ownership.go`, not review comments.
- **Generated aggregator deferred.** Owner blank imports are hand-listed in `main.go` for
  now; the generated command-provider aggregator (Phase 7) was deferred by explicit user
  direction. The interim manual list is honest and the ownership gate covers correctness.

## Consequences

- All 15 owner-backed roots are owner-owned and registry-dispatched: interface, firewall,
  sysctl, tacacs, resolve, l2tp, traffic-control, plugin, yang, env, data, schema, bgp,
  config, cli. Each lives in `internal/.../cli` (or `internal/component/cli/client` for
  the cli facade) and imports nothing under `cmd/ze`.
- Ownership is **enforced**: `make ze-command-ownership-check` (`scripts/checks/
  command_ownership.go`, test `TestNoOwnerAllowlistIsEnforced`) fails if an owner package
  imports `cmd/ze`, if `RegisterRootHandler` is called from `cmd/ze`, or if a central root
  is not allowlisted. It is routed from `scripts/dev/verify_wiring_docs.py` so changed
  command files re-run it.
- `ai/patterns/cli-command.md` and `ai/patterns/registration.md` now document the
  owner-backed model; `validate-commands` (`scripts/docvalid/commands.go`) scans owner
  `cli` register.go files and accepts the `registry` selector, keeping the YANG/handler
  contract honest.

## Gotchas

- **Cascading leaf relocation.** Migrating an owner pulls in every shared `cmd/ze/internal`
  helper it uses; each must move to `internal/core` (with a shim) before the owner can be
  cmd/ze-free. helpfmt/suggest/ssh-client were the recurring ones.
- **cmd/ze tests share one process registry.** Never call `ResetForTest()` from a `cmd/ze`
  test -- it wipes init-registered roots other tests depend on. Use sentinel names.
- **Code-review hook on file deletes.** Deleting a `cmd/ze/<domain>` package while
  `main.go` still imports it blocks; fix `main.go` (remove import + switch case, add blank
  import) in the same batch as the delete.
- **bgp/cli/schema move triggers the generator.** The `ze-bgp-tools-cmd` YANG schema moved
  to `internal/component/bgp/cli/schema`, which `discoverSchemaPackages` then finds; rerun
  `go run scripts/codegen/plugin_imports.go` to keep `plugin/all` current.
- **cli is the command-tree import island.** `cmd/ze/cli` blank-imports `plugin/all` and is
  the command-tree facade for 6 packages; it moved to `internal/component/cli/client`
  (package `client`, importers alias it `cli`) -- it could not merge into the existing
  `internal/component/cli` (self-import + plugin/all cycle).
- **Doc source anchors.** Moving packages broke ~38 `<!-- source: cmd/ze/... -->` doc
  anchors; `make ze-doc-test` catches them -- repoint to the new internal paths.
- **build-ignore-only package fails golangci (exit 7).** `scripts/checks` needed a
  non-ignore companion `checks_test.go` so the linter has a buildable target.

## Status

Phases 1-4 (foundation + all 15 owner migrations), Phase 7 enforcement gate, and Phase 8
docs are complete and verified (`go build ./...`, lint, `ze-doc-test`,
`ze-command-ownership-check` all green). **Remaining:** Phase 5 (move owner-specific
daemon YANG RPCs out of the central `internal/component/cmd/<verb>` packages -- a large,
behaviour-neutral refactor; the YANG/handler contract already passes), Phase 6 (relocate
the doctor-check registry to a leaf so owners register their own checks, per [838]
(838-doctor-check-ownership.md)), the Phase 7 generated aggregator (deferred), and Phase 9
closure.

## Files

- `internal/component/command/registry/{registry.go,registry_test.go}`
- `cmd/ze/internal/cmdregistry/registry.go` (shim)
- `cmd/ze/main.go`, `cmd/ze/help_ai.go`, `cmd/ze/help_command.go`
- `internal/core/{helpfmt,suggest,ssh/client}/` + `cmd/ze/internal/{helpfmt,suggest,ssh/client}/` shims
- `internal/component/{iface,firewall,tacacs,resolve,l2tp,traffic,plugin,bgp,config}/cli/`,
  `internal/plugins/sysctl/cli/`, `internal/plugins/env/`,
  `internal/component/config/{yang,schema,storage}/cli/`, `internal/component/cli/client/`
- `scripts/checks/command_ownership.go`, `scripts/checks/checks_test.go`, `mk/inventory.mk`,
  `scripts/dev/verify_wiring_docs.py`, `scripts/docvalid/commands.go`
- `ai/patterns/cli-command.md`, `ai/patterns/registration.md`, ~38 `docs/` + `ai/` anchor fixes
