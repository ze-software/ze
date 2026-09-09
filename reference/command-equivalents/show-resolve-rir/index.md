# `show resolve rir`

Show which Regional Internet Registry holds an AS number.

## Ze command

- Registry path: `show resolve rir`
- Usage: `show resolve rir <asn>`
- Mode: Read-only
- Wire method: `ze-show:resolve-rir`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Reads the RIR delegation table that ships with the binary, or the newer copy an earlier 'update resolve rir' stored. Reaches no network. Reports the registry, its whois server, and the delegated range that holds the AS number. An AS number in no delegated range and a table that cannot be read are two different answers.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `asn` | union | yes | any value of this type |

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
