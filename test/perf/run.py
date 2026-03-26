#!/usr/bin/env python3
"""Ze performance benchmark runner.

Orchestrates Docker containers for each DUT (Ze, FRR, BIRD, GoBGP, rustbgpd,
RustyBGP, freeRtr), runs ze-perf against each, collects results, and generates reports.

Usage:
    python3 test/perf/run.py --build [DUT ...]    # build images only
    python3 test/perf/run.py --test [DUT ...]     # test only (images must exist)
    python3 test/perf/run.py --build --test [DUT ...]  # build then test
    python3 test/perf/run.py --build --test ze freertr # specific DUTs

Arguments:
    --build         Build/pull Docker images for selected DUTs
    --test          Run benchmarks (images must already exist)
    DUT ...         Optional DUT names to filter (default: all)
                    Available: ze, frr, bird, gobgp, rustbgpd, rustybgp, freertr

Environment:
    FRR_IMAGE       - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
    RUSTBGPD_IMAGE  - rustbgpd image (default: rustbgpd-interop, built from source)
    FREERTR_DIR     - freeRtr source (default: ../../../m36/freeRtr relative to project)
    DUT_ROUTES      - number of routes to send (default: 100000)
    DUT_SEED        - route generation seed (default: 42)
    DUT_REPEAT      - benchmark iterations (default: 3)
    NO_BUILD        - skip Docker image builds when set (e.g. NO_BUILD=1)
"""

import argparse
import atexit
import json
import os
import subprocess
import sys
import time

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))
INTEROP_DIR = os.path.join(PROJECT_ROOT, "test", "interop")
CONFIGS_DIR = os.path.join(SCRIPT_DIR, "configs")
RESULTS_DIR = os.path.join(SCRIPT_DIR, "results")

ZE_PERF = os.path.join(PROJECT_ROOT, "bin", "ze-perf")
ZE_PERF_LINUX = os.path.join(PROJECT_ROOT, "bin", "ze-perf-linux")

SUBNET = "172.31.0.0/24"
SENDER_IP = "172.31.0.10"
RECEIVER_IP = "172.31.0.11"

FRR_IMAGE = os.environ.get("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1")
RUSTBGPD_IMAGE = os.environ.get("RUSTBGPD_IMAGE", "rustbgpd-interop")
FREERTR_DIR = os.environ.get("FREERTR_DIR", os.path.abspath(
    os.path.join(PROJECT_ROOT, "..", "..", "..", "m36", "freeRtr")))
DUT_ROUTES = int(os.environ.get("DUT_ROUTES", "100000"))
DUT_SEED = int(os.environ.get("DUT_SEED", "42"))
DUT_REPEAT = int(os.environ.get("DUT_REPEAT", "3"))
NO_BUILD = bool(os.environ.get("NO_BUILD"))

# Each DUT: name, image, ip, port, sender_port (0=use port), receiver_port (0=use port)
DUTS = [
    {"name": "ze",       "image": "ze-interop",    "ip": "172.31.0.2", "port": 179, "sender_port": 1790, "receiver_port": 1791},
    {"name": "frr",      "image": FRR_IMAGE,       "ip": "172.31.0.3", "port": 179, "sender_port": 0, "receiver_port": 0},
    {"name": "bird",     "image": "bird-interop",  "ip": "172.31.0.4", "port": 179, "sender_port": 0, "receiver_port": 0},
    {"name": "gobgp",    "image": "gobgp-interop", "ip": "172.31.0.5", "port": 179, "sender_port": 0, "receiver_port": 0},
    {"name": "rustbgpd", "image": RUSTBGPD_IMAGE,  "ip": "172.31.0.6", "port": 179, "sender_port": 0, "receiver_port": 0},
    {"name": "rustybgp", "image": "rustybgp-interop", "ip": "172.31.0.7", "port": 179, "sender_port": 0, "receiver_port": 0},
    {"name": "freertr",  "image": "freertr-interop",  "ip": "172.31.0.8", "port": 179, "sender_port": 0, "receiver_port": 0},
]

