# Declared summary without its explanation

A command declares two help texts: the one-line summary a list row shows, and
the long explanation its own help page prints. Neither is derived from the
other, so a declaration that carries only the summary renders with an empty
explanation. Nothing at the declaration site says a text is missing, and the
gate that counts them is the only reader that can tell.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-24 | spec-plugin-optional-dependency-refusal (phase 6b) | `request` and `show policy` command nodes, warned by `validateNode` (`config/yang/command.go`) | Every `ze cli -c` and daemon start of the stripped build logs `WARN YANG command node missing ze:help summary` for `request` and `show policy`. No `container request` in any `-cmd.yang` carries `ze:help`; `show policy` defers its text to `ze-policy-cmd.yang`, built with `ze_bgp` only | open: give `request` one summary in an always-on module, and `show policy` its text where the node is declared in every build |
| 2026-09-10 | spec-ipsec-dataplane-inspection | Host `ze pipe json compact` startup | The command emits `WARN YANG command node missing description path=request` on stderr. Its numeric JSON output and exit code are correct. | open: Not changed. The declaration's producer was not traced; inspect that declaration before changing it. |
| 2026-09-07 | daemon-backed-command-catalog | seven YANG RPCs, judged by `TestEveryCommandNodeHasASummary` (`internal/le/docvalid/helpshape_test.go`) | Red at HEAD with every YANG module named in the report clean in the working tree: `ze-bgp-cmd-peer-api:peer-add`, `:peer-save`, `ze-bgp-cmd-update-api:peer-update-hex`, `ze-cli-set-api:bgp-peer-save`, `:bgp-peer-with` and two more. Each carries `description` and no `ze:help` beside it, so the run reports `missing-long-help 7` over 184 RPCs. Walked into while running `go test ./internal/le/docvalid/` for Phase 4 of this spec | open: not fixed, and not this spec's surface |
