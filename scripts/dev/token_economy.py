#!/usr/bin/env python3
"""Measure where this repository's Claude Code sessions spend their tokens.

The bill of an agent session is `round trips x context`: every API call is
charged the whole context it carries, so the two terms that move it are the
number of calls and the size of the context at each one. This tool reports both
from the local transcript store, per session and per subagent phase, and adds a
capped-context counterfactual that says what the same calls would have fed the
model had context been held at a ceiling.

Store layout (read-only, written by Claude Code):

    ~/.claude/projects/<project-slug>/<session>.jsonl
    ~/.claude/projects/<project-slug>/<session>/subagents/agent-<id>.jsonl
    ~/.claude/projects/<project-slug>/<session>/subagents/agent-<id>.meta.json

ONE API CALL IS WRITTEN AS SEVERAL ASSISTANT RECORDS that share one
`message.id`. Counting records instead of ids inflates every context figure:
one measured transcript holds 168 assistant records for 98 API calls, and a
26-file sample held 3,613 records for 1,732 calls.

The records of one id are not identical. The three context fields repeat
unchanged, but `output_tokens` GROWS and only the last record carries the
finished count, so each field is merged by maximum. Both properties are
`scan_transcript`, which is the correctness of this tool.

ONE API CALL ALSO APPEARS IN SEVERAL TRANSCRIPTS. A resumed session and a
forked agent both copy the earlier records into their own file, so a per-file
dedup still counts those calls once per file. Measured over this store: 487
message ids live in two transcripts and carry 208M context tokens between
them. `assign_owners` picks one transcript per id for the whole store.

The store is machine-local, so the figures describe one developer's machine and
not the repository. An absent store is reported as such, never as zero cost.

Token counts only. No pricing: what is measured here is what is fed to the
model, and turning that into money needs a price list this tool has no business
carrying. The one approximation is the size of a tool result, which the
transcript records as text and not as tokens: it is reported as characters
divided by CHARS_PER_TOKEN, and the report says so on the line that prints it.

Usage:
    python3 scripts/dev/token_economy.py              # this repo's slug
    python3 scripts/dev/token_economy.py --cap 150000 # counterfactual ceiling
    python3 scripts/dev/token_economy.py --top 5      # list five sessions
"""

from __future__ import annotations

import argparse
import json
import re
import statistics
import sys
from dataclasses import dataclass, field, replace
from pathlib import Path

DEFAULT_ROOT = Path.home() / ".claude" / "projects"
REPO_ROOT = Path(__file__).resolve().parents[2]

CAP_MIN, CAP_MAX = 1, 1_000_000
TOP_MIN, TOP_MAX = 1, 1000
DEFAULT_CAP = 200_000
DEFAULT_TOP = 15

# Upper edges of the context-size buckets, in tokens. The last bucket is open.
BUCKET_EDGES = (50_000, 100_000, 200_000, 400_000, 600_000, 800_000, 1_000_000)

# A tool result is recorded as text, never as a token count, so its size is an
# APPROXIMATION. Every figure derived from it says so where it is printed.
CHARS_PER_TOKEN = 3.6

# Tools listed in the result-size table. The rest are folded into one row, so a
# long tail of one-off tools does not bury the tools that carry the tokens.
RESULT_TABLE_ROWS = 10

NON_SLUG = re.compile(r"[^A-Za-z0-9-]")

# Phase classification is a KEYWORD HEURISTIC over the spawn description in
# <agent>.meta.json, not a recorded fact: nothing in the store labels an agent
# with the phase it ran. First match wins, so the order below is the ranking.
# A description matching nothing is `unclassified` rather than silently folded
# into a neighbouring phase.
PHASE_RULES: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("review", ("review", "critique", "adversarial", "lens", "referee")),
    ("audit", ("audit", "conformance")),
    ("fix/debug", ("debug", "fix", "repair", "failing", "diagnose", "flake", "red ")),
    ("test", ("test", "coverage", "fixture", "reproduce")),
    (
        "implement",
        ("implement", "build", "wire", "port", "migrate", "refactor", "add "),
    ),
    ("docs/rules", ("doc", "rule", "spec", "learned", "summar", "index", "write")),
    (
        "research",
        (
            "research",
            "explore",
            "investigate",
            "find",
            "search",
            "survey",
            "classify",
            "measure",
            "check",
            "verify",
            "read",
            "map",
            "trace",
        ),
    ),
)


@dataclass(frozen=True)
class Call:
    """One API call, from the deduped `message.usage` of its assistant records."""

    input: int = 0
    cache_write: int = 0
    cache_read: int = 0
    output: int = 0
    tools: int = 0

    @property
    def context(self) -> int:
        """Tokens fed to the model on this call: what the call is charged for."""
        return self.input + self.cache_write + self.cache_read


@dataclass(frozen=True)
class ToolCall:
    """One `tool_use` block, with the size of the result fed back for it.

    `result_chars` is 0 when no `tool_result` names this block: the tool was
    denied, the transcript ends before the result, or the result carried no
    text. That is a real zero and is counted as one, never dropped.
    """

    name: str
    file_path: str = ""
    result_chars: int = 0


