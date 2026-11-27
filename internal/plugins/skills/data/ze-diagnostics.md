---
name: ze-diagnostics
description: Read Ze diagnostics, explanations, and fix plans.
---

# Ze Diagnostics

Use this when Ze config fails to parse or validate, or when triaging a misbehaving Ze daemon at runtime.

## Commands

```sh
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
```

## Diagnostic Shape

Fields from `show config validation | json`:

- `code`: stable lower-kebab diagnostic code
- `severity`: `error` or `warning`
- `message`: short human summary
- `path`, `line`, `column`, `length`: source span
- `expected` and `actual`: mismatch facts when available
- `help`: concise next action
- `fix-safety`: safety label for a repair
- `repair`: optional repair id and summary
- `related`: extra spans or facts

## Fix Safety

- `format-only`: formatting only
- `section-local`: confined to one config section
- `behavior-preserving`: no runtime behavior change
- `api-changing`: config API shape may change
- `target-changing`: runtime target may change
- `requires-human-review`: cannot prove the edit is safe

## Agent Triage

1. Run the failing command with `--json`.
2. Use the span to inspect the relevant config.
3. Run `ze show diagnostic <code>` before broad changes.
4. Fix the earliest root cause first.
5. Re-validate after each fix.

## Live Runtime Triage

When a running Ze daemon misbehaves, start with one structured call before issuing individual queries.

### 1. Capture One Triage Snapshot

Over CLI:

```sh
ze cli -c "show system triage | json"
```

Over MCP, call `ze_show_system` with `action: "triage"` (or `ze_execute` with `"show system triage"`).

One round-trip returns build provenance (`version`), component `health`, daemon `doctor` findings, recent `errors`, active `warnings`, saved `crashes`, unhealthy `plugins`, `reload` status, `recent-logs` (warn and error), and process `runtime` vitals. Sections whose runtime source is unreachable report `available: false` with a reason; capped lists report `total-count` and `truncated` so you know when to fetch full details.

### 2. Drill Down Based on What Triage Flags

- **Crashes and recovered panics (`doctor-internal-defect`)**: inspect the saved crash report named in `crashes` or the error `fix` field with `show crashes name <file>`.
- **Report-bus errors and warnings**: fetch full lists with `show errors` or `show warnings`, and explain any unfamiliar code with `ze show diagnostic <code>`.
- **Logs and event history**: query `show log recent level err` (reads the dedicated fault ring, immune to `info` bursts; also supports `level warn`, `component <name>`, and `count <N>`), `show event recent` (includes `source`, `subject`, and `direction` when present), and `show event delivery`.
- **Hangs, deadlocks, or memory growth**: inspect waiting goroutines with `show system goroutines mode blocked` (use `mode full` only for a hang), and capture AI-readable top-function JSON profile summaries without `go tool pprof` using `show system profile type heap | summary`, `type cpu duration 10s | summary`, `type mutex | summary`, `type block | summary`, `type goroutine | summary`, or `type allocs | summary` (over MCP, pass `pipe: [{"name": "summary"}]`). Bare `show system profile` without `| summary` returns raw `pprof-base64` bytes for `go tool pprof`.
- **Pre-incident execution traces**: list auto-captured and manual Go flight-recorder snapshots with `show system trace`, capture one immediately with `request system trace snapshot`, and fetch a snapshot's trace bytes with `show system trace id <id>`.
- **Plugin and reload issues**: run `show doctor`, `show reload-status`, and `show plugin list`.
- **Forked and internal plugin runtime introspection (`| plugin <name>`)**: scope any of the five runtime diagnostic commands to a specific running plugin process without shell access:
  - `show system goroutines mode blocked | plugin <name>` (also `mode summary` or `mode full`; runs on an independent SDK worker so it still answers when the plugin's ordinary `eventLoop` is deadlocked)
  - `show system profile type heap | plugin <name> | summary` (composable in either order with `| summary` across `cpu`, `heap`, `goroutine`, `allocs`, `mutex`, and `block`)
  - `show runtime memory | plugin <name>` (Go `MemStats`, goroutines, CPU count, `GOMAXPROCS`, Go version)
  - `show system memory | plugin <name>` (Linux `/proc/<pid>/status`, `wchan`, and open `fd-count`; in-daemon plugins report `shared-daemon-process: true`)
  - `show log recent level err | plugin <name>` (queries the plugin's own process-local `slogutil` ring; also supports `level`, `component`, and `count`)
  - Over MCP, pass structured pipe entries with the exact `"args"` list syntax: `pipe: [{"name": "plugin", "args": ["<name>"]}]` (or `pipe: [{"name": "plugin", "args": ["<name>"]}, {"name": "summary"}]` on `show system profile`).
  - **Interpreting responses**: every answer includes `plugin`, `process-mode` (`"internal"` for in-daemon goroutines or `"external"` for forked subprocesses), and `pid`. When a forked external plugin is wedged, timed out, disconnected, or non-Go, the daemon never sends `SIGQUIT` or any terminating signal; instead it returns `fallback: true` with `fallback-reason` (`"timeout"`, `"unsupported"`, or `"transport-error"`), `rpc-error`, `host` Linux `/proc/<pid>` metrics (`state`, `wchan`, `threads`, `vm-rss-kb`, `fd-count`), and up to 32 recent `stderr-tail` lines.

### 3. Bug Report Essentials

Include these facts from triage and drill-down when reporting a runtime bug:

- Exact `version` identity (`release`, `build-date`, `commit`, `modified`, `go`, `os`, `arch`, and `image` when present)
- Failing `doctor` and `errors` codes (including `doctor-internal-defect`) and their messages
- Relevant `crashes` file name and stack trace when any crash or recovered panic occurred
- Recent warn and error log entries and relevant `show event recent` history
- Concrete steps and config that triggered the behavior
