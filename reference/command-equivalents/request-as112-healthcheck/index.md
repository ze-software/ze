# `request as112 healthcheck [target <ip>]`

## Ze command

- Syntax: `request as112 healthcheck [target <ip>]`
- Registry path: `request as112 healthcheck`
- Mode: Daemon
- Wire method: `ze-as112:health`
- Global pipes: yes

One-shot authoritative query against an anycast service address (or the given target), exit 0 iff the expected AS112 answer comes back. Finding M4: the tool a healthcheck probe calls, since dig is not on the gokrazy appliance and 'ze resolve dns' cannot target a specific server. Usage: request as112 healthcheck [target <ip>].

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
