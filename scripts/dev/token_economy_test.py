#!/usr/bin/env python3
"""Unit tests for token_economy.py (transcript token accounting)."""

from __future__ import annotations

import io
import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from token_economy import (  # noqa: E402
    Call,
    ToolCall,
    capped_counterfactual,
    find_sessions,
    glue_dash_values,
    histogram,
    main,
    phase_of,
    repeat_reads,
    result_sizes,
    scan,
    scan_transcript,
    single_tool_share,
    slug_for_path,
    tool_call_distribution,
    totals,
)


def usage(inp: int = 0, write: int = 0, read: int = 0, out: int = 0) -> dict:
    return {
        "input_tokens": inp,
        "cache_creation_input_tokens": write,
        "cache_read_input_tokens": read,
        "output_tokens": out,
    }


def assistant(
    mid: str,
    u: dict,
    tools: list | None = None,
    origin: str | None = None,
    own: str | None = None,
) -> dict:
    """One assistant record of a transcript, as Claude Code writes it.

    `tools` is a list of (tool_use_id, name, file_path) triples. `origin` is
    the `session_id` field naming the session that MADE the call, and `own` the
    `sessionId` field naming the transcript the record now sits in; a resumed
    session rewrites the second and leaves the first.
    """
    record = {
        "type": "assistant",
        "message": {
            "id": mid,
            "role": "assistant",
            "usage": u,
            "content": [
                {
                    "type": "tool_use",
                    "id": tid,
                    "name": name,
                    "input": {"file_path": fp},
                }
                for tid, name, fp in (tools or [])
            ],
        },
    }
    if origin is not None:
        record["session_id"] = origin
    if own is not None:
        record["sessionId"] = own
    return record


def tool_results(pairs: list) -> dict:
    """One user record carrying the results of (tool_use_id, text) pairs."""
    return {
        "type": "user",
        "message": {
            "role": "user",
            "content": [
                {"type": "tool_result", "tool_use_id": tid, "content": text}
                for tid, text in pairs
            ],
        },
    }


def write_jsonl(path: Path, records: list) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as fh:
        for rec in records:
            fh.write(json.dumps(rec) + "\n")
    return path


def store_with_session(
    tmp: str,
    slug: str = "-proj",
    session: str = "sess-1",
    main_records: list | None = None,
    agents: dict | None = None,
) -> Path:
    """Build a fixture transcript store: <root>/<slug>/<session>.jsonl (+ subagents)."""
    root = Path(tmp) / "projects"
    store = root / slug
    write_jsonl(store / f"{session}.jsonl", main_records or [])
    for name, (meta, records) in (agents or {}).items():
        write_jsonl(store / session / "subagents" / f"{name}.jsonl", records)
        (store / session / "subagents" / f"{name}.meta.json").write_text(
            json.dumps(meta), encoding="utf-8"
        )
    return root


def store_with_sessions(tmp: str, sessions: dict, slug: str = "-proj") -> Path:
    """A store of several sessions: {sid: (main_records, {agent: (meta, records)})}."""
    root = Path(tmp) / "projects"
    store = root / slug
    for sid, (main_records, agents) in sessions.items():
        write_jsonl(store / f"{sid}.jsonl", main_records)
        for name, (meta, records) in (agents or {}).items():
            write_jsonl(store / sid / "subagents" / f"{name}.jsonl", records)
            (store / sid / "subagents" / f"{name}.meta.json").write_text(
                json.dumps(meta), encoding="utf-8"
            )
    return root


