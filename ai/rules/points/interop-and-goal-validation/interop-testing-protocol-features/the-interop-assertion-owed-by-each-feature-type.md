---
kind: table
level:
stage:
---
| Feature type | Interop assertion |
|-------------|-------------------|
| New address family / NLRI | Routes exchanged and installed by peer daemon |
| New capability | Capability negotiated, verified in peer's neighbor output |
| Session behavior (GR, route refresh) | Session survives the event, peer confirms expected behavior |
| Policy (community, filter, role) | Peer receives/rejects routes per the policy |
| Wire format change | Peer accepts the message, no NOTIFICATION |
| Authentication (MD5, EAP, PSK) | Session authenticates, handshake completes |
