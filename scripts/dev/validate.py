#!/usr/bin/env python3
"""Post-verify validation: catches recurring implementation mistakes.

Each check derives from a documented defect pattern in
plan/learned/RECURRING-PATTERNS.md.

Exit codes:
  0 - all checks passed
  1 - findings (ISSUE severity)
  2 - script error
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

YELLOW = "\033[33m"
RED = "\033[31m"
GREEN = "\033[32m"
RESET = "\033[0m"


@dataclass(frozen=True)
class Finding:
    severity: str
    file: str
    line: int
    message: str

    def __str__(self) -> str:
        loc = f"{self.file}:{self.line}" if self.line else self.file
        return f"[{self.severity}] {loc}: {self.message}"


SOURCE_ANCHOR_RE = re.compile(r"<!--\s*source:\s*(\S+)\s+--")
SOURCE_ANCHOR_LINE_RE = re.compile(r"<!--\s*source:\s*\S+\.go:\d+\s")
AC_ROW_RE = re.compile(r"^\|\s*(AC-\d+)\s*\|([^|]*)\|([^|]*)\|([^|]*)\|")
EXPORTED_FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(")
EXPORTED_TYPE_RE = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\b")
# A method declaration with a named or unnamed receiver. recvtype is the
# receiver type name. It lets the wiring check distinguish concrete interface
# implementations from free functions.
FUNC_RECV_RE = re.compile(
    r"^func\s+\(\s*(?:\w+\s+)?\*?(?P<recvtype>[A-Za-z_][A-Za-z0-9_]*)(?:\[[^\]]*\])?\s*\)\s*[A-Z]"
)
EXPORTED_IFACE_RE = re.compile(r"^type\s+[A-Z][A-Za-z0-9_]*\s+interface\s*\{")
EXPORTED_IFACE_NAMED_RE = re.compile(
    r"^type\s+(?P<name>[A-Z][A-Za-z0-9_]*)\s+interface\s*\{"
)
IFACE_METHOD_RE = re.compile(r"^\s*([A-Z][A-Za-z0-9_]*)\s*\(")
REGISTERED_SERVER_RE = re.compile(
    r"\b[A-Za-z_][A-Za-z0-9_]*\.Register"
    r"(?P<interface>[A-Z][A-Za-z0-9_]*Server)\s*\("
    r"[^,\n]+,\s*&(?P<receiver>[a-z][A-Za-z0-9_]*)\s*\{"
)
# grpc-go calls these methods only through stats.Handler. The concrete handler
# is private, so neither the same-package interface rule nor a bare
# cross-package name search can observe the dispatch. Keep this allowlist exact:
# another exported method on the handler still needs a caller.
INTERFACE_DISPATCH_METHODS = {
    ("internal/component/api/grpc", "transportCompletionStatsHandler"): frozenset(
        {"TagRPC", "HandleRPC", "TagConn", "HandleConn"}
    ),
}
SPEC_STATUS_RE = re.compile(r"^\|\s*Status\s*\|\s*(\S+)\s*\|", re.MULTILINE)

CLI_PATHS = (
    "internal/component/cli/",
    "internal/component/cmd/",
    "internal/plugins/",
)
REGISTER_RE = re.compile(r'MustRegister\w+\(\s*"([^"]+)"')


def find_repo_root(start: Path | None = None) -> Path:
    cur = (start or Path.cwd()).resolve()
    while True:
        if (cur / "go.mod").exists():
            return cur
        if cur.parent == cur:
            raise SystemExit("could not find go.mod above " + str(start or Path.cwd()))
        cur = cur.parent


def changed_files(root: Path) -> list[str]:
    files: set[str] = set()
    commands = (
        ("git", "diff", "--name-only", "HEAD"),
        ("git", "ls-files", "--others", "--exclude-standard"),
    )
    for cmd in commands:
        proc = subprocess.run(
            cmd, cwd=root, text=True, capture_output=True, check=False
        )
        if proc.returncode == 0:
            for line in proc.stdout.splitlines():
                line = line.strip()
                if line:
                    files.add(line)
    return sorted(files)


def check_source_anchor_line_numbers(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return findings
    for md_file in sorted(docs_dir.rglob("*.md")):
        try:
            lines = md_file.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, 1):
            m = SOURCE_ANCHOR_LINE_RE.search(line)
            if m:
                anchor = SOURCE_ANCHOR_RE.search(line)
                path = anchor.group(1) if anchor else "unknown"
                rel = md_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"source anchor {path} contains line number; use path only (line numbers rot)",
                    )
                )
    return findings


def check_source_anchor_stale_paths(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return findings
    for md_file in sorted(docs_dir.rglob("*.md")):
        try:
            lines = md_file.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, 1):
            m = SOURCE_ANCHOR_RE.search(line)
            if not m:
                continue
            path = m.group(1)
            # External anchors point outside the repo and cannot be resolved
            # here, so -- like http(s) URLs -- they only document provenance we
            # cannot verify: a home-relative checkout (~/...) or an absolute
            # path (/...) is never an in-repo anchor, which is always
            # repo-relative. Skip bare single-token names too (no path).
            if path.startswith(("http://", "https://", "~", "/")) or "/" not in path:
                continue
            path_clean = re.sub(r":\d+$", "", path)
            # A path that climbs out of the repository root names a SIBLING
            # checkout (`../gh-pages/tools/build.py` is the website's), so it is
            # external in the same way a `~` or absolute path is. Whether it
            # resolves says where the reader keeps their checkouts, not whether
            # the documentation is fresh: it exists on a machine that clones
            # both repositories side by side and never on a CI runner that
            # clones one. Escaping the root is the PROPERTY that makes an anchor
            # unresolvable here; the spellings above are three instances of it.
            if os.path.normpath(path_clean).startswith(".."):
                continue
            if not (root / path_clean).exists():
                rel = md_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"source anchor points to non-existent file: {path_clean}",
                    )
                )
    return findings


def _has_cross_pkg_ref(
    root: Path, sym: str, pkg_dir: str, search_dirs: list[str]
) -> bool:
    """True if sym is referenced (whole word) by another package.

    Test files are excluded, so a symbol only tests call reads as unwired. A
    helper package under internal/test/ inverts that: its whole purpose is to be
    called from another package's _test.go, so there a cross-package TEST caller
    is the wiring. The exemption is scoped to where the symbol is DEFINED, not
    to where it is called, so it counts test callers for internal/test/golden
    while leaving every other package's rule intact. It stays this narrow on
    purpose: internal/test/ also holds product code that ships in the ze-test
    binary (cmd/ze/ze_test_register.go blank-imports internal/test/cli), so
    dropping the subtree from the checked population would blind the gate over
    120 files to clear 3 findings.
    """
    defined_in_test_helper = pkg_dir.startswith("internal/test/")
    proc = subprocess.run(
        ["grep", "-rlw", "--include=*.go", sym] + search_dirs,
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    for mf in (f.strip() for f in proc.stdout.splitlines()):
        if not mf:
            continue
        if str(Path(mf).parent) == pkg_dir:
            continue
        if not mf.endswith("_test.go"):
            return True
        # A test caller counts only for a helper defined under internal/test/,
        # and only when the file IMPORTS that helper. `grep -w` matches a bare
        # word, and over _test.go that is far too loose to accept on its own:
        # it cleared Colors.Red on the English word in an ike comment,
        # BaseTest.GetName on a protobuf accessor, and runner.TestSet on
        # `func TestSet(t *testing.T)` in env_test.go -- a collision class that
        # exists only among test files. The import narrows the FILE to one that
        # could call the helper. It does not make the match a reference: a word
        # colliding inside an importing file still clears, exactly as the
        # non-test branch above clears Compare on slogutil.go. That bare-word
        # standard is the gate's, not this branch's, and this branch is
        # strictly narrower than it.
        if not defined_in_test_helper:
            continue
        if _imports_package(root, mf, pkg_dir):
            return True
    return False


def _imports_package(root: Path, go_file: str, pkg_dir: str) -> bool:
    """True if go_file's source carries an import path ending in pkg_dir."""
    try:
        content = (root / go_file).read_text(encoding="utf-8")
    except OSError:
        return False
    return f'/{pkg_dir}"' in content


