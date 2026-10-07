# Resolution CLI and Pipes

## Meta

| Field | Value |
|-------|-------|
| Name | Resolution CLI and Pipes |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/ze_core_pipe.go, internal/component/command/pipe.go, internal/component/command/answer_shape.go |
| Real-path tests | test/parse/resolve-dns-help.ci, test/parse/resolve-cymru-invalid.ci, test/parse/resolve-cymru-noargs.ci, test/ui/pipe-operators.ci, test/ui/pipe-local-command.ci, test/ui/monitor-ping-pipe-resolve-log.ci |
| Docs | docs/guide/cli.md, docs/features/pipe-operators.generated.md |
| Doc review | 2026-10-07: every source anchor resolves (runPipe, validateDeclaredShape refuses a resolve or origin pipe over an undeclared command via ShapeForCommand, RegisterAddressFields) |
| Defect review | 2026-10-07: audit found no open immediate spec against the resolve CLI or the pipe layer; journal rows naming a Component, not each re-verified here: blanket-mechanism-hid-missing-cases.md:9, declared-format-contradicts-payload.md:25, declared-format-contradicts-payload.md:28, documentation-shows-config-the-parser-refuses.md:18, gate-excludes-part-of-its-population.md:141, green-that-could-not-have-been-red.md:238, guard-message-teaches-the-violation.md:11, silent-fall-through.md:55, silent-fall-through.md:62 |
| Extra criteria | supported: a positive ze resolve answer per resolver against a mock = test/plugin/resolve-cymru-answer.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Offline `ze resolve` tool for DNS, Team Cymru ASN names, PeeringDB prefix counts, and IRR AS-SET expansion. The `\| resolve` (reverse DNS) and `\| origin` (ASN/network lookup via Team Cymru) pipe operators decorate the fields a command DECLARES to hold an IP address, and are refused by name over a command that declares none. `ze pipe` brings these pipe operators to offline commands: `ze show debug profile \| ze pipe match reactor`, `ze show debug profile \| ze pipe count`, `ze show debug profile \| ze pipe resolve`; standalone input carries no declaration, so there they walk every field whose value parses as an address. <!-- source: cmd/ze/ze_core_pipe.go -- runPipe --> <!-- source: internal/component/command/pipe.go -- validateDeclaredShape, ProcessStandalonePipesChecked --> <!-- source: internal/component/command/answer_shape.go -- RegisterAddressFields, AddressFieldsForCommand -->
