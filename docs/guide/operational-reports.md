# Operational Reports (`show warnings`, `show errors`)

Ze surfaces operator-visible issues from every subsystem through a single
**report bus** backed by two CLI commands. This page describes the operator
workflow. For the internal API and push contract used by subsystem authors,
see [`docs/architecture/api/commands.md`](../architecture/api/commands.md#operational-report-bus-ze-showwarnings-ze-showerrors).

## When to use which

Two severities, different meanings:

| You want to know... | Command | Severity |
|---------------------|---------|----------|
| "Is anything approaching a problem state right now?" | `ze show warnings` | `warning` (state) |
| "What just broke?" or "What went wrong recently?" | `ze show errors` | `error` (event) |

**Warnings** are state-based. They reflect a current condition. When the
condition resolves (prefix count drops back, peer is updated, etc.), the
warning disappears from the list automatically.

**Errors** are event-based. Each occurrence is a distinct entry. Errors
do not "resolve"; they age out of a bounded ring buffer. Operators
typically read errors chronologically from the top.

<!-- source: internal/core/report/report.go -- Severity contract in package godoc -->

## Quick start

```sh
ze show warnings                  # JSON of active warnings (newest-updated first)
ze show errors                    # JSON of recent error events (newest first)
```

Both commands work against a running daemon and return JSON. Example:

```json
{
  "warnings": [
    {
      "source": "bgp",
      "code": "prefix-threshold",
      "severity": "warning",
      "subject": "10.0.0.1/ipv4/unicast",
      "message": "ipv4/unicast prefix count 8500 at or above warning threshold 8000 (max 10000)",
      "detail": {
        "family": "ipv4/unicast",
        "count": 8500,
        "warning": 8000,
        "maximum": 10000
      },
      "raised": "2026-04-07T14:22:11Z",
      "updated": "2026-04-07T14:58:03Z"
    }
  ],
  "count": 1
}
```

<!-- source: internal/core/report/report.go -- Issue struct JSON shape -->

## Entry fields

Every entry, regardless of severity or source, has the same shape:

| Field | What it tells you |
|-------|-------------------|
| `source` | Which subsystem raised it (`bgp`, `config`, `iface`, ...). Use this to filter or join with other diagnostics. |
| `code` | Stable identifier of the condition or event. Scripts should match on `code`, not `message`. |
| `severity` | `"warning"` or `"error"`. Same as the verb used to query. |
| `subject` | What the issue is about: usually a peer address, file path, or transaction id. Used as the primary "which thing is affected" key. |
| `message` | A one-line human-readable description. Safe for banners and dashboards. |
| `detail` | Optional structured context. Depends on the producer. See the code-specific tables below. |
| `raised` | RFC 3339 UTC timestamp of the first observation. |
| `updated` | For warnings, the most recent re-raise. For errors, equals `raised`. |

`detail` is omitted entirely when empty, so consumers should treat its
absence and its presence as equivalent.

<!-- source: internal/core/report/report.go -- Issue struct with json tags, copyDetail empty normalization -->

## Day-one vocabulary

The BGP subsystem ships with five codes. Any future subsystem can add its
own by calling the push API documented in the architecture reference.

### Warnings (BGP)

| Code | Subject | Detail keys | Raised when | Cleared when |
|------|---------|-------------|-------------|--------------|
| `prefix-threshold` | `<peer-addr>/<afi>/<safi>` | `family`, `count`, `warning`, `maximum` | Per-family received prefix count crosses the configured warning threshold (set via `peer { session { family { ipv4/unicast { prefix { warning N ; } } } } }`) | Per-family count drops back below the threshold |
| `prefix-stale` | peer address | `updated` (ISO date of last refresh) | `peer { prefix { updated YYYY-MM-DD ; } }` is older than 180 days, checked at peer add time | Peer is removed, or peer is re-added with a fresher `updated` date (e.g., after a PeeringDB refresh) |

<!-- source: internal/component/bgp/reactor/session_prefix.go -- raisePrefixThreshold, clearPrefixThreshold, RaisePrefixStale, ClearPrefixStale -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- RaisePrefixStale call at AddPeer, ClearPrefixStale call at RemovePeer -->

### Errors (BGP)

| Code | Subject | Detail keys | Raised when |
|------|---------|-------------|-------------|
| `notification-sent` | peer address | `code`, `subcode`, `direction=sent` | Ze sends any BGP NOTIFICATION (RFC 4271 §4.5) to the peer. Includes all causes: operator teardown, hold-timer expiry, protocol error, prefix-max exceeded. |
| `notification-received` | peer address | `code`, `subcode`, `direction=received` | The peer sends a NOTIFICATION to ze. |
| `session-dropped` | peer address | `reason` (string: "connection lost" or "session closed") | An Established session ends WITHOUT a NOTIFICATION exchange (TCP loss, peer FIN, internal teardown without wire notification). Suppressed when a notification-sent or notification-received was already raised in the same session. |

<!-- source: internal/component/bgp/reactor/session_prefix.go -- reportCodeNotificationSent, reportCodeNotificationReceived, reportCodeSessionDropped constants -->
<!-- source: internal/component/bgp/reactor/peer_stats.go -- IncrNotificationSent/Received -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- FSM Established->Idle branch raises session-dropped when !notificationExchanged -->

## Common operator workflows

### "Is anything wrong right now?"

```sh
ze show warnings
```

If `count` is 0, no active operational concerns. If non-zero, look at each
entry's `source`, `code`, and `subject`. The `message` field gives you
enough context to act without needing to look up the producer.

### "What happened to peer 10.0.0.1 in the last few minutes?"

```sh
ze show errors | jq '.errors[] | select(.subject == "10.0.0.1")'
```

The ring buffer retains the most recent `errorCap` events (default 256).
For a specific peer's event history use the filter above.

### "What triggered my alert?"

If an alert fires on `ze_peer_notifications_received_total` or the
login banner count, both read from the same bus. Correlate with:

```sh
ze show errors | jq '.errors[] | select(.code == "notification-received")'
```

### "Clear a warning I already fixed"

Warnings are cleared automatically when their condition resolves.
For `prefix-threshold`: withdraw routes until the count drops below
threshold. For `prefix-stale`: update the peer config with a fresh
`updated` date and reload. There is no manual "acknowledge" command;
the bus is state-driven, not operator-driven.

## Login banner

The Ze CLI login banner reads the same report bus, filtered by source `bgp`.

- 0 warnings -> banner is silent.
- 1 warning -> banner shows the detail line.
- 2+ warnings -> banner shows `"N warnings"` and points to `show warnings`.

This is the same code path as `ze show warnings` (the banner just applies
a source filter and a formatting rule). One source of truth, one answer.

<!-- source: internal/component/bgp/config/loader.go -- collectPrefixWarnings -->

## Capacity limits

The bus is bounded to prevent any producer bug (infinite raise loop,
runaway string) from exhausting memory.

| Env var | Default | Maximum |
|---------|---------|---------|
| `ze.report.warnings.max` | 1024 active warnings | 10000 |
| `ze.report.errors.max` | 256 retained events | 10000 |

Operator values above the maximum are clamped with a warn log at daemon
startup. Zero or negative values fall back to the default.

At cap, new warnings evict the oldest-by-`Updated` entry. New errors
evict the oldest event in the ring buffer. Eviction is logged at warn
level at most once per minute per source to avoid log flooding.

<!-- source: internal/core/report/report.go -- newStore, maxWarningCap, maxErrorCap, evictOldestWarning -->

## Field length limits

Per-field length caps prevent multi-megabyte entries from reaching the
bus. Over-limit raise calls are rejected at the boundary and logged at
debug level.

| Field | Max bytes |
|-------|-----------|
| `source` | 64 |
| `code` | 64 |
| `subject` | 256 |
| `message` | 1024 |
| `detail` keys | 16 entries (no per-value length check; pass only primitive types) |

<!-- source: internal/core/report/report.go -- validFields, maxSourceLen, maxCodeLen, maxSubjectLen, maxMessageLen, maxDetailKeys -->

## See also

- [`docs/architecture/api/commands.md`](../architecture/api/commands.md#operational-report-bus-ze-showwarnings-ze-showerrors) -- full RPC contract, push API for subsystem authors, lock ordering, concurrency model.
- [`docs/features.md`](../features.md) -- where the report bus sits in the feature list.
- `internal/core/report/` -- the source of truth. Read the package godoc in `report.go` before adding a new producer.
