# `show system date`

## Ze command

- Syntax: `show system date`
- Registry path: `show system date`
- Mode: Read-only
- Wire method: `ze-show:system-date`
- Global pipes: yes

Show the daemon's current wall-clock time and timezone. Useful for correlating log timestamps when the box is in a different timezone than you are.

## Mapping intents

### System time and NTP state

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show date` (verified, vyos-cli)
  - Intent: System time and NTP state
- `show ntp` (verified, vyos-cli)
  - Intent: System time and NTP state
