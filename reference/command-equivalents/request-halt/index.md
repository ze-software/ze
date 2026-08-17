# `request halt`

## Ze command

- Syntax: `request halt`
- Registry path: `request halt`
- Mode: Daemon
- Wire method: `ze-system:daemon-quit`
- Global pipes: yes

Dump goroutine stacks to stderr and terminate immediately.

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
