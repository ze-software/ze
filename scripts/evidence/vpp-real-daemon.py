#!/usr/bin/env python3
"""Run a real VPP daemon in Docker and prove ze can program its FIB."""

from __future__ import annotations

import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path


VPP_IMAGE = os.environ.get("ZE_VPP_DOCKER_IMAGE", "ligato/vpp-base:latest")
VPP_PLATFORM = os.environ.get("ZE_VPP_DOCKER_PLATFORM", "linux/amd64")
GOARCH = os.environ.get("ZE_VPP_DOCKER_GOARCH", "amd64")
PREFIX = "10.20.0.0/24"
NEXT_HOP = "10.0.0.1"


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
    inspect = run(["docker", "image", "inspect", VPP_IMAGE], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if inspect.returncode == 0:
        return
    print(f"pulling {VPP_IMAGE}...", file=sys.stderr)
    pull = run(["docker", "pull", VPP_IMAGE])
    if pull.returncode != 0:
        raise SystemExit(f"docker pull {VPP_IMAGE} failed")


def ensure_linux_binaries(root: Path) -> tuple[Path, Path]:
    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / f"ze-linux-{GOARCH}"
    ze_test = bindir / f"ze-test-linux-{GOARCH}"

    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = GOARCH
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))

    for out, pkg in ((ze, "./cmd/ze"), (ze_test, "./cmd/ze-test")):
        build = run(["go", "build", "-o", str(out), pkg], cwd=root, env=env)
        if build.returncode != 0:
            raise SystemExit(f"go build {pkg} failed")
    return ze, ze_test


def wait_for_path(path: Path, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if path.exists():
            return True
        time.sleep(0.1)
    return False


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def terminate(proc: subprocess.Popen[str] | None, grace: float = 3.0) -> None:
    if proc is None or proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=grace)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2.0)


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


def wait_for_peer(peer: subprocess.Popen[str], timeout_s: float) -> bool:
    assert peer.stdout is not None
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if peer.poll() is not None:
            return False
        line = peer.stdout.readline()
        if not line:
            time.sleep(0.05)
            continue
        sys.stderr.write("peer> " + line)
        if "listening on" in line:
            drain("peer> ", peer.stdout)
            return True
    return False


