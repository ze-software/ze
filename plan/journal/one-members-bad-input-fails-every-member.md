# One member's bad input fails every member

A shared apply, reconcile or publish path takes every registered member's work
in one batch. One member's unusable input fails the batch, so members that did
nothing wrong lose their state too. The operator sees a failure attributed to
the whole subsystem and no line naming the member that caused it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-09 | policyroute-interface-list-matches-no-packet | `ApplyAll` (`internal/component/firewall/registry.go`) over a term `lowerIfaceMatch` (`internal/plugins/firewall/nft/lower_linux.go`) cannot lower | A bare `interface "*";` under a policy route reaches `parseIfaceSpec` (`internal/plugins/policyroute/config.go`), which strips the trailing `*` and leaves `Name` empty. `lowerIfaceMatch` returns `errInterfaceNameMustNotBeEmpty` for an empty name. `ApplyAll` gathers every owner's tables into ONE slice and hands them to ONE `Backend.Apply`, so the refusal fails the reconcile for copp, ddos-local, flowspec-firewall and the operator's own `firewall {}` block, not for policy routes alone. Verified at all three producers on 2026-09-09 | not fixed. Two ends are available and the choice is not obvious: refuse the empty name at config verify, where the operator is told which leaf is wrong and no other owner is touched, or make `Apply` isolate a failing owner so one bad table cannot take the batch down. The first is narrow and this defect's own fix; the second is the class's fix and reaches every future member. A row rather than a fix because the spec that found it was closing and neither end is a one-line change |