# One const spec's leading form: "Name1, Name2  Type = expr", "Name = expr",
# "Name Type", or a bare "Name". Group 1 = comma-separated names, group 2 = the
# explicit type (absent for value-only or bare specs), group 3 = the "=" when an
# expression is present.
CONST_SPEC_RE = re.compile(
    r"^\s*([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)"
    r"(?:\s+([A-Za-z_][A-Za-z0-9_.]*))?"
    r"(\s*=)?"
)


def _const_spec(text: str) -> tuple[list[str], str | None, bool]:
    """Parse one const spec into (names, explicit_type, has_value_expression)."""
    m = CONST_SPEC_RE.match(text)
    if not m:
        return [], None, False
    names = [n.strip() for n in m.group(1).split(",")]
    return names, m.group(2), m.group(3) is not None


def _exported(names: list[str]) -> list[str]:
    return [n for n in names if n[:1].isupper()]


def _exported_consts_of_type(root: Path, pkg_dir: str, type_name: str) -> set[str]:
    """Exported const identifiers declared with type_name in pkg_dir.

    A typed enum is reached through its constant values, so the bare type name
    may never appear in another package (callers switch on RouteVerbInstall,
    never spelling RouteVerb) and the bare-name grep undercounts. This recovers
    the constants -- single-line consts, block consts, multi-name specs, and the
    Go iota idiom where a bare spec inherits the type of the last preceding spec
    that carried one (an explicit "= expr" with no type resets that inheritance,
    matching the language spec) -- so the caller can prove the type is reachable.
    """
    names: set[str] = set()
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return names
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        in_block = False
        inherits = False  # do bare specs in this block currently inherit type_name?
        for line in content.splitlines():
            stripped = line.strip()
            if not in_block:
                if re.match(r"^const\s*\(", stripped):
                    in_block = True
                    inherits = False
                else:
                    m = re.match(r"^const\s+(.+)$", stripped)
                    if m:
                        spec, etype, _ = _const_spec(m.group(1))
                        if etype == type_name:
                            names.update(_exported(spec))
                continue
            if stripped.startswith(")"):
                in_block = False
                inherits = False
                continue
            if not stripped or stripped.startswith("//"):
                continue
            spec, etype, has_value = _const_spec(line)
            if not spec:
                continue
            if etype is not None:
                inherits = etype == type_name
                if inherits:
                    names.update(_exported(spec))
            elif has_value:
                inherits = False  # own type from the value; resets iota inheritance
            elif inherits:
                names.update(_exported(spec))
    return names


