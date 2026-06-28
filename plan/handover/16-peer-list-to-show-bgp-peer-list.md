# Handover: Remove bare read/action commands that bypass `show`/`request` verbs

## Problem

Several read-only commands are exposed as bare CLI verbs instead of living under `show`.
Several action commands are exposed as bare CLI verbs instead of living under `request`.
This violates the grammar rule that read operations use `show` and actions use `request`.
Users should type `show bgp summary`, not `summary`. The bare forms should only exist
as plugin API commands (ExaBGP compat), not as user-facing CLI paths.

## Commands to Remove from CLI Grammar

| Bare command | Correct `show` equivalent | YANG file | Issue |
|---|---|---|---|
| `peer list` | `show bgp peer list` (exists, line 37) | ze-peer-cmd.yang:125 | Duplicate |
| `summary` | `show summary` (exists, line 21) | ze-peer-cmd.yang:9 | Duplicate |
| `log levels` | `show log levels` (does NOT exist yet) | ze-cli-log-cmd.yang | Missing show path |
| `log recent` | `show log recent` (does NOT exist yet) | ze-cli-log-cmd.yang | Missing show path |
| `metrics values` | `show metrics values` (does NOT exist yet) | ze-cli-metrics-cmd.yang | Missing show path |
| `metrics list` | `show metrics list` (does NOT exist yet) | ze-cli-metrics-cmd.yang | Missing show path |

| `cache list` | `show cache` | ze-cli-cache-cmd.yang | Mixed: needs split |
| `cache retain <id>` | `request cache retain <id>` | ze-cli-cache-cmd.yang | Action without verb |
| `cache release <id>` | `request cache release <id>` | ze-cli-cache-cmd.yang | Action without verb |
| `cache expire <id>` | `request cache expire <id>` | ze-cli-cache-cmd.yang | Action without verb |
| `cache forward <id>` | `request cache forward <id>` | ze-cli-cache-cmd.yang | Action without verb |
| `log set <sub> <lvl>` | `set log <sub> <lvl>` | ze-cli-log-cmd.yang | Verb after noun |

Also violations (mutations without `request` verb, and naming):

| Bare command | Correct form | YANG file |
|---|---|---|
| `peer <sel> teardown` | `request peer teardown <sel>` | ze-peer-cmd.yang:132 |
| `peer <sel> pause` | `request peer pause <sel>` | ze-peer-cmd.yang:140 |
| `peer <sel> resume` | `request peer resume <sel>` | ze-peer-cmd.yang:148 |
| `peer <sel> flush` | `request peer flush <sel>` | ze-peer-cmd.yang:156 |
| `peer <sel> plugin session ready` | `request peer plugin session ready <sel>` | ze-peer-cmd.yang:172 |
| `peer <sel> refresh` | `request peer refresh <sel>` | ze-refresh-cmd.yang:12 |
| `peer <sel> borr` | `request peer borr <sel>` | ze-refresh-cmd.yang:21 |
| `peer <sel> eorr` | `request peer eorr <sel>` | ze-refresh-cmd.yang:30 |
| `peer <sel> clear soft` | `request peer clear soft <sel>` | ze-refresh-cmd.yang:43 |
| `daemon shutdown` | `request daemon shutdown` | ze-system-cmd.yang |
| `daemon reboot` | `request daemon reboot` | ze-system-cmd.yang |
| `daemon reload` | `request daemon reload` | ze-system-cmd.yang |
| `interface create-dummy <name>` | `create interface dummy <name>` | ze-iface-cmd.yang |
| `interface create-veth <name>` | `create interface veth <name>` | ze-iface-cmd.yang |
| `interface create-bridge <name>` | `create interface bridge <name>` | ze-iface-cmd.yang |
| `interface delete <name>` | `delete interface <name>` | ze-iface-cmd.yang |
| `interface up <name>` | `request interface <name> up` | ze-iface-cmd.yang |
| `interface down <name>` | `request interface <name> down` | ze-iface-cmd.yang |
| `interface mtu <name> <bytes>` | `request interface <name> mtu <bytes>` | ze-iface-cmd.yang |
| `interface mac <name> <addr>` | `request interface <name> mac <addr>` | ze-iface-cmd.yang |
| `interface addr-add <name> <pfx>` | `request interface <name> address add <pfx>` | ze-iface-cmd.yang |
| `interface addr-del <name> <pfx>` | `request interface <name> address delete <pfx>` | ze-iface-cmd.yang |
| `interface unit-add <parent> <vid>` | `request interface <parent> unit add <vid>` | ze-iface-cmd.yang |
| `interface unit-del <parent> <vid>` | `request interface <parent> unit delete <vid>` | ze-iface-cmd.yang |
| `interface migrate <src> <dst>` | `request interface <src> migrate <dst>` | ze-iface-cmd.yang |
| `l2tp tunnel <id> teardown` | `request l2tp tunnel teardown <id>` | ze-l2tp-cmd.yang:192 |
| `l2tp tunnel teardown-all` | `request l2tp tunnel teardown all` | ze-l2tp-cmd.yang:200 |
| `l2tp session <id> teardown` | `request l2tp session teardown <id>` | ze-l2tp-cmd.yang:213 |
| `l2tp session teardown-all` | `request l2tp session teardown all` | ze-l2tp-cmd.yang:221 |
| `subscribe` | `request subscribe` | ze-cli-subscribe-cmd.yang:8 |
| `unsubscribe` | `request unsubscribe` | ze-cli-subscribe-cmd.yang:17 |
| `commit <action>` | `request commit <action>` | ze-cli-commit-cmd.yang:8 |

