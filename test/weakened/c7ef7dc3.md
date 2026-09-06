# Test weakenings this commit accepts

Four `.ci` files lose directive lines. In three of them the assertion MOVED to a
new sibling file named in the row, and in the fourth the removed lines were
byte-identical copies of one file-level assertion. No assertion power leaves the
suite.

The cause is one fact about the runner, now documented in
`docs/architecture/testing/ci-format.md`: `expect=stdout:`, `expect=stderr:`,
`reject=stdout:` and `reject=stderr:contains=` all read `Record.ClientOutput`,
the accumulated stdout AND stderr of every command in the file. They are
FILE-level. A file that asserts a needle PRESENT for one command and ABSENT for
another asserts two contradictory things about one string, so the negative half
has to live in its own file.

| Test | Reason |
|------|--------|
| no-install-appliance | `expect=stdout:not-contains=appliance` asserted NOTHING: `not-contains` is not a key `expect=stdout` reads, so the line was dropped in silence. Corrected to `reject=stdout:contains=appliance`, it could not stay: seq=2 was `ze appliance --help`, whose output carries the word 16 times over the one buffer both commands write to. seq=2 and its own `expect=stdout:contains=init`, `contains=build` and stderr negative moved WHOLE to `test/appliance/appliance-help-not-deprecated.ci`, which is committed here. Both files say at the top why they are two. |
| vpp-doctor-hugepages | Same shape, and this one was self-contradictory on its face: line 37 asserted `contains=doctor-vpp-hugepages` for the vpp-enabled config and line 41 `not-contains=` the identical string for the disabled one, over one buffer. Only the vacuous spelling of line 41 kept the file green. seq=2, its `vpp-off.conf` tmpfs block and the corrected negative moved WHOLE to `test/plugin/vpp-doctor-hugepages-quiet.ci`, committed here; both files pass. The file-level `expect=exit:code=0` went with it, because it only ever checked the LAST quick-exit `ze` command and seq=1 already carries its own per-command `exit=1`. |
| dash-stdio | `expect=stdout:not_contains=router-id 1.2.3.4` (an underscore, a key nothing reads) asserted nothing. Corrected, it contradicts seq=1, seq=4 and seq=5 of the same file, which each assert that same string PRESENT. The scoped-output case moved to `test/ui/dash-stdio-path-scope.ci`, committed here, carrying the stdin block and the positive assertions with it; the original keeps its own copy of seq=2 for the positive half. |
| completion-words-no-env-pollution | Nothing moved and nothing was dropped: three IDENTICAL `expect=stdout:not-contains=` lines, one under each `cmd=`, became one `reject=stdout:pattern=(?m)^env\t`, and two of the three `expect=exit:code=0` lines went with them. Both directives are file-level (`Record.ExpectExitCode` is a single value a later line overwrites), so the copies asserted exactly what the survivor asserts and read as three per-command checks that the runner cannot perform. The needle is also TIGHTER than before: two of the three lines used the bare substring `env`, clean only while no summary in those three command trees says "environment". |