SUFFIX = str(os.getpid())
NETWORK = f"ze-perf-{SUFFIX}"


import platform

# Use buildx if available (preferred), fall back to legacy builder.
USE_BUILDX = subprocess.run(
    ["docker", "buildx", "version"],
    capture_output=True, timeout=5,
).returncode == 0


def docker(*args, check=True, timeout=60, capture=False, **kwargs):
    """Run a docker command. Uses buildx on macOS, legacy builder on Linux."""
    args = list(args)
    if args[:2] == ["buildx", "build"] and not USE_BUILDX:
        args = ["build"] + [a for a in args[2:] if a != "--load"]
    cmd = ["docker"] + args
    if capture:
        return subprocess.run(cmd, check=check, timeout=timeout, capture_output=True, text=True, **kwargs)
    return subprocess.run(cmd, check=check, timeout=timeout, **kwargs)


def build_linux_binary():
    """Cross-compile ze-perf for Linux if missing or wrong architecture."""
    import platform
    go_arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), "amd64")
    if os.path.exists(ZE_PERF_LINUX):
        # Verify existing binary matches current architecture.
        r = subprocess.run(["file", ZE_PERF_LINUX], capture_output=True, text=True, timeout=5)
        if go_arch == "amd64" and "x86-64" in r.stdout:
            return
        if go_arch == "arm64" and "aarch64" in r.stdout:
            return
        print(f"Rebuilding ze-perf-linux for {go_arch}...")
    else:
        print(f"Cross-compiling ze-perf for Linux/{go_arch}...")
    env = {**os.environ, "GOOS": "linux", "GOARCH": go_arch, "CGO_ENABLED": "0"}
    subprocess.run(
        ["go", "build", "-o", ZE_PERF_LINUX, "./cmd/ze-perf/"],
        check=True, timeout=120, env=env, cwd=PROJECT_ROOT,
    )


def build_images(needed_duts):
    """Build/pull only the images needed for the requested DUTs."""
    if os.environ.get("NO_BUILD"):
        print("  skipping image builds (NO_BUILD=1)")
        return

    needed = {d["name"] for d in needed_duts}

    if "ze" in needed:
        print("Building Ze image...")
        docker("buildx", "build", "--load", "-t", "ze-interop",
               "-f", os.path.join(INTEROP_DIR, "Dockerfile.ze"),
               PROJECT_ROOT, "--quiet", timeout=600)

    if "bird" in needed:
        print("Building BIRD image...")
        docker("buildx", "build", "--load", "-t", "bird-interop",
               "-f", os.path.join(INTEROP_DIR, "Dockerfile.bird"),
               INTEROP_DIR, "--quiet", timeout=120)

    if "gobgp" in needed:
        print("Building GoBGP image...")
        try:
            docker("buildx", "build", "--load", "-t", "gobgp-interop",
                   "-f", os.path.join(INTEROP_DIR, "Dockerfile.gobgp"),
                   INTEROP_DIR, "--quiet", timeout=300)
        except subprocess.CalledProcessError:
            print("  warning: GoBGP image build failed")

    if "frr" in needed:
        print("Pulling FRR image...")
        docker("pull", FRR_IMAGE, "--quiet", timeout=120)

    if "rustbgpd" in needed:
        print("Building rustbgpd image...")
        try:
            docker("buildx", "build", "--load", "-t", "rustbgpd-interop",
                   "-f", os.path.join(INTEROP_DIR, "Dockerfile.rustbgpd"),
                   INTEROP_DIR, "--quiet", timeout=900)
        except subprocess.CalledProcessError:
            print("  warning: rustbgpd image build failed")

    if "rustybgp" in needed:
        print("Building RustyBGP image...")
        try:
            docker("buildx", "build", "--load", "-t", "rustybgp-interop",
                   "-f", os.path.join(INTEROP_DIR, "Dockerfile.rustybgp"),
                   INTEROP_DIR, "--quiet", timeout=900)
        except subprocess.CalledProcessError:
            print("  warning: RustyBGP image build failed")

    if "freertr" in needed:
        print("Building freeRtr image...")
        if not os.path.isdir(FREERTR_DIR):
            print(f"  warning: freeRtr source not found at {FREERTR_DIR}")
            print("  set FREERTR_DIR to the freeRtr repository root")
        else:
            try:
                docker("buildx", "build", "--load", "-t", "freertr-interop",
                       "-f", os.path.join(INTEROP_DIR, "Dockerfile.freertr"),
                       FREERTR_DIR, "--quiet", timeout=600)
            except subprocess.CalledProcessError:
                print("  warning: freeRtr image build failed")


