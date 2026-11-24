---
spec: ppp-l2tp-aaa-one-off-defects
topic: L2TP
found-by: review
---

## Before

Subscriber and CPE sessions hit several PPP, L2TP, PPPoE, and RADIUS defects:

- Subscriber authentication handler precedence depended on plugin registration order when both RADIUS and local authentication were compiled in.
- L2TP outgoing calls and PPPoE relays always dialed from the first configured `environment l2tp server` listener, failing when a loopback or other-family listener came first.
- A RADIUS `Access-Accept` returning `Framed-IP-Address = 255.255.255.255` (`0xFFFFFFFF`, peer-negotiated per RFC 2865 Section 5.8) was discarded instead of letting the subscriber negotiate its own IPv4 address via IPCP.
- `radius.EncodeVSA` included the outer Type and Length octets while `radius.DecodeVSA` read the attribute value, requiring callers to slice `[2:]`.
- Forged PPPoE PADT frames with a mismatched source MAC logged unbounded WARN lines.
- The PPPoE client silently dropped unsupported protocols while LCP was Opened instead of replying with LCP Protocol-Reject (RFC 1661 Section 5.7), and tore down the PPP link immediately when the last NCP closed rather than keeping LCP open for an idle period (RFC 1661 Section 3.7).
- Random IPv6CP interface identifiers did not clear the universal/local (`u`) bit required by RFC 5072 Section 4.1.
- L2TP metrics stayed unbound if the metrics registry initialized after the L2TP subsystem started.

## After

- Subscriber authentication uses an explicit priority registry: configured RADIUS (`AuthPriorityRemote`) always takes precedence over local users (`AuthPriorityLocal`), and removing RADIUS immediately falls back to local authentication.
- Outgoing L2TP calls and PPPoE relays select a bound listener that can reach the remote's IP family and scope; `ze config validate`, startup, and reload enforce the same check.
- RADIUS `Framed-IP-Address = 255.255.255.255` bypasses pool allocation and lets IPCP accept a peer-proposed unicast IPv4 address outside configured pools and not already held by another live session, releasing it on IPCP down, LCP reset, or teardown.
- `radius.EncodeVSA` and `radius.DecodeVSA` are direct inverses over the VSA attribute value with no `[2:]` slicing in callers.
- PPPoE PADT MAC-mismatch frames log at Debug with `reason=padt-mac-mismatch` and are rate-limited to five messages per second per interface server.
- The PPPoE client sends LCP Protocol-Reject for unsupported protocols while LCP is Opened (including IPv6CP/IPv6 when IPv6 is disabled) and keeps LCP open up to 30 seconds after the last NCP closes so an NCP can reopen before closing the link.
- Random IPv6CP interface identifiers always clear the RFC 5072 Section 4.1 `u` bit before validation.
- L2TP metrics bind immediately or lazily through `InjectPluginMetrics` whenever the registry registers later.

## Corrects

none

## Exposure

Deployments using RADIUS `Framed-IP-Address = 255.255.255.255` (peer-selected IPv4) enforce that peer-proposed IPv4 addresses are valid unicast addresses distinct from the local gateway, outside every configured IPv4 pool, and not already held by another live session, preventing a subscriber from claiming a pool address or hijacking another subscriber's active IPv4 address. PPPoE servers facing forged PADT frames from spoofed MAC addresses are now protected from WARN log flooding via per-interface rate-limited Debug logging.

## Upstream

none

## Asked by

none