@dataclass
class Agent:
    """One subagent transcript plus the spawn metadata beside it."""

    name: str
    agent_type: str
    description: str
    calls: list[Call] = field(default_factory=list)
    tool_calls: list[ToolCall] = field(default_factory=list)
    is_fork: bool = False
    prompt_chars: int = 0

    @property
    def phase(self) -> str:
        return phase_of(self.description, self.agent_type)

    @property
    def harness_floor(self) -> int:
        """First-call context with the spawn prompt taken back out.

        What the harness supplied before the agent did anything: system
        prompt, tool schemas, and the SubagentStart injection. Subtracting the
        prompt is what makes the number comparable between two agents, and
        stable as a session grows. Without it the figure tracks how much the
        parent happened to write, which is why the raw first call cannot be
        cited as a property of an agent TYPE.

        A fork is excluded by its caller, not here: it inherits the parent's
        whole context, so nothing about it is a floor.
        """
        if not self.calls:
            return 0
        return max(0, self.calls[0].context - int(approx_tokens(self.prompt_chars)))


@dataclass
class Session:
    """A main-thread transcript and the subagents spawned under it."""

    sid: str
    main_calls: list[Call] = field(default_factory=list)
    agents: list[Agent] = field(default_factory=list)
    main_tool_calls: list[ToolCall] = field(default_factory=list)

    @property
    def subagent_calls(self) -> list[Call]:
        return [c for a in self.agents for c in a.calls]

    @property
    def subagent_call_count(self) -> int:
        return len(self.subagent_calls)

    @property
    def all_calls(self) -> list[Call]:
        return self.main_calls + self.subagent_calls

    @property
    def threads(self) -> list[list[ToolCall]]:
        """One tool-call list per CONTEXT WINDOW: the main thread, then each agent."""
        return [self.main_tool_calls] + [a.tool_calls for a in self.agents]

    @property
    def all_tool_calls(self) -> list[ToolCall]:
        return [t for thread in self.threads for t in thread]


@dataclass(frozen=True)
class Bucket:
    """One row of the context histogram."""

    label: str
    calls: int
    context: int
    share: float


def slug_for_path(path: str | Path) -> str:
    """The project-slug Claude Code derives from a working directory."""
    return NON_SLUG.sub("-", str(path))


def _int(value) -> int:
    """A usage field, treated as untrusted input: anything unusable reads 0."""
    if isinstance(value, bool):
        return 0
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    return 0


def iter_records(path: Path):
    """Yield each JSON object of a transcript, skipping lines that do not parse.

    Transcript lines are untrusted input: a truncated final line (a session
    still running, a killed process) must cost the rest of the report nothing.
    """
    try:
        handle = path.open("r", encoding="utf-8", errors="replace")
    except OSError:
        return
    with handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except ValueError:
                continue
            if isinstance(record, dict):
                yield record


@dataclass
class Scan:
    """One transcript, read once: its API calls, their origin, and its tool use.

    Keyed by `message.id` throughout, so the store-wide owner resolution in
    `assign_owners` can drop an id and everything hanging off it together.
    """

    calls: dict[str, Call] = field(default_factory=dict)
    origins: dict[str, str] = field(default_factory=dict)
    tools: dict[str, list[ToolCall]] = field(default_factory=dict)
    # Characters of the FIRST user message: for a subagent, the prompt its
    # parent wrote. Held so `agent_type_startup` can take it back out of the
    # first call and leave what the harness supplied. Zero when the transcript
    # opens with no user text.
    prompt_chars: int = 0


def _blocks(message) -> list:
    """The content blocks of a message record, or an empty list."""
    if not isinstance(message, dict):
        return []
    content = message.get("content")
    return (
        [b for b in content if isinstance(b, dict)] if isinstance(content, list) else []
    )


def _result_chars(block: dict) -> int:
    """Characters of one `tool_result`, whose content is text or a block list."""
    content = block.get("content")
    if isinstance(content, str):
        return len(content)
    if isinstance(content, list):
        return sum(
            len(b.get("text", ""))
            for b in content
            if isinstance(b, dict) and isinstance(b.get("text"), str)
        )
    return 0