def _type_used_as_field_in_pkg(root: Path, pkg_dir: str, type_name: str) -> bool:
    """True if type_name is used as a struct field type within its own package.

    A type composed into a struct is reached through field access (inv.CPU,
    cap.Families), so its bare name need not appear in any other package -- the
    same blind spot the constants check covers, for serialized and wire structs.
    Scoped to the declaring package: a field declaration in another package would
    name the type, so the cross-package grep already covers that case.
    """
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return False
    # Exported field name, optional []/*/map/chan wrappers, then the type. Only
    # matched inside a struct body, so a "Name Type = value" const spec (which
    # has the same leading shape) is never mistaken for a field.
    field_re = re.compile(
        r"^\s*[A-Z][A-Za-z0-9_]*\s+(?:\[\]|\*|map\[[^\]]+\]|chan\s+)*\*?"
        + re.escape(type_name)
        + r"\b"
    )
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        struct_depth = 0  # brace nesting while inside one or more struct bodies
        for line in content.splitlines():
            if struct_depth > 0:
                if field_re.match(line):
                    return True
                struct_depth = max(0, struct_depth + line.count("{") - line.count("}"))
            elif "struct {" in line:
                struct_depth = max(0, line.count("{") - line.count("}"))
    return False


def _func_signature(line: str) -> tuple[str, str, str] | None:
    """(name, parameter-list, return-signature) of an exported func/method decl.

    Balances parentheses from the parameter-list open so a func-typed parameter's
    inner ')' and a multi-value return tuple '(*T, error)' are both handled. The
    receiver of a method is consumed by EXPORTED_FUNC_RE's optional group, so the
    balance starts at the PARAMETER list, never the receiver. Returns None when
    the line is not an exported func declaration.
    """
    m = EXPORTED_FUNC_RE.match(line)
    if not m:
        return None
    start = m.end() - 1  # index of the '(' opening the parameter list
    i = start
    depth = 0
    n = len(line)
    while i < n:
        c = line[i]
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                break
        i += 1
    return m.group(1), line[start + 1 : i], line[i + 1 :].split("{", 1)[0]


