---
kind: note
level:
stage:
---
Incident: session l2tp-8a-auth-pool (2026-04-21) proposed a new direct-call mechanism between core and plugins, not discovering that DirectBridge already provides typed function calls. Root cause: no document in the research path mentioned DirectBridge for request/response. Fixed by splitting the cross-plugin comm row into broadcast vs request/response and adding DirectBridge to the anti-pattern table.