def scan(path: Path) -> Scan:
    """Read one transcript: API calls, the session that made them, and tool use.

    One call is written as several assistant records. The three CONTEXT fields
    repeat unchanged on each of them, so counting records instead of message
    ids multiplies the context figures by the split factor.

    `output_tokens` behaves differently and the difference is load-bearing: it
    GROWS across the records of one id, and only the last record carries the
    finished count. Measured over 1,172 message ids, the growth is monotonic in
    every one, and taking the first record's value summed 149k output tokens
    where the true figure is 1.22M. Each field is therefore merged by MAXIMUM,
    which is exact for a repeated field and for a monotonic counter alike, and
    which does not depend on the order the records are read in.

    Keying: `message.id` is present on every usage-carrying record measured
    (0 missing of 3,613), and `requestId` never disagrees with it. The record's
    own `uuid` is deliberately NOT a fallback -- it is per RECORD, so using it
    would restore the double-count this function exists to prevent.

    `tool_use` blocks are ALSO split across the records of one id, and the same
    block is written again on each one, so they are deduped by their own
    `toolu_` id before being attached to the call. Their results arrive in the
    later `user` records, keyed by `tool_use_id`, and are stitched on at the
    end because a result always follows the call it answers.
    """
    found = Scan()
    uses: dict[str, dict[str, tuple[str, str]]] = {}
    results: dict[str, int] = {}
    for record in iter_records(path):
        kind = record.get("type")
        message = record.get("message")
        if kind == "user":
            if not found.calls and not found.prompt_chars:
                # Before the first assistant record, so this is the spawn
                # prompt rather than a tool result fed back mid-run.
                content = message.get("content") if isinstance(message, dict) else None
                if isinstance(content, str):
                    found.prompt_chars = len(content)
                elif isinstance(content, list):
                    found.prompt_chars = sum(
                        len(b.get("text") or "")
                        for b in content
                        if isinstance(b, dict) and b.get("type") == "text"
                    )
            for block in _blocks(message):
                if block.get("type") != "tool_result":
                    continue
                use_id = block.get("tool_use_id")
                if isinstance(use_id, str) and use_id:
                    results[use_id] = max(results.get(use_id, 0), _result_chars(block))
            continue
        if kind != "assistant" or not isinstance(message, dict):
            continue
        usage = message.get("usage")
        if not isinstance(usage, dict):
            continue
        key = message.get("id") or record.get("requestId")
        if not isinstance(key, str) or not key:
            continue
        call = Call(
            input=_int(usage.get("input_tokens")),
            cache_write=_int(usage.get("cache_creation_input_tokens")),
            cache_read=_int(usage.get("cache_read_input_tokens")),
            output=_int(usage.get("output_tokens")),
        )
        previous = found.calls.get(key)
        found.calls[key] = call if previous is None else _merge(previous, call)
        # `session_id` names the session that MADE the call and `sessionId` the
        # file it now sits in. A resumed session rewrites the second and leaves
        # the first, which is what tells a copy from an original.
        origin = record.get("session_id") or record.get("sessionId")
        if isinstance(origin, str) and origin and key not in found.origins:
            found.origins[key] = origin
        for block in _blocks(message):
            if block.get("type") != "tool_use":
                continue
            use_id = block.get("id")
            name = block.get("name")
            if not isinstance(use_id, str) or not isinstance(name, str):
                continue
            data = block.get("input")
            target = data.get("file_path") if isinstance(data, dict) else None
            uses.setdefault(key, {})[use_id] = (
                name,
                target if isinstance(target, str) else "",
            )
    for key, by_use_id in uses.items():
        found.tools[key] = [
            ToolCall(name=name, file_path=target, result_chars=results.get(use_id, 0))
            for use_id, (name, target) in by_use_id.items()
        ]
        found.calls[key] = replace(found.calls[key], tools=len(by_use_id))
    return found


def scan_transcript(path: Path) -> list[Call]:
    """Every API call of one transcript, deduped by `message.id`. See `scan`."""
    return list(scan(path).calls.values())


def _merge(first: Call, second: Call) -> Call:
    """Field-wise maximum of two records of the SAME API call.

    Maximum, not last-record-wins: the merge must not depend on the order the
    records are read in, and only `output` varies between them at all. See
    `scan` for the measurement behind both halves of that sentence.
    """
    return Call(
        input=max(first.input, second.input),
        cache_write=max(first.cache_write, second.cache_write),
        cache_read=max(first.cache_read, second.cache_read),
        output=max(first.output, second.output),
        tools=max(first.tools, second.tools),
    )


def read_meta(path: Path) -> tuple[str, str, bool]:
    """The `agentType`, `description` and fork flag of a spawn.

    A fork inherits its parent agent's whole conversation, so its transcript
    repeats the parent's records. `assign_owners` uses the flag to give those
    calls back to the parent. Anything unreadable reads as empty and not-a-fork.
    """
    try:
        meta = json.loads(path.read_text(encoding="utf-8", errors="replace"))
    except (OSError, ValueError):
        return ("", "", False)
    if not isinstance(meta, dict):
        return ("", "", False)
    agent_type = meta.get("agentType")
    description = meta.get("description")
    return (
        agent_type if isinstance(agent_type, str) else "",
        description if isinstance(description, str) else "",
        meta.get("isFork") is True or bool(meta.get("parentAgentId")),
    )


def phase_of(description: str, agent_type: str = "") -> str:
    """Classify a spawn into a workflow phase from its description keywords."""
    text = f"{description} {agent_type}".lower()
    if not description.strip():
        return "unclassified"
    for phase, keywords in PHASE_RULES:
        if any(word in text for word in keywords):
            return phase
    return "unclassified"


