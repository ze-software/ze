# `show bgp update-delay`

Show the startup convergence hold and what it waits for.

## Ze command

- Registry path: `show bgp update-delay`
- Usage: `show bgp update-delay`
- Mode: Read-only
- Wire method: `ze-bgp:update-delay`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The command takes no argument, because the hold belongs to the speaker and not to a peer. configured says an operator set bgp update-delay max-delay. holding says this speaker is withholding its first advertisement right now, and released says the hold has ended. reason names the condition that ended it: converged, establish-wait or max-delay. expected-peers, peers-held and peers-converged say what the hold is still waiting for, so you can name the neighbor that has not finished. Read this command when a speaker has come up and advertised nothing: it separates a hold that is working from a daemon that is wedged.

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
