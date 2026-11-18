# A YANG constraint the schema declares is not enforced

The module states a range, length or pattern, and the config parser accepts a
value outside it, so the only guard is whatever a consumer happens to check
later.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | spec-web-cli-config-one-off-defects (AC-8) | leaf-list ranges in `config.ParseTreeWithYANG` | A leaf-list accepts 0 despite range 1..2, leaving refusal to a consumer. | fixed: `yangValueLeaf` retains native restrictions for all multi-valued nodes; `memberLeaf` delegates to `ValidateLeafValue`. `TestYANGLeafListNativeRange` and length, union and decimal controls pass across four forms and boundaries. `leaf-list-native-range.ci` refuses shipped CoPP port 0 before save and accepts 65535. |
