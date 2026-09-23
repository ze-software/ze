#!/usr/bin/env python3
"""Exercise real child cancellation without starting another VM."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys
import time

here = Path(__file__).resolve().parent
out = here / 'executor-lifecycle-001'
out.mkdir(exist_ok=False)
source = here / 'rfc-native-exec.py'
spec = importlib.util.spec_from_file_location('native_exec_lifecycle', source)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
observations = []
for name, timeout, parent in [('deadline', 0.05, os.getppid()), ('parent-exited', 30, -1),
                              ('signal', 30, os.getppid())]:
    module.RUN_TIMEOUT_SECONDS = timeout
    module.PARENT_PID = parent
    started = time.monotonic()
    code = ('import os,signal,time; time.sleep(0.05); os.kill(os.getppid(), signal.SIGTERM)'
            if name == 'signal' else 'import time; time.sleep(60)')
    row = module.run([sys.executable, '-c', code],
                     Path.cwd(), os.environ.copy(), out, name)
    duration = time.monotonic() - started
    accepted = (row.get('stop_reason') == name and row['exit_code'] is not None and
                not row.get('execution_error') and duration < 4 and
                (name != 'signal' or row['exit_code'] == 0))
    observations.append({'case': name, 'accepted': accepted, 'duration_seconds': duration,
                         'exit_code': row['exit_code'], 'stop_reason': row.get('stop_reason')})
result = {'passed': all(row['accepted'] for row in observations), 'observations': observations,
          'wrapper_sha256': hashlib.sha256(source.read_bytes()).hexdigest(),
          'boundary': 'Real child cancellation through run(). Parent loss is injected by an impossible expected parent PID, not an actual Go-parent death. Full native Go-exec and forced SIGKILL cleanup are not exercised here.'}
(out / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2))
raise SystemExit(0 if result['passed'] else 1)
