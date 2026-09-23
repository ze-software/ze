#!/usr/bin/env python3
"""Disposable passive evidence adapter for the unchanged native RSVP carrier."""
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import traceback


ROOT = Path("/workspace")
EVIDENCE = ROOT / "plan/verification-evidence/2026-09-22-baseline/rsvp-kernel-preparation"
INTERVAL = 2.0
MAX_SAMPLES = 121
FILTER = "ether proto 0x8847 or ether proto 0x8848 or arp or (ip and (udp or ip proto 46 or ip proto 89 or icmp))"
SOURCES = (
    "docs/architecture/mpls/mpls-kernel.md",
    "internal/plugins/rsvpte/producer_integration_linux_test.go",
    "internal/plugins/rsvpte/transport_integration_linux_test.go",
    "internal/plugins/rsvpte/register.go",
    "internal/plugins/rsvpte/engine.go",
    "internal/plugins/rsvpte/reservation.go",
    "internal/plugins/rsvpte/fib.go",
    "internal/core/mplsfib/events.go",
    "internal/plugins/fib/kernel/mplsentry_linux.go",
    "internal/plugins/fib/kernel/mplscontext_linux.go",
)
# Executed once per namespace/sample. Every read has its own timestamp and
# value/error; any missing required file or failed directory enumeration exits 1.
PROC_READER = r'''
import json, sys, time
from pathlib import Path
failed = False

def emit(path, operation):
    global failed
    row = {"path": str(path), "operation": operation, "started_unix_ns": time.time_ns()}
    try:
        if operation == "list_directory":
            value = sorted(p.name for p in path.iterdir())
        else:
            value = path.read_text()
        row.update(status="ok", value=value)
    except OSError as exc:
        failed = True
        value = [] if operation == "list_directory" else None
        row.update(status="error", error=str(exc), errno=exc.errno)
    row["finished_unix_ns"] = time.time_ns()
    print(json.dumps(row), flush=True)
    return value

paths = {"/proc/net/snmp", "/proc/net/netstat", "/proc/net/dev",
         "/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/mpls/platform_labels"}
links = ["lo", "primary", "bypass-a", "bypass-b"] if sys.argv[1] == "h" else (
        ["lo", "primary", "bypass-a", "bypass-b", "tail"] if sys.argv[1] == "m" else ["lo", "tail"])
for base, leaf, expected in (
    ("/proc/sys/net/mpls/conf", "input", links),
    ("/proc/sys/net/ipv4/conf", "rp_filter", links + ["all", "default"]),
):
    names = emit(Path(base), "list_directory")
    for name in sorted(set(names) | set(expected)):
        paths.add(base + "/" + name + "/" + leaf)
for path in sorted(paths):
    emit(Path(path), "read_file")
sys.exit(1 if failed else 0)
'''


def stamp():
    return {"unix_ns": time.time_ns(), "monotonic_ns": time.monotonic_ns()}


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


def observe(prefix, argv, timeout=2):
    """File-backed output stays complete, including after timeout or spawn failure."""
    row = {"argv": argv, "started": stamp(), "exit_code": None,
           "stdout": str(prefix) + ".stdout", "stderr": str(prefix) + ".stderr"}
    with Path(row["stdout"]).open("xb") as stdout, Path(row["stderr"]).open("xb") as stderr:
        try:
            process = subprocess.Popen(argv, stdout=stdout, stderr=stderr)
            row["pid"] = process.pid
            try:
                row["exit_code"] = process.wait(timeout=timeout)
                row["status"] = "ok" if row["exit_code"] == 0 else "error"
            except subprocess.TimeoutExpired:
                row.update(status="timeout", kill_requested=stamp())
                process.kill()
                row["exit_code"] = process.wait()
        except OSError as exc:
            row.update(status="spawn-error", error=str(exc), errno=exc.errno)
            stderr.write((str(exc) + "\n").encode())
    row["finished"] = stamp()
    save(Path(str(prefix) + ".json"), row)
    return row