class TestStoreWideDedup(unittest.TestCase):
    """One API call is charged once, however many transcripts hold its records.

    A resumed session and a forked agent both copy the earlier records into
    their own file. Dedup scoped to one file counts those calls once PER FILE:
    on this machine's store that was 487 API calls and 208,167,121 context
    tokens counted twice, with the parent's calls shown on the copy's row too.
    """

    def test_resumed_session_does_not_recount_the_original(self):
        """`session_id` names the session that made the call; `sessionId` the file.

        The resumed session is named so that it sorts FIRST, which is what makes
        this test discriminate: the copy would win any tiebreak on collection
        order, so the call lands back on the original only when `session_id` is
        read. `sessionId` alone cannot tell them apart -- a resume rewrites it to
        the copy's own id in every record it copied.
        """
        with tempfile.TemporaryDirectory() as tmp:
            shared = assistant(
                "msg_A", usage(read=500_000, out=10), origin="zzz-orig", own="zzz-orig"
            )
            copy = assistant(
                "msg_A",
                usage(read=500_000, out=10),
                origin="zzz-orig",
                own="aaa-resumed",
            )
            root = store_with_sessions(
                tmp,
                {
                    "zzz-orig": ([shared], {}),
                    "aaa-resumed": (
                        [
                            copy,
                            assistant(
                                "msg_B",
                                usage(read=7_000, out=5),
                                origin="aaa-resumed",
                                own="aaa-resumed",
                            ),
                        ],
                        {},
                    ),
                },
            )
            by_sid = {s.sid: s for s in find_sessions(root / "-proj")}
            every = [c for s in by_sid.values() for c in s.all_calls]
            self.assertEqual(len(every), 2)
            self.assertEqual(sum(c.context for c in every), 507_000)
            # The call belongs to the session that made it ...
            self.assertEqual(
                [c.context for c in by_sid["zzz-orig"].main_calls], [500_000]
            )
            # ... and the copy keeps only the call it actually made.
            self.assertEqual(
                [c.context for c in by_sid["aaa-resumed"].main_calls], [7_000]
            )

    def test_forked_agent_calls_stay_with_the_parent_agent(self):
        """A fork inherits the parent's context, so its transcript repeats it."""
        with tempfile.TemporaryDirectory() as tmp:
            inherited = assistant("msg_A", usage(read=300_000, out=9), own="s1")
            root = store_with_sessions(
                tmp,
                {
                    "s1": (
                        [],
                        {
                            "agent-parent": (
                                {
                                    "agentType": "general-purpose",
                                    "description": "Implement phase 2",
                                },
                                [inherited],
                            ),
                            "agent-child": (
                                {
                                    "agentType": "fork",
                                    "isFork": True,
                                    "parentAgentId": "parent",
                                    "description": "Implement phase 2 continued",
                                },
                                [
                                    inherited,
                                    assistant("msg_B", usage(read=11_000, out=3)),
                                ],
                            ),
                        },
                    )
                },
            )
            session = find_sessions(root / "-proj")[0]
            self.assertEqual(session.subagent_call_count, 2)
            by_name = {a.name: a for a in session.agents}
            self.assertEqual(
                [c.context for c in by_name["agent-parent"].calls], [300_000]
            )
            self.assertEqual(
                [c.context for c in by_name["agent-child"].calls], [11_000]
            )

    def test_duplicate_with_no_owner_signal_is_still_counted_once(self):
        """No recorded fact separates the two, so the first in order takes it."""
        with tempfile.TemporaryDirectory() as tmp:
            record = assistant("msg_A", usage(read=1_000, out=1))
            root = store_with_sessions(
                tmp, {"aaa": ([record], {}), "bbb": ([record], {})}
            )
            sessions = find_sessions(root / "-proj")
            every = [c for s in sessions for c in s.all_calls]
            self.assertEqual(len(every), 1)
            self.assertEqual(sum(c.context for c in every), 1_000)


