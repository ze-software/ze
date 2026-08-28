# `resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N]`

## Ze command

- Syntax: `resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N]`
- Registry path: `resolve traceroute`
- Mode: Read-only
- Wire method: `ze-resolve:traceroute`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Traceroute from the router with optional source binding. Usage: resolve traceroute <target> [source <ip>] [max-hops N] [timeout D] [probes N].

## Mapping intents

### Ping and traceroute diagnostics

Category: Diagnostics

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS
- `ping <target>` (verified, nokia-mdcli)
  - Intent: Ping and traceroute diagnostics

### VyOS
- `ping <target>` (verified, vyos-cli)
  - Intent: Ping and traceroute diagnostics
- `traceroute <target>` (verified, vyos-cli)
  - Intent: Ping and traceroute diagnostics
