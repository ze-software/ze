---
spec: data-store-maintenance-through-the-daemon
topic: CLI
feature: backup-and-restore
---

## Before

On a box where ze was running, `ze data rm`, `ze data write`, `ze data import`
and `ze config import` were refused, because the running daemon owns the
store and the store takes one writer. `ze doctor` could report a FlowSpec
announcement record left behind after the `ddos flowspec` section was removed,
and the only way to clear that warning was to stop ze, remove the record, and
start it again. A remote operator (SSH, REST, gRPC, MCP, the web) had no way to
check or salvage the store at all.

## After

The same local commands now work while ze runs: they send the change to the
daemon, which applies it through the store it already owns, and they print the
same lines as before. With ze stopped they act on the store directly, as they
always did. Five daemon commands carry the work on every surface:
`delete data key <key>`, `update data key <key> content <base64>`,
`update data config name <name> content <base64>`,
`request data repair path <file>` and `show data check`.

The two key commands reach runtime state only and refuse a key whose owning
plugin is running, naming the plugin restart instead. A config imported
through the daemon becomes a new version, and the daemon's own config goes
through the reload, so a config the reload refuses changes nothing. The base64
content is never written to the audit log, the accounting record, the SSH exec
log or the session transcript. One command carries at most 195,840 bytes; a
larger config is restored from a backup file on the daemon's host with
`request data restore path <file> config`.

The FlowSpec orphan report now names the two steps in order: withdraw the rule
with the command it prints, then remove the record with
`ze data rm meta/ddos-flowspec/announcement`, which works while ze runs.

An offline `ze data import`, `ze config import` or `ze data restore <file>
config` is now refused when a daemon stopped during a `commit confirmed`
window, because that daemon's next start would revert the change.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