def sample(namespace, number, out):
    directory = out / "samples" / namespace / f"{number:03d}"
    directory.mkdir(parents=True)
    prefix = ["ip", "netns", "exec", namespace]
    commands = (
        ("ipv4-routes", ["ip", "-4", "-details", "route", "show", "table", "all"]),
        ("mpls-routes", ["ip", "-f", "mpls", "-details", "route", "show", "table", "all"]),
        ("ipv4-rules", ["ip", "-4", "rule", "show"]),
        ("links", ["ip", "-statistics", "-details", "link", "show"]),
        ("neighbors", ["ip", "-4", "-statistics", "neigh", "show"]),
        ("addresses", ["ip", "-4", "address", "show"]),
        ("proc", [sys.executable, "-c", PROC_READER, namespace[-1]]),
    )
    errors = []
    for name, command in commands:
        row = observe(directory / name, prefix + command)
        if row["status"] != "ok":
            errors.append({"observation": str(directory / (name + ".json")), "status": row["status"]})
    return errors


def start_capture(namespace, out):
    prefix = out / "captures" / namespace
    row = {"namespace": namespace, "started": stamp(), "exit_code": None,
           "pcap": str(prefix) + ".pcap", "stdout": str(prefix) + ".stdout",
           "stderr": str(prefix) + ".stderr", "signals": []}
    row["argv"] = ["ip", "netns", "exec", namespace, "tcpdump", "-i", "any",
                   "-p", "-n", "-s", "0", "-U", "-B", "2048", "-Z", "root",
                   "-w", row["pcap"], FILTER]
    process = None
    with Path(row["stdout"]).open("xb") as stdout, Path(row["stderr"]).open("xb") as stderr:
        try:
            process = subprocess.Popen(row["argv"], stdout=stdout, stderr=stderr)
            row.update(pid=process.pid, status="running")
        except OSError as exc:
            row.update(status="spawn-error", error=str(exc), errno=exc.errno)
            stderr.write((str(exc) + "\n").encode())
            row["finished"] = stamp()
    save(Path(str(prefix) + ".json"), row)
    return process, row


