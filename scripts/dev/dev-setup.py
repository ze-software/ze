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
import glob
import platform
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

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
    Tool(name="python3", probe=["python3"], brew="python", apt="python3"),
    Tool(
        name="uv",
        probe=["uv"],
        brew="uv",
        apt=None,
        note="not in apt repos; install via: curl -LsSf https://astral.sh/uv/install.sh | sh",
    ),
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
    Tool(name="ruff", probe=["ruff"], pipx_install="ruff"),
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

    Mirrors the apt path: dev-setup never runs sudo itself, it prints the
    command for the operator to run.
    """
    print(f'  Run: echo "{USERNS_SYSCTL} = 0" | sudo tee {USERNS_CONF}')
    print(f"  Run: sudo sysctl -w {USERNS_SYSCTL}=0")


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
        print(f"  Run: sudo apt-get install -y {pkg}")
        return False

    return False


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
            pending_manual.append(tool.name)
        else:
            if tool.required:
                missing_required.append(tool.name)
                print(f"  [MISSING]   {tool.name} (required)")
            else:
                skipped.append(tool.name)
                print(f"  [skipped]   {tool.name} (optional)")

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
        else:
            print("  [missing]   userns-unrestricted (REQUIRED) -- run:")
            print_userns_fix()
            pending_manual.append("userns-unrestricted")

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
