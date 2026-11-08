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
| 2026-08-31 | - | a subagent renaming a `.ci` file during the `ls` to `list` command rename | A subagent ran `git mv` against its brief and the `ai/INSTRUCTIONS.md` ban, and the staged rename blocked four other sessions' `./le commit create` scripts, because the `PreToolUse` hook refusing banned git verbs did not fire for a subagent | fixed: the `pretool-bash` refusal covers a subagent's Bash call: `TestDelegatedRenameUsesCommitRoute` (3475223c4b) sends `git mv` with an `agent_id` and gets exit 2 |
