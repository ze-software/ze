# Managed Configuration

**Status:** Implemented. The client half (outbound connection, fetch, cache, reconnect) and the
hub-serving half are both wired. The hub serves managed clients through a dedicated managed-config
TLS listener that reuses the shared auth + MuxConn primitives (see "Architecture" below).
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedServer (dedicated listener + serving loop) -->
<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer (wires ManagedServer from hub config) -->

**Purpose:** Centralised configuration over TLS, with a local persistent cache for partition resilience.

---

## Design decisions

These decisions describe the approved designs for managed listener reload and post-release fleet web editing.

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener." | Keeping a changed managed certificate until the next restart; refusing a reload because it changes the managed certificate; rebinding the managed listener or closing its sessions to change its certificate; keeping the managed certificate anywhere but the configuration's `pki` block | plan/release/features/spec-fleet-1-device-registry.md | 2026-11-23 |
| Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface. | Requiring first contact or a pre-existing file for that web operation, or letting a fleet device editor write the hub configuration | plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md | 2026-11-25 |

---

## Vision

A managed client's config contains everything: BGP sessions, plugin settings, AND the hub connection info. The `bus { hub { client <name> { host; port; secret } } }` block declares who the client is and where its hub lives (client blocks use `host` for the remote address). After the first boot (provisioned via `ze init`), the cached config is self-describing -- `ze start` reads it and knows everything.

Every ze instance has at least one `server` block for local plugins and SSH. The hub infrastructure is universal -- managed config is just one more use of it.

---

## Hub Block Design

### Unified Named Blocks

All hub configuration uses explicit `server` and `client` keywords under `bus { hub { } }`:

| Keyword | Direction | Has `ip`/`host`+`port` | Purpose |
|---------|-----------|------------------------|---------|
| `server <name>` | Accepts connections | `ip`+`port` (listen address) | Local plugins, SSH editor, remote managed clients |
| `client <name>` (under `server`) | N/A | N/A | Declares an accepted remote client with its secret |
| `client <name>` (at hub level) | Connects outbound | `host`+`port` (remote address) | Fetch config from a remote hub |

### Examples

**Standalone instance** (local plugins only):

```
bus {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-...";
        }
    }
}
```

**Central hub** (serves local plugins AND remote managed clients):

```
bus {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-...";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret-...";
            client edge-01 { secret "..."; }
            client edge-02 { secret "..."; }
        }
    }
}
```

**Managed client** (local plugins + remote config):

```
bus {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-...";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "...";
        }
    }
}
```

### Key Rules

- Every ze instance has at least one `server` block (for local plugins and SSH)
- A `server` block listens; a hub-level `client` block connects outbound
- `client` blocks nested under `server` declare accepted remote clients (with per-client secrets)
- Hub-level `client` blocks declare outbound connections to remote hubs
- A `server` block uses `ip`+`port` for listening; a `client` block uses `host`+`port` for connecting
- Multiple `server` blocks allowed (different secrets for different plugin groups)
- The plugin acceptor binds the FIRST `server` block and nothing else. The managed listener binds every block that declares `client` entries. One address cannot carry both, so the block that serves managed clients needs its own port. The acceptor takes the address first. The managed listener is then skipped with an Error log. Every managed client that dials it is refused by an acceptor which routes only to a plugin waiter. `ze doctor` reports the collision under `doctor-hub-managed-collision`
- The client name in `client edge-01 { }` IS the client's identity

<!-- source: internal/component/plugin/acceptor.go -- hubAcceptorServer -->
<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer -->
<!-- source: internal/component/plugin/doctor/check_managed_listener.go -- diagnoseManagedListener -->

---

## Architecture

### Dedicated managed listener (reuses auth + transport)

The hub already provides the auth and transport primitives:
- Per-client token authentication (`#0 auth {"token":"...","name":"..."}`) via `AuthenticateWithLookup`
- MuxConn multiplexed RPCs (`#id verb [json]\n`)
<!-- source: internal/component/plugin/ipc/tls.go -- AuthenticateWithLookup, StartListeners -->
<!-- source: pkg/plugin/rpc/ -- MuxConn -->

Managed clients do NOT flow through the plugin `PluginAcceptor`: that acceptor routes an
authenticated connection to an `ExpectPlugin(name)` waiter, and only engine-spawned plugin
processes register as waiters, so a managed client (which connects inbound at any time, with no
waiter) would be dropped. Instead the hub runs a **dedicated managed-config TLS listener** bound
to each `server` block that declares `client` entries. It reuses `AuthenticateWithLookup` (per-
client secret) and MuxConn, so no new auth or wire protocol is introduced.
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedServer -->

Managed configuration adds:
- Per-client secrets in `server` blocks (instead of one shared secret)
- `config-fetch` and `config-changed` RPCs answered by the managed listener
- Client configs stored as entries in the hub's live tree

The daemon retains the managed server and calls its synchronous `Stop` before
returning its borrowed store. The outbound client also has a cancel-and-join
owner. Shutdown waits for `CheckManaged` and `CommitOver` callbacks already using
storage, including candidate writes before reload admission. A pending full-store
restore cannot replace the store while these managed operations still use it.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/component/plugin/server/managed_serve.go -- Stop -->
<!-- source: internal/component/managed/client.go -- RunManagedClient -->

### Device registry

The managed server keeps one record per device name across its serving
blocks. An authenticated session supplies its start time and source address.
The hub records the end of the session and the latest config fetch and
acknowledgement. The `show fleet devices` table and the
`show fleet device name <name>` document join these facts with the current
client declaration, including a declared device before its first connection.
Removing a declaration before the device has any history drops its row.
Devices with retained history remain available by name. Removing or restoring
a declaration preserves unreadable record bytes until an authenticated event
replaces them.
<!-- source: internal/component/plugin/server/managed_registry.go -- ManagedDevice, ManagedDeviceRecord, ManagedRegistrySnapshot -->

The current hub version is the hash of the configuration file the hub serves
that device. The first registry read computes it, and the storage observer
invalidates only the changed client's hash. The reader hashes outside the
session lock. The device table lists the held-record keys once to determine
which devices have diverged. A named detail reads only that device's
configuration and held key. A held key counts even when its contents cannot
be decoded. Name completion reads the sorted names from memory.
<!-- source: internal/component/plugin/server/managed_registry_versions.go -- registrySnapshot, registryDevice, currentConfigVersion, MarkManagedConfigChanged -->
<!-- source: internal/component/plugin/server/managed_registry.go -- ManagedDeviceNames -->
<!-- source: internal/component/plugin/server/managed_held.go -- HeldClients -->
<!-- source: internal/component/managed/cmd_devices.go -- rowForDevice, deviceConfigState -->

Each `ManagedDeviceRecord` also holds optional pointers to immutable
`ManagedInventorySnapshot` and `ManagedHealthSnapshot` values, each pairing a
validated report with the hub's UTC receipt timestamp. Both snapshots survive
disconnect and hub restart, remain absent (`nil`) for older records or devices
that have never reported, and are deep-cloned on snapshot so callers cannot
mutate live or writer-retained state. Three read-only commands expose them on
every command surface: `show fleet device inventory name <name>`, `show fleet
device health name <name>`, and `show fleet health`. Both per-device commands
report explicit `reported` and `status` fields alongside `connected` and
`received-at`: `show fleet device inventory name <name>` sets `status` to
`reported` or `not-reported`, whereas `show fleet device health name <name>`
sets `status` to `not-reported`, `healthy`, `degraded`, or `down`. The estate
summary classifies each
declared device once into `healthy`, `degraded`, `down`, or `unreported`,
retains a disconnected device's last reported health with `connected` set to
false, and lists degraded or down devices in name order.
<!-- source: internal/component/plugin/server/managed_registry.go -- ManagedInventorySnapshot, ManagedHealthSnapshot, recordInventoryReport, recordHealthReport -->
<!-- source: internal/component/plugin/server/fleet_report.go -- ManagedFleetHealth, FleetHealthSummary -->
<!-- source: internal/component/managed/cmd_reports.go -- handleFleetDeviceInventory, handleFleetDeviceHealth, handleFleetHealth -->

### Roles

| Role | Config | Description |
|------|--------|-------------|
| Hub (serves config) | `server central { client edge-01 { secret } }` | Accepts managed clients, serves their configs |
| Managed client (provisioning) | `ze init --managed` with the hub address, the client's secret and how to trust the hub | One-time setup, stored in the tree |
| Managed client (running) | `client edge-01 { host; port; secret }` in cached config | Connects to hub, fetches config |
| Standalone | Only `server local { }` blocks, no hub-level `client` | Local plugins, no remote hub |

`ze init --managed` provisions identity and hub connection information in the
tree: the instance name, the hub address, the client's secret, and how the
first start authenticates the hub (below). `ze start` reads that store and
connects when managed mode is enabled. After the first fetch, the cached
config carries its own `client` block. CLI flags (`--server`, `--name`,
`--token`) can override these values for troubleshooting.

| What `ze init --managed` takes | How | Store key |
|-------------------------------|-----|-----------|
| Instance name | prompt, line 5 | `meta/instance/name` |
| Hub address, `host:port` | prompt, line 6 | `meta/managed/hub/address` |
| The client's secret, 32 characters or more | prompt, line 7 | `meta/managed/hub/token` |
| The hub's certificate authority root | `--hub-ca <file>`, a PEM file | `meta/managed/hub/ca` |
| Trust on first use | `--hub-trust-first-use` | `meta/managed/hub/trust` = `first-use` |

The hub address and the secret may be left empty: nothing is stored, and the
first start names them with `--server` and `--token`. `--hub-ca` and
`--hub-trust-first-use` need `--managed` and exclude each other. `ze init`
refuses an address that is not `host:port`, a secret shorter than a hub
accepts, and a `--hub-ca` file that is not exactly one PEM certificate of a
certificate authority.
<!-- source: internal/plugins/init/main.go -- managedInitFromFlags, runInitPublishedProfile -->
<!-- source: pkg/zefs/keys.go -- KeyManagedHubAddress, KeyManagedHubToken, KeyManagedHubCA, KeyManagedHubTrust -->

### Per-Client Secrets

The hub accepts up to 4,096 distinct client names across its server blocks.
An operator can label a client under `bus hub server <block> client <name>`.
Blocks that declare the same client must give it the same labels. The hub
checks the label sets independently of each listener's authentication token.
<!-- source: internal/component/config/loader_extract.go -- ExtractHubConfig, readManagedClientLabels -->

