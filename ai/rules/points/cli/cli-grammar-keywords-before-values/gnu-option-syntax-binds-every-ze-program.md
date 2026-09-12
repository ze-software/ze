---
kind: directive
level: MUST
stage:
---
**Every program this repository ships MUST use GNU option syntax, and a VALUE MUST NOT begin with a dash (owner directive, 2026-09-12).** Two dashes introduce one LONG option, so `--help` is help. One dash introduces a CLUSTER of SHORT options, so `-help` is `-h -e -l -p`: a help request followed by three options that do not exist. `-help` MUST NOT be read as a second spelling of `--help`, because the two forms belong to different registers and blessing the cluster would make the short form unparseable. This binds `ze`, `ze-chaos`, `ze-perf`, `ze-analyse`, `ze-test`, `ze-gok`, `ze-installer`, `ze-serial-shell` and `le` alike, and a program that reaches argv at all MUST answer to it.

**A token that begins with a dash is an OPTION, so it MUST NOT be consumed as the value a keyword introduced, and a dash-leading token where a value is expected MUST be refused by name.** The directive above declares the whole option register, `--version`, `-V`, `--help` and `-h`, and nothing else in the grammar carries a dash, so a dash-leading token in a value slot is always either an undeclared option or a typo. Neither is data. A negative number is not a counter-example: no command takes one, and the tests that pass `-1`, `-1s` and `-3` are proving those values are refused.

**A dash-leading token in the TRAILING position asks a question, and it MUST NOT start work.** This is what a malformed help request costs when the rule is absent: `./le stress-repro run suite -help` read the cluster as the value of `suite` and started a load generator measured at about 943% CPU for twenty minutes on a shared development machine (2026-09-11). Refusing the token is cheap and running it is not, so the doubt resolves toward the refusal.
