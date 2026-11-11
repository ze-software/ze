# Pointer shared across the names it indexes

One pointer records WHICH version is current, and the storage it indexes is
keyed by version AND by name. The pointer carries only the version, so a reader
that asks for a different name resolves to a path nobody wrote. The lookup then
fails as "not found" rather than as "the current version belongs to another
name", and the fallback below it reports a third thing again.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-02 | - | `ze start`, `internal/component/config/storage/pointer.go` | Under blob storage `pointerPath` answers `pointerKey(pointer)`. That function takes no config name. So `meta/config/active` is ONE pointer for the whole database. `versionPath` is per name, and a version lives at `file/<stamp>/<name>`. On this checkout the pointer holds `20260827-193839.365`. The only blob at that stamp is `file/20260827-193839.365/startup-smoke.conf`, which a functional test wrote. `ze start` defaults to the name `ze.conf`. `ReadActiveConfig` then resolves `file/20260827-193839.365/ze.conf`, which does not exist. The error the operator reads is neither of those two facts. The blob miss is discarded. `os.ReadFile(configPath)` runs as the filesystem fallback. The daemon reports `read config: open ze.conf: no such file or directory`, which names a file in the working directory. `file/active/ze.conf` IS in the database. `ReadActiveConfig` reaches that legacy path only when the pointer is ABSENT. A wrong pointer therefore hides a config that is present. A plain `ze start` cannot boot on this machine | open: not fixed. An operator question surfaced it during an unrelated CLI spec, and it blocks no goal of that spec. The workaround is a positional path, which falls through to `os.ReadFile`. Whoever takes it decides the intent first. One active pointer per DATABASE means the reader must not demand the caller's name. A pointer per NAME means `pointerKey` owes the name that `versionPath` already takes |
| 2026-09-04 | fixit-tunnel-traffic-proof-is-one-directional | `./le commit create ... append`, the message path it derives from session and tag | The script path carried a random suffix and the message path `tmp/commit-msg-<session>-<tag>.txt` did not, so `append` under one tag overwrote commit A's message and both `git commit -F` lines named one file | fixed: `bc987697e1` ("a prepared commit's message carries its own suffix"); `allocateMessage` (`internal/le/commit/script.go`) writes `commit-msg-<session>-<tag>-<nonce>.txt`, the random suffix this row proposed |
| 2026-09-05 | - | `./le commit create`, twice under one tag in one session | Second occurrence, without `append`: two `create` calls under one tag made two scripts and one message file, so the first script committed the second's subject, which landed on main as `x` | fixed: `bc987697e1`; `allocateMessage` (`internal/le/commit/script.go`) gives each prepared commit its own `commit-msg-<session>-<tag>-<nonce>.txt`, so a second `create` cannot overwrite the first's message |
| 2026-09-05 | - | same-tag create after truncated output | Recovering a truncated script path overwrote another prepared message; [original details](#2026-09-05-third-same-tag-create). | fixed: bc987697e1; exclusive-message proof ([P-5][tooling]) |

## Occurrence details

### 2026-09-05 third same-tag create

Original surface: `./le commit create`, twice under one tag in one session, for the third time in one day

Original symptom: Third occurrence of the two rows above, an hour after the second and reached the same ordinary way: the `script=` line was truncated out of a piped result and `create` was re-run to recover it. It left `tmp/commit-b7d16f63-b-bdcdab.sh` with the subject `x`, staging `ai/PACKAGE-MAP.md`, unrun. The trigger is not carelessness, which is why two rows asking authors to change a habit did not stop it: truncating a long tool output and re-running the command is the obvious move, and the tooling punished it in silence

[tooling]: ../post-release/engineering/tooling-one-off-evidence.md