@dataclass
class Transcript:
    """One transcript file, its identity in the store, and what it holds."""

    path: Path
    sid: str
    is_agent: bool
    agent_type: str = ""
    description: str = ""
    is_fork: bool = False
    found: Scan = field(default_factory=Scan)


def collect(store: Path) -> list[Transcript]:
    """Every transcript of a project store, read once, in a stable order."""
    out = [
        Transcript(path=p, sid=p.stem, is_agent=False, found=scan(p))
        for p in sorted(store.glob("*.jsonl"))
    ]
    for sub in sorted(store.glob("*/subagents/agent-*.jsonl")):
        agent_type, description, is_fork = read_meta(sub.with_suffix(".meta.json"))
        out.append(
            Transcript(
                path=sub,
                sid=sub.parent.parent.name,
                is_agent=True,
                agent_type=agent_type,
                description=description,
                is_fork=is_fork,
                found=scan(sub),
            )
        )
    return out


def assign_owners(transcripts: list[Transcript]) -> list[set[str]]:
    """One owning transcript per `message.id`, for the whole store.

    A resumed session and a forked agent both COPY the earlier records into
    their own transcript, so a dedup scoped to one file counts those calls once
    per file. Measured over this machine's store on 2026-08-05: 487 ids sit in
    two transcripts, worth 208,167,121 context tokens and 487 API calls, and
    the per-session table showed the parent's calls on the copy's row too.

    Ownership is decided from what the store RECORDS, in this order:

    1. The transcript whose own session equals the session the record names as
       the one that made the call (`session_id`, falling back to `sessionId`).
       This settles a resumed main session: the copy rewrites `sessionId` to
       its own and leaves `session_id` pointing at the original. It settles all
       431 main-to-main duplicates here, and the 3 agent duplicates that a
       resume copied into a second session directory.
    2. Failing that, a transcript that is not a fork beats one that is. A fork
       inherits its parent's context, so `<agent>.meta.json` carries
       `isFork: true` and `parentAgentId`. This settles the remaining 53.
    3. Failing that, the first transcript in collection order. Arbitrary and
       documented as arbitrary: no fact in the store separates the candidates,
       and the store-wide totals do not depend on which one wins.

    `sessionId` alone is NOT a discriminator: a resumed session rewrites it to
    its own id in every record it copied, so it names the file rather than the
    call in exactly the case that needs telling apart.
    """
    owners: list[set[str]] = [set() for _ in transcripts]
    candidates: dict[str, list[int]] = {}
    for index, transcript in enumerate(transcripts):
        for key in transcript.found.calls:
            candidates.setdefault(key, []).append(index)
    for key, indexes in candidates.items():
        if len(indexes) > 1:
            matched = [
                i
                for i in indexes
                if transcripts[i].found.origins.get(key) == transcripts[i].sid
            ]
            if len(matched) == 1:
                indexes = matched
            else:
                pool = matched or indexes
                original = [i for i in pool if not transcripts[i].is_fork]
                indexes = original or pool
        owners[indexes[0]].add(key)
    return owners


def find_sessions(store: Path) -> list[Session]:
    """Every session of a project store, each with its subagent transcripts.

    Every API call is attributed to exactly ONE transcript store-wide; see
    `assign_owners` for the rule and the duplication it removes.
    """
    transcripts = collect(store)
    owners = assign_owners(transcripts)
    sessions: dict[str, Session] = {}
    for transcript, owned in zip(transcripts, owners):
        found = transcript.found
        calls = [found.calls[key] for key in found.calls if key in owned]
        tools = [t for key in found.tools if key in owned for t in found.tools[key]]
        session = sessions.setdefault(transcript.sid, Session(sid=transcript.sid))
        if transcript.is_agent:
            session.agents.append(
                Agent(
                    name=transcript.path.stem,
                    agent_type=transcript.agent_type,
                    description=transcript.description,
                    calls=calls,
                    tool_calls=tools,
                    is_fork=transcript.is_fork,
                    prompt_chars=found.prompt_chars,
                )
            )
        else:
            session.main_calls = calls
            session.main_tool_calls = tools
    return [sessions[sid] for sid in sorted(sessions)]


def totals(calls: list[Call]) -> dict:
    """Aggregate one set of calls. Context is summed AND kept per call."""
    contexts = [c.context for c in calls]
    return {
        "calls": len(calls),
        "input": sum(c.input for c in calls),
        "cache_write": sum(c.cache_write for c in calls),
        "cache_read": sum(c.cache_read for c in calls),
        "output": sum(c.output for c in calls),
        "context": sum(contexts),
        "context_max": max(contexts, default=0),
        "context_mean": (sum(contexts) / len(contexts)) if contexts else 0.0,
    }


def bucket_labels() -> list[str]:
    labels = []
    low = 0
    for edge in BUCKET_EDGES:
        labels.append(f"{_short(low)}-{_short(edge)}")
        low = edge
    labels.append(f"{_short(BUCKET_EDGES[-1])}+")
    return labels


