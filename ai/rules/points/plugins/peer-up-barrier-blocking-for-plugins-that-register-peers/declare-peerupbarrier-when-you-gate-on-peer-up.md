---
kind: note
level: MUST
stage:
---
A plugin that decides, ON the peer-up event, whether a peer is eligible to
receive traffic MUST declare `PeerUpBarrier: true`. Declaring it makes the
engine hold that peer's initial-sync End-of-RIB until the plugin has taken
delivery of the event, so **"End-of-RIB sent" implies "every barrier plugin has
registered this peer"**, the property a peer, or a test, needs to treat the
marker as the go-ahead to send.
