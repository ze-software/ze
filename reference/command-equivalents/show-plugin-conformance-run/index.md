# `show plugin conformance run`

Start a program in declare mode and check it behaves as a plugin\.

## Ze command

- Registry path: `show plugin conformance run`
- Usage: `show plugin conformance run <command> [name <name>]`
- Mode: read\-only\, offline
- Wire method: `ze-plugin:show-conformance-run`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: check\, stage\, verdict\, reason\, fix
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The program is started in declare mode\, the way \'show plugin declarations\' starts a configured plugin\, and what it writes is judged with the checks the daemon applies at Stage 1\. One row is written for each check\: its name\, the stage it belongs to\, a verdict of pass\, fail or not\-judged\, the reason\, and the change that fixes a failure\. A behavior this version does not judge keeps its row with the verdict not\-judged\, so an answer with no failure is not full conformance\. The command exits 1 when a check failed\. It runs in your own ze process only\: the daemon starts no program a remote caller names\, and answers with the command to run in your own shell\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `command` | string | yes | any value of this type | Run string of the program to check\, as one quoted token | The run string an operator would configure under plugin external \<name\> run\, as one quoted token\. A shell starts it\, and a relative path is read from the caller\'s directory\. |
| `name` | string | no | any value of this type | Plugin name the report is written for | The plugin name the checks name in their refusals\. It defaults to the file name of the program the run string starts\. |

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
