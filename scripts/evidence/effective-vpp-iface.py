#!/usr/bin/env python3
"""Run a real VPP daemon in Docker and prove ze programs VPP interface features
(tunnels, SPAN mirror, wireguard, LCP TAP shadow) via the GoVPP binary API.

This is the interface-feature counterpart to effective-vpp.py (FIB / traffic /
firewall). It mirrors that script's harness (ligato/vpp-base under Docker, ze
built for linux and run inside the container, vppctl for ground-truth
assertions) but never edits it.

Plugin gating: the wireguard and linux-cp (LCP) features need VPP plugins the
base image may or may not ship. Each scenario probes `show plugins` first and,
when the plugin is absent, records an evidence-backed SKIP with the reason
instead of a false failure -- the goal is "prove to the point the image allows".

Runbook: `python3 scripts/evidence/effective-vpp-iface.py` (needs Docker + a
privileged container). Overridable via ZE_VPP_DOCKER_IMAGE / _PLATFORM / _GOARCH.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

from feature_tags import daemon_build_tags


VPP_IMAGE = os.environ.get("ZE_VPP_DOCKER_IMAGE", "ligato/vpp-base:latest")
VPP_PLATFORM = os.environ.get("ZE_VPP_DOCKER_PLATFORM", "linux/amd64")
GOARCH = os.environ.get("ZE_VPP_DOCKER_GOARCH", "amd64")

GRE_LOCAL = "10.10.10.1"
GRE_REMOTE = "10.10.10.2"


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def require_cmd(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"missing required command: {name}")


def ensure_image() -> None:
    inspect = run(
        ["docker", "image", "inspect", VPP_IMAGE],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if inspect.returncode == 0:
        return
    print(f"pulling {VPP_IMAGE}...", file=sys.stderr)
    if run(["docker", "pull", VPP_IMAGE]).returncode != 0:
        raise SystemExit(f"docker pull {VPP_IMAGE} failed")


def ensure_linux_ze(root: Path) -> Path:
    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / f"ze-linux-{GOARCH}"
    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = GOARCH
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))
    build = run(
        ["go", "build", "-tags", daemon_build_tags(root), "-o", str(ze), "./cmd/ze"],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


def wait_for_path(path: Path, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if path.exists():
            return True
        time.sleep(0.1)
    return False


def drain(prefix: str, stream) -> list[str]:
    lines: list[str] = []

    def worker() -> None:
        try:
            for line in stream:
                lines.append(line)
                sys.stderr.write(prefix + line)
        except (ValueError, OSError):
            pass

    threading.Thread(target=worker, daemon=True).start()
    return lines


def terminate(proc: subprocess.Popen[str] | None, grace: float = 3.0) -> None:
    import signal

    if proc is None or proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=grace)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2.0)


def vppctl(container: str, command: str) -> subprocess.CompletedProcess[str]:
    return run(
        [
            "docker",
            "exec",
            container,
            "vppctl",
            "-s",
            "/run/vpp/cli.sock",
            *command.split(),
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def vppctl_text(container: str, command: str) -> str:
    out = vppctl(container, command)
    text = (out.stdout or "") + (out.stderr or "")
    if out.returncode != 0:
        raise SystemExit(f"vppctl {command!r} failed:\n{text}")
    return text


def plugin_loaded(container: str, so_name: str) -> bool:
    out = vppctl(container, "show plugins")
    text = (out.stdout or "") + (out.stderr or "")
    return so_name in text


def ze_env(container: str, ze: Path, root: Path, config_path: Path) -> list[str]:
    return [
        "docker",
        "exec",
        "--interactive",
        "--env",
        "ZE_LOG_VPP=info",
        "--env",
        "ZE_LOG_INTERFACE=debug",
        "--env",
        "ZE_LOG_BGP=info",
        "--env",
        "ZE_STORAGE_BLOB=false",
        "--env",
        "ZE_CONFIG_DIR=/run/vpp/ze",
        container,
        f"/src/{ze.relative_to(root)}",
        # `start <config>`: the bare `ze <config>` launch form was removed from the
        # CLI (learned 1248), so a positional path now dies with "unknown command".
        "start",
        str(config_path),
    ]


def start_ze(container: str, ze: Path, root: Path, config_path: Path):
    daemon = subprocess.Popen(
        ze_env(container, ze, root, config_path),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert daemon.stderr is not None
    lines = drain("ze> ", daemon.stderr)
    return daemon, lines


def vpp_config(lcp_enabled: bool = False) -> str:
    # LCP is off by default: enabling it makes ze create an lcp_itf_pair for
    # every loopback, which fails the whole apply on a VPP build without
    # linux_cp_plugin.so (honest exact-or-reject at the binapi layer). Only the
    # LCP scenario -- which gates on plugin presence first -- turns it on.
    lcp = "true" if lcp_enabled else "false"
    return (
        "vpp {\n"
        "    enabled true;\n"
        "    external true;\n"
        "    api-socket /run/vpp/api.sock;\n"
        "    stats { socket-path /run/vpp/stats.sock; }\n"
        f"    lcp {{ enabled {lcp}; netns host; }}\n"
        "    plugins { wireguard true; }\n"
        "}\n"
    )


def wait_condition(probe, timeout_s: float):
    """Poll probe() -> (bool, str) until true or timeout; return the last (bool, str)."""
    deadline = time.time() + timeout_s
    ok, text = False, ""
    while time.time() < deadline:
        ok, text = probe()
        if ok:
            return True, text
        time.sleep(0.5)
    return ok, text


def run_tunnel_evidence(container: str, root: Path, ze: Path, work: Path) -> int:
    """AC-2: ze creates a GRE tunnel on real VPP via gre_tunnel_add_del."""
    config = (
        vpp_config() + "interface {\n"
        "    backend vpp;\n"
        "    tunnel gre0 {\n"
        "        encapsulation {\n"
        "            gre {\n"
        f"                local {{ ip {GRE_LOCAL}; }}\n"
        f"                remote {{ ip {GRE_REMOTE}; }}\n"
        "            }\n"
        "        }\n"
        "    }\n"
        "}\n"
    )
    (work / "tunnel.conf").write_text(config, encoding="utf-8")
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/tunnel.conf"))
    try:

        def probe():
            out = vppctl(container, "show gre tunnel")
            text = (out.stdout or "") + (out.stderr or "")
            return (GRE_REMOTE in text or GRE_LOCAL in text), text

        ok, text = wait_condition(probe, 25)
        if not ok:
            # Fall back to the generic interface list (naming differs by version).
            ifaces = vppctl(container, "show interface")
            text += (ifaces.stdout or "") + (ifaces.stderr or "")
            ok = "gre" in text.lower()
        if not ok:
            sys.stderr.write("FAIL: GRE tunnel not observed on real VPP\n")
            sys.stderr.write(text)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP created a GRE tunnel {GRE_LOCAL} -> {GRE_REMOTE}")
        return 0
    finally:
        terminate(daemon)


def run_mirror_evidence(container: str, root: Path, ze: Path, work: Path) -> int:
    """AC-4: ze programs a SPAN mirror on real VPP via
    sw_interface_span_enable_disable. Two loopbacks are created by ze; one
    mirrors its ingress to the other."""
    config = (
        vpp_config() + "interface {\n"
        "    backend vpp;\n"
        "    dummy mdst0 { }\n"
        "    dummy msrc0 {\n"
        "        unit default { mirror { ingress mdst0; } }\n"
        "    }\n"
        "}\n"
    )
    (work / "mirror.conf").write_text(config, encoding="utf-8")
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/mirror.conf"))
    try:

        def probe():
            out = vppctl(container, "show interface span")
            text = (out.stdout or "") + (out.stderr or "")
            # A configured SPAN entry names the source interface and a
            # direction (rx/tx/both); an empty table has neither.
            return ("rx" in text.lower() or "both" in text.lower()), text

        ok, text = wait_condition(probe, 25)
        if not ok:
            sys.stderr.write("FAIL: SPAN mirror not observed on real VPP\n")
            sys.stderr.write(text)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print("OK: real VPP programmed a SPAN mirror (msrc0 rx -> mdst0)")
        return 0
    finally:
        terminate(daemon)


def run_wireguard_evidence(container: str, root: Path, ze: Path, work: Path) -> int:
    """AC-5: ze programs a wireguard interface on real VPP. Skips (evidence-backed)
    when the wireguard plugin is not loaded in the base image."""
    if not plugin_loaded(container, "wireguard_plugin.so"):
        print(
            "SKIP: wireguard_plugin.so not loaded in this image; ze rejects at apply "
            "and doctor-vpp-wireguard flags it (unit-tested). Image limit recorded."
        )
        return 0
    # A valid 32-byte base64 Curve25519 private key.
    priv = "aHZkc2ZqaHZra2hkZnZoamtkc2Zoa2RoaGRma2poZmg="
    config = (
        vpp_config() + "interface {\n"
        "    backend vpp;\n"
        "    wireguard wg0 {\n"
        "        listen-port 51820;\n"
        f'        private-key "{priv}";\n'
        "    }\n"
        "}\n"
    )
    (work / "wg.conf").write_text(config, encoding="utf-8")
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/wg.conf"))
    try:

        def probe():
            out = vppctl(container, "show wireguard interface")
            text = (out.stdout or "") + (out.stderr or "")
            return "51820" in text or "wg" in text.lower(), text

        ok, text = wait_condition(probe, 25)
        if not ok:
            sys.stderr.write("FAIL: wireguard interface not observed on real VPP\n")
            sys.stderr.write(text)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print("OK: real VPP created a wireguard interface (listen-port 51820)")
        return 0
    finally:
        terminate(daemon)


def run_lcp_evidence(container: str, root: Path, ze: Path, work: Path) -> int:
    """AC-6: ze creates an LCP pair shadowing a VPP loopback into a Linux TAP.
    Skips (evidence-backed) when the linux-cp plugins are absent."""
    if not (
        plugin_loaded(container, "linux_cp_plugin.so")
        and plugin_loaded(container, "linux_nl_plugin.so")
    ):
        print(
            "SKIP: linux_cp_plugin.so / linux_nl_plugin.so not loaded in this image; "
            "LCP pair creation is unit-tested and doctor-vpp-lcp-netns covers the "
            "netns constraint. Image limit recorded."
        )
        return 0
    config = (
        vpp_config(lcp_enabled=True) + "interface {\n"
        "    backend vpp;\n"
        "    dummy loop0 {\n"
        "        unit default { address 10.99.0.1/32; }\n"
        "    }\n"
        "}\n"
    )
    (work / "lcp.conf").write_text(config, encoding="utf-8")
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/lcp.conf"))
    try:

        def probe():
            out = vppctl(container, "show lcp")
            text = (out.stdout or "") + (out.stderr or "")
            return "loop0" in text or "tap" in text.lower(), text

        ok, text = wait_condition(probe, 25)
        if not ok:
            sys.stderr.write("FAIL: LCP pair not observed on real VPP\n")
            sys.stderr.write(text)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        # The host TAP should be visible in the container's Linux netns.
        tap = run(
            ["docker", "exec", container, "ip", "link", "show"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        print("OK: real VPP created an LCP pair shadowing loop0")
        sys.stderr.write((tap.stdout or "") + (tap.stderr or ""))
        return 0
    finally:
        terminate(daemon)


def main() -> int:
    require_cmd("docker")
    root = repo_root()
    ensure_image()
    ze = ensure_linux_ze(root)

    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="vpp-iface-", dir=tmp_parent))
    (work / "ze").mkdir(parents=True, exist_ok=True)

    # Load all plugins by default (no `plugin default { disable }`) except dpdk,
    # so wireguard / linux-cp load when the image ships them.
    (work / "startup.conf").write_text(
        "unix {\n  nodaemon\n  cli-listen /run/vpp/cli.sock\n  log /run/vpp/vpp.log\n}\n\n"
        "api-segment {\n  prefix vpp\n}\n\n"
        "socksvr {\n  socket-name /run/vpp/api.sock\n}\n\n"
        "plugins {\n  plugin dpdk_plugin.so { disable }\n}\n\n"
        "statseg {\n  socket-name /run/vpp/stats.sock\n}\n",
        encoding="utf-8",
    )

    container = f"ze-vpp-iface-{os.getpid()}"
    vpp = run(
        [
            "docker",
            "run",
            "--rm",
            "--detach",
            "--privileged",
            "--platform",
            VPP_PLATFORM,
            "--name",
            container,
            "-v",
            f"{root}:/src",
            "-v",
            f"{work}:/run/vpp",
            "-w",
            "/src",
            "--entrypoint",
            "sleep",
            VPP_IMAGE,
            "infinity",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if vpp.returncode != 0:
        sys.stderr.write(vpp.stderr or "")
        raise SystemExit("failed to start VPP container")

    try:
        if (
            run(
                [
                    "docker",
                    "exec",
                    "--detach",
                    container,
                    "vpp",
                    "-c",
                    "/run/vpp/startup.conf",
                ]
            ).returncode
            != 0
        ):
            raise SystemExit("failed to start VPP inside container")
        if not wait_for_path(work / "api.sock", 30):
            logs = run(
                ["docker", "logs", container],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            sys.stderr.write((logs.stdout or "") + (logs.stderr or ""))
            raise SystemExit("VPP API socket did not appear")
        if not wait_for_path(work / "cli.sock", 30):
            raise SystemExit("VPP CLI socket did not appear")

        version = vppctl(container, "show version")
        if version.returncode != 0:
            sys.stderr.write((version.stdout or "") + (version.stderr or ""))
            raise SystemExit("vppctl show version failed")
        sys.stderr.write(version.stdout or "")

        # Report plugin capabilities up front (recorded in the runbook).
        for so in ("wireguard_plugin.so", "linux_cp_plugin.so", "linux_nl_plugin.so"):
            print(f"PLUGIN: {so} loaded={plugin_loaded(container, so)}")

        rc = run_tunnel_evidence(container, root, ze, work)
        if rc != 0:
            return rc
        rc = run_mirror_evidence(container, root, ze, work)
        if rc != 0:
            return rc
        rc = run_wireguard_evidence(container, root, ze, work)
        if rc != 0:
            return rc
        return run_lcp_evidence(container, root, ze, work)
    finally:
        run(
            ["docker", "rm", "-f", container],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


if __name__ == "__main__":
    raise SystemExit(main())
