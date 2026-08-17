#!/usr/bin/env python3
"""Shared helpers for Ze L2TP scale tests.

Orchestrates Ze + ze-test l2tp-scale on loopback. No root, no
namespaces, no Docker. Runs on macOS and Linux.

Architecture:
    ze listens 127.0.0.1:<port>         (L2TP LNS)
    ze-test l2tp-scale --target ...     (LAC simulator + mock RADIUS)
"""

import atexit
import json
import os
import shutil
import signal
import subprocess
import sys
import time

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "120"))
except ValueError:
    SESSION_TIMEOUT = 120

VERBOSE = os.environ.get("VERBOSE", "0") == "1"

_processes = []


# --- Logging ----------------------------------------------------------------


def log_info(msg):
    print("  %s" % msg)


def log_pass(msg):
    print("  \033[32m✓ %s\033[0m" % msg)


def log_fail(msg):
    print("  \033[31m✗ %s\033[0m" % msg)


def log_debug(msg):
    if VERBOSE:
        print("  [debug] %s" % msg)


# --- Process helpers ---------------------------------------------------------


def _cleanup():
    for p in _processes:
        try:
            p.terminate()
            p.wait(timeout=5)
        except Exception:
            try:
                p.kill()
            except Exception:
                pass
    _processes.clear()


atexit.register(_cleanup)


def find_binary(name):
    """Find a built binary in bin/."""
    path = os.path.join(PROJECT_ROOT, "bin", name)
    if os.path.isfile(path):
        return path
    env_path = os.environ.get("%s_BINARY" % name.upper().replace("-", "_"), "")
    if env_path and os.path.isfile(env_path):
        return env_path
    return None


def find_free_port():
    """Find an available UDP port on loopback."""
    import socket

    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


# --- Scale Test Scenario -----------------------------------------------------


class ScaleScenario:
    """Manages a scale test: Ze process + ze-test l2tp-scale."""

    def __init__(self, scenario_dir, ze_binary=None, ze_test_binary=None):
        self.scenario_dir = scenario_dir
        self.ze_binary = ze_binary or find_binary("ze")
        self.ze_test_binary = ze_test_binary or find_binary("ze-test")
        self.ze_proc = None
        self.l2tp_port = 0
        self.result = None

    def setup(self, config_text=None, l2tp_port=None, extra_env=None):
        """Start Ze with L2TP enabled on a free port."""
        if not self.ze_binary:
            raise RuntimeError("ze binary not found (run 'make ze-build' first)")
        if not self.ze_test_binary:
            raise RuntimeError(
                "ze-test binary not found (run 'make test-runner' first)"
            )

        self.l2tp_port = l2tp_port or find_free_port()

        if config_text is None:
            config_text = self._default_config()

        env = dict(os.environ)
        env["ze.log.l2tp"] = "warn"
        env["ze.l2tp.skip-kernel-probe"] = "true"
        if extra_env:
            env.update(extra_env)

        self.ze_proc = subprocess.Popen(
            [self.ze_binary, "-"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
        )
        _processes.append(self.ze_proc)

        self.ze_proc.stdin.write(config_text.encode())
        self.ze_proc.stdin.close()

        time.sleep(0.5)
        if self.ze_proc.poll() is not None:
            stderr = self.ze_proc.stderr.read().decode()
            raise RuntimeError("ze exited early: %s" % stderr[:500])

        log_debug(
            "ze started on 127.0.0.1:%d (pid %d)" % (self.l2tp_port, self.ze_proc.pid)
        )

    def run_scale(
        self,
        tunnels=10,
        sessions=200,
        secret="s3cr3t",
        radius_delay="0s",
        dwell="2s",
        extra_args=None,
    ):
        """Run ze-test l2tp-scale and capture JSON result."""
        cmd = [
            self.ze_test_binary,
            "l2tp-scale",
            "--target",
            "127.0.0.1:%d" % self.l2tp_port,
            "--tunnels",
            str(tunnels),
            "--sessions",
            str(sessions),
            "--secret",
            secret,
            "--radius-delay",
            radius_delay,
            "--dwell",
            dwell,
            "--json",
        ]
        if extra_args:
            cmd.extend(extra_args)

        log_debug("running: %s" % " ".join(cmd))
        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=SESSION_TIMEOUT,
        )

        if proc.stdout.strip():
            try:
                self.result = json.loads(proc.stdout)
            except json.JSONDecodeError:
                log_fail("invalid JSON output: %s" % proc.stdout[:200])
                self.result = None

        if proc.returncode != 0:
            stderr_tail = proc.stderr[-500:] if proc.stderr else ""
            log_debug("ze-test l2tp-scale stderr: %s" % stderr_tail)

        return self.result

    def teardown(self):
        """Stop Ze."""
        if self.ze_proc and self.ze_proc.poll() is None:
            self.ze_proc.send_signal(signal.SIGTERM)
            try:
                self.ze_proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.ze_proc.kill()
                self.ze_proc.wait(timeout=5)
        if self.ze_proc in _processes:
            _processes.remove(self.ze_proc)
        self.ze_proc = None

    def _default_config(self):
        return """\
l2tp {{
    enabled true
    shared-secret s3cr3t
    auth {{
        local {{
            user test {{
                password testpass
            }}
        }}
    }}
    pool {{
        ipv4 {{
            gateway 10.255.0.1
            start 10.255.0.2
            end 10.255.15.254
            dns-primary 8.8.8.8
        }}
    }}
}}
environment {{
    l2tp {{
        server main {{
            ip 127.0.0.1
            port {port}
        }}
    }}
}}
""".format(port=self.l2tp_port)
