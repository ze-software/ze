# `show system dns`

Learned DNS resolvers and each consumer\'s dns\-source selection\.

## Ze command

- Registry path: `show system dns`
- Usage: `show system dns`
- Mode: Read-only
- Wire method: `ze-system-dns:show-system-dns`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The answer lists every learned resolver record \(its origin\, interface\, unit\, addresses\, when it was received and when it expires\)\, then every dns\-source consumer\'s selection\: the router\'s own under the consumer name system\, and each consumer that published one \(the LAN resolver\, each DHCP subnet\, each RA unit\, each subscriber pool\)\. Each selection carries its config path\, the instance it belongs to\, the sources and mode evaluated\, default true while the built\-in default list applies\, the classes used and the servers handed on\. A consumer that has not published yet is absent\: its plugin is not running or has not received the first snapshot\. A plugin removed while the daemon runs withdraws its selection\, so it is absent too\.

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
