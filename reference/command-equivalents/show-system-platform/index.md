# `show system platform`

## Ze command

- Syntax: `show system platform`
- Registry path: `show system platform`
- Mode: Read-only
- Wire method: `ze-show:system-platform`
- Global pipes: yes

Show what kind of platform the daemon is running on. Reports whether this is gokrazy, systemd, container, plain-linux, or darwin, along with platform-specific capabilities.

## Mapping intents

### CPU, memory, platform, and host health

Category: System

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show hardware cpu` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
- `show system memory` (verified, vyos-cli)
  - Intent: CPU, memory, platform, and host health
