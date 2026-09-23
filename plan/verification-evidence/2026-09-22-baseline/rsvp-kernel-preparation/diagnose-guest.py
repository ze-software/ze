#!/usr/bin/env python3
"""Observe the unchanged native carrier through fresh operator SSH sessions."""
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import re
import subprocess
import sys
import time


def namespaces():
    result = subprocess.run(["ip", "netns", "list"], capture_output=True, text=True, check=True)
    return {line.split()[0] for line in result.stdout.splitlines() if line.strip()}


def capture(namespace, sample, output):
    for label, command in (
        ("stacks", "show system goroutines full"),
        ("neighbors", "show ospf neighbor"),
        ("interfaces", "show ospf interface"),
    ):
        args = ["sshpass", "-p", "testpass", "ip", "netns", "exec", namespace,
                "ssh", "-p", "2222", "-o", "StrictHostKeyChecking=no",
                "-o", "UserKnownHostsFile=/dev/null", "-o", "ConnectTimeout=3",
                "admin@127.0.0.1", command + " | json"]
        prefix = output / (str(sample) + "-" + namespace + "-" + label)
        started = time.time()
        try:
            result = subprocess.run(args, capture_output=True, timeout=6, check=False)
            stdout, stderr, status = result.stdout, result.stderr, result.returncode
        except subprocess.TimeoutExpired as exc:
            stdout, stderr, status = exc.stdout or b"", exc.stderr or b"", "timeout"
        prefix.with_suffix(".stdout").write_bytes(stdout)
        prefix.with_suffix(".stderr").write_bytes(stderr)
        prefix.with_suffix(".json").write_text(json.dumps({
            "namespace": namespace, "command": command,
            "started_unix": started, "finished_unix": time.time(), "exit_code": status,
        }, indent=2) + "\n")


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: diagnose-guest.py ABSOLUTE_RSVP_TEST ABSOLUTE_ZE NEW_OUTPUT_DIRECTORY")
    output = Path(sys.argv[3])
    if output.exists():
        raise SystemExit("output directory already exists")
    previous = namespaces()
    process = subprocess.Popen([sys.executable, str(Path(__file__).with_name("run-guest.py")), *sys.argv[1:]])
    observed = set()
    first_seen = None
    next_sample = None
    sample = 0
    try:
        with ThreadPoolExecutor(max_workers=2) as pool:
            while process.poll() is None:
                owned = sorted(name for name in namespaces() - previous
                               if re.fullmatch(r"rsvp-[0-9]+-[hm]", name))
                if len(owned) == 2 and first_seen is None:
                    observed.update(owned)
                    first_seen = time.monotonic()
                    next_sample = first_seen + 12
                if len(owned) == 2 and next_sample is not None and time.monotonic() >= next_sample:
                    diagnostics = output / "diagnostics"
                    diagnostics.mkdir(exist_ok=True)
                    work = [pool.submit(capture, name, sample, diagnostics) for name in owned]
                    for item in work:
                        item.result()
                    sample += 1
                    next_sample = time.monotonic() + 5
                time.sleep(0.25)
        status = process.wait()
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
    if output.is_dir():
        (output / "diagnostic-invocation.json").write_text(json.dumps({
            "purpose": "Live userspace progression diagnosis; original carrier assertions are unchanged.",
            "observed_owned_namespaces": sorted(observed), "samples": sample,
            "carrier_exit_code": status,
            "boundary": "Existing daemon/carrier artifacts; this does not validate concurrent source edits.",
        }, indent=2) + "\n")
    return status


if __name__ == "__main__":
    sys.exit(main())
