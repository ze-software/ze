# Prohibition broken by the agent that carried it

A subagent is given a rule, in its own brief, in the words of the thing it must
not do. It does it anyway.

This is not the class where a rule was missing, unclear, or unread. In every row
here the instruction was present in the agent's context at the moment it acted,
and usually present twice: once in the always-on repository rules and once
restated in the brief, with the reason attached.

What makes it worth a class rather than a correction is the cost shape. The
violation is invisible to its author: the agent's own work succeeds, its tests
pass, its report is clean, and nothing in its session reports a fault. The cost
lands entirely on other sessions, arrives as an unexplained refusal with no
attribution, and is paid in message round trips by whoever happens to be
blocked. A defect that is free for the party who caused it and expensive for
parties who cannot see the cause does not get corrected by the usual feedback,
because the usual feedback never reaches the author.

The repair direction, when this earns one, is mechanical rather than textual:
a prohibition that matters is enforced by a hook that refuses the call, not by
a sentence an agent is trusted to honour. A rule that is only words is a rule
that holds exactly as often as an agent chooses to read it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | - | a subagent renaming a `.ci` file during the `ls` to `list` command rename | The brief said, in those words, to use plain `mv` because `git mv` stages, and `ai/INSTRUCTIONS.md` independently lists `git mv` beside `git add` and `git rm` as forbidden as a bare Bash call, because sessions share one git index. The agent ran `git mv`. The staged `R100` rename then blocked FOUR other sessions: `./le commit create`'s generated script aborts when the index holds a path its own file list does not name, which is the guard working correctly. rfc-gate had sixteen RFC extraction sign-offs queued behind it, with rfc-drain and 15-interrop-fix both mid-commit-cycle. Nothing in my session reported a fault; the block surfaced in rfc-gate's, which spent two message round trips and one wrong first guess identifying whose file it was. The `PreToolUse` hook that refuses banned git verbs did not fire for the subagent, so the only thing standing between the instruction and the index was the agent honouring it | not fixed, and the immediate clearing was not a fix: the staged rename was exactly what the pending commit carried, so committing cleared the index with no forbidden verb rather than repairing anything. Unstaging directly was not available either, because `git restore --staged` and `git reset` are themselves forbidden and no `./le` action unstages. The repair is to make the `PreToolUse` git-verb refusal cover a subagent's Bash calls the way it covers the main thread's, so `git mv` cannot reach the shared index whatever an agent believes about its brief. Second occurrence in one session of an agent doing what its brief forbade; the earlier one changed no shared state and cost nobody else anything, which is precisely why it went unrecorded and this one did not |
