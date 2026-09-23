#!/usr/bin/env python3
"""Disposable Go -exec adapter; the native recorder owns all proof decisions."""
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import time

WORKSPACE = Path('/home/thomas/Code/github.com/ze-software/ze/main')
HERE = Path(__file__).resolve().parent
LE = WORKSPACE / 'tmp/session/2026-09-21-97650314-2770-467e-8578-1739803540cf/scratch/baseline-runtime-accepted/bin/le'
KERNEL = WORKSPACE / 'tmp/kernel/build/vmlinuz'
LE_SHA = 'f95c1c702156279e2258be8b222c0ca0821a2e4e4911004bf29487022a7b002a'
KERNEL_SHA = '48b3607b49b79e23e632f0d5a553ba9f7b87c4527ca3b40917eee11501758a83'
ENV_KEYS = ('ZE_TAGS', 'CGO_ENABLED', 'GOFLAGS', 'GOTMPDIR', 'GOCOVERDIR',
            'GOOS', 'GOARCH', 'GOROOT', 'GOTOOLCHAIN', 'GOENV', 'GOWORK',
            'GOCACHE', 'GOMODCACHE', 'GOMAXPROCS', 'GODEBUG', 'GOTRACEBACK')
PARENT_PID = os.getppid()
RUN_TIMEOUT_SECONDS = 720


