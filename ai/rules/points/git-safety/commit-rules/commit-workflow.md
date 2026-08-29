---
kind: directive
level: MUST
stage:
---
**Commit workflow:**
1. You MUST use `./le commit session` to create or reuse this harness session's
   eight-hex commit namespace.
2. You MUST use `./le commit create` to write one message file and one commit
   script. Pass `file <path>` once per explicit file and `remove <path>` for
   tracked deletions. The `script=` line is the only authoritative path.
   `append` adds a later commit block to a prepared script; `script <path>` names
   it. `replace` rewrites that named script. A new `create` with no `script`
   always gets a distinct path.
3. The native command writes executable scripts using `git commit -F`, rejects
   ignored/generated paths, checks verification freshness for the explicit file
   population, and records verification debt rather than dropping a local
   commit. `push "<owner authorisation>"` is refused while debt is open. It also
   enforces discovery-index freshness; use `./le discovery-index update`.
4. `./le commit create` is the sole staging and commit route. There is no
   hand-written fallback.
5. You MUST run the finished script yourself with `bash` and the printed path.
   For a commit carrying Go/module/vendor paths, run
   `./le repository tracked-build check` immediately afterwards. Report the commit SHA,
   included files, message file, script path, push status, and verification
   evidence or skip reason.
6. Before creating the script, read `.gitignore`; add only canonical sources.
**The command asks for no lesson artifact, and it MUST NOT be made to.** Apply a
lesson by updating the surface that governs behaviour. Journal rows exist only
to count recurrence of a problem class.
