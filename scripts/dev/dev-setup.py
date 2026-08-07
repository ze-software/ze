#!/usr/bin/env python3
"""Unified dev setup: install all tools a Ze dev/test workflow needs.

Replaces the old ze-setup-build / ze-setup-lint / ze-setup Makefile chain
with a single Python script that handles OS detection, probing, and
installing all build, lint, and appliance/evidence dependencies.

Usage:
    make ze-setup              # install missing tools + vendor Go deps
    make ze-setup CHECK=1      # probe only, exit nonzero if required tools missing
"""

from __future__ import annotations

import argparse
import getpass
import glob
import grp
import json
import os
import platform
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]

# DRIFT-GUARD: appliance check names from applianceDoctorChecks()
# in internal/appliance/doctor_checks.go. The Go drift test
# (dev_setup_drift_test.go) parses this dict and verifies every
# installable check name from applianceDoctorChecks() appears here.
# Do NOT rename this variable or change its format without updating the test.
APPLIANCE_CHECKS = {
    "appliance-grub": {
        "brew": None,
        "apt": "grub-efi-amd64-bin",
        "probe": ["grub-mkstandalone", "grub2-mkstandalone"],
        "note": "no first-party Homebrew formula; macOS skips grub (ISO builds are Linux/container-only)",
    },
    "appliance-xorriso": {
        "brew": "xorriso",
        "apt": "xorriso",
        "probe": ["xorriso"],
    },
    "appliance-e2fsprogs": {
        "brew": "e2fsprogs",
        "apt": "e2fsprogs",
        "probe": ["mkfs.ext4", "debugfs"],
        "note": "keg-only on macOS; Go code resolves via Cellar glob, no PATH change needed",
    },
}


@dataclass
class Tool:
    name: str
    probe: list[str]
    brew: str | None = None
    apt: str | None = None
    go_install: str | None = None
    pipx_install: str | None = None
    required: bool = True
    note: str = ""
    probe_any: bool = False


REQUIRED_TOOLS: list[Tool] = [
    Tool(name="go", probe=["go"], brew="go", apt="golang-go"),
    Tool(name="git", probe=["git"], brew="git", apt="git"),
    Tool(name="protobuf", probe=["protoc"], brew="protobuf", apt="protobuf-compiler"),
    Tool(name="jq", probe=["jq"], brew="jq", apt="jq"),
    Tool(
        name="golangci-lint",
        probe=["golangci-lint"],
        go_install="github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1",
    ),
    Tool(
        name="goimports",
        probe=["goimports"],
        go_install="golang.org/x/tools/cmd/goimports@latest",
    ),
    Tool(
        name="gopls",
        probe=["gopls"],
        go_install="golang.org/x/tools/gopls@latest",
        note=(
            "language server behind the agent LSP tool; without it every LSP call"
            " returns ENOENT and the session reads whole files instead"
            " (ai/rules/context-economy.md)"
        ),
    ),
    Tool(name="python3", probe=["python3"], brew="python", apt="python3"),
    Tool(
        name="qemu",
        probe=["qemu-system-x86_64", "qemu-system-aarch64"],
        probe_any=True,
        brew="qemu",
        apt="qemu-system-x86",
    ),
    Tool(
        name="e2fsprogs",
        probe=["mkfs.ext4", "debugfs"],
        probe_any=True,
        brew="e2fsprogs",
        apt="e2fsprogs",
        note="keg-only on macOS; Go code resolves via Cellar glob",
    ),
    Tool(name="xorriso", probe=["xorriso"], brew="xorriso", apt="xorriso"),
    Tool(
        name="grub",
        probe=["grub-mkstandalone", "grub2-mkstandalone"],
        probe_any=True,
        brew=None,
        apt="grub-efi-amd64-bin",
        note="no first-party Homebrew formula; macOS skips (ISO builds are Linux/container-only)",
    ),
    Tool(name="pipx", probe=["pipx"], brew="pipx", apt="pipx"),
    # uv installs through pipx on BOTH platforms, and the reason is Linux: uv is
    # not in the Debian or Ubuntu repositories, so `apt=None` left a REQUIRED
    # tool with no package there. `_has_package` answers False for that state
    # and the loop prints `[skipped]`, so the tool the evidence SSH probe needs
    # (`uv run --with paramiko`) went missing without ever reaching
    # `missing_required`: check mode said "All required tools present" on a box
    # that had no uv. A guard that reports green on absence is worse than none.
    #
    # pipx over brew on macOS, and over the curl installer everywhere: one route
    # is one thing to fix, and `curl | sh` is a supply-chain hole this script
    # would be opening on every dev machine. It must come after the pipx row --
    # install_tool skips a pipx tool when pipx is not there yet.
    Tool(
        name="uv",
        probe=["uv"],
        pipx_install="uv",
        note="not in apt repos, so pipx is the one route that works on both platforms",
    ),
    Tool(name="ruff", probe=["ruff"], pipx_install="ruff"),
    Tool(
        name="pyright",
        probe=["pyright", "pyright-langserver"],
        pipx_install="pyright",
        note=(
            "language server for the Python half of the tree; gopls answers a"
            " symbol question about a .go file, pyright answers the same"
            " question about scripts/dev/*.py and the hooks"
            " (ai/rules/context-economy.md)"
        ),
    ),
]

