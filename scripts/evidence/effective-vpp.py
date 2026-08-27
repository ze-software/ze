#!/usr/bin/env python3
"""Run a real VPP daemon in Docker and prove ze can program FIB, traffic, and firewall."""

from __future__ import annotations

import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

from feature_tags import feature_tags


VPP_IMAGE = os.environ.get("ZE_VPP_DOCKER_IMAGE", "ligato/vpp-base:latest")
VPP_PLATFORM = os.environ.get("ZE_VPP_DOCKER_PLATFORM", "linux/amd64")
GOARCH = os.environ.get("ZE_VPP_DOCKER_GOARCH", "amd64")
PREFIX = "10.20.0.0/24"
NEXT_HOP = "10.0.0.1"
MPLS_PREFIX = "10.30.0.0/24"
MPLS_LABEL = 100
TRAFFIC_POLICER_CLASS = "default"
# The IPsec case: what the probe binary prints, and what VPP must report back. The SPI
# and the salt are the values internal/component/ike/dataplane
# vpp_real_integration_test.go installs. The salt is the LAST FOUR OCTETS of the AEAD
# key material, so a backend that sent all 36 octets as the key, or a hardcoded zero
# salt, makes VPP report a different number.
IPSEC_REPORT_PREFIX = "ze-vpp-ipsec:"
IPSEC_SPI = 0x11223344
IPSEC_INBOUND_SPI = 0x55667788
IPSEC_SALT = "0xdeadbeef"
# The 32 cipher octets alone. All 36 KEYMAT octets used to go into this field with the
# salt hardcoded to 0, so VPP read an over-long key and a zero salt.
IPSEC_CIPHER_KEY = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"


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
    inspect = run(
        ["docker", "image", "inspect", VPP_IMAGE],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if inspect.returncode == 0:
        return
    print(f"pulling {VPP_IMAGE}...", file=sys.stderr)
    pull = run(["docker", "pull", VPP_IMAGE])
    if pull.returncode != 0:
        raise SystemExit(f"docker pull {VPP_IMAGE} failed")


def ensure_linux_binaries(root: Path) -> tuple[Path, Path]:
    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    ze = bindir / f"ze-linux-{GOARCH}"
    ze_test = bindir / f"ze-test-linux-{GOARCH}"

    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = GOARCH
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))

    features = " ".join(feature_tags(root))
    build = run(
        [
            "go",
            "build",
            "-tags",
            f"ze_core ze_distro {features}",
            "-o",
            str(ze),
            "./cmd/ze",
        ],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go build ./cmd/ze failed")
    # ze-test is the ze_test-tagged build of ./cmd/ze (there is no cmd/ze-test
    # directory; it was consolidated into cmd/ze selected by build tag, matching
    # the Makefile's `-tags 'ze_test $(ZE_FEATURES)' -o bin/ze-test ./cmd/ze`).
    build = run(
        ["go", "build", "-tags", f"ze_test {features}", "-o", str(ze_test), "./cmd/ze"],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go build ze-test (-tags ze_test ./cmd/ze) failed")
    return ze, ze_test


def wait_for_path(path: Path, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if path.exists():
            return True
        time.sleep(0.1)
    return False


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


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


def wait_for_peer(peer: subprocess.Popen[str], timeout_s: float) -> bool:
    assert peer.stdout is not None
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if peer.poll() is not None:
            return False
        line = peer.stdout.readline()
        if not line:
            time.sleep(0.05)
            continue
        sys.stderr.write("peer> " + line)
        if "listening on" in line:
            drain("peer> ", peer.stdout)
            return True
    return False


def vppctl(container: str, command: str) -> subprocess.CompletedProcess[str]:
    return run(
        [
            "docker",
            "exec",
            container,
            "vppctl",
            "-s",
            "/run/vpp/cli.sock",
            *command.split(),
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def vppctl_text(container: str, command: str) -> str:
    out = vppctl(container, command)
    text = (out.stdout or "") + (out.stderr or "")
    if out.returncode != 0:
        raise SystemExit(f"vppctl {command!r} failed:\n{text}")
    return text


def route_present(container: str) -> tuple[bool, str]:
    out = vppctl(container, f"show ip fib {PREFIX}")
    text = (out.stdout or "") + (out.stderr or "")
    return PREFIX in text, text


def create_loopback(container: str) -> str:
    text = vppctl_text(container, "create loopback interface")
    for token in text.replace("\n", " ").split():
        if token.startswith("loop") and token[4:].isdigit():
            iface = token
            break
    else:
        iface = "loop0"
    vppctl_text(container, f"set interface state {iface} up")
    interfaces = vppctl_text(container, "show interface")
    if iface not in interfaces:
        raise SystemExit(
            f"created VPP loopback {iface!r} not visible in show interface:\n{interfaces}"
        )
    return iface


def sw_if_index(container: str, iface: str) -> int:
    """Return the VPP sw_if_index of iface, read from `show interface`."""
    text = vppctl_text(container, "show interface")
    for line in text.splitlines():
        fields = line.split()
        if len(fields) >= 2 and fields[0] == iface:
            return int(fields[1])
    raise SystemExit(f"interface {iface!r} has no index in show interface:\n{text}")


def ensure_ipsec_probe(root: Path) -> Path:
    """Compile the IKE dataplane test binary that programs VPP over the API socket.

    It is `go test -c` of internal/component/ike/dataplane, so what runs against VPP is
    the shipped backend rather than a copy of it. The ze daemon is not the vehicle: no
    config leaf selects the IPsec dataplane, the only selector is the private
    ze.test.ike.dataplane override (ike/engine/testport.go), and IKE would still need a
    peer to negotiate with before it programmed anything.
    """
    require_cmd("go")
    bindir = root / "tmp" / "evidence" / "bin"
    bindir.mkdir(parents=True, exist_ok=True)
    probe = bindir / f"ipsec-vpp-linux-{GOARCH}"

    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = GOARCH
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOCACHE", str(root / "tmp" / "go-cache"))

    build = run(
        [
            "go",
            "test",
            "-c",
            "-tags",
            "ze_core ze_vpp integration",
            "-o",
            str(probe),
            "./internal/component/ike/dataplane",
        ],
        cwd=root,
        env=env,
    )
    if build.returncode != 0:
        raise SystemExit("go test -c ./internal/component/ike/dataplane failed")
    return probe


def run_ipsec_evidence(
    container: str, root: Path, probe: Path, api_sock: Path, iface: str
) -> int:
    """Install an IPsec SA and two policies in a real VPP, then read them back.

    This is the spec's AC-7. Every other test of this backend agrees with the
    generated binapi by construction; only a running VPP can say whether VPP accepts
    what the backend sends.

    The probe runs a cleanup half first, which installs its own SA and policy, closes
    the backend, and asserts VPP holds neither afterwards. What this function then
    reads back is the second half, which is deliberately left open so the state
    survives the probe process.
    """
    index = sw_if_index(container, iface)
    probe_run = run(
        [
            "docker",
            "exec",
            "--env",
            f"ZE_VPP_IPSEC_API_SOCKET={api_sock}",
            "--env",
            f"ZE_VPP_IPSEC_SW_IF_INDEX={index}",
            container,
            f"/src/{probe.relative_to(root)}",
            "-test.run",
            "TestVPPRealDataplaneInstalls",
            "-test.v",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    output = probe_run.stdout or ""
    sys.stderr.write(output)
    if probe_run.returncode != 0:
        sys.stderr.write(
            "FAIL: the IKE dataplane backend could not program a real VPP\n"
        )
        return 1
    if "SKIP" in output:
        sys.stderr.write("FAIL: the IPsec probe skipped; it must program the VPP\n")
        return 1

    ids = {}
    for line in output.splitlines():
        if line.startswith(IPSEC_REPORT_PREFIX):
            key, _, value = line[len(IPSEC_REPORT_PREFIX) :].strip().partition("=")
            ids[key] = value
    if "spd-id" not in ids or "sad-id" not in ids:
        sys.stderr.write(f"FAIL: the IPsec probe reported no ids: {ids}\n")
        return 1
    # The probe's first half installs an SA and a policy, closes the backend, and
    # asserts against this VPP that neither survives. Nothing in VPP expires either,
    # so a ze that exits without deleting leaves a dead session's SAs installed and
    # its SPD still bound. These two keys say that half ran and passed.
    if "close-removed-spi" not in ids or "close-removed-spd-id" not in ids:
        sys.stderr.write(
            f"FAIL: the IPsec probe did not report the close-cleanup half: {ids}\n"
        )
        return 1
    print(
        f"OK: Close removed the SA {ids['close-removed-spi']} and SPD "
        f"{ids['close-removed-spd-id']} it installed, so a restart leaves no orphan"
    )

    # `show ipsec sa` lists one line per SA; the index in brackets is what
    # `show ipsec sa <index>` takes, and it is not the sad_id.
    summary = vppctl_text(container, "show ipsec sa")
    for needle in (
        f"spi {IPSEC_SPI}",
        f"spi {IPSEC_INBOUND_SPI}",
        "protocol:esp",
        "tunnel",
    ):
        if needle not in summary:
            sys.stderr.write(
                f"FAIL: real VPP SA list does not report {needle!r}\n{summary}\n"
            )
            return 1
    # Direction: VPP records the inbound flag on exactly one of the two SAs.
    inbound_lines = [line for line in summary.splitlines() if "inbound" in line]
    if len(inbound_lines) != 1 or f"spi {IPSEC_INBOUND_SPI}" not in inbound_lines[0]:
        sys.stderr.write(
            f"FAIL: exactly one SA must carry the inbound flag, and it must be spi "
            f"{IPSEC_INBOUND_SPI}\n{summary}\n"
        )
        return 1
    print("OK: real VPP holds both SAs ze installed, and flags one of them inbound")
    print(summary)

    sa_index = None
    for line in summary.splitlines():
        stripped = line.strip()
        if f"spi {IPSEC_SPI}" in stripped and stripped.startswith("["):
            sa_index = stripped[1 : stripped.index("]")]
            break
    if sa_index is None:
        sys.stderr.write(f"FAIL: no runtime index for spi {IPSEC_SPI}\n{summary}\n")
        return 1
    detail = vppctl_text(container, f"show ipsec sa {sa_index}")
    for needle in (
        f"salt {IPSEC_SALT}",
        "aes-gcm-256",
        IPSEC_CIPHER_KEY,
        "integrity alg none",
        # ECN, measured rather than assumed. RFC 7296 Section 2.24 (RFC7296-2.24-1 and
        # -2) requires a tunnel-mode SA created by IKEv2 to copy the congestion
        # indication on encapsulation and back onto the inner header on decapsulation.
        # VPP's tunnel encap/decap flags default to NONE, so an unset field is a tunnel
        # that DISCARDS the indication. These two tokens are what a real VPP prints for
        # the flags ecnFullFunctionality sets (ike/dataplane/vpp.go, InstallSA). The
        # unit tests agree with the generated binapi by construction; only this run says
        # VPP accepted the flags and holds them.
        "encap-copy-ecn",
        "decap-copy-ecn",
    ):
        if needle not in detail:
            sys.stderr.write(
                f"FAIL: real VPP SA does not report {needle!r}\n{detail}\n"
            )
            return 1
    print("OK: real VPP reports the AEAD cipher key and its salt in their own fields")
    print("OK: real VPP holds the ECN copy flags RFC 7296 Section 2.24 requires")
    print(detail)

    # `show ipsec all` prints every SPD; `show ipsec spd <id>` takes a runtime index
    # rather than the spd_id, so this reads the whole thing.
    spd_text = vppctl_text(container, "show ipsec all")
    if f"spd {ids['spd-id']}" not in spd_text:
        sys.stderr.write(f"FAIL: real VPP holds no SPD {ids['spd-id']}\n{spd_text}\n")
        return 1
    if f"{ids['spd-id']} -> {iface}" not in spd_text:
        sys.stderr.write(
            f"FAIL: SPD {ids['spd-id']} is not bound to {iface}\n{spd_text}\n"
        )
        return 1
    outbound = [
        line.strip() for line in spd_text.splitlines() if "type ip4-outbound" in line
    ]
    if len(outbound) != 2:
        sys.stderr.write(
            f"FAIL: want two outbound policies, got {outbound}\n{spd_text}\n"
        )
        return 1
    # PRIORITY ORDER, measured rather than assumed: VPP prints the chain in stored
    # order. Ze ranks LOWER first and negates, so the IKE bypass (Ze 100, VPP -100)
    # must be listed AHEAD of the child SA policy (Ze 2000, VPP -2000).
    if "priority -100 action bypass" not in outbound[0]:
        sys.stderr.write(f"FAIL: the IKE bypass must sort first, got {outbound}\n")
        return 1
    if "priority -2000 action protect" not in outbound[1]:
        sys.stderr.write(
            f"FAIL: the child SA policy must sort second, got {outbound}\n"
        )
        return 1
    # PROTOCOL, measured rather than assumed: Ze's any-protocol 0 must reach VPP as
    # its own any. Passing the zero through made VPP read IP protocol 0, hop-by-hop
    # options, and the policy then matched no traffic at all.
    if "protocol any" not in outbound[1]:
        sys.stderr.write(
            f"FAIL: the child SA policy must match every protocol, got {outbound[1]}\n"
        )
        return 1
    print(
        f"OK: real VPP holds SPD {ids['spd-id']} bound to {iface}, with both policies"
    )
    print(spd_text)
    return 0


def policer_name(iface: str) -> str:
    return f"ze/{iface}/{TRAFFIC_POLICER_CLASS}"


def policer_present(container: str, name: str) -> tuple[bool, str]:
    text = vppctl_text(container, "show policer")
    return name in text, text


def policer_feature_bound(container: str, iface: str) -> tuple[bool, str]:
    text = vppctl_text(container, f"show interface features {iface}")
    return "policer" in text.lower(), text


def wait_policer(
    container: str, name: str, want_present: bool, timeout_s: float
) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = policer_present(container, name)
        last = text
        if present == want_present:
            return True, text
        time.sleep(0.5)
    return False, last


def wait_policer_bound(
    container: str, iface: str, timeout_s: float
) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = policer_feature_bound(container, iface)
        last = text
        if present:
            return True, text
        time.sleep(0.5)
    return False, last


def wait_log(lines: list[str], needle: str, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if any(needle in line for line in lines):
            return True
        time.sleep(0.1)
    return False


def stop_peer(container: str, process_name: str) -> None:
    run(
        ["docker", "exec", container, "pkill", "-TERM", "-f", process_name],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def wait_route(
    container: str, want_present: bool, timeout_s: float
) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = route_present(container)
        last = text
        if present == want_present:
            return True, text
        time.sleep(0.5)
    return False, last


def ze_env(
    container: str, ze: Path, root: Path, config_path: Path, port: int | None = None
) -> list[str]:
    env = [
        "docker",
        "exec",
        "--interactive",
        "--env",
        "ZE_LOG_VPP=info",
        "--env",
        "ZE_LOG_FIB_VPP=debug",
        "--env",
        "ZE_LOG_TRAFFIC=debug",
        "--env",
        "ZE_LOG_TRAFFIC_VPP=debug",
        "--env",
        "ZE_LOG_FIREWALL=debug",
        "--env",
        "ZE_LOG_FIREWALL_VPP=debug",
        "--env",
        "ZE_LOG_BGP=info",
        "--env",
        "ZE_STORAGE_BLOB=false",
        "--env",
        "ZE_CONFIG_DIR=/run/vpp/ze",
    ]
    if port is not None:
        env.extend(["--env", f"ZE_TEST_BGP_PORT={port}"])
    # `start <config>`: the bare `ze <config>` launch form was removed from the CLI
    # (learned 1248), so a positional path now dies with "unknown command".
    env.extend([container, f"/src/{ze.relative_to(root)}", "start", str(config_path)])
    return env


def start_ze(
    container: str, ze: Path, root: Path, config_path: Path, port: int | None = None
) -> tuple[subprocess.Popen[str], list[str]]:
    daemon = subprocess.Popen(
        ze_env(container, ze, root, config_path, port),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert daemon.stderr is not None
    lines = drain("ze> ", daemon.stderr)
    return daemon, lines


def vpp_config(api_sock: Path) -> str:
    return f"""vpp {{
    enabled true;
    external true;
    api-socket {api_sock};
    stats {{ socket-path /run/vpp/stats.sock; }}
}}
"""


def fib_config(api_sock: Path) -> str:
    return f"""bgp {{
    peer peer1 {{
        connection {{
            remote {{ ip 127.0.0.1; }}
            local  {{ ip 127.0.0.1; accept false; }}
        }}
        session {{
            asn {{ local 1; remote 1; }}
            router-id 1.2.3.4;
            family {{ ipv4/unicast {{ prefix {{ maximum 10000; }} }} }}
            capability {{ graceful-restart disable; }}
        }}
        behavior {{ group-updates disable; }}
    }}
}}

{vpp_config(api_sock).rstrip()}
fib {{
    vpp {{ enabled true; }}
}}
"""


def traffic_config(api_sock: Path, iface: str, with_interface: bool) -> str:
    if not with_interface:
        return (
            vpp_config(api_sock)
            + "\ntraffic {\n    control {\n        backend vpp;\n    }\n}\n"
        )
    return (
        vpp_config(api_sock)
        + f"""
traffic {{
    control {{
        backend vpp;
        interface {iface} {{
            qdisc {{
                type htb;
                default-class {TRAFFIC_POLICER_CLASS};
                class {TRAFFIC_POLICER_CLASS} {{
                    rate 1mbit;
                    ceil 2mbit;
                }}
            }}
        }}
    }}
}}
"""
    )


TRAFFIC_PROTO_CLASS = "tcp"
TRAFFIC_PROTO_NUMBER = 6  # TCP


def traffic_protocol_config(api_sock: Path, iface: str) -> str:
    """Single HTB class carrying a protocol filter, so its policer is bound via
    the ingress policer-classify pipeline (ip4+ip6 classify tables) instead of
    the egress policer-output path."""
    return (
        vpp_config(api_sock)
        + f"""
traffic {{
    control {{
        backend vpp;
        interface {iface} {{
            qdisc {{
                type htb;
                default-class {TRAFFIC_PROTO_CLASS};
                class {TRAFFIC_PROTO_CLASS} {{
                    rate 1mbit;
                    ceil 2mbit;
                    match protocol {{ value {TRAFFIC_PROTO_NUMBER}; }}
                }}
            }}
        }}
    }}
}}
"""
    )


def proto_policer_name(iface: str) -> str:
    return f"ze/{iface}/{TRAFFIC_PROTO_CLASS}"


# --- DSCP (police-by-dscp) + multi-class steering evidence configs -----------

TRAFFIC_DSCP_CLASS = "cs6"
TRAFFIC_DSCP_VALUE = 48  # cs6


def traffic_dscp_config(api_sock: Path, iface: str) -> str:
    """Single HTB class carrying a dscp filter (police-by-dscp): the class
    policer is bound via the ingress policer-classify pipeline (ip4+ip6 classify
    tables matching the TOS/TC bits), NOT a QoS remark."""
    return (
        vpp_config(api_sock)
        + f"""
traffic {{
    control {{
        backend vpp;
        interface {iface} {{
            qdisc {{
                type htb;
                default-class {TRAFFIC_DSCP_CLASS};
                class {TRAFFIC_DSCP_CLASS} {{
                    rate 1mbit;
                    ceil 2mbit;
                    match dscp {{ value {TRAFFIC_DSCP_VALUE}; }}
                }}
            }}
        }}
    }}
}}
"""
    )


def dscp_policer_name(iface: str) -> str:
    return f"ze/{iface}/{TRAFFIC_DSCP_CLASS}"


TRAFFIC_MC_CLASS_A = "web"
TRAFFIC_MC_PROTO_A = 6  # TCP
TRAFFIC_MC_CLASS_B = "dns"
TRAFFIC_MC_PROTO_B = 17  # UDP


def traffic_multiclass_config(api_sock: Path, iface: str) -> str:
    """Two HTB classes, each with a DIFFERENT protocol filter: same-field
    multi-class steering. classify programs ONE table per family with two
    sessions steering to two distinct per-class policers (no chaining)."""
    return (
        vpp_config(api_sock)
        + f"""
traffic {{
    control {{
        backend vpp;
        interface {iface} {{
            qdisc {{
                type htb;
                class {TRAFFIC_MC_CLASS_A} {{
                    rate 10mbit;
                    ceil 100mbit;
                    match protocol {{ value {TRAFFIC_MC_PROTO_A}; }}
                }}
                class {TRAFFIC_MC_CLASS_B} {{
                    rate 1mbit;
                    ceil 100mbit;
                    match protocol {{ value {TRAFFIC_MC_PROTO_B}; }}
                }}
            }}
        }}
    }}
}}
"""
    )


def mc_policer_name(iface: str, cls: str) -> str:
    return f"ze/{iface}/{cls}"


def classify_tables_present(container: str) -> tuple[bool, str]:
    text = vppctl_text(container, "show classify tables")
    return "No classifier tables configured" not in text, text


def policer_classify_bound(container: str, iface: str) -> tuple[bool, str]:
    text = vppctl_text(container, f"show interface features {iface}")
    return "policer-classify" in text.lower(), text


def wait_condition(probe, want: bool, timeout_s: float) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = probe()
        last = text
        if present == want:
            return True, text
        time.sleep(0.5)
    return False, last


def write_config(path: Path, content: str) -> None:
    path.write_text(content, encoding="utf-8")


def run_fib_evidence(
    container: str, root: Path, ze: Path, ze_test: Path, work: Path, api_sock: Path
) -> int:
    port = free_port()
    peer_script = work / "peer-script"
    peer_script.write_text(
        "option=tcp_connections:value=1\n"
        f"option=update:value=send-route:prefix={PREFIX}:next-hop={NEXT_HOP}:origin-as=65001\n",
        encoding="utf-8",
    )
    peer = subprocess.Popen(
        [
            "docker",
            "exec",
            container,
            f"/src/{ze_test.relative_to(root)}",
            "peer",
            "--mode",
            "sink",
            "--port",
            str(port),
            "/run/vpp/peer-script",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert peer.stderr is not None
    drain("peer-err> ", peer.stderr)
    if not wait_for_peer(peer, 5):
        terminate(peer)
        raise SystemExit("ze-test peer did not start")

    config_path = work / "fib.conf"
    write_config(config_path, fib_config(api_sock))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/fib.conf"), port)
    try:
        ok, last_fib = wait_route(container, True, 25)
        if not ok:
            sys.stderr.write("FAIL: real VPP FIB route not observed\n")
            sys.stderr.write(last_fib)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP FIB contains {PREFIX}")

        stop_peer(container, ze_test.name)
        try:
            peer.wait(timeout=5)
        except subprocess.TimeoutExpired:
            terminate(peer)
        withdrawn, last_fib = wait_route(container, False, 15)
        if withdrawn:
            print(f"OK: real VPP FIB withdrew {PREFIX}")
            return 0

        sys.stderr.write("FAIL: real VPP FIB route was not withdrawn\n")
        sys.stderr.write(last_fib)
        sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
        return 1
    finally:
        terminate(daemon)
        terminate(peer)


def mpls_fib_config(api_sock: Path) -> str:
    return f"""bgp {{
    peer peer1 {{
        connection {{
            remote {{ ip 127.0.0.1; }}
            local  {{ ip 127.0.0.1; accept false; }}
        }}
        session {{
            asn {{ local 1; remote 1; }}
            router-id 1.2.3.4;
            family {{
                ipv4/unicast {{ prefix {{ maximum 10000; }} }}
                ipv4/mpls-label {{ prefix {{ maximum 10000; }} }}
            }}
            capability {{ graceful-restart disable; }}
        }}
        behavior {{ group-updates disable; }}
    }}
}}

{vpp_config(api_sock).rstrip()}
fib {{
    vpp {{ enabled true; }}
}}
"""


def mpls_route_present(container: str) -> tuple[bool, str]:
    out = vppctl(container, f"show ip fib {MPLS_PREFIX}")
    text = (out.stdout or "") + (out.stderr or "")
    has_prefix = MPLS_PREFIX in text
    has_label = str(MPLS_LABEL) in text and "label" in text.lower()
    return has_prefix and has_label, text


def wait_mpls_route(
    container: str, want_present: bool, timeout_s: float
) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = mpls_route_present(container)
        last = text
        if present == want_present:
            return True, text
        time.sleep(0.5)
    return False, last


def run_mpls_evidence(
    container: str, root: Path, ze: Path, ze_test: Path, work: Path, api_sock: Path
) -> int:
    port = free_port()
    peer_script = work / "mpls-peer-script"
    peer_script.write_text(
        "option=tcp_connections:value=1\n"
        f"option=update:value=send-route:prefix={MPLS_PREFIX}:next-hop={NEXT_HOP}:origin-as=65001:label={MPLS_LABEL}\n",
        encoding="utf-8",
    )
    peer = subprocess.Popen(
        [
            "docker",
            "exec",
            container,
            f"/src/{ze_test.relative_to(root)}",
            "peer",
            "--mode",
            "sink",
            "--port",
            str(port),
            "/run/vpp/mpls-peer-script",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert peer.stderr is not None
    drain("mpls-peer-err> ", peer.stderr)
    if not wait_for_peer(peer, 5):
        terminate(peer)
        raise SystemExit("ze-test peer (mpls) did not start")

    config_path = work / "mpls-fib.conf"
    write_config(config_path, mpls_fib_config(api_sock))
    daemon, ze_lines = start_ze(
        container, ze, root, Path("/run/vpp/mpls-fib.conf"), port
    )
    try:
        ok, last_fib = wait_mpls_route(container, True, 25)
        if not ok:
            sys.stderr.write("FAIL: real VPP MPLS label push route not observed\n")
            sys.stderr.write(last_fib)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP FIB contains {MPLS_PREFIX} with MPLS label {MPLS_LABEL}")

        stop_peer(container, ze_test.name)
        try:
            peer.wait(timeout=5)
        except subprocess.TimeoutExpired:
            terminate(peer)

        withdrawn, last_fib = wait_mpls_route(container, False, 15)
        if withdrawn:
            print(f"OK: real VPP FIB withdrew MPLS route {MPLS_PREFIX}")
            return 0

        sys.stderr.write("FAIL: real VPP MPLS route was not withdrawn\n")
        sys.stderr.write(last_fib)
        sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
        return 1
    finally:
        terminate(daemon)
        terminate(peer)


def run_traffic_evidence(
    container: str, root: Path, ze: Path, work: Path, api_sock: Path, iface: str
) -> int:
    name = policer_name(iface)
    config_path = work / "traffic.conf"
    write_config(config_path, traffic_config(api_sock, iface, True))

    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic.conf"))
    try:
        if not wait_log(ze_lines, "traffic-control config applied", 25):
            sys.stderr.write("FAIL: traffic-control apply log not observed\n")
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        ok, last = wait_policer(container, name, True, 15)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP policer {name} not observed after apply\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        bound, features = wait_policer_bound(container, iface, 15)
        if not bound:
            sys.stderr.write(
                f"FAIL: real VPP policer feature not observed on {iface}\n"
            )
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP traffic policer {name} exists and is bound to {iface}")

    finally:
        terminate(daemon)

    write_config(config_path, traffic_config(api_sock, iface, True))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic.conf"))
    try:
        ok, last = wait_policer(container, name, True, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP policer {name} missing after ze restart with same config\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        bound, features = wait_policer_bound(container, iface, 15)
        if not bound:
            sys.stderr.write(
                f"FAIL: real VPP policer feature not observed on {iface} after ze restart\n"
            )
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP traffic policer {name} survived ze restart with same config"
        )
    finally:
        terminate(daemon)

    write_config(config_path, traffic_config(api_sock, iface, False))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic.conf"))
    try:
        ok, last = wait_policer(container, name, False, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP orphan policer {name} survived ze restart cleanup\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP startup cleanup removed orphan traffic policer {name}")
        return 0
    finally:
        terminate(daemon)


def run_traffic_protocol_evidence(
    container: str, root: Path, ze: Path, work: Path, api_sock: Path, iface: str
) -> int:
    """AC-1: a `filter protocol` class programs classify tables that are bound
    to the interface's policer-classify feature (the R-1 "table created but
    never attached" killer), and its policer exists. Runs after
    run_traffic_evidence, which leaves the interface clean."""
    name = proto_policer_name(iface)
    config_path = work / "traffic-proto.conf"
    write_config(config_path, traffic_protocol_config(api_sock, iface))

    daemon, ze_lines = start_ze(
        container, ze, root, Path("/run/vpp/traffic-proto.conf")
    )
    try:
        if not wait_log(ze_lines, "traffic-control config applied", 25):
            sys.stderr.write("FAIL: traffic-control protocol apply log not observed\n")
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        ok, last = wait_policer(container, name, True, 15)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP protocol-filter policer {name} not observed\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        tables_ok, tables = wait_condition(
            lambda: classify_tables_present(container), True, 15
        )
        if not tables_ok:
            sys.stderr.write(
                "FAIL: real VPP classify tables not observed after apply\n"
            )
            sys.stderr.write(tables)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        # R-1 killer: the classify table must be ATTACHED to the interface's
        # policer-classify feature, not merely created.
        bound_ok, features = wait_condition(
            lambda: policer_classify_bound(container, iface), True, 15
        )
        if not bound_ok:
            sys.stderr.write(
                f"FAIL: policer-classify feature not bound on {iface} (R-1: table never attached)\n"
            )
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP protocol filter programmed classify tables bound to {iface} steering to policer {name}"
        )
    finally:
        terminate(daemon)

    # Removal: dropping the interface tears down the classify binding and
    # deletes the policer (in-process reconcile).
    write_config(config_path, traffic_config(api_sock, iface, False))
    daemon, ze_lines = start_ze(
        container, ze, root, Path("/run/vpp/traffic-proto.conf")
    )
    try:
        ok, last = wait_policer(container, name, False, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP protocol-filter policer {name} survived removal\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP protocol-filter policer {name} removed on reconcile")
        return 0
    finally:
        terminate(daemon)


def run_traffic_dscp_evidence(
    container: str, root: Path, ze: Path, work: Path, api_sock: Path, iface: str
) -> int:
    """AC-2 (police-by-dscp): a `filter dscp` class programs classify tables
    that are bound to the interface's policer-classify feature and steer the
    DSCP-matched traffic to the class policer -- the SAME pipeline as protocol,
    NOT a QoS remark. Proves the TOS/TC classify offsets are accepted by real
    VPP. Runs after run_traffic_protocol_evidence, which leaves the iface clean."""
    name = dscp_policer_name(iface)
    config_path = work / "traffic-dscp.conf"
    write_config(config_path, traffic_dscp_config(api_sock, iface))

    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic-dscp.conf"))
    try:
        if not wait_log(ze_lines, "traffic-control config applied", 25):
            sys.stderr.write("FAIL: traffic-control dscp apply log not observed\n")
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        ok, last = wait_policer(container, name, True, 15)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP dscp-filter policer {name} not observed\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        tables_ok, tables = wait_condition(
            lambda: classify_tables_present(container), True, 15
        )
        if not tables_ok:
            sys.stderr.write("FAIL: real VPP dscp classify tables not observed\n")
            sys.stderr.write(tables)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        bound_ok, features = wait_condition(
            lambda: policer_classify_bound(container, iface), True, 15
        )
        if not bound_ok:
            sys.stderr.write(
                f"FAIL: policer-classify feature not bound on {iface} for dscp (R-1)\n"
            )
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP dscp filter programmed classify tables bound to {iface} steering to policer {name}"
        )
    finally:
        terminate(daemon)

    write_config(config_path, traffic_config(api_sock, iface, False))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic-dscp.conf"))
    try:
        ok, last = wait_policer(container, name, False, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP dscp-filter policer {name} survived removal\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(f"OK: real VPP dscp-filter policer {name} removed on reconcile")
        return 0
    finally:
        terminate(daemon)


def run_traffic_multiclass_evidence(
    container: str, root: Path, ze: Path, work: Path, api_sock: Path, iface: str
) -> int:
    """AC-5: a multi-class HTB where every class carries a protocol filter
    programs per-class policers steered by classify. Same-field classes share
    ONE table per family with a session per class -> distinct policers. Proves
    real VPP accepts per-class steering (both policers exist + tables bound)."""
    name_a = mc_policer_name(iface, TRAFFIC_MC_CLASS_A)
    name_b = mc_policer_name(iface, TRAFFIC_MC_CLASS_B)
    config_path = work / "traffic-mc.conf"
    write_config(config_path, traffic_multiclass_config(api_sock, iface))

    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic-mc.conf"))
    try:
        if not wait_log(ze_lines, "traffic-control config applied", 25):
            sys.stderr.write(
                "FAIL: traffic-control multi-class apply log not observed\n"
            )
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        for name in (name_a, name_b):
            ok, last = wait_policer(container, name, True, 15)
            if not ok:
                sys.stderr.write(
                    f"FAIL: real VPP multi-class policer {name} not observed\n"
                )
                sys.stderr.write(last)
                sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
                return 1
        bound_ok, features = wait_condition(
            lambda: policer_classify_bound(container, iface), True, 15
        )
        if not bound_ok:
            sys.stderr.write(
                f"FAIL: policer-classify feature not bound on {iface} for multi-class\n"
            )
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP multi-class steering: policers {name_a} + {name_b} exist, classify bound to {iface}"
        )
    finally:
        terminate(daemon)

    write_config(config_path, traffic_config(api_sock, iface, False))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/traffic-mc.conf"))
    try:
        for name in (name_a, name_b):
            ok, last = wait_policer(container, name, False, 25)
            if not ok:
                sys.stderr.write(
                    f"FAIL: real VPP multi-class policer {name} survived removal\n"
                )
                sys.stderr.write(last)
                sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
                return 1
        print(
            f"OK: real VPP multi-class policers {name_a} + {name_b} removed on reconcile"
        )
        return 0
    finally:
        terminate(daemon)


FIREWALL_ACL_TAG = "ze/wan/input"


def firewall_config(api_sock: Path, with_rules: bool) -> str:
    """A firewall config the VPP backend can express in full.

    It carried a conntrack term (`connection-state established,related`) whose leaf is
    annotated ze:backend "nft" (internal/component/firewall/yang/ze-firewall-conf.yang),
    so the vpp backend REFUSED the commit and ze never started. The refusal is correct
    (ai/rules/protocol.md), and asking a VPP evidence run for an nft-only match was
    never something this test could prove.
    """
    if not with_rules:
        return vpp_config(api_sock) + "\nfirewall {\n    backend vpp;\n}\n"
    return (
        vpp_config(api_sock)
        + """
firewall {
    backend vpp;
    table wan {
        family inet;
        chain input {
            type filter;
            hook input;
            priority 0;
            policy drop;
            term allow-ssh {
                from {
                    protocol tcp;
                    destination-port 22;
                }
                then {
                    accept;
                }
            }
            term drop-all {
                then {
                    drop;
                }
            }
        }
    }
}
"""
    )


def acl_present(container: str, tag: str) -> tuple[bool, str]:
    text = vppctl_text(container, "show acl-plugin acl")
    return tag in text, text


def wait_acl(
    container: str, tag: str, want_present: bool, timeout_s: float
) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        present, text = acl_present(container, tag)
        last = text
        if present == want_present:
            return True, text
        time.sleep(0.5)
    return False, last


def acl_bound_to_interface(container: str, iface: str) -> tuple[bool, str]:
    text = vppctl_text(container, f"show acl-plugin interface {iface}")
    return "input acl" in text.lower() or "inbound" in text.lower(), text


def wait_acl_bound(container: str, iface: str, timeout_s: float) -> tuple[bool, str]:
    last = ""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        bound, text = acl_bound_to_interface(container, iface)
        last = text
        if bound:
            return True, text
        time.sleep(0.5)
    return False, last


def run_firewall_evidence(
    container: str, root: Path, ze: Path, work: Path, api_sock: Path, iface: str
) -> int:
    config_path = work / "firewall.conf"
    write_config(config_path, firewall_config(api_sock, True))

    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/firewall.conf"))
    try:
        if not wait_log(ze_lines, "firewall config applied", 25):
            sys.stderr.write("FAIL: firewall config apply log not observed\n")
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        ok, last = wait_acl(container, FIREWALL_ACL_TAG, True, 15)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP ACL with tag {FIREWALL_ACL_TAG} not observed after apply\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        bound, features = wait_acl_bound(container, iface, 15)
        if not bound:
            sys.stderr.write(f"FAIL: real VPP ACL not bound to interface {iface}\n")
            sys.stderr.write(features)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP firewall ACL {FIREWALL_ACL_TAG} exists and is bound to {iface}"
        )
    finally:
        terminate(daemon)

    write_config(config_path, firewall_config(api_sock, True))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/firewall.conf"))
    try:
        ok, last = wait_acl(container, FIREWALL_ACL_TAG, True, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP ACL {FIREWALL_ACL_TAG} missing after ze restart with same config\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP firewall ACL {FIREWALL_ACL_TAG} survived ze restart with same config"
        )
    finally:
        terminate(daemon)

    write_config(config_path, firewall_config(api_sock, False))
    daemon, ze_lines = start_ze(container, ze, root, Path("/run/vpp/firewall.conf"))
    try:
        ok, last = wait_acl(container, FIREWALL_ACL_TAG, False, 25)
        if not ok:
            sys.stderr.write(
                f"FAIL: real VPP orphan ACL {FIREWALL_ACL_TAG} survived ze restart cleanup\n"
            )
            sys.stderr.write(last)
            sys.stderr.write("\nze log tail:\n" + "".join(ze_lines[-80:]))
            return 1
        print(
            f"OK: real VPP startup cleanup removed orphan firewall ACL {FIREWALL_ACL_TAG}"
        )
        return 0
    finally:
        terminate(daemon)


def main() -> int:
    require_cmd("docker")
    root = repo_root()
    ensure_image()
    ze, ze_test = ensure_linux_binaries(root)
    ipsec_probe = ensure_ipsec_probe(root)

    tmp_parent = root / "tmp" / "evidence"
    tmp_parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="vpp-real-", dir=tmp_parent))
    ze_config_dir = work / "ze"
    ze_config_dir.mkdir(parents=True, exist_ok=True)
    api_sock = Path("/run/vpp/api.sock")

    startup = work / "startup.conf"
    startup.write_text(
        "unix {\n"
        "  nodaemon\n"
        "  cli-listen /run/vpp/cli.sock\n"
        "  log /run/vpp/vpp.log\n"
        "}\n\n"
        "api-segment {\n"
        "  prefix vpp\n"
        "}\n\n"
        "socksvr {\n"
        f"  socket-name {api_sock}\n"
        "}\n\n"
        "plugins {\n"
        "  plugin dpdk_plugin.so { disable }\n"
        "}\n\n"
        "statseg {\n"
        "  socket-name /run/vpp/stats.sock\n"
        "}\n",
        encoding="utf-8",
    )

    container = f"ze-vpp-evidence-{os.getpid()}"
    vpp = run(
        [
            "docker",
            "run",
            "--rm",
            "--detach",
            "--privileged",
            "--platform",
            VPP_PLATFORM,
            "--name",
            container,
            "-v",
            f"{root}:/src",
            "-v",
            f"{work}:/run/vpp",
            "-w",
            "/src",
            "--entrypoint",
            "sleep",
            VPP_IMAGE,
            "infinity",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if vpp.returncode != 0:
        sys.stderr.write(vpp.stderr or "")
        raise SystemExit("failed to start VPP container")

    try:
        start_vpp = run(
            [
                "docker",
                "exec",
                "--detach",
                container,
                "vpp",
                "-c",
                "/run/vpp/startup.conf",
            ]
        )
        if start_vpp.returncode != 0:
            raise SystemExit("failed to start VPP inside container")
        if not wait_for_path(work / "api.sock", 30):
            logs = run(
                ["docker", "logs", container],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            sys.stderr.write((logs.stdout or "") + (logs.stderr or ""))
            raise SystemExit("VPP API socket did not appear")
        if not wait_for_path(work / "cli.sock", 30):
            raise SystemExit("VPP CLI socket did not appear")

        version = vppctl(container, "show version")
        if version.returncode != 0:
            sys.stderr.write((version.stdout or "") + (version.stderr or ""))
            raise SystemExit("vppctl show version failed")
        sys.stderr.write(version.stdout or "")

        iface = create_loopback(container)
        print(f"OK: created real VPP loopback interface {iface}")

        # The IPsec case runs FIRST because it drives the backend over the API socket
        # directly, so it depends on nothing the ze daemon cases set up.
        ipsec_rc = run_ipsec_evidence(container, root, ipsec_probe, api_sock, iface)
        if ipsec_rc != 0:
            return ipsec_rc

        fib_rc = run_fib_evidence(container, root, ze, ze_test, work, api_sock)
        if fib_rc != 0:
            return fib_rc
        mpls_rc = run_mpls_evidence(container, root, ze, ze_test, work, api_sock)
        if mpls_rc != 0:
            return mpls_rc
        traffic_rc = run_traffic_evidence(container, root, ze, work, api_sock, iface)
        if traffic_rc != 0:
            return traffic_rc
        traffic_proto_rc = run_traffic_protocol_evidence(
            container, root, ze, work, api_sock, iface
        )
        if traffic_proto_rc != 0:
            return traffic_proto_rc
        traffic_dscp_rc = run_traffic_dscp_evidence(
            container, root, ze, work, api_sock, iface
        )
        if traffic_dscp_rc != 0:
            return traffic_dscp_rc
        traffic_mc_rc = run_traffic_multiclass_evidence(
            container, root, ze, work, api_sock, iface
        )
        if traffic_mc_rc != 0:
            return traffic_mc_rc
        return run_firewall_evidence(container, root, ze, work, api_sock, iface)
    finally:
        run(
            ["docker", "rm", "-f", container],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


if __name__ == "__main__":
    raise SystemExit(main())
