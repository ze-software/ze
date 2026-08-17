#!/usr/bin/env python3
"""Render contrib/netlab/ with a real netlab and compare against the golden files.

`contrib/netlab/` mirrors the artifacts a netlab daemon integration needs: the
daemon definition, the Jinja2 templates that emit ze configuration, and one
reference topology. The mirror lives beside the config syntax it emits so a
syntax change breaks a check here rather than a user's lab.

What this does, in order:

1. Build a scratch lab directory. `contrib/netlab/ze.yml` becomes
   `topology-defaults.yml` under a `daemons: ze:` key, and `contrib/netlab/ze/`
   becomes `templates/ze/`. netlab reads both without anything being installed
   into its package tree (`netsim/defaults/paths.yml`: `templates.dirs` starts
   with `topology:templates` and `.`, and `netsim/utils/read.py` USER_DEFAULTS
   starts with `./topology-defaults.yml`). The operator's netlab install is
   never written to.
2. Run `netlab create`, which renders every node's configuration.
3. Compare each rendered config against `contrib/netlab/golden/<node>.conf`.
4. Run `ze config validate` on each golden file, so the mirror cannot hold
   config text the daemon would reject.

Steps 2 and 3 are the drift check. Step 4 is what makes the golden evidence
rather than a snapshot of whatever the template happened to emit.

Absence of netlab is a FAILURE, never a skip: a check that passes when its
dependency is missing reports "no drift" about a render it never performed.
Run it through `make ze-netlab-render-check`.

    --update   rewrite the golden files from this run (review the diff)
"""

from __future__ import annotations

import argparse
import difflib
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import NoReturn

REPO = Path(__file__).resolve().parents[2]
CONTRIB = REPO / "contrib" / "netlab"
GOLDEN = CONTRIB / "golden"


def fail(message: str, *hints: str) -> NoReturn:
    print(f"error: {message}", file=sys.stderr)
    for hint in hints:
        print(f"  {hint}", file=sys.stderr)
    sys.exit(1)


def find_netlab() -> Path:
    """Locate the netlab executable, failing loudly when it is absent."""
    override = os.environ.get("NETLAB")
    if override:
        path = Path(override)
        if not path.is_file() or not os.access(path, os.X_OK):
            fail(f"NETLAB={override} is not an executable file")
        return path
    found = shutil.which("netlab")
    if not found:
        fail(
            "netlab not found on PATH",
            "This check renders the contrib/netlab templates with a real netlab;",
            "it cannot verify them without one, and it will not pass silently.",
            "Install it with: pip install networklab",
            "Or point at an existing install: make ze-netlab-render-check NETLAB=/path/to/netlab",
        )
    return Path(found)


def netlab_python(netlab: Path) -> Path:
    """The interpreter that owns the netlab install.

    netlab depends on PyYAML, so its own interpreter is guaranteed to have it.
    The system python3 running this script is not.
    """
    candidate = netlab.parent / "python3"
    if candidate.is_file():
        return candidate
    return Path(sys.executable)


