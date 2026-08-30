---
kind: directive
level: MUST
stage:
---
- **A plugin that decides, ON the peer-up event, whether a peer is eligible to receive traffic MUST declare `PeerUpBarrier: true` in its `registry.Registration`.** The engine then holds that peer's initial-sync End-of-RIB until the plugin has taken delivery of the event, so "End-of-RIB sent" implies "every barrier plugin has registered this peer", the property a peer or a test needs to treat the marker as the go-ahead to send. How the barrier is counted, acknowledged and bounded is `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier".
