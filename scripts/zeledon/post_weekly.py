#!/usr/bin/env python3
"""Post Zeledon weekly updates to Discord ze-news, consistently every time.

Usage:
    python3 post_weekly.py --all <path/to/blog/posts/dir> --yes
    python3 post_weekly.py <path/to/blog/post.md> --yes

The normal way to run this is the --all form, regularly (weekly, or
whenever you remember). It scans the posts directory, skips any week
that's already archived, skips any week that hasn't fully ended yet, and
posts everything else in chronological order -- one missed week or ten,
same command. There's no separate "catch up" mode: falling behind and
catching up are the same operation.

Reads gh-pages blog/posts/<start-date>.md files (front matter: `covers:
<start> .. <end>`), splits the Zeledon-voice body into Discord messages
under Discord's 2000-char limit (splitting only at "**emoji Header**"
section boundaries, never mid-section), and posts each chunk in order via
the discord.sh CLI ($DISCORD_SH, default ~/Unix/bin/discord.sh).

A week is only posted once it has fully ended (covers' end date is before
today) -- posting mid-week would describe a week that hasn't finished
shipping yet. Single-file mode refuses an incomplete week too, unless
--force.

Whether the header line gets "-- Week of <date>" inserted is decided
automatically: a live, in-time post's own Discord timestamp already says
which week it is, but a post landing more than a week after its content's
week ended needs the date stated in the text, or the timestamp lies about
when the work happened. Override with --date-stamp / --no-date-stamp.

Without --yes this only prints what would be sent (message count, char
counts, full text) and does not touch Discord -- review the plan first.

A chunk Discord refuses for rate limiting is retried, because that refusal
is transient. Any other failure stops the post and prints the --resume-from
number that finishes it, since re-running from the top would repeat every
chunk that already landed in the channel.

After a successful post, writes weekly/<covers-start-date>-weekly.md
(posted date, channel, covers, date_stamped flag, and the exact text sent
-- not the source file's text, so the archive is a byte-accurate record of
what actually landed in the channel). Already-archived weeks are always
skipped, so a batch run is safe to re-run if interrupted partway.
"""

import argparse
import datetime
import os
import pathlib
import re
import shutil
import subprocess
import sys
import time

HERE = pathlib.Path(__file__).resolve().parent
WEEKLY_DIR = HERE / "weekly"
# discord.sh carries the bot token, so it stays in the owner's private bin
# rather than this public repo. Which private bin differs per machine, and a
# single hardcoded default sent one weekly update to a "not found" exit, so
# the known locations are searched in turn and PATH answers the rest.
# DISCORD_SH overrides all of it.
DISCORD_SH_CANDIDATES = (
    pathlib.Path.home() / "Unix" / "bin" / "discord.sh",
    pathlib.Path.home() / "bin" / "discord.sh",
)


def find_discord_sh():
    override = os.environ.get("DISCORD_SH")
    if override:
        return pathlib.Path(override)
    for candidate in DISCORD_SH_CANDIDATES:
        if candidate.exists():
            return candidate
    on_path = shutil.which("discord.sh")
    if on_path:
        return pathlib.Path(on_path)
    return DISCORD_SH_CANDIDATES[0]


DISCORD_SH = find_discord_sh()

LIMIT = 1900  # Discord's hard cap is 2000; leave margin
# Discord's client groups consecutive same-author messages (no repeated
# avatar/username, tighter spacing) when they land close together. That
# grouping is exactly what we want WITHIN a single weekly update: its
# chunks should read as one block. It is NOT what we want between two
# distinct weekly posts in an --all run -- without a real gap, separate
# weeks collapse into one wall of text with only the first showing who
# posted it. So this delay is inserted only BETWEEN posts, never between
# the chunks of one post. 65s reliably breaks grouping with margin over
# the ~1 minute window.
SEND_DELAY = 65.0
STALE_AFTER_DAYS = 7  # older than this since week-end -> stamp the date
# Seconds to wait before each retry of a chunk Discord refused for rate
# limiting. One entry per retry, so the list length is the retry count.
RATE_LIMIT_BACKOFF = [5, 15, 30, 60]

FRONT_MATTER_RE = re.compile(r"^---\n(.*?)\n---\n(.*)$", re.DOTALL)
SECTION_SPLIT_RE = re.compile(r"(?=^\*\*[^\n]+\*\*$)", re.MULTILINE)
HEADER_LINE_RE = re.compile(r"^\*\*📅 Ze Weekly Update\*\*$", re.MULTILINE)