def write_daemon_defaults(python: Path, source: Path, target: Path) -> None:
    """Wrap contrib/netlab/ze.yml as `daemons: ze: ...` in topology-defaults.yml.

    Re-serialized through PyYAML rather than indented as text: the source file
    opens with a `---` document marker, which is not legal once indented.
    """
    script = (
        "import sys, yaml, pathlib\n"
        "src, dst = sys.argv[1], sys.argv[2]\n"
        "d = yaml.safe_load(pathlib.Path(src).read_text())\n"
        "pathlib.Path(dst).write_text(yaml.safe_dump({'daemons': {'ze': d}}, sort_keys=False))\n"
    )
    result = subprocess.run(
        [str(python), "-c", script, str(source), str(target)],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        fail(f"could not read {source}", result.stderr.strip())


def build_lab(python: Path, lab: Path) -> None:
    (lab / "templates").mkdir(parents=True, exist_ok=True)
    write_daemon_defaults(python, CONTRIB / "ze.yml", lab / "topology-defaults.yml")
    shutil.copytree(CONTRIB / "ze", lab / "templates" / "ze")
    shutil.copy(CONTRIB / "topology.yml", lab / "topology.yml")


def run_netlab_create(netlab: Path, lab: Path) -> None:
    result = subprocess.run(
        [str(netlab), "create"],
        cwd=lab,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0 or "Errors encountered" in result.stdout:
        print(result.stdout, file=sys.stderr)
        print(result.stderr, file=sys.stderr)
        fail(
            "`netlab create` failed on contrib/netlab/topology.yml",
            "Every module the topology declares needs a daemon_config key in",
            "contrib/netlab/ze.yml and a template in contrib/netlab/ze/.",
        )


def rendered_configs(lab: Path) -> dict[str, str]:
    """The ze configuration netlab rendered for each node.

    contrib/netlab/ze.yml maps the `ze` daemon_config key to /etc/ze/ze.conf,
    and the clab provider writes that file as node_files/<node>/ze.
    """
    node_files = lab / "node_files"
    if not node_files.is_dir():
        fail(f"netlab create wrote no {node_files}")
    out: dict[str, str] = {}
    for node_dir in sorted(node_files.iterdir()):
        config = node_dir / "ze"
        if config.is_file():
            out[node_dir.name] = config.read_text()
    if not out:
        fail(
            "netlab create rendered no ze configuration",
            "contrib/netlab/ze.yml must map the `ze` daemon_config key to a real file.",
        )
    return out


def ze_binary() -> Path | None:
    """The ze binary to validate the golden files with, when one is available."""
    override = os.environ.get("ZE_BIN")
    if override:
        path = Path(override)
        if not path.is_file():
            fail(f"ZE_BIN={override} does not exist")
        return path
    for candidate in (REPO / "bin" / "ze",):
        if candidate.is_file():
            return candidate
    found = shutil.which("ze")
    return Path(found) if found else None


def validate_golden(ze: Path, names: list[str]) -> int:
    problems = 0
    for name in names:
        path = GOLDEN / f"{name}.conf"
        result = subprocess.run(
            [str(ze), "config", "validate", str(path)],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            problems += 1
            print(f"FAIL: {path} is not valid ze configuration", file=sys.stderr)
            print(result.stdout + result.stderr, file=sys.stderr)
        else:
            print(f"ok: {path.relative_to(REPO)} validates")
    return problems


def compare(rendered: dict[str, str], update: bool) -> int:
    GOLDEN.mkdir(parents=True, exist_ok=True)
    problems = 0
    for name, text in rendered.items():
        path = GOLDEN / f"{name}.conf"
        if update:
            path.write_text(text)
            print(f"updated {path.relative_to(REPO)}")
            continue
        if not path.is_file():
            problems += 1
            print(f"FAIL: no golden file for node {name} at {path}", file=sys.stderr)
            print(text, file=sys.stderr)
            continue
        want = path.read_text()
        if want != text:
            problems += 1
            print(
                f"FAIL: {path.relative_to(REPO)} does not match the render",
                file=sys.stderr,
            )
            diff = difflib.unified_diff(
                want.splitlines(keepends=True),
                text.splitlines(keepends=True),
                fromfile=f"golden/{name}.conf",
                tofile=f"rendered/{name}",
            )
            sys.stderr.writelines(diff)
        else:
            print(f"ok: {path.relative_to(REPO)} matches the render")

    stale = {p.stem for p in GOLDEN.glob("*.conf")} - set(rendered)
    for name in sorted(stale):
        problems += 1
        print(
            f"FAIL: golden/{name}.conf has no node in contrib/netlab/topology.yml",
            file=sys.stderr,
        )
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--update",
        action="store_true",
        help="rewrite contrib/netlab/golden/ from this run",
    )
    args = parser.parse_args()

    netlab = find_netlab()
    python = netlab_python(netlab)
    print(f"netlab: {netlab}")

    with tempfile.TemporaryDirectory(prefix="ze-netlab-render-") as tmp:
        lab = Path(tmp) / "lab"
        build_lab(python, lab)
        run_netlab_create(netlab, lab)
        rendered = rendered_configs(lab)

    problems = compare(rendered, args.update)

    ze = ze_binary()
    if ze is None:
        fail(
            "no ze binary to validate the golden files with",
            "Run `make ze-build` first, or set ZE_BIN=/path/to/ze.",
            "The render is only evidence if the daemon accepts what it produced.",
        )
    problems += validate_golden(ze, sorted(rendered))

    if problems:
        print(
            f"\nze-netlab-render-check FAILED ({problems} problem(s))", file=sys.stderr
        )
        if not args.update:
            print(
                "If the render is right and the golden is stale, rerun with --update "
                "and review the diff.",
                file=sys.stderr,
            )
        return 1
    print(f"\nze-netlab-render-check OK ({len(rendered)} node(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
