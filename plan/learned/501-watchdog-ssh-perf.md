# 501 -- Watchdog SSH Performance

## Context

The conf-watchdog ExaBGP compatibility test was failing systematically on slower machines.
The test exercises Ze's watchdog feature: a process bridge translates ExaBGP watchdog
commands into Ze SSH exec commands on a 200ms cycle. The bridge was opening a new SSH
connection (TCP + key exchange + auth) for every single command, taking 100-200ms per
round-trip and racing the watchdog timer.

## Decisions

- Chose persistent SSH connection (one connect, many exec_command calls) over making
  the watchdog script pace off acknowledgments, because it requires no API contract
  changes and paramiko natively supports multiple exec channels on one transport.
- Removed the standalone `send_ssh_command` helper entirely rather than keeping both
  patterns, since the bridge is the only caller.

## Consequences

- Each watchdog command now costs ~5-10ms (new channel on existing transport) instead
  of ~100-200ms (full handshake + auth). Should eliminate timing failures on slow machines.
- If future process bridge commands need SSH, they reuse the same persistent connection.

## Gotchas

- The SSH connection is opened before the watchdog subprocess starts, so a connection
  failure is caught early rather than mid-stream.
- paramiko exec_command opens a new channel per call on the same transport -- this is
  the intended usage, not a workaround.

## Files

- `test/exabgp-compat/bin/exabgp` -- persistent SSH in `run_process_bridge`, removed `send_ssh_command`