def parse_post(path):
    text = path.read_text()
    m = FRONT_MATTER_RE.match(text)
    if not m:
        print("error: %s has no front matter" % path, file=sys.stderr)
        sys.exit(1)
    meta = {}
    for line in m.group(1).splitlines():
        key, _, value = line.partition(":")
        meta[key.strip()] = value.strip()
    return meta, m.group(2).strip()


def start_date(covers):
    return covers.split("..")[0].strip().split(" ")[0]


def end_date(covers):
    end_part = covers.split("..")[1].strip().split(" ")[0]
    return datetime.date.fromisoformat(end_part)


def days_since_week_end(covers, today):
    return (today - end_date(covers)).days


def apply_date_stamp(body, date):
    new_header = "**📅 Ze Weekly Update -- Week of %s**" % date
    if HEADER_LINE_RE.search(body):
        return HEADER_LINE_RE.sub(new_header, body, count=1)
    print(
        "warning: no '**📅 Ze Weekly Update**' header line found, "
        "posting body unchanged (no date inserted)",
        file=sys.stderr,
    )
    return body


def chunk(body):
    parts = [p.strip() for p in SECTION_SPLIT_RE.split(body) if p.strip()]
    chunks = []
    cur = ""
    for part in parts:
        candidate = (cur + "\n\n" + part) if cur else part
        if len(candidate) <= LIMIT:
            cur = candidate
        else:
            if cur:
                chunks.append(cur)
            if len(part) <= LIMIT:
                cur = part
            else:
                # a single section is itself too long: hard-split on
                # paragraph boundaries as a last resort
                cur = ""
                for para in part.split("\n\n"):
                    piece = (cur + "\n\n" + para) if cur else para
                    if len(piece) <= LIMIT:
                        cur = piece
                    else:
                        if cur:
                            chunks.append(cur)
                        cur = para
    if cur:
        chunks.append(cur)
    return chunks


def send(chunks, channel, resume_from=1):
    """Send every chunk of one post back-to-back, with no delay between
    chunks: the parts of a single weekly update are meant to read as one
    grouped block. The gap that breaks Discord's message grouping is
    inserted between distinct posts (see the --all loop in main), not
    between the parts of one post.

    Sending them back-to-back is what runs into Discord's per-channel
    message rate limit, so a chunk refused that way is retried on the
    schedule in RATE_LIMIT_BACKOFF rather than killing the post. The
    limit is transient by definition: the only correct answer is to wait
    and send the same chunk again.

    discord.sh reports the API's `message` field and drops `retry_after`,
    so the wait cannot be read from the refusal and the schedule is fixed.

    resume_from is 1-based and names the first chunk to send, so a post
    left half-delivered by an unrecoverable failure can be finished
    without repeating what already landed in the channel."""
    if not DISCORD_SH.exists():
        print("error: %s not found" % DISCORD_SH, file=sys.stderr)
        sys.exit(1)
    for i, c in enumerate(chunks):
        if i + 1 < resume_from:
            print("skip chunk %d/%d -- already sent" % (i + 1, len(chunks)))
            continue
        for attempt, wait in enumerate(RATE_LIMIT_BACKOFF + [None]):
            result = subprocess.run(
                ["bash", str(DISCORD_SH), "--channel", channel, "--text", c],
                capture_output=True,
                text=True,
            )
            if result.returncode == 0 and "ok" in result.stdout:
                break
            report = "%s %s" % (result.stdout.strip(), result.stderr.strip())
            if wait is None or "rate limited" not in report.lower():
                print(
                    "error: send failed on chunk %d/%d: %s"
                    % (i + 1, len(chunks), report.strip()),
                    file=sys.stderr,
                )
                print(
                    "chunks 1 to %d are in %s. Finish this post with "
                    "--resume-from %d (a fresh run would repeat them)."
                    % (i, channel, i + 1),
                    file=sys.stderr,
                )
                sys.exit(1)
            print(
                "rate limited on chunk %d/%d, retrying in %ds (attempt %d/%d)"
                % (i + 1, len(chunks), wait, attempt + 1, len(RATE_LIMIT_BACKOFF))
            )
            time.sleep(wait)
        print("sent chunk %d/%d (%d chars)" % (i + 1, len(chunks), len(c)))


