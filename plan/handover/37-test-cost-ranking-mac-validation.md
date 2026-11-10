# Handover 37: validate the test cost ranking on the Mac

**Run this on Thomas's Mac only.** The Linux development VM is a host of unknown
performance, so it cannot produce either half (`ai/META-DECISIONS.md`,
performance figures).

**Spec:** `plan/post-release/engineering/spec-test-cost-ranking-mac-validation.md`
(ready). Claim it with `./le spec claim spec-test-cost-ranking-mac-validation.md`
and read it whole first.

## State

`le test cost report` now lists a test when its share of its suite or package
run is at least three times an even share (`3/N` for `N` tests), with no floor
in seconds. A group of three tests or fewer ranks nothing. It landed in
`3cfda91c17` and `8735f42cba`, and `spec-test-cost-ranking-across-hosts` closed
after them. Nothing is uncommitted for this work.

What is not yet known is whether the rule names the same tests on two hosts.
The rule reads shares only, so it should, but only if a test's share of its own
run holds across hosts. That is what this handover checks.

## What to do

| Step | Where | Command |
|------|-------|---------|
| 1 | macOS | `./le test ui -a`, three times (a `.ci` test is ranked only from three baseline samples) |
| 2 | macOS | `./le test cost report suite ui` |
| 3 | macOS | a Go package with at least twenty tests: `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt \| sort -u \| tr '\n' ' ')"; ./le job run label unit-pkg quiet command go test -v -race -tags "$tags" <package>`, then `./le test cost report log <the log path it printed>` |
| 4 | Linux QEMU guest on the Mac | the same three steps inside `./le test qemu docker-lab command "..." kernel <matching runtime vmlinuz> harness`, which boots Ze's docker-host image (Ze's system) with the guest le built and the checkout at `/workspace`. Never `./le test qemu run`: it boots an Alpine guest. Give the guest its own `.ci` baseline (A-2 in the spec): the checkout is shared, so copy the macOS `tmp/test-timings.json` aside first, or pass `baseline <path>` |
| 5 | either | compare the ranked test names and order of the two reports, suite against suite and package against package |

## What to record

Only the ranked test names and their order, per host, in the spec's AC-1. Never
a time or a share as a figure. A difference is named with the host that listed
the test. If the lists differ, AC-2 owes a change to `shareFactor` or to the
page `docs/contributing/running-commands.md`, "Which tests cost the most of
their run", so a reader is not misled.

## Do not assume

- That any QEMU guest runs Ze's system: `./le test qemu run` boots Alpine even
  with `kernel`, so use `./le test qemu docker-lab command ... harness`, which
  boots Ze's image and requires the matching runtime kernel.
- That the two hosts' `.ci` baselines are separate: one shared checkout holds one
  `tmp/test-timings.json`.
- That a test near the bar flipping between hosts is noise: record it, with
  which host listed it.
