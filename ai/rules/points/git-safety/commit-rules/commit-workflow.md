---
kind: directive
level: MUST
stage:
---
**Commit workflow:**
1. You MUST use `./le commit session` to create or reuse this harness session's eight-hex commit namespace.
2. You MUST use `./le commit create` to write one message file and one commit script. It is the sole staging and commit route, and there is no hand-written fallback. `docs/contributing/committing.md` carries the keywords, the refusals, and worked invocations.
3. **The `script=` line the command prints is the only authoritative path. You MUST copy it, and MUST NOT construct it from the session id.**
4. Before creating the script, you MUST read `.gitignore` and name only canonical sources.
5. You MUST read the generated message file before running the script, at the path on the `message=` line.
6. You MUST run the finished script yourself with `bash` and the printed path. For a commit carrying Go, module, or vendor paths, you MUST run `./le repository tracked-build check` immediately afterwards.
7. You MUST report the commit SHA, the included files, the message file, the script path, the push status, and the verification evidence or the skip reason.
**The command asks for no lesson artifact, and it MUST NOT be made to.** Apply a lesson by updating the surface that governs behaviour. Journal rows exist only to count recurrence of a problem class.