OPTIONAL_TOOLS: list[Tool] = [
    Tool(
        name="sshpass",
        probe=["sshpass"],
        brew="sshpass",
        apt="sshpass",
        required=False,
        note="SSH-probe fallback only; uv+paramiko is primary",
    ),
    Tool(
        name="docker",
        probe=["docker"],
        brew="docker",
        apt="docker.io",
        required=False,
        note="container appliance/kernel builds",
    ),
    Tool(
        name="colima",
        probe=["colima"],
        brew="colima",
        required=False,
        note="macOS Docker runtime",
    ),
    Tool(
        name="xl2tpd",
        probe=["xl2tpd"],
        brew=None,
        apt="xl2tpd",
        required=False,
        note="Linux root-only L2TP LAC peer for L2TP PPP evidence tests (ze-deployment-l2tp-ppp-test, ze-deployment-gokrazy-l2tp-ppp-test)",
    ),
    Tool(
        name="ppp",
        probe=["pppd"],
        brew=None,
        apt="ppp",
        required=False,
        note="Linux root-only pppd for the same L2TP PPP/NCP evidence tests",
    ),
]


# --- Language server: gopls must ANSWER, not merely exist -----------------
#
# `gopls` on PATH is not the same thing as a working language server, and the
# difference is invisible until an LSP call is made. On this machine gopls was
# absent for weeks while the BLOCKING "load LSP first" rule was satisfied every
# session, because that gate lifts on the query text and no call was ever made:
# every one would have returned ENOENT. A presence check would not have caught
# it either. So this check RUNS the server and requires an answer.
#
# The probe file is internal/core/clock/clock.go: 128 lines, and its package
# imports `time` and nothing else, which is the smallest dependency footprint
# in the repository. gopls must load and type-check the package to answer, so a
# probe on a package with a wide import graph would measure the graph rather
# than the server.
GOPLS_PROBE_FILE = "internal/core/clock/clock.go"

# gopls type-checks the package before it answers, so the first run on a cold
# Go build cache pays for the standard library too. Measured warm on this
# machine: 3.4s. The timeout is ~35x that, which leaves a cold cache room
# without letting a hung server hold `make ze-setup` open indefinitely. A check
# that reds spuriously is a check somebody disables.
GOPLS_PROBE_TIMEOUT = 120

# One line of `gopls symbols` output: a name, a symbol kind, and a range.
GOPLS_SYMBOL_LINE = re.compile(r"\b[A-Z][A-Za-z]+\s+\d+:\d+-\d+:\d+")

GOPLS_NOT_INSTALLED = "gopls is not installed; the gopls row above installs it"
GOPLS_NOT_ANSWERING = "gopls is present but not answering"


def gopls_probe(timeout: int = GOPLS_PROBE_TIMEOUT) -> subprocess.CompletedProcess:
    """Ask the language server for the symbols of one small file.

    Split out from `gopls_status` so a unit test can fake the answer instead of
    shelling out to a real server it cannot control.
    """
    return subprocess.run(
        ["gopls", "symbols", GOPLS_PROBE_FILE],
        cwd=str(REPO_ROOT),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=timeout,
    )


