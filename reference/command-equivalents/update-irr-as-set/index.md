# `update irr as-set`

Fetch the IRR dataset of an AS\-SET now\.

## Ze command

- Registry path: `update irr as-set`
- Usage: `update irr as-set <as-set>`
- Mode: Daemon
- Wire method: `ze-irr:update-irr-as-set`
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

Fetches and stores the prefixes of one AS\-SET\. A firewall commit naming an AS\-SET with nothing stored is refused until this has run\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `as-set` | string | yes | any value of this type | AS\-SET name\. | The RPSL set name\, for example AS\-EXAMPLE or RIPE\:\:AS\-EXAMPLE\. Letters\, digits\, hyphen\, underscore\, colon and period are accepted\. |

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
