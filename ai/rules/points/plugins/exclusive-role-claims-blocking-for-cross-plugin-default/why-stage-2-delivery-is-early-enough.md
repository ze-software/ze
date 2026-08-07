---
kind: note
level:
stage:
---
Stage 2 is part of the sequential handshake, so it completes before Stage 5 ready
and therefore before `SignalPluginStartupComplete` -> StartPeers. The token is
opaque to the engine: A and B agree on the spelling, and B spells it itself rather
than importing A's package, so deleting A leaves B building and self-serving.