def gopls_status(probe=gopls_probe) -> tuple[str, str]:
    """Whether the language server answers, and why not when it does not.

    Returns a state and a message written for whoever has to fix it:

    "ok"       -- the server answered with symbols.
    "absent"   -- no gopls on PATH. A DIFFERENT problem with a different fix,
                  and the Tool row above already installs it.
    "broken"   -- gopls ran and gave nothing usable: a timeout, a non-zero
                  exit, or output with no symbol in it. Typically a broken
                  module cache, a package that does not build, or a version
                  mismatch -- none of which installing gopls again repairs.
    "na"       -- the probe file is not there, so this is not a Ze checkout and
                  there is nothing to ask about.
    """
    if shutil.which("gopls") is None:
        return ("absent", GOPLS_NOT_INSTALLED)
    if not (REPO_ROOT / GOPLS_PROBE_FILE).is_file():
        return ("na", f"no {GOPLS_PROBE_FILE} to probe")
    try:
        result = probe()
    except subprocess.TimeoutExpired:
        return (
            "broken",
            f"{GOPLS_NOT_ANSWERING}: no reply within {GOPLS_PROBE_TIMEOUT}s on"
            f" {GOPLS_PROBE_FILE}",
        )
    except OSError as err:
        return ("broken", f"{GOPLS_NOT_ANSWERING}: {err}")
    out = result.stdout.decode(errors="replace")
    if result.returncode != 0:
        detail = result.stderr.decode(errors="replace").strip().splitlines()
        why = detail[0] if detail else f"exit {result.returncode}"
        return ("broken", f"{GOPLS_NOT_ANSWERING}: {why}")
    symbols = [line for line in out.splitlines() if GOPLS_SYMBOL_LINE.search(line)]
    if not symbols:
        return (
            "broken",
            f"{GOPLS_NOT_ANSWERING}: no symbol in its reply for {GOPLS_PROBE_FILE}",
        )
    return ("ok", f"{len(symbols)} symbols from {GOPLS_PROBE_FILE}")


# The Python half of the same story. `scripts/dev/*.py` and `.claude/hooks/*.py`
# were read 405 times in the measured transcript store with no symbol server on
# the machine at all, so every one of those reads was a whole file where a
# symbol was the question (ai/rules/context-economy.md).
#
# The probe file is this script. `gopls_status` carries an "n/a" state for the
# day its probe file is renamed out from under it; a probe that names the
# running script cannot reach that state, so `pyright_status` does not carry it.
# Repointing this at another file therefore means restoring that branch.
PYRIGHT_PROBE_FILE = "scripts/dev/dev-setup.py"

# Measured warm on this machine: 0.5s. The first run after install downloads a
# node runtime, which is the slow case this budget is for. Same reasoning as
# GOPLS_PROBE_TIMEOUT: generous enough that a cold run never reds spuriously.
PYRIGHT_PROBE_TIMEOUT = 120

PYRIGHT_NOT_INSTALLED = "pyright is not installed; the pyright row above installs it"
PYRIGHT_NOT_ANSWERING = "pyright is present but not answering"


def pyright_probe(timeout: int = PYRIGHT_PROBE_TIMEOUT) -> subprocess.CompletedProcess:
    """Ask the language server to analyse one file and report what it did.

    Split out from `pyright_status` for the same reason `gopls_probe` is: a unit
    test fakes the answer rather than shelling out to a real server.
    """
    return subprocess.run(
        ["pyright", "--outputjson", PYRIGHT_PROBE_FILE],
        cwd=str(REPO_ROOT),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=timeout,
    )


def pyright_summary(out: str) -> dict | None:
    """The analysis summary from a reply that may carry a bootstrap preamble.

    `json.loads(out)` was wrong here, and it was wrong on exactly one run: the
    first. With no global node on PATH the pipx wrapper installs one, and
    `_install_node_env` (`pyright/node.py`) runs nodeenv through
    `subprocess.run(args, check=True)` with no redirection, so nodeenv's
    progress lands on the stdout this probe captured. Only the npm path is
    silenced (`install_pyright`, `pyright/_utils.py`, `silent = '--outputjson'
    in args`). The reply is then valid JSON with text in front of it, the whole
    decode fails, and `make ze-setup` reds on the fresh Linux box it exists to
    prepare. The second run is green, which makes it read as flakiness.

    Returns None when no JSON object in `out` carries a summary. The preamble
    is not itself valid JSON (nodeenv prints a Python dict repr, single quotes),
    so scanning is unambiguous rather than a guess at where the reply starts.
    """
    decoder = json.JSONDecoder()
    start = out.find("{")
    while start != -1:
        try:
            value, _ = decoder.raw_decode(out, start)
        except ValueError:
            value = None
        if isinstance(value, dict) and isinstance(value.get("summary"), dict):
            return value["summary"]
        start = out.find("{", start + 1)
    return None


