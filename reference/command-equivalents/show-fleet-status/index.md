# `show fleet status`

Whether this device follows a hub and matches its last configuration\.

## Ze command

- Registry path: `show fleet status`
- Usage: `show fleet status`
- Mode: read\-only\, offline
- Wire method: `ze-managed:show-fleet-status`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: managed\, frozen\, hub\-declared\, hub\-name\, hub\-address\, baseline\-recorded\, baseline\-hash\, active\-recorded\, active\-hash\, diverged\, daemon\-answered\, client\, connection\, holding\, held\-by\-hub\, revert\-offered
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Reports whether this device is managed and frozen\, the hub its configuration declares\, the hash of the last configuration the hub gave it \(the baseline\)\, the hash of the configuration it runs\, and whether the two differ\. It reads the store and changes nothing\, so ze show fleet status answers with no daemon too\, and the answer says whether a running daemon gave it\.

## Arguments

No command-specific arguments listed.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
