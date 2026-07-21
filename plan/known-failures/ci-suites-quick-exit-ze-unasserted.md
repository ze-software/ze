### `.ci` suites -- 108 quick-exit `ze` commands across 50 files are silently unasserted

Found 2026-07-15 while building the vrrp suite. `expect=exit:code=` is
**file-level**: `Record.ExpectExitCode` is a single value (`record_parse.go:486`
-- a later line overwrites an earlier one) and `runOrchestrated` compares it
against `lastQuickZeErr`, the exit status of only the **last** quick-exit `ze`
command (`runner_exec.go:911-915`; the `case quickZe:` branch just stores the
error and continues). A file running several `ze config validate` commands
therefore asserts only the final one. Proven with a probe: a file whose `seq=1`
ran a **valid** config (exit 0) under `expect=exit:code=1` **passed**.

Stdout expectations are file-level in the same way (matched against accumulated
output), so `expect=stdout:contains=` can be satisfied by a different command
than intended -- e.g. two rejection cases both asserting `contains=vrid` are both
satisfied by the first one's message.

Fixed at the source for new tests: `cmd=...:exit=N` asserts a command's own exit
code the moment it finishes (`ci-format.md` "Process Commands"). It is opt-in, so
existing files are unaffected -- and consequently still unasserted.

Worst offenders (`quick-ze` commands / unasserted): `test/ui/format-operators.ci`
15/14, `test/ospf/ospf-config.ci` 7/6, `test/ospf/ospf-bfd-config.ci` 5/4,
`test/ospf/ospf-virtual-link-config.ci` 5/4, `test/ospfv3/ospf-ipsec-config.ci`
5/4, `test/ui/skills-list-get.ci` 5/4, `test/isis/isis-doctor.ci` 4/3.

**Not yet swept:** arming the 108 belongs to the suites' owners -- it may surface
real defects (a validation that never rejected what its test claimed). Re-measure
with the script in the vrrp session, or by counting `cmd=foreground` quick-`ze`
lines without `:exit=` in any file that has more than one.

The full plugin suite shows load-induced flakiness under max parallelism -- e.g.
`257`, `258`, `312` failed in one `--all` run but pass 3/3 in isolation. Running
two full `--all` suites back-to-back melts down (resource exhaustion: ~50
timeouts, ~200 "failures"). Triage individual tests in isolation; treat a
contiguous block of failures or a spike of timeouts in `--all` as a
harness/resource artifact, not real regressions.
