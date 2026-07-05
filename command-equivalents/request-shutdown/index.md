# `request shutdown`

## Ze command

- Syntax: `request shutdown`
- Registry path: `request shutdown`
- Mode: Daemon
- Wire method: `ze-system:daemon-shutdown`
- Global pipes: yes

Gracefully shutdown: drain connections, close peers, exit.

## Mapping intents

### Reload, reboot, halt, or shut down

Category: Lifecycle

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `poweroff` (verified, vyos-cli)
  - Intent: Reload, reboot, halt, or shut down
- `reboot` (verified, vyos-cli)
  - Intent: Reload, reboot, halt, or shut down
