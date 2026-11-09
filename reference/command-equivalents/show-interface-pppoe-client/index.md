# `show interface pppoe-client`

Show PPPoE client sessions\, the DNS servers they learned and their delegated prefixes\.

## Ze command

- Registry path: `show interface pppoe-client`
- Usage: `show interface pppoe-client`
- Mode: Read-only
- Wire method: `ze-iface:show-interface-pppoe-client`
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

One object per configured PPPoE client\: its state\, source interface\, session id\, local and peer address\, uptime\, and the DNS servers learned from IPCP \(RFC 1877\)\, primary first\. dns\-servers is an empty list when the access concentrator supplied none\; a configured system name\-server still takes precedence in resolv\.conf\. delegated\-prefixes lists each prefix the client\'s DHCPv6 lease holds \(RFC 9915 IA\_PD\) as prefix\, valid\-lifetime and preferred\-lifetime\, sorted by prefix\: the lifetimes are the whole seconds that remain at the moment of the answer\, 4294967295 meaning infinite\, and the list is empty when no prefix is delegated\.

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
