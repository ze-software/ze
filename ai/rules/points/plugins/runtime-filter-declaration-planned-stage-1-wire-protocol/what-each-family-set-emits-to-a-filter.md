---
kind: directive
level: MUST
stage:
---
- **A filter plugin that needs per-NLRI decisions on a non-CIDR family MUST declare `raw=true` and MUST parse `FilterUpdateInput.Raw` itself.** For EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC and every future non-CIDR family the text protocol emits `nlri <family> <op>` as a marker with no prefixes. A CIDR family inlines its prefixes, so `raw=false` is sufficient there. The contract is `docs/architecture/api/process-protocol.md`, "Non-CIDR Families in the Filter Text Protocol".
