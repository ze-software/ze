# Configuration Reload

Ze supports live configuration reload via SIGHUP or `ze signal reload`:

- Add/remove peers without restart
- Update peer settings with automatic reconciliation
- Graceful failure on invalid config (keeps running)
- Rapid successive reloads handled correctly
- Plugin config, config-provider roots, subsystem reload, and changed external
  plugin replacement roll back on failure

Dataplane objects still rely on each component's own reconciliation and journal
logic, so privileged reload tests remain part of release evidence.
