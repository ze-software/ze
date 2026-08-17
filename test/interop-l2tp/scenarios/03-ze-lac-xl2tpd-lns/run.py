#!/usr/bin/env python3
"""Scenario 03: ze as L2TP initiator (LAC/dialer) vs a real xl2tpd LNS.

This is the inverse of scenarios 01/02 (where ze is the LNS answerer). Here
ze DIALS a real xl2tpd daemon running in LNS mode: it sends the SCCRQ,
xl2tpd answers SCCRP, ze sends SCCCN, and the L2TP control connection is
established on BOTH sides. This proves spec-followup-l2tp-call's new
initiator/LAC tunnel path (SCCRQ initiation, SCCRP handling, SCCCN)
interoperates with an independent RFC 2661 implementation.

Trigger: the `request l2tp outgoing-call` RPC, issued over ze's token-guarded
REST API. It dials the remote and then attempts an OCRQ. xl2tpd does NOT
implement the outgoing-call answerer side (it logs "message type 7
(Outgoing-Call-Request)" and closes the tunnel -- see README, A-6), so the
call itself does not complete against xl2tpd; the interop proof is the
established control connection. The full OCRQ/OCRP/OCCN call flow is proven
at the functional tier by test/l2tp/lns-outgoing-call.ci (Python LAC peer).

Topology: xl2tpd runs in Docker (--network host) as the peer; ze runs from
the repo's bin/ze as the system under test, with isolated filesystem storage
so it never touches the committed etc/ze blob. This mirrors the standard
interop pattern (containerised peer, native SUT) and needs no ze image build.

Usage:
    python3 test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns/run.py
    VERBOSE=1 python3 .../run.py     # dump container + ze logs on the console
"""

import json
import os
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

SCEN_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCEN_DIR, "..", "..", "..", ".."))
LAC_DOCKERFILE = os.path.join(SCEN_DIR, "..", "..", "Dockerfile.lac")

PEER_IMAGE = "ze-l2tp-lac"  # reuses the interop lab's xl2tpd image
PEER_CONTAINER = "ze-l2tp-03-lns-%d" % os.getpid()
XL2TPD_PORT = 17010  # xl2tpd LNS control port (host network)
ZE_LISTEN_PORT = 17011  # ze L2TP listener
ZE_REST_PORT = 17012  # ze REST API
TOKEN = "secret"
VERBOSE = os.environ.get("VERBOSE", "0") == "1"


def log(msg):
    print("  %s" % msg)


def log_pass(msg):
    print("  \033[32m✓ %s\033[0m" % msg)


def log_fail(msg):
    print("  \033[31m✗ %s\033[0m" % msg)


def run(cmd, **kw):
    return subprocess.run(cmd, capture_output=True, text=True, **kw)


def docker_logs(container):
    r = run(["docker", "logs", container], timeout=15)
    return r.stdout + r.stderr


def ensure_peer_image():
    have = run(["docker", "image", "inspect", PEER_IMAGE], timeout=30)
    if have.returncode == 0:
        return
    log("building xl2tpd peer image %s ..." % PEER_IMAGE)
    b = run(
        [
            "docker",
            "build",
            "-t",
            PEER_IMAGE,
            "-f",
            LAC_DOCKERFILE,
            os.path.dirname(LAC_DOCKERFILE),
        ],
        timeout=600,
    )
    if b.returncode != 0:
        raise RuntimeError("peer image build failed:\n%s" % (b.stdout + b.stderr))


def ensure_ze_binary():
    ze = os.path.join(PROJECT_ROOT, "bin", "ze")
    if os.path.isfile(ze):
        return ze
    log("building bin/ze ...")
    b = run(["make", "bin/ze"], cwd=PROJECT_ROOT, timeout=600)
    if b.returncode != 0:
        raise RuntimeError("bin/ze build failed:\n%s" % (b.stdout + b.stderr))
    return ze


def start_peer():
    run(["docker", "rm", "-f", PEER_CONTAINER], timeout=30)
    args = [
        "docker",
        "run",
        "-d",
        "--name",
        PEER_CONTAINER,
        "--network",
        "host",
        "-v",
        "%s:/etc/xl2tpd/xl2tpd.conf:ro" % os.path.join(SCEN_DIR, "xl2tpd.conf"),
        "-v",
        "%s:/etc/ppp/options.xl2tpd:ro" % os.path.join(SCEN_DIR, "options.xl2tpd"),
        PEER_IMAGE,
    ]
    r = run(args, timeout=60)
    if r.returncode != 0:
        raise RuntimeError(
            "xl2tpd container failed to start:\n%s" % (r.stdout + r.stderr)
        )
    # Wait for xl2tpd to bind its listener.
    for _ in range(30):
        if "Listening on IP address" in docker_logs(PEER_CONTAINER):
            return
        time.sleep(0.5)
    raise RuntimeError(
        "xl2tpd did not report Listening:\n%s" % docker_logs(PEER_CONTAINER)
    )


