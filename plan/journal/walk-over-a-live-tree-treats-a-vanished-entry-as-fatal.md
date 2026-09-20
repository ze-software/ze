# A walk over a live tree treats a vanished entry as fatal

A directory is read while another process writes to it. Between the readdir
that lists an entry and the stat or open that reads it, the owner removes it.
The walk reports `no such file or directory` for a path it had just been told
existed, and the caller reads a correct directory as a broken one.

The tell is a failure that names a path nobody recognizes, moves to a different
victim on every run, and never reproduces alone.

Two halves, and both are worth fixing. The WRITER must not use a fixed file
name in a directory it shares, because two writers then delete each other's
file. The READER must skip `fs.ErrNotExist` from `entry.Info()`, from the open,
and from the walk function's own error argument, because a live tree gives no
other answer.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-20 | - (walked into while driving the ExaBGP compatibility suite to green) | `copyConfigDir` (`internal/test/cli/cmd_exabgp.go`) reading the run's shared `etc/ze`, against `probeWritable` (`internal/core/crashlog/persist.go`) writing into `etc/ze/crash` | One case out of 42 failed per run, a different one each time, with `lstat .../testbin-pid-<pid>-exabgp/etc/ze/crash/.probe: no such file or directory` or the same path under `open`. The 42 cases share one binary root and run six at a time, so they share `etc/ze`; `copyConfigDir` walks it to seed each case's own config directory while other cases' daemons start. `resolveCrashDir` probes each candidate by creating `.probe`, closing it and removing it, under a name every process picks identically, so the file appears and disappears in the shared tree throughout the run. The walk's `entry.Info()` then lstats a name the readdir had listed and the probe had already deleted, and one fatal error failed the whole case before its client started | FIXED 2026-09-20, both halves. `probeWritable` uses `os.CreateTemp(dir, ".probe-")`, so no two processes pick the same name and neither can delete the other's file: the shared name was wrong even with no reader present, because the first process to finish made the second read its own successful write as a failure. `copyConfigDir` answers `nil` for `fs.ErrNotExist` at the walk error, at `entry.Info()` and at the read, because an entry that vanishes under a live tree is the tree behaving correctly. Neither half alone is enough: a unique name still leaves a window where the file exists, and a tolerant reader does not stop two daemons racing each other |
