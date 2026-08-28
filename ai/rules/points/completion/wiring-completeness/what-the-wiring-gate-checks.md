---
kind: directive
level: MUST
stage:
---
**The wiring gate MUST verify that:**
- new exported Go symbols under `internal/` or `cmd/` have a non-test
  production reference in `internal/` or `cmd/`;
- command declaration changes run `./le docvalid command-contract`;
- source-anchored documentation changes run doc drift and stale-anchor
  checks;
- plugin registration and generated inventory source changes run
  registry-backed inventory checks.
