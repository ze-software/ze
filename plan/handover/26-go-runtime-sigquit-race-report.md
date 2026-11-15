# 26 Report the Go runtime traceback hexdump throw to golang/go

One job: report a Go runtime bug upstream, correctly. An AI found the bug,
diagnosed it and wrote the fix. The report says Thomas used an AI assistant to
do all the legwork, and it does not describe the method. A wrong or inflated
report would cost Thomas credibility with the Go team, so every sentence in
it is something observed in a run or read in the Go source, and what is not
established is listed as such. Nothing is posted to GitHub until Thomas has
read and approved the exact text.

## State on 2026-11-15

The report is prepared and waits for Thomas's review. Nothing is posted.

| Piece | State |
|-------|-------|
| Issue text, in Thomas's voice | Written: `~/Code/go-race-sigquit-repro/report/issue.md` on the Mac |
| Reproducers | Two standalone programs: darwin/arm64 `-race`, linux/amd64 `-asan` |
| Fix and test, as a CL would carry them | Diff against golang/go `840c8327` in `cl/`, applied in the clone `go-tip/` |
| Evidence | `results/` and `analysis/`; `README.md` maps each number in the issue to its file |
| Independent review | Two passes by a separate reviewer told to falsify the text; all findings corrected. Not reviewed again after the last corrections |
| Thomas's review and approval | Owed |
| Posting, and mailing the CL | Owed, on Thomas's order only |

That directory is outside the repository on purpose: the report must stand
without Ze. It is not backed up by git.

## What the bug is

Since go1.26 (CL 681375, "runtime: add hexdumper"), `tracebackHexdump` throws
`fatal error: unsafe.Slice: len out of range` when the frame it is given is
far from the stack it is given, because the clamped range is inverted and
`hexdumpWords` now takes a length. The frame is off the stack when a signal
arrives while a goroutine is in `racecall`, `msancall` or `asancall`, which
switch SP to the system stack without switching g. A SIGQUIT dump then stops
after its first goroutine with that fatal error and `panic during panic`.
The second traceback starts the running goroutine from `gp.sched`, whose SP
`gogo` cleared, and faults: the process is killed by SIGSEGV on linux and
spins for ever on darwin. The full account, with line references at
`840c8327`, is in the issue text.

## What this session established, and what the earlier text had wrong

- Reproduced on go1.26.0, go1.27.2, go1.27.4 and tip on both platforms;
  go1.25.0 is clean on both. With the fix, no child throws.
- The stall on darwin is observed, not inferred: lldb shows the main thread
  on the load in `resolveInternal` with `EXC_BAD_ACCESS` at address 0.
- **Linux does not hang.** The earlier version of this file read
  `exec.test hang 10m0s` in the builder logs as a ten minute hang. That is the
  name of the test's helper command. The logs end in
  `signal: segmentation fault` within a fraction of a second.
- The darwin trigger needs the race runtime to be in a kernel call
  (`swtch_pri`) on behalf of a user goroutine. A signal inside the race
  runtime's own code is handled quietly.
- On linux/amd64 the bug did not reproduce with `-race` (190 children). It
  reproduces with `-asan` in more than half the children.
- golang/go#73470 is closed (inactivity, 2026-10-29), but #55167, open, shows
  the same failure eleven more times up to 2026-10-05, on the go1.26 and
  go1.27 release branches too. #76585, #76912, #78059, #78976 and a plan9
  builder show the same throw through the other callers of the hexdump.
- No issue or CL names the defect. One comment on #81238 mentions the throw
  in passing.

## What is in Ze

- The patch: `internal/le/go/stdpatch/patches/runtime-traceback-hexdump-off-stack.patch`,
  stamped `Ze-Reviewed-Against: go1.27.3`, documented in
  `docs/contributing/patching-the-standard-library.md`. Its code is the same
  guard the upstream diff carries; the comment wording differs.
- Its header still says the darwin stall is inferred and that nothing is
  reported upstream. Both lines are corrected in step 4 below, not before:
  one edit, when there is an issue number to cite.

## Decisions that are Thomas's

| Decision | What each answer changes |
|----------|--------------------------|
| Approve, change or reject the issue text | Nothing is posted without it |
| Publish the `go env` block as it is | It holds home directory paths and the Ze cache path. The Go issue form asks for it; trimming it is the alternative |
| Offer the CL in the issue | The text says only that he has signed the Google CLA, and makes no explicit offer to mail the change. Remove the sentence if he would rather only report |
| Name the project | The text says "a project of mine". The handover commit 54cebcd0a6 already cross-references Ze on #73470 |

## How to finish

1. Thomas reads `report/issue.md` and the `README.md` beside it. Changes go
   into `report/issue.template.md`, then the text is rebuilt (the README says
   how) so the embedded files stay exact.
2. Before posting, search golang/go issues and Gerrit again for
   `tracebackHexdump`, `hexdumpWords` and `unsafe.Slice: len out of range`,
   and check that golang/go master still has the unguarded call. The text
   cites tip at `840c8327` (2026-11-14); if master moved in that function,
   rerun the test and correct the references.
3. Post only the approved text, through `gh`, on Thomas's order. The first
   line of `issue.md` is the title. If he wants the CL, the commit message is
   `report/cl-commit-message.txt` and the change is the working tree of
   `go-tip/`; it goes to Gerrit under his account.
4. After posting, update the patch header's "Upstream status" line and its
   "inferred, not traced" sentence, and the doc page, with the issue link,
   through `./le commit create`. Then close this handover.
5. Remove the scratch on the Linux host `claude`: `~/.cache/go-sigquit-asan`,
   `~/.cache/go-sigquit-repro`, `~/.cache/go-tip-sigquit`. Keep the Mac
   directory until the issue is resolved upstream.

## Traps met while preparing it

- `./le job run` puts Ze's patched go1.27.3 toolchain on the path. A build
  meant to be stock names its GOROOT and its `go` binary explicitly, and the
  binary's line table is checked before its result is trusted.
- The Bash hook reads the text of a quoted remote command, a heredoc or a
  README as a local command, and refuses `go build`, `go test` and filters in
  it. Files are written with the Write tool, and a remote run carries
  `ZE_ADMIT_RAW` with its reason in front of the `go` command.
- A hung child of the reproducer never exits. Kill it by pid.
