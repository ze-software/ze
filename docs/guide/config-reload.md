# Configuration Reload

Ze supports live configuration reload without restarting the daemon. Changed peers are updated, new peers are added, and removed peers are disconnected.
<!-- source: cmd/ze/hub/main_reload.go -- handleSIGHUPReload, signalReload, doReloadContext -->
<!-- source: internal/component/bgp/reactor/peer_settings_apply.go -- applyHotSwappableSettings -->
<!-- source: internal/component/bgp/reactor/peer_derived_bindings.go -- bindProcesses, unbindProcesses, restartForBinding -->
<!-- source: internal/component/bgp/reactor/peer_bfd.go -- Peer.applyBFDOptInChange -->

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
| A peer `create bgp peer` built, which the file does not declare | Nothing: the peer keeps its session. `delete bgp peer` removes it, and `update bgp config` writes it into the file |
| Peer settings changed, other than the in-place changes below | Session restarted with new config |
| A configured peer gains, loses or changes a receive-only `attach process` block, one with no `send` and no `open` grant | The peer keeps its session. A process attached to an up session receives a `state up` for the peer with reason `attached` before any message of the session. When its grant covers routes the peer sent and the peer already announced some, Ze sends one ROUTE-REFRESH per negotiated family so the process learns them. A detached process receives a `state down` for that peer alone, and one whose grant changed receives a down, then an up |
| The same change on a session that cannot catch the process up | Session restarted with NOTIFICATION Cease subcode 6 (Other Configuration Change): the peer announced routes and did not advertise Route Refresh, or the grant covers routes Ze sent and Ze already advertised one, or the session reached Established while the reload ran |
| A peer gains, loses or edits a BFD opt-in (`connection bfd`) with no `strict true` before or after: added, removed, set `enabled false` or back, or its profile or mode changed | The peer keeps its session. An Established peer opens its BFD session at once; a removed or disabled opt-in releases it (when no other client holds that session, the BFD plugin puts it in AdminDown, RFC 5882 Section 8, see `bfd.md`); an edited opt-in releases the old session and opens one under the new profile. A peer that is not Established opens BFD when it next reaches Established |
| A BFD opt-in change with `strict true` before or after | Session restarted with NOTIFICATION Cease subcode 6: strict mode is announced in the OPEN and gates OpenConfirm, so only a new session can take it. The restart log names `BFD` |
| A dynamic group's template changed in a way a running session can take: a receive-only `attach process` binding, a filter chain, or a non-strict BFD opt-in | Every running dynamic peer of the group keeps its session and takes the change in place, with the same `state up` reason `attached`, ROUTE-REFRESH and restart cases as a configured peer (above). An import-chain edit applies to the routes received after the reload: a route the peer sent before it and that the new chain rejects stays in `show bgp adj-rib-in` until the peer sends it again |
| A dynamic group's template changed in a way a running session cannot take | Each running dynamic peer of the group is torn down with NOTIFICATION Cease subcode 6 (Other Configuration Change) and is accepted under the new template when it reconnects |
| A dynamic group removed, or its range no longer covers a running dynamic peer | That peer is torn down with NOTIFICATION Cease subcode 3 (Peer De-configured) |
| A reload that fails after the dynamic groups were applied | The previous group set and every in-place change are restored. A dynamic peer the reload tore down is not rebuilt: it is accepted under the restored group when it reconnects |
| Plugin config changed | A plugin that registers config apply handlers takes the change at commit and restores its previous config if the commit fails. `bgp-filter-prefix`, `bgp-filter-aspath`, `bgp-filter-aspath-length`, `bgp-filter-community-match`, `bgp-filter-remove-private-as`, `bgp-filter-community`, `bgp-filter-family`, `bgp-filter-modify` and `bgp-filter-path-asn`: an edited, added or removed definition (`prefix-list`, `as-path-list`, `as-path-length`, `community-match`, `remove-private-as`, `community` and a peer's community filter or blackhole agreement, `family-filter`, `modify` and `defaults attribute`, `reject-asn`) applies to every UPDATE received or sent after the reload; a route stored before it is not re-evaluated until the peer sends it again, and a route already sent to a peer is not sent again under the edited filter until its source sends it again or the receiving peer's session is re-established, when `bgp-adj-rib-in` replays the stored routes to it through the edited egress filters. With `bgp-rs`, a ROUTE-REFRESH from the receiving peer makes the route server ask every other established source that supports route refresh to send its routes again. `bgp-watchdog`: a pool entry the reload keeps stays announced or withdrawn as it was, an announced entry whose attributes changed is announced again with the new ones, an announced entry the reload removes is withdrawn, and a new entry starts as it would at startup. `bgp-healthcheck`: an unchanged probe keeps running, a removed or changed probe stops and withdraws its route without holding the commit, and a new or changed probe starts its checks after the stopped probe of the same name or group has withdrawn; a failed commit restarts the previous probes from INIT. `bgp-rib`: an edited `multipath maximum-paths` (back to 1 when removed; outside 1 to 256 the commit is refused), `multipath relax-as-path` or blackhole honoring rule reselects every stored prefix at the reload, without waiting for a new UPDATE, and a failed commit reselects under the previous rules; `as-notation` is recorded by the daemon when the reload commits. `bgp-role`: an edited `strict` takes effect at the peer's next OPEN and an edited `export` applies to every UPDATE sent after the reload; the role a peer declared in its OPEN, whether or not the peer has a `role` block, is forgotten only when a commit removes that peer (or, for a dynamic peer, its group), never by an edit elsewhere or a failed commit. A change to a peer's role value is refused, because it is sent in the OPEN. `routing-table`: an edited table set is installed at commit, and `static` resolves a route's table against the tables of the same commit, so one commit can add a table and a route in it. `as112` and `geodns`: the edited `service as112` or `service geodns` config is installed at commit. `dhcpserver`: a subnet whose prefix, ranges and static mappings are unchanged keeps its leases, even when its options or the PXE settings change; a subnet whose addresses change starts with no leases. A reload that names a `listen-interface` the server cannot bind fails, and the server keeps its previous config. `tftpserver`: the listeners are bound again and a transfer already running is cut, so the client fetches the file again under the new config. A reload that names a `listen-interface` the server cannot bind fails, and the server keeps its previous config. `imageserver`: the HTTP servers are replaced, which cuts a download in flight. A reload that names a `listen-interface` the server cannot resolve or bind fails, and the server keeps its previous config. For each of these, an edit elsewhere under `service` restarts nothing. `cos`: an edited `class-of-service` profile is installed at commit, and every existing VLAN unit whose maps change, because its profile or its own `ingress-qos-map` or `egress-qos-map` changed, is programmed with exactly the new maps; a unit whose maps are unchanged is left alone, and a failed commit programs the previous maps back. One commit can add a profile and a unit that uses it; removing a profile a unit still names is refused. `exabgp-bridge`: any change under `exabgp bridge` is refused with the restart steps, because the bridge's scripts keep the config they started with. A plugin that wants config but registers no apply handler, and does not declare that it applies nothing, refuses at config-verify every reload that changes a root it declares in `WantsConfig`: the commit fails, the running config stays in force, and the error names the plugin, the changed roots and the restart that delivers the change at startup (see `plugin-development/handlers.md`, "A plugin with no apply handler") |
| Static routes changed | New routes announced, old withdrawn |
| Capability changes | Session restarted to renegotiate |
| A user given a new `plaintext-password` | Ze hashes the leaf during the load, and the user logs in with the new password |
| A user removed | The user stops authenticating at once on the web password, the web session cookie, the SSH password, the SSH public key, and `Bearer <user>:<pass>` over REST and gRPC |
| An API token added, rotated, or removed | The running REST and gRPC servers rebuild their credentials without rebinding |
| A web `certificate` reference changed | The listener serves the new chain on the next handshake with no rebind |
| A looking-glass `certificate` reference changed | The same, on the looking-glass listener. A name the new store does not define refuses the whole commit and puts the prior store back |

<!-- source: internal/component/bgp/reactor/reactor_api.go -- reactorAPIAdapter.ReloadRunning, withPeerEntries -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName and the looking-glass certificate gate -->
<!-- source: cmd/ze/hub/listener_migrate.go -- updateLGCertificate -->

The reload runs the same loader as daemon start, so it applies the password transform
with no branch of its own. Every load that hashes a leaf warns that the file still
holds the plaintext. Refer to
[Passwords in a config file](authentication.md#passwords-in-a-config-file).
<!-- source: internal/component/config/loader.go -- LoadConfig calls ApplyPasswordHashing, warnPlaintextOnDisk -->

## Moving a Management Listener

A reload moves the web server, the looking glass and the MCP server to a new
address or port with no restart. Change the `ip` or `port` of a `server` entry
under `environment { web }`, `environment { looking-glass }` or
`environment { mcp }`, then commit or reload. The REST and gRPC listeners of
`api-server` move the same way.
<!-- source: cmd/ze/hub/listener_migrate.go -- buildChanges, reloadListeners -->

```
environment {
    web {
        enabled true;
        server main {
            ip 0.0.0.0;
            port 9443;
        }
    }
}
```

| Step or case | What Ze does |
|--------------|--------------|
| New address | Ze binds the new address and serves on it before it closes the old one |
| Old address | It stops accepting new connections. A connection it already accepted stays open and is served to its end |
| An address that is already in use | The bind fails, the reload fails, and the service keeps every listener it had |
| One service fails after another moved | Ze moves the services that already moved back to their old addresses, and the whole reload fails |
| One service takes an address another gives up | Ze moves those two services last and logs `sequenced listener migration (brief gap expected)`. An address cannot be bound before it is released, so a bind that fails here fails the reload as above |
| A web, MCP, REST or gRPC server with no authentication | The reload is refused if it would move that server to a non-loopback address. The looking glass is public, so this check does not apply to it |
<!-- source: internal/component/web/server.go -- WebServer.Reconfigure -->
<!-- source: internal/component/lg/server.go -- LGServer.Reconfigure -->
<!-- source: cmd/ze/hub/service_mcp.go -- mcpServerHandle.Reconfigure -->
<!-- source: cmd/ze/hub/listener_migrate.go -- migrateListeners, rollbackAppliedListeners, detectConflicts, checkReloadExposure -->

## BGP Globals

A reload applies a changed global `router-id` and global local AS
(`session { asn { local } }`). `show bgp` and the `ze_info` metric report the
new values. A peer whose effective BGP Identifier changes is restarted: ze
sends NOTIFICATION Cease "Other Configuration Change" and the next OPEN carries
the new Identifier (RFC 4271 Section 4.2). A peer that sets its own
`session { router-id }` keeps its session. A peer added at run time inherits
the new values. A reload that fails restores the previous values.
<!-- source: internal/component/bgp/reactor/reload_globals.go -- applyGlobalsJournaled -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- reconcilePeersJournaled -->

## What Requires Restart

| Change | Why |
|--------|-----|
| Hub listen address/port | Listener cannot be changed at runtime |
| SSH server settings | Server cannot be reconfigured live |
| The web or MCP authentication MODE | Both choose it once, when they are built. A reload that asks for a different mode fails the whole commit before anything is applied |
| Anything under `exabgp bridge` (its processes, their `run` command, families, route refresh), and adding or removing the bridge | The bridge's scripts keep the config they started with. The reload is refused at verify, naming the leaf, with the restart steps |

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
reload still running after those 3 seconds is canceled: the daemon prints
`shutdown: canceling config reload after 3s; waiting for cleanup`, then waits for
the canceled reload to release the daemon's resources before it exits.

A reload that a commit started (an SSH or web `commit`, an API full reload, a
managed commit) is not cut short either: shutdown waits for it to finish, with no
time limit, because it writes the store that shutdown is about to close. A commit
that arrives once shutdown has begun is refused with `daemon is shutting down;
config reload refused`: re-run it against the restarted daemon.

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
`ze-plugin:system-daemon-reload` request over the daemon socket, and the daemon runs
the reload that SIGHUP runs.
<!-- source: internal/component/cli/reload.go -- newSocketReloadNotifier -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, SetFullReloadFunc -->
<!-- source: cmd/ze/hub/main_reload.go -- newRequestReloadFunc, handleSIGHUPReload, signalReload, doReloadContext -->

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
