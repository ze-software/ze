---
kind: note
level:
stage:
---
The `-cmd` (grammar tree) and `-api` (handler) modules for operational verbs are currently named several ways for the same verb (`ze-cli-monitor-cmd` vs `ze-monitor-cmd` vs `ze-command-monitor-cmd`; `ze-bgp-cmd-log-api` for a non-BGP command). Converging them on one scheme is a rename that touches `//go:embed`, `register.go`, and YANG dispatch keys, so it is tracked separately and NOT done piecemeal.
