# Pointer shared across the names it indexes

One pointer records WHICH version is current, and the storage it indexes is
keyed by version AND by name. The pointer carries only the version, so a reader
that asks for a different name resolves to a path nobody wrote. The lookup then
fails as "not found" rather than as "the current version belongs to another
name", and the fallback below it reports a third thing again.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-02 | - | `ze start`, `internal/component/config/storage/pointer.go` | Under blob storage `pointerPath` answers `pointerKey(pointer)`. That function takes no config name. So `meta/config/active` is ONE pointer for the whole database. `versionPath` is per name, and a version lives at `file/<stamp>/<name>`. On this checkout the pointer holds `20260827-193839.365`. The only blob at that stamp is `file/20260827-193839.365/startup-smoke.conf`, which a functional test wrote. `ze start` defaults to the name `ze.conf`. `ReadActiveConfig` then resolves `file/20260827-193839.365/ze.conf`, which does not exist. The error the operator reads is neither of those two facts. The blob miss is discarded. `os.ReadFile(configPath)` runs as the filesystem fallback. The daemon reports `read config: open ze.conf: no such file or directory`, which names a file in the working directory. `file/active/ze.conf` IS in the database. `ReadActiveConfig` reaches that legacy path only when the pointer is ABSENT. A wrong pointer therefore hides a config that is present. A plain `ze start` cannot boot on this machine | not fixed. An operator question surfaced it during an unrelated CLI spec, and it blocks no goal of that spec. The workaround is a positional path, which falls through to `os.ReadFile`. Whoever takes it decides the intent first. One active pointer per DATABASE means the reader must not demand the caller's name. A pointer per NAME means `pointerKey` owes the name that `versionPath` already takes |
