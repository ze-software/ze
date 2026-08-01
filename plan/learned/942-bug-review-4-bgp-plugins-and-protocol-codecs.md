# bug-review-4-bgp-plugins-and-protocol-codecs

## Summary

For BGP plugins, registration completeness is behavior. A codec can be correct but still user-broken if the family chain omits the registry, config, route encoder, CLI, or test link that exposes it.

## Key decisions

- Used a per-family NLRI chain matrix: family registration, splitter, in-process decode, NLRI encode, route encoder, config parser, JSON display, and tests.
- Reviewed BGP command plugin schemas and RPC handlers as owned plugin surfaces.
- Kept decode-only encode completeness candidates unpromoted unless a user-facing failure or product policy was proven.

## Results

- Created `plan/review-bug-review-bgp-plugins.md`.
- Accepted BPLUG-001 into `plan/spec-bugfix-bgp-nlri-strictness.md`.
- Accepted BPLUG-002 into `plan/spec-bugfix-bgp-srpolicy-encode.md`.
- Left BPLUG-P1 and BPLUG-P2 plausible because BGP-LS fallback and decode-only family policy need more evidence or a product decision.

## Gotchas

- Silent parser fall-through is worse than unsupported input. The command appears to work while dropping operator intent.
- A registered family with decode/config support but no canonical route encoder creates split-brain behavior across CLI, config, and compatibility paths.
- Missing in-process decoder registration is not automatically a user bug if a production CLI path has a subprocess or DirectBridge fallback.

## Verification

- Child report includes assigned package ledger, NLRI family-chain matrix, command/RPC wiring matrix, accepted findings, plausible findings, rejected candidates, and cleared classes.

## Files

None recorded.
