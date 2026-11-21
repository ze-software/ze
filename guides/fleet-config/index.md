# Fleet Configuration

One Ze, the hub, holds the full configuration of every other Ze, the managed
clients. Each client connects to the hub over TLS, fetches its own
configuration and applies it. When you change a client's configuration on the
hub, the hub tells that client, and the client fetches and applies the change.
<!-- source: cmd/ze/hub/managed_server.go -- startManagedServer -->
<!-- source: internal/component/managed/client.go -- RunManagedClient -->

> **Status:** Implemented. The hub serves each client its configuration, and
> a client fetches, applies, caches and reconnects.

```
        Hub                                Managed clients
    ┌──────────────┐                    ┌─────────────┐
    │ ze           │ ◄──── TLS ──────── │ ze  edge-01 │
    │              │                    └─────────────┘
    │ client-edge-01.conf               ┌─────────────┐
    │ client-edge-02.conf ◄─── TLS ──── │ ze  edge-02 │
    └──────────────┘                    └─────────────┘
```

What you get, and what you do not:

| Fleet configuration does | Fleet configuration does not |
|--------------------------|------------------------------|
| Keep one full configuration for each client, on the hub | Build a configuration from a template: each client has its own file |
| Push a change to the one client it belongs to | Roll a change out to a group of clients in stages |
| Keep a client running when the hub is down | Record which version each client runs |

This guide follows the order of a first deployment:

