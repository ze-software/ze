# `request reload`

## Ze command

- Syntax: `request reload`
- Registry path: `request reload`
- Mode: Daemon
- Wire method: `ze-system:daemon-reload`
- Global pipes: yes

Reload the configuration without restarting.

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
