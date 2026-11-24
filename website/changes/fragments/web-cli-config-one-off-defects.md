---
spec: web-cli-config-one-off-defects
topic: Web UI
found-by: review
---

## Before
Opening the web interface landed on the experimental Workbench instead of the graphical YANG editor, multi-valued and freeform settings lacked graphical editors, `/config/edit/` redirects did not preserve the active UI mode or per-user candidate, and offline `.zefs` backups could not be uploaded, edited, created from the running system, or downloaded through the browser. Corrupted pending change files required filesystem access to recover, bare list paths such as `show bgp peer` returned empty output or dumped the entire configuration, `compare` could diff against a stale baseline, and path-less diffs could collapse identical lines across sibling blocks.

Across the CLI, SSH, and configuration layers, plain `ssh <host> <command>` sessions received protocol answer frame lines on stderr after JSON or text output, YAML output could emit unquoted indicators or reserved words, set-meta saves dropped configuration comments, OSPF silently skipped malformed keyed lists, leaf-list `range` and `length` constraints were unenforced, `ze:sensitive` values could be echoed in `ze config set` or web terminal feedback, stripped builds warned about missing command summaries on `request` and `show policy`, and `ze init --sha256` completed as a boolean flag instead of taking a value.

## After
The graphical YANG editor is now the default web interface (with Workbench still selectable as an experimental alternative), including multi-valued and freeform editors, advanced structural/confirmed-commit actions, `/config/edit/` mode-preserving routing, lock-safe per-user tree snapshots, browser CLI streaming and persistent masked command history, explicit browser recovery for unreadable pending change files, an independent browser workspace for uploading, editing, and downloading `.zefs` backups, full system-backup creation from `/backups/`, and live commit-time activation of `environment backup directory`.

In the CLI, SSH, and configuration layers, `show <list>` renders the list's entries or an explicit empty notice, `compare` reads the live committed configuration, diffs render path-qualified lines, set and set-meta serialization preserves all configuration comments, OSPF fails closed on non-map keyed lists while accepting empty areas, leaf-list `range`/`length` and `uses` augments/`deviate add` constraints are enforced, sensitive values are masked in `ze config set` and web terminal feedback, plain OpenSSH exec receives clean stdout and JSON with zero answer frame lines on stderr (framing is opt-in via `ZE_ANSWER_FRAME=1`), YAML output quotes and escapes reserved literals and indicators cleanly, attached console completion offers daemon paths instead of offline roots, stripped builds carry complete `request` and `show policy` summaries, and `ze init --sha256` completes as a value-taking flag.

## Corrects
none

## Exposure
Deployments exposing the web terminal or `ze config set` could echo sensitive leaf values in command feedback before masking was enforced across display paths; deployments using concurrent web editor requests or concurrent SSH editor commits could race shared candidate trees until per-user snapshots and in-flight commit fencing were added. Backup workspace uploads and system snapshots require edit-capable authenticated same-origin requests and isolate private per-account files from the live store namespace.

## Upstream
none

## Asked by
none