def start_ze(ze_bin, workdir, logfile):
    cfg = open(os.path.join(SCEN_DIR, "ze.conf"), "rb").read()
    env = dict(os.environ)
    env["ZE_STORAGE_BLOB"] = "false"  # filesystem storage, not the blob
    env["ze.l2tp.skip-kernel-probe"] = "true"
    env["ze.log.l2tp"] = "debug"
    lf = open(logfile, "wb")
    # cwd=workdir isolates any storage ze creates to the temp dir.
    proc = subprocess.Popen(
        [ze_bin, "-"], cwd=workdir, env=env, stdin=subprocess.PIPE, stdout=lf, stderr=lf
    )
    proc.stdin.write(cfg)
    proc.stdin.close()
    for _ in range(40):
        if os.path.isfile(logfile) and "L2TP listener bound" in open(logfile).read():
            return proc
        if proc.poll() is not None:
            raise RuntimeError(
                "ze exited early (rc=%s):\n%s" % (proc.returncode, open(logfile).read())
            )
        time.sleep(0.25)
    raise RuntimeError("ze did not bind its L2TP listener:\n%s" % open(logfile).read())


def trigger_dial():
    url = "http://127.0.0.1:%d/api/v1/execute" % ZE_REST_PORT
    payload = json.dumps(
        {"command": "request l2tp outgoing-call remote xl2tpd called 12345"}
    ).encode()
    req = urllib.request.Request(
        url,
        data=payload,
        headers={
            "Authorization": "Bearer " + TOKEN,
            "Content-Type": "application/json",
        },
    )
    # Poll the REST API until it accepts the request; the RPC blocks until the
    # dial reaches a terminal outcome.
    deadline = time.time() + 15
    while time.time() < deadline:
        try:
            resp = urllib.request.urlopen(req, timeout=25)
            return json.loads(resp.read())
        except urllib.error.HTTPError as e:
            return {"http-error": e.code, "body": e.read().decode("utf-8", "replace")}
        except (urllib.error.URLError, ConnectionRefusedError, OSError):
            time.sleep(0.4)
    raise RuntimeError("REST API never accepted the outgoing-call request")


def main():
    ensure_peer_image()
    ze_bin = ensure_ze_binary()
    workdir = tempfile.mkdtemp(prefix="ze-l2tp-03-")
    ze_log = os.path.join(workdir, "ze.log")
    ze_proc = None
    try:
        log(
            "starting xl2tpd LNS peer (Docker, --network host, port %d)..."
            % XL2TPD_PORT
        )
        start_peer()
        log_pass("xl2tpd LNS listening")

        log("starting ze initiator (native bin/ze)...")
        ze_proc = start_ze(ze_bin, workdir, ze_log)
        log_pass("ze L2TP listener bound; REST API up")

        log("triggering `request l2tp outgoing-call` (ze dials xl2tpd)...")
        rpc = trigger_dial()
        # The RPC surfaces an error because xl2tpd cannot answer the OCRQ; the
        # interop proof is the established control connection, asserted below.
        log("RPC result: %s" % json.dumps(rpc))

        time.sleep(1.0)
        ze_text = open(ze_log).read()
        peer_text = docker_logs(PEER_CONTAINER)
        if VERBOSE:
            print(
                "\n----- ze log -----\n%s\n----- xl2tpd log -----\n%s"
                % (ze_text, peer_text)
            )

        ok = True
        if "tunnel now established (initiator)" in ze_text:
            log_pass("ze established the tunnel as initiator (SCCRQ->SCCRP->SCCCN)")
        else:
            log_fail("ze did not log initiator tunnel establishment")
            ok = False

        if "Connection established" in peer_text:
            log_pass("xl2tpd confirms the control connection from ze is established")
        else:
            log_fail("xl2tpd did not confirm connection establishment")
            ok = False

        if "Outgoing-Call-Request" in peer_text:
            log_pass(
                "xl2tpd received ze's OCRQ (message type 7); it cannot answer it (A-6)"
            )
        else:
            log("(note) xl2tpd did not log the OCRQ; tunnel interop still proven")

        if not ok:
            raise AssertionError("initiator tunnel interop assertions failed")
        print(
            "\n\033[32mPASS  ze-initiator L2TP tunnel interoperates with xl2tpd LNS\033[0m"
        )
        return 0
    finally:
        run(["docker", "rm", "-f", PEER_CONTAINER], timeout=30)
        if ze_proc and ze_proc.poll() is None:
            ze_proc.terminate()
            try:
                ze_proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                ze_proc.kill()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:  # noqa: BLE001
        log_fail("FAIL: %s" % e)
        sys.exit(1)
