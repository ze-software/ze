---
kind: directive
level:
stage:
---
**Always spell `--pattern` in full: `-p` is a DIFFERENT flag in most suites.** `-p` is `--parallel` (an int) for `ze-test bgp <type>`, `ze-test exabgp`, `ze-test vpp` and every `.ci` suite on the shared runner (`internal/test/cli/cmd_bgp.go`, `cmd_exabgp.go`, `cmd_vpp.go`, `ci_runner.go`), and `--pattern` (a string) only for `ze-test editor` and `ze-test web` (`cmd_editor.go`, `cmd_web.go`); `--pattern` itself has no short form anywhere. So `ze-test bgp plugin -p rfc7606-relay-one-field` is not a filtered run, it is a parse failure -- exit 2, no output, no tests -- and it reads as "nothing to report" rather than as an error.