def histogram(calls: list[Call]) -> list[Bucket]:
    """Attribute context tokens to the bucket of the call that fed them."""
    labels = bucket_labels()
    counts = [0] * len(labels)
    context = [0] * len(labels)
    for call in calls:
        index = len(BUCKET_EDGES)
        for i, edge in enumerate(BUCKET_EDGES):
            if call.context <= edge:
                index = i
                break
        counts[index] += 1
        context[index] += call.context
    grand = sum(context)
    return [
        Bucket(
            label=label,
            calls=counts[i],
            context=context[i],
            share=(100.0 * context[i] / grand) if grand else 0.0,
        )
        for i, label in enumerate(labels)
    ]


def approx_tokens(chars: int | float) -> float:
    """A character count as tokens. An approximation, and labelled as one."""
    return chars / CHARS_PER_TOKEN


def tool_call_distribution(calls: list[Call]) -> list[tuple[str, int, float]]:
    """How many tool calls each API call carried: label, API calls, share.

    A round trip that carries one tool call pays its whole context for one
    result, so this distribution is what a batching rule is argued from. Rows
    run 0, 1, 2, 3 and `4+`; every API call lands in exactly one.
    """
    counts = [0, 0, 0, 0, 0]
    for call in calls:
        counts[min(call.tools, 4)] += 1
    total = len(calls)
    labels = ["0", "1", "2", "3", "4+"]
    return [
        (label, counts[i], (100.0 * counts[i] / total) if total else 0.0)
        for i, label in enumerate(labels)
    ]


def single_tool_share(calls: list[Call]) -> tuple[int, int, float]:
    """Tool-carrying API calls, of which how many carried exactly one, and the share."""
    using = [c for c in calls if c.tools]
    alone = sum(1 for c in using if c.tools == 1)
    return (len(using), alone, (100.0 * alone / len(using)) if using else 0.0)


def result_sizes(tool_calls: list[ToolCall]) -> list[tuple[str, int, float, float]]:
    """Per tool name: results, mean tokens, total tokens. Biggest total first.

    Size is the recorded `tool_result` text, so it is CHARACTERS divided by
    CHARS_PER_TOKEN. A denied or unanswered call contributes a real zero.
    """
    by_name: dict[str, list[int]] = {}
    for tool in tool_calls:
        by_name.setdefault(tool.name, []).append(tool.result_chars)
    rows = [
        (
            name,
            len(chars),
            approx_tokens(sum(chars) / len(chars)),
            approx_tokens(sum(chars)),
        )
        for name, chars in by_name.items()
    ]
    return sorted(rows, key=lambda row: row[3], reverse=True)


def repeat_reads(group: list[ToolCall], tool: str = "Read") -> tuple[int, int]:
    """Reads naming a file, and how many named one the group had already read.

    The group is a CONTEXT WINDOW: one thread, or one whole session. A repeat
    is `reads - distinct paths`, which needs no ordering -- whichever read is
    called the first one, the count of the rest is the same. A call with no
    `file_path` is in neither figure, because there is nothing to compare.
    """
    paths = [t.file_path for t in group if t.name == tool and t.file_path]
    return (len(paths), len(paths) - len(set(paths)))


def agent_type_startup(agents: list[Agent]) -> list[tuple[str, int, int, int, int]]:
    """Per agent type: (type, agents, calls, median harness floor, context).

    The floor is `Agent.harness_floor`: the first call with the spawn prompt
    subtracted, so it is what the harness gave the agent and not what its
    parent wrote. It is re-fed on every later call, so it multiplies by
    `calls` rather than being paid once. That is what the column is for.

    Subtracting the prompt is what makes the figure a property of the agent
    TYPE. The raw first call is not: it moves with each spawn's prompt length,
    so a median over it drifts as a live session grows, and a number cited
    from it stops reproducing. Scope with `--session` as well, because the
    always-on preamble changes size between sessions and it is the largest
    term here.

    A `fork` is reported with a floor of 0. It inherits its parent's whole
    context, so no part of its first call is a harness floor, and leaving the
    inherited figure in this column invited exactly the misreading above.

    Sorted by total context, descending: the type to fix first is the one
    feeding the most, not the one with the highest floor.
    """
    by_type: dict[str, list[Agent]] = {}
    for agent in agents:
        by_type.setdefault(agent.agent_type or "unknown", []).append(agent)
    rows = []
    for name, members in by_type.items():
        calls = [c for a in members for c in a.calls]
        floors = [a.harness_floor for a in members if a.calls and not a.is_fork]
        median = int(statistics.median(floors)) if floors else 0
        rows.append((name, len(members), len(calls), median, totals(calls)["context"]))
    return sorted(rows, key=lambda r: r[4], reverse=True)


