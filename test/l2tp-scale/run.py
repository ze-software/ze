#!/usr/bin/env python3
"""Ze L2TP scale test runner.

Runs on loopback, no root required. Uses ze-test l2tp-scale for the
LAC simulator and mock RADIUS.

Usage:
    python3 test/l2tp-scale/run.py                  # run all scenarios
    python3 test/l2tp-scale/run.py 2k-sessions      # run specific scenario
    VERBOSE=1 python3 test/l2tp-scale/run.py         # verbose output
"""

import importlib.util
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "..", ".."))

sys.path.insert(0, SCRIPT_DIR)

from harness import log_fail, log_info, log_pass  # noqa: E402


def load_check(check_path):
    """Load a check.py module from path."""
    spec = importlib.util.spec_from_file_location("check", check_path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def main():
    scenario_filter = ""
    if len(sys.argv) > 1:
        scenario_filter = sys.argv[1]

    for name in ["ze", "ze-test"]:
        path = os.path.join(PROJECT_ROOT, "bin", name)
        if not os.path.isfile(path):
            print(
                "error: bin/%s not found (run 'make %s' first)"
                % (name, "ze" if name == "ze" else "test-runner"),
                file=sys.stderr,
            )
            sys.exit(1)

    print("")
    print("━" * 40)
    print(" Ze L2TP Scale Tests (loopback, no root)")
    print("━" * 40)
    print("")

    passed = 0
    failed = 0
    failed_names = []

    for scenario_name in sorted(os.listdir(SCRIPT_DIR)):
        scenario_dir = os.path.join(SCRIPT_DIR, scenario_name)
        if not os.path.isdir(scenario_dir):
            continue
        check_path = os.path.join(scenario_dir, "check.py")
        if not os.path.isfile(check_path):
            continue
        if scenario_filter and scenario_name != scenario_filter:
            continue

        print("── %s ──" % scenario_name)

        try:
            mod = load_check(check_path)
            mod.run()
            log_pass("PASS")
            passed += 1
        except BaseException as e:
            if isinstance(e, KeyboardInterrupt):
                log_fail("INTERRUPTED")
                failed += 1
                failed_names.append(scenario_name)
                break
            log_fail("FAIL: %s" % e)
            failed += 1
            failed_names.append(scenario_name)

        print("")

    if scenario_filter and passed + failed == 0:
        print(
            "error: no scenario matching '%s' found" % scenario_filter,
            file=sys.stderr,
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