def _wired_func_names_naming_type(
    root: Path, pkg_dir: str, type_name: str, search_dirs: list[str], part: int
) -> bool:
    """True if an exported same-package func names type_name in signature `part`
    (1 = parameter list, 2 = return signature) AND itself has a cross-package
    non-test caller.

    The cross-package-caller requirement on the CONTAINING function is what bounds
    both seams below. Without it, any type mentioned in any signature would be
    exempt, which is nearly every dead exported type: the check would fail open.
    """
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return False
    word = re.compile(r"(?<![\w.])" + re.escape(type_name) + r"\b")
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        for line in content.splitlines():
            sig = _func_signature(line)
            if sig is None or not word.search(sig[part]):
                continue
            func_name = sig[0]
            if func_name == type_name:
                continue  # its own constructor name collision; ignore
            if _has_cross_pkg_ref(root, func_name, pkg_dir, search_dirs):
                return True
    return False


def _type_returned_by_wired_func(
    root: Path, pkg_dir: str, type_name: str, search_dirs: list[str]
) -> bool:
    """True if an exported same-package func returns type_name and is itself wired.

    A type reached only through an exported constructor or accessor
    (NewEvaluator() *Evaluator, Global() *Evaluator) is never spelled by name in
    another package: idiomatic callers write `ev := pkg.Global()` and let type
    inference name it. The bare-name grep therefore undercounts. If any exported
    function whose return signature names the type has a cross-package caller, the
    type is produced across a package boundary and is wired -- flagging it is a
    false positive (the constructor-seam case, sibling to the constants and
    struct-field cases above).
    """
    return _wired_func_names_naming_type(root, pkg_dir, type_name, search_dirs, 2)


def _type_used_as_param_of_wired_func(
    root: Path, pkg_dir: str, type_name: str, search_dirs: list[str]
) -> bool:
    """True if an exported same-package func takes type_name as a parameter and is
    itself wired.

    A type that is only the PARAMETER of an exported setter or registration hook
    (SetPingFactory(PingFactory), SetCommandCompleter(CommandModeCompleter)) is
    never spelled by name in another package: Go's structural assignability means
    the caller passes a plain func literal or a concrete value
    (m.SetPingFactory(streamingPingFactory)), so the bare-name grep undercounts.
    The type is part of the exported CONTRACT of a wired function, which is what
    makes it live (the parameter-seam case, sibling to the constructor-return
    case above).

    Bounded by the same rule as that sibling: the function TAKING the parameter
    must itself have a cross-package non-test caller. A type whose only exported
    consumer is dead stays flagged.
    """
    return _wired_func_names_naming_type(root, pkg_dir, type_name, search_dirs, 1)


def _pkg_exported_interface_methods(root: Path, pkg_dir: str) -> set[str]:
    """Method names declared by exported interfaces in pkg_dir's non-test files.

    A method that satisfies an exported interface is reached through interface dispatch
    (e.g. a pluggable transport backend), which the bare-name cross-package grep cannot
    see. Combined with an UNEXPORTED receiver type -- unnameable from another package --
    such a method is wired through the interface, not dead, so flagging it is a false
    positive (the interface-seam case).
    """
    names: set[str] = set()
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return names
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        depth = 0  # brace nesting while inside an exported interface body
        in_iface = False
        for line in content.splitlines():
            if not in_iface:
                if EXPORTED_IFACE_RE.match(line):
                    depth = line.count("{") - line.count("}")
                    in_iface = depth > 0
                continue
            mm = IFACE_METHOD_RE.match(line)
            if mm:
                names.add(mm.group(1))
            depth += line.count("{") - line.count("}")
            if depth <= 0:
                in_iface = False
    return names