Reads under wrong verb (not `show`):

| Bare command | Correct form | YANG file |
|---|---|---|
| `daemon status` | `show daemon status` | ze-system-cmd.yang |
| `l2tp` (reads: tunnels, sessions, statistics, config, listeners, observer, ...) | `show l2tp ...` | ze-l2tp-cmd.yang |
| `l2tp-health` | `show l2tp health` | ze-l2tp-cmd.yang |
| `pppoe` (reads: sessions, statistics, interfaces) | `show pppoe ...` | ze-pppoe-cmd.yang |

The `iface` abbreviation is renamed to `interface` throughout, and hyphenated
compound actions (`addr-add`, `addr-del`, `unit-add`, `unit-del`, `create-dummy`,
`create-veth`, `create-bridge`, `teardown-all`) are split into full words.

NOT violations:
- `set log <sub> <level>` (uses `set` verb, correct for config mutation)
- `delete bgp peer <sel>` (uses `delete` verb, correct)
- `clear dns cache` / `clear interface counters` / `clear vpn ipsec sa` (uses `clear` verb, correct)
- `update system firmware ...` (uses `update` verb, correct)
- `monitor ...` (uses `monitor` verb, correct for streaming output)

## Part 1: `peer list` (duplicate, remove)

### Current State

`ze-peer-cmd.yang` lines 118-181 define a top-level `container peer` with:
- `peer > list` (line 125) -- duplicates `show > bgp > peer > list` (line 37)
- `peer > teardown` (line 132) -- mutation, correct without `show`
- `peer > pause` (line 140) -- mutation, correct
- `peer > resume` (line 148) -- mutation, correct
- `peer > flush` (line 156) -- mutation, correct
- `peer > plugin > session > ready` (line 172) -- lifecycle signal, correct

Only `peer > list` is the problem. The others are actions.

**Handler:** `ze-bgp:peer-list` in `internal/component/bgp/plugins/cmd/peer/peer.go:37`.
Both YANG paths dispatch to the same handler.

**API RPC:** `ze-bgp-cmd-peer-api.yang:8` declares `rpc peer-list`. This is the plugin API
surface (ExaBGP compat). Plugins send `peer * list` as text commands. This must continue
to work as an API/plugin command, just not as a user-facing CLI verb.

### Users of `peer list` / `peer * list`

