# A YANG constraint the schema declares is not enforced

The module states a range, length or pattern, and the config parser accepts a
value outside it, so the only guard is whatever a consumer happens to check
later.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | leaf-list `range` in `config.ParseTreeWithYANG` | `policy { match M { level [ 0 ]; } }` parses although the `level` leaf-list declares range 1..2; only the match plugin's own verify (`parseMatchLevel`) refuses it. A leaf-list with no consumer-side check would accept any value | open: apply the leaf-list's type constraints per value at parse, as for a leaf, with a test |