def _type_embedded_in_wired_interface(
    root: Path, pkg_dir: str, type_name: str, search_dirs: list[str]
) -> bool:
    """True when a live exported interface composes type_name.

    Go interface embedding carries the embedded contract without callers ever
    spelling its type name. The outer interface must itself have a
    cross-package production reference, or one dead interface could hide
    another.
    """
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return False
    embedded = re.compile(r"^\s*" + re.escape(type_name) + r"\s*$")
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            lines = go_file.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        outer = ""
        depth = 0
        for line in lines:
            if depth <= 0:
                match = EXPORTED_IFACE_NAMED_RE.match(line)
                if match is None:
                    continue
                outer = match.group("name")
                depth = line.count("{") - line.count("}")
                continue
            if embedded.match(line) and _has_cross_pkg_ref(
                root, outer, pkg_dir, search_dirs
            ):
                return True
            depth += line.count("{") - line.count("}")
    return False


def _registered_interface_methods_by_receiver(
    root: Path, pkg_dir: str
) -> dict[str, set[str]]:
    """Exported API methods reached through a registered service implementation.

    Generated gRPC clients call the service interface, never the concrete
    unexported implementation type. Binding an implementation to a generated
    Register*Server function is therefore the production call site that makes
    its interface methods reachable.
    """
    registrations: set[tuple[str, str]] = set()
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return {}
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        registrations.update(
            (match.group("receiver"), match.group("interface"))
            for match in REGISTERED_SERVER_RE.finditer(content)
        )
    if not registrations:
        return {}

    methods_by_interface: dict[str, set[str]] = {}
    wanted_interfaces = {interface for _, interface in registrations}
    interface_declarations = {
        interface: re.compile(
            r"^type\s+" + re.escape(interface) + r"\s+interface\s*\{",
            re.MULTILINE,
        )
        for interface in wanted_interfaces
    }
    for source_dir in ("api", "internal", "pkg"):
        source_path = root / source_dir
        if not source_path.is_dir():
            continue
        for go_file in sorted(source_path.rglob("*.go")):
            if go_file.name.endswith("_test.go"):
                continue
            try:
                content = go_file.read_text(encoding="utf-8")
            except OSError:
                continue
            for interface, declaration in interface_declarations.items():
                match = declaration.search(content)
                if match is None:
                    continue
                names = methods_by_interface.setdefault(interface, set())
                depth = 1
                for line in content[match.end() :].splitlines():
                    method = IFACE_METHOD_RE.match(line)
                    if method:
                        names.add(method.group(1))
                    depth += line.count("{") - line.count("}")
                    if depth <= 0:
                        break

    methods_by_receiver: dict[str, set[str]] = {}
    for receiver, interface in registrations:
        methods_by_receiver.setdefault(receiver, set()).update(
            methods_by_interface.get(interface, set())
        )
    return methods_by_receiver


