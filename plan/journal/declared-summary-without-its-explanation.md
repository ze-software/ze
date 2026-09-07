# Declared summary without its explanation

A command declares two help texts: the one-line summary a list row shows, and
the long explanation its own help page prints. Neither is derived from the
other, so a declaration that carries only the summary renders with an empty
explanation. Nothing at the declaration site says a text is missing, and the
gate that counts them is the only reader that can tell.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | daemon-backed-command-catalog | seven YANG RPCs, judged by `TestEveryCommandNodeHasASummary` (`internal/le/docvalid/helpshape_test.go`) | Red at HEAD with every YANG module named in the report clean in the working tree: `ze-bgp-cmd-peer-api:peer-add`, `:peer-save`, `ze-bgp-cmd-update-api:peer-update-hex`, `ze-cli-set-api:bgp-peer-save`, `:bgp-peer-with` and two more. Each carries `description` and no `ze:help` beside it, so the run reports `missing-long-help 7` over 184 RPCs. Walked into while running `go test ./internal/le/docvalid/` for Phase 4 of this spec | not fixed, and not this spec's surface |
