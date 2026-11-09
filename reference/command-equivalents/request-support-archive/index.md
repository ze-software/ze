# `request support archive`

Collect logs\, config\, state and diagnostics into one archive\.

## Ze command

- Registry path: `request support archive`
- Usage: `request support archive [module <module>] [exclude <exclude>] [since <since>] [reason <reason>]`
- Mode: daemon, offline
- Wire method: `ze-support:request-support-archive`
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

The archive holds logs\, config\, state and diagnostics\, and is what to send when you report an issue\. Secrets are redacted unless asked for\. Modules can be selected or excluded\, and a time window narrows the logs the archive holds\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `module` | string | no | any value of this type | Collect only these modules \(comma\-separated\)\. | Only the named modules are collected\, given as a comma\-separated list of module names\. Not given together with exclude\. |
| `exclude` | string | no | any value of this type | Collect every module except these \(comma\-separated\)\. | Every module except the named ones is collected\, given as a comma\-separated list of module names\. Not given together with module\. |
| `since` | string | no | any value of this type | Start the logs at this time or this long ago\. | The logs the archive holds start at this time\: a duration back from now \(2h\, 30m\) or a date or time \(2026\-05\-25\, 2026\-05\-25T14\:00\:00Z\)\. |
| `reason` | string | no | any value of this type | Why the archive was collected\, recorded in its manifest\. | Free text recorded in the archive\'s manifest\, cut at 256 characters\. |

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