def pyright_status(probe=pyright_probe) -> tuple[str, str]:
    """Whether the language server answers, and why not when it does not.

    Three states, not the four `gopls_status` has: `na` is absent because
    PYRIGHT_PROBE_FILE is this script, which cannot be missing while this
    function runs. Repoint the probe at another file and the state comes back
    with it (`test_the_probe_file_is_this_script`).

    One difference from gopls decides the rest of the implementation: pyright
    exits 1 when it finds a type error, so the exit code says whether the CODE
    is clean, never whether the SERVER worked. A check keyed on it would go red
    the first time somebody's script gained a diagnostic, and a check that reds
    spuriously is a check somebody disables. `summary.filesAnalyzed` is the
    field that answers the question asked here.
    """
    if shutil.which("pyright") is None:
        return ("absent", PYRIGHT_NOT_INSTALLED)
    try:
        result = probe()
    except subprocess.TimeoutExpired:
        return (
            "broken",
            f"{PYRIGHT_NOT_ANSWERING}: no reply within {PYRIGHT_PROBE_TIMEOUT}s on"
            f" {PYRIGHT_PROBE_FILE}",
        )
    except OSError as err:
        return ("broken", f"{PYRIGHT_NOT_ANSWERING}: {err}")
    summary = pyright_summary(result.stdout.decode(errors="replace"))
    try:
        analyzed = int(summary["filesAnalyzed"])
    except (ValueError, KeyError, TypeError):
        detail = result.stderr.decode(errors="replace").strip().splitlines()
        why = (
            detail[0] if detail else f"no summary in its reply for {PYRIGHT_PROBE_FILE}"
        )
        return ("broken", f"{PYRIGHT_NOT_ANSWERING}: {why}")
    if analyzed < 1:
        return (
            "broken",
            f"{PYRIGHT_NOT_ANSWERING}: analysed no file for {PYRIGHT_PROBE_FILE}",
        )
    return ("ok", f"{analyzed} file analysed from {PYRIGHT_PROBE_FILE}")


# --- Root: one route, and it never waits for a password nobody can type ---
#
# Three things here need root, all of them on Linux: `apt-get`, the sysctl
# drop-in below, and the `usermod` that grants KVM. They reached it three
# different ways, and two of those could hang forever. `sudo` reads its
# password prompt from the stdin it inherits, so a run with no terminal waits
# for an answer that is never coming, and `sudo tee` fed a config line on stdin
# hands that line to the prompt instead of to the file.
#
# So privilege is decided BEFORE a command runs, and sudo is always given `-n`.
# A password is asked for once, by `sudo -v`, and only when a terminal is
# attached to answer it. Every other case prints the command for a human and
# returns False. A setup script that says what to run is recoverable; one that
# blocks with no output is not.

# `sudo -n true` touches no network and reads a local timestamp file. A second
# of that is already pathological, so this bound only exists to stop a wedged
# sudo (an unreachable LDAP sudoers source, typically) from holding the run.
SUDO_PROBE_TIMEOUT = 15