def capped_counterfactual(calls: list[Call], cap: int) -> tuple[int, int, float]:
    """Real context total, the same calls with context capped, and the ratio.

    This is arithmetic over calls that already happened, never a prediction: a
    session run under a smaller context would have made different calls.
    """
    real = sum(c.context for c in calls)
    capped = sum(min(c.context, cap) for c in calls)
    return (real, capped, (100.0 * capped / real) if real else 0.0)


def _short(value: float) -> str:
    """A token count at three digits: 1_180_000 reads as 1.18M.

    The unit is chosen on the ROUNDED figure, so 999,999 reads as 1M rather
    than the 1000k a threshold on the raw value produces.
    """
    for unit, size in (("B", 1_000_000_000), ("M", 1_000_000), ("k", 1_000)):
        scaled = value / size
        if abs(scaled) >= 0.9995:
            return (
                f"{scaled:.0f}{unit}" if abs(scaled) >= 100 else f"{scaled:.3g}{unit}"
            )
    return f"{value:.0f}"


def _row(cells: list[str]) -> str:
    return "| " + " | ".join(cells) + " |"


def render(sessions: list[Session], cap: int, top: int) -> list[str]:
    """The whole report as lines, so tests read it without capturing stdout."""
    out: list[str] = []
    every_call = [c for s in sessions for c in s.all_calls]
    main_calls = [c for s in sessions for c in s.main_calls]
    sub_calls = [c for s in sessions for c in s.subagent_calls]
    agents = [a for s in sessions for a in s.agents]
    grand = totals(every_call)

    ranked = sorted(
        sessions, key=lambda s: totals(s.all_calls)["context"], reverse=True
    )
    shown = ranked[:top]
    out.append(f"Per session, by context tokens (top {len(shown)} of {len(sessions)})")
    out.append(
        _row(
            [
                "session",
                "calls",
                "main",
                "sub",
                "agents",
                "mean ctx",
                "max ctx",
                "cache-read",
                "cache-write",
                "output",
            ]
        )
    )
    out.append(_row(["---"] * 10))
    for session in shown:
        agg = totals(session.all_calls)
        out.append(
            _row(
                [
                    session.sid[:8],
                    f"{agg['calls']:,}",
                    f"{len(session.main_calls):,}",
                    f"{session.subagent_call_count:,}",
                    f"{len(session.agents):,}",
                    _short(agg["context_mean"]),
                    _short(agg["context_max"]),
                    _short(agg["cache_read"]),
                    _short(agg["cache_write"]),
                    _short(agg["output"]),
                ]
            )
        )

    out.append("")
    out.append("Totals")
    out.append(
        f"  API calls: {grand['calls']:,}"
        f"  (main {len(main_calls):,}, subagent {len(sub_calls):,})"
    )
    out.append(f"  Sessions: {len(sessions):,}   Subagents: {len(agents):,}")
    out.append(
        f"  Context fed: {_short(grand['context'])}"
        f"   mean {_short(grand['context_mean'])} per call"
        f"   max {_short(grand['context_max'])}"
    )
    feed = grand["cache_read"] + grand["cache_write"] + grand["input"]
    for name, value in (
        ("cache-read", grand["cache_read"]),
        ("cache-write", grand["cache_write"]),
        ("input", grand["input"]),
        ("output", grand["output"]),
    ):
        share = (100.0 * value / feed) if feed and name != "output" else None
        tail = f"  ({share:.0f}% of context fed)" if share is not None else ""
        out.append(f"  {name:<12}{_short(value):>8}{tail}")
    # A transcript can record calls whose every usage field is zero, so the
    # share is guarded on the DENOMINATOR rather than on the call list.
    if sub_calls and grand["context"]:
        sub_share = 100.0 * totals(sub_calls)["context"] / grand["context"]
        out.append(f"  Subagent share of context fed: {sub_share:.0f}%")

    out.append("")
    out.append("Context histogram: where the context tokens were fed")
    out.append(_row(["context at the call", "calls", "context tokens", "share"]))
    out.append(_row(["---"] * 4))
    for bucket in histogram(every_call):
        if not bucket.calls:
            continue
        out.append(
            _row(
                [
                    bucket.label,
                    f"{bucket.calls:,}",
                    _short(bucket.context),
                    f"{bucket.share:.1f}%",
                ]
            )
        )

    out.append("")
    out.append("Context histogram, main thread against subagents")
    out.append(
        _row(
            [
                "context at the call",
                "main calls",
                "main context",
                "main share",
                "sub calls",
                "sub context",
                "sub share",
            ]
        )
    )
    out.append(_row(["---"] * 7))
    for main_bucket, sub_bucket in zip(histogram(main_calls), histogram(sub_calls)):
        if not main_bucket.calls and not sub_bucket.calls:
            continue
        out.append(
            _row(
                [
                    main_bucket.label,
                    f"{main_bucket.calls:,}",
                    _short(main_bucket.context),
                    f"{main_bucket.share:.1f}%",
                    f"{sub_bucket.calls:,}",
                    _short(sub_bucket.context),
                    f"{sub_bucket.share:.1f}%",
                ]
            )
        )
    out.append("  Each share is of that column's own context total, so a main-thread")
    out.append("  figure is never diluted by the subagent calls beside it.")

    real, capped, pct = capped_counterfactual(every_call, cap)
    out.append("")
    out.append(f"Capped-context counterfactual (cap {cap:,} tokens)")
    out.append(f"  real     {_short(real)}")
    out.append(f"  capped   {_short(capped)}   = {pct:.1f}% of real")
    out.append(
        "  Arithmetic over the calls that were made, not a forecast: a run under"
    )
    out.append("  this cap would have made different calls.")

    if agents:
        out.append("")
        out.append(
            "Subagent phases, keyed on the spawn description in <agent>.meta.json"
        )
        out.append(
            _row(
                [
                    "phase",
                    "agents",
                    "calls",
                    "calls/agent",
                    "context tokens",
                    "share",
                    "mean ctx",
                ]
            )
        )
        out.append(_row(["---"] * 7))
        sub_context = totals(sub_calls)["context"]
        by_phase: dict[str, list[Agent]] = {}
        for agent in agents:
            by_phase.setdefault(agent.phase, []).append(agent)
        rows = []
        for phase, members in by_phase.items():
            calls = [c for a in members for c in a.calls]
            agg = totals(calls)
            rows.append((agg["context"], phase, len(members), agg))
        for context, phase, count, agg in sorted(rows, reverse=True):
            share = (100.0 * context / sub_context) if sub_context else 0.0
            out.append(
                _row(
                    [
                        phase,
                        f"{count:,}",
                        f"{agg['calls']:,}",
                        f"{agg['calls'] / count:.0f}",
                        _short(context),
                        f"{share:.1f}%",
                        _short(agg["context_mean"]),
                    ]
                )
            )
        out.append(
            "  Phase is a keyword heuristic over the description; nothing in the"
        )
        out.append("  store records the phase an agent ran. calls/agent times mean")
        out.append("  ctx is the context one agent of that phase feeds the model.")

        out.append("")
        out.append(
            "Subagent harness floor by agent type, from <agent>.meta.json agentType"
        )
        out.append(
            _row(["agent type", "agents", "calls", "median floor", "context", "share"])
        )
        out.append(_row(["---"] * 6))
        for name, count, calls, median, context in agent_type_startup(agents):
            share = (100.0 * context / sub_context) if sub_context else 0.0
            out.append(
                _row(
                    [
                        name,
                        f"{count:,}",
                        f"{calls:,}",
                        f"{median:,}",
                        _short(context),
                        f"{share:.1f}%",
                    ]
                )
            )
        out.append("  median floor is the first call with the spawn prompt subtracted:")
        out.append("  what the harness gave the agent before it did anything. It is")
        out.append("  re-fed on every later call, so it multiplies by calls. The")
        out.append("  prompt comes out because otherwise the number tracks how much")
        out.append("  the parent wrote, and drifts as a live session grows. A fork")
        out.append("  reports 0: it inherits its parent's context, so it has no floor.")
        out.append("  ACROSS sessions the rows do not compare, because the always-on")
        out.append("  preamble changes size. Scope with make ze-token-economy")
        out.append("  ZE_SESSION=<id>. See ai/agents/, ai/rules/context-economy.md.")

    out.extend(_render_tools(sessions))
    return out


