# Audit Trail

Ze keeps a local structured audit log for operator actions that change state or fail authentication. The log is append-only from the user's point of view: there is a query command, but no command to delete or truncate records.

## What Is Recorded

| Action | Surfaces |
|--------|----------|
| `config-commit` | SSH/CLI config editor, web config editor, REST config sessions, gRPC config sessions, gNMI Set |
| `config-discard` | SSH/CLI config editor, web config editor (the Discard button, the web terminal and the CLI bar, `discard <path>` included), REST config sessions, gRPC config sessions. A `rollback` in the SSH editor or the web terminal also writes one |
| `config-download` | web `GET /config/download` |
| `config-upload` | web `POST /config/upload`, once the uploaded config is applied |
| `daemon-reload` | SIGHUP, and `request reload` through the daemon on any surface |
| `data-key-change` | `delete data key` and `update data key` through the daemon on any surface, which `ze data rm` and `ze data write` also send while ze runs |
| `data-config-import` | `update data config` through the daemon on any surface, which `ze data import` also sends while ze runs. The detail holds the name and the placeholder in place of the content |
| `pki-private-key-export` | `show pki certificate name <name> bundle pem confirm` through the daemon on any surface. Records `success`, `denied` (authorization refusal or missing `confirm`), and `error` outcomes with redacted command detail and zero private-key bytes |
| `auth-fail` | SSH, web, REST, gRPC, gNMI, MCP |

Each entry has `timestamp`, `actor`, `remote-addr`, `surface`, `action`, `detail`, and `outcome` fields. `detail` carries the config diff for commit and discard records when Ze has one.

The web terminal's `commit confirmed` and the CLI bar's `commit now` record
one successful `config-commit` entry after publication. The detail is the same
path-qualified, secret-masked review shown before committing. A refused commit
keeps the pending changes and records no successful commit entry.

An ordinary daemon command records an entry only when it succeeds. A refused or
failed ordinary command records nothing. Commands that register
`RequireExplicitAuthz` (such as `pki-private-key-export`) also record `denied`
when authorization or mandatory argument checks refuse the request, and `error`
when the handler refuses it. In every case, `detail` holds only the redacted
command line (never the response body), with credential values, `content`
payloads, and any private-key PEM blocks replaced by `<redacted>`.

A discard record's `detail` lists only the changes that discard dropped. `discard <path>` keeps your changes outside that path, so they are not in its record, and they appear in the commit record that later applies them. One exception: the SSH editor's diff lines carry no path, so when two places hold the same pending line (two peers both changed to the same hold time, say) and `discard <path>` drops one of them, the SSH record can also name the line you kept. The web records are exact. A whole discard run with nothing pending still writes a record, with an empty `detail`, on every surface, so the trail shows every discard an operator ran. A rollback record's `detail` is the rollback itself (the command in the web terminal, `rollback <backup>` in the SSH editor), a download record's is the file name, and an upload record's is `uploaded configuration`.

Before a whole web discard, Ze opens the user's saved changes to prepare its
audit detail, including after idle eviction. If that open fails, discard stops
without dropping changes or recording a successful action. For a whole web
discard, an empty detail means a successful read found no pending intent for
that user.

<!-- source: internal/core/audit/audit.go -- Entry fields and action constants -->
<!-- source: internal/component/web/handler_config_commit.go -- Discard button and commit audit recording -->
<!-- source: internal/component/web/handler_config_transfer.go -- config-download and config-upload audit recording -->
<!-- source: internal/core/audit/dropped.go -- DroppedDiff -->
<!-- source: internal/component/cli/model_commands_commit.go -- cmdDiscardSession, cmdRollback -->
<!-- source: internal/component/web/cli_terminal.go -- terminalAuditContext, terminalAuditSucceeded, runWebDiscard -->
<!-- source: internal/component/web/cli_commit_audit_paths_test.go -- TestBrowserCommandCommitAudit -->
<!-- source: internal/component/web/cli.go -- HandleCLICommandWithAuthorizerAndAudit, dispatchCLICommand, discardAuditDetail -->
<!-- source: internal/component/api/rest/server.go -- REST config and auth-fail audit recording -->
<!-- source: internal/component/api/grpc/server.go -- gRPC config and auth-fail audit recording -->
<!-- source: internal/component/gnmi/server.go -- gNMI auth-fail audit recording -->
<!-- source: internal/component/gnmi/set.go -- gNMI config-commit audit recording -->
<!-- source: internal/component/mcp/audit.go -- MCP auth-fail audit recording -->
<!-- source: internal/component/plugin/server/command.go -- recordCommandAudit, recordCommandAuditOutcome -->
<!-- source: internal/core/audit/action_pki_private_key.go -- ActionPKIPrivateKeyExport -->
<!-- source: internal/component/config/storage/cli/register_data_rpc.go -- data-key-delete AuditAction -->

