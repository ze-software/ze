#!/usr/bin/env python3
"""Ze L2TP PPP/NCP Docker interop lab runner.

Usage:
    python3 test/l2tp-interop/run.py                          # run all scenarios
    python3 test/l2tp-interop/run.py 01-ppp-ipv4              # run specific scenario
    VERBOSE=1 python3 test/l2tp-interop/run.py                # verbose output

Environment:
    FRR_IMAGE       - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
    VERBOSE         - set to 1 for debug output
    NO_BUILD        - set to 1 to skip image builds
    SESSION_TIMEOUT - session/PPP establishment timeout in seconds (default: 90)
"""

import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

sys.path.insert(0, SCRIPT_DIR)

from lab import Scenario, log_fail, log_pass, preflight_strict


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


def build_images(frr_image, no_build=False, need_frr=True):
    if no_build:
        print("  skipping image builds (NO_BUILD=1)")
        return

    print("Building Ze L2TP LNS image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-l2tp-interop",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.ze"),
            PROJECT_ROOT,
            "-q",
        ],
        check=True,
        timeout=600,
    )

    print("Building LAC peer image...")
    subprocess.run(
        [
            "docker",
            "build",
            "-t",
            "ze-l2tp-lac",
            "-f",
            os.path.join(SCRIPT_DIR, "Dockerfile.lac"),
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
        print("Docker unavailable, cannot run L2TP interop lab")
        sys.exit(1)

    preflight_strict()

    scenarios_dir = os.path.join(SCRIPT_DIR, "scenarios")
    need_frr = _any_scenario_needs_frr(scenarios_dir, scenario_filter)
    build_images(frr_image, no_build, need_frr=need_frr)

    print("")
    print("━" * 40)
    print(" Ze L2TP PPP/NCP Interop Lab")
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
