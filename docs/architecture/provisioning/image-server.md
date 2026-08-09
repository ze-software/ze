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
- **`http.ServeFile` does the serving.** Range requests, content type, and
  conditional requests come from the standard library. No manual file I/O.
- **Path traversal is prevented by rejecting the filename, not by cleaning it.**
  A name containing a separator, a null byte, a dot segment, or any name that
  `filepath.Clean` would change is refused. Only flat names inside the
  configured directories are served.
- **The ZeFS database is built when the server is configured**, not per request,
  and only when both an SSH username and a password hash are configured. It is
  written to a temporary directory that is removed on stop and on reconfigure.
- The plugin imports no other provisioning plugin. It is independent of the web
  component, the DHCP server, and the TFTP server.
- Server timeouts are set explicitly (read, write, and header size) so a
  provisioning listener is not trivially held open.

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