## Storage

For a daemon started from a real config file, the audit log is stored next to that file as `<config-base>.audit.jsonl`. For stdin configs (`ze -`) and web-only test mode, Ze uses an in-memory audit log.

The in-memory query cache keeps the newest records up to the configured retention limit in code. The default is 10,000 entries, with accepted bounds from 100 to 100,000.

Each record rewrites the file. When a write fails (a full disk, a directory ze
cannot write), the action still goes ahead and the record stays in memory;
`show doctor` reports `doctor-audit-record-failed` with the file, the error,
the time and how many writes failed, whichever surface wrote. The next write
that lands rewrites the file with every record still held, and the row clears.

<!-- source: cmd/ze/hub/audit.go -- defaultAuditPath and openAuditLog, which publishes the log -->
<!-- source: internal/core/audit/audit.go -- retention bounds -->
<!-- source: internal/core/audit/write_outcome.go -- WriteFailure, PublishedWriteFailure -->
<!-- source: internal/component/doctor/checks_audit.go -- checkAuditRecordFailed -->
<!-- source: internal/core/audit/store.go -- JSON-lines persistence -->

## Querying

Use `show audit` from the daemon CLI or `ze show audit` from the shell:

```
ze show audit
ze show audit action config-commit
ze show audit actor alice
ze show audit surface web count 20
ze show audit since 2026-05-24T10:00:00Z until 2026-05-24T11:00:00Z
```

The response is structured data with `entries` and `count`. Filters can be combined. Time filters use RFC 3339 timestamps.

<!-- source: internal/component/cmd/show/audit.go -- show audit filters -->

## Authorization Notes

Web config mutations now use the same profile authorizer as SSH, API, and MCP command dispatch. A read-only user can still open `/show/` pages, but `POST /config/set/`, add, delete, rename, commit, and discard are denied before the draft is changed.

REST and gRPC without a configured token or per-user authenticator use the built-in `api` identity in read-only mode. Read commands work. Command writes and config sessions return 403. Likewise, unauthenticated loopback gNMI allows `Capabilities`, `Get`, and `Subscribe`, and refuses `Set` with `PermissionDenied`.

<!-- source: internal/component/web/handler_config.go -- authorizeWebConfigMutation -->
<!-- source: internal/component/api/engine.go -- read-only caller enforcement -->

## Fleet Audit Trail

A Ze hub maintains a centralized, append-only fleet audit log in its live ZeFS
store. Each entry carries a monotonic sequence number (`seq`), UTC `event-time`
and `receipt-time`, `type`, `device`, `actor`, `surface`, and `outcome`.
Entries also carry optional `version` and `previous-version` hashes, safe
`diagnostic-code`, `summary`, and bounded `metadata`. Entries store version
hashes and safe metadata only. Raw configuration text and secrets never enter
the log.

### Recorded Fleet Event Types

| Event Type | When Recorded |
|------------|---------------|
| `device.connect` | A managed device completes TLS and token authentication on the hub |
| `device.disconnect` | An authenticated managed device session closes |
| `config.fetch` | The hub serves a configuration (full or diff) in answer to `config-fetch` |
| `config.push` | The hub delivers a `config-changed` notice to a connected device |
| `config.ack` | A managed device acknowledges that it applied a configuration |
| `config.reject` | A managed device refuses a configuration or reports that it holds a local edit |
| `config.held` | The hub stores a held configuration copy uploaded by a holding device |
| `divergence.adopt` | An operator adopts a device's held configuration on the running hub |
| `divergence.revert` | An operator records a revert decision for a holding device on the running hub |
| `fleet.disable` | A managed device records `request fleet disable` and delivers its receipt on reconnect |
| `fleet.enable` | A managed device records `request fleet enable` and delivers its receipt on reconnect |
| `fleet.receipt.gap` | A managed device reports that its offline receipt outbox overflowed before it reconnected |

