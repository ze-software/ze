---
kind: directive
level: MUST
stage:
---
The helper-generated commit script MUST produce two commits:
1. **Commit A (implementation + spec):** `./le commit create replace`
   with `--file` for all code, tests, docs, journal row,
   AND the spec file itself (with all edits from implementation).
2. **Commit B (spec closure):** `./le commit create append remove plan/<spec>` only.
   If the spec has a deferral shard AND every row in it is terminal,
   `--remove plan/deferrals/<spec-stem>.md` in the SAME commit B: deferrals are
   sharded per source ("Central Log", below). **A shard still holding a
   live row is NOT removed.** That row is homed at a different spec, and the shard is
   only where it is written down, so removing it deletes a record of live work. Read
   the Status column before you add the `--remove`; an all-terminal shard is residue
   and goes, a live-bearing shard survives its source spec and keeps its name.
   **This extends to FOREIGN shards this closure emptied.** Resolving a row homed
   here can set the last live row of another spec's shard to `done`; that shard is
   now residue and the same commit B removes it. Nobody else will: every other
   closure scopes its `--remove` to its own stem.
