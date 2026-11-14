---
spec: rfc-verdict-fix-services
topic: RFC Compliance
found-by: rfc-check
---

## Before

A tunnel with remote `0.0.0.0` or `::` passed `ze config validate` and a
commit. The netlink backend then refused it when applying, with no fix named,
and the VPP backend programmed it.

<!-- source: internal/component/iface/config.go -- parseTunnelEntry -->
<!-- source: internal/plugins/iface/netlink/tunnel_linux.go -- parseTunnelRemote -->
<!-- source: internal/plugins/iface/vpp/tunnel.go -- tunnelEndpoints -->

## After

A tunnel whose remote is `0.0.0.0` or `::` is now refused by
`ze config validate` and by a commit, for every tunnel kind and with either
backend, and the message names the command that sets a real remote. RFC 4213
Section 3.6 requires the decapsulator to verify the tunnel source, and an
unspecified remote verifies nothing.

<!-- source: internal/component/iface/tunnel.go -- CheckRemote -->

## Corrects

none

## Exposure

Only tunnels on the VPP backend were programmed with an unspecified remote.
Operators on the netlink backend met the refusal only after the commit. A
tunnel with a specific remote address was never exposed.

## Upstream

none

## Asked by

none