def container_name(dut_name):
    return f"ze-perf-{dut_name}-{SUFFIX}"


def start_dut(dut):
    """Start a DUT container. Returns True on success."""
    name = dut["name"]
    cname = container_name(name)
    ip = dut["image"]

    volume_map = {
        "ze":       ["-v", f"{CONFIGS_DIR}/ze.conf:/etc/ze/bgp.conf:ro"],
        "frr":      ["-v", f"{CONFIGS_DIR}/frr.conf:/etc/frr/frr.conf:ro",
                     "-v", f"{INTEROP_DIR}/daemons:/etc/frr/daemons:ro",
                     "-v", f"{INTEROP_DIR}/vtysh.conf:/etc/frr/vtysh.conf:ro"],
        "bird":     ["-v", f"{CONFIGS_DIR}/bird.conf:/etc/bird/bird.conf:ro"],
        "gobgp":    ["-v", f"{CONFIGS_DIR}/gobgp.toml:/etc/gobgp/gobgp.toml:ro"],
        "rustbgpd": ["-v", f"{CONFIGS_DIR}/rustbgpd.toml:/etc/rustbgpd/config.toml:ro"],
        "rustybgp": ["-v", f"{CONFIGS_DIR}/rustybgp.toml:/etc/rustybgp/config.toml:ro"],
        "freertr":  ["-v", f"{CONFIGS_DIR}/freertr-hw.txt:/etc/freertr/freertr-hw.txt:ro",
                     "-v", f"{CONFIGS_DIR}/freertr-sw.txt:/etc/freertr/freertr-sw.txt:ro"],
    }

    caps = ["--cap-add", "NET_ADMIN"]
    if name == "frr":
        caps += ["--cap-add", "SYS_ADMIN"]
    if name == "freertr":
        caps += ["--cap-add", "NET_RAW"]

    extra = []
    if name == "ze":
        extra = ["/etc/ze/bgp.conf"]

    cmd = ["run", "-d", "--name", cname,
           "--network", NETWORK, "--ip", dut["ip"]] + caps + volume_map[name] + [dut["image"]] + extra

    try:
        docker(*cmd, timeout=30)
        print(f"  started {name} at {dut['ip']}")
        return True
    except subprocess.CalledProcessError as e:
        print(f"  FAIL: could not start {name}: {e}", file=sys.stderr)
        return False


def stop_dut(dut_name):
    docker("rm", "-f", container_name(dut_name), check=False, timeout=10,
           capture=True)


def wait_dut_ready(dut_name, timeout_s=30):
    """Wait for container to be running."""
    # freeRtr needs extra time: JVM startup + rawInt bridge initialization.
    if dut_name == "freertr":
        timeout_s = max(timeout_s, 45)
    cname = container_name(dut_name)
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        r = docker("inspect", cname, "--format", "{{.State.Running}}",
                   check=False, capture=True, timeout=10)
        if r.returncode == 0 and "true" in r.stdout:
            time.sleep(5)  # Give the BGP daemon time to bind its port.
            print(f"  {dut_name} ready")
            return True
        time.sleep(1)
    print(f"  error: {dut_name} did not start within {timeout_s}s", file=sys.stderr)
    docker("logs", cname, "--tail", "20", check=False, timeout=10)
    return False


