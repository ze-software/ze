#!/usr/bin/env python3
"""Run a Python evidence script inside a privileged Linux container.

Generic Docker wrapper for evidence scripts that need Linux kernel
features unavailable on macOS. Builds a static Linux ze binary on the
host, starts a privileged Alpine container, installs requested packages,
and runs the inner script with ZE_EVIDENCE_ZE_BINARY pointing to the
pre-built binary.

Usage:
    python3 scripts/evidence/docker-run.py \\
        scripts/evidence/effective-l2tp-ppp.py \\
        iproute2 kmod ppp python3 xl2tpd

    # With extra env vars forwarded into the container:
    python3 scripts/evidence/docker-run.py \\
        --env ZE_L2TP_PPP_LISTEN_PORT=1705 \\
        scripts/evidence/effective-l2tp-ppp.py \\
        iproute2 kmod ppp python3 xl2tpd
"""

from __future__ import annotations

import argparse
import os
import signal
import shutil
import subprocess
import sys
from pathlib import Path


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file():
            return parent
    raise SystemExit("cannot locate repository root")


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, check=False, **kwargs)


def ensure_image(image: str) -> None:
    inspect = run(
        ["docker", "image", "inspect", image],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if inspect.returncode == 0:
        return
    print(f"pulling {image}...", file=sys.stderr)
    pull = run(["docker", "pull", image])
    if pull.returncode != 0:
        raise SystemExit(f"docker pull {image} failed")


def ensure_linux_ze(root: Path, goarch: str) -> Path:
    if shutil.which("go") is None:
        raise SystemExit("missing required command: go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / f"ze-linux-{goarch}"

    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))

    build = run(["go", "build", "-o", str(ze), "./cmd/ze"], cwd=root, env=env)
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    return ze


def module_mount_args() -> list[str]:
    if Path("/lib/modules").is_dir():
        return ["-v", "/lib/modules:/lib/modules:ro"]
    return []


def ze_env_args() -> list[str]:
    args: list[str] = []
    for key, value in os.environ.items():
        if key.startswith("ZE_") or key.startswith("ze."):
            args.extend(["--env", f"{key}={value}"])
    return args


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Run a Python evidence script inside a privileged Linux container.",
    )
    parser.add_argument("script", help="Evidence script path (relative to repo root)")
    parser.add_argument("packages", nargs="*", help="Alpine packages to install")
    parser.add_argument(
        "--env",
        dest="extra_env",
        action="append",
        default=[],
        help="Extra KEY=VALUE env vars to forward (repeatable)",
    )
    parser.add_argument(
        "--image", default=os.environ.get("ZE_DOCKER_EVIDENCE_IMAGE", "alpine:3.20")
    )
    parser.add_argument(
        "--platform",
        default=os.environ.get("ZE_DOCKER_EVIDENCE_PLATFORM", "linux/amd64"),
    )
    parser.add_argument(
        "--goarch", default=os.environ.get("ZE_DOCKER_EVIDENCE_GOARCH", "amd64")
    )
    args = parser.parse_args()

    if shutil.which("docker") is None:
        raise SystemExit("missing required command: docker")

    root = repo_root()
    script = Path(args.script)
    if not script.is_absolute():
        script = root / script
    if not script.is_file():
        raise SystemExit(f"evidence script not found: {script}")

    ensure_image(args.image)
    ze = ensure_linux_ze(root, args.goarch)

    container = f"ze-evidence-{os.getpid()}"

    started = run(
        [
            "docker",
            "run",
            "--rm",
            "--detach",
            "--privileged",
            "--platform",
            args.platform,
            "--name",
            container,
            "-v",
            f"{root}:/src",
            *module_mount_args(),
            "-w",
            "/src",
            args.image,
            "sleep",
            "infinity",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if started.returncode != 0:
        sys.stderr.write((started.stdout or "") + (started.stderr or ""))
        raise SystemExit("failed to start evidence container")

    def _handle_term(signum, _frame):
        run(
            ["docker", "rm", "-f", container],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, _handle_term)

    try:
        if args.packages:
            install = run(
                [
                    "docker",
                    "exec",
                    container,
                    "apk",
                    "add",
                    "--no-cache",
                    *args.packages,
                ],
            )
            if install.returncode != 0:
                raise SystemExit(f"apk add {' '.join(args.packages)} failed")

        env_args = [
            "--env",
            f"ZE_EVIDENCE_ZE_BINARY=/src/{ze.relative_to(root)}",
        ]
        env_args.extend(ze_env_args())
        for item in args.extra_env:
            env_args.extend(["--env", item])

        rel_script = script.relative_to(root)
        test = run(
            ["docker", "exec", *env_args, container, "python3", f"/src/{rel_script}"],
        )
        return test.returncode
    finally:
        run(
            ["docker", "rm", "-f", container],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


if __name__ == "__main__":
    raise SystemExit(main())
