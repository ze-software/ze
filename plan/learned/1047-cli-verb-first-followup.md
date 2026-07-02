# 1047 -- cli-verb-first-followup

## Context

Follow-up to `829-command-verb-first` (which explicitly deferred the YANG-tree
commands). Converted the remaining noun-first offline/daemon commands to verb-first
grammar without shadowing existing daemon commands: `debug` (offline profile
management), `host show`, `crashes show`, and `event list`. Started with invented
`enable`/`disable` verbs; pivoted mid-implementation to `set`/`delete` per user
directive "follow docs.vyos.io" (VyOS models per-subsystem debug log level as
`set system syslog ... level <l>` config and has no enable/disable operational verb).

## Decisions

- **debug uses `set`/`delete`/`show`/`clear`, not new `enable`/`disable` verbs.**
  VyOS-aligned and reuses existing verb roots, so it eliminated all new-verb
  infrastructure (no `yangVerbs` edit, no verb-root anchor modules, no 5-site
  verb-list ripple). `set debug module <n>` = enable, `delete debug module <n>` =
  disable, `set debug active name <n>` = restore, `show debug profile [name <n>]` =
  stored view. Registered as offline locals under set/delete/show/clear; the daemon
  `show debug` (live runtime state) is untouched and NOT shadowed.
- **Do not shadow existing daemon commands.** `show debug`/`show host`/`show crashes`/
  `show event *` already exist as daemon commands. The offline entry points are the
  no-daemon complement, so they must not register a plain local at the daemon path
  (local-first dispatch would shadow the daemon).
- **New offline-fallback mechanism** for read-only daemon commands: host/crashes
  register `registry.RegisterOfflineFallback("show host"/"show crashes", RunShow)`;
  the CLI serves the daemon when reachable and the in-process handler only after a
  connection failure. Essential for crashes (you inspect a crash when the daemon has
  died). The noun-first `host show`/`crashes show` were removed.
- `event list` -> `show event list` merged onto the existing `show event` container
  (recent/namespaces), kept owned by the meta plugin module (grammar rule: cleanup
  is not permission to move ownership). WireMethod `ze-bgp:event-list` kept (label).
- `command`/`plugin` left out of scope: `command` is a cohesive Meta group; operational
  `set plugin` would collide with the config-tree `plugin` node.

## Consequences

- The offline-fallback is reachable ONLY because `cmdutil.RunCommand` was changed to
  not reject a command that has a registered offline fallback (see Gotchas).
- Hard removal of the noun-first forms (user override of cli-grammar.md's deprecation
  requirement); `ze host`, `ze crashes`, `ze debug <module>` now error.
- `show host` on the offline path is JSON-only (`--text` was an offline-only flag; the
  verb-first daemon grammar has no `--flag`).

## Gotchas

- **The offline fallback in `cli.Run`/`runBGP` is dead code unless `RunCommand` lets the
  command through.** `cmdutil.RunCommand` validates every command against the CLI
  binary's YANG tree (`IsValidCommand`) and returns "unknown command" BEFORE calling
  `cli.Run` -- so `show crashes`/`show host` (not in the CLI tree) never reached the
  runBGP fallback. Fix: in `RunCommand`, skip the unknown-command rejection when
  `registry.LookupOfflineFallback(cmdWords)` finds a handler, routing it to the daemon
  path (daemon when up, fallback when down). Caught only by end-to-end smoke test, not
  by build/unit tests.
- YANG container-merge across modules warns on a **description mismatch**: when merging
  `show event list` onto the central `show event` container, do NOT redeclare the
  `event` container's description (the central schema owns it) or the loader warns.
- Hooks: `switch args[0]` is blocked as "command dispatch" even for argument-keyword
  parsing after a variable selector -- use if/else. New `panic(...)` must be a literal
  `"BUG..."` string (no `+` concat, no non-literal). `fmt.Fprintf(os.Stderr,...)` is
  blocked in non-`register.go`/`cmd/` files even when the surrounding file already uses
  it -- anchor edits to exclude the existing lines from the changed region.

## Files

**Mechanism:** `internal/component/command/registry/registry.go` (RegisterOfflineFallback/
LookupOfflineFallback), `internal/component/cli/client/main.go` (runOfflineFallback hook),
`cmd/ze/internal/cmdutil/cmdutil.go` (fallback routing past IsValidCommand).

**debug:** `internal/plugins/debug/{debug.go,register.go,profile.go,debug_test.go}`.

**host/crashes:** `internal/plugins/host/{host.go,register.go}`,
`internal/plugins/crashes/{crashes.go,register.go}`.

**event:** `internal/plugins/meta/yang/ze-command-meta-cmd.yang`,
`internal/component/cmd/subscribe/yang/ze-cli-subscribe-cmd.yang`.

**Tests:** `test/plugin/debug-toggle.ci`, `test/ui/{debug-enable-show,debug-help,
debug-invalid-subsystem,cli-show-crashes-offline,show-event-list}.ci`,
`test/parse/cli-host-show-{cpu,kernel,bogus}.ci`.

**Docs:** `docs/features.md`, `docs/guide/{command-reference,operations,debugging-tools}.md`.
