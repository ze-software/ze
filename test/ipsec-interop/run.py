#!/usr/bin/env python3
"""Ze IPsec Docker interop lab runner.

Usage:
    python3 test/ipsec-interop/run.py                          # run all scenarios
    python3 test/ipsec-interop/run.py 01-psk-site-to-site      # run specific scenario
    VERBOSE=1 python3 test/ipsec-interop/run.py                # verbose output

Environment:
    FRR_IMAGE       - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
    VERBOSE         - set to 1 for debug output
    NO_BUILD        - set to 1 to skip image builds
    SESSION_TIMEOUT - IKE establishment timeout in seconds (default: 90)
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

sys.path.insert(0, SCRIPT_DIR)

from lab import Scenario, log_fail, log_pass


def _any_scenario_needs_frr(scenarios_dir, scenario_filter):
    for name in sorted(os.listdir(scenarios_dir)):
        d = os.path.join(scenarios_dir, name)
        if not os.path.isdir(d):
            continue
        if scenario_filter and name != scenario_filter:
            continue
        if os.path.isfile(os.path.join(d, "frr.conf")):
            return True
    return False


def _feature_gate_tags():
    """Sorted ze_<feature> build tags from feature-gates.txt, the single source of
    truth. Derived, not hardcoded, so the lab tracks ZE_FEATURES when a gate is added
    -- see ai/rules/plugins.md.

    The IKE subsystem is one of those gates (ze_ike). A lab binary built without it
    holds no ipsec schema, so ze refuses its own scenario config with "unknown
    top-level keyword: vpn" and no IKE packet is ever sent.
    """
    tags = set()
    with open(os.path.join(PROJECT_ROOT, "feature-gates.txt"), encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            tags.add(line.split()[0])
    return sorted(tags)


def build_images(frr_image, no_build=False, need_frr=True):
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return

    ze_bin = os.path.join(PROJECT_ROOT, "bin", "ze-linux")
    build_tags = ",".join(["ze_core", "ze_distro"] + _feature_gate_tags())
    print("Cross-compiling ze for linux...")
    subprocess.run(
        ["go", "build", "-tags", build_tags, "-o", ze_bin, "./cmd/ze"],
        check=True,
        timeout=300,
        cwd=PROJECT_ROOT,
        env={**os.environ, "CGO_ENABLED": "0", "GOOS": "linux"},
    )
    ze_interop = os.path.join(SCRIPT_DIR, "ze-linux")
    import shutil

    shutil.copy2(ze_bin, ze_interop)

    print("Building Ze IPsec image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-ipsec-interop",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.ze"),
            PROJECT_ROOT,
            "-q",
        ],
        check=True,
        timeout=600,
    )

    print("Building strongSwan peer image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-ipsec-strongswan",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.strongswan"),
            SCRIPT_DIR,
            "-q",
        ],
        check=True,
        timeout=600,
    )

    if need_frr:
        print("Pulling FRR image...")
        subprocess.run(
            ["docker", "pull", "-q", frr_image],
            check=True,
            timeout=600,
        )


def main():
    frr_image = os.environ.get("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1")
    no_build = os.environ.get("NO_BUILD", "0") == "1"

    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    try:
        result = subprocess.run(
            ["docker", "info"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        docker_ok = result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        docker_ok = False
    if not docker_ok:
        # FAIL CLOSED, and say so on STDERR with the remediation, matching
        # test/interop/run.py. Every scenario runs in containers, so an unreachable
        # Docker means nothing was verified -- and a runner that reports an absence
        # on stdout with no next step leaves the reader to guess whether the lab was
        # skipped or broken (ai/rules/cli.md: what / why / next).
        print(
            "error: Docker unavailable, cannot run the IPsec interop lab -- every "
            "scenario runs in containers. Start Docker (or install it), then re-run: "
            "make ze-interop-ipsec-test",
            file=sys.stderr,
        )
        sys.exit(1)

    scenarios_dir = os.path.join(SCRIPT_DIR, "scenarios")
    need_frr = _any_scenario_needs_frr(scenarios_dir, scenario_filter)
    build_images(frr_image, no_build, need_frr=need_frr)

    print("")
    print("━" * 40)
    print(" Ze IPsec Interop Lab")
    print("━" * 40)
    print("")

    passed = 0
    failed = 0
    failed_names = []

    for scenario_name in sorted(os.listdir(scenarios_dir)):
        scenario_dir = os.path.join(scenarios_dir, scenario_name)
        if not os.path.isdir(scenario_dir):
            continue

        if scenario_filter and scenario_name != scenario_filter:
            continue

        check_path = os.path.join(scenario_dir, "check.py")
        if not os.path.isfile(check_path):
            continue

        print("── %s ──" % scenario_name)

        scenario = Scenario(scenario_dir, frr_image)
        try:
            scenario.setup()
            scenario.run_check()
            log_pass("PASS")
            passed += 1
        except BaseException as e:
            if isinstance(e, KeyboardInterrupt):
                log_fail("INTERRUPTED")
                failed += 1
                failed_names.append(scenario_name)
                scenario.teardown()
                break
            log_fail("FAIL: %s" % e)
            scenario.dump_logs()
            failed += 1
            failed_names.append(scenario_name)
        finally:
            scenario.teardown()

        print("")

    if scenario_filter and passed + failed == 0:
        print(
            "error: no scenario matching '%s' found" % scenario_filter, file=sys.stderr
        )
        sys.exit(1)

    print("━" * 40)
    if failed == 0:
        print("\033[32mPASS  %d scenario(s)\033[0m" % passed)
    else:
        print(
            "\033[31mFAIL  %d passed, %d failed: %s\033[0m"
            % (passed, failed, " ".join(failed_names))
        )
    print("━" * 40)

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
