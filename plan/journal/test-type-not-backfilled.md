| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-06 | spec-enum-switch-exhaustiveness | Native RFC filename/tag convention | Nine RFC test files broke the native filename and tag convention (five untagged, two misnamed, two probes) | fixed: five carry `// RFC naming: untagged --` markers (`d3e5b4a770`, `39798d536a`, `875eaecb9b`), two renamed with their records (`a184a563d1`, `d3e5b4a770`), both probes are no longer test files; `./le rfc check` names none of them (same evidence section) |
| 2026-06-14 | - | tests | new test type applied only to new code, existing code uncovered | open: ran selector across whole applicable set and filed gaps |
| 2026-06-14 | - | tests | fuzz target not back-filled to equivalent existing code | open: enumerated applicable sites and added backlog |
