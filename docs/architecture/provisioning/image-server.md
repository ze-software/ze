# Image server

The image server publishes what a bare-metal install needs over HTTP: gokrazy
disk images, installer boot files, the iPXE boot script, and a pre-provisioned
ZeFS database.

<!-- source: internal/plugins/imageserver/register.go -- plugin registration and listener lifecycle -->
<!-- source: internal/plugins/imageserver/handler.go -- newMux, serveImage, serveBoot, serveBootIPXE, serveZefs -->
<!-- source: internal/plugins/imageserver/config.go -- config parsing and verification -->

## Decisions

- **Its own HTTP listener, not the web component.** Image transfers are large
  and long, and they must not compete with the web UI. The two have independent
  lifecycles, so provisioning can run with no web UI at all.
- **One listener for each `listen-interface` entry.** The leaf is a `leaf-list`,
  and the TFTP server and the DHCP server bind one listener for each entry of
  the same leaf. A host that provisions two install networks names two
  interfaces and gets HTTP on both. An entry that does not resolve is logged and
  skipped, so one wrong name does not stop the others; when nothing binds, the
  plugin logs `no interfaces bound` rather than `started`. With no entry the
  port is bound on every address of the host.
- **Each listener serves a boot script naming its own address.** The mux is
  built for the address it binds, so `/install/boot/boot.ipxe` sends the client
  back to the interface it reached. One mux shared by every listener would send
  a client on the second network to the first network's address.
- **`http.ServeFile` does the serving.** Range requests, content type, and
  conditional requests come from the standard library. No manual file I/O.
- **Path traversal is prevented by rejecting the filename, not by cleaning it.**
  A name containing a separator, a null byte, a dot segment, or any name that
  `filepath.Clean` would change is refused. Only flat names inside the
  configured directories are served.
- **The ZeFS database is built when the server is configured**, not per request,
  and only when both an SSH username and a password hash are configured. It is
  written to a temporary directory that is removed on stop and on reconfigure.
  It is an artifact, so `buildZefsDB` writes every key and data slot exact-fit
  (`zefs.Spare(0)`, `docs/architecture/zefs-format.md`): the installer copies
  it whole, and padding would only enlarge the download. A running device can
  take the same file with `ze init --from http://<server>/install/database.zefs`,
  which fetches it through the installer's helper (`internal/core/fetch`).
- The plugin imports no other provisioning plugin. It is independent of the web
  component, the DHCP server, and the TFTP server.
- Server timeouts are set explicitly (read, write, and header size) so a
  provisioning listener is not trivially held open.

## Config reload

A reload that changes nothing under `service image-server` leaves the server
alone: `sdk.ConfigTx` finds the parsed config equal to the running one and
installs nothing. A reload that changes it first builds the zefs database in a
new temporary directory, then closes every HTTP server and starts new ones for
the new config, and logs `imageserver: config applied`. `http.Server.Close`
closes the active connections as well, so a download in flight is cut and the
client has to fetch it again. The old zefs directory is removed once the new
servers serve.

A reload resolves and binds every `listen-interface` or none. When one cannot
be resolved or bound, or the zefs database cannot be built, the apply returns
an error naming the cause and the fix, so the commit fails and rolls back, and
the replaced config's servers are bound again, each on the address it served,
one by one, so when another process takes the port of one of them in that
window, the others still serve. The error then wraps
`sdk.ErrRunningConfigLost`, says how many of the interfaces that served are
lost and names each one with its address and cause, and ends with
`sdk.RunningConfigLostNext`: the rollback binds the running config again and
restarts the plugin if that fails too, the reload answer's `Rollback:` line
says which happened, and the fix is to free the port, then, for a listener a
restarted plugin could not bind, to stop the whole daemon and start it again,
which restarts every protocol and drops every session. The text names the
restart routes of `sdk.RestartSteps()`, one per line: no command restarts one
plugin, and `ze signal restart` leaves ze stopped. A commit of the running
config is no fix: it has no diff, so no apply reaches the plugin. The
transaction is broken: its rollback installs the running config again, and when
that fails too it answers an error, so the bridge acks `CodeBroken` and the
engine restarts the plugin; until an install succeeds, a later apply of the
running config installs it rather than skipping it as unchanged. The boot
configure is not strict: a configure error at boot fails the plugin and aborts
the whole daemon start, and an interface's IPv4 address may arrive later
through DHCP, so it logs `resolve interface failed` or `listen failed` for an
interface and serves on the others, or logs `no interfaces bound`. A VLAN,
bridge or dummy that ze's own `interface {}` config creates exists by then,
because the server starts after the interface plugin (`StartAfter:
["interface"]`). An address or an interface that appears later is not bound
until the next reload.
<!-- source: internal/plugins/imageserver/config_tx.go -- imageServer.install, imageServer.restore, lostImageTarget, prepareZefs, imageServer.stop; pkg/plugin/sdk/config_tx.go -- ConfigTx.Apply, ConfigTx.Rollback -->
<!-- source: internal/plugins/imageserver/register.go -- bindImage, listenTargets, startTargets -->

## Traps

- **The ZeFS host and port constants must match what the hub reads back.** They
  are the `ze init` defaults. Change one side alone and the credentials are
  written under a key nothing looks up.
- **`http.ServeFile` redirects a trailing slash to the directory**, which then
  404s. The mux pattern for the database endpoint carries no trailing slash for
  that reason.
- Registration is already proven by the plugin-availability test. A separate
  "is it registered" test duplicates that coverage.

## Related

- `pxe-staging.md` for the dynamic boot-script endpoint and the iPXE chain
- `tftp-server.md` for the first stage of the same boot
