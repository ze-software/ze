#!/usr/bin/env python3
"""Scenario 05: Profile 1M route injection.

Single 1M round with CPU and heap profiling via Ze's --pprof flag.
The injection is driven by `ze-test peer --mode inject` in bb-ns.

Usage:
    sudo ZE_PPROF=1 VERBOSE=1 python3 test/stress/run.py 05-profile-1m

Profiles saved to tmp/stress-profile-{cpu,heap,goroutine}.pb.gz
Analyze: go tool pprof -http=:8080 tmp/stress-profile-cpu.pb.gz
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from harness import (
    Timer,
    ZE_NS,
    BB_IP,
    _nsexec_ok,
    start_peer_inject,
    wait_peer_done,
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
    pprof = os.environ.get("ZE_PPROF")
    os.makedirs(PROFILE_DIR, exist_ok=True)

    prefix_count = 1_000_000
    log_info("")
    log_info("=== Profiling: %d prefixes ===" % prefix_count)

    # Start CPU profile in background -- captures during injection + settle.
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

    with Timer("peer inject (handshake + stream + dwell)"):
        peer = start_peer_inject(
            prefix_base="10.0.0.0/24",
            prefix_count=prefix_count,
            nexthop=BB_IP,
            asn=65100,
            dwell="60s",
        )
        metrics = wait_peer_done(peer, timeout=600)

    if metrics:
        if "bytes" in metrics:
            log_info(
                "built %d msgs, %d bytes in %s"
                % (metrics["messages"], metrics["bytes"], metrics["build_time"])
            )
        if "mbps" in metrics:
            log_info(
                "sent in %s at %.1f MB/s" % (metrics["send_time"], metrics["mbps"])
            )

    if pprof:
        fetch_profile("heap", "%s/heap" % PPROF)
        fetch_profile("goroutine", "%s/goroutine?debug=0" % PPROF)
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
