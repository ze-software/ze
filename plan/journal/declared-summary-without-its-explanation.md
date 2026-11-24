# Declared summary without its explanation

A command declares two help texts: the one-line summary a list row shows, and
the long explanation its own help page prints. Neither is derived from the
other, so a declaration that carries only the summary renders with an empty
explanation. Nothing at the declaration site says a text is missing, and the
gate that counts them is the only reader that can tell.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-24 | spec-plugin-optional-dependency-refusal (phase 6b) | `request` and `show policy` command nodes, warned by `validateNode` (`config/yang/command.go`) | Every `ze cli -c` and daemon start of the stripped build logs `WARN YANG command node missing ze:help summary` for `request` and `show policy`. No `container request` in any `-cmd.yang` carries `ze:help`; `show policy` defers its text to `ze-policy-cmd.yang`, built with `ze_bgp` only | fixed: `8b2a59ad0b` moves summaries to always-on YANG; `TestStrippedCoreCommandModulesCarryAllSummaries` and `config/yang` pass. |
| 2026-09-10 | spec-ipsec-dataplane-inspection | Host `ze pipe json compact` startup | The command emits `WARN YANG command node missing description path=request` on stderr. Its numeric JSON output and exit code are correct. | fixed: `8b2a59ad0b` eliminates missing-summary WARNs in all build flavors; `internal/component/config/yang` (314.5s) passes with zero `missing ze:help summary` warnings. |
| 2026-09-07 | daemon-backed-command-catalog | seven YANG RPCs, judged by `TestEveryCommandNodeHasASummary` (`internal/le/docvalid/helpshape_test.go`) | Red at HEAD: seven RPCs (`ze-bgp-cmd-peer-api:peer-add`, `:peer-save`, `ze-bgp-cmd-update-api:peer-update-hex`, `ze-cli-set-api:bgp-peer-save`, `:bgp-peer-with` and two more) had `description` without `ze:help` | fixed: consolidated RPCs carry `ze:help` + `description`; `TestEveryCommandNodeHasASummary` passed in `internal/le/doc/yangcontract`. |
