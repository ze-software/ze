---
kind: table
level:
stage:
---
Each row names where the expected outcome is declared, never the outcome itself, so the page stays its only statement.

| Scenario | Where the expected outcome is declared |
|----------|----------------------------------------|
| A plugin's process ends after it declared, once for each `failure-policy` value (`restart`, `ignore`, `fatal`) | `docs/architecture/api/process-protocol.md`, "Failure-Policy Declaration" |
| A plugin fails a startup stage after it declared | `docs/architecture/api/process-protocol.md`, "Failure-Policy Declaration" |
| A restarting plugin passes its restart bound, within the window and over the daemon's life | `docs/architecture/api/process-protocol.md`, "A restart is bounded" |
| The operator's `respawn` leaf disagrees with the declaration (`false` on a `restart` plugin, `true` on one that declared it must not restart) | `docs/architecture/api/process-protocol.md`, "The `respawn` leaf" |
| A plugin exits during the stop drain or during teardown | `docs/architecture/api/process-protocol.md`, "A stop ends every restart" |
| A signal arrives during the stop: in the config fence, in the drain, and after teardown began | `docs/architecture/hub-architecture.md`, "A signal during the stop follows one table" |
| A reload or commit arrives after shutdown began | `docs/architecture/hub-architecture.md`, "A commit that reaches the gate after shutdown began" |
| A plugin crashes during verify, crashes during apply, misses its deadline, or reports `broken` | `docs/architecture/config/transaction-protocol.md`, "Failure Modes" |
| A reload's context ends during a plugin's startup handshake, or a sibling fails while the plugin waits at the tier barrier | `docs/architecture/api/process-protocol.md`, "A failure during a reload means the plugin failed" |
| A plugin dispatches another plugin's command at startup | `docs/architecture/plugin/plugin-system.md`, "Startup phases and callbacks" |
