# `request reboot`

## Ze command

- Syntax: `request reboot`
- Registry path: `request reboot`
- Mode: Daemon
- Wire method: `ze-system:daemon-reboot`
- Global pipes: yes

Gracefully shutdown then reboot the system.

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