def check_cross_package_wiring(root: Path, changed: list[str]) -> list[Finding]:
    go_files = [
        f
        for f in changed
        if f.endswith(".go")
        and not f.endswith("_test.go")
        and (f.startswith("internal/") or f.startswith("cmd/"))
    ]
    if not go_files:
        return []

    symbols: list[tuple[str, int, str, str, str, str | None]] = []
    for go_file in go_files:
        full_path = root / go_file
        if not full_path.exists():
            continue
        pkg_dir = str(Path(go_file).parent)
        try:
            content = full_path.read_text(encoding="utf-8")
        except OSError:
            continue
        for line_num, line in enumerate(content.splitlines(), 1):
            m = EXPORTED_FUNC_RE.match(line)
            if m:
                rm = FUNC_RECV_RE.match(line)
                kind = "func"
                if rm and rm.group("recvtype")[:1].islower():
                    kind = "method_unexported"
                symbols.append(
                    (
                        go_file,
                        line_num,
                        m.group(1),
                        pkg_dir,
                        kind,
                        rm.group("recvtype") if rm else None,
                    )
                )
                continue
            m = EXPORTED_TYPE_RE.match(line)
            if m:
                symbols.append((go_file, line_num, m.group(1), pkg_dir, "type", None))

    if not symbols:
        return []

    findings: list[Finding] = []
    # scripts/ is a legitimate caller domain: the //go:build ignore gates under
    # scripts/checks (cli_grammar.go, command_ownership.go) call exported helpers
    # (grammar.CheckSiblings/CheckNode/CheckRootNamespace/ExemptCategory). They are
    # excluded from normal Go builds but are real callers, so a symbol wired only
    # through a gate must not read as dead.
    search_dirs = [
        d for d in ("internal", "cmd", "pkg", "scripts") if (root / d).is_dir()
    ]
    if not search_dirs:
        return findings

    registered_methods = {
        pkg_dir: _registered_interface_methods_by_receiver(root, pkg_dir)
        for pkg_dir in {symbol[3] for symbol in symbols}
    }
    for go_file, line_num, sym, pkg_dir, kind, receiver in symbols:
        # *ForTest helpers exist to be called by tests in other packages; the
        # caller search excludes test files by design, so they would always be
        # flagged. The naming convention declares the test-only intent.
        if sym.endswith("ForTest"):
            continue

        if _has_cross_pkg_ref(root, sym, pkg_dir, search_dirs):
            continue

        # A type may be reached without ever spelling its bare name in another
        # package: callers switch on its constants (RouteVerbInstall), read it
        # through a struct field (inv.CPU, cap.Families), take it from a wired
        # constructor (ev := pkg.Global()), or pass an assignable value to a wired
        # setter (m.SetPingFactory(streamingPingFactory)). Each makes the type
        # wired even though the cross-package name grep found nothing.
        if kind == "type" and (
            any(
                _has_cross_pkg_ref(root, const, pkg_dir, search_dirs)
                for const in _exported_consts_of_type(root, pkg_dir, sym)
            )
            or _type_used_as_field_in_pkg(root, pkg_dir, sym)
            or _type_returned_by_wired_func(root, pkg_dir, sym, search_dirs)
            or _type_used_as_param_of_wired_func(root, pkg_dir, sym, search_dirs)
            or _type_embedded_in_wired_interface(
                root, pkg_dir, sym, search_dirs
            )
        ):
            continue
        # from another package, so it has no cross-package caller by construction. When
        # its name matches an exported interface method declared in the same package, it
        # is reached through interface dispatch (e.g. a pluggable transport backend an
        # external package or test implements) -- the grep sees the interface, not the
        # concrete method. Flagging it is a false positive.
        if kind == "method_unexported" and sym in _pkg_exported_interface_methods(
            root, pkg_dir
        ):
            continue

        # Generated gRPC clients call an exported service interface. The
        # concrete implementation type stays package-private and is reached
        # when startup registers it with the generated Register*Server hook.
        if (
            kind == "method_unexported"
            and receiver is not None
            and sym in registered_methods[pkg_dir].get(receiver, set())
        ):
            continue

        # External interface dispatch can make an exported receiver's methods
        # live without any caller spelling those method names. The exact
        # receiver/method list above is the auditable interface conformance
        # declaration; it must not exempt neighboring methods.
        if (
            receiver is not None
            and sym in INTERFACE_DISPATCH_METHODS.get((pkg_dir, receiver), ())
        ):
            continue

        findings.append(
            Finding(
                severity="ISSUE",
                file=go_file,
                line=line_num,
                message=f"exported symbol {sym} has no cross-package non-test caller",
            )
        )

    return findings


