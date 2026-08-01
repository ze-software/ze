# 811: imageserver plugin (HTTP provisioning)

## Context

spec-install-3: new imageserver plugin at `internal/plugins/imageserver/` serving gokrazy disk images, installer boot files (kernel, initrd, iPXE config), and pre-provisioned zefs databases over HTTP. Part of the ze install umbrella for PXE-based bare-metal provisioning.

## Decisions

- **Own HTTP listener, not web component.** The imageserver plugin runs its own `http.Server` on a configurable port, separate from the web UI. Image serving (large files, long transfers) must not affect web UI performance. Independent lifecycle: can be enabled without the web component.
- **`http.ServeFile` for all serving.** Range requests, Content-Type detection, and conditional requests handled automatically by the stdlib. No manual file I/O.
- **Path traversal prevention via filename validation.** The handler rejects names containing `/`, `\\`, null bytes, `.`, `..`, or names where `filepath.Clean(name) != name`. Only flat filenames within configured directories are served.
- **Zefs database built at configure time.** When `ssh-username` and `ssh-password-hash` are both configured, `buildZefsDB()` creates a temporary zefs database with SSH credentials using the same key pattern as `ze init` (`KeySSHUsername.Key("127.0.0.1", "2222")`, `KeySSHPassword.Key("127.0.0.1", "2222")`, `KeySSHDefault.Pattern = "127.0.0.1/2222"`). The database is served at `/install/database.zefs`.
- **Temp directory cleanup on reconfigure/stop.** The zefs temp directory is cleaned up when the server stops or reconfigures, preventing temp file leaks.

## Consequences

- The imageserver plugin is fully independent: no imports from web, dhcpserver, or tftpserver.
- The zefs database endpoint provides the SSH credential flow for bootstrap mode (spec-install-5).
- HTTP timeouts configured on the server (`ReadTimeout: 30s`, `WriteTimeout: 5min`, `MaxHeaderBytes: 64KB`) provide basic DoS protection for the provisioning use case.

## Gotchas

- `TestImageServerRegistered` is not a separate test file. Registration is already verified by `TestAvailablePlugins` in `cmd/ze/main_test.go`. Adding a redundant test would duplicate coverage.
- The zefs host/port (`127.0.0.1`/`2222`) must match what `loadZefsUsers()` reads in `cmd/ze/hub/main_servers.go`. These are constants in the imageserver package matching `ze init` defaults.
- `http.ServeFile` redirects `GET /install/database.zefs/` (trailing slash) to the directory, which would 404. The mux pattern `/install/database.zefs` (no trailing slash) avoids this.

## Files

None recorded.