#### Tests using `ze cli -c "peer list"` (SSH CLI path, MUST migrate):
- `test/plugin/ssh-user-login-yang.ci` (lines 101, 111, 132, 146)
- `test/plugin/authz-allow.ci` (lines 127, 140)
- `test/plugin/authz-default.ci` (line 113)
- `test/plugin/ssh-pubkey-auth.ci` (lines 109, 129)
- `test/plugin/tacacs-singleconnect.ci` (line 105)

These should become `ze cli -c "show bgp peer list"`.

#### Tests using dispatch `peer * list` or `peer list` (plugin API path, keep working):
- `test/plugin/api-peer-list.ci` (line 37)
- `test/plugin/api-peer-remove.ci` (lines 37, 52)
- `test/plugin/cli-run-command-peer.ci` (line 37)
- `test/plugin/config-edit-ssh.ci` (line 54)
- `test/plugin/config-edit-ssh-session.ci` (dispatch `peer list`)
- `test/plugin/elicitation-accept.ci` (line 18)
- `test/plugin/task-identity-scope.ci` (line 6)

These go through `dispatch-command` / plugin API, not the CLI grammar.

#### RBAC profiles referencing `peer list` as a match pattern:
- `test/parse/rbac-profile.ci` (line 51)
- `test/plugin/authz-allow.ci` (line 68)
- `test/plugin/authz-deny.ci` (line 55)
- `test/plugin/authz-default.ci` (line 57)

These RBAC `match "peer list"` entries must become `match "show bgp peer list"`.

#### ExaBGP compat:
- `test/exabgp-compat/etc/run/api-v6-comprehensive.run` (line 353)

Plugin text command API, dispatches directly to the RPC. Verify it still routes after removal.

### Action

1. Remove `container list` from `container peer` in `ze-peer-cmd.yang` (lines 125-130).
2. Verify `show > bgp > peer > list` still works (already exists).
3. Update `ze cli -c "peer list"` tests to `ze cli -c "show bgp peer list"`.
4. Update RBAC match patterns from `peer list` to `show bgp peer list`.
5. Verify plugin dispatch still routes `peer * list` to the RPC.

## Part 2: `summary` (duplicate, remove)

### Current State

`ze-peer-cmd.yang` line 9: top-level `container summary` with `ze:command "ze-bgp:summary"`.
Line 21: `show > summary` with the same command. Exact duplicate.

### Users

```
grep -rn '"summary"' test/ --include="*.ci"
```

Key hits:
- `test/plugin/api-bgp-summary.ci` (dispatch `summary`)
- `test/plugin/tacacs-singleconnect.ci` (`ze cli -c "summary"`)
- `test/plugin/cli-show-summary.ci` (dispatch `summary`)

### Action

1. Remove top-level `container summary` from `ze-peer-cmd.yang` (lines 9-15).
2. `show > summary` (line 21) already exists with same handler.
3. Update `ze cli -c "summary"` to `ze cli -c "show summary"`.
4. Check if dispatch `summary` routes through YANG (same risk as peer list).

## Part 3: `log` commands (reads missing show, set has wrong verb order)

### Current State

`ze-cli-log-cmd.yang` defines under a `log` container:
- `log > levels` (ze-bgp:log-levels) -- read: shows current levels
- `log > recent` (ze-bgp:log-recent) -- read: shows recent entries
- `log > set` (ze-bgp:log-set) -- mutation: changes level

No `show > log > levels` or `show > log > recent` exists anywhere.
`log set` puts the verb after the noun; should be `set log`.

### Before / After

```
log levels                           → show log levels
log recent                           → show log recent
log set <subsystem> <level>          → set log <subsystem> <level>
```

### Action

1. Create `show > log > levels` and `show > log > recent` in the appropriate show YANG
   (either ze-cli-show-cmd.yang or a new ze-cli-log-show-cmd.yang owned by the log package).
2. Wire to same handlers (`ze-bgp:log-levels`, `ze-bgp:log-recent`).
3. Create `set > log` wired to `ze-bgp:log-set`.
4. Remove all three from the bare `log` container in ze-cli-log-cmd.yang.
5. Update tests using `ze cli -c "log levels"` / `ze cli -c "log recent"` / `ze cli -c "log set ..."`.
6. Update RBAC match patterns if any reference `log set`.