def check_spec_ac_completeness(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    plan_dir = root / "plan"
    if not plan_dir.is_dir():
        return findings

    for spec_file in sorted(plan_dir.glob("spec-*.md")):
        try:
            content = spec_file.read_text(encoding="utf-8")
        except OSError:
            continue

        status_m = SPEC_STATUS_RE.search(content)
        if not status_m or status_m.group(1) != "in-progress":
            continue

        in_audit = False
        lines = content.splitlines()
        for i, line in enumerate(lines, 1):
            if "### Acceptance Criteria" in line:
                in_audit = True
                continue
            if in_audit and line.startswith("### "):
                in_audit = False
                continue
            if not in_audit:
                continue

            m = AC_ROW_RE.match(line)
            if not m:
                continue
            ac_id = m.group(1).strip()
            demonstrated_by = m.group(3).strip()
            if not demonstrated_by:
                rel = spec_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"{ac_id} has empty 'Demonstrated By' column",
                    )
                )

    return findings


def check_cli_handler_coverage(root: Path, changed: list[str]) -> list[Finding]:
    cli_files = [
        f
        for f in changed
        if f.endswith(".go")
        and not f.endswith("_test.go")
        and any(f.startswith(p) for p in CLI_PATHS)
    ]
    if not cli_files:
        return []

    findings: list[Finding] = []
    test_dir = root / "test"
    if not test_dir.is_dir():
        return findings

    ci_content: str | None = None

    for cli_file in cli_files:
        full_path = root / cli_file
        if not full_path.exists():
            continue
        try:
            content = full_path.read_text(encoding="utf-8")
        except OSError:
            continue

        commands = REGISTER_RE.findall(content)
        if not commands:
            continue

        if ci_content is None:
            parts = []
            for ci_file in test_dir.rglob("*.ci"):
                try:
                    parts.append(ci_file.read_text(encoding="utf-8"))
                except OSError:
                    continue
            ci_content = "\n".join(parts)

        for cmd in commands:
            if cmd not in ci_content:
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=cli_file,
                        line=0,
                        message=f"CLI command '{cmd}' has no .ci test mentioning it",
                    )
                )

    return findings


def run_checks(root: Path, changed: list[str]) -> list[Finding]:
    findings: list[Finding] = []
    findings.extend(check_source_anchor_line_numbers(root))
    findings.extend(check_source_anchor_stale_paths(root))
    findings.extend(check_cross_package_wiring(root, changed))
    findings.extend(check_spec_ac_completeness(root))
    findings.extend(check_cli_handler_coverage(root, changed))
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Post-verify validation: catches recurring implementation mistakes."
    )
    parser.add_argument("--root", default=".", help="repository root (default: cwd)")
    parser.add_argument(
        "--changed-file",
        action="append",
        default=None,
        dest="changed_files",
        help=(
            "changed file to check (repeatable). Give it once with an empty "
            "value to declare an explicitly EMPTY changed set, which runs the "
            "three tree-wide checks and neither changed-file check. Omit it "
            "and the set comes from git diff"
        ),
    )
    args = parser.parse_args()

    try:
        root = find_repo_root(Path(args.root))
    except SystemExit:
        print(
            f"{RED}error: not in a git repository with go.mod{RESET}", file=sys.stderr
        )
        return 2

    # The flag being GIVEN selects the set, never the truthiness of the list it
    # built. `make ze-repository-tree-check` passes `--changed-file ''` to declare an
    # empty set, and an empty list is falsy: reading it as "no flag" would send
    # that target back to git diff and put both changed-file checks inside
    # `make ze-precommit-verify`, where they judge other sessions' half-written files.
    if args.changed_files is None:
        changed = changed_files(root)
    else:
        changed = [f for f in args.changed_files if f]
    findings = run_checks(root, changed)

    if not findings:
        print(f"{GREEN}ze-repository-check: all checks passed{RESET}")
        return 0

    issues = [f for f in findings if f.severity == "ISSUE"]
    warns = [f for f in findings if f.severity == "WARN"]

    if issues:
        print(f"{RED}ze-repository-check: {len(issues)} issue(s) found{RESET}")
        for f in issues:
            print(f"  {RED}{f}{RESET}")

    if warns:
        print(f"{YELLOW}ze-repository-check: {len(warns)} warning(s){RESET}")
        for f in warns:
            print(f"  {YELLOW}{f}{RESET}")

    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
