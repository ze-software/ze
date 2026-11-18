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
[what happens when the hub is down](#when-the-hub-is-down) and
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

**To edit with the configuration editor,** on the hub's own host:

```bash
ze config edit client-edge-01.conf
```

The editor runs inside the running hub. When you commit, the hub records a new
version of that client's configuration and pushes it to the client, as it
pushes an import. The file must exist first: give the hub the first version
with `ze data import`.

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
| `doctor-hub-client-missing` on the client | The client applied a configuration with no `client` block, so it follows no hub | Add the `client` block to its configuration on the hub. Then run `ze init --managed --force` on the client, and start it |
<!-- source: internal/component/plugin/doctor/check_managed_runtime.go -- checkManagedClientRejected, checkManagedListenerUnbound -->
<!-- source: internal/component/plugin/server/managed_doctor.go -- authRefused, notifyDropped -->
<!-- source: internal/component/managed/doctor.go -- checkHubReachable, checkHubClientDeclared -->

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
| `--connect-timeout <duration>` | `ze.managed.connect.timeout` | Time allowed to reach the hub, for each attempt. Default `5s` |
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
| `meta/instance/managed` | `true` for a managed client | `ze init --managed` |
| `meta/instance/name` | The client's name | `ze init` |
| `meta/managed/hub/address` | The hub's address, for the first start | `ze init --managed` |
| `meta/managed/hub/token` | The client's secret, for the first start | `ze init --managed` |
| `meta/managed/hub/ca` | The hub's root | `ze init --hub-ca`, or the first start with first use |
| `meta/managed/hub/trust` | `first-use` when first use was chosen | `ze init --hub-trust-first-use` |

Set `meta/instance/managed` to `false` to make a client stop following its hub
with no change to its configuration.
<!-- source: pkg/zefs/keys.go -- KeyManagedHubAddress, KeyManagedHubToken, KeyManagedHubCA, KeyManagedHubTrust -->
<!-- source: cmd/ze/ze_core_start.go -- isManaged -->

The design, the protocol and the reasons are in
[the architecture page](https://github.com/ze-software/ze/blob/main/docs/architecture/fleet-config.md).
