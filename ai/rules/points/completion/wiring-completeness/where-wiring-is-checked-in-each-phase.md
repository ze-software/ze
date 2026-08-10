---
kind: directive
level: MUST
stage:
---
1. **Design phase:** the spec's Wiring Test table names every entry point before implementation starts.
2. **Implementation phase:** `/ze-implement` step 4 creates the entry point skeleton and a failing wiring test before any feature code is written.
3. **Review phase:** `/ze-review` step 1 checks wiring before any other analysis.
4. **Completion phase:** the mechanical check below catches anything that slipped through.

Each phase MUST perform the check it owns.
