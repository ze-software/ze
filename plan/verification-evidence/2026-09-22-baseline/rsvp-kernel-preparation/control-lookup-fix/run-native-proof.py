#!/usr/bin/env python3
"""Disposable recorder for actual native control tests and producer execution."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import time

root = Path('/workspace/plan/verification-evidence/2026-09-22-baseline/rsvp-kernel-preparation')
red, green, daemon, out = map(Path, sys.argv[1:])
if len(sys.argv) != 5 or not all(path.is_absolute() for path in (red, green, daemon, out)):
    raise SystemExit('require absolute RED, GREEN, daemon and new output paths')
if out.exists() or not out.resolve().is_relative_to(root):
    raise SystemExit('refuse existing or out-of-scope output')
out.mkdir(parents=True, exist_ok=False)

def digest(path):
    value = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            value.update(chunk)
    return value.hexdigest()

def save(name, value):
    (out / name).write_text(json.dumps(value, indent=2) + '\n')

def run(name, argv, timeout):
    row = {'argv': argv, 'started_ns': time.time_ns(), 'exit_code': None,
           'stdout': name + '.stdout', 'stderr': name + '.stderr'}
    with (out / row['stdout']).open('xb') as stdout, (out / row['stderr']).open('xb') as stderr:
        try:
            completed = subprocess.run(argv, stdout=stdout, stderr=stderr, timeout=timeout, check=False)
            row['exit_code'] = completed.returncode
        except (OSError, subprocess.TimeoutExpired) as exc:
            row['execution_error'] = str(exc)
    row['finished_ns'] = time.time_ns()
    save(name + '.json', row)
    text = (out / row['stdout']).read_text(errors='replace') + (out / row['stderr']).read_text(errors='replace')
    return row, text

inputs = [{'path': str(path), 'sha256_before': digest(path)} for path in (red, green, daemon)]
save('inputs.json', inputs)
unit = 'TestIntegrationRSVPPathAvoidsDataFEC'
red_row, red_text = run('red', [str(red), '-test.v', '-test.run=^' + unit + '$', '-test.timeout=30s'], 45)
red_expected = (red_row['exit_code'] == 1 and
                'ordinary PATH entered endpoint data FEC with labels [16011]' in red_text and
                re.search(r'^--- FAIL: ' + unit + r' \(', red_text, re.MULTILINE) is not None and
                '--- SKIP:' not in red_text)
names = ['PathCarriage', 'PathAvoidsDataFEC', 'SelectedBypass', 'ReceiveOversize', 'CloseIdle']
green_row, green_text = run('green', [str(green), '-test.v',
    '-test.run=^TestIntegrationRSVP(' + '|'.join(names) + ')$', '-test.timeout=60s'], 90)
green_pass = (green_row['exit_code'] == 0 and '--- SKIP:' not in green_text and '--- FAIL:' not in green_text and
              all(re.search(r'^--- PASS: TestIntegrationRSVP' + name + r' \(', green_text, re.MULTILINE) for name in names))
producer = {'status': 'not-run', 'reason': 'native transport cases did not all pass'}
if green_pass:
    producer, _ = run('producer-action', [sys.executable, str(root / 'capture-native-guest.py'),
        str(green), str(daemon), str(root / 'attempt-010')], 300)
for entry in inputs:
    entry['sha256_after'] = digest(Path(entry['path']))
save('inputs.json', inputs)
unchanged = all(entry['sha256_before'] == entry['sha256_after'] for entry in inputs)
result = {'red_expected_failure_observed': red_expected, 'green_five_cases_passed': bool(green_pass),
          'inputs_unchanged': unchanged, 'red_exit': red_row['exit_code'], 'green_exit': green_row['exit_code'],
          'producer_action': producer,
          'boundary': 'Native behavioral discrimination only. This does not mint RFC discrimination records or prove foreign-peer interoperability.'}
save('result.json', result)
print(json.dumps(result, indent=2))
raise SystemExit(0 if red_expected and green_pass and unchanged and producer.get('exit_code') == 0 else 1)
