#!/usr/bin/env python3
"""A session's binaries live in that session's own dated directory.

make (mk/helper-session.mk) decides where `make ze-build` writes; Go
(internal/test/sessionpath) decides where the test runner looks. Both must
resolve the SAME directory: tmp/session/<YYYY-MM-DD>-<sid>/bin. When they
disagree, a session builds into one directory and execs from another, and
nothing is red -- the runner simply picks up a sibling session's binary. Three
independent derivations of the session id drifted for weeks behind a prose
"MUST stay identical" comment, which is why the agreement is a test here.

The directory is LOOKED UP, never recomputed: a consumer takes the single
directory matching tmp/session/????-??-??-<sid>, and creates
tmp/session/<today>-<sid> only on a miss. A consumer that recomputed the name
from today's date would move a session's directory at midnight and orphan the
binaries the session is running.

This file replaces scripts/dev/session_bin_suffix_test.py, which pinned the
NAME suffix bin/ze-<sid>. Three properties of that gate survive the change and
are carried over: an unsafe id falls back to the shared bin/, the charset check
never executes the id as shell, and off-session output does not move. The
fourth, a guard against an id that reproduces another binary's name, is retired
by the design: a directory gives each session its own namespace, so
ZE_SESSION_ID=test can no longer write over bin/ze-test.

Run: python3 scripts/dev/session_bin_dir_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import concurrent.futures
import datetime
import os
import pathlib
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]

# The one root for per-session state (spec-session-bin-directory, 2026-08-03).
SESSION_ROOT = "tmp/session"

# Mirrors sidSafe in internal/test/sessionpath/sessionpath.go, itself a mirror of
# _SID_SAFE_RE in .claude/hooks/lib/session_id.py.
GO_SID_SAFE = re.compile(r"\A[A-Za-z0-9._-]+\Z")

# A resolved session bin path: tmp/session/<YYYY-MM-DD>-<sid>/bin/<name>.
DATED_BIN = re.compile(
    r"\Atmp/session/(\d{4}-\d{2}-\d{2})-(?P<sid>[A-Za-z0-9._-]+)/bin\Z"
)

# The probe program below imports internal/test/sessionpath, so it must live
# INSIDE this module: an internal package is importable only under
# github.com/ze-software/ze. bin/ is gitignored, and a directory whose name
# starts with "_" is invisible to the ./... pattern, so the probe reaches no
# gate and no commit while it exists.
PROBE_DIR = ROOT / "bin" / f"_sessionpath-probe-{os.getpid()}"
PROBE_SRC = """package main

import (
	"fmt"

	"github.com/ze-software/ze/internal/test/sessionpath"
)