def _render_tools(sessions: list[Session]) -> list[str]:
    """The tool-use half of the report: round trips, result sizes, repeated reads."""
    out: list[str] = []
    every_call = [c for s in sessions for c in s.all_calls]
    every_tool = [t for s in sessions for t in s.all_tool_calls]
    if not every_tool:
        return out

    out.append("")
    out.append("Tool calls per API call")
    out.append(_row(["tool calls on the call", "API calls", "share"]))
    out.append(_row(["---"] * 3))
    for label, count, share in tool_call_distribution(every_call):
        out.append(_row([label, f"{count:,}", f"{share:.1f}%"]))
    using, alone, share = single_tool_share(every_call)
    out.append(
        f"  Of the {using:,} API calls that used a tool, {alone:,} used exactly"
        f" one ({share:.1f}%)."
    )
    out.append("  Each of those paid its whole context for a single result.")

    out.append("")
    out.append(
        f"Tool results fed back, by tool (tokens = characters / {CHARS_PER_TOKEN},"
        " an approximation)"
    )
    out.append(_row(["tool", "results", "mean tokens", "total tokens"]))
    out.append(_row(["---"] * 4))
    rows = result_sizes(every_tool)
    for name, count, mean, total in rows[:RESULT_TABLE_ROWS]:
        out.append(_row([name, f"{count:,}", _short(mean), _short(total)]))
    rest = rows[RESULT_TABLE_ROWS:]
    if rest:
        count = sum(row[1] for row in rest)
        total = sum(row[3] for row in rest)
        out.append(
            _row(
                [
                    f"{len(rest)} other tools",
                    f"{count:,}",
                    _short(total / count) if count else "0",
                    _short(total),
                ]
            )
        )

    out.append("")
    out.append("Repeated reads and the main thread's tool mix")
    thread_reads, thread_repeats = 0, 0
    session_reads, session_repeats = 0, 0
    for session in sessions:
        for thread in session.threads:
            reads, repeats = repeat_reads(thread)
            thread_reads += reads
            thread_repeats += repeats
        reads, repeats = repeat_reads(session.all_tool_calls)
        session_reads += reads
        session_repeats += repeats
    for label, reads, repeats in (
        ("one thread", thread_reads, thread_repeats),
        ("one session", session_reads, session_repeats),
    ):
        share = (100.0 * repeats / reads) if reads else 0.0
        out.append(
            f"  Read calls naming a file: {reads:,}   already read within"
            f" {label}: {repeats:,} ({share:.1f}%)"
        )
    out.append("  A thread is one context window; a session is its main thread and")
    out.append("  every agent under it, which do not share a context window.")
    main_tools = [t for s in sessions for t in s.main_tool_calls]
    bash = sum(1 for t in main_tools if t.name == "Bash")
    out.append(
        f"  Main-thread tool calls: {len(main_tools):,}   of them Bash: {bash:,}"
    )
    return out


