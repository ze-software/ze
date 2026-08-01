# 777 -- systemd service management command

## Context

`ze install local` already had a minimal systemd path coupled to binary installation. The service spec needed an operator-facing command that manages only the service lifecycle for standard Linux hosts, after `ze init` created `database.zefs`.

## Decisions

- Added `ze service install`, `ze service uninstall`, and `ze service status` as an offline root command under `cmd/ze/service/`, with registration through `cmdregistry.RegisterRoot` and dispatch from `cmd/ze/main.go`.
- Kept `--dry-run` free of root, Linux, systemctl, and filesystem prerequisites so the generated unit can be reviewed and functionally tested on non-systemd hosts.
- Required non-dry-run install/uninstall to run on Linux with `systemctl`, and required root before writing `/etc/systemd/system/ze.service` or modifying ownership.
- Made non-dry-run install refuse missing `database.zefs` instead of creating config state. The intended sequence stays `ze init`, then `ze service install`.
- Warned on active config containing `daemon { user }`, because systemd already applies `User=ze` before exec and a second in-process privilege drop can make startup fail.
- Generated the unit with systemd-managed privilege drop (`User=ze`, `Group=ze`) and capabilities (`CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_NET_BIND_SERVICE`) rather than relying on ze's internal privilege drop.
- Set `XDG_RUNTIME_DIR=/run/ze` and `RuntimeDirectory=ze` in the unit so sockets and runtime files use `/run/ze` instead of falling back to `/tmp` for a non-root daemon.

## Consequences

- The systemd service path is separate from binary installation. `ze service uninstall` removes only the unit and leaves the binary, config, user, and group alone.
- `ze service install --dry-run --config /path` is the portable functional-test path because it exercises CLI dispatch and unit generation without touching the host.
- The existing `ze install local` systemd helper remains in place. Operators can use either the all-in-one installer or the stricter service-specific command.

## Gotchas

- `systemctl status` returns a non-zero exit code for inactive or failed units. `ze service status` preserves that behavior because it is a wrapper around systemctl.
- Unit-file generation tests must not depend on the absolute build path. The functional test checks stable fragments and uses `--config /custom/path` for deterministic config lines.
- The correct gated functional-test directory for a new shell CLI is `test/ui/`, even though the original spec named `test/service/`.

## Files

- `internal/plugins/systemd/main.go` -- root dispatch, runtime abstraction, real OS operations
- `internal/plugins/systemd/cmd_install.go` -- install flags, prerequisites, user/group creation, chown, systemctl enable/start
- `internal/plugins/systemd/cmd_uninstall.go` -- stop, disable, remove, daemon-reload
- `cmd/ze/service/cmd_status.go` -- systemctl status wrapper
- `internal/plugins/systemd/unit.go` -- systemd unit generation
- `internal/plugins/systemd/main.go` -- platform gating via `runtime.GOOS`
- `cmd/ze/service/register.go` -- command registry metadata
- `cmd/ze/main.go` -- top-level dispatch wiring
- `internal/plugins/systemd/unit_test.go`, `internal/plugins/systemd/service_test.go` -- AC-linked unit tests
- `test/ui/service-unit-gen.ci` -- functional dry-run coverage
