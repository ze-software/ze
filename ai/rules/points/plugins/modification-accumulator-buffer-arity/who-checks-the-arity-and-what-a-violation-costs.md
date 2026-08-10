---
kind: note
level:
stage:
---
`Op` does NOT check this. It holds no attribute-width table and runs once per
forwarded UPDATE, so the check belongs to the handler that already knows its own
width: `filter_community.wholeValues`, called from
`filter_community.genericCommunityHandler`. A violation is refused per
operation, logged with the attribute code, the value width and the buffer
length, and counted as `ze_bgp_attr_mod_remove_buffer_refused_total`. The
attribute's other operations still apply. Contract:
`filterapi.ModAccumulator.Op`. Architecture:
`docs/architecture/core-design.md`, "Buffer arity, list-valued attributes".
