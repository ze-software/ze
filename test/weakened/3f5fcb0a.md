| Test | Reason |
|---|---|
| TestRelayPayloadKeepsSourceAttributeOrder | Replaced by TestRelayPayloadLeadsWithMPReach in the same file. The owner ruling of 2026-10-19 reverses the behavior it pinned: a relay now writes MP_REACH_NLRI first (RFC 7606 Section 5.1), so the old order is wrong by design. The replacement asserts the whole reconstructed body byte for byte, which is stricter than the old code-list comparison. |
| relay_payload_test | The two assertions counted out are the old test's code-list scan; TestRelayPayloadLeadsWithMPReach replaces them with one exact equality on the whole reconstructed body, which covers every byte the scan covered. |
