---
kind: directive
level: MUST
stage:
---
**Each feature type MUST prove the interop assertion its row names:**

| Feature type | Interop assertion |
|-------------|-------------------|
| New address family or NLRI | Routes exchanged and installed by the peer daemon |
| New capability | Capability negotiated, verified in the peer's neighbor output |
| Session behavior (GR, route refresh) | Session survives the event, and the peer confirms the expected behavior |
| Policy (community, filter, role) | Peer receives or rejects routes per the policy |
| Wire format change | Peer accepts the message, no NOTIFICATION |
| Authentication (MD5, EAP, PSK) | Session authenticates, handshake completes |