def write_archive(date, meta, body, channel, date_stamped):
    WEEKLY_DIR.mkdir(parents=True, exist_ok=True)
    dest = WEEKLY_DIR / ("%s-weekly.md" % date)
    front = ["posted: %s" % time.strftime("%Y-%m-%d"), "channel: %s" % channel]
    front.append("covers: %s" % meta["covers"])
    if date_stamped:
        front.append("backfilled: true")
    text = "---\n%s\n---\n\n%s\n" % ("\n".join(front), body)
    dest.write_text(text)
    print("archived -> %s" % dest)


def process_one(path, channel, date_stamp, confirm, today, force, resume_from=1):
    meta, body = parse_post(path)
    date = start_date(meta["covers"])
    remaining_days = -days_since_week_end(meta["covers"], today)

    if remaining_days > 0 and not force:
        print(
            "refuse to post %s: week doesn't end until %s (%d day(s) away). "
            "Use --force to override."
            % (path.name, meta["covers"].split("..")[1].strip(), remaining_days),
            file=sys.stderr,
        )
        sys.exit(1)

    if date_stamp is None:
        date_stamp = days_since_week_end(meta["covers"], today) > STALE_AFTER_DAYS
    if date_stamp:
        body = apply_date_stamp(body, date)

    chunks = chunk(body)

    if resume_from > len(chunks):
        print(
            "error: --resume-from %d but %s splits into %d chunk(s)"
            % (resume_from, path.name, len(chunks)),
            file=sys.stderr,
        )
        sys.exit(1)

    print(
        "=== %s (covers %s) -- date-stamped: %s ==="
        % (date, meta["covers"], date_stamp)
    )
    print("%d message(s):" % len(chunks))
    for i, c in enumerate(chunks):
        state = " -- already sent, skipping" if i + 1 < resume_from else ""
        print("--- chunk %d/%d (%d chars)%s ---" % (i + 1, len(chunks), len(c), state))
        print(c)
        print()

    if not confirm:
        print("(dry run -- pass --yes to actually post)")
        return False

    send(chunks, channel, resume_from)
    write_archive(date, meta, body, channel, date_stamp)
    return True


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "source", nargs="?", type=pathlib.Path, help="one blog post .md file"
    )
    parser.add_argument(
        "--all",
        type=pathlib.Path,
        help="process every eligible, unarchived .md file in this directory, "
        "chronologically -- the normal way to run this tool",
    )
    parser.add_argument(
        "--date-stamp",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="force 'Week of <date>' into the header on/off; default is "
        "automatic (stamped if posted more than %d days after the week ended)"
        % STALE_AFTER_DAYS,
    )
    parser.add_argument("--channel", default="ze-news", choices=["ze-news", "ze-test"])
    parser.add_argument(
        "--yes",
        action="store_true",
        help="actually post; without this, only prints the plan",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="allow posting a week that hasn't fully ended yet (single-file mode only)",
    )
    parser.add_argument(
        "--resume-from",
        type=int,
        default=1,
        metavar="N",
        help="finish a post left half-sent: start at chunk N and treat the "
        "earlier ones as already in the channel (single-file mode only)",
    )
    args = parser.parse_args()

    if not args.source and not args.all:
        parser.error("give a source file or --all <dir>")

    if args.resume_from < 1:
        parser.error("--resume-from takes a 1-based chunk number")

    if args.resume_from > 1 and not args.source:
        parser.error(
            "--resume-from names chunks of one post, so it needs a source file"
        )

    today = datetime.date.today()

    if args.source:
        process_one(
            args.source,
            args.channel,
            args.date_stamp,
            args.yes,
            today,
            args.force,
            args.resume_from,
        )
        return 0

    files = sorted(args.all.glob("*.md"))
    if not files:
        print("error: no .md files in %s" % args.all, file=sys.stderr)
        return 1

    posted_any = False
    for f in files:
        meta, _ = parse_post(f)
        date = start_date(meta["covers"])
        archive_path = WEEKLY_DIR / ("%s-weekly.md" % date)
        if archive_path.exists():
            print("skip %s -- already archived at %s" % (f.name, archive_path))
            continue
        if days_since_week_end(meta["covers"], today) < 0:
            print("skip %s -- week not over yet (covers %s)" % (f.name, meta["covers"]))
            continue
        # Separate consecutive posts so Discord shows them as distinct
        # weekly updates rather than one grouped block. The gap goes
        # between posts, never between the chunks of a single post.
        if posted_any:
            time.sleep(SEND_DELAY)
        if process_one(f, args.channel, args.date_stamp, args.yes, today, force=False):
            posted_any = True

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
