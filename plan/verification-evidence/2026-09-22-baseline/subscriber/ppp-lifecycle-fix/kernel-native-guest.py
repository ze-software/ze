#!/usr/bin/env python3
"""Retain the old/new kernel regression and the independent PPP restart run."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

old, current, driver, daemon, destination = map(Path, sys.argv[1:6])
destination.mkdir(mode=0o700)

def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

name = 'TestRemovePointToPointAddressAllowsReuse'
for label, binary, expected in [('before', old, 1), ('current', current, 0)]:
    target = destination / ('kernel-' + label)
    target.mkdir(mode=0o700)
    argv = [str(binary), '-test.run=^' + name + '$', '-test.v', '-test.count=1', '-test.timeout=30s']
    record = {'argv': argv, 'kernel': list(os.uname()), 'binary_sha256': digest(binary),
              'started_unix': time.time(), 'expected_exit': expected}
    try:
        result = subprocess.run(argv, capture_output=True, timeout=60)
        record['exit_code'] = result.returncode
        stdout, stderr = result.stdout, result.stderr
        text = (stdout + stderr).decode('utf-8', errors='replace')
        marker = ('--- FAIL: ' if expected else '--- PASS: ') + name
        record['accepted'] = result.returncode == expected and marker in text and '--- SKIP:' not in text
        if expected:
            record['accepted'] = record['accepted'] and 'remove point-to-point address by local CIDR' in text and 'cannot assign requested address' in text
    except subprocess.TimeoutExpired as error:
        stdout, stderr = error.stdout or b'', error.stderr or b''
        record.update(timed_out=True, accepted=False)
    record['finished_unix'] = time.time()
    record['binary_sha256_after'] = digest(binary)
    record['accepted'] = record['accepted'] and record['binary_sha256'] == record['binary_sha256_after']
    (target / 'stdout.log').write_bytes(stdout)
    (target / 'stderr.log').write_bytes(stderr)
    (target / 'result.json').write_text(json.dumps(record, indent=2) + '\n')
    print(json.dumps(record), flush=True)
    if not record['accepted']:
        raise SystemExit(1)

collector = Path('/workspace/plan/verification-evidence/2026-09-22-baseline/subscriber/l2tp-credential-preparation/guest-exec.py')
result = subprocess.run(['python3', str(collector), str(driver), str(daemon), str(destination / 'subscriber')], timeout=270)
raise SystemExit(result.returncode)