def vppctl(container: str, command: str) -> subprocess.CompletedProcess[str]:
    return run([
        "docker", "exec", container,
        "vppctl", "-s", "/run/vpp/cli.sock",
        *command.split(),
    ], stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def route_present(container: str) -> tuple[bool, str]:
    out = vppctl(container, f"show ip fib {PREFIX}")
    text = (out.stdout or "") + (out.stderr or "")
    return PREFIX in text, text


def stop_peer(container: str, process_name: str) -> None:
    run(["docker", "exec", container, "pkill", "-TERM", "-f", process_name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def wait_route(container: str, want_present: bool, timeout_s: float) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = route_present(container)
        last = text
        if present == want_present:
            return True, text
        time.sleep(0.5)
    return False, last


def main() -> int:
    require_cmd("docker")
    root = repo_root()
    ensure_image()
    ze, ze_test = ensure_linux_binaries(root)

    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="vpp-real-", dir=tmp_parent))
    ze_config_dir = work / "ze"
    ze_config_dir.mkdir(parents=True, exist_ok=True)
    api_sock = Path("/run/vpp/api.sock")

    startup = work / "startup.conf"
    startup.write_text(
        "unix {\n"
        "  nodaemon\n"
        "  cli-listen /run/vpp/cli.sock\n"
        "  log /run/vpp/vpp.log\n"
        "}\n\n"
        "api-segment {\n"
        "  prefix vpp\n"
        "}\n\n"
        "socksvr {\n"
        f"  socket-name {api_sock}\n"
        "}\n\n"
        "plugins {\n"
        "  plugin dpdk_plugin.so { disable }\n"
        "}\n\n"
        "statseg {\n"
        "  socket-name /run/vpp/stats.sock\n"
        "}\n",
        encoding="utf-8",
    )

    container = f"ze-vpp-evidence-{os.getpid()}"
    vpp = run([
        "docker", "run", "--rm", "--detach", "--privileged",
        "--platform", VPP_PLATFORM,
        "--name", container,
        "-v", f"{root}:/src",
        "-v", f"{work}:/run/vpp",
        "-w", "/src",
        "--entrypoint", "sleep",
        VPP_IMAGE,
        "infinity",
    ], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if vpp.returncode != 0:
        sys.stderr.write(vpp.stderr or "")
        raise SystemExit("failed to start VPP container")

    peer: subprocess.Popen[str] | None = None
    daemon: subprocess.Popen[str] | None = None
    try:
        start_vpp = run(["docker", "exec", "--detach", container, "vpp", "-c", "/run/vpp/startup.conf"])
        if start_vpp.returncode != 0:
            raise SystemExit("failed to start VPP inside container")
        if not wait_for_path(work / "api.sock", 30):
            logs = run(["docker", "logs", container], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            sys.stderr.write((logs.stdout or "") + (logs.stderr or ""))
            raise SystemExit("VPP API socket did not appear")
        if not wait_for_path(work / "cli.sock", 30):
            raise SystemExit("VPP CLI socket did not appear")

        version = vppctl(container, "show version")
        if version.returncode != 0:
            sys.stderr.write((version.stdout or "") + (version.stderr or ""))
            raise SystemExit("vppctl show version failed")
        sys.stderr.write(version.stdout or "")

        port = free_port()
        peer_script = work / "peer-script"
        peer_script.write_text(
            "option=tcp_connections:value=1\n"
            f"option=update:value=send-route:prefix={PREFIX}:next-hop={NEXT_HOP}:origin-as=65001\n",
            encoding="utf-8",
        )
        peer = subprocess.Popen(
            [
                "docker", "exec", container,
                f"/src/{ze_test.relative_to(root)}", "peer", "--mode", "sink", "--port", str(port), "/run/vpp/peer-script",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        assert peer.stderr is not None
        drain("peer-err> ", peer.stderr)
        if not wait_for_peer(peer, 5):
            raise SystemExit("ze-test peer did not start")

        config = f"""bgp {{
    peer peer1 {{
        connection {{
            remote {{ ip 127.0.0.1; }}
            local  {{ ip 127.0.0.1; accept false; }}
        }}
        session {{
            asn {{ local 1; remote 1; }}
            router-id 1.2.3.4;
            family {{ ipv4/unicast {{ prefix {{ maximum 10000; }} }} }}
            capability {{ graceful-restart disable; }}
        }}
        behavior {{ group-updates disable; }}
    }}
}}

vpp {{
    enabled true;
    external true;
    api-socket {api_sock};
    stats {{ socket-path /run/vpp/stats.sock; }}
}}

fib {{
    vpp {{ enabled true; }}
}}
"""

        daemon = subprocess.Popen(
            [
                "docker", "exec", "--interactive",
                "--env", "ZE_LOG_VPP=info",
                "--env", "ZE_LOG_FIB_VPP=debug",
                "--env", "ZE_LOG_BGP=info",
                "--env", "ZE_STORAGE_BLOB=false",
                "--env", "ZE_CONFIG_DIR=/run/vpp/ze",
                "--env", f"ZE_TEST_BGP_PORT={port}",
                container, f"/src/{ze.relative_to(root)}", "-",
            ],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        assert daemon.stdin is not None
        assert daemon.stderr is not None
        daemon.stdin.write(config)
        daemon.stdin.close()
        ze_lines = drain("ze> ", daemon.stderr)

        ok, last_fib = wait_route(container, True, 25)
        if ok:
            print(f"OK: real VPP FIB contains {PREFIX}")
            stop_peer(container, ze_test.name)
            try:
                peer.wait(timeout=5)
            except subprocess.TimeoutExpired:
                terminate(peer)
            withdrawn, last_fib = wait_route(container, False, 15)
            if withdrawn:
                print(f"OK: real VPP FIB withdrew {PREFIX}")
                return 0

            sys.stderr.write("FAIL: real VPP FIB route was not withdrawn\n")
            sys.stderr.write(last_fib)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1

        sys.stderr.write("FAIL: real VPP FIB route not observed\n")
        sys.stderr.write(last_fib)
        sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
        return 1
    finally:
        terminate(daemon)
        terminate(peer)
        run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    raise SystemExit(main())