At `#0 auth`, the hub looks up the token against the `client` entries nested under the `server` block whose listener accepted the connection. A client cannot use another client's token, and a client is admitted only on the address of the block that declares it: on any other managed listener it is an unknown name, whatever token it sends. A client that must be reachable through two blocks is declared under both.
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedListener, handleConn -->

| Server config | Client config | Effect |
|--------------|--------------|--------|
| `server central { client edge-01 { secret "abc"; } }` | `client edge-01 { secret "abc"; }` | Match, accepted |
| `server central { client edge-01 { secret "abc"; } }` | `client edge-01 { secret "xyz"; }` | Mismatch, rejected |
| No `client edge-01` in any server | Any | Rejected: unknown client |
| `server central { client edge-01 { secret "abc"; } }`, and the client dials `server other` | `client edge-01 { secret "abc"; }` | Rejected: unknown client on that listener |

The managed listener authenticates with the per-client secrets alone and no
shared secret, so a name with no `client` entry has no expected token.
`AuthenticateWithLookup` refuses an empty expected token whatever the client
sends: an empty token would otherwise compare equal to it, and any name would
connect. Every refusal is logged at WARN under `doctor-managed-client-rejected`
and recorded for doctor (below) under the claimed name, never with the token.
<!-- source: internal/component/plugin/ipc/tls.go -- AuthenticateWithLookup -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- authRefused -->

### Device connections and configuration history

A hub keeps one registry entry for each declared client name, including a
device that has never connected. Each entry admits one session. A duplicate
connection leaves the first session and its connection time intact. A
successful admission records the client address without its TCP port. A
disconnect records when that session ended. A reconnect preserves the time
of the first connection and records the start of the new session.
<!-- source: internal/component/plugin/server/managed_registry.go -- ManagedDeviceRecord, admitOnListener, disconnectDevice -->
<!-- source: internal/component/plugin/server/managed_serve.go -- NewManagedServer, handleConn -->

Each fetch records the version the device named and whether the hub served
configuration, answered that the device is current, or encountered a read
error. Each acknowledgement records its version and outcome. The hub
records a holding acknowledgement when it has a copy of the version the
device runs. It records a refusal when that copy is missing.
<!-- source: internal/component/plugin/server/managed_registry.go -- recordFetch, recordAck -->
<!-- source: internal/component/plugin/server/managed_serve.go -- handleRequest, handleConfigAck, holdShownFromCopy -->

### What doctor reads from the managed listener

`startManagedServer` hands the managed server the plugin acceptor's address
(`ManagedServerConfig.Acceptor`, the first `server` block). The server
publishes its record after `Start` attempts its configured addresses, and
`Stop` withdraws it. Doctor reads the record through `ManagedRuntimeHealth`.
A startup failure that prevents publication remains visible with its cause.
When a configuration drops a client, its last named rejection clears.
<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- ManagedRuntimeHealth, managedDoctorState, notifyDropped -->
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedListenerUnbound, checkManagedClientRejected -->

| Event | Recorded as | Doctor code |
|-------|-------------|-------------|
| Managed server startup fails before publishing the registry | the startup error and its repair | `doctor-managed-server-unavailable` (error) |
| A listen address that does not bind, other than the acceptor's own | the address and the socket or certificate error with its repair | `doctor-managed-listener-unbound` (error) |
| A listen address the plugin acceptor already holds | not recorded; logged at ERROR, and the local `hub-managed-listener` check reports it from the config | `doctor-hub-managed-collision` |
| A second session under a name already connected | that client's last refusal | `doctor-managed-client-rejected` (warning) |
| A `config-ack` with an error | that client's last refusal, cleared by the next successful ack | `doctor-managed-client-rejected` (warning) |
| A client line exceeding `managedReadMax` (`ManagedRejectOversize`) | connection closed; recorded as that client's last refusal | `doctor-managed-client-rejected` (warning) |
| A configured name with a token that is not its secret | that client's last refusal | `doctor-managed-client-rejected` (warning) |
| A name with no `client` entry | the 16 most recent such names, the oldest forgotten first | `doctor-managed-client-rejected` (warning) |
| A remote address whose TLS handshake ends with one of the six certificate-rejection alerts | up to 16 remote addresses (without port) kept for one hour, with count, first/last times, presented certificate, devices last seen from that address, and a forgotten-count row; cleared when an authenticated session opens from that address | `doctor-managed-certificate-refused` (warning) |
| Any other connection that ends before naming a client (non-certificate fatal TLS alerts, TLS version/cipher mismatch, plain TCP close, non-TLS bytes, timeout, malformed first line) | up to 16 remote addresses apart from the certificate-refused class, kept for one hour, with count, first/last times, error, and a forgotten-count row; cleared on authentication | `doctor-managed-client-rejected` (warning) |
| All 32 unauthenticated handshake slots occupied at once (`managedMaxHandshakes`) | saturation time, count, and the remote addresses holding slots | `doctor-managed-handshake-saturated` (warning) |
| A device history write failure under `meta/fleet/device/<name>/record` (or failed removal of a foreign key) | per-device failure with key and store error, cleared by the next successful write or removal | `doctor-managed-device-record-unwritable` (error) |
| A stored device record that fails read or strict JSON decode at startup | per-device failure with key and decode error, cleared when a new event replaces it or `ze data rm` deletes it | `doctor-managed-device-record-unreadable` (error) |
| A `config-changed` a connected client was not given (the notification could not be built, or the client did not accept it) | an event on the report bus naming the client and the cause | `doctor-managed-client-notify-dropped` |

### Config Storage (Hub Side)