def sha(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def save(path, value):
    with Path(path).open('x') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


def inside(value, base):
    path = Path(value)
    if not path.is_absolute():
        path = base / path
    path = path.resolve()
    path.relative_to(WORKSPACE)
    return path


def guest_path(path):
    return str(Path('/workspace') / path.relative_to(WORKSPACE))


def identity(path):
    return {'path': str(path), 'sha256': sha(path), 'bytes': path.stat().st_size}


def status(code):
    return code if code >= 0 else 128 - code


def run(argv, cwd, environment, out, name):
    row = {'argv': argv, 'cwd': str(cwd), 'started_ns': time.time_ns(),
           'exit_code': None, 'stdout': name + '.stdout', 'stderr': name + '.stderr',
           'parent_pid': PARENT_PID, 'timeout_seconds': RUN_TIMEOUT_SECONDS}
    row['environment_choices'] = {key: environment.get(key) for key in ENV_KEYS}
    save(out / (name + '-invocation.json'), row)
    with (out / row['stdout']).open('xb') as stdout, (out / row['stderr']).open('xb') as stderr:
        child = None
        handlers = {}
        deadline = time.monotonic() + RUN_TIMEOUT_SECONDS
        def cancellation_reason():
            return ('parent-exited' if PARENT_PID <= 1 or os.getppid() != PARENT_PID else
                    'signal' if row.get('signals') else
                    'deadline' if time.monotonic() >= deadline else None)
        try:
            child = subprocess.Popen(argv, cwd=cwd, env=environment,
                                     stdout=stdout, stderr=stderr, start_new_session=True)
            row['pid'] = child.pid
            row['signals'] = []
            def forward(signum, frame):
                row['signals'].append({'signal': signum, 'at_ns': time.time_ns()})
            def signal_child(signum):
                try:
                    os.killpg(child.pid, signum)
                except ProcessLookupError:
                    pass
            for signum in (signal.SIGINT, signal.SIGTERM):
                handlers[signum] = signal.signal(signum, forward)
            while True:
                reason = cancellation_reason()
                if reason:
                    row['stop_reason'] = reason
                    signal_child(signal.SIGTERM)
                    try:
                        row['exit_code'] = child.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        signal_child(signal.SIGKILL)
                        row['exit_code'] = child.wait(timeout=5)
                    break
                try:
                    row['exit_code'] = child.wait(timeout=1)
                    break
                except subprocess.TimeoutExpired:
                    continue
        except (OSError, subprocess.TimeoutExpired) as exc:
            row['execution_error'] = str(exc)
        finally:
            for signum, handler in handlers.items():
                signal.signal(signum, handler)
            reason = cancellation_reason()
            if reason:
                row.setdefault('stop_reason', reason)
    row['finished_ns'] = time.time_ns()
    save(out / (name + '-result.json'), row)
    return row


def guest(config_file):
    config = json.loads(Path(config_file).read_text())
    out = Path(config['guest_evidence'])
    environment = os.environ.copy()
    environment.pop('GOFLAGS', None)
    environment.update(config['guest_environment'])
    binary = Path(config['guest_argv'][0])
    before = identity(binary)
    if before['sha256'] != config['binary']['sha256'] or sha(__file__) != config['wrapper']['sha256']:
        raise RuntimeError('guest binary or wrapper differs from host identity')
    save(out / 'guest-context.json', {
        'argv': sys.argv, 'uname': list(os.uname()), 'uid': os.getuid(),
        'environment_choices': {key: environment.get(key) for key in ENV_KEYS},
        'binary_before': before, 'wrapper': identity(Path(__file__))})
    result = run(config['guest_argv'], config['guest_cwd'], environment, out, 'test')
    if result.get('stop_reason') or result.get('execution_error'):
        return 125
    save(out / 'guest-complete.json', {
        'exit_code': result['exit_code'], 'binary_after': identity(binary),
        'finished_ns': time.time_ns()})
    return status(result['exit_code']) if result['exit_code'] is not None else 125


def host(arguments):
    out = Path(tempfile.mkdtemp(prefix='rfc-exec-', dir=HERE))
    save(out / 'received.json', {
        'argv': sys.argv, 'cwd': os.getcwd(), 'started_ns': time.time_ns(),
        'environment_choices': {key: os.environ.get(key) for key in ENV_KEYS}})
    try:
        if not arguments:
            raise ValueError('Go -exec must supply the compiled test binary')
        cwd = inside(os.getcwd(), WORKSPACE)
        binary = inside(arguments[0], cwd)
        if not binary.is_file() or not os.access(binary, os.X_OK):
            raise ValueError('compiled test binary must be executable')
        if os.environ.get('CGO_ENABLED') != '0':
            raise ValueError('CGO_ENABLED=0 is required')
        if 'integration' not in os.environ.get('ZE_TAGS', '').replace(',', ' ').split():
            raise ValueError('ZE_TAGS must include integration')
        scratch = inside(os.environ['GOTMPDIR'], cwd)
        if not scratch.is_dir():
            raise ValueError('GOTMPDIR must be an existing workspace directory')
        if sha(LE) != LE_SHA or sha(KERNEL) != KERNEL_SHA:
            raise ValueError('accepted le or kernel hash changed')
        # Go sends coverage destinations in test flags. Resolve every supplied
        # destination before translating; symlinks cannot escape the workspace.
        path_flags = {'-test.coverprofile', '-test.gocoverdir', '-test.outputdir',
                      '-test.cpuprofile', '-test.memprofile', '-test.blockprofile',
                      '-test.mutexprofile', '-test.trace', '-test.testlogfile'}
        flags = list(arguments[1:])
        outputdir = cwd
        for index, value in enumerate(flags):
            if value == '--':
                break
            key, equal, operand = value.partition('=')
            if key == '-test.outputdir':
                outputdir = inside(operand if equal else flags[index + 1], cwd)
        mapped = []
        coverage = []
        index = 0
        while index < len(flags):
            value = flags[index]
            key, equal, operand = value.partition('=')
            if value == '--':
                mapped.extend(flags[index:])
                break
            if key in path_flags:
                if not equal:
                    index += 1
                    operand = flags[index]
                if operand:
                    base = cwd if key in ('-test.outputdir', '-test.gocoverdir') else outputdir
                    path = inside(operand, base)
                    operand = guest_path(path)
                    if key in ('-test.coverprofile', '-test.gocoverdir'):
                        coverage.append({'flag': key, 'host': str(path), 'guest': operand})
                mapped.append(key + '=' + operand)
            else:
                mapped.append(value)
            index += 1
        guest_environment = {'CGO_ENABLED': '0', 'ZE_TAGS': os.environ['ZE_TAGS']}
        for key in ('GOMAXPROCS', 'GODEBUG', 'GOTRACEBACK'):
            if key in os.environ:
                guest_environment[key] = os.environ[key]
        if os.environ.get('GOCOVERDIR'):
            path = inside(os.environ['GOCOVERDIR'], cwd)
            guest_environment['GOCOVERDIR'] = guest_path(path)
            coverage.append({'flag': 'GOCOVERDIR', 'host': str(path), 'guest': guest_path(path)})
        config = {
            'host_argv': arguments, 'host_cwd': str(cwd),
            'guest_argv': [guest_path(binary)] + mapped, 'guest_cwd': guest_path(cwd),
            'guest_evidence': guest_path(out), 'guest_environment': guest_environment,
            'coverage_paths': coverage, 'binary': identity(binary),
            'kernel': identity(KERNEL), 'wrapper': identity(Path(__file__)), 'le': identity(LE),
            'environment_policy': 'Native action inherits host environment with GOFLAGS removed; guest uses native guest environment plus the listed runtime/coverage choices.'}
        save(out / 'config.json', config)
        command = shlex.join(['python3', guest_path(Path(__file__).resolve()),
                              '--guest', guest_path(out / 'config.json')])
        native_argv = [str(LE), 'le', 'qemu', 'run', 'kernel', str(KERNEL),
                       'packages', 'iproute2 python3', 'timeout', '8m', 'command', command]
        native_environment = os.environ.copy()
        native_environment.pop('GOFLAGS', None)
        action = run(native_argv, WORKSPACE, native_environment, out, 'native-action')
        if action.get('stop_reason') or action.get('execution_error'):
            raise RuntimeError('native action incomplete: ' + str(action))
        # Replay exact bytes, once, through the streams the Go parent provided.
        # The action's boot/JSON chatter stays in evidence rather than test output.
        for name, stream in (('test.stdout', sys.stdout.buffer), ('test.stderr', sys.stderr.buffer)):
            path = out / name
            if path.exists():
                with path.open('rb') as source:
                    shutil.copyfileobj(source, stream)
                stream.flush()
        # Go may remove its temporary cover directory after -exec returns.
        # Preserve a separate copy while leaving the originals in place for Go.
        saved_coverage = []
        for index, item in enumerate(coverage):
            source = Path(item['host'])
            snapshot = out / ('coverage-' + str(index))
            row = dict(item)
            row['exists_after_guest'] = source.exists()
            if source.is_dir():
                shutil.copytree(source, snapshot)
                row['snapshot'] = snapshot.name
            elif source.is_file():
                shutil.copy2(source, snapshot)
                row['snapshot'] = snapshot.name
                row['sha256'] = sha(source)
            saved_coverage.append(row)
        completion = out / 'guest-complete.json'
        observed = json.loads(completion.read_text()) if completion.exists() else {}
        code = observed.get('exit_code')
        if code is None:
            raise RuntimeError('guest completion missing; native action exit ' + str(action['exit_code']))
        if observed['binary_after']['sha256'] != config['binary']['sha256']:
            raise RuntimeError('test binary changed during native execution')
        # An infrastructure failure cannot turn a successful test into a proof.
        forwarded = status(code)
        if code == 0 and action['exit_code'] != 0:
            forwarded = 125
        save(out / 'host-result.json', {
            'guest_exit_code': code, 'native_action_exit_code': action['exit_code'],
            'forwarded_exit_code': forwarded, 'coverage': saved_coverage,
            'finished_ns': time.time_ns()})
        if forwarded == 125 and code == 0:
            print('rfc-native-exec: native action failed after guest exit zero; evidence ' + str(out), file=sys.stderr)
        return forwarded
    except Exception as exc:
        save(out / 'adapter-error.json', {'error': str(exc), 'exit_code': 125, 'at_ns': time.time_ns()})
        print('rfc-native-exec: ' + str(exc) + '; evidence ' + str(out), file=sys.stderr)
        return 125


if __name__ == '__main__':
    if len(sys.argv) == 3 and sys.argv[1] == '--guest':
        raise SystemExit(guest(sys.argv[2]))
    raise SystemExit(host(sys.argv[1:]))
