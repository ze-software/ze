# A header field is derived from the message instead of the node that sends it

A field the standard defines as a property of the sender is computed from the
message being built. The two agree in the common case, so tests built from
single-role nodes pass, and only a node whose role differs from the message's
shows the wrong value.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | route-policy-0-umbrella (found, not caused) | IS-IS `lspTypeBlock` (`internal/plugins/isis/lsdb/origination.go`) | IS Type comes from the LSP's level, so an L1L2 IS reports IS Type 1 in its Level 1 LSPs; RFC 1142 says the field reports the IS type, which for an L1L2 node is Level 2 (value 3). 053d17934f fixed only the L2 LSP case | open: pass the node's level into the originator and set IS Type 3 in every LSP an L1L2 IS originates, with a test over an L1L2 node's L1 LSP |