def run_perf(dut):
    """Run ze-perf inside a Docker container against the DUT."""
    runner = f"ze-perf-runner-{SUFFIX}"
    result_name = f"{dut['name']}.json"

    # Start runner container at SENDER_IP.
    docker("run", "-d", "--name", runner,
           "--network", NETWORK, "--ip", SENDER_IP,
           "--cap-add", "NET_ADMIN",
           "-v", f"{ZE_PERF_LINUX}:/usr/local/bin/ze-perf:ro",
           "-v", f"{RESULTS_DIR}:/results",
           "alpine:3.21", "sleep", "3600",
           timeout=30)

    try:
        # Add receiver IP as second address.
        docker("exec", runner, "ip", "addr", "add", f"{RECEIVER_IP}/24", "dev", "eth0",
               check=False, timeout=10, capture=True)

        # freeRtr has its own TCP/IP stack and validates checksums.
        # Docker veth uses checksum offloading, so disable it on the runner.
        if dut["name"] == "freertr":
            docker("exec", runner, "apk", "add", "--no-cache", "ethtool",
                   check=False, timeout=30, capture=True)
            docker("exec", runner, "ethtool", "-K", "eth0", "tx", "off",
                   check=False, timeout=10, capture=True)

        # Scale timeouts based on route count.
        # Conservative: ~4s per 1000 routes (covers slow DUTs like freeRtr ~250-1000/s).
        convergence_timeout_s = max(30, (DUT_ROUTES // 1000) * 4 + 30)
        # Total iterations: warmup(1) + repeat. Each takes convergence + iter-delay(8s) + overhead(30s).
        total_iterations = 1 + DUT_REPEAT
        subprocess_timeout_s = total_iterations * (convergence_timeout_s + 40) + 60

        # Build ze-perf command.
        cmd = [
            "exec", runner, "/usr/local/bin/ze-perf", "run",
            "--dut-addr", dut["ip"],
            "--dut-port", str(dut["port"]),
            "--dut-asn", "65000",
            "--dut-name", dut["name"],
            "--sender-addr", SENDER_IP,
            "--sender-asn", "65001",
            "--receiver-addr", RECEIVER_IP,
            "--receiver-asn", "65002",
            "--routes", str(DUT_ROUTES),
            "--seed", str(DUT_SEED),
            "--repeat", str(DUT_REPEAT),
            "--warmup-runs", "1",
            "--iter-delay", "8s",
            "--warmup", "2s",
            "--connect-timeout", "20s",
            "--duration", f"{convergence_timeout_s}s",
            "--output", f"/results/{result_name}",
        ]

        if dut["sender_port"]:
            cmd += ["--sender-port", str(dut["sender_port"])]
        if dut["receiver_port"]:
            cmd += ["--receiver-port", str(dut["receiver_port"])]

        docker(*cmd, timeout=subprocess_timeout_s)
        return True

    except subprocess.CalledProcessError:
        return False

    finally:
        docker("rm", "-f", runner, check=False, timeout=10, capture=True)


def cleanup():
    """Remove all containers and the network."""
    print("\nCleaning up...")
    for dut in DUTS:  # noqa: B007 -- need dut["name"] not index
        stop_dut(dut["name"])
    docker("rm", "-f", f"ze-perf-runner-{SUFFIX}", check=False, timeout=10, capture=True)
    docker("network", "rm", NETWORK, check=False, timeout=10, capture=True)


def parse_args():
    all_names = [d["name"] for d in DUTS]
    parser = argparse.ArgumentParser(
        description="Ze performance benchmark runner.",
        epilog=f"Available DUTs: {', '.join(all_names)}",
    )
    parser.add_argument("--build", action="store_true", help="build Docker images")
    parser.add_argument("--test", action="store_true", help="run benchmarks")
    parser.add_argument("duts", nargs="*", metavar="DUT", help="DUT names (default: all)")
    args = parser.parse_args()

    if not args.build and not args.test:
        parser.error("at least one of --build or --test is required")

    return args


def main():
    args = parse_args()

    # Filter DUTs.
    requested = set(args.duts) if args.duts else None
    duts = [d for d in DUTS if requested is None or d["name"] in requested]

    if not duts:
        print(f"error: no matching DUTs. Available: {', '.join(d['name'] for d in DUTS)}", file=sys.stderr)
        return 1

    if args.build:
        build_images(duts)

    if not args.test:
        return 0

    # Check prerequisites.
    if not os.path.isfile(ZE_PERF):
        print(f"error: ze-perf not found. Run: make ze-perf", file=sys.stderr)
        return 1

    build_linux_binary()

    atexit.register(cleanup)

    os.makedirs(RESULTS_DIR, exist_ok=True)

    # Create network (remove stale one first if subnet conflicts).
    r = docker("network", "create", "--subnet", SUBNET, NETWORK,
               check=False, timeout=10, capture=True)
    if r.returncode != 0:
        # Subnet conflict -- remove any old ze-perf network and retry.
        old = docker("network", "ls", "--filter", f"name=ze-perf-", "--format", "{{{{.Name}}}}",
                     check=False, timeout=10, capture=True)
        for net in old.stdout.strip().splitlines():
            docker("network", "rm", net, check=False, timeout=10, capture=True)
        r = docker("network", "create", "--subnet", SUBNET, NETWORK,
                   check=False, timeout=10, capture=True)
        if r.returncode != 0:
            print(f"error: cannot create Docker network: {r.stderr.strip()}", file=sys.stderr)
            return 1

    print()
    print("--------------------------------------------")
    print(f" Ze Performance Benchmarks")
    print(f" Routes: {DUT_ROUTES}  Seed: {DUT_SEED}  Repeat: {DUT_REPEAT}")
    print("--------------------------------------------")
    print()

    passed = 0
    failed = 0
    failed_names = []
    result_files = []

    for dut in duts:
        name = dut["name"]
        print(f"-- {name} --")

        if not start_dut(dut):
            failed += 1
            failed_names.append(name)
            continue

        if not wait_dut_ready(name):
            stop_dut(name)
            failed += 1
            failed_names.append(name)
            continue

        result_file = os.path.join(RESULTS_DIR, f"{name}.json")
        if run_perf(dut):
            print(f"  PASS")
            passed += 1
            result_files.append(result_file)
        else:
            print(f"  FAIL")
            failed += 1
            failed_names.append(name)

        stop_dut(name)
        print()

    # Generate reports.
    print("--------------------------------------------")

    if result_files:
        print("\nComparison report:\n")
        subprocess.run(
            [ZE_PERF, "report", "--md"] + result_files,
            check=False, timeout=30,
        )

        html_path = os.path.join(RESULTS_DIR, "report.html")
        with open(html_path, "w") as f:
            subprocess.run(
                [ZE_PERF, "report", "--html"] + result_files,
                check=False, timeout=30, stdout=f,
            )
        print(f"\nHTML report: {html_path}")

        perf_doc = os.path.join(PROJECT_ROOT, "docs", "performance.md")
        with open(perf_doc, "w") as f:
            subprocess.run(
                [ZE_PERF, "report", "--doc"] + result_files,
                check=False, timeout=30, stdout=f,
            )
        print(f"Performance doc: {perf_doc}")

    print()
    if failed == 0:
        print(f"PASS  {passed} DUT(s) benchmarked")
    else:
        print(f"FAIL  {passed} passed, {failed} failed: {' '.join(failed_names)}")
    print("--------------------------------------------")

    return 1 if failed > 0 else 0


if __name__ == "__main__":
    sys.exit(main())
