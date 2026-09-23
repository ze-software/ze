#!/usr/bin/env python3
"""Disposable QEMU-only adapter for 02-ze-ac-pppd-client; not a native action.
Assertions and pppd argv mirror internal/le/interoplab/pppoe/check_ac.go.
Run only through host.py. Preparation is not evidence of a runtime pass.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import struct
import subprocess
import tempfile
import time
import traceback

parser = argparse.ArgumentParser()
parser.add_argument("--ze", required=True)
parser.add_argument("--scenario", required=True)
parser.add_argument("--out", required=True)
args = parser.parse_args()
durable_out = Path(args.out)
durable_out.mkdir(parents=True, exist_ok=False)
output_dir = tempfile.TemporaryDirectory(prefix="ze-ac-evidence-", dir="/tmp")
out = Path(output_dir.name)
report = {"status": "running", "assertions": [], "started_at": time.time()}
processes = []
handles = []
namespaces = []
runtime_dir = None
ze_ns = "ac-proof-ze"
client_ns = "ac-proof-client"


def save_report():
    (out / "result.json").write_text(json.dumps(report, indent=2) + "\n")


def require(value, message):
    if not value:
        raise RuntimeError(message)


def observed(name):
    report["assertions"].append(name)
    save_report()


def run(argv, ns=None, check=True):
    if ns:
        argv = ["ip", "netns", "exec", ns] + argv
    result = subprocess.run(argv, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=15)
    with (out / "commands.jsonl").open("a") as log:
        log.write(json.dumps({"argv": argv, "code": result.returncode,
                              "stdout": result.stdout, "stderr": result.stderr}) + "\n")
    if check and result.returncode:
        raise RuntimeError(f"{argv}: rc={result.returncode}: {result.stderr}")
    return result.stdout


def start(argv, ns, log, env=None):
    handle = (out / log).open("w")
    handles.append(handle)
    proc = subprocess.Popen(["ip", "netns", "exec", ns] + argv,
                            stdout=handle, stderr=subprocess.STDOUT, env=env)
    processes.append(proc)
    return proc


def stop(proc, sig=signal.SIGTERM):
    if proc.poll() is None:
        proc.send_signal(sig)
        try:
            proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
            raise RuntimeError(f"process {proc.pid} did not exit after {sig}")


def wait_for(label, probe, seconds=60):
    end = time.monotonic() + seconds
    last = None
    while time.monotonic() < end:
        last = probe()
        if last:
            return last
        time.sleep(0.25)
    raise RuntimeError(f"timeout: {label}; last={last!r}")


def sessions():
    text = run(["curl", "-sS", "--fail-with-body", "--max-time", "3",
                "-X", "POST", "http://127.0.0.1:9099/api/v1/execute",
                "-H", "Authorization: Bearer ze-pppoe-interop",
                "-H", "Content-Type: application/json",
                "-d", '{"command":"show pppoe sessions"}'], ze_ns)
    response = json.loads(text)
    require(response.get("status") != "error", text)
    data = response.get("data") or []
    require(isinstance(data, list), "REST session data must be a list")
    return data


def links():
    return json.loads(run(["ip", "-j", "link", "show", "type", "ppp"], client_ns))


def read_log(name):
    return (out / name).read_text(errors="replace")


def line_has(text, prefix, needle):
    return any(line.startswith(prefix) and needle in line for line in text.splitlines())


def capture(name):
    proc = start(["tcpdump", "-U", "-n", "-i", "eth0", "-s", "0", "-w",
                  str(out / (name + ".pcap")), "ether", "proto", "0x8863"],
                 client_ns, name + "-tcpdump.log")
    wait_for("tcpdump listening", lambda: "listening on" in read_log(name + "-tcpdump.log"), 10)
    return proc


def discovery_frames(name, partial=False):
    # tcpdump's Ethernet pcap; inspect PPPoE discovery codes independently of Ze's codec.
    raw = (out / (name + ".pcap")).read_bytes()
    if partial and len(raw) < 24:
        return []
    require(raw[:4] in (b"\xd4\xc3\xb2\xa1", b"\xa1\xb2\xc3\xd4"), "unsupported pcap header")
    endian = "<" if raw[:4] == b"\xd4\xc3\xb2\xa1" else ">"
    require(len(raw) >= 24 and struct.unpack_from(endian + "I", raw, 20)[0] == 1,
            "capture must contain Ethernet frames")
    offset, frames = 24, []
    while offset < len(raw):
        if partial and offset + 16 > len(raw):
            break
        require(offset + 16 <= len(raw), "truncated pcap record")
        size = struct.unpack_from(endian + "I", raw, offset + 8)[0]
        offset += 16
        frame = raw[offset:offset + size]
        offset += size
        if partial and len(frame) != size:
            break
        require(len(frame) == size, "truncated captured frame")
        if len(frame) >= 20 and frame[12:14] == b"\x88\x63":
            frames.append({"code": frame[15], "sid": int.from_bytes(frame[16:18], "big"),
                           "src": frame[6:12].hex(), "dst": frame[:6].hex()})
    return frames


def discovery(name, expected_sid=None):
    frames = discovery_frames(name)
    (out / (name + "-discovery.json")).write_text(json.dumps(frames, indent=2) + "\n")
    cursor = 0
    selected = []
    for code in (0x09, 0x07, 0x19, 0x65):
        while cursor < len(frames) and frames[cursor]["code"] != code:
            cursor += 1
        require(cursor < len(frames), f"{name}: missing ordered discovery code {code:#x}")
        selected.append(frames[cursor])
        cursor += 1
    padi, pado, padr, pads = selected
    require(pado["dst"] == padi["src"] == padr["src"] == pads["dst"] and
            pado["src"] == padr["dst"] == pads["src"], "discovery MAC endpoints differ")
    require(pads["sid"] > 0, "PADS allocated no session ID")
    if expected_sid is not None:
        require(pads["sid"] == expected_sid, "wire PADS SID differs from Ze REST SID")
        require(any(f["code"] == 0xa7 and f["sid"] == expected_sid for f in frames),
                "no wire PADT for established session")


def dial(password, log):
    # Exact protocol options from pppdDial, including service and CHAP-only policy.
    return start(["pppd", "plugin", "pppoe.so", "nic-eth0", "user", "alice",
                  "password", password, "noauth", "refuse-pap", "refuse-eap",
                  "refuse-mschap", "refuse-mschap-v2", "noipdefault", "nodefaultroute",
                  "noaccomp", "nopcomp", "mtu", "1492", "mru", "1492",
                  "lcp-echo-interval", "10", "lcp-echo-failure", "5", "maxfail", "1",
                  "nodetach", "debug", "rp_pppoe_service", "internet"], client_ns, log)


def alarm(signum, frame):
    raise RuntimeError("guest proof exceeded its 420-second bound")


save_report()
signal.signal(signal.SIGALRM, alarm)
signal.alarm(420)
try:
    require(os.geteuid() == 0, "guest must be root")
    require(Path("/workspace").is_dir(), "run only in native QEMU guest")
    for key in ("ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"):
        require(key not in os.environ, f"refusing {key}")
    for command in ("ip", "ping", "pppd", "tcpdump", "curl", "modprobe"):
        require(shutil.which(command), f"missing guest command: {command}")
    (out / "kernel.txt").write_text(run(["uname", "-a"]))
    (out / "packages.txt").write_text(run(["apk", "info", "-v"]))
    for module in ("ppp_generic", "pppox", "pppoe"):
        run(["modprobe", module], check=False)
    require(Path("/dev/ppp").exists() and stat.S_ISCHR(Path("/dev/ppp").stat().st_mode),
            "missing /dev/ppp character device")
    require(Path("/sys/module/pppoe").exists() or Path("/proc/net/pppoe").exists(),
            "missing kernel PPPoE support")
    require(list(Path("/usr/lib/pppd").glob("*/pppoe.so")), "missing ppp-pppoe plugin")
    observed("kernel PPP and PPPoE requirements present; no probe bypass")
    runtime_dir = tempfile.TemporaryDirectory(prefix="ze-ac-proof-", dir="/tmp")
    config_dir = Path(runtime_dir.name)
    config = config_dir / "ze.conf"
    shutil.copyfile(Path(args.scenario) / "ze.conf", config)
    report["config_sha256"] = hashlib.sha256(config.read_bytes()).hexdigest()
    for ns in (ze_ns, client_ns):
        run(["ip", "netns", "add", ns])
        namespaces.append(ns)
        run(["ip", "link", "set", "lo", "up"], ns)
    run(["ip", "link", "add", "acz", "type", "veth", "peer", "name", "acc"])
    for device, ns in (("acz", ze_ns), ("acc", client_ns)):
        run(["ip", "link", "set", device, "netns", ns])
        run(["ip", "link", "set", device, "name", "eth0"], ns)
        run(["ip", "link", "set", "eth0", "up"], ns)
    env = dict(os.environ, **{"ze.config.dir": str(config_dir),
                            "ze.log.pppoe": "debug", "ze.log.l2tp": "debug"})
    ze = start([args.ze, "start", str(config)], ze_ns, "ze.log", env)
    wait_for("Ze bound eth0", lambda: "PPPoE interface configured" in read_log("ze.log"))
    def rest_ready():
        try:
            return sessions() == []
        except (RuntimeError, json.JSONDecodeError):
            return False
    wait_for("Ze REST ready and initially empty", rest_ready)
    require(not links(), "client PPP state was not initially empty")
    cap = capture("accepted")
    client = dial("s3cr3t", "accepted-pppd.log")
    active = wait_for("one Ze discovery session", sessions, 45)
    require(len(active) == 1 and active[0]["sid"] > 0 and
            active[0]["service-name"] == "internet" and active[0]["interface"] == "eth0",
            f"unexpected discovery session: {active}")
    (out / "active-sessions.json").write_text(json.dumps(active, indent=2) + "\n")
    sid = active[0]["sid"]
    ppp_links = wait_for("kernel client PPP interface", links, 75)
    require(len(ppp_links) == 1, f"expected one client PPP interface: {ppp_links}")
    iface = ppp_links[0]["ifname"]
    def address_ready():
        text = run(["ip", "-o", "addr", "show", "dev", iface], client_ns)
        return text if "10.20.0.2" in text and "10.20.0.1" in text else None
    address = wait_for("IPCP point-to-point address", address_ready, 75)
    (out / "client-address.txt").write_text(address)
    route = run(["ip", "-o", "route", "show", "10.20.0.1"], client_ns)
    require("10.20.0.1" in route and "dev " + iface in route, "missing kernel PPP route")
    (out / "client-route.txt").write_text(route)
    text = read_log("accepted-pppd.log")
    for token in ("sent [LCP ConfReq", "rcvd [LCP ConfAck", "rcvd [LCP ConfReq",
                  "sent [LCP ConfAck", "rcvd [CHAP Challenge", "rcvd [CHAP Success",
                  "rcvd [IPCP ConfAck"):
        require(token in text, "accepted trace missing " + token)
    for prefix in ("sent [LCP ConfReq", "rcvd [LCP ConfReq"):
        for option in ("<mru 1492>", "<magic 0x"):
            require(line_has(text, prefix, option), f"missing {prefix} {option}")
    require(line_has(text, "rcvd [LCP ConfReq", "<auth chap MD5>"), "Ze did not demand CHAP-MD5")
    require(line_has(text, "sent [CHAP Response", 'name = "alice"'), "missing named CHAP response")
    observed("bidirectional LCP, MRU 1492, magic, CHAP-MD5 success and IPCP address/route")
    (out / "ping.txt").write_text(run(["ping", "-c", "3", "-W", "3", "-I", iface, "10.20.0.1"], client_ns))
    observed("ICMP crosses the negotiated kernel PPP interface to Ze")
    stop(client)
    require("Sent PADT" in read_log("accepted-pppd.log"), "pppd sent no PADT")
    wait_for("Ze state cleared after good dial", lambda: not sessions(), 30)
    require(not links(), "PPP link survived accepted teardown")
    wait_for("captured PADT", lambda: any(
        frame["code"] == 0xa7 and frame["sid"] == sid
        for frame in discovery_frames("accepted", partial=True)), 10)
    stop(cap, signal.SIGINT)
    discovery("accepted", sid)
    (out / "accepted-teardown-sessions.json").write_text(json.dumps(sessions()) + "\n")
    observed("wire PADI/PADO/PADR/PADS, matching REST SID, PADT and empty teardown state")
    cap = capture("rejected")
    client = dial("wrong-secret", "rejected-pppd.log")
    wait_for("wrong-secret pppd exits", lambda: client.poll() is not None, 60)
    text = read_log("rejected-pppd.log")
    require("rcvd [CHAP Challenge" in text and "rcvd [CHAP Failure" in text,
            "wrong-secret dial did not reach CHAP refusal")
    require("rcvd [IPCP ConfAck" not in text, "rejected session reached IPCP")
    require(not links(), "rejected dial retained a client PPP interface")
    wait_for("Ze state cleared after refusal", lambda: not sessions(), 30)
    wait_for("captured rejected PADS", lambda: any(
        frame["code"] == 0x65
        for frame in discovery_frames("rejected", partial=True)), 10)
    stop(cap, signal.SIGINT)
    discovery("rejected")
    (out / "rejected-teardown-sessions.json").write_text(json.dumps(sessions()) + "\n")
    observed("wrong-secret discovery reaches CHAP refusal without IPCP or residual session/link")
    report["status"] = "pass"
except Exception as exc:
    report["status"] = "fail"
    report["error"] = str(exc)
    (out / "exception.txt").write_text(traceback.format_exc())
finally:
    signal.alarm(0)
    cleanup_errors = []
    for proc in reversed(processes):
        try:
            stop(proc)
        except Exception as exc:
            cleanup_errors.append(str(exc))
    for ns in reversed(namespaces):
        try:
            run(["ip", "netns", "delete", ns])
        except Exception as exc:
            cleanup_errors.append(str(exc))
    if runtime_dir is not None:
        try:
            shutil.copytree(config_dir, out / "ze-config")
            runtime_dir.cleanup()
        except Exception as exc:
            cleanup_errors.append(str(exc))
    for handle in handles:
        handle.close()
    report["cleanup_errors"] = cleanup_errors
    if cleanup_errors:
        report["status"] = "fail"
    report["finished_at"] = time.time()
    save_report()
    shutil.copytree(out, durable_out, dirs_exist_ok=True)
    output_dir.cleanup()
raise SystemExit(0 if report["status"] == "pass" else 1)
