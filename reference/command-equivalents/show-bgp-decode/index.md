# `show bgp decode`

Decode a hex-encoded BGP message into readable JSON.

## Ze command

- Registry path: `show bgp decode`
- Usage: `show bgp decode`
- Mode: Read-only
- Wire method: `ze-show:bgp-decode`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Paste a hex BGP UPDATE and get back parsed attributes, NLRI, and withdrawn prefixes. Use it to read pcap captures or to debug wire issues. The web UI carries the same tool under tools.

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
