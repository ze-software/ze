---
kind: directive
level: MUST
stage:
---
**A genuine RFC-term difference is the ONLY allowed divergence from a sibling protocol's modelling, and it MUST be justified in the leaf or container `description`.** Two exist today: the metric name (OSPF `cost` against IS-IS `metric`) and the router identity (`router-id` for BGP, OSPF and RSVP-TE, `lsr-id` for LDP, `system-id` plus `net` for IS-IS). The canonical model for every shared concept is `docs/architecture/config/yang-config-design.md`.
