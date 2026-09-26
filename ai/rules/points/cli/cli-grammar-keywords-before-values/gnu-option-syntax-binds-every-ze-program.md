---
kind: directive
level: MUST
stage:
rationale: ai/rationale/cli.md
---
**Every program this repository ships MUST use GNU option syntax, and a value MUST NOT begin with a dash.** Two dashes introduce one long option, so `--help` is help. One dash introduces a cluster of short options, so `-help` is `-h -e -l -p`: a help request followed by three options that do not exist. `-help` MUST NOT be read as a second spelling of `--help`, because the two forms belong to different registers and blessing the cluster would make the short form unparseable. This binds `ze`, `le test`, `ze-installer`, `ze-serial-shell` and `le` alike, with every program `le` forwards to, and a program that reaches argv at all MUST answer to it.

**A token that begins with a dash is an option, so it MUST NOT be consumed as the value a keyword introduced, and a dash-leading token where a value is expected MUST be refused by name.** The directive above declares the whole option register, `--version`, `-V`, `--help` and `-h`, and nothing else in the grammar carries a dash, so a dash-leading token in a value slot is always either an undeclared option or a typo. Neither is data. A negative number is not a counter-example: no command takes one, and the tests that pass `-1`, `-1s` and `-3` are proving those values are refused.

**A dash-leading token in the trailing position asks a question, and it MUST NOT start work.** Refusing the token is cheap and running it is not, so the doubt resolves toward the refusal.
