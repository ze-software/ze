#!/usr/bin/env python3
"""Capture the existing native RSVP producer carrier; contains no RSVP scenario."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: run-guest.py ABSOLUTE_RSVP_TEST ABSOLUTE_ZE NEW_OUTPUT_DIRECTORY")
    binary, daemon, out = map(Path, sys.argv[1:])
    if not binary.is_absolute() or not daemon.is_absolute():
        raise SystemExit("test and daemon paths must be absolute guest paths")
    if not Path("/workspace").is_dir() or os.geteuid() != 0:
        raise SystemExit("run as root inside the native QEMU guest")
    for path in (binary, daemon):
        if not path.is_file() or not os.access(path, os.X_OK):
            raise SystemExit(f"missing executable input: {path}")
    for command in ("ip", "strace"):
        if shutil.which(command) is None:
            raise SystemExit(f"missing guest package command: {command}")
    out = out.resolve()
    evidence = Path("/workspace/plan/verification-evidence/2026-09-22-baseline/rsvp-kernel-preparation").resolve()
    if not out.is_relative_to(evidence) or out == evidence:
        raise SystemExit("output must be a new child of rsvp-kernel-preparation")
    out.mkdir(parents=True, exist_ok=False)
    sources = out / "sources"
    sources.mkdir()
    root = Path("/workspace")
    names = [
        "internal/plugins/rsvpte/producer_integration_linux_test.go",
        "internal/plugins/rsvpte/transport_integration_linux_test.go",
        "internal/plugins/rsvpte/register.go",
        "internal/plugins/rsvpte/engine.go",
        "internal/plugins/rsvpte/reservation.go",
        "internal/plugins/rsvpte/fib.go",
        "internal/core/mplsfib/events.go",
        "internal/plugins/fib/kernel/mplsentry_linux.go",
        "internal/plugins/fib/kernel/mplscontext_linux.go",
    ]
    source_hashes = {}
    for name in names:
        target = sources / name.replace("/", "__")
        shutil.copyfile(root / name, target)
        source_hashes[name] = digest(target)
    command = [
        "strace", "-f", "-qq", "-ttt", "-s", "4096", "-e", "trace=network",
        "-o", str(out / "network.trace"), str(binary), "-test.v",
        "-test.run=^TestRSVPNativeProducer$", "-test.timeout=4m",
        "-rsvp-producer-daemon", str(daemon),
    ]
    metadata = {
        "status": "running", "started_unix": time.time(), "command": command,
        "kernel_release": os.uname().release,
        "binary": {"path": str(binary), "sha256": digest(binary)},
        "daemon": {"path": str(daemon), "sha256": digest(daemon)},
        "source_sha256": source_hashes,
        "boundary": "Ze-to-Ze native kernel proof only; no foreign RSVP peer",
    }
    (out / "invocation.json").write_text(json.dumps(metadata, indent=2) + "\n")
    (out / "guest-process-status.txt").write_text(Path("/proc/self/status").read_text())
    (out / "ip-version.txt").write_bytes(subprocess.check_output(["ip", "-Version"]))
    # strace preserves netlink reads, AF_PACKET receives, exact UDP payloads,
    # and ENETUNREACH after withdrawal before testing.T removes its temp files.
    # It does not inject errors or replace the native forwarding owner.
    with (out / "producer.log").open("wb") as log:
        completed = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, check=False)
    text = (out / "producer.log").read_text(errors="replace")
    passed = re.search(r"^--- PASS: TestRSVPNativeProducer \(", text, re.MULTILINE) is not None
    skipped = re.search(r"^--- SKIP: TestRSVPNativeProducer \(", text, re.MULTILINE) is not None
    trace = out / "network.trace"
    trace_present = trace.is_file() and trace.stat().st_size > 0
    accepted = completed.returncode == 0 and passed and not skipped and trace_present
    result = {
        "status": "carrier-pass" if accepted else "unverified-skip" if skipped else "failed",
        "exit_code": completed.returncode, "exact_test_pass": passed, "skipped": skipped,
        "network_trace_present": trace_present, "finished_unix": time.time(),
        "scope": metadata["boundary"],
        "review_required": "Match binary build provenance to the copied sources; inspect network.trace with the carrier assertions in preparation.json. This wrapper does not independently decode netlink or packet bytes.",
    }
    (out / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))
    return 0 if accepted else 1


if __name__ == "__main__":
    sys.exit(main())
