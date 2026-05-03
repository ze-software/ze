#!/usr/bin/env python3
"""Run ze against a real xl2tpd LAC peer in Docker."""

from __future__ import annotations

import os
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path


IMAGE = os.environ.get("ZE_L2TP_DOCKER_IMAGE", "alpine:3.20")
PLATFORM = os.environ.get("ZE_L2TP_DOCKER_PLATFORM", "linux/amd64")
GOARCH = os.environ.get("ZE_L2TP_DOCKER_GOARCH", "amd64")


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
    inspect = run(["docker", "image", "inspect", IMAGE], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if inspect.returncode == 0:
        return
    print(f"pulling {IMAGE}...", file=sys.stderr)
    pull = run(["docker", "pull", IMAGE])
    if pull.returncode != 0:
        raise SystemExit(f"docker pull {IMAGE} failed")


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

    build = run(["go", "build", "-o", str(ze), "./cmd/ze"], cwd=root, env=env)
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


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


def wait_for_line(proc: subprocess.Popen[str], prefix: str, needle: str, timeout_s: float) -> tuple[bool, list[str]]:
    assert proc.stderr is not None
    lines: list[str] = []
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if proc.poll() is not None:
            return False, lines
        line = proc.stderr.readline()
        if not line:
            time.sleep(0.05)
            continue
        lines.append(line)
        sys.stderr.write(prefix + line)
        if needle in line:
            return True, lines
    return False, lines


def main() -> int:
    require_cmd("docker")
    root = repo_root()
    ensure_image()
    ze = ensure_linux_ze(root)

    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="l2tp-peer-", dir=tmp_parent))
    (work / "ze").mkdir(parents=True, exist_ok=True)

    (work / "xl2tpd.conf").write_text(
        "[global]\n"
        "port = 1702\n"
        "auth file = /run/l2tp/l2tp-secrets\n"
        "debug tunnel = yes\n"
        "debug state = yes\n"
        "debug packet = yes\n"
        "debug avp = yes\n"
        "\n"
        "[lac ze]\n"
        "lns = 127.0.0.1\n"
        "autodial = yes\n"
        "redial = yes\n"
        "redial timeout = 1\n"
        "max redials = 5\n"
        "require authentication = no\n"
        "ppp debug = yes\n"
        "pppoptfile = /run/l2tp/ppp-options\n"
        "length bit = yes\n",
        encoding="utf-8",
    )
    (work / "l2tp-secrets").write_text("* * s3cr3t\n", encoding="utf-8")
    (work / "ppp-options").write_text(
        "noauth\n"
        "name alice\n"
        "password alice\n"
        "refuse-eap\n"
        "nodefaultroute\n"
        "ipcp-accept-local\n"
        "ipcp-accept-remote\n"
        "debug\n"
        "nodetach\n",
        encoding="utf-8",
    )

    container = f"ze-l2tp-evidence-{os.getpid()}"
    started = run([
        "docker", "run", "--rm", "--detach", "--privileged",
        "--platform", PLATFORM,
        "--name", container,
        "-v", f"{root}:/src",
        "-v", f"{work}:/run/l2tp",
        "-w", "/src",
        IMAGE,
        "sleep", "infinity",
    ], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if started.returncode != 0:
        sys.stderr.write(started.stderr or "")
        raise SystemExit("failed to start L2TP evidence container")

    ze_proc: subprocess.Popen[str] | None = None
    xl2tpd: subprocess.Popen[str] | None = None
    try:
        install = run(["docker", "exec", container, "apk", "add", "--no-cache", "xl2tpd", "ppp"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if install.returncode != 0:
            sys.stderr.write((install.stdout or "") + (install.stderr or ""))
            raise SystemExit("apk add xl2tpd ppp failed")

        config = """l2tp {
    enabled true;
    auth-method none;
    allow-no-auth true;
    hello-interval 5;
    max-tunnels 4;
    max-sessions 4;
}
environment {
    l2tp {
        server main {
            ip 127.0.0.1;
            port 1701;
        }
    }
}
"""
        ze_proc = subprocess.Popen(
            [
                "docker", "exec", "--interactive",
                "--env", "ZE_LOG_L2TP=debug",
                "--env", "ze.l2tp.skip-kernel-probe=true",
                "--env", "ZE_STORAGE_BLOB=false",
                "--env", "ZE_CONFIG_DIR=/run/l2tp/ze",
                container, f"/src/{ze.relative_to(root)}", "-",
            ],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        assert ze_proc.stdin is not None
        ze_proc.stdin.write(config)
        ze_proc.stdin.close()

        ready, ze_lines = wait_for_line(ze_proc, "ze> ", "L2TP listener bound", 20)
        if not ready:
            raise SystemExit("ze L2TP listener did not start")
        ze_more = drain("ze> ", ze_proc.stderr)

        xl2tpd = subprocess.Popen(
            [
                "docker", "exec", container,
                "xl2tpd", "-D",
                "-c", "/run/l2tp/xl2tpd.conf",
                "-s", "/run/l2tp/l2tp-secrets",
                "-p", "/run/l2tp/xl2tpd.pid",
                "-C", "/run/l2tp/l2tp-control",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        assert xl2tpd.stdout is not None
        assert xl2tpd.stderr is not None
        drain("xl2tpd> ", xl2tpd.stdout)
        drain("xl2tpd-err> ", xl2tpd.stderr)

        deadline = time.time() + 20
        while time.time() < deadline:
            all_ze_lines = ze_lines + ze_more
            if any("session established" in line for line in all_ze_lines):
                print("OK: real xl2tpd peer established an L2TP session")
                return 0
            if ze_proc.poll() is not None:
                break
            time.sleep(0.2)

        sys.stderr.write("FAIL: xl2tpd session establishment not observed\n")
        all_ze_lines = ze_lines + ze_more
        sys.stderr.write("ze log tail:\n" + "".join(all_ze_lines[-80:]))
        return 1
    finally:
        terminate(xl2tpd)
        terminate(ze_proc)
        run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    raise SystemExit(main())
