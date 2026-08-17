# `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]`

## Ze command

- Syntax: `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]`
- Registry path: `resolve ping`
- Mode: Read-only
- Wire method: `ze-resolve:ping`
- Global pipes: yes

Ping from the router with optional source binding. Usage: resolve ping <target> [source <ip>] [count <n>] [size <bytes>].

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
