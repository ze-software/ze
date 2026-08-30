---
kind: directive
level: MUST NOT
stage:
---
- **The peer-up barrier and the API-sync wait MUST NOT be merged, and a barrier plugin MUST NOT drag in `apiSync`'s 500 ms IPC grace.** `apiSync` counts plugins that SEND routes; a barrier plugin only registers. A route sender's signal satisfying a registrar's obligation is a fail-open (`ai/rules/evidence.md`).
