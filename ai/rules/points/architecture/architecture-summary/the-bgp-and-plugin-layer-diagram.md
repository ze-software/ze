---
kind: fence
level:
stage:
---
```
BGP Subsystem (internal/component/bgp/):
  Peers (FSM) → Wire Layer → Reactor (event loop, BGP cache) → EventDispatcher
   ║ formatted events (down) / commands (up)
Plugin Infrastructure (internal/plugin/):
  Registry · Process Manager · Hub · SDK · DirectBridge
   ║ JSON events + base64 wire bytes (down) / text commands (up)
Plugins: RIB, RR, GR, etc. (Go/Python/Rust)
```