def stop_capture(process, row, out):
    if process is not None:
        row["exited_before_stop"] = process.poll() is not None
        for sig, timeout in ((signal.SIGINT, 5), (signal.SIGTERM, 2), (signal.SIGKILL, None)):
            if process.poll() is not None:
                break
            row["signals"].append({"signal": sig.name, "requested": stamp()})
            try:
                process.send_signal(sig)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                continue
        row["exit_code"] = process.wait()
        row["finished"] = stamp()
        row["status"] = "ok" if (row["exit_code"] == 0 and not row["exited_before_stop"]
                                  and len(row["signals"]) == 1) else "error"
    path = Path(row["pcap"])
    row["pcap_present"] = path.is_file()
    row["pcap_bytes"] = path.stat().st_size if path.is_file() else None
    if not row["pcap_present"]:
        row["status"] = "error"
    save(out / "captures" / (row["namespace"] + ".json"), row)
    return row


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: capture-native-guest.py ABSOLUTE_RSVP_TEST ABSOLUTE_ZE NEW_ABSOLUTE_OUTPUT_DIRECTORY")
    binary, daemon, out = map(Path, sys.argv[1:])
    if not all(path.is_absolute() for path in (binary, daemon, out)):
        raise SystemExit("all three arguments must be absolute guest paths")
    if not ROOT.is_dir() or os.geteuid() != 0:
        raise SystemExit("run as root inside the native QEMU guest")
    for path in (binary, daemon):
        if not path.is_file() or not os.access(path, os.X_OK):
            raise SystemExit(f"missing executable input: {path}")
    # lexists also rejects a dangling output symlink. Never reuse an attempt.
    if os.path.lexists(out):
        raise SystemExit("output already exists")
    out = out.resolve()
    if not out.is_relative_to(EVIDENCE.resolve()) or out == EVIDENCE.resolve():
        raise SystemExit("output must be a new child of rsvp-kernel-preparation")
    out.mkdir(parents=True, exist_ok=False)
    for child in ("sources", "captures", "observations"):
        (out / child).mkdir()
    errors, captures, pending = [], {}, {}
    process = None
    command = [str(binary), "-test.v", "-test.run=^TestRSVPNativeProducer$", "-test.timeout=4m",
               "-rsvp-producer-daemon", str(daemon)]
    metadata = {"status": "preparing", "adapter_argv": sys.argv, "argv": command,
                "started": stamp(), "cwd": os.getcwd(), "kernel": list(os.uname()),
                "binary": {"path": str(binary)}, "daemon": {"path": str(daemon)},
                "source_snapshots": [], "sampling_interval_seconds": INTERVAL,
                "maximum_samples_per_namespace": MAX_SAMPLES,
                "boundary": "Source copies describe the workspace, not proven binary build provenance. Capture presence is not forwarding proof."}
    save(out / "invocation.json", metadata)
    carrier = {"argv": command, "exit_code": None, "status": "not-started",
               "stdout": str(out / "producer.stdout"), "stderr": str(out / "producer.stderr")}
    observed, counts, next_sample = set(), {}, {}
    interrupted = []
    old_handlers = {}

    def interrupt(signum, frame):
        interrupted.append({"signal": signal.Signals(signum).name, "received": stamp()})
        if process is not None and process.poll() is None:
            process.send_signal(signum)

    pool = ThreadPoolExecutor(max_workers=3)
    try:
        for key, path in (("binary", binary), ("daemon", daemon)):
            metadata[key]["sha256_before"] = digest(path)
        for source in [ROOT / name for name in SOURCES] + [Path(__file__).resolve()]:
            target = out / "sources" / str(source.relative_to(ROOT)).replace("/", "__")
            entry = {"path": str(source), "copy": str(target), "started": stamp()}
            try:
                target.write_bytes(source.read_bytes())
                entry.update(status="ok", sha256=digest(target))
            except OSError as exc:
                entry.update(status="error", error=str(exc), errno=exc.errno)
                errors.append(entry)
            entry["finished"] = stamp()
            metadata["source_snapshots"].append(entry)
        for name, argv in (("ip-version", ["ip", "-Version"]),
                           ("tcpdump-version", ["tcpdump", "--version"]),
                           ("guest-status", [sys.executable, "-c", "from pathlib import Path; print(Path('/proc/self/status').read_text(), end='')"])):
            row = observe(out / "observations" / name, argv)
            if row["status"] != "ok":
                errors.append(row)
        for sig in (signal.SIGINT, signal.SIGTERM):
            old_handlers[sig] = signal.signal(sig, interrupt)
        with Path(carrier["stdout"]).open("xb") as stdout, Path(carrier["stderr"]).open("xb") as stderr:
            carrier["started"] = stamp()
            process = subprocess.Popen(command, stdout=stdout, stderr=stderr)
            carrier.update(pid=process.pid, status="running")
            namespaces = [f"rsvp-{process.pid}-{node}" for node in "hme"]
            metadata.update(status="running", child_pid=process.pid, owned_namespaces=namespaces)
            save(out / "invocation.json", metadata)
            save(out / "carrier.json", carrier)
            with (out / "namespace-discovery.jsonl").open("x") as discovery:
                while process.poll() is None:
                    for namespace in namespaces:
                        # Only exact names derived from our direct native child's PID.
                        path = Path("/var/run/netns") / namespace
                        event = {"path": str(path), "operation": "stat", "started": stamp()}
                        try:
                            info = path.stat()
                            exists = True
                            event.update(status="present", device=info.st_dev, inode=info.st_ino)
                        except FileNotFoundError:
                            exists = False
                            event["status"] = "absent"
                        except OSError as exc:
                            exists = False
                            event.update(status="error", error=str(exc), errno=exc.errno)
                            errors.append(event)
                        event["finished"] = stamp()
                        discovery.write(json.dumps(event) + "\n")
                        if exists and namespace not in observed:
                            observed.add(namespace)
                            captures[namespace] = start_capture(namespace, out)
                            counts[namespace], next_sample[namespace] = 0, 0
                        future = pending.get(namespace)
                        if future is not None and future.done():
                            errors.extend(future.result())
                            del pending[namespace]
                        if (exists and namespace not in pending and counts[namespace] < MAX_SAMPLES
                                and time.monotonic() >= next_sample[namespace]):
                            pending[namespace] = pool.submit(sample, namespace, counts[namespace], out)
                            counts[namespace] += 1
                            next_sample[namespace] = time.monotonic() + INTERVAL
                    discovery.flush()
                    time.sleep(0.1)
            carrier.update(exit_code=process.wait(), status="exited", finished=stamp())
    except Exception:
        errors.append({"status": "adapter-error", "at": stamp(), "traceback": traceback.format_exc()})
    finally:
        # Normal carrier exit owns namespace cleanup; this adapter never deletes one.
        if process is not None and process.poll() is None:
            carrier["adapter_termination"] = stamp()
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                carrier["adapter_kill"] = stamp()
                process.kill()
                process.wait()
        if process is not None:
            carrier.update(exit_code=process.returncode, finished=carrier.get("finished", stamp()))
        for namespace, (capture, row) in captures.items():
            try:
                final = stop_capture(capture, row, out)
                if final["status"] != "ok":
                    errors.append({"capture": namespace, "status": final["status"]})
            except Exception:
                errors.append({"capture": namespace, "traceback": traceback.format_exc()})
        pool.shutdown(wait=True)
        for namespace, future in pending.items():
            try:
                errors.extend(future.result())
            except Exception:
                errors.append({"sample": namespace, "traceback": traceback.format_exc()})
        for sig, handler in old_handlers.items():
            signal.signal(sig, handler)
        for key, path in (("binary", binary), ("daemon", daemon)):
            try:
                metadata[key]["sha256_after"] = digest(path)
                if metadata[key].get("sha256_before") != metadata[key]["sha256_after"]:
                    errors.append({"input_changed": str(path)})
            except OSError as exc:
                errors.append({"input": str(path), "error": str(exc)})
        save(out / "carrier.json", carrier)
        metadata.update(status="finished", finished=stamp())
        save(out / "invocation.json", metadata)

    text = "\n".join(Path(carrier[key]).read_text(errors="replace")
                     for key in ("stdout", "stderr") if Path(carrier[key]).is_file())
    passed = re.search(r"^--- PASS: TestRSVPNativeProducer \(", text, re.MULTILINE) is not None
    skipped = re.search(r"^--- SKIP: TestRSVPNativeProducer \(", text, re.MULTILINE) is not None
    failed = re.search(r"^--- FAIL: TestRSVPNativeProducer \(", text, re.MULTILINE) is not None
    exact_pass = carrier["exit_code"] == 0 and passed and not skipped and not failed
    test_status = "pass" if exact_pass else "skip" if skipped and carrier["exit_code"] == 0 else "failure"
    missing = sorted(set(metadata.get("owned_namespaces", [])) - observed)
    if missing:
        errors.append({"namespaces_never_observed": missing})
    if interrupted:
        errors.append({"adapter_interrupted": interrupted})
    result = {"test_status": test_status, "test_exit_code": carrier["exit_code"],
              "exact_test_pass": exact_pass, "pass_marker": passed, "skip_marker": skipped,
              "failure_marker": failed, "collection_status": "errors" if errors else "complete",
              "collection_errors": errors, "observed_namespaces": sorted(observed),
              "samples_per_namespace": counts, "finished": stamp(),
              "forwarding_boundary": "Only the unchanged carrier's exact-payload receive/source/label/MAC assertions prove delivery. A route readback or pcap file alone does not. Foreign in-label EEXIST is the intended negative control.",
              "review_required": "Check input build provenance, packet chronology and tcpdump loss counters; no automatic pcap forwarding verdict."}
    # Keep test failure/skip separate from collection failure, including when both occur.
    code = 0 if exact_pass and not errors else 2 if test_status == "skip" else 1 if test_status == "failure" else 3
    result["adapter_exit_code"] = code
    save(out / "result.json", result)
    print(json.dumps(result, indent=2))
    return code


if __name__ == "__main__":
    sys.exit(main())