### Users

```
grep -rn '"log levels"\|"log recent"' test/ --include="*.ci"
```

- `test/plugin/cli-log-show.ci`
- Any RBAC profiles matching `log levels` or `log recent`

## Part 4: `metrics values` / `metrics list` (missing show path, create + remove)

### Current State

`ze-cli-metrics-cmd.yang` defines under a `metrics` container:
- `metrics > values` (ze-bgp:metrics-values) -- read: dump Prometheus text
- `metrics > list` (ze-bgp:metrics-list) -- read: list metric names

No `show > metrics > values` or `show > metrics > list` exists.

### Action

1. Create `show > metrics > values` and `show > metrics > list` in the show YANG.
2. Wire to same handlers.
3. Remove bare `metrics > values` and `metrics > list`.
4. Update tests.

### Users

```
grep -rn '"metrics values"\|"metrics list"' test/ --include="*.ci"
```

- `test/plugin/cli-metrics-show.ci`
- `test/plugin/cli-metrics-show-deep.ci`
- `test/plugin/cli-metrics-list.ci`
- `test/plugin/cli-metrics-list-deep.ci`
- `test/plugin/cli-metrics-plugin-health.ci`

## Part 5: `cache` (mixed read/action, needs split)

### Current State

`ze-cli-cache-cmd.yang` defines a single `container cache` with `ze:command "ze-bgp:cache"`.
One handler dispatches all subactions via args: `cache list`, `cache retain <id>`,
`cache release <id>`, `cache expire <id>`, `cache forward <id>`.

- `cache list` is a read: should be `show cache`
- `cache retain/release/expire/forward` are actions: should be `request cache <action>`

### Before / After

```
cache list                           → show cache
cache retain <id>                    → request cache retain <id>
cache release <id>                   → request cache release <id>
cache expire <id>                    → request cache expire <id>
cache forward <id>                   → request cache forward <id>
```

### Action

1. Split the YANG into two paths:
   - `show > cache` wired to `ze-bgp:cache` with implicit `list` action (read)
   - `request > cache > retain/release/expire/forward` (actions)
2. Split the single handler or add arg routing so `show cache` maps to list internally.
3. The `request` verb container may not exist yet in the YANG tree; create it if needed.
4. Update tests.

### Users

```
grep -rn '"cache' test/ --include="*.ci"
```

- `test/plugin/api-cache-ops.ci`
- `test/plugin/api-cache-forward.ci`

## Shared Risk: Plugin Dispatch Routing

All four parts share the same risk: the `dispatch-command` path used by plugins
(`api._call_engine('ze-plugin-engine:dispatch-command', {command: '...'})`) may route
through the YANG CLI tree. If so, removing bare commands breaks plugin dispatches.

**Investigation needed:** Read `internal/component/plugin/server/dispatch.go` (or wherever
dispatch-command resolves). Determine if it:
- (a) Uses the YANG tree (removing the node breaks dispatch)
- (b) Uses the RPC registry directly (removing the YANG node is safe for dispatch)
- (c) Tries YANG first, falls back to RPC registry (safe but verify)

If (a): plugin dispatches must be rewritten to use the `show` path, OR the dispatch
system needs a fallback to the RPC registry for API compatibility.

## Execution Order

1. Investigate dispatch routing (shared risk, blocks all parts)
2. Part 1: `peer list` (simplest, duplicate exists)
3. Part 2: `summary` (same pattern)
4. Part 3: `log` reads (needs new show paths)
5. Part 4: `metrics` reads (needs new show paths)
6. Part 5: `cache` (needs handler split)

## Files to Change

- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang`
- `internal/component/cmd/log/schema/ze-cli-log-cmd.yang`
- `internal/component/cmd/metrics/schema/ze-cli-metrics-cmd.yang`
- `internal/component/bgp/plugins/cmd/cache/schema/ze-cli-cache-cmd.yang`
- Show YANG (ze-cli-show-cmd.yang or new owner modules for log/metrics/cache)
- Cache handler split (if needed)
- All test files listed above
- Possibly `internal/component/plugin/server/` dispatch routing