def _bounded(name: str, low: int, high: int):
    def parse(raw: str) -> int:
        try:
            value = int(raw)
        except ValueError:
            raise argparse.ArgumentTypeError(f"{name} must be an integer, got {raw!r}")
        if value < low or value > high:
            raise argparse.ArgumentTypeError(
                f"{name} must be between {low} and {high}, got {value}"
            )
        return value

    return parse


# Options whose value legitimately begins with "-". A project slug is derived
# from an absolute path, so it ALWAYS does: "-home-thomas-Code-...". argparse
# reads such a token as another option and fails with "expected one argument",
# so the value is glued to its flag before parsing.
DASH_VALUE_OPTIONS = ("--project", "--root")


def glue_dash_values(argv: list[str]) -> list[str]:
    """Rewrite `--project -slug` as `--project=-slug`, leaving the rest alone."""
    out: list[str] = []
    index = 0
    while index < len(argv):
        token = argv[index]
        following = argv[index + 1] if index + 1 < len(argv) else None
        if (
            token in DASH_VALUE_OPTIONS
            and following is not None
            and following.startswith("-")
        ):
            out.append(f"{token}={following}")
            index += 2
            continue
        out.append(token)
        index += 1
    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Token accounting over the local Claude Code transcript store."
    )
    parser.add_argument(
        "--root",
        default=str(DEFAULT_ROOT),
        help="transcript store root (default: ~/.claude/projects)",
    )
    parser.add_argument(
        "--project",
        default=slug_for_path(REPO_ROOT),
        help="project slug under the store root (default: this checkout's slug)",
    )
    parser.add_argument(
        "--cap",
        type=_bounded("--cap", CAP_MIN, CAP_MAX),
        default=DEFAULT_CAP,
        help=f"context ceiling for the counterfactual, {CAP_MIN}-{CAP_MAX} tokens",
    )
    parser.add_argument(
        "--top",
        type=_bounded("--top", TOP_MIN, TOP_MAX),
        default=DEFAULT_TOP,
        help=f"sessions listed in the per-session table, {TOP_MIN}-{TOP_MAX}",
    )
    parser.add_argument(
        "--session",
        default="",
        help=(
            "report only sessions whose id starts with this prefix. The "
            "startup-context comparison between two agent types is only valid "
            "inside one session, because the always-on preamble changes size "
            "between them"
        ),
    )
    args = parser.parse_args(
        glue_dash_values(list(sys.argv[1:] if argv is None else argv))
    )

    store = Path(args.root) / args.project

    # An absent or empty store is a clean skip with a stated reason. Printing a
    # table of zeros here would read as "this work is free" (AC-2).
    if not store.is_dir():
        print(f"Token economy: {args.project}")
        print(f"  Looked for:  {store}")
        print("  Found no transcript store there, so there is nothing to measure.")
        print("  Transcripts are machine-local; a fresh checkout has none.")
        return 0

    sessions = [s for s in find_sessions(store) if s.all_calls]
    if args.session:
        sessions = [s for s in sessions if s.sid.startswith(args.session)]
        if not sessions:
            print(f"Token economy: {args.project}")
            print(f"  Store: {store}")
            print(f"  No session id starts with {args.session!r}.")
            return 0
    if not sessions:
        print(f"Token economy: {args.project}")
        print(f"  Looked for:  {store}")
        print("  The directory holds no transcript with a recorded API call, so")
        print("  there is nothing to measure.")
        return 0

    print(f"Token economy: {args.project}")
    print(f"  Store: {store}")
    if args.session:
        print(f"  Session: {args.session} ({len(sessions)} matched)")
    print("")
    for line in render(sessions, cap=args.cap, top=args.top):
        print(line)
    return 0


if __name__ == "__main__":
    sys.exit(main())
