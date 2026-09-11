# Configuration Reload

Ze supports live configuration reload without restarting the daemon. Changed peers are updated, new peers are added, and removed peers are disconnected.
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, doReload -->

## Triggering a Reload

```bash
ze signal reload                    # Via SSH command
kill -HUP $(pidof ze)               # Direct signal
```

## What Can Change Live

| Change | Effect |
|--------|--------|
| New peer added | Session initiated |
| Peer removed | Session torn down with NOTIFICATION |
| Peer settings changed | Session restarted with new config |
| Plugin config changed | Plugin reloaded |
| Static routes changed | New routes announced, old withdrawn |
| Capability changes | Session restarted to renegotiate |
| A user given a new `plaintext-password` | Ze hashes the leaf during the load, and the user logs in with the new password |
| A user removed | The user stops authenticating at once on the web password, the web session cookie, the SSH password, the SSH public key, and `Bearer <user>:<pass>` over REST and gRPC |
| An API token added, rotated, or removed | The running REST and gRPC servers rebuild their credentials without rebinding |
| A web `certificate` reference changed | The listener serves the new chain on the next handshake with no rebind |
| A looking-glass `certificate` reference changed | The same, on the looking-glass listener. A name the new store does not define refuses the whole commit and puts the prior store back |

<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName and the looking-glass certificate gate -->
<!-- source: cmd/ze/hub/listener_migrate.go -- updateLGCertificate -->

