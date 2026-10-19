# `clear irr cache as-set`

Remove the cached IRR answers of one AS\-SET\.

## Ze command

- Registry path: `clear irr cache as-set`
- Usage: `clear irr cache as-set <name>`
- Mode: Daemon
- Wire method: `ze-resolve:clear-irr-cache-as-set`
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

Removes the members and the prefix list of one AS\-SET and keeps the other entries\, at every owner\. The answer reports found true when any owner held one\, removed\, and the owners rows\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `name` | string | yes | any value of this type | AS\-SET name to clear\. | The AS\-SET name\, compared as an exact string\: give it as it was looked up\, for example AS\-EXAMPLE\. A name with a control character is refused\. |

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
