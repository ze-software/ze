# Deferrals: followup-web-cli-ux

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-09 | spec-followup-web-cli-ux AC-8/9 | Nushell shell-generator glue for the new flag inventory + `ze config show` config-section completion | AC-8 scopes completion to bash/zsh/fish (all wired + tested); nushell's single-completer model needs separate, un-runnable-here wiring | `plan/spec-finish-web-cli-ux.md` (work item added; re-homed 2026-07-16 -- the original destination `spec-followup-web-cli-ux.md` was retired in `49d1185ed` with `plan/learned/1094`, which does not mention nushell, so this was never done) | deferred |
| 2026-07-09 | spec-followup-web-cli-ux AC-5 | Subprocess-plugin web-route extensions (out-of-process plugins registering Go `http.Handler`s) | Architectural: ze plugins are subprocesses (JSON/text IPC), cannot register in-process handlers; AC-5 scoped to in-tree component pages by user decision | none (permanent exclusion; AC-5 covers in-tree pages) | cancelled |
| 2026-07-09 | spec-followup-web-cli-ux AC-1 tail | Control-hiding on purpose-built workbench pages (bgp peers/groups/policy, system, firewall, interfaces Add buttons via `workbench_table.html`/`WorkbenchTableData`) | Those page builders construct table data without the `*http.Request`, so `ReadOnly` cannot be threaded without a wider refactor; enforcement is already complete (route gate 403 + per-mutation authz), so this is UI polish only | `plan/spec-finish-web-cli-ux.md` (work item added; re-homed 2026-07-16 -- the original destination was retired in `49d1185ed`. Recorded there as UI polish only: enforcement is already complete, so this hides controls a read-only operator cannot use anyway) | deferred |

