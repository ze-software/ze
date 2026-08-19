# Deferrals: cli-show-bgp-is-the-command

Rows deferred from `plan/spec-cli-show-bgp-is-the-command.md`. Each row names
where the work goes, so nothing is recorded without a destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-cli-show-bgp-is-the-command | A hint for a retired command, so an operator who types a command that used to exist is told what replaced it rather than only that it is unknown | `show bgp summary` is the first command this repository retires, and the need is general rather than specific to it. A hint needs a record of what a retired name became, which no registry holds today, and a rule for how long a retired name stays known. Building that for one command would either hard-code the one case or invent the facility inside a spec whose subject is a different thing | needs a destination spec | open |