def privilege_mode() -> str:
    """How this process can run a root command right now.

    "root"        -- already root. No sudo is used, so none needs to be there.
    "sudo"        -- sudo acts with no password: NOPASSWD, or a live timestamp.
    "sudo-prompt" -- sudo wants a password and a terminal is attached to type
                     it on.
    "none"        -- no route to root that would not block. The caller prints
                     the command instead of running it.
    """
    if os.geteuid() == 0:
        return "root"
    if shutil.which("sudo") is None:
        return "none"
    try:
        probe = subprocess.run(
            ["sudo", "-n", "true"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=SUDO_PROBE_TIMEOUT,
        )
    except (OSError, subprocess.TimeoutExpired):
        return "none"
    if probe.returncode == 0:
        return "sudo"
    return "sudo-prompt" if sys.stdin.isatty() else "none"


def run_privileged(
    argv: list[str],
    stdin: bytes | None = None,
    shown: str | None = None,
    env: dict[str, str] | None = None,
) -> tuple[bool, str]:
    """Run one command as root. Returns (ok, detail).

    `shown` overrides the echoed command for a caller whose argv is not what a
    human would copy: the sysctl drop-in is written by piping into `tee`, and
    printing the argv alone would show a `tee` with no content.

    `detail` is written for whoever has to fix it: the command's own first line
    of complaint, or the reason root was out of reach.
    """
    mode = privilege_mode()
    label = shown or " ".join(argv if mode == "root" else ["sudo", *argv])
    if mode == "none":
        return (False, f"no password-free route to root for `{label}`")
    if mode == "sudo-prompt":
        print("  sudo needs your password")
        if subprocess.run(["sudo", "-v"]).returncode != 0:
            return (False, f"sudo could not authenticate for `{label}`")
    # `-n` even after `sudo -v` has cached the timestamp: the prompt is what
    # eats a piped stdin, and this way no code path can reach one.
    full = argv if mode == "root" else ["sudo", "-n", *argv]
    print(f"  Run: {label}")
    try:
        result = subprocess.run(
            full,
            input=stdin,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            env=env,
        )
    except OSError as err:
        return (False, f"{label}: {err}")
    if result.returncode != 0:
        complaint = result.stderr.decode(errors="replace").strip().splitlines()
        why = complaint[0] if complaint else f"exit {result.returncode}"
        return (False, f"{label}: {why}")
    return (True, label)


# --- Linux system tunable: unprivileged user namespaces -------------------
#
# Ubuntu 23.10+ ships kernel.apparmor_restrict_unprivileged_userns=1, which
# blocks the user-namespace sandbox Chrome relies on. The agent-browser web
# functional tests then cannot launch Chrome ("No usable sandbox!"), so the
# restriction must be lifted globally. This is a kernel tunable, not a binary,
# so it lives outside the Tool machinery.
USERNS_SYSCTL = "kernel.apparmor_restrict_unprivileged_userns"
USERNS_PROC = Path("/proc/sys/kernel/apparmor_restrict_unprivileged_userns")
USERNS_CONF = "/etc/sysctl.d/60-ze-userns.conf"


def userns_status() -> str:
    """Report the unprivileged-userns restriction state.

    Returns "ok" (allowed, value 0), "restricted" (blocked, value 1), or
    "na" (kernel has no such knob -- nothing to do, e.g. non-AppArmor host).
    """
    if not USERNS_PROC.exists():
        return "na"
    try:
        return "restricted" if USERNS_PROC.read_text().strip() == "1" else "ok"
    except OSError:
        return "na"


def print_userns_fix() -> None:
    """Print the commands that lift the restriction persistently.

    Used as the manual fallback when the automatic apply cannot run sudo.
    """
    print(f'  Run: echo "{USERNS_SYSCTL} = 0" | sudo tee {USERNS_CONF}')
    print(f"  Run: sudo sysctl -w {USERNS_SYSCTL}=0")


def apply_userns_fix() -> bool:
    """Print, then run, the commands that lift the restriction persistently.

    Writes the /etc/sysctl.d drop-in (survives reboot) and applies the value
    live, through `run_privileged`. Each command is echoed before it runs.
    Returns True only when the restriction is actually cleared; on any failure
    (e.g. sudo needs a password and no terminal is attached to type it on) it
    returns False so the caller can fall back to printing the manual commands.
    """
    steps = [
        (
            ["tee", USERNS_CONF],
            f"{USERNS_SYSCTL} = 0\n".encode(),
            f'echo "{USERNS_SYSCTL} = 0" | sudo tee {USERNS_CONF}',
        ),
        (["sysctl", "-w", f"{USERNS_SYSCTL}=0"], None, None),
    ]
    for argv, stdin, shown in steps:
        ok, detail = run_privileged(argv, stdin=stdin, shown=shown)
        if not ok:
            print(f"  FAIL: {detail}")
            return False
    return userns_status() == "ok"


# --- Linux system state: KVM device access -------------------------------
#
# QEMU-backed evidence (appliance boot proofs, the ze-qemu-* targets) runs under
# KVM when it can. /dev/kvm is root:kvm 0660, so the invoking user must be in the
# kvm group; without it qemu does not fall back, it dies with "Could not access
# KVM kernel module: Permission denied" and the caller reports a timeout. Group
# membership is neither a binary nor a kernel tunable, so like the userns knob it
# lives outside the Tool machinery.
#
# Linux only. macOS has no /dev/kvm: QEMU uses the Apple hypervisor (hvf), which
# needs no group, and the evidence scripts select it by sys.platform.
KVM_DEV = Path("/dev/kvm")
KVM_GROUP = "kvm"


def kvm_status() -> str:
    """Report whether QEMU can use KVM as this user.

    "ok"            -- /dev/kvm is readable and writable in this process now.
    "pending-login" -- the user IS in the kvm group but the running session
                       predates that; group membership is fixed at login.
    "no-group"      -- the device exists and the user is not in the group.
    "na"            -- no /dev/kvm at all (no hardware virt, or a VM without
                       nested virt). QEMU still runs under tcg, only slower,
                       so there is nothing to fix.
    """
    if not KVM_DEV.exists():
        return "na"
    if os.access(KVM_DEV, os.R_OK | os.W_OK):
        return "ok"
    return "pending-login" if in_kvm_group() else "no-group"


def in_kvm_group() -> bool:
    """True when the user is listed in the kvm group in the group database.

    Deliberately not os.access: after `usermod -aG` the database says yes while
    every already-running process still says no, and telling those two states
    apart is the difference between "run this command" and "log back in".
    """
    try:
        return getpass.getuser() in grp.getgrnam(KVM_GROUP).gr_mem
    except KeyError:
        return False


def print_kvm_fix() -> None:
    """Print the commands that grant KVM access."""
    print(f"  Run: sudo usermod -aG {KVM_GROUP} {getpass.getuser()}")
    print(
        "  Then log out and back in, or prefix a command with:"
        f" sg {KVM_GROUP} -c '<command>'"
    )


def apply_kvm_fix() -> bool:
    """Add the invoking user to the kvm group via sudo.

    Returns True when the group database lists the user afterwards. That is NOT
    the same as usable: this process keeps the groups it was started with, so
    the caller must still tell the user to log back in.
    """
    ok, detail = run_privileged(["usermod", "-aG", KVM_GROUP, getpass.getuser()])
    if not ok:
        print(f"  FAIL: {detail}")
        return False
    return in_kvm_group()


def detect_os() -> str | None:
    if platform.system() == "Darwin":
        if shutil.which("brew"):
            return "brew"
        return None
    if platform.system() == "Linux":
        if shutil.which("apt-get"):
            return "apt"
        return None
    return None


def probe_tool(tool: Tool) -> bool:
    if tool.name == "e2fsprogs" and platform.system() == "Darwin":
        return _probe_e2fsprogs_macos()
    if tool.probe_any:
        return any(shutil.which(p) is not None for p in tool.probe)
    return all(shutil.which(p) is not None for p in tool.probe)


def _probe_e2fsprogs_macos() -> bool:
    cellar = glob.glob("/opt/homebrew/Cellar/e2fsprogs/*/sbin")
    if cellar:
        latest = sorted(cellar)[-1]
        if (Path(latest) / "mkfs.ext4").is_file() and (
            Path(latest) / "debugfs"
        ).is_file():
            return True
    for d in ["/opt/homebrew/sbin", "/usr/sbin", "/sbin"]:
        if (Path(d) / "mkfs.ext4").is_file() and (Path(d) / "debugfs").is_file():
            return True
    return False


def install_tool(tool: Tool, pkg_mgr: str) -> bool:
    if tool.go_install:
        if not shutil.which("go"):
            print(f"  SKIP {tool.name}: go not available yet")
            return False
        print(f"  go install {tool.go_install}")
        r = subprocess.run(
            ["go", "install", tool.go_install],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        if r.returncode != 0:
            print(f"  FAIL {tool.name}: {r.stdout.decode().strip()}")
            return False
        return True

    if tool.pipx_install:
        if not shutil.which("pipx"):
            print(f"  SKIP {tool.name}: pipx not available yet")
            return False
        print(f"  pipx install {tool.pipx_install}")
        r = subprocess.run(
            ["pipx", "install", "--force", tool.pipx_install],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        if r.returncode != 0:
            print(f"  FAIL {tool.name}: {r.stdout.decode().strip()}")
            return False
        return True

    if pkg_mgr == "brew":
        pkg = tool.brew
        if pkg is None:
            if tool.note:
                print(f"  SKIP {tool.name}: {tool.note}")
            return False
        print(f"  brew install {pkg}")
        r = subprocess.run(
            ["brew", "install", pkg],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        if r.returncode != 0:
            print(f"  FAIL {tool.name}: {r.stdout.decode().strip()}")
            return False
        return True

    if pkg_mgr == "apt":
        pkg = tool.apt
        if pkg is None:
            if tool.note:
                print(f"  SKIP {tool.name}: {tool.note}")
            return False
        return apt_install(pkg, tool.name)

    return False


# One `apt-get update` per run, taken before the first install. A container
# image ships no package lists at all, and an install against an empty list
# fails with "Unable to locate package <x>" -- which reads as a wrong package
# name, not a missing index. Per-tool it would be ~20s of network each, for an
# index that does not change during one run.
_apt_updated = False


def apt_install(pkg: str, name: str) -> bool:
    """Install one Debian package, or print the command when root is out of reach.

    `DEBIAN_FRONTEND=noninteractive` rides in the argv rather than in `env=`
    because sudo resets the environment (`env_reset` is the default in
    sudoers), so an exported value does not reach apt-get. Without it a package
    carrying a debconf prompt stops the run dead.
    """
    global _apt_updated

    manual = f"sudo apt-get install -y {pkg}"
    if privilege_mode() == "none":
        print(f"  Run: {manual}")
        return False

    if not _apt_updated:
        # One attempt, success or not: a stale index still installs most
        # packages, and re-running a failing update per tool helps nobody.
        _apt_updated = True
        ok, detail = run_privileged(["apt-get", "update"])
        if not ok:
            print(f"  WARN apt-get update: {detail}")

    ok, detail = run_privileged(
        ["env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", pkg]
    )
    if not ok:
        print(f"  FAIL {name}: {detail}")
        print(f"  Run: {manual}")
        return False
    return True


def vendor_go_deps() -> bool:
    if not shutil.which("go"):
        print("  SKIP vendoring: go not available")
        return False
    print("  go mod tidy && go mod vendor")
    for cmd in [["go", "mod", "tidy"], ["go", "mod", "vendor"]]:
        r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if r.returncode != 0:
            print(f"  FAIL {' '.join(cmd)}: {r.stdout.decode().strip()}")
            return False
    return True


def _has_package(tool: Tool, pkg_mgr: str) -> bool:
    if tool.go_install or tool.pipx_install:
        return True
    if pkg_mgr == "brew":
        return tool.brew is not None
    if pkg_mgr == "apt":
        return tool.apt is not None
    return False


def print_manual_list(tools: list[Tool]) -> None:
    print("\nManual installation required. Tools needed:\n")
    print("Required:")
    for t in tools:
        if t.required:
            probes = ", ".join(t.probe)
            note = f" ({t.note})" if t.note else ""
            print(f"  {t.name}: {probes}{note}")
    print("\nOptional:")
    for t in tools:
        if not t.required:
            probes = ", ".join(t.probe)
            note = f" ({t.note})" if t.note else ""
            print(f"  {t.name}: {probes}{note}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Ze dev setup")
    parser.add_argument(
        "--check",
        action="store_true",
        help="probe only; exit nonzero if a required tool is missing",
    )
    args = parser.parse_args()

    all_tools = REQUIRED_TOOLS + OPTIONAL_TOOLS
    pkg_mgr = detect_os()

    if pkg_mgr is None:
        print("Unsupported platform: no brew (macOS) or apt (Linux) found.")
        print_manual_list(all_tools)
        return 1

    print(f"Ze dev setup (package manager: {pkg_mgr})\n")

    missing_required: list[str] = []
    pending_manual: list[str] = []
    installed: list[str] = []
    skipped: list[str] = []

    for tool in all_tools:
        present = probe_tool(tool)
        if present:
            print(f"  [present]   {tool.name}")
            continue

        if args.check:
            if not _has_package(tool, pkg_mgr):
                note = tool.note or "no package for this platform"
                print(f"  [skipped]   {tool.name} ({note})")
                skipped.append(tool.name)
            elif tool.required:
                print(f"  [missing]   {tool.name} (REQUIRED)")
                missing_required.append(tool.name)
            else:
                print(f"  [missing]   {tool.name} (optional)")
            continue

        if not _has_package(tool, pkg_mgr):
            note = tool.note or "no package for this platform"
            print(f"  [skipped]   {tool.name} ({note})")
            skipped.append(tool.name)
            continue

        ok = install_tool(tool, pkg_mgr)
        if ok:
            if probe_tool(tool):
                print(f"  [installed] {tool.name}")
                installed.append(tool.name)
            else:
                print(f"  [installed] {tool.name} (not yet on PATH)")
                installed.append(tool.name)
        elif pkg_mgr == "apt" and tool.apt is not None:
            # apt_install has already printed why, and the command to run.
            print(f"  [pending]   {tool.name} (not installed)")
            pending_manual.append(tool.name)
        else:
            if tool.required:
                missing_required.append(tool.name)
                print(f"  [MISSING]   {tool.name} (required)")
            else:
                skipped.append(tool.name)
                print(f"  [skipped]   {tool.name} (optional)")

    # Not a binary but a behaviour: gopls on PATH is not a working language
    # server, and every LSP call fails the same silent way when it is not one.
    # Runs in both modes -- an install that leaves a mute server is not a setup.
    state, detail = gopls_status()
    if state == "ok":
        print(f"  [present]   gopls-answers ({detail})")
    elif state == "na":
        print(f"  [present]   gopls-answers (n/a: {detail})")
    elif state == "absent":
        print(f"  [skipped]   gopls-answers ({detail})")
        skipped.append("gopls-answers")
    else:
        print(f"  [MISSING]   gopls-answers ({detail})")
        missing_required.append("gopls-answers")

    # The same behaviour check for the Python half of the tree. Presence proves
    # nothing here either: pyright bootstraps a node runtime on first use, so a
    # binary on PATH that never ran is exactly the state gopls was in for weeks.
    state, detail = pyright_status()
    if state == "ok":
        print(f"  [present]   pyright-answers ({detail})")
    elif state == "absent":
        print(f"  [skipped]   pyright-answers ({detail})")
        skipped.append("pyright-answers")
    else:
        print(f"  [MISSING]   pyright-answers ({detail})")
        missing_required.append("pyright-answers")

    # Linux kernel tunable (not a binary): unprivileged user namespaces must be
    # allowed or Chrome's sandbox fails, breaking the agent-browser web tests.
    if platform.system() == "Linux":
        status = userns_status()
        if status == "ok" or status == "na":
            note = "" if status == "ok" else " (n/a: no apparmor userns knob)"
            print(f"  [present]   userns-unrestricted{note}")
        elif args.check:
            print("  [missing]   userns-unrestricted (REQUIRED)")
            missing_required.append("userns-unrestricted")
        elif apply_userns_fix():
            print("  [installed] userns-unrestricted")
            installed.append("userns-unrestricted")
        else:
            print("  Could not apply automatically; run manually:")
            print_userns_fix()
            pending_manual.append("userns-unrestricted")

    # Linux system state (not a binary): QEMU-backed evidence needs the invoking
    # user in the kvm group, or qemu refuses to start instead of using tcg.
    if platform.system() == "Linux":
        status = kvm_status()
        if status == "ok":
            print("  [present]   kvm-access")
        elif status == "na":
            print("  [present]   kvm-access (n/a: no /dev/kvm; QEMU uses tcg)")
        elif status == "pending-login":
            print(
                "  [pending]   kvm-access (in the kvm group; log out and back"
                f" in, or use: sg {KVM_GROUP} -c '<command>')"
            )
            pending_manual.append("kvm-access")
        elif args.check:
            print("  [missing]   kvm-access (REQUIRED)")
            missing_required.append("kvm-access")
        elif apply_kvm_fix():
            print(
                "  [installed] kvm-access (log out and back in to pick up the"
                " new group)"
            )
            pending_manual.append("kvm-access")
        else:
            print("  Could not apply automatically; run manually:")
            print_kvm_fix()
            pending_manual.append("kvm-access")

    if not args.check:
        print()
        vendor_go_deps()

    print()
    if missing_required:
        print(f"Missing required tools: {', '.join(missing_required)}")
        return 1

    if pending_manual:
        print(f"Run the install commands above for: {', '.join(pending_manual)}")
        print("Then re-run: make ze-setup")
        return 1

    if args.check:
        print("All required tools present.")
    else:
        summary = []
        if installed:
            summary.append(f"installed: {', '.join(installed)}")
        if skipped:
            summary.append(f"skipped (optional): {', '.join(skipped)}")
        if summary:
            print(f"Setup complete. {'; '.join(summary)}")
        else:
            print("Setup complete. All tools already present.")
        print("Verify with: make ze-smoke")
    return 0


if __name__ == "__main__":
    sys.exit(main())