// Prints the directory the test runner builds and looks for binaries in,
// relative to the checkout root.
func main() { fmt.Println(sessionpath.BinDir(".")) }
"""


def setUpModule():
    PROBE_DIR.mkdir(parents=True, exist_ok=True)
    (PROBE_DIR / "main.go").write_text(PROBE_SRC, encoding="utf-8")


def tearDownModule():
    shutil.rmtree(PROBE_DIR, ignore_errors=True)


def _clean_env(session_id=None):
    """A child environment carrying exactly the session id under test."""
    env = dict(os.environ)
    env.pop("CLAUDE_CODE_SESSION_ID", None)
    env.pop("CLAUDE_CODE_FORK_SUBAGENT", None)
    env.pop("ZE_SESSION_ID", None)
    if session_id is not None:
        env["ZE_SESSION_ID"] = session_id
    # Makefile exports GOCACHE into every recipe; a direct `go run` from python
    # gets no such export and would compile against the user's default cache.
    cache = ROOT / "cache" / "go-cache"
    if (ROOT / "cache").exists():
        env.setdefault("GOCACHE", str(cache))
    return env


def ze_path(session_id=None):
    """`make ze-session-binary-path` with ZE_SESSION_ID set, or with no session at all.

    The id goes on the command line because that is the case with the sharpest
    edge: a command-line assignment outranks every makefile assignment, so a
    validator that reads only the environment would let `make ze-build
    ZE_SESSION_ID=../../etc` reach the -o path.
    """
    cmd = ["make", "ze-session-binary-path"]
    if session_id is not None:
        cmd.append(f"ZE_SESSION_ID={session_id}")
    return subprocess.run(
        cmd,
        cwd=str(ROOT),
        env=_clean_env(),
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def go_bin_dir(session_id=None):
    """internal/test/sessionpath.BinDir, relative to the checkout root."""
    return subprocess.run(
        ["go", "run", f"./bin/{PROBE_DIR.name}/main.go"],
        cwd=str(ROOT),
        env=_clean_env(session_id),
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def shell_session_dir(session_id):
    """`_session_dir` from .claude/hooks/lib/session-dir.sh, the SHELL copy.

    It answers the session DIRECTORY, one level above the bin directory make and
    Go answer, so the caller compares it against the parent.
    """
    return subprocess.run(
        [
            "bash",
            "-c",
            '. .claude/hooks/lib/session-dir.sh; _session_dir "$1"',
            "_",
            session_id,
        ],
        cwd=str(ROOT),
        env=_clean_env(),
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def python_session_dir(session_id):
    """`session_dir` from .claude/hooks/pretool-writeedit.py, the PYTHON copy.

    Imported by path rather than re-implemented, so a rule change in the hook
    reaches this test with no edit here.
    """
    return subprocess.run(
        [
            "python3",
            "-c",
            "import importlib.util,sys;"
            "s=importlib.util.spec_from_file_location('we',"
            "'.claude/hooks/pretool-writeedit.py');"
            "m=importlib.util.module_from_spec(s);s.loader.exec_module(m);"
            "print(m.session_dir(sys.argv[1]))",
            session_id,
        ],
        cwd=str(ROOT),
        env=_clean_env(),
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def go_accepts(sid):
    """What internal/test/sessionpath.ID() would do with sid."""
    return bool(GO_SID_SAFE.match(sid)) and sid not in (".", "..")


def today():
    return datetime.date.today().isoformat()


def ze_path_dated(sid):
    """`make ze-session-binary-path`, with the dates that could legitimately name the directory.

    A run that straddles midnight would otherwise fail on a date that was
    correct when the call was made.
    """
    before = today()
    out = ze_path(sid)
    return out, {before, today()}


class SessionDirCase(unittest.TestCase):
    """A test case that owns its probe ids and removes their directories."""

    def setUp(self):
        self.sids = []

    def tearDown(self):
        for sid in self.sids:
            for path in (ROOT / SESSION_ROOT).glob(f"????-??-??-{sid}"):
                shutil.rmtree(path, ignore_errors=True)

    def sid(self, tag):
        """A probe id no live session can own, tracked for cleanup."""
        sid = f"zesbd-{os.getpid()}-{tag}"
        self.sids.append(sid)
        return sid

    def dated_dirs(self, sid):
        return sorted(p.name for p in (ROOT / SESSION_ROOT).glob(f"????-??-??-{sid}"))


class TestZePathIsSessionDirectory(SessionDirCase):
    """AC-1: on-session, the binary is a BARE name in the session directory."""

    def test_path_is_the_dated_session_directory(self):
        for tag in ("plain", "uuid-shaped"):
            sid = self.sid(tag)
            with self.subTest(sid=sid):
                got, dates = ze_path_dated(sid)
                want = {f"{SESSION_ROOT}/{d}-{sid}/bin/ze" for d in dates}
                self.assertIn(
                    got,
                    want,
                    f"ZE_SESSION_ID={sid} must write into this session's own "
                    f"directory, not into the shared bin/",
                )

    def test_the_binary_keeps_its_bare_name(self):
        """The directory carries the id, so the file name must not.

        A suffixed name breaks argv[0] personality dispatch (cmd/ze/dispatch.go
        binarySuffixRoot reads the segment after the last '-') and makes a .ci
        test unable to exec `ze` by bare name off one PATH entry.
        """
        sid = self.sid("bare")
        got = ze_path(sid)
        self.assertEqual(
            "ze",
            got.rsplit("/", 1)[-1],
            f"{got} carries the session id in the file name",
        )

    def test_an_id_equal_to_a_binary_suffix_is_accepted(self):
        """AC-6: a directory gives each session its own namespace.

        Under the suffix design ZE_SESSION_ID=test yielded bin/ze-test, the real
        test-runner binary, so the id had to be refused. A directory cannot
        collide, so the id is now usable and the guard is gone.
        """
        # "test" and "perf" are the suffixes of real binaries (ze-test, ze-perf).
        for name in ("test", "perf"):
            with self.subTest(sid=name):
                got, dates = ze_path_dated(name)
                self.sids.append(name)
                want = {f"{SESSION_ROOT}/{d}-{name}/bin/ze" for d in dates}
                self.assertIn(
                    got,
                    want,
                    f"ZE_SESSION_ID={name} must resolve a session directory; "
                    f"no binary name can collide with it",
                )


class TestZePathOffSessionIsSharedBin(SessionDirCase):
    """AC-2: a human shell and CI keep today's exact paths.

    This is the half of the design that must not move. It is green before the
    session directory lands and it stays green after.
    """

    def test_no_session_id_is_the_shared_bin(self):
        self.assertEqual("bin/ze", ze_path())

    def test_unsafe_id_falls_back_to_the_shared_bin(self):
        """AC-5: a malformed id degrades to today's behavior, never to a bad path.

        The id is now a path COMPONENT rather than a name suffix, so '/', '.'
        and '..' would escape or alias the session root.
        """
        for sid in (
            "a+b",
            "a@b",
            "a!b",
            "a:b",
            "a b",
            "..",
            ".",
            "../../etc",
            "a/b",
            "",
        ):
            with self.subTest(sid=sid):
                self.assertFalse(go_accepts(sid), "fixture must be Go-rejectable")
                self.assertEqual(
                    "bin/ze",
                    ze_path(sid),
                    f"make accepted {sid!r} that Go rejects: the build and the "
                    f"test runner would disagree about this session's artifacts",
                )
        self.assertEqual(
            [],
            sorted(p.name for p in (ROOT / SESSION_ROOT).glob("*etc*")),
            "a rejected id reached the session root",
        )


class TestMakeAndGoAgreeOnBinDir(SessionDirCase):
    """AC-1/AC-4/R-11: one directory, resolved by all FOUR implementations.

    make's answer is the directory of `make ze-session-binary-path`; Go's is
    sessionpath.BinDir; the shell's is `_session_dir` in
    .claude/hooks/lib/session-dir.sh; python's is `session_dir` in
    .claude/hooks/pretool-writeedit.py. A disagreement is silent at runtime: the
    build writes one place and the runner, the hook, or the digest reads
    another.

    Four copies is what R-11 names as the risk, and the shell and python copies
    were outside this test while four file headers cited it as what stops the
    drift. Three independent derivations of the session ID drifted for weeks
    behind exactly that shape of prose invariant.
    """

    def test_every_implementation_resolves_the_same_session_directory(self):
        sid = self.sid("agree4")
        # Dated YESTERDAY on purpose. A directory dated today is no
        # discriminator: a copy whose glob is broken falls through to today's
        # date and agrees by accident. Only an existing directory the fallback
        # would NOT name proves each copy took the lookup branch (AC-13).
        planted = (datetime.date.today() - datetime.timedelta(days=1)).isoformat()
        (ROOT / SESSION_ROOT / f"{planted}-{sid}").mkdir(parents=True, exist_ok=True)
        make_session = ze_path(sid).rsplit("/", 2)[0]
        go_session = go_bin_dir(sid).rsplit("/", 1)[0]
        # Compared as RESOLVED directories, not as strings. Three copies answer
        # relative to the checkout and the python one answers absolute, because
        # its consumer (state_file) compares against the absolute path the Write
        # tool sends. That is a representation difference, not a rule
        # difference, and the rule is what must not drift.
        answers = {
            name: str((ROOT / raw).resolve())
            for name, raw in (
                ("make (mk/helper-session.mk ZE_SCRATCH_DIR)", make_session),
                ("go (sessionpath.Root)", go_session),
                ("shell (session-dir.sh _session_dir)", shell_session_dir(sid)),
                ("python (pretool-writeedit.py session_dir)", python_session_dir(sid)),
            )
        }
        self.assertEqual(
            len(set(answers.values())),
            1,
            f"the four session-directory resolvers disagree: {answers}",
        )

    def test_a_dated_regular_file_is_no_session_directory_in_any_copy(self):
        """The one input that splits a glob from a `[ -d ]` test.

        A copy that takes the first glob match of ANY type answers with a
        regular file; a copy that requires a directory falls through to today's
        date. Both answers look reasonable in isolation and the pair is a silent
        split, which is R-11's whole shape.
        """
        sid = self.sid("notadir")
        yesterday = (datetime.date.today() - datetime.timedelta(days=1)).isoformat()
        decoy = ROOT / SESSION_ROOT / f"{yesterday}-{sid}"
        decoy.parent.mkdir(parents=True, exist_ok=True)
        decoy.write_text("not a directory\n", encoding="utf-8")
        self.addCleanup(decoy.unlink, True)

        answers = {
            name: str((ROOT / raw).resolve())
            for name, raw in (
                ("make", ze_path(sid).rsplit("/", 2)[0]),
                ("go", go_bin_dir(sid).rsplit("/", 1)[0]),
                ("shell", shell_session_dir(sid)),
                ("python", python_session_dir(sid)),
            )
        }
        self.assertEqual(
            len(set(answers.values())),
            1,
            f"a dated regular file splits the resolvers: {answers}",
        )
        self.assertEqual(
            str((ROOT / SESSION_ROOT / f"{today()}-{sid}").resolve()),
            answers["make"],
            "a regular file was accepted as this session's directory",
        )

    def test_on_session_both_resolve_the_same_dated_directory(self):
        sid = self.sid("agree")
        make_dir = ze_path(sid).rsplit("/", 1)[0]
        self.assertEqual(
            make_dir,
            go_bin_dir(sid),
            "make and internal/test/sessionpath disagree about this session's "
            "bin directory",
        )
        self.assertRegex(
            make_dir,
            DATED_BIN,
            "the agreed directory is not the dated session directory",
        )

    def test_off_session_both_resolve_the_shared_bin(self):
        self.assertEqual("bin", ze_path().rsplit("/", 1)[0])
        self.assertEqual("bin", go_bin_dir())

    def test_a_rejected_id_falls_back_in_both(self):
        for sid in ("a+b", "..", "a/b"):
            with self.subTest(sid=sid):
                self.assertEqual("bin", ze_path(sid).rsplit("/", 1)[0])
                self.assertEqual("bin", go_bin_dir(sid))


class TestSessionDirIsStableAcrossMidnight(SessionDirCase):
    """AC-13: the directory is found by glob, and dated only when it is created.

    A consumer that recomputed <today>-<sid> would resolve one directory at
    23:59 and a different one at 00:01, orphaning the binaries the session is
    running.
    """

    def plant(self, sid, date):
        path = ROOT / SESSION_ROOT / f"{date}-{sid}"
        (path / "bin").mkdir(parents=True, exist_ok=True)
        return f"{SESSION_ROOT}/{date}-{sid}"

    def test_an_existing_dated_directory_wins_over_today(self):
        sid = self.sid("midnight")
        yesterday = (datetime.date.today() - datetime.timedelta(days=1)).isoformat()
        planted = self.plant(sid, yesterday)

        # Each resolver is asserted in its own subTest, so one that drifts does
        # not hide the other.
        with self.subTest(resolver="make"):
            self.assertEqual(
                f"{planted}/bin/ze",
                ze_path(sid),
                "make minted a directory dated today while this session "
                "already owns one",
            )
        with self.subTest(resolver="sessionpath.BinDir"):
            self.assertEqual(
                f"{planted}/bin",
                go_bin_dir(sid),
                "sessionpath.BinDir minted a directory dated today while this "
                "session already owns one",
            )
        self.assertEqual(
            [f"{yesterday}-{sid}"],
            self.dated_dirs(sid),
            "resolving the directory created a second one",
        )

    def test_repeated_resolution_returns_one_directory(self):
        """One session, one directory, however many times it is resolved.

        This passes before the session directory lands, because a name suffix
        is stable too. It refuses the one shape the glob-then-create rule can
        still be built wrong in: a resolver that mints a directory per call.
        """
        sid = self.sid("repeat")
        first = ze_path(sid)
        second = ze_path(sid)
        self.assertEqual(first, second, "two calls resolved two directories")
        self.assertLessEqual(
            len(self.dated_dirs(sid)),
            1,
            f"more than one directory exists for {sid}: {self.dated_dirs(sid)}",
        )


class TestSessionDirsSortByDate(SessionDirCase):
    """AC-14: the date is a PREFIX, so `ls` orders by age and a glob selects a month.

    Once deletion is manual, the operator's only tools are the shell's own. A
    date anywhere but the front breaks both of them: `ls` would order by session
    id, and `rm -rf tmp/session/2026-07-*` would select nothing. The names come
    from the real resolver rather than from a fixture, so a producer that
    dropped the prefix or moved it reds this.
    """

    def resolved_name(self, sid, date):
        """The directory name `make ze-session-binary-path` resolves for a planted dated dir."""
        path = ROOT / SESSION_ROOT / f"{date}-{sid}"
        (path / "bin").mkdir(parents=True, exist_ok=True)
        resolved = ze_path(sid)
        prefix = f"{SESSION_ROOT}/"
        self.assertTrue(resolved.startswith(prefix), resolved)
        return resolved[len(prefix) :].split("/", 1)[0]

    def test_lexical_order_is_date_order(self):
        # Ids chosen so that ordering by ID alone would REVERSE the date order:
        # `a` is the newest session and sorts first, `c` is the oldest and sorts
        # last. Only a date-first name puts them back in age order.
        planted = [
            ("2026-07-04", self.sid("sortc")),
            ("2026-08-09", self.sid("sortb")),
            ("2026-09-02", self.sid("sorta")),
        ]
        names = [self.resolved_name(sid, date) for date, sid in planted]

        self.assertEqual(
            [f"{date}-{sid}" for date, sid in planted],
            sorted(names),
            f"lexical order is not date order: {sorted(names)}",
        )

    def test_a_month_glob_selects_exactly_that_month(self):
        july = self.resolved_name(self.sid("july"), "2026-07-31")
        august = self.resolved_name(self.sid("august"), "2026-08-01")

        selected = sorted(
            p.name for p in (ROOT / SESSION_ROOT).glob("2026-07-*") if p.is_dir()
        )

        self.assertIn(july, selected, "the July session is outside the July glob")
        self.assertNotIn(august, selected, "an August session is inside the July glob")


class TestNoSuffixVocabularyRemains(unittest.TestCase):
    """AC-12: the retired design is named nowhere a reader would take as current.

    A session's binaries took a NAME suffix (`bin/ze-<sid>`) until this spec
    moved them into the session's own directory. Four identifiers carried that
    design, and every one is gone: `ZE_BIN_SUFFIX` and `ZE_BIN_NAMES` from
    mk/helper-session.mk, `reap_binaries` from session-scratch.sh, `bare_named_perf`
    from test/perf/run.py. A tree that still names one is either running the old
    mechanism or telling its reader the old mechanism is live, and the second
    costs as much as the first.

    Two kinds of file are exempt, and both are exempt because naming what was
    retired IS their subject. `plan/` is not scanned at all: a spec, a journal
    row and a learned summary are dated records of what was true when they were
    written, and this spec's own text has to name what it retired.
    `ai/rules/points/RETIRED.md` is the ledger of retired rule points, so it
    quotes the old instruction beside the new one on purpose. `*_test.py` is
    skipped for the reason the AC-16 scan skips it: this file spells every
    banned token below.
    """

    TREES = ("Makefile", "mk", "scripts", ".claude", "internal", "test", "ai", "docs")
    RECORDS = ("RETIRED.md",)
    BANNED = (
        (re.compile(r"\bZE_BIN_SUFFIX\b"), "the session name-suffix variable"),
        (re.compile(r"\bZE_BIN_NAMES\b"), "the binary-name collision guard"),
        (re.compile(r"\breap_binaries\b"), "the suffixed-binary reaper"),
        (re.compile(r"\bbare_named_perf\b"), "the ze-perf bare-naming shim"),
        (
            re.compile(r"bin/[A-Za-z0-9_.*<>-]*-<(?:id|sid|session[-_]id)>"),
            "a suffixed binary path",
        ),
    )

    def test_no_file_names_the_retired_design(self):
        offenders = []
        for tree in self.TREES:
            base = ROOT / tree
            paths = [base] if base.is_file() else sorted(base.rglob("*"))
            for path in paths:
                if not path.is_file() or path.name.endswith("_test.py"):
                    continue
                if path.name in self.RECORDS:
                    continue
                try:
                    text = path.read_text()
                except (UnicodeDecodeError, OSError):
                    continue
                for line_no, line in enumerate(text.splitlines(), 1):
                    for pattern, what in self.BANNED:
                        if pattern.search(line):
                            rel = path.relative_to(ROOT)
                            offenders.append(f"{rel}:{line_no}: {what}")
        self.assertEqual(
            offenders,
            [],
            "the retired suffix design is still named: " + "; ".join(offenders),
        )


SEED_SCRIPT = "scripts/dev/session-seed-store.sh"

# A stand-in for the freshly built `ze`, so the seeding contract is testable
# without an 85 MB build. It does what `ze init` does -- read five credential
# lines from stdin and write the database under the config directory its OWN
# location resolves to (internal/core/paths/paths.go ConfigDirFromBinary) --
# and records what it was given. ZESEED_FAKE_NOOP makes it exit 0 while
# writing no database: that is the silent-empty-store shape the seeding exists
# to prevent, so the script must fail on it. ZESEED_FAKE_DELAY holds the
# database back so a concurrency test has a window to race in; the run is
# counted BEFORE the delay, so a second caller that got through shows up in the
# count even though its database write would have come last.
FAKE_ZE = """#!/bin/bash
here=$(cd "$(dirname "$0")" && pwd)
etc="$here/../etc/ze"
mkdir -p "$etc"
cat > "$etc/.init-stdin"
printf '%s\\n' "$@" > "$etc/.init-argv"
echo run >> "$etc/.init-count"
[ -n "${ZESEED_FAKE_NOOP:-}" ] && exit 0
[ -n "${ZESEED_FAKE_DELAY:-}" ] && sleep "$ZESEED_FAKE_DELAY"
echo fake > "$etc/database.zefs"
echo "initialized $etc/database.zefs"
"""


def seed(binary, env=None):
    """Run the seeding script the way a ze_core recipe runs it."""
    return subprocess.run(
        [SEED_SCRIPT, binary],
        cwd=str(ROOT),
        env=env or _clean_env(),
        capture_output=True,
        text=True,
        check=False,
    )


def make_n(target, session_id=None):
    """`make -n <target>`: the recipe as make would run it, without running it."""
    cmd = ["make", "-n", target]
    if session_id is not None:
        cmd.append(f"ZE_SESSION_ID={session_id}")
    return subprocess.run(
        cmd,
        cwd=str(ROOT),
        env=_clean_env(),
        capture_output=True,
        text=True,
        check=True,
    ).stdout


class TestSessionStoreIsSeeded(SessionDirCase):
    """AC-8: a session's isolated store is a SEEDED store, and provably so.

    `ze` resolves its config and database directory from its own location, so a
    binary in this session's directory reads <session-dir>/etc/ze. That
    isolation is the intent. The hazard is what happens next: NewBlob
    (internal/component/config/storage/blob.go) creates the blob and returns a
    nil error when it is absent, so an unseeded session is not red -- it is a
    daemon with no users and a fresh SSH host key (R-1, R-2). Every assertion
    below exists to keep that state unreachable.
    """

    def plant(self, tag, name="ze"):
        """A session directory holding a stand-in binary, ready to be seeded."""
        sid = self.sid(tag)
        session = ROOT / SESSION_ROOT / f"{today()}-{sid}"
        (session / "bin").mkdir(parents=True)
        fake = session / "bin" / name
        fake.write_text(FAKE_ZE, encoding="utf-8")
        fake.chmod(0o755)
        return f"{SESSION_ROOT}/{today()}-{sid}/bin/{name}", session / "etc" / "ze"

    def test_the_first_build_seeds_the_store(self):
        binary, etc = self.plant("seed")
        proc = seed(binary)
        self.assertEqual(0, proc.returncode, proc.stderr)
        self.assertTrue(
            (etc / "database.zefs").exists(),
            f"{binary} was built and its store is still empty",
        )

        lines = (etc / ".init-stdin").read_text(encoding="utf-8").splitlines()
        self.assertEqual(5, len(lines), f"ze init needs 5 credential lines: {lines}")
        user, password, host, port, name = lines
        self.assertEqual("admin", user)
        self.assertEqual(["127.0.0.1", "2222"], [host, port])
        self.assertEqual(
            etc.parents[1].name, name, "the store is not named for the session"
        )
        self.assertRegex(password, r"\A[0-9a-f]{48}\Z", "the password is not generated")

    def test_a_parallel_build_seeds_exactly_once(self):
        """`make -j build` runs three ze_core recipes, so three seeders at once.

        The existence test at the top of the script is a check-then-act: without
        a lock all three see no database, all three reach `ze init`, and they
        race on one database file. `make build` names $(ZEBIN_ZE),
        $(ZEBIN_APPLIANCE) and $(ZEBIN_STRIPPED) as prerequisites and there is
        no .NOTPARALLEL, so this is a first `make -j build` on a new session,
        not a contrived case.

        The count is the assertion that matters: three exits of 0 would also be
        satisfied by three concurrent inits that happened not to corrupt
        anything.
        """
        binary, etc = self.plant("parallel")
        for name in ("ze-appliance", "ze-stripped"):
            twin = ROOT / binary
            sibling = twin.parent / name
            sibling.write_text(FAKE_ZE, encoding="utf-8")
            sibling.chmod(0o755)

        env = dict(_clean_env(), ZESEED_FAKE_DELAY="2")
        binaries = [binary, f"{binary.rsplit('/', 1)[0]}/ze-appliance"]
        binaries.append(f"{binary.rsplit('/', 1)[0]}/ze-stripped")
        with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
            results = list(pool.map(lambda b: seed(b, env), binaries))

        for proc, which in zip(results, binaries):
            self.assertEqual(0, proc.returncode, f"{which}: {proc.stderr}")
        self.assertTrue((etc / "database.zefs").exists(), "no store was seeded")
        runs = (etc / ".init-count").read_text(encoding="utf-8").split()
        self.assertEqual(
            1,
            len(runs),
            f"ze init ran {len(runs)} times concurrently on one database",
        )
        self.assertFalse(
            (etc / ".seed-lock").exists(), "the seed lock outlived the seeding"
        )

    def test_the_generated_password_stays_in_the_session_and_is_owner_only(self):
        """The secret is generated per session, under a gitignored root.

        `.gitignore` carries `tmp/*`, so nothing here can reach a commit; the
        mode is what keeps it from every other account on the machine.
        """
        binary, etc = self.plant("secret")
        self.assertEqual(0, seed(binary).returncode)
        pwfile = etc / ".dev-password"
        self.assertEqual(0o600, pwfile.stat().st_mode & 0o777, f"{pwfile} is not 0600")
        self.assertEqual(
            "",
            subprocess.run(
                ["git", "status", "--porcelain", "--ignored=no", str(pwfile)],
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                check=True,
            ).stdout.strip(),
            "the generated password is visible to git",
        )

    def test_a_second_build_neither_reseeds_nor_rotates(self):
        binary, etc = self.plant("idempotent")
        self.assertEqual(0, seed(binary).returncode)
        first = (etc / ".dev-password").read_text(encoding="utf-8")

        proc = seed(binary)
        self.assertEqual(0, proc.returncode, proc.stderr)
        self.assertEqual(
            1,
            len((etc / ".init-count").read_text(encoding="utf-8").split()),
            "the second build reseeded a store that was already seeded",
        )
        self.assertEqual(
            first,
            (etc / ".dev-password").read_text(encoding="utf-8"),
            "the second build rotated this session's password",
        )

    def test_an_init_that_leaves_no_store_fails_the_build(self):
        """The silent-empty-store shape, refused.

        An exit code is not evidence that a store exists. If it were trusted,
        this phase would have shipped the failure it exists to close.
        """
        binary, etc = self.plant("noop")
        env = _clean_env()
        env["ZESEED_FAKE_NOOP"] = "1"
        proc = seed(binary, env)
        self.assertNotEqual(0, proc.returncode, "an empty store was accepted")
        self.assertIn("database.zefs", proc.stderr)
        self.assertFalse((etc / "database.zefs").exists())

    def test_seeding_is_refused_outside_a_session_directory(self):
        """bin/ holds the operator's ze, whose store is the operator's own.

        The refusal is asserted on the guard's own word and on the absence of
        anything written, not on a non-zero exit: `bin` and `tmp/session` fail
        further down for their own reasons (no binary there, no store there),
        so an exit code alone lets the guard be deleted with the test green.
        """
        for path in (
            "bin/ze",
            "bin/ze-stripped",
            "tmp/ze",
            "tmp/session/ze",
            "tmp/session/2026-08-10-x/y/bin/ze",
        ):
            with self.subTest(path=path):
                proc = seed(path)
                self.assertNotEqual(0, proc.returncode, f"{path} was accepted")
                self.assertIn(
                    f"refusing {path}",
                    proc.stderr,
                    f"{path} was refused for some other reason: {proc.stderr}",
                )
                # Where the script WOULD have seeded, derived its way: the
                # binary's directory, minus a trailing bin/. `bin/ze` resolves
                # the repository's own etc/ze, which exists and must stay the
                # operator's, so the assertion is on the file only this script
                # writes, never on the directory.
                bindir = (ROOT / path).parent
                session = bindir.parent if bindir.name == "bin" else bindir
                self.assertFalse(
                    (session / "etc" / "ze" / ".dev-password").exists(),
                    f"seeding wrote a password for {path}",
                )
        self.assertFalse(
            (ROOT / "etc" / "ze" / ".dev-password").exists(),
            "seeding wrote into the repository's own config directory",
        )

    def test_a_config_dir_override_is_refused(self):
        """ze.config.dir would send the seed somewhere the script cannot vouch for.

        Both spellings reach internal/core/env the same way: normalize() lowers
        the name and treats dots and underscores as equivalent.
        """
        for key in ("ZE_CONFIG_DIR", "ze.config.dir"):
            with self.subTest(key=key):
                binary, etc = self.plant(f"override-{key.replace('.', '-')}")
                env = _clean_env()
                env[key] = str(ROOT / "etc" / "ze")
                proc = seed(binary, env)
                self.assertNotEqual(0, proc.returncode, f"{key} was ignored")
                self.assertIn(key, proc.stderr)
                self.assertFalse((etc / "database.zefs").exists())

    def test_every_ze_core_recipe_seeds_and_only_on_session(self):
        """AC-8 does not stop at `make ze-build`; AC-2 keeps every recipe unmoved.

        ze, ze-appliance, and ze-stripped each link `internal/core/resolve`,
        `internal/component/ssh` and `internal/plugins/init` (measured with
        `go list -deps` over each recipe's own tags), so each resolves the same
        <session-dir>/etc/ze and each can seed it. A target that seeded only
        under `make ze-build` would leave the same silent empty store one target over.
        """
        sid = self.sid("recipe")
        targets = (
            ("ze-build", "ze"),
            ("ze-appliance-build", "ze-appliance"),
            ("ze-stripped-build", "ze-stripped"),
        )
        for target, binary_name in targets:
            with self.subTest(target=target, session=True):
                self.assertIn(
                    f"{SEED_SCRIPT} {SESSION_ROOT}/{today()}-{sid}/bin/{binary_name}",
                    make_n(target, sid),
                    f"`make {target}` builds a session binary that resolves "
                    f"<session-dir>/etc/ze and leaves that store empty",
                )
            with self.subTest(target=target, session=False):
                off = make_n(target)
                self.assertNotIn("session-seed-store", off, "a human's build seeds")
                self.assertNotIn(SESSION_ROOT, off, "a human's build moved")

        # The rest link no init and reach no silent path, so they never seed.
        for target in (
            "ze-setup-build",
            "ze-test-build",
            "ze-chaos-build",
            "ze-analyze-build",
            "ze-perf-build",
        ):
            with self.subTest(target=target):
                self.assertNotIn(
                    "session-seed-store",
                    make_n(target, sid),
                    f"`make {target}` seeds with a binary that carries no init",
                )

    def test_a_stripped_only_build_seeds_its_store(self):
        """The real binary, not a stand-in, and the narrowest ze_core target.

        This is the one assertion that runs the shipped `ze init` end to end.
        A session that builds only ze-stripped used to get an isolated store
        that nothing ever seeded, and no error anywhere (R-1, R-2).
        """
        sid = self.sid("stripped-build")
        proc = subprocess.run(
            ["make", "ze-stripped-build", f"ZE_SESSION_ID={sid}"],
            cwd=str(ROOT),
            env=_clean_env(),
            capture_output=True,
            text=True,
            check=False,
            timeout=1800,
        )
        self.assertEqual(0, proc.returncode, proc.stdout + proc.stderr)

        dated = self.dated_dirs(sid)
        self.assertEqual(1, len(dated), f"expected one session directory: {dated}")
        session = ROOT / SESSION_ROOT / dated[0]
        self.assertTrue((session / "bin" / "ze-stripped").exists())
        self.assertFalse(
            (session / "bin" / "ze").exists(),
            "the probe built more than the stripped target, so it proves nothing",
        )

        db = session / "etc" / "ze" / "database.zefs"
        self.assertTrue(db.exists(), "a stripped-only session got an empty store")
        self.assertGreater(db.stat().st_size, 0, "the store is a zero-length file")
        self.assertEqual(
            0o600,
            (session / "etc" / "ze" / ".dev-password").stat().st_mode & 0o777,
        )


class TestValidationDoesNotRunShell(unittest.TestCase):
    """The charset check interpolates the id into a shell command.

    A single quote is the only character that can terminate that literal, so it
    is refused in pure make before the shell sees it. If that guard regresses,
    `make` executes attacker-chosen shell at parse time -- on every invocation.
    """

    def test_quote_bearing_id_is_refused_and_not_executed(self):
        marker = "ZE-INJECTION-MARKER"
        hostile = f"a'; echo {marker}; '"
        proc = subprocess.run(
            ["make", "ze-session-binary-path", f"ZE_SESSION_ID={hostile}"],
            cwd=str(ROOT),
            env=_clean_env(),
            capture_output=True,
            text=True,
            check=True,
        )
        self.assertEqual("bin/ze", proc.stdout.strip())
        self.assertNotIn(marker, proc.stdout + proc.stderr, "shell injection executed")


class CleanSessionsCase(unittest.TestCase):
    """Drives `make ze-session-clean` against a fixture session root.

    The target is real and so is the recipe; only the root it sweeps is
    redirected, through the ZE_SESSION_ROOT that mk/helper-session.mk already defines
    and every other consumer already reads. A command-line assignment outranks
    the makefile one, which is the same edge `ze_path` exercises for the id.

    Nothing here can reach the live tmp/session/: every fixture root is a fresh
    temporary directory, and each test asserts the exact set of survivors.
    """

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="ze-clean-sessions-")
        self.root = pathlib.Path(self.tmp) / "tmp" / "session"
        self.root.mkdir(parents=True)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def plant_dir(self, name):
        path = self.root / name
        (path / "bin").mkdir(parents=True)
        (path / "bin" / "ze").write_text("binary", encoding="utf-8")
        return path

    def plant_flat(self, name):
        path = self.root / name
        path.write_text("flat marker", encoding="utf-8")
        return path

    def clean(self, before=None):
        cmd = ["make", "ze-session-clean", f"ZE_SESSION_ROOT={self.root}"]
        if before is not None:
            cmd.append(f"BEFORE={before}")
        return subprocess.run(
            cmd,
            cwd=str(ROOT),
            env=_clean_env(),
            capture_output=True,
            text=True,
            check=False,
        )

    def survivors(self):
        return sorted(p.name for p in self.root.iterdir())


class TestCleanSessionsRefusesWithoutBefore(CleanSessionsCase):
    """AC-15: no BEFORE, no deletion.

    The date is the operator's, always typed and never defaulted. A target that
    fell back to "now minus a window" would be the age timer this spec deletes,
    reintroduced behind a make target.
    """

    def test_a_bare_invocation_removes_nothing(self):
        self.plant_dir("2020-01-01-ancient")
        self.plant_flat(".sid-by-pid-42-987")

        proc = self.clean()

        self.assertEqual(2, proc.returncode, proc.stdout + proc.stderr)
        self.assertIn("BEFORE", proc.stdout + proc.stderr)
        self.assertEqual(
            [".sid-by-pid-42-987", "2020-01-01-ancient"],
            self.survivors(),
            "a bare invocation deleted a session",
        )

    def test_an_empty_before_removes_nothing(self):
        # `make ze-session-clean BEFORE=` is what a shell variable that did not
        # expand looks like. It must refuse exactly as the bare form does.
        self.plant_dir("2020-01-01-ancient")

        proc = self.clean(before="")

        self.assertEqual(2, proc.returncode, proc.stdout + proc.stderr)
        self.assertEqual(["2020-01-01-ancient"], self.survivors())

    def test_a_date_that_is_not_a_date_removes_nothing(self):
        self.plant_dir("2020-01-01-ancient")

        proc = self.clean(before="last-tuesday")

        self.assertEqual(2, proc.returncode, proc.stdout + proc.stderr)
        self.assertIn("YYYY-MM-DD", proc.stdout + proc.stderr)
        self.assertEqual(["2020-01-01-ancient"], self.survivors())

    def test_a_before_that_is_shell_metacharacters_runs_nothing(self):
        """BEFORE is DATA, so the format check must see it before a shell does.

        Splicing $(BEFORE) into the recipe put the operator's typing inside a
        double-quoted shell literal, where `";touch marker;x"` closed the quote
        and ran. The refusal message printed afterwards, which reads as though
        the guard held. mk/helper-session.mk refuses a quote in the session id for the
        same reason one file away.
        """
        marker = pathlib.Path(self.tmp) / "injected"
        self.plant_dir("2020-01-01-ancient")

        proc = self.clean(before=f'";touch {marker};x"')

        self.assertEqual(2, proc.returncode, proc.stdout + proc.stderr)
        self.assertFalse(marker.exists(), "BEFORE reached a shell before the guard")
        self.assertEqual(["2020-01-01-ancient"], self.survivors())


class TestCleanSessionsRemovesOnlyOlder(CleanSessionsCase):
    """AC-15: strictly older dated directories, and nothing else.

    The boundary is the point: a directory dated exactly BEFORE is not older
    than BEFORE, and an operator clearing "everything before the 1st" must not
    lose the session they started on the 1st.
    """

    def test_the_boundary_date_survives(self):
        self.plant_dir("2026-07-31-older")
        self.plant_dir("2026-08-01-boundary")
        self.plant_dir("2026-08-02-newer")

        proc = self.clean(before="2026-08-01")

        self.assertEqual(0, proc.returncode, proc.stdout + proc.stderr)
        self.assertEqual(
            ["2026-08-01-boundary", "2026-08-02-newer"],
            self.survivors(),
            "the comparison is not strict",
        )

    def test_the_binaries_go_with_the_directory(self):
        older = self.plant_dir("2026-07-31-older")

        self.clean(before="2026-08-01")

        self.assertFalse(older.exists(), "the directory survived its own removal")

    def test_the_flat_marker_files_are_never_candidates(self):
        # .sid-by-pid-<clipid> mints the id a directory is named for, and
        # .closure-ack-<stem> is keyed by spec stem, so both live flat beside
        # the dated directories and outlive every one of them. The gate markers
        # stay flat with them (2026-08-03 decision).
        #
        # The last entry is a per-spec digest from BEFORE the digest moved into
        # <session-dir>/state/ on 2026-08-10. A date sweep must not take it
        # either: _find_latest_state_for_spec still reads that location, so a
        # resuming session's only digest can be one of these.
        flat = [
            ".sid-by-pid-42-987",
            ".closure-ack-some-spec",
            ".session-abcd",
            "session-state-some-spec-abcd.md",
        ]
        for name in flat:
            self.plant_flat(name)
        self.plant_dir("2020-01-01-ancient")

        proc = self.clean(before="2099-01-01")

        self.assertEqual(0, proc.returncode, proc.stdout + proc.stderr)
        self.assertEqual(sorted(flat), self.survivors(), "a flat marker was swept")

    def test_nothing_outside_the_session_root_is_reachable(self):
        # The security property (spec Security Review, "Deletion scope"): the
        # target can only ever remove <root>/<dated-dir>. A neighbour named like
        # a session, a directory that is not dated, and the root's own parent
        # are all planted and all must survive a sweep dated far in the future.
        checkout = pathlib.Path(self.tmp)
        (checkout / "bin").mkdir()
        (checkout / "bin" / "ze").write_text("shared", encoding="utf-8")
        (checkout / "tmp" / "2026-07-01-decoy").mkdir()
        (checkout / "tmp" / "verify").mkdir()
        (self.root / "kernel").mkdir()
        self.plant_dir("2020-01-01-ancient")

        proc = self.clean(before="2099-01-01")

        self.assertEqual(0, proc.returncode, proc.stdout + proc.stderr)
        self.assertTrue((checkout / "bin" / "ze").is_file(), "bin/ was reached")
        self.assertTrue((checkout / "tmp" / "2026-07-01-decoy").is_dir())
        self.assertTrue((checkout / "tmp" / "verify").is_dir())
        self.assertEqual(["kernel"], self.survivors(), "an undated directory was swept")


if __name__ == "__main__":
    unittest.main()
