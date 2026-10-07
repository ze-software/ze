# Outbound source-address

## Meta

| Field | Value |
|-------|-------|
| Name | Outbound source-address |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/network/network.go, internal/component/bgp/plugins/bmp/sender.go, internal/component/bgp/plugins/rpki/rtr_session.go, internal/plugins/flowexport/sender.go, internal/component/resolve/irr/client.go, internal/component/managed/client.go, internal/plugins/ldp/register.go |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor file exists; test/parse/source-address-*.ci only prove the config parses, so they are not listed as real-path evidence of the bind |
| Defect review | 2026-10-07: no open spec or journal row found naming source-address |
| Extra criteria | supported: per service a functional test where the far end asserts the bound source address = none yet |

## Description

Optional `source-address` local-IP binding for outbound service connections: BMP collector, RPKI/RTR cache, flow-export (UDP), IRR whois, and the managed hub TLS client; LDP binds its `transport-address` as the session source (RFC 5036). Unset = OS-selected source (unchanged). Shared `network.RealDialer` applies the bind for TCP/TLS; flow-export binds the UDP socket. See [Outbound Source Address](guide/configuration.md#outbound-source-address). <!-- source: internal/core/network/network.go -- RealDialer.LocalAddr --> <!-- source: internal/component/bgp/plugins/bmp/sender.go -- BMP source-address --> <!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- RTR source-address --> <!-- source: internal/plugins/flowexport/sender.go -- UDP source bind --> <!-- source: internal/component/resolve/irr/client.go -- IRR source-address --> <!-- source: internal/component/managed/client.go -- managed hub TLS source-address --> <!-- source: internal/plugins/ldp/register.go -- LDP transport-address binding -->
