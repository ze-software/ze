# `clear cymru cache address`

Remove the cached Cymru origin of one IP address\.

## Ze command

- Registry path: `clear cymru cache address`
- Usage: `clear cymru cache address <ip-address>`
- Mode: Daemon
- Wire method: `ze-resolve:clear-cymru-cache-address`
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

Removes the origin of one address\. The name of the AS it pointed at stays cached\, because it belongs to the AS\. The answer reports found true when any owner held it\, removed\, and the owners rows\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `ip-address` | union | yes | any value of this type | IP address to clear\. | An IPv4 or IPv6 address\. Any spelling of the address finds the entry\, so 2001\:DB8\:\:1 and 2001\:db8\:0\:\:1 are the same key\. |

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
