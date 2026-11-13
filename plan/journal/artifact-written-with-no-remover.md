# Artifact written with no remover

A program writes one file per run into a directory it shares with every other
run, and nothing ever removes it: not the writer, not a reader, not a cleaner.
Each file is small and correct, so no run notices. The directory grows by one
entry per run for the life of the checkout, and every program that lists it
pays for all of them.

The tell is a directory whose entry count tracks the number of runs ever made,
and a rule that says a reader never deletes what it did not write, with no
second rule saying who does.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | main-thread-schedules-heavy-work (walked into, research) | `(*Ticket).Release` and `writeResult` (`internal/le/job/job.go`, `registry.go`), the job registry `tmp/.ze-jobs` | Release removes the entry and the log and keeps the `.rc` verdict receipt, and waiters never prune: the directory held 4,756 receipts beside 13 entries, so every admission scan and `le job report` lists thousands of dead files | fixed: `./le scratch clean` prunes the registry (`job.Prune`): receipts whose entry is gone, and the files of dead jobs and waiters, once older than 3h |
