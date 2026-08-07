---
kind: note
level: MUST
stage:
---
Genuine RFC-term differences are the ONLY allowed divergence and MUST be justified in the leaf/container `description`:
- Metric name: OSPF `cost` vs IS-IS `metric` (each is that protocol's RFC term).
- Router identity: `router-id` (BGP/OSPF/RSVP-TE) vs `lsr-id` (LDP) vs `system-id` + `net` (IS-IS).
