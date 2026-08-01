# 720 -- Config Archive v2: Named Archives with Triggers and System Identity

## Context

Ze had a flat config archive model: `archive { location X; }` with a single list of locations, hardcoded filename format, and only commit/manual triggers. The CLI (`ze config archive`) read config files directly, bypassing the daemon. The spec redesigned this into named archive blocks under `system {}`, each with its own location, filename format, trigger type (commit/manual/daily/hourly), and change detection. The `system.host` and `system.domain` identity leaves replaced direct `os.Hostname()` calls, with `os.Hostname()` as the fallback default.

## Decisions

- Chose `os.Hostname()` as the default for `system.host` over `"unknown"`, because the hostname should always be meaningful even without explicit config.
- Chose scheduler in `archive/scheduler.go` (same package) over a separate `scheduler` component, because scheduling is archive-specific with no other consumers.
- Chose RPC-only CLI over standalone file-reading CLI, because Ze's CLI always goes through the daemon via SSH. There is no standalone mode.
- Kept `os.Hostname()` calls in `host/kernel_linux.go`, `web/workbench_dashboard.go` (fallback), `exabgp/bridge`, and `cmd/ze/init` where SystemConfig is unavailable or not yet created.
- Editor commit-triggered archives do not emit `config/archive` events because the editor runs in the CLI process, not in the daemon's event system. Manual and time-based triggers (which go through the daemon) do emit events.

## Consequences

- The `EventEmitter` callback pattern enables future plugin backends (S3, git push) by subscribing to `config/archive` events on the daemon side.
- The archive scheduler must be wired into daemon startup. Any daemon startup code that creates the plugin server must also create and start the scheduler with a context that cancels on shutdown.
- The editor's archive path remains local (no event emission). If plugin subscribers need commit-trigger events, the editor would need refactoring to dispatch archives through the daemon RPC instead of locally.

## Gotchas

- The `block-sprintf-new.sh` hook blocks `fmt.Sprintf` everywhere, including RPC handlers that are not hot paths. Use string concatenation for error messages in handlers.
- The `nilerr` linter catches `return &plugin.Response{Status: StatusError, Data: msg}, nil` when an `err` is in scope. Follow the existing pattern: return the error in both the response data AND as the second return value.
- YANG schema registration requires `init()` imports in both `plugin/all/all.go` (schema) and `cmd/ze/cli/main.go` (RPC handler). Missing either causes silent failure.
- Functional test configs that go through `ReadConfigFromPath` need the YANG schema to be registered. Unit tests using `config.NewTree()` bypass YANG entirely and work without imports.

## Files

### Modified
- `internal/component/config/system/system.go` -- default host changed to os.Hostname()
- `internal/component/config/system/system_test.go` -- updated default assertion
- `internal/component/config/archive/archive.go` -- added EventEmitter to NewNotifier
- `internal/component/config/archive/archive_test.go` -- updated NewNotifier calls, added event emission test
- `internal/component/config/cli/cmd_archive.go` -- rewritten to use SSH RPC
- `internal/component/config/cli/cmd_archive_test.go` -- rewritten for RPC-based CLI
- `internal/component/config/cli/cmd_edit.go` -- updated NewNotifier call (nil eventFn)
- `internal/component/config/cli/main.go` -- updated usage text
- `internal/component/cli/client/main.go` -- added archive cmd import
- `internal/component/plugin/all/all.go` -- added archive schema import
- `plan/spec-config-2-archive.md` -- added review checklists, updated status

### Created
- `internal/component/config/archive/scheduler.go` -- time-based trigger scheduler
- `internal/component/config/archive/scheduler_test.go` -- scheduler tests
- `internal/component/config/archive/cmd/archive.go` -- RPC handler for config archive trigger
- `internal/component/config/archive/yang/ze-config-archive-api.yang` -- RPC definition
- `internal/component/cmd/archive/yang/ze-config-archive-cmd.yang` -- CLI command tree
- `internal/component/config/archive/yang/embed.go` -- YANG embedding
- `internal/component/config/archive/yang/register.go` -- YANG module registration
