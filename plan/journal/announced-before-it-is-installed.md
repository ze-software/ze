# A state is announced before the system holds it

A component announces a state (an address, a route, a role) as soon as it has
asked for it, while the layer that installs it runs later on its own worker.
Peers act on the announcement before the state exists.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | spec-vrrp-owner-arp-rfc-defects (found, not caused) | `doInstallVIPs` then `AnnounceFailover` (`internal/plugins/vrrp/instance.go`); `installVIPs` is the asynchronous `iface.RegisterOwnedAddresses` | An owner at boot sends its gratuitous ARP or unsolicited NA before the reconcile installs the address, against RFC 9568 8.1.2 and 8.2.2 (RFC9568-8.1.2-2, -8.2.2-4) | open: gate the first announcement on the reconcile having installed the addresses, or install synchronously for VRRP |