1. [Set up the hub](#1-set-up-the-hub)
2. [Write each client's configuration](#2-write-each-clients-configuration)
3. [Provision a client](#3-provision-a-client)
4. [Start the client](#4-start-the-client)

Then the daily work: [change a client's configuration](#change-a-clients-configuration),
[what happens when the hub is down](#when-the-hub-is-down),
[how a client leaves and rejoins the fleet](#leave-and-rejoin-the-fleet),
[what to do when a client holds](#when-a-client-holds) and
[troubleshooting](#troubleshooting). The [reference](#reference) is at the end.

## 1. Set Up the Hub

The hub is an ordinary Ze. It becomes a hub when one `server` block of its
configuration lists the clients it accepts.

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret-of-32-character";
            client edge-01 { secret "edge-01-secret-of-at-least-32-chars"; }
            client edge-02 { secret "edge-02-secret-of-at-least-32-chars"; }
        }
    }
}
```

Three rules apply to this block:

- **One entry and one secret for each client.** The name is the client's
  identity. The secret belongs to that name only: the hub refuses it under
  another client's name. A secret has 32 characters or more.
- **The clients need their own port.** The first `server` block is the one the
  hub's local plugins use. The block that lists clients is a different block
  with a different port, `1791` here.
- **A client connects to the block that lists it.** With two blocks that list
  clients, the hub refuses a client on the address of the other block. List a
  client under both blocks if it must reach both.
<!-- source: internal/component/plugin/server/managed_serve.go -- ManagedListener, handleConn -->
<!-- source: internal/component/plugin/doctor/check_managed_listener.go -- diagnoseManagedListener -->

Start the hub with `ze start`. It also needs the root of its certificate
authority later, in step 3. Print it now and save it to a file:

```bash
ze show pki local-ca pem > hub-root.pem
```

The hub makes this authority at its first start and keeps it. The root is
public: it holds no private key.
<!-- source: internal/component/pki/show.go -- handleShowPKILocalCAPEM -->

## 2. Write Each Client's Configuration

The hub stores one full configuration for each client, under the name
`client-<name>.conf`. Write it as you write any Ze configuration, and add the
`client` block that tells the client where its hub is:

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-of-at-least-32-characters";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "edge-01-secret-of-at-least-32-chars";
        }
    }
}

bgp {
    ...
}
```

The `client` block must be there. The name and the secret are the ones in the
hub's `client edge-01` entry. A client that applies a configuration with no
`client` block stops following its hub.
<!-- source: internal/component/managed/client.go -- adoptEndpoint -->

Give the file to a running hub, on the hub's own host:

```bash
ze data import client-edge-01.conf
```

The hub keeps it as a new version of that client's configuration. It refuses a
name that has no `client` entry in step 1, because no client would fetch it.
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreConfigFromSource -->

## 3. Provision a Client

On the new client, `ze init --managed` asks for what the first start needs.
It asks for the usual SSH login, then for three things about the hub:

| Prompt | What to enter |
|--------|---------------|
| `name` | The client's name on the hub, `edge-01` |
| `hub address` | The hub's address and port, `10.0.0.1:1791` |
| `hub token` | The secret of this client's entry on the hub |

You also choose how the client makes sure that it talks to your hub and not
to something else. The client checks the hub before it sends its secret.
Choose one of these:

| Choice | Command | When to use it |
|--------|---------|----------------|
| Give the hub's root | `ze init --managed --hub-ca hub-root.pem` | The safe choice. Copy `hub-root.pem` from step 1 to the client first |
| Trust on first use | `ze init --managed --hub-trust-first-use` | When you cannot copy a file to each device, and you trust the network during provisioning |
| Neither | `ze init --managed` | Only when a public certificate authority issued the hub's certificate |

**Give the hub's root.** The client checks every connection, the first one
included, against that root. It refuses a hub that the root did not issue.

**Trust on first use.** The first start accepts the hub that answers, one
time. It keeps the certificate authority that this hub presents and prints
its SHA-256. It checks every later connection against the authority it kept.
To be sure of what it kept, compare the printed SHA-256 with the hub's root.
First use never happens a second time. To trust a different hub, run
`ze init --managed --force` again.

With either choice, the client keeps the root in its store. Its configuration
then needs no certificate settings at all.
<!-- source: internal/plugins/init/main.go -- managedInitFromFlags, runInitPublishedProfile -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig, FirstUseTLSConfig -->

To provision with no prompt, give the seven answers on standard input:

```bash
echo -e "admin\nsecret\n127.0.0.1\n2222\nedge-01\n10.0.0.1:1791\nedge-01-secret-of-at-least-32-chars" \
    | ze init --managed --hub-ca hub-root.pem
```

The seven lines are: SSH user, SSH password, SSH address, SSH port, client
name, hub address, hub token.

## 4. Start the Client

```bash
ze start
```

On this first start the client connects to the hub, checks it, fetches its
configuration, checks the configuration, keeps a copy and starts. Every later
start is the same command.

You can leave the hub address and the token empty at `ze init`. Then give them
at the first start. The flags go before `start`:

```bash
ze --server 10.0.0.1:1791 --token "edge-01-secret-of-at-least-32-chars" start
```

When the first start lacks a value, it stops and names the value and the flag
that gives it.
<!-- source: cmd/ze/ze_core_start.go -- cmdStartManaged, readManagedBootstrap, fetchInitialConfig -->

## Change a Client's Configuration

Edit the client's file and give it to the running hub again:

```bash
ze data import client-edge-01.conf
```

The hub tells `edge-01` at once. The client then does four things:

1. It fetches the new configuration.
2. It checks it with the same checks as `ze config validate`.
3. It applies it.
4. It tells the hub the result.

A client that refuses the configuration keeps the one it runs. The hub shows
the refusal and its reason in `show doctor`.

A change made on the client while it fetches yours is kept. The client
applies your configuration only over the one it ran when it fetched. When its
configuration changed in between, it applies nothing, fetches once more, and
holds (see "When a Client Holds").
<!-- source: internal/component/managed/client.go -- holdOrApply, activeCheck, refetchOverLocalCommit -->

A change reaches the client it belongs to and no other. To change ten
clients, import ten files. A client that is not connected gets the change the
next time it connects.
<!-- source: internal/component/plugin/server/managed_serve.go -- NotifyConfigChanged, pushConfigChanged -->
<!-- source: internal/component/managed/client.go -- fetchAndProcess -->

**To move a client to another hub, or to change its secret,** change the
`client` block in its configuration. The client applies the configuration and
reconnects with the new values. It needs no restart. Change the secret in the
hub's `client` entry at the same time, or the client cannot come back.
<!-- source: internal/component/managed/client.go -- adoptEndpoint -->

**To restore a client's configuration from a backup,** on a running hub:

```
request data restore path /var/backups/edge-01.zefs config client edge-01
```

The hub pushes the restored configuration as it pushes any change.
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreConfigFromSource -->

**To edit with the configuration editor,** on the hub's own host, in the
folder that holds the hub's store (its `ze.config.dir`):

```bash
ze config edit client-edge-01.conf
```

The command opens the store of the folder its path names, so a bare name
typed in another folder finds no store there.
<!-- source: internal/core/resolve/resolve.go -- StoreDir -->

The editor runs inside the running hub. When you commit, the hub records a new
version of that client's configuration and pushes it to the client, as it
pushes an import. The file must exist first: give the hub the first version
with `ze data import`.

The hub refuses to open the editor on the configuration of a client that its
own configuration does not declare, as it refuses to import one. Add the
`client` entry to the hub first. The client does not have to be connected: a
client that is away gets the change the next time it connects.
<!-- source: cmd/ze/hub/session_factory.go -- undeclaredClientSessionRefusal -->

The history and the archive of a client's configuration use the same name:

```bash
ze config history client-edge-01.conf
ze config archive backup client-edge-01.conf
```
<!-- source: internal/component/config/cli/cmd_edit.go -- runStoredEditor -->
<!-- source: cmd/ze/hub/session_factory.go -- buildSessionModelFactory -->
<!-- source: internal/component/cli/editor_commit.go -- publishCommitted -->
<!-- source: internal/component/config/cli/cmd_history.go -- config history command -->
<!-- source: internal/component/config/cli/cmd_archive.go -- config archive command -->

## When the Hub Is Down

A client does not need its hub to run.

| Situation | What the client does |
|-----------|----------------------|
| The hub goes down while the client runs | It keeps its configuration and tries the hub again |
| The client starts and the hub is down | It starts from the copy it kept, and tries the hub in the background |
| The hub comes back | The client reconnects and fetches the configuration if it changed |
| The first start, with no copy yet, and the hub is down | It stops with an error: it has no configuration to run |

The client waits 1 second before the first new attempt, then twice as long
each time, up to 60 seconds.
<!-- source: internal/component/managed/reconnect.go -- newBackoff -->
<!-- source: internal/component/managed/client.go -- RunManagedClient -->

## Leave and Rejoin the Fleet

A client can leave its fleet and rejoin it. Neither command changes the
client's configuration, and neither needs a restart of a client that was
started as a managed client.

### Leave the Fleet

On the client:

```bash
ze request fleet disable
```

The command does two things, in this order:

1. It stores `meta/instance/managed` as `false`.
2. It closes the connection to the hub, and answers when the connection is
   closed. If the client was applying a configuration from the hub at that
   moment, it applies that configuration first.

The client keeps running the configuration it has. What the hub writes for
that client after this does not reach it.

The command needs no hub. It works when the hub cannot be reached, when the
client has no connection, and when the client's configuration names no hub.
It also works when the client is stopped: it stores the flag, and the next
`ze start` starts the client outside the fleet.
<!-- source: internal/component/managed/cmd_fleet.go -- disableOnDaemon, disableStopped, changeFleetLocal -->
<!-- source: cmd/ze/hub/managed_owner.go -- managedClientOwner, stopWith, join -->

### Rejoin the Fleet

On the client:

```bash
ze request fleet enable
```

The command stores `meta/instance/managed` as `true`. What happens next
depends on the client:

| The client | What the command does |
|------------|-----------------------|
| Runs, and was started as a managed client | It starts the hub connection at once, from the configuration the client runs at that moment. The client does not restart |
| Runs, and was started outside the fleet | It stores the flag only. The answer says that the hub connection starts at the next `ze start` |
| Is stopped | It stores the flag. The next `ze start` starts the client as a managed client |

The command is refused, and the flag stays as it was, while the client's
configuration has no `client` block. Such a client has no hub to follow. Add
the block to the client's configuration on the hub, then run
`ze init --managed --force` on the client.

What a client does after it rejoins depends on what happened while it was
away:

- Nobody changed the client. It follows its hub: it fetches the hub's
  configuration, and applies it if it is a different one.
- The client was changed. It holds, and `ze doctor` on the client reports
  `doctor-managed-diverged` with the two ways to end the hold. See
  [When a Client Holds](#when-a-client-holds).
<!-- source: internal/component/managed/cmd_fleet.go -- enableOnDaemon, enableStopped, requireDeclaredHub, noDeclaredHub -->
<!-- source: cmd/ze/hub/managed_owner.go -- managedClientOwner, startWith -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline -->

### See Where a Client Stands

```bash
ze show fleet status
```

The answer says whether the client is managed, which hub its configuration
names, and the two hashes that decide a hold. Three fields describe the hub
connection of a running client:

| Field | Value | Meaning |
|-------|-------|---------|
| `client` | `running` | The client is connected to its hub, or tries to connect |
| `client` | `stopping` | `request fleet disable` is closing the hub connection. The status does not wait for the close |
| `client` | `stopped` | The client was started as a managed client and makes no attempt to connect: it left the fleet, its configuration names no hub, or its hub block could not be read at start |
| `client` | `not-started` | The client was started outside the fleet. A hub connection starts only at its next `ze start` |
| `connection` | `connected`, `not-connected` | Whether the hub accepted the client and that connection is still open |
| `holding` | `holding`, `not-holding` | Whether the client took nothing from its hub at its last fetch |

When the client is stopped, each of the three fields says `no-daemon`.

Use the two commands to leave and to rejoin. Another write of
`meta/instance/managed`, with `update data key` or `ze data write`, decides
the next start and does not close a connection that is open.
<!-- source: internal/component/managed/cmd_fleet.go -- daemonClientReport, reportOf, noDaemonReport -->
<!-- source: internal/component/managed/client.go -- RunManagedClient -->

## When a Client Refuses Your Change

A managed client takes its configuration from its hub. While it is managed,
it refuses a change that you make on the client itself, with the daemon
running or stopped. The refusal reads:

```
the configuration was not changed: this device is managed by a hub
local changes are frozen while it follows a hub, so that a hub push cannot overwrite one
fix: request fleet disable
```
<!-- source: internal/component/config/storage/freeze.go -- ErrConfigFrozen, refuseFrozenConfigLocked -->

A commit in the editor, an SSH configuration upload, an import, a restore and
a rollback all give this problem, reason and fix, and the fix is always the
last line. Some of them
put the command or the step that failed in front of the first line, for
example `write candidate:`.

With the daemon stopped, `ze config rollback <N> <file>` is the exception: it
rolls back the file it names on disk and does not touch the stored
configuration, so it is not refused. A configuration read from a file is
never frozen.
<!-- source: internal/component/config/storage/cli/data_rpc_config.go -- daemonConfigImportRefusal -->
<!-- source: cmd/ze/hub/config_upload.go -- Upload -->
<!-- source: internal/component/config/cli/cmd_rollback.go -- rollbackStoredConfig, storedRollbackRefusal, cmdRollbackThrough -->

Nothing was written. The client runs the configuration it ran before.

To change the configuration of a client that stays in the fleet, change it
on the hub (see "Change a Client's Configuration").

To change it on the client, take the client out of the fleet first:

```bash
ze request fleet disable
```

The client then accepts your change. `show fleet status` reports `frozen` as
`false`. When the client rejoins with `request fleet enable`, it holds until
its hub has the same configuration (see "When a Client Holds").
<!-- source: internal/component/managed/cmd_fleet.go -- readFleetStatus -->

The refusal covers the commands that record a new version of the
configuration. These change the client's configuration without a refusal:

- a raw write of a store key, with `ze data write` or `update data key`,
  such as a write of the active version pointer back to an earlier version
- the restore of a whole store
- a rename of a stored configuration
- the first import of a configuration name the store does not hold yet

A change made that way is not lost either: the client holds when it next
compares its configuration with its hub's.
<!-- source: internal/component/config/storage/import_config.go -- ImportConfig -->

A daemon that was started from a configuration file or from standard input,
and not from its store, follows no hub. It refuses nothing: its input is its
configuration. `show fleet status` reports `frozen` as `false` there, even
when `managed` is `true`.
<!-- source: internal/component/config/storage/freeze.go -- refuseFrozenConfigLocked -->

## When a Client Holds

A client remembers which configuration its hub last gave it. When the
configuration it runs is a different one, the client takes nothing from the
hub. It holds.

The hold protects a change made on the client itself, for example during an
emergency. Without it, the next configuration from the hub would replace that
change, and nothing would say so. This is true even when the change is made
while the client fetches a configuration from the hub: the client does not
apply that configuration over the change, and holds.

A client that holds:

- keeps running its own configuration
- stays connected to the hub
- refuses each configuration the hub offers, and tells the hub why

A restart is not a change. A client that nobody changed follows its hub after
a restart, as before.
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, holdFetched -->
<!-- source: internal/component/managed/baseline.go -- Diverged -->

### See That a Client Holds

On the client:

```bash
ze doctor
ze show fleet status
```

`ze doctor` reports `doctor-managed-diverged`, and its fix names the hub
commands that end the hold, with the client's name and the hash of the
configuration it runs: `show fleet divergence name <client>`, then adopt or
revert. For a configuration larger than 1 MiB, which the client does not send
its hub, it names the two manual routes of "End the Hold" instead.
`show fleet status` shows the hash of the configuration the client runs, the
hash of the last one its hub gave it, and `diverged`. Both commands answer
with the client running or stopped. A running client that refused its hub's
configuration at its last fetch also shows `holding` in the `holding` field,
`held-by-hub` with the hash of the copy the hub keeps (`none` when it keeps
none), and `revert-offered`, which says whether the hub offered to discard
the client's change (`offered`) or offered it for a version the client no
longer runs (`stale`).

On the hub, `ze doctor` and `show doctor` list each client whose copy the hub
keeps under `doctor-managed-client-diverged`, with both hashes and the three
commands, with the hub running or stopped. A copy for a client no server
block declares any more is reported as orphaned. A client whose copy the hub
does not keep is listed under `doctor-managed-client-rejected` instead: the
reason starts with `holding:` and gives the same two hashes.

To see what the client changed, on the hub:

```bash
ze show fleet divergence list
ze show fleet divergence name edge-01
```

The list gives each client whose copy the hub keeps, its two hashes, when the
client's configuration was written and when the hub received the copy, and
`declared` false for an orphaned copy. The show of one client adds the
changes from the hub's configuration to the client's, every secret masked,
and their count. It never shows either configuration's text. When the client's
configuration holds a section of an external plugin the hub did not start, the
hub cannot compare the two: the show says so and names the plugins, and adopt
and revert still work.
<!-- source: internal/component/managed/doctor.go -- checkManagedDiverged, managedDivergedFix, checkHubClientsDiverged -->
<!-- source: internal/component/managed/cmd_divergence_show.go -- readDivergenceList, readDivergenceDetail, divergenceParseRefusal -->
<!-- source: internal/component/managed/cmd_fleet.go -- readFleetStatus, reportOf, hubAnswerOf -->
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- managedRejectedRow -->

### End the Hold

The hold ends when the hub and the client hold the same configuration. You
keep the client's change on the hub with adopt, or discard it with revert.

**Keep the client's configuration.** Adopt the copy the hub keeps. Read
its hash in the show, then give that hash to adopt, on the hub:

```bash
ze show fleet divergence name edge-01
ze update fleet divergence name edge-01 adopt version <device-version>
```

`<device-version>` is the `device-version` the show answered. The hub makes
its copy the configuration it serves the client, and tells the client. The
client finds that the hub now holds the configuration it runs, and it stops
holding. It applies nothing: it already runs that configuration.
`show fleet status` on the client no longer shows `diverged`, and the hub no
longer lists the client in `show fleet divergence list`.

Adopt is refused, and writes nothing, when:

- the hash is not the one the hub keeps: the client sent a newer copy after
  you looked. The refusal names the hash the hub keeps. Read the show again
- the hub keeps no copy for the client. The refusal names
  `show fleet divergence list`
- no server block declares the client. Add the client entry with
  `ze config edit`
- a change to `client-edge-01.conf` is staged. Commit or discard it with
  `ze config edit client-edge-01.conf`

Beside a running hub, `ze update` sends the line to it. With the hub stopped
it is refused, fix `ze start`: only the running hub records the audit action
`fleet-divergence-adopt`, so an adopt is taken only there. Adopt needs the
edit permission, as every `update` command does, and the hub records it in
its audit log (`show audit action fleet-divergence-adopt`).
<!-- source: internal/component/managed/cmd_divergence.go -- handleDivergenceAdopt, adoptLocal, adoptHeld, adoptHeldLocked -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, settleBaseline -->

**Keep it by hand.** The hub keeps no copy, and there is nothing to adopt,
when the configuration is larger than 1 MiB, when the hub or the client runs
a version of Ze that does not send or keep the copy, or when the hub refused
the copy. Give the configuration to the hub yourself. On the client, print
the configuration it runs into a file named for the hub:

```bash
ze data cat file/active/edge-01.conf > client-edge-01.conf
```

Copy that file to the hub's host and import it there:

```bash
ze data import client-edge-01.conf
```

The hub tells the client, and the hold ends as it does for adopt. The file
must be the client's configuration byte for byte. If `show fleet status` on the client still shows `diverged` after the
import, the two copies differ.

The client also tells the hub that it no longer holds, and the hub removes
the `doctor-managed-client-rejected` row of that client from `show doctor`.
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, settleBaseline -->
<!-- source: internal/component/config/storage/cli/main.go -- cmdCat, importConfigs -->
<!-- source: internal/component/plugin/server/managed_serve.go -- handleRequest -->

**Discard the client's configuration.** Revert the client from the hub.
Read the two hashes in the show, the copy's `device-version` and the
`hub-version` of the configuration the hub serves the client, then give both
to revert, on the hub:

```bash
ze show fleet divergence name edge-01
ze request fleet divergence name edge-01 revert version <device-version> hub-version <hub-version>
```

The hub stores your decision with its copy and tells the client. At its next
fetch the client applies the configuration the hub serves it, in the same way
as any change from its hub, and it stops holding: `show fleet status` on the
client no longer shows `diverged`, and the hub no longer lists the client. A
client that is not connected takes the revert when it next connects, and the
show answers `revert` `pending` until then.

The decision is bound to the two hashes you gave. When the hub's
configuration for the client was written between your show and your revert,
the revert is refused and stores nothing: read the show again. The client
applies nothing, and holds, when either hash changed after you decided and
before it fetched:

- the client's operator edited again. The client sends the hub its newer
  copy, which replaces the one you reverted, and the show answers `revert`
  `none`. Read the show again, and decide on the newer copy
- the hub's configuration for the client was written again. You did not see
  that configuration when you decided, so the client does not take it. The
  show answers `revert` `void` and, in `revert-void`, the revert to run again
  after you read the diff

Revert is refused, and stores nothing, in the cases where adopt is refused,
and when the hub holds no configuration for the client. It needs the hub to
run: run it again after `ze start`. Revert needs the edit permission, and the
hub records it in its audit log (`show audit action fleet-divergence-revert`).
A client with a confirmed commit open refuses the revert as it refuses any
change from its hub, and takes it at the fetch after the window closes.
<!-- source: internal/component/managed/cmd_divergence.go -- handleDivergenceRevert, revertLocal, revertHeld, revertHeldLocked -->
<!-- source: internal/component/managed/cmd_divergence_show.go -- divergenceRowOf, revertVoidReason -->
<!-- source: internal/component/managed/client.go -- fetchAgainstBaseline, applyReverted -->

**Discard it by hand.** A client whose copy the hub does not keep, or that
runs a version of Ze that ignores a revert, is put back on the version its hub
sent from the client itself. Take the client out of the fleet, put it back
on the version its hub sent, and let it rejoin. On the client,
`ze config history` lists the versions of its configuration. Restore the one
the hub sent:

```bash
ze request fleet disable
ze config history edge-01.conf
ze config rollback <N> edge-01.conf
ze show fleet status
ze request fleet enable
```

`show fleet status` must no longer show `diverged` before the last command.
If it does, the restored version is not the one the hub sent, and the client
holds again when it rejoins.

The client then runs the configuration its hub last gave it, so it does not
hold when it rejoins. It connects to the hub at once and takes the hub's
configuration if the hub's copy changed meanwhile.
<!-- source: internal/component/managed/doctor.go -- managedDivergedManualFix -->
<!-- source: internal/component/managed/cmd_fleet.go -- disableOnDaemon, enableOnDaemon, readFleetStatus -->
<!-- source: internal/component/config/cli/cmd_history.go -- config history command -->

A client provisioned before Ze recorded this, or provisioned again with
`ze init --managed --force`, has no record of what its hub gave it. It does
not hold. It takes the hub's configuration once, and holds from then on when
it is changed.
<!-- source: internal/component/managed/baseline.go -- Diverged -->

## Troubleshooting

Start with `show doctor` on the hub, and `ze doctor` on the client.

| What you see | Cause | Fix |
|--------------|-------|-----|
| The first start stops and names a missing value | `ze init` did not get the hub address, the name or the token | Give it with `--server`, `--name` or `--token` |
| The first start fails on the hub's certificate | The client has no root for this hub | Run `ze init --managed --force` with `--hub-ca` or `--hub-trust-first-use` |
| The client fails on the hub's certificate after a hub was replaced | The client holds the root of the old hub | Run `ze init --managed --force` with the new hub's root |
| `doctor-managed-client-rejected` on the hub, wrong secret or unknown name | The client's name or secret does not match a `client` entry on the block it connects to | Correct the `client` entry on the hub, or the `client` block in the client's configuration |
| `doctor-managed-client-rejected`, name already connected | A second Ze uses the same client name | Give each client its own name and entry |
| `doctor-managed-client-rejected`, the client refused its configuration | The configuration does not pass the client's checks | Read the reason in the row, correct the file, and import it again |
| `doctor-managed-listener-unbound` on the hub | The hub cannot listen on the address of the clients' block | Correct the address or free the port |
| `doctor-hub-managed-collision` on the hub | The clients' block uses the address of the first `server` block | Give the clients' block its own port |
| `doctor-managed-client-notify-dropped` on the hub | A connected client did not accept the notice of a change | Nothing in most cases: the hub disconnects that client, and it fetches when it reconnects |
| `doctor-hub-unreachable` on the client | The client cannot reach the hub's address | Check the address and the route. The client keeps running and retrying |
| The client logs `doctor-hub-unreachable` after a change, with `context deadline exceeded` | The hub did not send the configuration within 30 seconds: a large configuration on a slow link, or a hub that stalls. The client keeps the configuration it runs | Check the link to the hub and the hub's load. The client fetches again at the next change, or when it reconnects |
| The client reconnects again and again, with `fetch: ... context deadline exceeded` in its log | The same, when the client connects: each connection ends when the fetch reaches its 30 seconds | The same. The client keeps running its configuration meanwhile |
| The first start fails with `context deadline exceeded` after 30 seconds | The hub did not send the first configuration in time. The client writes no configuration | Check the link to the hub, then run `ze start` again |
| `doctor-hub-client-missing` on the client | The client applied a configuration with no `client` block, so it follows no hub | Add the `client` block to its configuration on the hub. Then run `ze init --managed --force` on the client, and start it |
| `doctor-managed-diverged` on the client | The client runs a configuration that is not the last one its hub gave it, so it holds | See [When a Client Holds](#when-a-client-holds) |
| `doctor-managed-client-rejected` on the hub, the reason starts with `holding:` | The same: that client holds | See [When a Client Holds](#when-a-client-holds) |
| A client holds after `request fleet enable` | The client was changed while it was out of the fleet | See [When a Client Holds](#when-a-client-holds) |
| `request fleet enable` is refused: the configuration names no hub | The client's configuration has no `client` block, so the client has no hub to rejoin | Add the `client` block to its configuration on the hub. Then run `ze init --managed --force` on the client, and start it |
| `show fleet status` shows `managed` false | The client left the fleet with `request fleet disable`, and takes nothing from its hub | Run `ze request fleet enable` on the client |
| `show fleet status` shows `managed` true and `client` `not-started` | The client was started outside the fleet, and `request fleet enable` ran after that | Restart Ze on the client: the hub connection starts at `ze start` |
| `show fleet status` shows `managed` true and `client` `stopped` | The client's configuration names no hub | See the `doctor-hub-client-missing` row |
| `show fleet status` shows `managed` true and `client` `stopped`, and the configuration names a hub | The client left the fleet with `request fleet disable`, and `meta/instance/managed` was then written back to `true` with `ze data write` or `update data key`. A raw write starts no client | Run `ze request fleet enable` on the client |
| `show fleet status` shows `managed` true and `client` `stopped`, and the client logged `config-hub-invalid` at start | The client could not read the `client` block of its configuration, so it started no hub client | Correct the `client` block in its configuration on the hub, and check it there with `ze config validate`. Then run `ze init --managed --force` on the client, and start it |
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedClientRejected, checkManagedListenerUnbound -->
<!-- source: cmd/ze/hub/managed_owner.go -- startAtBoot, state -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- authRefused, notifyDropped -->
<!-- source: internal/component/managed/doctor.go -- checkHubReachable, checkHubClientDeclared, checkManagedDiverged -->
<!-- source: internal/component/managed/client.go -- configFetchTimeout, handleConfigChangedRequest -->
<!-- source: internal/component/managed/cmd_fleet.go -- noDeclaredHub, daemonClientReport, reportOf -->

## Reference

### `ze init` Options for a Managed Client

| Option | Meaning |
|--------|---------|
| `--managed` | Make this Ze a managed client |
| `--hub-ca <file>` | The hub's root, one PEM certificate of a certificate authority. Needs `--managed` |
| `--hub-trust-first-use` | Keep the certificate authority the hub presents on the first connection. Needs `--managed`, and cannot be given with `--hub-ca` |
| `--force` | Replace an existing store, to provision again |
<!-- source: internal/plugins/init/main.go -- Run, managedInitFromFlags -->

### `ze start` Flags

The flags go before the command: `ze --server 10.0.0.1:1791 start`.

| Flag | Environment | Meaning |
|------|-------------|---------|
| `--server <host:port>` | `ze.managed.server` | Hub address |
| `--name <name>` | `ze.managed.name` | Client name |
| `--token <secret>` | `ze.managed.token` | Client secret |
| `--connect-timeout <duration>` | `ze.managed.connect.timeout` | Time allowed to reach the hub, for each attempt: to connect, to check the hub's certificate and, at the first start, to authenticate. Fetching the configuration has its own bound of 30 seconds. Default `5s` |
| (none) | `ze.managed.tls.insecure` | Do not check the hub's certificate. For development only: the client then sends its secret to a hub it did not check |

`--server`, `--name` and `--token` replace what `ze init` stored, and what the
`client` block of the configuration says, for as long as that Ze runs. Use them
for a first start and for troubleshooting.
<!-- source: cmd/ze/ze_core_dispatch.go -- zeParseGlobalFlags -->
<!-- source: cmd/ze/ze_core_start.go -- hubClientEndpoint, readManagedBootstrap -->

### The `client` Block of a Client's Configuration

| Leaf | Meaning |
|------|---------|
| `host`, `port` | The hub's address |
| `secret` | The client's secret on the hub |
| `ca <name>` | Optional. A `pki ca` entry that holds the hub's root. It replaces the root the client's store keeps |
| `source-address <ip>` | Optional. The local address the client connects from |

A client that was provisioned with `--hub-ca` or `--hub-trust-first-use` needs
no `ca`. Use `ca` to put the hub's root in the configuration instead: paste
what `ze show pki local-ca pem` prints into a `pki ca` block, and name that
block.

```
pki {
    ca fleet-hub-root {
        certificate "-----BEGIN CERTIFICATE-----
MIIB...
-----END CERTIFICATE-----";
    }
}
```
<!-- source: internal/component/config/loader_extract.go -- extractHubClientConfig -->
<!-- source: internal/component/managed/tls.go -- clientTLSConfig -->

### What the Client Keeps in Its Store

| Key | Holds | Written by |
|-----|-------|------------|
| `meta/instance/managed` | `true` for a managed client, `false` for one that left the fleet | `ze init --managed`, `request fleet disable`, `request fleet enable` |
| `meta/instance/name` | The client's name | `ze init` |
| `meta/managed/hub/address` | The hub's address, for the first start | `ze init --managed` |
| `meta/managed/hub/token` | The client's secret, for the first start | `ze init --managed` |
| `meta/managed/hub/ca` | The hub's root | `ze init --hub-ca`, or the first start with first use |
| `meta/managed/hub/trust` | `first-use` when first use was chosen | `ze init --hub-trust-first-use` |
| `meta/managed/base-version` | The hash of the last configuration the hub gave this client. See [When a Client Holds](#when-a-client-holds) | The client: at its first start, each time it applies a configuration from the hub, and when the hub confirms that it holds the configuration the client runs |

`request fleet disable` and `request fleet enable` change
`meta/instance/managed` with no change to the client's configuration. See
[Leave and Rejoin the Fleet](#leave-and-rejoin-the-fleet).
<!-- source: internal/component/managed/cmd_fleet.go -- storeManagedFlag -->
<!-- source: pkg/zefs/keys.go -- KeyManagedHubAddress, KeyManagedHubToken, KeyManagedHubCA, KeyManagedHubTrust, KeyManagedBaseVersion -->
<!-- source: internal/component/managed/baseline.go -- StoreBaseline -->
<!-- source: cmd/ze/ze_core_start.go -- isManaged -->

The design, the protocol and the reasons are in
[the architecture page](https://github.com/ze-software/ze/blob/main/docs/architecture/fleet-config.md).