class TestScan(unittest.TestCase):
    def test_dedupes_split_records(self):
        """AC-3: one API call written as three records is counted once.

        Claude Code repeats the SAME message.usage on every split record of one
        API call. Counting records instead of message ids doubles every figure.
        """
        with tempfile.TemporaryDirectory() as tmp:
            u = usage(inp=2, write=1000, read=9000, out=100)
            path = write_jsonl(
                Path(tmp) / "s.jsonl",
                [
                    assistant("msg_A", u),
                    assistant("msg_A", u),
                    assistant("msg_A", u),
                    assistant("msg_B", usage(inp=1, read=5000, out=50)),
                ],
            )
            calls = scan_transcript(path)
            self.assertEqual(len(calls), 2)
            first = calls[0]
            self.assertEqual(first.context, 2 + 1000 + 9000)
            self.assertEqual(first.output, 100)
            agg = totals(calls)
            self.assertEqual(agg["calls"], 2)
            self.assertEqual(agg["output"], 150)
            self.assertEqual(agg["cache_read"], 14000)

    def test_merges_growing_output_across_split_records(self):
        """Output GROWS across the records of one call; the context does not.

        Measured over 1,172 real message ids: growth is monotonic in every one,
        and reading the first record alone summed 149k output tokens where the
        finished figure is 1.22M.

        The FOURTH record is smaller than the third, and no real transcript
        holds one: it is here to make the test discriminate. `_merge` documents
        a field-wise MAXIMUM, and a monotonic fixture alone cannot tell that
        apart from last-record-wins -- both pass it. With the out-of-order
        record, only the documented maximum returns 264.
        """
        with tempfile.TemporaryDirectory() as tmp:
            path = write_jsonl(
                Path(tmp) / "s.jsonl",
                [
                    assistant("msg_A", usage(inp=2, write=42_151, read=11_089, out=1)),
                    assistant("msg_A", usage(inp=2, write=42_151, read=11_089, out=1)),
                    assistant(
                        "msg_A", usage(inp=2, write=42_151, read=11_089, out=264)
                    ),
                    assistant("msg_A", usage(inp=2, write=42_151, read=11_089, out=12)),
                ],
            )
            calls = scan_transcript(path)
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0].output, 264)
            self.assertEqual(calls[0].context, 2 + 42_151 + 11_089)

    def test_record_uuid_is_not_a_dedup_key(self):
        """A per-RECORD id as fallback would restore the double count."""
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "s.jsonl"
            records = []
            for uid in ("u1", "u2"):
                rec = {
                    "type": "assistant",
                    "uuid": uid,
                    "message": {"usage": usage(read=1000)},
                }
                records.append(rec)
            write_jsonl(path, records)
            self.assertEqual(scan_transcript(path), [])

    def test_skips_malformed_and_non_assistant_lines(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "s.jsonl"
            path.write_text(
                "\n".join(
                    [
                        json.dumps(assistant("msg_A", usage(read=100, out=5))),
                        "{not json at all",
                        "",
                        json.dumps({"type": "user", "message": {"content": "hi"}}),
                        json.dumps({"type": "assistant", "message": {"id": "msg_B"}}),
                        json.dumps(["a", "list", "not", "an", "object"]),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            calls = scan_transcript(path)
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0].context, 100)

    def test_counts_subagent_transcripts(self):
        """<session>/subagents/*.jsonl are attributed to their parent session."""
        with tempfile.TemporaryDirectory() as tmp:
            root = store_with_session(
                tmp,
                main_records=[assistant("m1", usage(read=1000, out=10))],
                agents={
                    "agent-aaa": (
                        {
                            "agentType": "general-purpose",
                            "description": "Review the diff",
                        },
                        [
                            assistant("a1", usage(read=2000, out=20)),
                            assistant("a1", usage(read=2000, out=20)),
                            assistant("a2", usage(read=3000, out=30)),
                        ],
                    ),
                    "agent-bbb": (
                        {
                            "agentType": "general-purpose",
                            "description": "Implement phase 2",
                        },
                        [assistant("b1", usage(read=4000, out=40))],
                    ),
                },
            )
            sessions = find_sessions(root / "-proj")
            self.assertEqual(len(sessions), 1)
            sess = sessions[0]
            self.assertEqual(sess.sid, "sess-1")
            self.assertEqual(len(sess.main_calls), 1)
            self.assertEqual(len(sess.agents), 2)
            self.assertEqual(sess.subagent_call_count, 3)
            self.assertEqual(
                sorted(a.description for a in sess.agents),
                ["Implement phase 2", "Review the diff"],
            )
            self.assertEqual(
                totals(sess.all_calls)["cache_read"], 1000 + 2000 + 3000 + 4000
            )


class TestAggregates(unittest.TestCase):
    def test_context_histogram_buckets(self):
        """Spend lands in the bucket of the call that spent it, not an average."""
        calls = [
            Call(input=0, cache_write=0, cache_read=10_000, output=1),
            Call(input=0, cache_write=0, cache_read=120_000, output=1),
            Call(input=0, cache_write=0, cache_read=150_000, output=1),
            Call(input=0, cache_write=0, cache_read=900_000, output=1),
        ]
        rows = histogram(calls)
        by_label = {row.label: row for row in rows}
        self.assertEqual(by_label["0-50k"].calls, 1)
        self.assertEqual(by_label["0-50k"].context, 10_000)
        self.assertEqual(by_label["100k-200k"].calls, 2)
        self.assertEqual(by_label["100k-200k"].context, 270_000)
        self.assertEqual(by_label["800k-1M"].calls, 1)
        self.assertEqual(sum(row.calls for row in rows), 4)
        self.assertEqual(sum(row.context for row in rows), 1_180_000)
        # Shares are of the context total, so they sum to 100%.
        self.assertAlmostEqual(sum(row.share for row in rows), 100.0, places=6)

    def test_capped_counterfactual(self):
        """The capped figure is the sum of per-call minimums, nothing else."""
        calls = [
            Call(input=0, cache_write=0, cache_read=50_000, output=1),
            Call(input=0, cache_write=0, cache_read=400_000, output=1),
            Call(input=0, cache_write=0, cache_read=900_000, output=1),
        ]
        cap = 150_000
        real, capped, pct = capped_counterfactual(calls, cap)
        self.assertEqual(real, 1_350_000)
        self.assertEqual(capped, sum(min(c.context, cap) for c in calls))
        self.assertEqual(capped, 50_000 + 150_000 + 150_000)
        self.assertAlmostEqual(pct, 100.0 * capped / real)

    def test_capped_counterfactual_is_empty_safe(self):
        real, capped, pct = capped_counterfactual([], 1000)
        self.assertEqual((real, capped), (0, 0))
        self.assertEqual(pct, 0.0)

    def test_phase_of_reads_the_spawn_description(self):
        self.assertEqual(phase_of("Review the isis diff", "general-purpose"), "review")
        self.assertEqual(
            phase_of("Audit spec against code", "general-purpose"), "audit"
        )
        self.assertEqual(
            phase_of("Debug the failing gate", "general-purpose"), "fix/debug"
        )
        self.assertEqual(phase_of("Implement phase 2", "general-purpose"), "implement")
        self.assertEqual(phase_of("", "general-purpose"), "unclassified")


class TestToolFigures(unittest.TestCase):
    """The figures a context-economy rule cites, each printed verbatim."""

    def test_tool_uses_are_deduped_with_their_api_call(self):
        """A split call repeats its tool_use blocks; the toolu id counts them once."""
        with tempfile.TemporaryDirectory() as tmp:
            tools = [("t1", "Read", "/a.go"), ("t2", "Bash", "")]
            path = write_jsonl(
                Path(tmp) / "s.jsonl",
                [
                    assistant("msg_A", usage(read=10), tools=tools),
                    assistant("msg_A", usage(read=10), tools=tools),
                    tool_results([("t1", "x" * 36), ("t2", "y" * 18)]),
                ],
            )
            found = scan(path)
            self.assertEqual(found.calls["msg_A"].tools, 2)
            by_name = {t.name: t for t in found.tools["msg_A"]}
            self.assertEqual(by_name["Read"].result_chars, 36)
            self.assertEqual(by_name["Read"].file_path, "/a.go")
            self.assertEqual(by_name["Bash"].result_chars, 18)

    def test_tool_call_distribution_and_single_tool_share(self):
        calls = [
            Call(cache_read=1, tools=0),
            Call(cache_read=1, tools=1),
            Call(cache_read=1, tools=1),
            Call(cache_read=1, tools=1),
            Call(cache_read=1, tools=2),
            Call(cache_read=1, tools=9),
        ]
        rows = {
            label: (count, share)
            for label, count, share in tool_call_distribution(calls)
        }
        self.assertEqual(rows["0"][0], 1)
        self.assertEqual(rows["1"][0], 3)
        self.assertEqual(rows["2"][0], 1)
        self.assertEqual(rows["3"][0], 0)
        # 9 tool calls is one row, not nine: the last bucket is open.
        self.assertEqual(rows["4+"][0], 1)
        self.assertEqual(sum(count for count, _ in rows.values()), len(calls))
        self.assertAlmostEqual(sum(share for _, share in rows.values()), 100.0)
        using, alone, share = single_tool_share(calls)
        # Of the FIVE calls that used a tool, three used exactly one.
        self.assertEqual((using, alone), (5, 3))
        self.assertAlmostEqual(share, 60.0)

    def test_result_sizes_are_characters_over_the_stated_divisor(self):
        tools = [
            ToolCall(name="Read", result_chars=36),
            ToolCall(name="Read", result_chars=0),
            ToolCall(name="Bash", result_chars=180),
        ]
        rows = {
            name: row for name, *row in [(r[0], *r[1:]) for r in result_sizes(tools)]
        }
        # Bash carries the most tokens, so it sorts first.
        self.assertEqual(result_sizes(tools)[0][0], "Bash")
        self.assertEqual(rows["Bash"][0], 1)
        self.assertAlmostEqual(rows["Bash"][2], 180 / 3.6)
        # An unanswered call is a real zero and stays in the mean's denominator.
        self.assertEqual(rows["Read"][0], 2)
        self.assertAlmostEqual(rows["Read"][1], (36 / 2) / 3.6)

    def test_repeat_reads_counts_only_files_read_twice(self):
        group = [
            ToolCall(name="Read", file_path="/a.go"),
            ToolCall(name="Read", file_path="/b.go"),
            ToolCall(name="Read", file_path="/a.go"),
            ToolCall(name="Read", file_path="/a.go"),
            ToolCall(name="Edit", file_path="/a.go"),
            ToolCall(name="Read", file_path=""),
        ]
        reads, repeats = repeat_reads(group)
        # Four Reads name a file; /a.go is read three times, so two are repeats.
        self.assertEqual((reads, repeats), (4, 2))
        # Reversing the order cannot change the count.
        self.assertEqual(repeat_reads(list(reversed(group))), (4, 2))
        self.assertEqual(repeat_reads([]), (0, 0))

    def test_report_carries_the_tool_figures(self):
        """Every new figure is printed, and the approximation is named beside it."""
        with tempfile.TemporaryDirectory() as tmp:
            root = store_with_sessions(
                tmp,
                {
                    "s1": (
                        [
                            assistant(
                                "m1",
                                usage(read=100_000, out=3),
                                tools=[("t1", "Bash", "")],
                            ),
                            assistant(
                                "m2",
                                usage(read=120_000, out=3),
                                tools=[("t2", "Read", "/a.go")],
                            ),
                            assistant(
                                "m3",
                                usage(read=130_000, out=3),
                                tools=[("t3", "Read", "/a.go")],
                            ),
                            tool_results(
                                [("t1", "b" * 36), ("t2", "r" * 72), ("t3", "r" * 72)]
                            ),
                        ],
                        {
                            "agent-aaa": (
                                {
                                    "agentType": "general-purpose",
                                    "description": "Implement phase 2",
                                },
                                [assistant("a1", usage(read=60_000, out=2))],
                            )
                        },
                    )
                },
            )
            buf = io.StringIO()
            with redirect_stdout(buf):
                self.assertEqual(main(["--root", str(root), "--project", "-proj"]), 0)
            out = buf.getvalue()
            for section in (
                "Context histogram, main thread against subagents",
                "Tool calls per API call",
                "Tool results fed back, by tool",
                "Repeated reads and the main thread's tool mix",
                "calls/agent",
            ):
                self.assertIn(section, out)
            self.assertIn("tokens = characters / 3.6, an approximation", out)
            self.assertIn("used exactly one (100.0%)", out)
            self.assertIn("already read within one thread: 1 (50.0%)", out)
            self.assertIn("Main-thread tool calls: 3   of them Bash: 1", out)


class TestMain(unittest.TestCase):
    def test_absent_store_is_explicit(self):
        """AC-2: the missing path and the reason, exit 0, and no zero totals."""
        with tempfile.TemporaryDirectory() as tmp:
            missing = Path(tmp) / "projects"
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(["--root", str(missing), "--project", "-nope"])
            out = buf.getvalue()
            self.assertEqual(code, 0)
            self.assertIn(str(missing / "-nope"), out)
            self.assertIn("no transcript store", out.lower())
            for banned in ("API calls: 0", "0 sessions", "Totals"):
                self.assertNotIn(banned, out)
            self.assertNotIn("0.0%", out)

    def test_empty_store_is_explicit(self):
        """A store directory holding no transcripts reports the same way."""
        with tempfile.TemporaryDirectory() as tmp:
            store = Path(tmp) / "projects" / "-proj"
            store.mkdir(parents=True)
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(["--root", str(store.parent), "--project", "-proj"])
            out = buf.getvalue()
            self.assertEqual(code, 0)
            self.assertIn(str(store), out)
            self.assertNotIn("Totals", out)

    def test_main_reports_from_fixture(self):
        """AC-1: every section of the report is present, and exit is 0."""
        with tempfile.TemporaryDirectory() as tmp:
            root = store_with_session(
                tmp,
                main_records=[
                    assistant("m1", usage(inp=5, write=20_000, read=100_000, out=300)),
                    assistant("m1", usage(inp=5, write=20_000, read=100_000, out=300)),
                    assistant("m2", usage(inp=5, write=0, read=700_000, out=400)),
                ],
                agents={
                    "agent-aaa": (
                        {
                            "agentType": "general-purpose",
                            "description": "Review the diff",
                        },
                        [
                            assistant(
                                "a1", usage(inp=2, write=5_000, read=60_000, out=90)
                            )
                        ],
                    ),
                },
            )
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(
                    ["--root", str(root), "--project", "-proj", "--cap", "150000"]
                )
            out = buf.getvalue()
            self.assertEqual(code, 0)
            for section in (
                "Per session",
                "Totals",
                "Context histogram",
                "Capped-context counterfactual",
                "Subagent phases",
            ):
                self.assertIn(section, out)
            self.assertIn("sess-1", out)
            self.assertIn("review", out)
            # Three API calls: two main (m1 deduped from two records) and one agent.
            self.assertIn("API calls: 3", out)
            self.assertIn("main 2", out)
            self.assertIn("subagent 1", out)
            self.assertIn("cap 150,000", out)
            # No pricing, ever.
            for money in ("$", "USD", "cost", "price"):
                self.assertNotIn(money, out.lower() if money.isalpha() else out)

    def test_zero_context_store_still_reports(self):
        """Calls whose every usage field is zero must not divide by zero."""
        with tempfile.TemporaryDirectory() as tmp:
            root = store_with_session(
                tmp,
                main_records=[assistant("m1", usage())],
                agents={
                    "agent-aaa": (
                        {"agentType": "general-purpose", "description": "Review it"},
                        [assistant("a1", usage())],
                    )
                },
            )
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(["--root", str(root), "--project", "-proj"])
            self.assertEqual(code, 0)
            self.assertIn("API calls: 2", buf.getvalue())

    def test_real_slug_is_accepted_as_a_project_argument(self):
        """A real slug starts with '-', which argparse reads as another option."""
        slug = slug_for_path("/home/thomas/Code/github.com/ze-software/ze/main")
        self.assertEqual(
            glue_dash_values(["--project", slug, "--cap", "10"]),
            [f"--project={slug}", "--cap", "10"],
        )
        with tempfile.TemporaryDirectory() as tmp:
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(["--root", str(Path(tmp) / "none"), "--project", slug])
            self.assertEqual(code, 0)
            self.assertIn(slug, buf.getvalue())

    def test_slug_for_path_matches_the_store_naming(self):
        self.assertEqual(
            slug_for_path("/home/thomas/Code/github.com/ze-software/ze/main"),
            "-home-thomas-Code-github-com-ze-software-ze-main",
        )


class TestBoundaries(unittest.TestCase):
    """Boundary tests: --cap 1..1000000, --top 1..1000."""

    def run_main(self, args: list) -> int:
        with tempfile.TemporaryDirectory() as tmp:
            buf = io.StringIO()
            with redirect_stdout(buf):
                return main(
                    ["--root", str(Path(tmp) / "none"), "--project", "-x"] + args
                )

    def assert_rejected(self, args: list):
        with self.assertRaises(SystemExit) as raised:
            with redirect_stderr(io.StringIO()):
                self.run_main(args)
        self.assertEqual(raised.exception.code, 2)

    def test_cap_last_valid(self):
        self.assertEqual(self.run_main(["--cap", "1000000"]), 0)
        self.assertEqual(self.run_main(["--cap", "1"]), 0)

    def test_cap_invalid_below(self):
        self.assert_rejected(["--cap", "0"])

    def test_cap_invalid_above(self):
        self.assert_rejected(["--cap", "1000001"])

    def test_top_last_valid(self):
        self.assertEqual(self.run_main(["--top", "1000"]), 0)
        self.assertEqual(self.run_main(["--top", "1"]), 0)

    def test_top_invalid_below(self):
        self.assert_rejected(["--top", "0"])

    def test_top_invalid_above(self):
        self.assert_rejected(["--top", "1001"])


if __name__ == "__main__":
    unittest.main()
