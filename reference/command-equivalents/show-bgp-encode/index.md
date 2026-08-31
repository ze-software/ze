# `show bgp encode`

Turn a route announcement into wire-format hex.

## Ze command

- Registry path: `show bgp encode`
- Usage: `show bgp encode`
- Mode: Read-only
- Wire method: `ze-show:bgp-encode`
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

Takes a route in API syntax and returns the BGP UPDATE as a hex string. Use it to build a test payload, to feed ze-test, or to verify that an announcement encodes correctly.

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
