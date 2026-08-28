# Self-update: the device pulls its own binary

`SelfUpdater` downloads a new `ze` binary, verifies its SHA-256, replaces the
running binary atomically, and optionally restarts. It targets standalone Linux
deployments. A gokrazy appliance has its own update mechanism.

<!-- source: internal/component/config/system/selfupdate.go -- SelfUpdater, download, verify, stage, restart -->
<!-- source: cmd/ze/update_serve.go -- the standalone update server that publishes the manifest -->
<!-- source: internal/plugins/update-cmd/cmd/firmware.go -- update system firmware CLI handlers -->
<!-- source: internal/plugins/update-cmd/cmd/show.go -- update status CLI handler -->

## Decisions

- **`SelfUpdater` is its own type, not a mode flag on the version checker.** The
  state machine differs: download, verify, stage, and restart, against
  fetch-and-compare. The hub picks between them from config. Auto-apply or a
  restart policy selects the self-updater; otherwise the checker runs unchanged.
- **Spread is deterministic per device and per version.** An FNV-1a hash of the
  device identity and the version string yields a delay inside the configured
  spread. The delay is stable across restarts for one device and version pair,
  so a restart loop cannot collapse a fleet onto the same instant. Device
  identity comes from `/etc/machine-id`, then the hostname, then crypto-random.
- **The maintenance window gates the binary replacement only.** Download and
  verification run at any time, so the binary is ready the moment the window
  opens.
- **Replacement is a hard link then a rename.** The current binary is hard-linked
  to a `.prev` backup, then the temporary file is renamed over the target. The
  target path is never absent during the operation. A cross-filesystem rename
  (`EXDEV`) is reported as a config error, because it means the staging
  directory is on the wrong filesystem.
- History is a circular buffer of 20 events, written atomically to the binary's
  directory. A missing or corrupt file starts empty rather than failing.
- Manual apply and manual download bypass server-side pause, spread, and the
  maintenance window. Auto-apply requires a SHA-256 in the manifest; a manual
  command warns and proceeds without one.

## Trap

The manifest's Go field is named `Ver` while its JSON tag stays `"version"`. The
native write hook rejects the `"version":` pattern in config-package files,
because a version number in a config struct is nearly always a mistake. This
manifest is a wire-protocol response, not config, so the tag is correct and the
field name carries the workaround.
<!-- source: internal/le/hookruntime/writeedit.go -- writeFilePatterns -->

## Related

- `ota-push.md` for the bastion pushing an image instead