The reload runs the same loader as daemon start, so it applies the password transform
with no branch of its own. Every load that hashes a leaf warns that the file still
holds the plaintext. Refer to
[Passwords in a config file](authentication.md#passwords-in-a-config-file).
<!-- source: internal/component/config/loader.go -- LoadConfig calls ApplyPasswordHashing, warnPlaintextOnDisk -->

## What Requires Restart

| Change | Why |
|--------|-----|
| BGP globals (`router-id`, `local { as }`) | Affects all peers, requires full restart |
| Hub listen address/port | Listener cannot be changed at runtime |
| SSH server settings | Server cannot be reconfigured live |
| The web or MCP authentication MODE | Both choose it once, when they are built. A reload that asks for a different mode fails the whole commit before anything is applied |

A user removal governs NEW connections. An open SSH session and an open SSE
stream both survive it by design, because an operator may be editing their own
user. A session a remote backend granted (RADIUS, TACACS+) is not revoked by the
local user list, which never authenticated it.
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers -->
<!-- source: internal/component/web/auth.go -- SessionStore.validateToken, webSession -->
<!-- source: internal/component/ssh/pubkey.go -- authenticatePublicKeyResult -->

## Error Handling

If the new configuration fails to parse:
- The daemon continues running with the previous configuration
- An error is logged with details about the parse failure
- No peers are affected

The same applies when the configuration parses but a registered value validator
refuses one of its values (an IS-IS hostname outside 7-bit ASCII, an invalid
NET, an unregistered internal plugin name). The reload fails on the same branch,
the staged candidate is cleared, and the daemon keeps the configuration it is
already running. There is no override and no force flag. The message is
`reload error: reload: parse config: config validation failed: ...` and it names
the section, the leaf and the rule. See "Upgrading from a release that validated
only on demand" in `docs/guide/configuration.md`.
<!-- source: internal/component/config/loader.go -- LoadConfig refuses through ValidateCustomSections -->
<!-- source: cmd/ze/hub/main_reload.go -- runReload loadErr branch, before ReloadConfig -->

If a plugin reload fails:
- The daemon logs the error
- In-process BGP continues with the old plugin state

## Signals

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration (add/remove/update peers) |
| `SIGTERM` / `SIGINT` | Graceful shutdown (NOTIFICATION Cease to all peers) |
| `SIGUSR1` | Dump status to stderr |
<!-- source: cmd/ze/hub/main.go -- signal.Notify for SIGINT/SIGTERM/SIGHUP -->

A SIGTERM that arrives while a reload is running does not cut it short. Shutdown
waits up to 3 seconds for the reload to report `sighup reload complete` or
`reload error: ...`, so the answer to a SIGHUP is never lost with the process. A
reload still running after those 3 seconds is left behind, and the daemon prints
`shutdown: config reload still running after 3s, stopping without its result`.

Shutdown also stands the config TRANSACTION down before it closes any plugin
connection. A closed connection is indistinguishable from a crashed plugin, so
closing first made the orchestrator elect a rollback and restart plugins the same
shutdown was about to kill, printing a burst of WARN lines saying plugins had
crashed when none had. The cancellation names shutdown as its cause, and a
transaction cancelled for that cause emits no abort and no rollback: there is no
running system left to restore. The wait is bounded at 3 seconds. Every other
cancellation still rolls back, including a reload that exceeds its own 30-second
deadline.
<!-- source: internal/component/config/transaction/orchestrator.go -- ErrShutdown cause handling -->
<!-- source: internal/component/plugin/server/reload.go -- stopTransaction before cleanup -->
<!-- source: cmd/ze/hub/main_reload.go -- awaitReloadWorker, reloadShutdownGrace -->
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler, SIGUSR1 status dump -->

## Reload Workflow

1. Ze reads the config file from disk
2. Parses and validates against YANG schemas, then applies the registered
   `ze:validate` value validators. A refusal stops the reload here, before any
   diff is computed
3. Computes the diff between old and new config
4. For each removed peer: sends NOTIFICATION Cease, closes session
5. For each new peer: creates session, initiates connection
6. For each changed peer: tears down old session, starts new one
7. Plugins receive config-verify then config-apply callbacks
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->
<!-- source: internal/component/bgp/reactor/config.go -- PeersFromTree, config diff -->

## The Order of a Commit

`commit` in the config editor and `ze signal reload` reach the same reload, so
the order below is the same for both. The editor sends the
`ze-system:daemon-reload` request over the daemon socket, and the daemon runs
the reload that SIGHUP runs.
<!-- source: internal/component/cli/reload.go -- newSocketReloadNotifier -->
<!-- source: cmd/ze/hub/main.go -- reloadAfterCommitContext -->
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, doReloadContext -->

Ze applies one commit in this order:

1. Ze reads and validates the whole new configuration. A refusal stops the
   commit here, and nothing on the host changes.
2. Sessions the new configuration removes are stopped.
3. Addresses the new configuration removes are taken off their interface.
4. Addresses the new configuration adds are put on their interface.
5. Sessions the new configuration adds are started.

Steps 2 to 5 hold because a session declares the local address it binds. Ze
keeps an address on the host until the last session bound to it is stopped. Ze
puts a new address on the host before it starts a session that binds it.
<!-- source: internal/component/config/transaction/depgraph.go -- addDerivedEdges -->

Any interruption is bounded by the commit. The sessions stop, the addresses
change, and the sessions come back.

### Which Changes Interrupt a Session

| The change | The effect on a protocol bound to the address |
|------------|-----------------------------------------------|
| The interface MTU, or its description | No interruption. The address does not move |
| The address is removed from the configuration | The sessions bound to it are stopped, in step 2 |
| A new address, with new sessions on it | The address is on the host before the first session starts |
| The address moves to another interface | Read the limitation below |

The DHCP server is an example of a listener an address move does not reach at
all. It binds every address on one device rather than one address, so an address
that moves does not invalidate its socket.
<!-- source: internal/plugins/dhcpserver/socket_linux.go -- listenDHCP -->

### Limitation: an Address That Moves Between Interfaces

An address that moves from one interface to another does NOT stop the BGP
sessions bound to it. A peer names the local address it binds. A peer does not
name the interface. A move therefore leaves the peer's own configuration byte
for byte identical, and Ze emits no stop and no start for it.
<!-- source: internal/component/bgp/plugin/operation.go -- decomposeBGPOperations -->

The session therefore keeps running while the address leaves one interface and
arrives on another. For that time it holds a binding to an address the host does
not have. The listener for the address is stopped and started again from the
kernel's own address notifications. That is a separate path from the commit, and
it is not ordered against the commit.
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddrRemovedPayload, handleAddrAddedPayload -->

To move an address with no such window, use two commits. Remove the peers that
bind it in the first commit. Add them again in the second commit, after the
move.

Ze owes a stop and a start for this case. The full order, and the part of it Ze
does not yet do, is in
[Config apply ordering](../architecture/config/apply-ordering.md).

## Best Practices

- Always validate before reload: `ze config validate config.conf`
- Use `ze config diff old.conf new.conf` to preview changes
- Monitor `ze cli bgp monitor event state` during reload to watch sessions
