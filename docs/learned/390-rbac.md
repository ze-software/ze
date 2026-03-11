# 390 — RBAC Authorization Library + Dispatcher Interface

## Objective

Add authz library (types, evaluation, matching) and Dispatcher authorization interface for Nokia-inspired profile-based RBAC. Runtime wiring (config→Store, Store→Dispatcher, SSH username propagation) is not yet implemented — this is infrastructure, not an end-to-end feature.

## Decisions

- **Two canonical types only:** `authz.Action` (Allow/Deny) is the single source of truth. `server.Authorizer` interface returns `authz.Action` directly — no duplicate type.
- **Store satisfies Authorizer:** `authz.Store` directly satisfies `server.Authorizer` — no adapter struct needed.
- **Prefix matching with word boundary:** case-insensitive, requires exact match or space after prefix. Prevents "restart" from matching "restartable".
- **Opt-in regex per entry:** regex compiled eagerly at `Validate()` time (config load). Lazy compile safety net for test code only.
- **Auto-numbered entries:** 10, 20, 30... with gap-based insert-before/after. Renumber when gap < 2.
- **Multi-profile:** first entry match across all assigned profiles wins. First profile's section default used when no entry matches in any profile.
- **Unknown commands treated as write:** subsystem/plugin commands that don't match builtins are authorized with `readOnly=false` (conservative default).

## Patterns

- **Single authorization chokepoint:** `Dispatcher.Dispatch()` checks auth after command resolution, before execution — covers builtins AND subsystem/plugin paths.
- **Nil-safe authorizer:** `d.authorizer == nil` means no auth configured, allows all. Zero-config compatible.
- **YANG schema registration via init():** `authz/schema/register.go` blank-imported from `plugin/all/all.go`, loaded in `yang_schema.go`.
- **Thread safety:** `sync.RWMutex` on Store for concurrent SSH sessions. Entries are read-only after config load.

## Gotchas

- **Authorization bypass on subsystem/plugin path:** Initial implementation only checked auth when `matchedCmd != nil` (builtin found). Commands falling through to subsystem/plugin dispatch bypassed RBAC entirely. Fixed by adding explicit auth check before the fallback path.
- **sync.Once incompatible with value-type slices:** Attempted `sync.Once` in `Entry` for thread-safe lazy regex compile. `copylocks` lint caught it — `sync.Once` contains `sync.noCopy` but entries are stored by value in slices (copied on range/append). Kept simple cached field approach since production entries are validated (single-threaded) at config load.
- **Auto-linter import removal:** Adding `authz` import without simultaneously using it caused `goimports` hook to remove it. Must add import + usage in same edit.
- **Not wired end-to-end:** Library + interface only. Still needed: config tree → `authz.Store`, `Store` → `Dispatcher.SetAuthorizer()`, SSH `sess.User()` → `CommandContext.Username`. SSH command executor is also nil (pre-existing from SSH server spec).

## Files

- `internal/component/authz/authz.go` — core types and logic (Action, Entry, Section, Profile, Store)
- `internal/component/authz/authz_test.go` — 44+ tests covering all paths
- `internal/component/authz/schema/ze-authz-conf.yang` — YANG schema
- `internal/component/authz/schema/embed.go` — YANG embed
- `internal/component/authz/schema/register.go` — YANG init registration
- `internal/component/plugin/server/command.go` — Authorizer interface, Dispatch auth checks
- `internal/component/plugin/server/command_test.go` — authorization dispatcher tests
- `internal/component/config/yang_schema.go` — YANG module loading
- `internal/component/plugin/all/all.go` — blank import for authz schema
- `internal/component/ssh/schema/ze-ssh-conf.yang` — profile leaf-list on user
- `test/parse/authz-config-valid.ci` — functional test: YANG accepts authz config
- `test/parse/authz-config-with-user-profile.ci` — functional test: user profile assignment
