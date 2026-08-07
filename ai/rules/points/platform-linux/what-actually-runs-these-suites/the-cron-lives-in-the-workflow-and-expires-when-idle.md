---
kind: note
level:
stage:
---
The cron lives IN `evidence-nightly.yml` (`on: schedule: - cron:`), so merging it
to the default branch CREATES the schedule, unlike Woodpecker, whose cron was a
separate repo setting nothing in the repo recorded. The one caveat: GitHub
disables scheduled workflows after 60 days with NO repository activity, so a long
quiet period silently stops the nightly; a `workflow_dispatch` (manual) trigger
is provided as the re-arm.