### Querying Fleet Audit Entries

Run `show fleet audit` on any hub command surface (CLI, SSH, REST, gRPC, MCP,
or the web command line). The command returns entries newest first and supports
four command-specific pipe filters:

| Pipe Filter | Meaning |
|-------------|---------|
| `device <name>` | Keep only entries for one exact managed device name |
| `type <event-type>` | Keep only entries matching one dot-separated event type |
| `since <RFC3339>` | Keep only entries with `event-time` at or after this RFC 3339 instant |
| `until <RFC3339>` | Keep only entries with `event-time` at or before this RFC 3339 instant |

Examples:

```
ze show fleet audit
ze show fleet audit | device edge-01
ze show fleet audit | type config.fetch
ze show fleet audit | since 2026-11-28T00:00:00Z | until 2026-11-28T12:00:00Z
ze show fleet audit | device edge-01 | json compact
```

### Per-Device Authorization

To read fleet audit rows, a user profile must allow both `show fleet audit` and
`show fleet device name <device>` for each device the user can inspect. Ze
enforces per-device authorization before it applies pipe filters or counts. A
restricted user never sees rows for unauthorized devices or hub-wide events.
Filtering by a denied device returns the same empty result as an unknown device.

### Retention Configuration

The hub keeps the newest 10,000 fleet audit entries by default and prunes the
oldest entries as new batches commit. Configure the ceiling (from `100` to
`100000`) under `bus hub audit`:

```
bus {
    hub {
        audit {
            max-entries 25000;
        }
    }
}
```

Reloading the hub updates the ceiling live and trims older entries on the next
publication. Direct live mutation of fleet audit keys (`meta/fleet/audit/*`,
`meta/managed/audit-receipts`, and `meta/managed/audit-intent`) through
`update data key` or `delete data key` is refused while the daemon runs.

### Offline Fleet Disable and Enable Receipts

An operator can run `request fleet disable` or `request fleet enable` while a
device cannot reach its hub. The device records each transition in a bounded
local outbox (up to 1,024 receipts) guarded by a recoverable write intent. On
the next authenticated reconnect, the device replays pending receipts over
`fleet-audit-receipts` in batches of up to 32 entries. The hub stamps each row
with `actor: "device:<name>"` and records the local actor and surface in
`metadata`. It commits the entries and device cursor atomically and ignores
duplicate retries if an acknowledgement is lost in flight.

### Doctor Diagnostics

Audit write errors and queue saturation never block configuration delivery.
Run `show doctor` to inspect audit health:

- `doctor-managed-audit-write-failed` (hub): persisting audit pages or manifest
  failed. Prior published rows remain readable until a later write succeeds.
- `doctor-managed-audit-dropped` (hub): the hub's bounded audit queue dropped
  events under overload, or a device reported an offline receipt outbox gap.
- `doctor-managed-audit-corrupt` (hub): unreadable audit keys were skipped at
  startup while healthy pages remained readable.
- `doctor-managed-audit-receipt-gap` (device): the device's offline receipt
  outbox overflowed its 1,024-entry ceiling or failed to decode.

<!-- source: internal/component/managed/cmd_audit.go -- registerAuditCommands, handleFleetAudit, fleetAuditRow -->
<!-- source: internal/component/managed/yang/ze-managed-cmd.yang -- container audit -->
<!-- source: internal/component/plugin/server/audit.go -- ManagedAuditStore, FleetAuditEntry, FleetAuditQuery -->
<!-- source: internal/component/managed/audit_receipts.go -- executeManagedModeChange, drainAuditReceipts -->
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedAuditWrite, checkManagedAuditDropped, checkManagedAuditCorrupt -->
<!-- source: internal/component/managed/doctor.go -- managedAuditReceiptsDoctorCheck, checkManagedAuditReceipts -->
