---
kind: directive
level: MUST
stage:
---
**A spec that implements or changes protocol behavior (BGP, IPsec, L2TP, PPPoE, or any wire protocol) MUST carry an interop test proving Ze works correctly with at least one other implementation, and every feature MUST carry goal validation proving it achieves its intended purpose rather than merely running without error.** The interop test MAY be omitted only for a pure internal refactor with no wire-visible change, a config-only feature with no protocol impact, or tooling with no protocol peer. When no scenario matches the feature, you MUST create one before you claim done. The suites, their scenario directories, their native actions, and the assertion each feature type owes are in `docs/architecture/testing/interop.md`.
