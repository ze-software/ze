# Rationale: Context Economy

The rules in `ai/rules/context-economy.md` and the five-minute directive in
`ai/rules/commands.md` come from one measurement, taken 2026-09-15 over the
eight sessions this checkout ran between 2026-09-07 and 2026-09-14: 10,736 API
calls, 99 subagents, read from the machine-local transcript store
(`./le ai tokens` prints the same shape over every session).

## What a call costs

An API call re-feeds every token of the context, and the price is set by how
each token arrives: a cache read costs 0.1 of a fresh token, a cache write 1.25
(a 5-minute cache) or 2 (a 1-hour cache), an output token 5.

| Where the spend went | Share |
|----------------------|-------|
| Cache reads (re-feeding the context) | 75% |
| Cache writes | 22% |
| Output, thinking included | 3% |

So the cost of a session is `calls x context at each call`, and neither the
tokens the model wrote nor the tokens a tool returned move it. Subagents were
81% of the total, and `ze-work` agents 91% of that.

Of the 10,736 calls, 89% carried exactly one tool call, and the median Bash
result was 600 tokens: each such call paid its whole context for one small
answer. That is why the rule counts turns, not tokens, and why a chunked read
costs more than a whole one past the second slice.

The main thread is the same arithmetic on a smaller share: 19% of the spend,
one session averaging 375k over 335 calls, because a 1M window never compacts.
No hook reaches the main thread, so the answer is a habit in
`.claude/rules/session-start.md`: one spec per session, and the next spec in a
fresh one.

## Why an editing agent hands off at 100 tool calls

A `ze-work` agent starts at a 45k floor (system prompt, CLAUDE.md, CORE.md, the
rules mirrors, the tool schemas) and grows by about 1.9k tokens per call.
Median agent: 71 calls, 194k at its last call. The eleven largest: 200-400
calls, 400-750k at their last call, and 41% of everything the eight sessions
spent. Capping every context at 200k would have cost 77% of what was paid,
at 100k 53%.

The break-even is short. Past 250k, one call reads 25k tokens for one tool
result. A successor pays its 45k floor once, at the write price, plus the
handoff it reads: about 60k token-equivalents, which the continuing agent
spends again every 25 calls.

The `/ze-implement` skill already cut a spec into one agent per phase and told
the agent that reaching the edge of its context was not reaching the edge of
its package. The phase agents still ran to 750k, because nothing named a size.
The number does.

## Why a subagent runs nothing past five minutes

Every subagent call was written with `ephemeral_5m_input_tokens`; every
main-thread call with `ephemeral_1h_input_tokens`. A tool call that outlasts
the 5-minute window ends the cache, and the next call rewrites the whole
context at the write price for one result.

Over the eight sessions: 65 such rewrites in subagents, 19.2M tokens written
again, about 10% of the spend. The price is the context at that moment, so a
fresh agent spawned for the gate alone (`/ze-verify` on `ze-read`) loses only
its 45k floor, while an implementation agent at 500k loses 500k. The commands that ran past five minutes, in
subagents only: `./le go lint run` (21 of 112 runs), `./le job run` (10 of
399), `./le test functional ui` (4 of 9), `./le test functional encode`, `./le test unit
config`. Twenty-one of the lint runs hit the 600-second Bash timeout, so the
agent lost the cache and got no result. The main thread ran no command past
five minutes and had no expiry of this kind.

## What a subagent reads

Across 103 agents, 6.5MB of file content entered contexts through Read, `cat`
and `sed -n`. Rules and docs were 1.33MB of it, `docs/contributing/ze-go-style.md`
alone 470KB over 30 agents, which is about 1.5% of the spend: the guide is a
small cost and stays whole. The reads that are pure loss:

| Read | Volume | Why it is a loss |
|------|--------|------------------|
| A `<persisted-output>` file read whole | 46 reads, 874KB | The command's output exceeded 33KB, and the whole of it entered the context to find a few lines |
| A file read in consecutive `sed -n` slices | 70 turns, 530KB | Each slice is a turn that re-feeds the context; one read of the range needed is one turn |

The spec itself was read 103 times over 103 agents, 796KB: once per agent, which
is the floor, and at 4-5k tokens a read it is cheap beside the 45k harness floor.
