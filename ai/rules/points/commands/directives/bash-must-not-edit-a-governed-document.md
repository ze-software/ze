---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
- **A file under `plan/` or `ai/rules/` MUST NOT be written from Bash.** Use the Write or Edit tool. `bashGovernedWrite` in `internal/le/hookruntime/bash.go` refuses it.
- **The Write/Edit surface runs the native checks in `internal/le/hookruntime/writeedit.go`.** A Bash write reaches none of them.
- **The bypass is common.** Redirects, in-place editors, `tee`, copy, move, and interpreter payloads can all write a governed document. The native Bash guard binds both direct shell verbs and interpreter-shaped writes.
- **The interpreter tier over-matches on purpose.** A payload merely reading `plan/` and writing its result to scratch can be refused. State the governed-write admission reason when that is genuinely the operation.
- **Writing ABOUT these trees trips it too, and that is the shape you will meet first.** A commit-body heredoc that merely NAMES `plan/` or `ai/rules/` beside a write primitive is refused, even when the only file it writes is scratch. The check's own author met this while writing the body for the commit that added the check. The answer is the Write tool for that file, which is the correct tool anyway, or the escape with a reason -- not a reworded sentence that dodges the pattern.
- **A refusal that is wrong is answered by `ZE_ADMIT_GOVERNED_WRITE="<reason>"`, never by rewording the command.** It mirrors `ZE_ADMIT_RAW`: an empty reason admits nothing, and the reason lands in the transcript, so the escape is auditable by READING the session rather than by trusting it. A false positive costs one env assignment; a false negative costs the guard, and that asymmetry is the whole argument.
- **READING from Bash stays free in the shell tier, because it binds on the write.** `grep`, `cat`, `sed -n`, and `./le commit create file plan/spec-x.md dry-run` are not document writes and are not refused; the commit path names those paths constantly and would otherwise refuse itself.
- **A generated artifact under those paths is written by its generator, not by hand, so it is not what this governs.** `./le rules render-update` and `internal/le/commit` write there as tools; the rule is about an agent editing a document.
