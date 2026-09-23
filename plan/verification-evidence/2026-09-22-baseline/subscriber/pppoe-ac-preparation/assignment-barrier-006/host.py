#!/usr/bin/env python3
"""Run the disposable AC proof through the registered native qemu run action.
Use from the repository root. This file has not been executed during preparation.
"""
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import time

root = Path.cwd()
prep = Path("plan/verification-evidence/2026-09-22-baseline/subscriber/pppoe-ac-preparation")
required = ("LE_BIN", "ZE_BIN", "KERNEL_PATH", "EVIDENCE_DIR")
for key in required:
    if not os.environ.get(key):
        raise SystemExit(f"{key} is required")
for key in ("ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"):
    if key in os.environ:
        raise SystemExit(f"refusing {key}")


def checkout_relative(value):
    # Keep lexical tmp paths: the native runner exports relocated tmp shares.
    return Path(os.path.abspath(value)).relative_to(root)


le = Path(os.path.abspath(os.environ["LE_BIN"]))
ze = checkout_relative(os.environ["ZE_BIN"])
kernel = Path(os.path.abspath(os.environ["KERNEL_PATH"]))
out = checkout_relative(os.environ["EVIDENCE_DIR"])
if prep not in out.parents:
    raise SystemExit("EVIDENCE_DIR must be a fresh child directory under pppoe-ac-preparation")
for path in (le, root / ze, kernel, root / prep / "guest.py"):
    if not path.is_file():
        raise SystemExit(f"missing input: {path}")
(root / out).mkdir(parents=True, exist_ok=False)
artifacts = root / out
scenario = Path("test/interop-pppoe/scenarios/02-ze-ac-pppd-client")
inputs = {"native": le, "daemon": root / ze, "kernel": kernel,
          "guest-driver": root / prep / "guest.py", "host-driver": Path(__file__),
          "scenario-config": root / scenario / "ze.conf",
          "native-checker": root / "internal/le/interoplab/pppoe/check_ac.go"}
provenance = {}
for name, path in inputs.items():
    with path.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    provenance[name] = {"path": str(path), "sha256": digest}
for name in ("host.py", "guest.py", "manifest.json"):
    shutil.copyfile(root / prep / name, artifacts / name)
shutil.copyfile(root / scenario / "ze.conf", artifacts / "scenario.conf")
shutil.copyfile(inputs["native-checker"], artifacts / "native-check_ac.go.txt")
(artifacts / "provenance.json").write_text(json.dumps(provenance, indent=2) + "\n")
guest = shlex.join(["python3", str(Path("/workspace") / out / "guest.py"),
                    "--ze", str(Path("/workspace") / ze),
                    "--scenario", str(Path("/workspace") / scenario),
                    "--out", str(Path("/workspace") / out / "guest")])
argv = [str(le), "le", "qemu", "run", "kernel", str(kernel), "timeout", "1200s",
        "packages", "python3 iproute2 iputils ppp ppp-pppoe kmod tcpdump curl",
        "command", guest, "|", "json"]
(artifacts / "invocation.json").write_text(json.dumps({"argv": argv, "cwd": str(root)}, indent=2) + "\n")
(artifacts / "started-at.txt").write_text(time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()) + "\n")
with (artifacts / "native.stdout").open("w") as stdout, (artifacts / "native.stderr").open("w") as stderr:
    result = subprocess.run(argv, stdout=stdout, stderr=stderr, check=False)
(artifacts / "native-exit.txt").write_text(str(result.returncode) + "\n")
(artifacts / "finished-at.txt").write_text(time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()) + "\n")
guest_result = artifacts / "guest/result.json"
passed = result.returncode == 0 and guest_result.is_file() and json.loads(guest_result.read_text()).get("status") == "pass"
print(f"{'PASS' if passed else 'FAIL'}: {artifacts}; inspect native.stdout, native.stderr and guest/result.json")
raise SystemExit(0 if passed else 1)