Client configs are entries in the hub's live tree, keyed by client name at
`file/active/client-<name>.conf` (the
`client-` prefix avoids collision with the hub's own config file).
<!-- source: internal/component/plugin/server/managed_serve.go -- ClientConfigKey -->

The admin can edit these through a live SSH editor owned by the hub:
`ze config edit client-<name>.conf` beside a running hub connects to the hub's
own editor and names that file. The commit runs on the hub's store handle, with
no reload of the hub, records a new active version and fires the write observer
below, so the client is told. The file must exist first; an import or a restore
gives it its first version. The attached console, the web editor and the API
config sessions edit the hub's own configuration only.
<!-- source: internal/component/config/cli/cmd_edit.go -- runStoredEditor -->
<!-- source: cmd/ze/hub/session_factory.go -- buildSessionModelFactory -->
<!-- source: internal/component/cli/editor_commit.go -- publishCommitted -->

The approved post-release fleet device Config tab adds a separate, authorized named-device editor. It can provision the first version or edit a later one before the device connects. It keeps each candidate scoped to its operator and declared target. The existing general web editor continues to edit the hub configuration.

The editor refuses the session when no serving block declares the client the
file is named for, and when the daemon serves no client at all, as the import,
the restore and the rollback do: the commit would record a version that no
client ever fetches. The refusal carries the reason and the fix of the import,
from the same producer. The hub reads the declared clients when the session
opens, so the answer is the one the data commands get. It does not ask whether
the client is connected. The hub may write the configuration of a declared
client at any time, and a client that is away fetches it when it connects.
This rule decides which files a hub writes. It does not decide which users may
open the editor.
<!-- source: cmd/ze/hub/session_factory.go -- undeclaredClientSessionRefusal -->
<!-- source: internal/component/config/storage/cli/data_rpc_config.go -- UnservedClientConfigRefusal, unservedClientRefusal -->
<!-- source: internal/component/plugin/server/managed_serve.go -- Serves -->

A running hub also takes a client config from a file: `ze data import
client-<name>.conf` beside it, or `update data config name client-<name>.conf
content <base64>` on any command surface, commits a new version through the
same store handle.
<!-- source: internal/component/config/storage/cli/main.go -- importConfigsThroughDaemon -->
<!-- source: internal/component/config/storage/cli/data_rpc_config.go -- handleDataConfigUpdate -->

Offline writers require the hub to be stopped because
the writable store handle holds an exclusive lifetime lock. Read-only
inspection remains available while the hub runs. `ze data rm` beside a running
hub sends its removal to the hub as `delete data key`, which removes
runtime-state keys only, so a client config (a configuration key) is still
removed with the hub stopped. A running hub also restores
a client's config from a backup artifact with `request data restore path <file>
config client <name>`: the RPC writes a version of `client-<name>.conf` and
promotes it under the store guard, with no reload of the hub, so the write
observer below pushes `config-changed`. It refuses a client that no serving
block names, because that key would be written and never served.
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreConfigFromSource -->

The hub's `Storage.SetWriteObserver` callback maps a written client-config key
back to the client's name and marks that client as owed `config-changed`,
without blocking the storage writer. A client fetches the updated configuration
after the notification or on its next connection.
<!-- source: internal/component/plugin/server/managed_serve.go -- Start -->
<!-- source: internal/component/config/storage/storage.go -- Storage -->

A change is never lost between the write and the push. The mark is held per
connected client until a notify worker takes it, so a burst of writes cannot
overflow anything, and several writes for one client before a worker reaches
it become one push. One worker pushes to a given client at a time: a client
that does not answer holds that one worker for the push timeout (10 seconds)
and never the pool, so every other client is still told at once.

A connected client that does not accept the notification is disconnected. It
would otherwise stay connected on its previous config with nothing left to
tell it. Closing the connection makes it reconnect under "Reconnect" below,
and a client fetches its config each time it connects. The event
`doctor-managed-client-notify-dropped` records that it happened. A client that
is not connected is owed nothing: it fetches when it connects.
<!-- source: internal/component/plugin/server/managed_serve.go -- NotifyConfigChanged, takePending, pushConfigChanged -->

The managed listener exposes Prometheus metrics: `ze_managed_clients_declared` (gauge),
`ze_managed_clients_connected` (gauge), `ze_managed_handshakes_pending` (gauge),
`ze_managed_config_fetch_total{result}`, `ze_managed_config_changed_pushed_total`,
`ze_managed_device_records_written_total{result}`, and `ze_managed_config_held_total{result}`.
<!-- source: internal/component/plugin/server/managed_serve.go -- metric registration -->

### Device history on the hub

On startup the hub loads the JSON records under `meta/fleet/device/<name>/record`.
It lists every recorded device offline until a new session authenticates.
A declaration with no recorded event appears in the registry without writing
a key. A device name must fit one store key segment. The hub rejects a slash,
a name equal to a single dot, or consecutive dots in a declared name.

Session, fetch and acknowledgement events update memory first. One writer
coalesces pending events by device and saves the latest facts. A clean stop
drains the final disconnections. The counter `ze_managed_device_records_written_total`
reports saves by result (`ok` or `failed`). An interrupted hub can lose events still
waiting for the writer. A failed write leaves the live facts visible with
`persisted` false and the next event retries it. Doctor names the device and
the store error. A stored record with unknown fields or an invalid value
stays visible with `unreadable` true until a new event replaces it.

Deleting a device record with `ze data rm meta/fleet/device/<name>/record`
also clears the running hub's copy. An undeclared device disappears from
the list. A declared device keeps its labels and, while connected, its current
session time and source address. When the session ends those facts still show
that it connected. Its next event writes the new history.
Overwriting a record through a data command makes the hub restore its own
history. The hub removes a newly written key for an unknown device. Each
automatic correction writes an info log naming the affected key once it succeeds.
If the correction fails, doctor reports the key and the store error.
The store reports a successful runtime-key deletion while it still serializes
changes to that key. The registry clears the corresponding history immediately.
The writer publishes its own successful save directly into the registry under
the same guard. A repair of malformed history restores the retained original
bytes. If the original read failed, doctor reports the failure until the
operator removes the key or a device event supplies new facts.
Optional `inventory` and `health` snapshots ride on the same `ManagedDeviceRecord`
value as immutable pointers so older records without either field load cleanly
as unreported, restart validation applies the same strict report checks as live
receivers, and operator deletion reconciliation retains reports that arrived
after the deletion was observed while dropping earlier ones.
<!-- source: internal/component/plugin/server/managed_record_store.go -- loadDeviceRecords, syncDeviceRecord, decodeDeviceRecord -->

### Config Storage (Client Side)

The managed client's local tree caches the config received from the hub. The config itself contains everything needed:

| What | Where |
|------|-------|
| Client name | Block name in `client <name> { }` at hub level |
| Hub address | `host` + `port` in the same block |
| Auth token | `secret` in the same block |
| BGP config | `bgp { }` block |
| Local plugins | `server local { }` block |
| Managed mode toggle | `meta/instance/managed` in the live store |
| Baseline: the hash of the last configuration the hub made active | `meta/managed/base-version` in the live store (see "The baseline and the hold") |
<!-- source: pkg/zefs/keys.go -- KeyInstanceManaged, KeyManagedBaseVersion -->

The `meta/instance/managed` flag says whether the device follows its hub. Two
commands change it, and with it the hub connection, without changing the
config.

`request fleet disable` stores the flag as false, then ends the hub connection.
The daemon cancels its hub client and waits for it, so a configuration the
client was applying is applied, and its baseline stored, before the connection
closes. The command needs no hub, no connection and no client, so it is the way
out for a managed device whose configuration names no hub.

`request fleet enable` stores the flag as true, then starts the hub client
again, with no restart. The client is built from the version of the
configuration the daemon runs at that moment, the one the active pointer
names, never from what the daemon read at boot. The command is
refused, and the flag left as it was, while that configuration names no
hub-level `client` block. Its fix is the one `doctor-hub-client-missing` gives.

A daemon that started managed owns its hub client for its whole life, whether
or not its configuration named a hub at start. The owner runs one client at
most, on a context no other worker of the daemon shares, refuses a second
start, and is joined before the daemon returns its store. A daemon that started
with the flag false owns no hub client. There `request fleet enable` stores the
flag and answers that the hub connection starts at the next `ze start`: an
unmanaged start runs services a managed start does not, and a hub client is
never attached to it. `request fleet disable` on such a daemon stores the flag.

The daemon runs one disable or enable at a time, from its flag write to the
end of its stop or its start. Two commands sent at once
therefore never leave the flag saying one thing and the client doing the
other: the flag and the client agree once each command has answered.

With no daemon running, `ze request fleet disable` and `ze request fleet
enable` write the flag into the store and the next `ze start` follows it.
While a daemon owns the store the same line is sent to that daemon. A flag
written by another route (`update data key`, `ze data write`) decides the next
start and is read by the running client before its next connection attempt. It
does not end a connection in use.

`show fleet status` reports the flag. From a running daemon it also reports
`client` (`running`, `stopping` while a stop is in progress, `stopped`, or
`not-started` for a daemon that started unmanaged), `connection`
(`connected`, `not-connected`) and `holding` (`holding`, `not-holding`). The
report never waits for a stop to finish.

`frozen` is true only when the store refuses an operator's change to the
device's own configuration (see "The freeze"): the device is managed and its
daemon runs a stored configuration, or no daemon runs and the device is
managed. A daemon started from a file or from standard input is never frozen,
so it reports `frozen` false even while `managed` is true.
<!-- source: internal/component/managed/cmd_fleet.go -- disableOnDaemon, enableOnDaemon, requireDeclaredHub, changeFleetLocal, FleetTarget, daemonClientReport -->
<!-- source: internal/component/managed/cmd_fleet.go -- readFleetStatus, frozenHandle -->
<!-- source: internal/component/config/storage/freeze.go -- refuseFrozenConfigLocked, OwnConfigFrozen -->
<!-- source: cmd/ze/hub/managed_owner.go -- managedClientOwner, startWith, stopWith, join, shutdown, state -->
<!-- source: cmd/ze/hub/main.go -- RunWithManagedClient, runYANGConfig -->
<!-- source: cmd/ze/ze_core_start.go -- cmdStartManaged, extractManagedClientConfig -->
<!-- source: internal/component/managed/client.go -- RunManagedClient -->
<!-- source: internal/component/managed/yang/ze-managed-cmd.yang -- container request -->

The running client follows the hub block of each config it applies. After a
pushed or fetched config is committed and acknowledged, the client reads that
config's hub-level `client` block again. When the name, `host`, `port`,
`secret`, `ca` or source address differ from the connection in use, it closes
that connection and reconnects at once with the new values, with no restart. A
config that leaves the block alone keeps the connection, and a config the
daemon refused changes nothing. A config with no hub-level `client` block asks
for no hub connection: the client stops following its hub and logs that it did,
which is what a restart on that config would do. The hub can no longer correct
that client, so `ze doctor` on the client reports it under
`doctor-hub-client-missing`: a managed client (`meta/instance/managed` is true)
whose configuration names no hub. The fix is on both sides: add the block to
the client's configuration on the hub, then provision the client again with
`ze init --managed --force` so that its next start fetches it.
<!-- source: internal/component/managed/doctor.go -- checkHubClientDeclared -->
<!-- source: internal/component/managed/client.go -- adoptEndpoint, RunManagedClient -->
<!-- source: cmd/ze/ze_core_start.go -- hubClientEndpoint -->

### The baseline and the hold

A managed client must not lose a configuration change made on the client
itself. The client therefore records which configuration its hub last gave it,
and applies nothing from the hub while it runs a different one.

**The baseline** is the version hash of the last configuration that became
active on the client from its hub. The store holds it under
`meta/managed/base-version`, as the 16 characters of the hash and a newline.
The hash is computed from the configuration the store holds as active, read
back after the write. It is never computed from the bytes the hub sent, so a
write that changed the text cannot make a client differ from its hub.
<!-- source: internal/component/managed/baseline.go -- StoreBaseline, RecordActiveAsBaseline, BaselineVersion -->

Three places write the baseline, and nothing else does:

| When | Writer | When the write fails |
|------|--------|----------------------|
| A hub push whose reload succeeded | `applyManagedPush`, through `recordPushBaseline` | The push stays acknowledged as applied, because the daemon runs it. One warning is logged under `doctor-managed-diverged` |
| The first start, when the fetched configuration is cached and before the client connects again | `cacheFirstConfig` | The start is refused with an operator error whose fix is `ze show data check`. The configuration stays cached, so the next start runs it with no baseline |
| A fetch that the hub answers `current` | `settleBaseline` | One warning is logged under `doctor-managed-diverged`. The connection is kept, and the next fetch records it |
<!-- source: cmd/ze/hub/managed.go -- applyManagedPush, recordPushBaseline -->
<!-- source: cmd/ze/ze_core_start.go -- cacheFirstConfig -->
<!-- source: internal/component/managed/client.go -- settleBaseline -->

A push that was refused, or whose reload failed, made nothing active and
leaves the baseline as it was. `ze init --managed --force` replaces the whole
store, so the baseline goes with it.
<!-- source: internal/plugins/init/main.go -- Run, the --force flag -->
<!-- test: internal/plugins/init/managed_hub_test.go TestInitForceRemovesBaseline -->

**The comparison.** Before each fetch, on a new connection and after each
`config-changed`, the client reads two hashes from its store: the hash of the
configuration the daemon runs, and the baseline. The configuration the daemon
runs is the version that the active pointer names. It is the plain file only
while no version is pointed. The client sends the first hash as the version of
its `config-fetch`. One rule, `Diverged`, compares the two: both are recorded,
and they are different.

| What the store says | The hub answers | What the client does |
|---------------------|-----------------|----------------------|
| No baseline, or the active hash equals the baseline | A configuration | It validates, applies and acknowledges as in "Config Change (Two-Phase)". The push records the new baseline. A local commit that landed during the fetch is kept: the stage refuses the push, and the client fetches once more over that edit and holds |
| Anything | `current` | It applies nothing. When the active hash is not the baseline already, it records the active hash as the baseline. A client that was holding then sends a `config-ack` with `ok:true` |
| The active hash differs from the baseline | A configuration whose `discard-version` is the active hash | It applies the configuration as a hub push, which records the new baseline (see "Revert" below) |
| The active hash differs from the baseline | A configuration with no `discard-version`, or one naming another hash | It holds |
| The active configuration or the baseline cannot be read | Nothing: no fetch is sent | It applies nothing, sends a `config-ack` with `ok:false` that says it could not compare, logs one warning under `doctor-managed-diverged`, and keeps the connection |
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, settleBaseline, holdFetched, applyReverted, refuseUncompared -->
<!-- source: internal/component/managed/baseline.go -- Diverged, ActiveVersion, activeConfig -->

**A client that holds** does these things, and no others:

- It does not decode, validate, store or apply the configuration the hub
  returned, unless the answer's `discard-version` is the hash it runs: that
  is a revert, and it stops holding (see "Revert" below). Otherwise the
  fetch is only how the client learns that the hub holds another
  configuration.
- It sends its hub a copy of the configuration it runs, with `config-held`,
  before it answers (see "The hub's copy of a hold" below).
- It sends a `config-ack` with `ok:false` and `holding:true`. The error text
  starts with `holding:`, carries the active hash and the baseline, and says
  whether the copy was sent. It carries no configuration content.
- It logs one warning under `doctor-managed-diverged`, with the active, the
  baseline and the offered hashes. Its fix names the hub commands only while
  the hub keeps a copy of the active version (the hub's `held-version`, or an
  upload the hub took: `uploadHeld` answers `kept`, `heldFix` picks the fix);
  otherwise it names the manual routes.
  <!-- source: internal/component/managed/client.go -- uploadHeld, heldFix -->
- It keeps the connection and keeps answering the heartbeat. A hold is not a
  connection error: the backoff starts again at each successful
  authentication, so a client that closed the connection would reconnect and
  hold again about once a second. A device whose local commits keep racing
  its hub's pushes pays that cost: each push whose two fetches both meet a
  local commit ends the connection (see "Config Change (Two-Phase)"), so it
  reconnects about once a second while the race lasts, and logs one Info line
  each round.
- It holds again at each later `config-changed`: it answers the notification,
  fetches and refuses. The hub therefore gets one refusal for each of its
  writes.

<!-- source: internal/component/managed/client.go -- holdFetched -->
<!-- test: internal/component/managed/client_held_test.go TestHoldAckSaysHolding -->

A client that holds applies nothing until a revert, so it does not follow a
change of the hub block either. A hub that moves, or changes that client's secret, while the
client holds loses it.
<!-- source: internal/component/managed/client.go -- holdFetched, handleConfigChangedRequest, applyFetched -->

**What the hub sees.** A `config-ack` with `holding:true` from a client
whose copy the hub keeps is not a refusal: the hub clears that client's
refused-configuration row, and the hold is shown from the copy. The copy must
be of the version the client named in its last `config-fetch` on that
connection, the one it runs (`managedSession`, `holdShownFromCopy`): a client
that edited again and whose newer copy was refused or too large still has
only its older copy on the hub, so its ack is recorded as a refusal. Otherwise the
hub records a failed `config-ack` as that client's last refusal, and `show doctor` on the hub lists it under
`doctor-managed-client-rejected` with the client's own text (see "What doctor
reads from the managed listener"). The hub reads nothing in that text. The row
is the one of any refused configuration, so its fix line is the fix of a
configuration the client could not validate. The client's text names
`ze doctor` on the client, which gives the two ways to end a hold. The hub
clears the row when that client next sends a `config-ack` with `ok:true`. A
client whose hold ends on a `current` answer sends one, with the hash of the
configuration it runs as the version, although it applied nothing: without it
the hub would go on listing a client whose hold was resolved. A client that
was not holding sends none for a `current` answer.
A holding client whose copy the hub does not keep (too large, refused, never
sent, or a client built before `config-held`) stays on that row, so no hold
is invisible on the hub.
<!-- source: internal/component/plugin/server/managed_serve.go -- handleRequest, handleConfigAck, holdShownFromCopy -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline -->
<!-- test: internal/component/managed/client_hold_test.go TestHoldEndedOnCurrentTellsTheHub, TestFirstBaselineSendsNoAck -->
<!-- test: internal/component/plugin/server/managed_held_test.go TestHoldingAckWithRecordClearsRefusal, TestHoldingAckWithoutRecordRecordsText -->

**The hub's copy of a hold.** A client that holds sends its hub the
configuration it runs, so the hub's operator can see the hold. The client
reads the bytes, their version hash and the stamp of the active version in
one read, so the copy is exactly the configuration whose hash it names. The
stamp is when that version was written into the client's store: a commit
writes the version it then runs, and a rollback runs again a version written
earlier, with that earlier time. A configuration with no active pointer has
no stamp, and the copy carries no `edited-at`.
<!-- source: internal/component/managed/baseline.go -- readHeldConfig, activeConfigStamped -->
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfigStamped -->
<!-- test: internal/component/managed/baseline_test.go TestHeldReadAnswersActivationStamp -->

The client sends no copy when:

| Condition | Why |
|-----------|-----|
| The hub's `config-fetch` answer names the active version as `held-version` | The hub keeps that copy already, so a client that holds sends it once, not at each hub write |
| The configuration is larger than 1 MiB (1048576 bytes) | The upload runs on the client's notification loop. The client logs a warning under `doctor-managed-diverged` naming the manual routes, and its ack says why no copy was sent |
| An upload of this version failed earlier in this run of the client | The same bytes fail the same way. A new version is sent; the same one waits for the client to restart |
| This connection's hub answered `config-held` with an error that carries no `config-held-refused` code | A hub built before the verb answers it as an unknown method. The next connection tries once more |

One upload waits at most 5 seconds (`heldUploadTimeout`), below the hub's
10-second bound on a `config-changed` round-trip (`ManagedPushTimeout`): the
client answers no `config-changed` while it uploads. An upload that fails is
logged and the connection is kept. The fetch before it is the one round-trip
of the loop whose bound is not below the push bound (see "Connection
Lifecycle").
<!-- source: internal/component/managed/client.go -- uploadHeld, heldUploads, heldUploadTimeout -->
<!-- test: internal/component/managed/client_held_test.go TestHoldSkipsUploadWhenHubHoldsActive, TestHoldUploadRefusedOverBound, TestHoldUploadsAtBound, TestHeldUploadDeadline, TestHeldUploadNotRetriedAfterFailure, TestHeldUploadOncePerConnectionOldHub -->

The hub takes the name from the authenticated session. It refuses, with the
`config-held-refused` code and nothing written, a copy larger than 1 MiB
decoded (checked on the encoded length first), a copy whose bytes do not hash
to the version it names, and a copy that does not parse with the hub's
schema. The refusal of a copy that does not parse carries none of the
parser's words, because they can quote the configuration and its secrets,
and the refusal reaches the hub's log, the client and the client's hold
ack, which `show doctor` prints. A section of an external plugin the hub did
not start is accepted, and the plugin is noted in the record. The record is one JSON value under
`meta/config/client-<name>.conf/held` (`version`, `config`, `edited-at`,
`received-at`, `unchecked-plugins`, and `revert` once a revert is decided),
written under the writer lock of `client-<name>.conf`, and a newer copy
replaces it whole, without the revert decision of the copy it replaces. It is in the store,
so it survives a hub restart, and each `config-fetch` answer that carries a
configuration names it as `held-version`. A record that does not decode is an
error, never "no record"; the fetch then names no `held-version`, so the
client's next copy replaces it. Each copy counts in
`ze_managed_config_held_total{result}` (`stored`, `refused`).
<!-- source: internal/component/plugin/server/managed_held.go -- handleConfigHeld, takeHeld, checkHeldConfig, heldRecords, ReadHeldRecord -->
<!-- source: internal/component/plugin/server/managed.go -- HandleConfigFetch -->
<!-- test: internal/component/plugin/server/managed_held_test.go TestHeldArmWritesRecordUnderAuthName, TestHeldArmRefusesOversize, TestHeldArmRefusesHashMismatch, TestHeldUploadMalformedRefused, TestHeldUploadRefusalQuotesNoConfiguration, TestHeldUploadUncheckedPluginSection, TestHeldArmReplacesRecord, TestHeldRecordDecodeStrict, TestFetchAnswersHeldVersion -->

The record is removed when the hub answers `current` to that client, when
the client acknowledges a configuration with `ok:true` (either proves the hub
and the client hold the same configuration, and the second is how a revert
ends), and by adopt. A rename of
`client-<name>.conf` is refused while its record exists, because the copy
would be left under a name no client fetches. A rollback or an import of the
same configuration is not refused: it keeps the name and changes only what
the hub holds.
<!-- source: internal/component/plugin/server/managed_serve.go -- handleConfigAck -->
<!-- source: internal/component/config/storage/rename_config.go -- prepareConfigRename, heldRenameRefusal -->
<!-- test: internal/component/plugin/server/managed_held_test.go TestFetchCurrentClearsRecord, TestAckOKClearsRecord -->
<!-- test: internal/component/config/storage/rename_config_test.go TestRenameRefusedWithHeldRecord -->
<!-- test: internal/component/config/storage/rollback_config_test.go TestRollbackAndImportIgnoreHeldRecord -->
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- managedRejectedRow -->

**Adopt: the hub keeps the client's change.** `update fleet divergence name
<client> adopt version <hash>` makes the copy the configuration the hub
serves that client. `<hash>` is the copy's device hash the operator saw in
`show fleet divergence name <client>`. Under one writer lock of `client-<name>.conf`, the
lock the `config-held` arm takes to write a copy, the hub:

1. refuses a client no server block declares, with the refusal every hub
   writer of an unserved client configuration answers
2. reads the record, and refuses when there is none or when its hash is not
   `<hash>`: the client sent another copy after the operator looked
3. publishes the copy's bytes as a new active version of
   `client-<name>.conf` (`storage.CommitConfigWithGuard`), and refuses,
   keeping the copy, while a change of that configuration is staged
4. removes the record

One lock across the four steps is what keeps a newer copy out of the middle:
it lands before, and step 2 refuses, or after, and it is kept. When the lock
is released, the store's write observer tells the client that its
configuration changed. The client fetches with the hash it runs, which is now
the hub's, the hub answers `current`, and the hold ends as below. Adopt
writes nothing on the client: the client already runs that configuration.

In the daemon, adopt acts on the store the daemon owns through the
`DivergenceTarget` the hub installs, and the dispatcher records the
`fleet-divergence-adopt` audit action. Beside a running hub, the local
`ze update` sends the line to that daemon. With the hub stopped it is
refused, fix `ze start`: only the running hub records the audit action
`fleet-divergence-adopt`, so an adopt is taken only there.
A copy over the 1 MiB bound is never sent, so the hub holds nothing to adopt,
and the manual import of "End the Hold" in the guide stays the route.
<!-- source: internal/component/managed/cmd_divergence.go -- handleDivergenceAdopt, adoptLocal, adoptHeld, adoptHeldLocked -->
<!-- source: internal/component/plugin/server/managed_held.go -- ReadHeldRecordWithGuard -->
<!-- source: internal/component/config/storage/cli/data_rpc_config.go -- ServedClientRefusal -->
<!-- test: internal/component/managed/cmd_divergence_adopt_test.go TestAdoptServesHeldBytes, TestAdoptRemovesHeldCopy, TestAdoptTakesOneGuard, TestAdoptNotifiesClient, TestAdoptRefusesStaleHash, TestAdoptRefusesStagedCandidate -->

**Revert: the client drops its change.** `request fleet divergence name
<client> revert version <hash> hub-version <hash>` decides that the client
discards the configuration it holds and takes the one the hub serves it.
`version` is the copy's device hash D and `hub-version` the hash V of the
configuration the hub serves the client, both as the operator saw them in
the show. Under the same writer lock, the hub refuses as adopt does (an
undeclared client, no record, a record of another hash), reads the
configuration it serves the client and refuses unless it still hashes to V,
and writes the record back carrying the decision: D, V, the user and the
time (`HeldRecord.WithRevert`). V is always the operator's hash, never the
configuration as it stands when the decision is taken, so a hub write
between the show and the revert refuses the revert rather than binding it
to a configuration nobody saw. It writes no `client-<name>.conf`, so after the lock
is released the handler tells the client itself (`NotifyConfigChanged`). A
client that is not connected learns it at its next connection's fetch.

The hub's `config-fetch` answer carries the decision as `discard-version` =
D only while two things still hold: the client fetches with version D, and
the configuration the hub serves it still hashes to V
(`RevertDecision.OfferedFor`). The client, in `fetchAgainstBaseline`, beside
the hold, applies the answer only when `discard-version` is exactly the hash
it runs (`applyReverted`). It applies it through `ClientConfig.CommitOver`,
the one commit callback every hub configuration takes, which the device
daemon wires to `applyManagedPush` staged with
`storage.StageMachineCandidateOver`: the machine entry the freeze exempts,
which re-reads the active configuration under the configuration's writer
lock and stages nothing unless it still hashes to D (`activeCheck`), then
`recordPushBaseline`. A local commit that lands while the fetch is answered
therefore wins, as it does over an ordinary push (see "Config Change
(Two-Phase)"): nothing is applied, and the device fetches once more naming
that newer edit (`refetchOverLocalCommit`), so its hold, its upload and its
holding ack answer a fetch of the version it runs, which is what lets the hub
show the hold from the copy rather than record a refusal. That second fetch
cannot fetch again: if the store changed during it too, the device applies
nothing and sends nothing more, reports its hold from a fresh read of the
store (a device back on its baseline does not hold), logs that decision once
at Info, and ends the connection, at connect and on the notification loop
alike. The client reconnects with its normal backoff, without a "connection
lost" warning, and the reconnect's fetch decides again over what the device
then runs: it holds, uploads its copy and acks holding, or applies.
It acks `ok:true` once applied, which removes the record.
<!-- source: internal/component/managed/client.go -- applyReverted, refetchOverLocalCommit, logLocalCommitTwice, runConnection, RunManagedClient, ClientConfig -->
<!-- source: cmd/ze/hub/managed.go -- wireManagedCommit, applyManagedPush -->
<!-- source: internal/component/config/storage/pointer.go -- StageMachineCandidateOver --> A confirmed-commit window open on the client refuses the push as
today: the ack is `ok:false`, the client still holds, and the decision stays
for its next fetch.

The two bindings make a stale decision inert on both sides:

| What changed after the decision | Effect |
|---------------------------------|--------|
| The client's operator edited again (active D2) | The client fetches with D2, so the hub offers nothing; the client holds and sends D2, whose record replaces the one the decision was on. `show fleet divergence name` reports `revert` `none` |
| The hub's configuration for the client was written again (V2) | The hub offers nothing, the client holds, and the show reports `revert` `void`, with the revert to decide again as the fix of `revert-void` |
| The client names another hash than its own in `discard-version` | It holds, and `show fleet status` on it reports `revert-offered` `stale` |

A hub can make a holding client apply only a configuration bound to the
client's own hash. A client built before `discard-version` ignores the field
and keeps holding; it is resolved by hand. With the hub stopped, the local
`ze request` refuses with `ze start` as the fix: only the running hub's
dispatcher records the `fleet-divergence-revert` audit action, and a stopped
hub offers nothing to a client anyway. Beside a running hub, the local command
sends the line to it.
<!-- source: internal/component/managed/cmd_divergence.go -- handleDivergenceRevert, revertLocal, revertHeld, revertHeldLocked -->
<!-- source: internal/component/plugin/server/managed_held.go -- RevertDecision, HeldRecord.WithRevert, WriteHeldRecordWithGuard, heldRecords.offer -->
<!-- source: internal/component/plugin/server/managed.go -- HandleConfigFetch -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, applyReverted -->
<!-- source: internal/component/managed/cmd_divergence_show.go -- divergenceRowOf, revertVoidReason -->
<!-- test: internal/component/plugin/server/managed_held_test.go TestFetchOffersDiscardOnlyForDAndV, TestHeldArmReplacesAndDropsDecision, TestHeldRecordKeepsDecision -->
<!-- test: internal/component/managed/client_held_test.go TestDiscardAppliesWhenActiveMatches, TestDiscardIgnoredWhenActiveDiffers, TestDiscardRefusedApplyStillHolds -->
<!-- test: internal/component/managed/cmd_divergence_revert_test.go TestRevertStoresDecisionAndNotifies, TestShowDivergenceReportsVoidDecision -->

**What the client's operator sees.** `ze doctor` on the client reports
`doctor-managed-diverged`, a warning, from the `managed-diverged` check. The
check reads the store only, so it reports with the daemon running or stopped.
It reports nothing on a client that is not managed, on one with no baseline,
and on one whose two hashes are equal. `show fleet status` reports both
hashes and whether they differ.
<!-- source: internal/component/managed/doctor.go -- checkManagedDiverged, diagnoseManagedDiverged -->
<!-- source: internal/component/managed/cmd_fleet.go -- readFleetStatus -->

**How a hold ends.** Two things end a hold. The first is the hub answering
`current` to the hash the client sent. That answer proves that the hub and
the client hold the same configuration, so the client records that hash as
its baseline and tells the hub with a `config-ack` (see "What the hub
sees"). Adopt and the manual import both end a hold this way: each makes the
hub hold the client's configuration. The second is a revert: a `config-fetch`
answer whose `discard-version` is exactly the hash the client runs, which the
client applies as a hub push (see "Revert" above). That is the one way the
hub makes a holding client apply a configuration, and only one the operator
decided on, bound to the configuration the client runs and to the one the
operator saw. No other message from the hub ends a hold.

**A client with no baseline is not holding.** A client provisioned before the
baseline existed, or again with `ze init --managed --force`, has none. It
applies its hub's configuration as before and records a baseline from then
on. The first fetch of such a client is the one fetch the baseline does not
protect.
<!-- source: internal/component/managed/baseline.go -- Diverged -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, settleBaseline -->

### The freeze

While `meta/instance/managed` is `true`, the store refuses an operator's
change to the client's own stored configuration. The store writes one operator
error:

```
the configuration was not changed: this device is managed by a hub
local changes are frozen while it follows a hub, so that a hub push cannot overwrite one
fix: request fleet disable
```
<!-- source: internal/component/config/storage/freeze.go -- ErrConfigFrozen -->

Every route answers the same problem, reason and fix, and the fix is always
the last line. Some routes put the command or the step that failed in front
of the first line, such as `write candidate:` or `restore config <name>:`.
The import, restore and rollback commands and the SSH configuration upload,
which wrap every other failure under a fix of their own, answer the freeze's
refusal as the freeze wrote it, so their answer ends on `request fleet
disable` too. They recognise it by the mark the freeze puts on its own
refusals, not by its type, so an operator error from another layer, a reload
that refused the configuration, keeps the route's own first line.
<!-- source: internal/component/config/storage/freeze.go -- FreezeRefusal -->
<!-- source: internal/component/config/storage/cli/data_rpc_config.go -- daemonConfigImportRefusal -->
<!-- source: internal/component/config/storage/cli/data_rpc_rollback.go -- configRollbackRefusal -->
<!-- source: internal/component/config/cli/cmd_rollback.go -- storedRollbackRefusal -->
<!-- source: cmd/ze/hub/config_upload.go -- Upload -->

**Where.** The refusal sits at the store's candidate staging step,
`WriteCandidateVersionWithGuard`, before any version is written. Every
versioned write an operator makes passes it: a session, console, web or API
commit, an SSH configuration upload, `update data config`, `request data restore ... config`, the rollback
command, and with the daemon stopped `ze config import` of a configuration
the store already holds, `ze data import` and `ze data restore ... config`. No
route carries a check of its own. With the daemon stopped,
`ze config rollback <N> <file>` is not one of them: it rolls back the disk file
it names, through a file editor, and never writes the store, and a configuration
read from a file is never frozen. The flag is read under the configuration lock
the staging already holds, so a refused write leaves no version and no
candidate pointer: a candidate left behind would make the next hub push fail.
<!-- source: internal/component/config/storage/pointer.go -- WriteCandidateVersion, WriteCandidateVersionWithGuard -->
<!-- source: internal/component/config/storage/freeze.go -- refuseFrozenConfigLocked, configFrozen, managedFlagRead -->
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWrite -->
<!-- source: internal/component/config/cli/cmd_rollback.go -- cmdRollbackThrough -->
<!-- source: internal/component/config/storage/restore.go -- restoreConfigLocked -->

**What is frozen.** Only the client's own stored configuration:

| Store handle | Frozen configuration |
|--------------|----------------------|
| A daemon started from its store (`BindConfigSource` with `ConfigSourceStored`) | The name the handle is bound to |
| A daemon started from a file or from standard input | None. That input is the authority, and such a daemon runs no hub client |
| A tool that opened the store with the daemon stopped | The name `meta/instance/name` selects, by the rule `ze start` uses (`InstanceConfigName`) |
| An explicit blob artifact, such as a backup | None. A backup carries the flag of the device it came from and is not that device |

Every other configuration in the same store stays writable. A hub that is
itself managed still writes the `client-<name>.conf` of each client it
declares, and records each as a version, while its own configuration is
refused.
<!-- source: internal/component/config/storage/freeze.go -- refuseFrozenConfigLocked, configFrozen, unboundConfigFrozen, OwnConfigFrozen, isBlobArtifact, sameConfigName -->
<!-- test: cmd/ze/hub/managed_hub_freeze_test.go TestManagedHubImportsDeclaredClientConfig -->
<!-- source: internal/component/config/storage/config_name.go -- InstanceConfigName -->

**What is exempt.** A writer that no operator decides stages through a
separately named entry, `StageMachineCandidate`, which does not read the
flag, or through its guarded form `StageMachineCandidateOver`, which every
hub push takes: the hub's push (`applyManagedPush`), the revert of an unconfirmed commit
on a running daemon (`confirmRevert`) and at boot (`restoreConfirmOnStart`),
the boot record of an explicit file (`initializeConfigSource`) and schema
recovery (`publishRecoveredConfig`). The signal reload stages the bytes the
source already holds through `StageSourceCandidate`, which skips the flag the
same way. A machine writer the freeze refused would stop a managed client from
following its hub or from recovering at boot.
<!-- source: internal/component/config/storage/pointer.go -- StageMachineCandidate, StageSourceCandidate -->
<!-- source: cmd/ze/hub/managed.go -- applyManagedPush -->
<!-- source: cmd/ze/hub/confirm_window.go -- confirmRevert, restoreConfirmOnStart -->
<!-- source: cmd/ze/hub/config_source.go -- initializeConfigSource, publishRecoveredConfig -->

**A flag the store cannot read refuses the write.** An absent flag means not
managed. A read that fails for any other reason refuses, naming
`meta/instance/managed` and the fix `ze show data check`. With the daemon
stopped, the same holds for `meta/instance/name` on a managed client.
<!-- source: internal/component/config/storage/freeze.go -- managedFlagRead, ownConfigNameRead, frozenUnknown -->
<!-- source: internal/component/config/storage/publication_intent.go -- readAdmissionKey, readAdmissionKeyLocked -->

**The freeze is best effort, and the hold is the guarantee.** The freeze tells
an operator early. It covers the writes that stage a version, and nothing
else. These routes change a managed client's configuration unrefused:

- a raw key write (`ze data write`, and `update data key` for a key it
  reaches), including a write of the active version pointer
  `meta/config/<name>/active` back to an earlier version the store holds
- a full-store restore
- a rename of a stored configuration
- the first import of a configuration name the store does not hold yet,
  which writes the key directly
- any change on a daemon started from a file or from standard input, whose
  input is not frozen at all

A change made that way is kept by the hold above: the client's active hash
then differs from its baseline, and it applies nothing its hub sends until
the two agree again. `test/managed/fleet-hold-keeps-local-edit.ci` proves it
with the active pointer written back to an earlier version while the client
is stopped and still managed.
<!-- source: internal/component/config/storage/import_config.go -- ImportConfig -->
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfig, activePointerKey -->

---

## Protocol

### RPCs

After `#0 auth`, the hub knows the client's name. All RPCs are scoped to that name implicitly.

| Verb | Direction | Payload | Response |
|------|-----------|---------|----------|
| `config-fetch` | Client to hub | `{"version":"<hash-or-empty>"}` | `ok {"version":"<hash>","config":"<base64>"}` or `ok {"status":"current"}` |
| `config-changed` | Hub to client | `{"version":"<hash>"}` | `ok {}` |
| `config-ack` | Client to hub | `{"version":"<hash>","ok":true}` or `{"version":"<hash>","ok":false,"error":"..."}`, and `"holding":true` on a hold | `ok {}` |
| `config-held` | Client to hub | `{"version":"<hash>","config":"<base64>","edited-at":"<RFC 3339 time>"}` | `ok {}`; an error with code `config-held-refused` for a copy the hub refused; any other error from a hub that does not take the verb |
| `inventory-report` | Client to hub | Compact JSON `InventoryReport` (max 64 KiB, per-section `available` flags, optional ECC counters, no raw SMART or drive serials) | `ok {}`; RPC error on invalid, oversized, stale-session, or rate-limited report |
| `health-report` | Client to hub | Compact JSON `HealthReport` (max 64 KiB, matching `/health` JSON status and component rows) | `ok {}`; RPC error on invalid, oversized, stale-session, or rate-limited report |
| `ping` | Either direction | `{}` | `ok {}` |

The `config-fetch` answer that carries a configuration may also carry
`"held-version":"<hash>"` and `"discard-version":"<hash>"`. Every field added
after the first four verbs is optional and omitted when unset, and the
payload of `config-held` names no client: the hub takes the name from the
authenticated session. A peer built before them decodes each message as it
did, because Go's JSON decoder ignores a key its type does not declare, and a
hub built before `config-held` answers it with an unknown-method error.

| Field | Message | Meaning |
|-------|---------|---------|
| `held-version` | `config-fetch` answer | The version hash of the configuration the hub holds as this client's held copy. Absent when the hub holds none |
| `discard-version` | `config-fetch` answer | Present only when the hub's operator decided that a client running exactly this version drops it and applies the configuration of the answer |
| `holding` | `config-ack` | `true` when the client refused because it holds a configuration of its own, not because the hub's one is invalid |
| `edited-at` | `config-held` | When the version the client runs was written into its store (the stamp of its active version). Absent when the client's store records no such stamp |
<!-- source: pkg/fleet/envelope.go -- VerbConfigHeld, ConfigHeld, ConfigFetchResponse, ConfigAck -->
<!-- test: pkg/fleet/envelope_test.go TestConfigHeldEnvelopeRoundTrip, TestOldPeerIgnoresNewFields -->

The hub caps concurrent unauthenticated handshakes at 32 (`managedMaxHandshakes`)
and releases each handshake slot as soon as authentication finishes, so
authenticated sessions are bounded by the declared client entries (up to 4,096)
and duplicate-name refusal rather than by a fixed connection cap. Reaching the
handshake cap is logged and reported under `doctor-managed-handshake-saturated`.

Each authenticated managed connection on the hub reads through a bounded frame
reader (`rpc.NewConnWithMaxRead`, `managedReadMax` = base64(`fleet.HeldConfigMax`)
+ 64 KiB, ~1.4 MiB). Its `bufio.Scanner` starts at 64 KiB and grows at most to
`managedReadMax` when a client uploads its largest held copy, so worst-case
read-buffer memory is bounded by declared clients times `managedReadMax`. A line
past `managedReadMax` closes that client's connection, records `ManagedRejectOversize`
under `doctor-managed-client-rejected`, and leaves all other clients running. On
the client side, `SendConfigAck` bounds `ConfigAck.Error` to `fleet.AckErrorMax`
(1,024 UTF-8 bytes) with a truncation marker (`... (truncated)`). The client's
inbound reader stays at `rpc.MaxMessageSize` (16 MiB) so the hub can serve
configurations larger than `HeldConfigMax`.
<!-- source: internal/component/plugin/server/managed_serve.go -- managedMaxHandshakes, managedReadMax, handleConn -->
<!-- source: pkg/fleet/envelope.go -- AckErrorMax, BoundAckError -->
<!-- source: internal/component/managed/client.go -- SendConfigAck -->

### Inventory and health reporting

Managed clients push bounded hardware summaries (`inventory-report`) and
component health snapshots (`health-report`) over the authenticated `MuxConn`
session. Both payloads are compact JSON capped at 64 KiB (`fleet.ReportBodyMax`)
before sending and before decoding, carry no client identity (the hub scopes
each report to the authenticated connection), and use separate producer and
receiver registries so `plugin/server` imports neither `host` nor `managed`.
`host` registers the hardware summary producer (`FleetInventoryInterval`, 24
hours) excluding raw SMART readings and block-device serials while marking
per-section availability and optional ECC counters; `managed` registers the
health producer (`FleetHealthInterval`, 5 minutes) matching `/health`'s JSON
shape.
<!-- source: pkg/fleet/envelope.go -- VerbInventoryReport, VerbHealthReport -->
<!-- source: pkg/fleet/report.go -- ReportBodyMax, InventoryReport, HealthReport, ReportProducer, RegisterReportProducer, EncodeInventoryReport, DecodeInventoryReport, EncodeHealthReport, DecodeHealthReport -->
<!-- source: internal/component/host/register.go -- init -->
<!-- source: internal/component/host/fleet_inventory.go -- FleetInventoryInterval, ProjectFleetInventory, collectFleetInventory -->
<!-- source: internal/component/managed/register_report.go -- init -->

After the initial `config-fetch` succeeds on an authenticated connection
(including when the client holds local edits), `startSessionReporter` starts one
per-session worker that sends each registered report immediately and repeats it
on its own interval (24 hours for inventory, 5 minutes for health). Each report
RPC uses a 30-second reply timeout (`defaultReportCallTimeout`) so a stalled or
refused report never blocks `ping` or `config-changed`, and the worker is
cancelled and joined before the connection ends. When a pre-Fleet-4 hub answers
`unknown method: <verb>`, the client suppresses only that report kind for the
rest of that connection and retries it on the next connection. Any collection,
encoding, transport, or hub-refusal failure raises a warning under
`doctor-managed-report-failed` (`CodeDoctorManagedReportFailed`) on the report
bus and clears it on the next accepted report of that kind.
<!-- source: internal/component/managed/report.go -- FleetHealthInterval, defaultReportCallTimeout, ProjectFleetHealth, startSessionReporter -->
<!-- source: internal/component/managed/register_doctor_codes.go -- CodeDoctorManagedReportFailed -->
<!-- source: internal/component/managed/client.go -- runConnection -->

On the hub, `ManagedServer.handleRequest` dispatches registered
`ManagedReportReceiver` handlers after the configuration verbs and before its
unknown-method refusal. The hub accepts the first inventory and health report on
each new connection and then enforces per-connection minimum intervals of 24
hours for inventory (`managedInventoryMinInterval`) and 1 minute for health
(`managedHealthMinInterval`). A rejected, invalid, oversized, rate-limited, or
superseded-session report returns an RPC error, leaves the previous accepted
`ManagedInventorySnapshot` or `ManagedHealthSnapshot` intact, and never
postpones the next permitted report interval. Each accepted report records the
hub's UTC receipt time, marks the device record dirty for Fleet-1 persistence,
and survives disconnect and hub restart.
<!-- source: internal/component/plugin/server/managed_serve.go -- handleRequest -->
<!-- source: internal/component/plugin/server/fleet_report.go -- ManagedReportReceiver, RegisterManagedReportReceiver, handleReport, managedInventoryMinInterval, managedHealthMinInterval -->
<!-- source: internal/component/plugin/server/managed_registry.go -- ManagedInventorySnapshot, ManagedHealthSnapshot, recordInventoryReport, recordHealthReport -->

### Version Hash

Truncated SHA-256 of the config bytes (hex-encoded, first 16 characters). Computed from content, never stored separately. Same content = same hash.

### Connection Lifecycle

1. Client reads cached config from the local tree (or bootstrap metadata from `ze init` on first boot)
2. Client extracts name, host, port, secret from hub-level `client <name> { }` block
3. Client connects via TLS to hub address
4. Client sends `#0 auth` with token and name
5. Hub validates token against `client` entry nested under the relevant `server` block
6. Client sends `config-fetch` with current version hash (or empty on first boot)
7. Hub reads the client's stored config and computes its hash
8. If hashes match: `{"status":"current"}`; if different: full config in response
9. Client persists the config and starts or reloads BGP
10. Heartbeat: `ping` every 30 seconds, timeout after 3 missed (90 seconds)
11. On a write through the hub's own store: hub sends `config-changed` to the connected client. See "Config Storage (Hub Side)" for which writes reach that store

Every `config-fetch` the client sends waits at most 30 seconds for its
answer (`configFetchTimeout`): at connect, after a `config-changed`, on the
fetch made again over a local commit, and at first boot. 30 seconds carry the
largest answer, 16 MiB (`rpc.MaxMessageSize`), on a link of about 4.5 Mbit/s.
A fetch at connect that reaches the bound ends the connection, and the client
reconnects with backoff. A fetch after a `config-changed` that reaches it is
logged under `doctor-hub-unreachable`, the connection is kept, and the next
`config-changed` is answered. An answer that arrives after the bound is
dropped, never applied. In both cases the client keeps the configuration it
runs.

This is the one bound of the client's notification loop that is not below the
hub's 10-second bound on a `config-changed` round-trip (`ManagedPushTimeout`),
by design: a bound that short would cut every large configuration on an
ordinary link. The cost is that a `config-changed` the hub sends while a
fetch runs longer than 10 seconds fails on the hub, which closes the
connection; the client fetches again when it reconnects, which delivers the
change that notice announced.
<!-- source: internal/component/managed/client.go -- configFetchTimeout, fetchConfigWithin, FetchConfig, handleConfigChangedRequest -->
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedPushTimeout -->
<!-- test: internal/component/managed/client_fetch_deadline_test.go TestFetchDeadlineAtConnect, TestFetchDeadlineOnNotificationLoop, TestFetchDeadlineDropsLateAnswer, TestFetchAtConnectAfterHubDropsPush -->
<!-- test: internal/component/managed/client_held_test.go TestFetchTimeoutDocumentsPushBoundTradeOff -->

A panic while the hub serves one client ends that client's connection only.
The connection closes, the client's name is released so it can authenticate
again, and the client reconnects under "Reconnect" below; every other client
keeps its connection. The panic is counted in
`ze_task_panics_total{component="plugin",task="managed-client"}`, logged at
ERROR and recorded for `ze show crashes` (`docs/guide/monitoring.md`,
"Contained Panics").
<!-- source: internal/component/plugin/server/managed_serve.go -- handleConn -->
<!-- source: internal/component/plugin/server/panic_guard.go -- managedPanics -->

### Config Change (Two-Phase)

| Phase | Action |
|-------|--------|
| 1. Notify | Hub sends `config-changed {"version":"<new-hash>"}` to connected client |
| 2. Fetch | Client sends `config-fetch {"version":"<new-hash>"}` when ready |
| 3. Validate | Client parses and validates the new config |
| 4. Apply | If valid: stage the config, reload BGP, promote the candidate, send `config-ack {"ok":true}` |
| 5. Reject | If invalid: send `config-ack {"ok":false,"error":"..."}`, keep running |

While the client daemon has a confirmed-commit window open, phase 4 stages
nothing: `wireManagedCommit` (`cmd/ze/hub/managed.go`) refuses the push through
`confirm.WriteOutside` with the window's refusal naming its owner, because the
window's revert would wipe the push, and the client answers
`config-ack {"ok":false}` carrying that refusal. The client keeps its old
version, so a later fetch, on the next `config-changed` or reconnection,
delivers the push again; nothing retries it when the window closes.

Phase 4's stage is guarded. Before its fetch the client read the hash of the
configuration it runs, and it decides to apply the hub's answer over that
version only. `wireManagedCommit` stages through
`storage.StageMachineCandidateOver`, which re-reads the active configuration
under the configuration's writer lock and hands it to the client's check
(`activeCheck`): a configuration that no longer hashes to the version read
before the fetch is refused, and nothing is staged. A local commit that
landed during the fetch therefore wins. The client sends no ack for that
answer and fetches once more, naming the newer edit
(`refetchOverLocalCommit`); that edit differs from its baseline, so the
client holds, sends the hub a copy and acks holding (see "The baseline and
the hold"). The fetch made again never leads to a third: if the store changed
during it too, the client applies nothing, logs that at Info and ends the
connection; the reconnect's fetch decides again over what it then runs. Any other refusal of the stage (a confirmed-commit
window, a candidate already staged, a failed reload) is acked
`config-ack {"ok":false}` with its cause and starts no second fetch.
<!-- source: internal/component/managed/client.go -- holdOrApply, applyFetched, activeCheck, refetchOverLocalCommit -->
<!-- source: cmd/ze/hub/managed.go -- wireManagedCommit -->

A client whose running configuration differs from its baseline fetches and
then goes no further: it does not validate or apply what the hub returned. It
holds, and answers `config-ack {"ok":false}` with the hold text (see "The
baseline and the hold").
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, holdFetched -->

The client controls timing. A router in the middle of graceful restart or convergence is not forced to reload.

---

## Client Startup Sequence

### First Boot (After `ze init`)

`ze init --managed` has stored identity, managed flag, hub address and secret in the tree. No config exists yet.

| Step | Action |
|------|--------|
| 1 | `ze start` |
| 2 | Read store: `meta/instance/name`, `meta/instance/managed`=true, `meta/managed/hub/address`, `meta/managed/hub/token`, `meta/managed/hub/ca` or `meta/managed/hub/trust`. A flag or its env var replaces the stored value. A value neither gives stops the start with an error naming it |
| 3 | Connect to hub, authenticate it (see "First start trust"), then authenticate to it, all within `ze.managed.connect.timeout` (default 5 seconds) |
| 4 | Fetch config (includes `server local { }` + `client edge-01 { }` + `bgp { }`), within the fetch's own 30-second bound (see "Connection Lifecycle"), not the connect bound. A hub that does not answer in time ends the first start with the fetch's deadline error, and nothing is written |
| 5 | Write config to the local tree |
| 6 | Start local hub server for plugins, start BGP |

The store now has the cached config. On subsequent boots, the config itself provides hub connection info.

### Subsequent Boots

| Step | Action |
|------|--------|
| 1 | `ze start` (no flags) |
| 2 | Read cached config from the local tree |
| 3 | Start local hub server (from `server local { }` block) |
| 4 | Extract name, host, port, secret from hub-level `client <name> { }` block |
| 5a | **Connection succeeds:** fetch latest config, persist it if newer, reload BGP if changed |
| 5b | **Connection fails:** start BGP from cached config, reconnect in background |

### Startup Matrix

| Hub | Cached config | Behavior |
|-----|--------------|----------|
| Reachable | Any | Fetch from hub, update the store, start BGP |
| Unreachable | Exists | Start from cached config, reconnect in background |
| Unreachable | Missing | First boot after init: exit with error (hub required) |

### Reconnect

| Parameter | Value |
|-----------|-------|
| Initial delay | 1 second |
| Backoff multiplier | 2x |
| Maximum delay | 60 seconds |
| Jitter | 10% random |

---

## Configuration

### YANG Structure

| Path | Type | Description |
|------|------|-------------|
| `plugin/hub` | container | Hub configuration |
| `plugin/hub/server` | list, keyed by name | Hub server instances |
| `plugin/hub/server/<name>/ip` | string | Listen address |
| `plugin/hub/server/<name>/port` | uint16 | Listen port |
| `plugin/hub/server/<name>/secret` | string (min 32 chars) | Shared secret for plugins connecting to this server |
| `plugin/hub/server/<name>/client` | list, keyed by name | Accepted remote managed clients |
| `plugin/hub/server/<name>/client/<name>/secret` | string (min 32 chars) | Per-client auth token |
| `plugin/hub/client` | list, keyed by name | Outbound hub connections |
| `plugin/hub/client/<name>/host` | string | Remote hub address |
| `plugin/hub/client/<name>/port` | uint16 | Remote hub port |
| `plugin/hub/client/<name>/secret` | string (min 32 chars) | Auth token |

### Client Side

Two sources of hub connection info, used at different stages:

| Stage | Source | Contains |
|-------|--------|----------|
| First boot (after `ze init --managed`) | Store metadata | `meta/instance/name`, `meta/managed/hub/address`, `meta/managed/hub/token` |
| Subsequent boots | Cached config | `client <name> { host; port; secret }` block |

The cached config takes over once fetched. Metadata from `ze init` bootstraps the first fetch.

CLI flag overrides (optional, for troubleshooting). `--server`, `--name` and
`--token` replace the matching value of the hub-level `client` block, on a
first boot and on a client that holds a cached config alike, and they keep
replacing it in every config the hub pushes while the daemon runs. They replace
a value and never stand in for the block: a config with no hub-level `client`
block connects to no hub whatever the flags say.
<!-- source: cmd/ze/ze_core_start.go -- hubClientEndpoint -->
<!-- source: cmd/ze/ze_core_dispatch.go -- zeParseGlobalFlags -->

| Flag | Env var | Description |
|------|---------|-------------|
| `--server <host:port>` | `ze.managed.server` | Override hub address |
| `--name <name>` | `ze.managed.name` | Override client name |
| `--token <token>` | `ze.managed.token` | Override auth token |
| `--connect-timeout <dur>` | `ze.managed.connect.timeout` | Bound on the dial and TLS handshake of each connection to the hub, and at first start on the authentication too (default: 5s). Every configuration fetch, the first one included, has its own 30-second bound instead (see "Connection Lifecycle"). A value that is not a duration is refused |
| (none) | `ze.managed.tls.insecure` | Skip server certificate verification (development only) |

### First start trust

First boot has no config, so it can name no `pki ca` entry. What authenticates
the hub then is what `ze init --managed` was told, in this order:

| `ze init --managed` was given | The first connection | Every later connection |
|-------------------------------|----------------------|------------------------|
| `--hub-ca <file>` | Verified against that root. A hub it did not issue is refused before the secret is sent | Verified against the same root, unless the config names a `ca` |
| `--hub-trust-first-use` | Whoever answers on the hub address is believed once. The hub MUST present a certificate authority after its certificate, and its certificate MUST verify against that authority for the hub's name; otherwise the connection is refused before the secret is sent. The authority is stored in `meta/managed/hub/ca` and printed with its SHA-256 | Verified against the stored authority. First use never happens twice: a client holding a root ignores the trust key |
| neither | Verified against the system CA pool, which a hub serving a certificate from its own authority fails. `ze.managed.tls.insecure` turns verification off for development | The config's `ca`, or the system pool |

The stored root stays the client's anchor after the first fetch whenever its
config names no `ca`, so a fleet provisioned with `--hub-ca` or first use needs
no `pki ca` block in each client's config. A `ca` named in the config wins
over it. `ze init --managed --force` replaces the stored root.

Trust on first use is for a network the operator trusts at provisioning time.
It moves the question "is this my hub" from every connection to the first one,
and it does not answer it: compare the printed SHA-256 with the hub's root
(`ze show pki local-ca pem` on the hub) to answer it.

For first use to have an authority to keep, the hub presents its root after
the certificate it issues. A client that already holds the root ignores it. A
hub that serves a named certificate (`certificate <name>`) presents what the
operator stored under that name, and first use works against it only when that
chain ends in the issuing authority.
<!-- source: cmd/ze/ze_core_start.go -- readManagedBootstrap, fetchInitialConfig, keepHubRoot, storedHubRoot -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig, FirstUseTLSConfig, presentedAuthority -->
<!-- source: internal/component/plugin/server/managed_serve.go -- withAuthority -->

Priority: CLI flag > env var > config block > store metadata. Store metadata
is read for the hub address, name and secret only while no config is cached.

---

## Security

| Concern | Mitigation |
|---------|-----------|
| Token per client | Each client has its own secret; token bound to name at auth |
| Token in config | Tree frame files are 0600 and directories 0700; token not logged |
| TLS MITM | TLS 1.3 minimum. The client authenticates the hub before it sends its token, against the certificate authority its `ca` leaf names, the root its store holds, or the system CA pool. See "Hub certificate and client trust" below and "First start trust" above. Trust on first use believes the first answerer once, by the operator's explicit choice at `ze init`. `ze.managed.tls.insecure` turns verification off and is for development only. |
| Client impersonation | Per-client secret + name binding; one connection per name |
| Config isolation | Client can only fetch its own config (name implicit from auth session) |
| Multiple servers | Different secrets for different trust levels (local vs. remote) |

### Hub certificate and client trust

The hub serves the pki store certificate its server block names:

```
bus {
    hub {
        server central {
            ip 0.0.0.0;
            port 1791;
            certificate fleet-hub;
            client edge-01 { secret "..."; }
        }
    }
}
```

The name wins whenever it is set, and a name that resolves to nothing is an
error rather than a fallback. With no `certificate` the listener serves a leaf
the daemon's own certificate authority issued. A managed registry with no
serving addresses loads neither a root nor a leaf. Preparing the first managed
address with no named certificate opens the durable authority, generating the
root on first use. The managed server keeps the loaded authority in memory, including when a new serving address needs a different leaf. The
listener reissues the leaf as it ages, and a restart mints another. A client
anchored on the root needs no change for either. Every server block that accepts managed clients serves one
certificate: a config where two of them name different certificates is refused
at load.

At startup an unresolved certificate leaves the managed server built with
nothing bound and each affected address reported unbound in doctor. Use
`edit pki certificate <name>` to supply the leaf, matching private key and chain
for the configured name, or remove the `certificate` leaf, and commit: the
reload binds the addresses and serves the configured certificate without a
restart. While the name remains unresolved, any reload that still names it is
refused at the certificate gate.

If the hub needs its own authority at startup and cannot load it, the managed
server stays unavailable. `ze data check` checks the store's integrity. A
clean result can still accompany a missing key or a mismatched pair. Restore
the original `meta/ca/cert` and `meta/ca/key` from a trusted copy, or contact
the CA administrator if recovery requires a trust change. Restart the hub
after the repair. A signer that loaded but failed to issue a leaf reports its
signing error on each affected address. `ze show pki local-ca pem` exports
the public issuer for comparison with the root trusted by the clients.

Config reloads update the managed client set, serving addresses and managed
certificate without restarting the hub. At the certificate gate before any
consumer applies, `runReloadContext` calls `prepareManagedReload`
(`PrepareManagedListeners`), which resolves the candidate's named chain or
prepares an issued leaf covering loopback and every explicit serving host. An
unresolved name, an unusable pair or a failed leaf replacement for a serving
named chain refuses the candidate whole and restores the previous PKI store
(`restorePKIAfter`). After candidate promotion, `ApplyManagedListeners` installs
the prepared certificate source before binding added addresses, admits newly
declared clients, closes sessions of removed clients or clients whose secret
changed, and closes removed addresses. Existing sessions on unchanged listeners
keep their sockets and the certificate they negotiated; the next handshake
presents the new certificate. If leaf issuance fails for an added host while
already serving the issued leaf, the existing leaf stays and the added address
is left unbound and reported in doctor.

<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer, prepareManagedReload -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext -->
<!-- source: internal/component/plugin/server/managed_apply.go -- PrepareManagedListeners, ApplyManagedListeners -->

A client authenticates the hub in one of two ways:

| The hub certificate | The client configuration |
|---------------------|--------------------------|
| Comes from a CA in the client's trust store | Nothing. Verification against the system CA pool is the default |
| Comes from a private CA, or from the hub's own authority | `ca`, naming a `pki ca` entry that holds the issuing root |

```
pki {
    ca fleet-hub-root {
        certificate "MIIB...";
    }
}

bus {
    hub {
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "...";
            ca fleet-hub-root;
        }
    }
}
```

`ze show pki local-ca pem` on the hub prints its authority root, which is where
the operator copies it from. The root is public material: the command prints the
certificate, its subject and its expiry, and the private key has no accessor.
The `certificate` leaf of a `pki ca` block takes that PEM document as it stands,
in quotes, headers and line breaks included. Base64 DER is read too, so a config
written before is unchanged. Then name that block from `ca`.

The anchor is the ISSUER, so a hub that presents a new certificate after a
restart stays reachable with no client change. A hex fingerprint could not do
that: it named one certificate and died with it. Ze no longer accepts the
`certificate-fingerprint` leaf, and a config still carrying it is refused with
an error naming `ca <pki-ca-name>` as the replacement.

The client sends its token only after the handshake, so a certificate the client
cannot authenticate ends the connection before the token is written. When a
running client fails before authentication succeeds, it classifies the failure
by step and x509/TLS error type, attaches the doctor code and fix to its
reconnect warning, and reports the failure to its daemon owner so `ze doctor`
(the runtime `hub-session` check) surfaces `doctor-hub-certificate-unverified`
(unknown issuer, hostname mismatch, validity dates, or undefined `pki ca`),
`doctor-hub-handshake-failed`, or `doctor-hub-auth-rejected`. On the hub, a
connection that terminates with a certificate-rejection TLS alert is recorded
under `doctor-managed-certificate-refused` per remote address alongside any
devices last seen from that address; any other pre-identification handshake
failure is recorded under `doctor-managed-client-rejected`. Both sides clear
their failure state as soon as a session authenticates.
<!-- source: internal/component/plugin/server/managed_serve.go -- managedCertificate, managedLeafHosts -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig, ClientTLSConfig -->
<!-- source: internal/component/managed/session_failure.go -- classifySessionFailure, HubSessionFailure -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- isCertRejectionAlert, recordCertRefused, recordUnidentified -->
<!-- source: internal/component/pki/show.go -- handleShowPKILocalCAPEM -->
<!-- source: internal/component/config/retired.go -- retiredKeywords, RetiredKeywordHint -->

---

## What's New vs. Reused

| Component | Status |
|-----------|--------|
| Hub TLS listener | Reuse as-is (now per `server` block) |
| Hub auth (`#0 auth`) | Extend: per-client secret lookup under `server` block |
| Hub MuxConn | Reuse as-is |
| Hub live tree | Shared Storage handle (client configs are entries) |
| `ze data rm` | Reuse as-is, with the hub stopped: a running hub refuses the removal of a client config, which is a configuration key |
| Named `server`/`client` hub blocks | **New:** replaces flat `listen`/`secret` fields |
| `config-fetch` / `config-changed` RPCs | **New:** hub-side handlers |
| Config change watcher | Hub notifies on stored config writes |
| Managed client component | Connection, fetch, cache, reconnect |
| `ze start` managed mode | Detect managed mode from store metadata or cached config |

---

## Non-Goals

| Not included | Why |
|-------------|-----|
| New server component | Hub already exists; extend it |
| Special metadata keys for the running hub connection | Config declares the hub connection; the `meta/managed/hub/` address and secret bootstrap the first start only. The stored root is the exception: it anchors the hub whenever the config names no `ca` |
| Multi-hub replication | One hub sufficient; HA via client cached config |
| Incremental config updates | Full config on change; configs are small (< 10KB typically) |

---

## Implementation

Reference spec: `plan/spec-fleet-config.md`

| Package | Purpose |
|---------|---------|
| `pkg/fleet/` | Shared types: version hash, config envelope, RPC verbs |
| `internal/component/plugin/server/` | Hub extensions: named server/client blocks, per-client auth, config-fetch handler |
| `internal/component/managed/` | Client: connection manager, reconnect, heartbeat |
| `cmd/ze/main.go` | cmdStart() extended for managed mode detection + CLI flag overrides |
<!-- source: internal/component/plugin/server/ -- hub server infrastructure -->
<!-- source: internal/component/config/storage/ -- shared live-store interface -->
