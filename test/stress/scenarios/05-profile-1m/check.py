#!/usr/bin/env python3
"""Scenario 05: Profile 1M route injection.

Single 1M round with CPU and heap profiling via Ze's --pprof flag.

Usage:
    sudo ZE_PPROF=1 VERBOSE=1 python3 test/stress/run.py 05-profile-1m

Profiles saved to tmp/stress-profile-{cpu,heap,goroutine}.pb.gz
Analyze: go tool pprof -http=:8080 tmp/stress-profile-cpu.pb.gz
"""

import os
import sys
import time
import subprocess

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from bngblaster import (
    BNGBlaster,
    Ze,
    Timer,
    ZE_NS,
    generate_updates,
    BB_IP,
    _nsexec_ok,
    log_info,
    log_pass,
    log_fail,
)

PROJECT_ROOT = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..", "..")
)
PROFILE_DIR = os.path.join(PROJECT_ROOT, "tmp")
PPROF = "http://127.0.0.1:6060/debug/pprof"


def fetch_profile(name, url, timeout=120):
    """Fetch a pprof profile from Ze's --pprof endpoint."""
    path = os.path.join(PROFILE_DIR, "stress-profile-%s.pb.gz" % name)
    _nsexec_ok(ZE_NS, ["curl", "-sS", "-o", path, url], timeout=timeout)
    if os.path.isfile(path) and os.path.getsize(path) > 0:
        log_pass(
            "saved %s profile: %s (%d bytes)" % (name, path, os.path.getsize(path))
        )
    else:
        log_fail("failed to save %s profile" % name)


def check():
    bb = BNGBlaster()
    ze = Ze(bb)
    pprof = os.environ.get("ZE_PPROF")

    os.makedirs(PROFILE_DIR, exist_ok=True)

    bb.wait_session_established()

    prefix_count = 1_000_000
    log_info("")
    log_info("=== Profiling: %d prefixes ===" % prefix_count)

    update_file = generate_updates(
        prefix_base="11.0.0.0/24",
        prefix_count=prefix_count,
        nexthop=BB_IP,
        asn=65100,
        filename="profile-1m.bgp",
    )

    # Start CPU profile in background (captures during injection).
    cpu_proc = None
    if pprof:
        log_info("starting 90s CPU profile capture...")
        cpu_path = os.path.join(PROFILE_DIR, "stress-profile-cpu.pb.gz")
        cpu_proc = subprocess.Popen(
            [
                "ip",
                "netns",
                "exec",
                ZE_NS,
                "curl",
                "-sS",
                "-o",
                cpu_path,
                "%s/profile?seconds=90" % PPROF,
            ],
        )
        time.sleep(1)

    with Timer("injection") as t_inject:
        bb.bgp_raw_update(update_file)
        bb.wait_raw_update_done(timeout=1800)

    with Timer("ze processing") as t_process:
        ze.wait_settled(timeout=60)

    sessions = bb.bgp_sessions()
    for s in sessions.get("bgp-sessions", []):
        if s.get("state") != "established":
            log_fail("session dropped")
            raise AssertionError("session dropped")

    rate = prefix_count / t_inject.elapsed if t_inject.elapsed > 0 else 0
    log_pass(
        "%d prefixes: injection %.2fs, settled %.2fs (%.0f routes/s)"
        % (prefix_count, t_inject.elapsed, t_process.elapsed, rate)
    )

    if pprof:
        # Fetch heap + goroutine profiles (instant snapshots).
        fetch_profile("heap", "%s/heap" % PPROF)
        fetch_profile("goroutine", "%s/goroutine?debug=0" % PPROF)

        # Wait for CPU profile to finish.
        if cpu_proc:
            log_info("waiting for CPU profile to complete...")
            cpu_proc.wait(timeout=120)
            cpu_path = os.path.join(PROFILE_DIR, "stress-profile-cpu.pb.gz")
            if os.path.isfile(cpu_path) and os.path.getsize(cpu_path) > 0:
                log_pass(
                    "CPU profile: %s (%d bytes)" % (cpu_path, os.path.getsize(cpu_path))
                )
            else:
                log_fail("CPU profile capture failed")

        log_info("")
        log_info("Analyze:")
        log_info("  go tool pprof -http=:8080 tmp/stress-profile-cpu.pb.gz")
        log_info("  go tool pprof -http=:8080 tmp/stress-profile-heap.pb.gz")
    else:
        log_info("(profiling disabled -- run with ZE_PPROF=1 to enable)")
