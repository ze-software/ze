#!/usr/bin/env python3
"""Ze PPPoE-client / accel-ppp Docker interop lab runner.

Usage:
    python3 test/pppoe-interop/run.py                       # run all scenarios
    python3 test/pppoe-interop/run.py 01-pppoe-chap-ipv4    # one scenario
    VERBOSE=1 python3 test/pppoe-interop/run.py             # verbose output

Environment:
    VERBOSE                  - set to 1 for debug output
    NO_BUILD                 - set to 1 to skip image builds
    SESSION_TIMEOUT          - PPP establishment timeout in seconds (default 90)
    ZE_PPPOE_INTEROP_SUFFIX  - container/network suffix (default PID)
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

sys.path.insert(0, SCRIPT_DIR)

from lab import Scenario, log_fail, log_pass, preflight_strict


def build_images(no_build=False):
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return

    print("Building Ze PPPoE-client image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-pppoe-interop",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.ze"),
            PROJECT_ROOT,
            "-q",
        ],
        check=True,
        timeout=600,
    )

    print("Building accel-ppp access-concentrator image (compiles from source)...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-pppoe-accel",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.accel"),
            SCRIPT_DIR,
            "-q",
        ],
        check=True,
        timeout=900,
    )

    print("Building pppd/rp-pppoe client image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-pppoe-client",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.client"),
            SCRIPT_DIR,
            "-q",
        ],
        check=True,
        timeout=600,
    )


def main():
    no_build = os.environ.get("NO_BUILD", "0") == "1"

    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    try:
        result = subprocess.run(
            ["docker", "info"], capture_output=True, text=True, timeout=15
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
            "error: Docker unavailable, cannot run the PPPoE interop lab -- every "
            "scenario runs in containers. Start Docker (or install it), then re-run: "
            "make ze-deployment-pppoe-accel-docker-test",
            file=sys.stderr,
        )
        sys.exit(1)

    preflight_strict()
    build_images(no_build)

    print("")
    print("━" * 40)
    print(" Ze PPPoE / accel-ppp Interop Lab")
    print("━" * 40)
    print("")

    scenarios_dir = os.path.join(SCRIPT_DIR, "scenarios")
    passed = 0
    failed = 0
    failed_names = []

    for scenario_name in sorted(os.listdir(scenarios_dir)):
        scenario_dir = os.path.join(scenarios_dir, scenario_name)
        if not os.path.isdir(scenario_dir):
            continue
        if scenario_filter and scenario_name != scenario_filter:
            continue
        if not os.path.isfile(os.path.join(scenario_dir, "check.py")):
            continue

        print("── %s ──" % scenario_name)

        scenario = Scenario(scenario_dir)
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
