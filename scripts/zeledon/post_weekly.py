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
import subprocess
import sys
import time

HERE = pathlib.Path(__file__).resolve().parent
WEEKLY_DIR = HERE / "weekly"
# discord.sh carries the bot token, so it stays in the private ~/Unix repo
# rather than this public one. Override the location with the DISCORD_SH env var.
DISCORD_SH = pathlib.Path(
    os.environ.get("DISCORD_SH", pathlib.Path.home() / "Unix" / "bin" / "discord.sh")
)

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


def send(chunks, channel):
    """Send every chunk of one post back-to-back, with no delay between
    chunks: the parts of a single weekly update are meant to read as one
    grouped block. The gap that breaks Discord's message grouping is
    inserted between distinct posts (see the --all loop in main), not
    between the parts of one post."""
    if not DISCORD_SH.exists():
        print("error: %s not found" % DISCORD_SH, file=sys.stderr)
        sys.exit(1)
    for i, c in enumerate(chunks):
        result = subprocess.run(
            ["bash", str(DISCORD_SH), "--channel", channel, "--text", c],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0 or "ok" not in result.stdout:
            print(
                "error: send failed on chunk %d/%d: %s %s"
                % (i + 1, len(chunks), result.stdout, result.stderr),
                file=sys.stderr,
            )
            sys.exit(1)
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


def process_one(path, channel, date_stamp, confirm, today, force):
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

    print(
        "=== %s (covers %s) -- date-stamped: %s ==="
        % (date, meta["covers"], date_stamp)
    )
    print("%d message(s):" % len(chunks))
    for i, c in enumerate(chunks):
        print("--- chunk %d/%d (%d chars) ---" % (i + 1, len(chunks), len(c)))
        print(c)
        print()

    if not confirm:
        print("(dry run -- pass --yes to actually post)")
        return False

    send(chunks, channel)
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
    args = parser.parse_args()

    if not args.source and not args.all:
        parser.error("give a source file or --all <dir>")

    today = datetime.date.today()

    if args.source:
        process_one(
            args.source, args.channel, args.date_stamp, args.yes, today, args.force
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
