# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| retainMUP | Not a weakening: its two `require` checks moved into the new helper `retainMUPCounts`, which `retainMUP` now calls, so every caller runs the same assertions; `TestMUPOtherRouteTypesSilentlyIgnoredAtIngress` gains a check that the other route types are discarded, never withdrawn. Another session's hunk, carried with the MUP T1ST fixture fix in the same file |
