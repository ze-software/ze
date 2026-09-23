#!/usr/bin/env python3
"""Run the existing native MPLS carrier and retain exact non-skipped outcomes."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

binary, destination, mode = sys.argv[1:]
if mode not in {"options", "full"}:
    raise SystemExit("mode must be options or full")
root = Path(destination)
root.mkdir(parents=True, exist_ok=False)
selector = "^TestMPLSIntegration_TransitIPv4Options$"
expected = {
    "TestMPLSIntegration_TransitIPv4Options",
    "TestMPLSIntegration_TransitIPv4Options/df-clear",
    "TestMPLSIntegration_TransitIPv4Options/df-set",
}
if mode == "full":
    selector = "^TestMPLSIntegration_(PathMTU|FragmentProgress|TransitIPv4Options)$"
    expected.update({
        "TestMPLSIntegration_PathMTU",
        "TestMPLSIntegration_PathMTU/push/downstream-path",
        "TestMPLSIntegration_PathMTU/push/local-link",
        "TestMPLSIntegration_PathMTU/transit/downstream-path",
        "TestMPLSIntegration_PathMTU/transit/local-link",
        "TestMPLSIntegration_FragmentProgress",
        "TestMPLSIntegration_FragmentProgress/rounded-zero-payload",
        "TestMPLSIntegration_FragmentProgress/physical-link-underflow",
        "TestMPLSIntegration_FragmentProgress/IPv4-options",
    })

def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

argv = ["go", "tool", "test2json", "-t", "-p", "github.com/ze-software/ze/internal/plugins/fib/kernel",
        binary, "-test.v=test2json", "-test.count=1", "-test.timeout=120s", "-test.run=" + selector]
meta = {"argv": argv, "mode": mode, "started": time.time(), "cwd": os.getcwd(),
        "kernel": list(os.uname()), "uid": os.geteuid(), "binary_sha256_before": digest(binary),
        "expected_tests": sorted(expected)}
(root / "invocation.json").write_text(json.dumps(meta, indent=2) + "\n")
for name, command in {"go-version": ["go", "version"], "capabilities": ["cat", "/proc/self/status"]}.items():
    observation = subprocess.run(command, capture_output=True, text=True, check=False)
    (root / (name + ".json")).write_text(json.dumps({"argv": command, "exit": observation.returncode,
        "stdout": observation.stdout, "stderr": observation.stderr}, indent=2) + "\n")
error = None
exit_code = None
with (root / "test.jsonl").open("wb") as stdout, (root / "test.stderr").open("wb") as stderr:
    try:
        result = subprocess.run(argv, stdout=stdout, stderr=stderr, timeout=150, check=False)
        exit_code = result.returncode
    except (OSError, subprocess.TimeoutExpired) as exc:
        error = str(exc)
passed, skipped, failed, parse_errors = set(), set(), set(), []
for number, line in enumerate((root / "test.jsonl").read_text().splitlines(), 1):
    try:
        event = json.loads(line)
    except json.JSONDecodeError as exc:
        parse_errors.append({"line": number, "error": str(exc)})
        continue
    name = event.get("Test")
    if not name:
        continue
    if event.get("Action") == "pass":
        passed.add(name)
    elif event.get("Action") == "skip":
        skipped.add(name)
    elif event.get("Action") == "fail":
        failed.add(name)
meta.update({"finished": time.time(), "exit": exit_code, "error": error,
             "binary_sha256_after": digest(binary)})
meta["binary_unchanged"] = meta["binary_sha256_before"] == meta["binary_sha256_after"]
proven = exit_code == 0 and error is None and not parse_errors and not skipped and not failed and expected <= passed and meta["binary_unchanged"]
report = {"proven": proven, "invocation": meta, "passed": sorted(passed), "skipped": sorted(skipped),
          "failed": sorted(failed), "missing_passes": sorted(expected - passed), "parse_errors": parse_errors,
          "limit": "Exact named native packet assertions only; not complete RFC conformance or independent RSVP-peer interoperability."}
(root / "result.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps({"proven": proven, "exit": exit_code, "failed": sorted(failed), "skipped": sorted(skipped),
                  "missing_passes": sorted(expected - passed), "error": error}))
raise SystemExit(0 if proven else 1)
