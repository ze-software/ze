#!/usr/bin/env python3
"""A weekly update reaches Discord whole, or says how to finish it.

The 2026-08-03 update went out as seven of its eight messages. Discord rate
limited the last one, send() exited on the first failure, and the archive is
written only after every chunk lands. So the channel held a post with no
"Coming up" section, nothing recorded the week as posted, and a plain re-run
would have sent the seven that already landed a second time.

Both halves of that are covered here. A rate limit is transient, so the chunk
is retried; anything else stops the post and prints the --resume-from number
that finishes it. Neither path runs often, which is exactly why neither can be
left to the next operator to discover.

The third test covers where discord.sh is. The tool defaulted to one hardcoded
path in the owner's private bin, that path exists on one of the two machines,
and on the other the run exited "not found" before sending anything.

Nothing here touches Discord: subprocess.run and time.sleep are replaced, so a
"send" is a recorded call and a backoff is a recorded number.

Run: python3 scripts/zeledon/post_weekly_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import io
import pathlib
import subprocess
import tempfile
import unittest
from unittest import mock

import post_weekly

# discord.sh echoes the API's `message` field verbatim and drops retry_after,
# so this string is all send() has to recognise a rate limit by.
RATE_LIMITED = "error: You are being rate limited."


def result(returncode, stdout):
    return subprocess.CompletedProcess(
        args=[], returncode=returncode, stdout=stdout, stderr=""
    )


OK = result(0, "ok\n")


class FakeDiscord:
    """Stands in for subprocess.run, answering each call from a script of
    replies and recording the text of every send."""

    def __init__(self, replies):
        self.replies = list(replies)
        self.sent = []

    def __call__(self, argv, **kwargs):
        self.sent.append(argv[argv.index("--text") + 1])
        return self.replies.pop(0) if self.replies else OK


class SendTest(unittest.TestCase):
    def setUp(self):
        # send() refuses to run at all when discord.sh is absent, and the test
        # machine may be either of the two layouts.
        patcher = mock.patch.object(post_weekly, "DISCORD_SH", pathlib.Path(__file__))
        patcher.start()
        self.addCleanup(patcher.stop)
        self.slept = []
        patcher = mock.patch.object(post_weekly.time, "sleep", self.slept.append)
        patcher.start()
        self.addCleanup(patcher.stop)

    def send(self, chunks, replies, **kwargs):
        fake = FakeDiscord(replies)
        with mock.patch.object(post_weekly.subprocess, "run", fake):
            post_weekly.send(chunks, "ze-test", **kwargs)
        return fake

    def test_rate_limited_chunk_is_retried_until_it_lands(self):
        """The failure that broke the 2026-08-03 post: without the retry this
        raises SystemExit and chunk 3 never reaches the channel."""
        fake = self.send(
            ["one", "two", "three"],
            [OK, OK, result(1, RATE_LIMITED), result(1, RATE_LIMITED), OK],
        )

        self.assertEqual(fake.sent, ["one", "two", "three", "three", "three"])
        self.assertEqual(self.slept, post_weekly.RATE_LIMIT_BACKOFF[:2])

    def test_retries_are_bounded(self):
        """A channel that never lets up must end the run rather than wait for
        ever, and it must still name the resume point."""
        replies = [OK] + [result(1, RATE_LIMITED)] * 20

        with self.assertRaises(SystemExit) as caught:
            self.send(["one", "two"], replies)

        self.assertEqual(caught.exception.code, 1)
        self.assertEqual(len(self.slept), len(post_weekly.RATE_LIMIT_BACKOFF))

    def test_other_failures_stop_the_post_and_name_the_resume_point(self):
        """A refusal that waiting cannot fix is not retried, and the operator
        is told which chunk to restart from -- chunk 3, the one that failed,
        never the top."""
        with mock.patch("sys.stderr.write") as written:
            with self.assertRaises(SystemExit):
                self.send(
                    ["one", "two", "three"],
                    [OK, OK, result(1, "error: Missing Access")],
                )

        stderr = "".join(call.args[0] for call in written.call_args_list)
        self.assertIn("--resume-from 3", stderr)
        self.assertIn("chunks 1 to 2", stderr)
        self.assertEqual(self.slept, [], "a permission error must not be retried")

    def test_resume_from_skips_what_already_landed(self):
        """Finishing the half-sent post must send chunk 3 alone. Sending all
        three again is the duplicate this flag exists to prevent."""
        fake = self.send(["one", "two", "three"], [OK], resume_from=3)

        self.assertEqual(fake.sent, ["three"])



class ParsePostTest(unittest.TestCase):
    def test_stat_snapshot_front_matter_is_not_discord_body(self):
        """VALIDATES: snapshot metadata stays outside the normalized public body.
        PREVENTS: site-only publication state becoming Discord text."""
        expected = "Public text before.\n\nPublic text after."
        with tempfile.TemporaryDirectory() as tmp:
            post = pathlib.Path(tmp) / "2026-08-10.md"
            post.write_text(
                "---\n"
                "covers: 2026-08-10 .. 2026-08-16\n"
                "ze-stat-snapshot: true\n"
                "---\n\n"
                + expected
                + "\n"
            )

            meta, body = post_weekly.parse_post(post)

        self.assertEqual(meta["ze-stat-snapshot"], "true")
        self.assertEqual(body, expected)
        self.assertNotIn("ze-stat-snapshot", body)

    def test_legacy_body_marker_fails_closed_with_front_matter_migration(self):
        """VALIDATES: the exact legacy body marker is rejected actionably.
        PREVENTS: invalid site metadata being posted or silently discarded."""
        with tempfile.TemporaryDirectory() as tmp:
            post = pathlib.Path(tmp) / "2026-08-10.md"
            post.write_text(
                "---\ncovers: 2026-08-10 .. 2026-08-16\n---\n\n"
                "Public text before.\n\n"
                "<!-- ze-stat-snapshot: weekly update, frozen at publication -->\n\n"
                "Public text after.\n"
            )

            with mock.patch.object(
                post_weekly.sys, "stderr", new_callable=io.StringIO
            ) as stderr:
                with self.assertRaises(SystemExit) as caught:
                    post_weekly.parse_post(post)

        self.assertEqual(caught.exception.code, 1)
        self.assertIn("ze-stat-snapshot: true", stderr.getvalue())
        self.assertIn("front matter", stderr.getvalue())

    def test_terminal_nbsp_is_normalized_before_body_is_chunked(self):
        """VALIDATES: archive body and Discord chunks share boundary normalization.
        PREVENTS: a terminal NBSP surviving only in the archived body."""
        with tempfile.TemporaryDirectory() as tmp:
            post = pathlib.Path(tmp) / "2026-08-10.md"
            post.write_text(
                "---\ncovers: 2026-08-10 .. 2026-08-16\n---\n\n"
                "Public text.\u00a0\n"
            )

            _, body = post_weekly.parse_post(post)
            chunks = post_weekly.chunk(body)

        self.assertEqual(body, "Public text.")
        self.assertEqual(chunks, ["Public text."])




class MainTest(unittest.TestCase):
    def test_help_uses_canonical_weekly_post_paths(self):
        """VALIDATES: argparse help names the repository's weekly post paths.
        PREVENTS: generic blog placeholders sending operators elsewhere."""
        with mock.patch.object(
            post_weekly.sys, "argv", ["post_weekly.py", "--help"]
        ):
            with mock.patch.object(
                post_weekly.sys, "stdout", new_callable=io.StringIO
            ) as stdout:
                with self.assertRaises(SystemExit) as caught:
                    post_weekly.main()

        help_text = stdout.getvalue()
        self.assertEqual(caught.exception.code, 0)
        self.assertIn("--all website/changes/posts/", help_text)
        self.assertIn("website/changes/posts/<start>.md", help_text)
        self.assertIn(
            "python3 scripts/zeledon/post_weekly.py "
            "--all website/changes/posts/ --yes",
            help_text,
        )
        self.assertIn(
            "python3 scripts/zeledon/post_weekly.py "
            "website/changes/posts/<start>.md --yes",
            help_text,
        )
        self.assertNotIn("path/to/blog", help_text)
        self.assertNotIn("blog post", help_text)




class ProcessOneTest(unittest.TestCase):
    """--resume-from names a chunk of a specific post, so a number past the end
    of that post is a mistake the operator must see before anything is sent."""

    def test_resume_from_past_the_last_chunk_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            post = pathlib.Path(tmp) / "2026-08-03.md"
            post.write_text(
                "---\ncovers: 2026-08-03 .. 2026-08-09\n---\n\n"
                "**📅 Ze Weekly Update**\n\nOne short section.\n"
            )

            with self.assertRaises(SystemExit) as caught:
                post_weekly.process_one(
                    post,
                    "ze-test",
                    date_stamp=False,
                    confirm=True,
                    today=post_weekly.datetime.date(2026, 8, 12),
                    force=False,
                    resume_from=9,
                )

            self.assertEqual(caught.exception.code, 1)


class FindDiscordShTest(unittest.TestCase):
    def test_env_override_wins(self):
        with mock.patch.dict(
            post_weekly.os.environ, {"DISCORD_SH": "/somewhere/discord.sh"}
        ):
            self.assertEqual(
                post_weekly.find_discord_sh(), pathlib.Path("/somewhere/discord.sh")
            )

    def test_second_candidate_is_found_when_the_first_is_absent(self):
        """The machine that exited 'not found': ~/Unix/bin holds nothing and
        ~/bin holds discord.sh. A single hardcoded default fails here."""
        with tempfile.TemporaryDirectory() as tmp:
            absent = pathlib.Path(tmp) / "Unix" / "bin" / "discord.sh"
            present = pathlib.Path(tmp) / "bin" / "discord.sh"
            present.parent.mkdir(parents=True)
            present.write_text("#!/usr/bin/env bash\n")

            with mock.patch.dict(post_weekly.os.environ, clear=False) as env:
                env.pop("DISCORD_SH", None)
                with mock.patch.object(
                    post_weekly, "DISCORD_SH_CANDIDATES", (absent, present)
                ):
                    self.assertEqual(post_weekly.find_discord_sh(), present)


if __name__ == "__main__":
    unittest.main()
