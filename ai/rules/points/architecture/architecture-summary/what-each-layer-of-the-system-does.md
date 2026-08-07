---
kind: note
level:
stage:
---
BGP Subsystem handles protocol: FSM manages peers, Wire Layer parses messages into WireUpdate, Reactor processes events, EventDispatcher bridges to Plugin Infrastructure. Plugin Infrastructure manages plugin lifecycle and message routing. Plugins implement RIB, dedup, policy.
