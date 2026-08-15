# `show uptime`

## Ze command

- Syntax: `show uptime`
- Registry path: `show uptime`
- Mode: Read-only
- Wire method: `ze-show:uptime`
- Global pipes: yes

Show how long the daemon has been running. Returns the start time and elapsed uptime. Handy after a maintenance window to confirm the process restarted.

## Mapping intents

### Software version and uptime

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show system uptime` (verified, vyos-cli)
  - Intent: Software version and uptime
